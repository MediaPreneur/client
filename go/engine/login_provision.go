// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package engine

import (
	"errors"
	"fmt"
	"sort"

	"golang.org/x/net/context"

	"github.com/keybase/client/go/kex2"
	"github.com/keybase/client/go/libkb"
	keybase1 "github.com/keybase/client/go/protocol"
)

// loginProvision is an engine that will provision the current
// device.  Only the Login engine should run it.
type loginProvision struct {
	libkb.Contextified
	arg           *loginProvisionArg
	lks           *libkb.LKSec
	signingKey    libkb.GenericKey
	encryptionKey libkb.GenericKey
	gpgCli        gpgInterface
	username      string
	devname       string
	cleanupOnErr  bool
	hasPGP        bool
	hasDevice     bool
}

// gpgInterface defines the portions of gpg client that provision
// needs.  This allows tests to stub out gpg client calls.
type gpgInterface interface {
	ImportKey(secret bool, fp libkb.PGPFingerprint) (*libkb.PGPKeyBundle, error)
	Index(secret bool, query string) (ki *libkb.GpgKeyIndex, w libkb.Warnings, err error)
}

type loginProvisionArg struct {
	DeviceType string // desktop or mobile
	ClientType keybase1.ClientType
	User       *libkb.User
}

// newLoginProvision creates a loginProvision engine.
func newLoginProvision(g *libkb.GlobalContext, arg *loginProvisionArg) *loginProvision {
	return &loginProvision{
		Contextified: libkb.NewContextified(g),
		arg:          arg,
	}
}

// Name is the unique engine name.
func (e *loginProvision) Name() string {
	return "loginProvision"
}

// GetPrereqs returns the engine prereqs.
func (e *loginProvision) Prereqs() Prereqs {
	return Prereqs{}
}

// RequiredUIs returns the required UIs.
func (e *loginProvision) RequiredUIs() []libkb.UIKind {
	return []libkb.UIKind{
		libkb.ProvisionUIKind,
		libkb.LoginUIKind,
		libkb.SecretUIKind,
		libkb.GPGUIKind,
	}
}

// SubConsumers returns the other UI consumers for this engine.
func (e *loginProvision) SubConsumers() []libkb.UIConsumer {
	return []libkb.UIConsumer{
		&DeviceWrap{},
		&PaperKeyPrimary{},
	}
}

// Run starts the engine.
func (e *loginProvision) Run(ctx *Context) error {
	if err := e.checkArg(); err != nil {
		return err
	}

	e.cleanupOnErr = true
	// based on information in e.arg.User, route the user
	// through the provisioning options
	if err := e.route(ctx); err != nil {
		// cleanup state because there was an error:
		e.cleanup()
		return err
	}

	if err := e.ensurePaperKey(ctx); err != nil {
		return err
	}

	if err := e.displaySuccess(ctx); err != nil {
		return err
	}

	return nil
}

// deviceWithType provisions this device with an existing device using the
// kex2 protocol.  provisionerType is the existing device type.
func (e *loginProvision) deviceWithType(ctx *Context, provisionerType keybase1.DeviceType) error {
	// make a new secret:
	secret, err := libkb.NewKex2Secret()
	if err != nil {
		return err
	}
	e.G().Log.Debug("secret phrase: %s", secret.Phrase())

	// make a new device:
	deviceID, err := libkb.NewDeviceID()
	if err != nil {
		return err
	}
	device := &libkb.Device{
		ID:   deviceID,
		Type: e.arg.DeviceType,
	}

	// create provisionee engine
	provisionee := NewKex2Provisionee(e.G(), device, secret.Secret())

	var canceler func()

	// display secret and prompt for secret from X in a goroutine:
	go func() {
		sb := secret.Secret()
		arg := keybase1.DisplayAndPromptSecretArg{
			Secret:          sb[:],
			Phrase:          secret.Phrase(),
			OtherDeviceType: provisionerType,
		}
		var contxt context.Context
		contxt, canceler = context.WithCancel(context.Background())
		receivedSecret, err := ctx.ProvisionUI.DisplayAndPromptSecret(contxt, arg)
		if err != nil {
			// cancel provisionee run:
			provisionee.Cancel()
			e.G().Log.Warning("DisplayAndPromptSecret error: %s", err)
		} else if receivedSecret.Secret != nil && len(receivedSecret.Secret) > 0 {
			e.G().Log.Debug("received secret, adding to provisionee")
			var ks kex2.Secret
			copy(ks[:], receivedSecret.Secret)
			provisionee.AddSecret(ks)
		} else if len(receivedSecret.Phrase) > 0 {
			e.G().Log.Debug("received secret phrase, adding to provisionee")
			ks, err := libkb.NewKex2SecretFromPhrase(receivedSecret.Phrase)
			if err != nil {
				e.G().Log.Warning("DisplayAndPromptSecret error: %s", err)
			} else {
				provisionee.AddSecret(ks.Secret())
			}
		}
	}()

	defer func() {
		if canceler != nil {
			e.G().Log.Debug("canceling DisplayAndPromptSecret call")
			canceler()
		}
	}()

	f := func(lctx libkb.LoginContext) error {
		// run provisionee
		ctx.LoginContext = lctx
		return RunEngine(provisionee, ctx)
	}
	if err := e.G().LoginState().ExternalFunc(f, "loginProvision.device - Run provisionee"); err != nil {
		return err
	}

	// need username, device name for ProvisionUI.ProvisioneeSuccess()
	e.username = provisionee.GetName()
	pdevice := provisionee.Device()
	if pdevice == nil {
		e.G().Log.Warning("nil provisionee device")
	} else if pdevice.Description == nil {
		e.G().Log.Warning("nil provisionee device description")
	} else {
		e.devname = *pdevice.Description
	}

	return nil
}

// paper attempts to provision the device via a paper key.
func (e *loginProvision) paper(ctx *Context, device *libkb.Device) error {
	// get the paper key from the user
	kp, err := e.getValidPaperKey(ctx)
	if err != nil {
		return err
	}

	e.G().Log.Debug("paper signing key kid: %s", kp.sigKey.GetKID())
	e.G().Log.Debug("paper encryption key kid: %s", kp.encKey.GetKID())

	// After obtaining login session, this will be called before the login state is released.
	// It signs this new device with the paper key.
	var afterLogin = func(lctx libkb.LoginContext) error {
		ctx.LoginContext = lctx

		// need lksec to store device keys locally
		if err := e.fetchLKS(ctx, kp.encKey); err != nil {
			return err
		}

		if err := e.makeDeviceKeysWithSigner(ctx, kp.sigKey); err != nil {
			return err
		}
		if err := lctx.LocalSession().SetDeviceProvisioned(e.G().Env.GetDeviceID()); err != nil {
			// not a fatal error, session will stay in memory
			e.G().Log.Warning("error saving session file: %s", err)
		}
		return nil
	}

	// need a session to continue to provision, login with paper sigKey
	return e.G().LoginState().LoginWithKey(ctx.LoginContext, e.arg.User, kp.sigKey, afterLogin)
}

func (e *loginProvision) getValidPaperKey(ctx *Context) (*keypair, error) {
	var lastErr error
	for i := 0; i < 10; i++ {
		// get the paper key from the user
		kp, err := e.getPaperKey(ctx)
		if err != nil {
			e.G().Log.Debug("getValidPaperKey attempt %d: %s", i, err)
			lastErr = err
			continue
		}

		// use the KID to find the uid
		uid, err := e.uidByKID(kp.sigKey.GetKID())
		if err != nil {
			e.G().Log.Debug("getValidPaperKey attempt %d: %s", i, err)
			lastErr = err
			continue
		}

		if uid.NotEqual(e.arg.User.GetUID()) {
			e.G().Log.Debug("paper key entered was for a different user")
			lastErr = fmt.Errorf("paper key valid, but for %s, not %s", uid, e.arg.User.GetUID())
			continue
		}

		// found a paper key that can be used for signing
		e.G().Log.Debug("found paper key match for %s", e.arg.User.GetName())
		return kp, nil
	}

	e.G().Log.Debug("getValidPaperKey retry attempts exhausted")
	return nil, lastErr
}

// pgpProvision attempts to provision with a synced pgp key.  It
// needs to get a session first to look for a synced pgp key.
func (e *loginProvision) pgpProvision(ctx *Context) error {
	// After obtaining login session, this will be called before the login state is released.
	// It tries to get the pgp key and uses it to provision new device keys for this device.
	var afterLogin = func(lctx libkb.LoginContext) error {
		ctx.LoginContext = lctx
		signer, err := e.syncedPGPKey(ctx)
		if err != nil {
			return err
		}

		if err := e.makeDeviceKeysWithSigner(ctx, signer); err != nil {
			return err
		}
		if err := lctx.LocalSession().SetDeviceProvisioned(e.G().Env.GetDeviceID()); err != nil {
			// not a fatal error, session will stay in memory
			e.G().Log.Warning("error saving session file: %s", err)
		}
		return nil
	}

	// need a session to try to get synced private key
	return e.G().LoginState().LoginWithPrompt(e.arg.User.GetName(), ctx.LoginUI, ctx.SecretUI, afterLogin)
}

// makeDeviceKeysWithSigner creates device keys given a signing
// key.
func (e *loginProvision) makeDeviceKeysWithSigner(ctx *Context, signer libkb.GenericKey) error {
	args, err := e.makeDeviceWrapArgs(ctx)
	if err != nil {
		return err
	}
	args.Signer = signer
	args.IsEldest = false // just to be explicit
	args.EldestKID = e.arg.User.GetEldestKID()

	return e.makeDeviceKeys(ctx, args)
}

// makeDeviceWrapArgs creates a base set of args for DeviceWrap.
// It ensures that LKSec is created.  It also gets a new device
// name for this device.
func (e *loginProvision) makeDeviceWrapArgs(ctx *Context) (*DeviceWrapArgs, error) {
	if err := e.ensureLKSec(ctx); err != nil {
		return nil, err
	}

	devname, err := e.deviceName(ctx)
	if err != nil {
		return nil, err
	}
	e.devname = devname

	return &DeviceWrapArgs{
		Me:         e.arg.User,
		DeviceName: e.devname,
		DeviceType: e.arg.DeviceType,
		Lks:        e.lks,
	}, nil
}

// ensureLKSec ensures we have LKSec for saving device keys.
func (e *loginProvision) ensureLKSec(ctx *Context) error {
	if e.lks != nil {
		return nil
	}

	pps, err := e.ppStream(ctx)
	if err != nil {
		return err
	}

	e.lks = libkb.NewLKSec(pps, e.arg.User.GetUID(), e.G())
	return nil
}

// ppStream gets the passphrase stream, either cached or via
// SecretUI.
func (e *loginProvision) ppStream(ctx *Context) (*libkb.PassphraseStream, error) {
	if ctx.LoginContext != nil {
		cached := ctx.LoginContext.PassphraseStreamCache()
		if cached == nil {
			return nil, errors.New("loginProvision: ppStream() -> nil PassphraseStreamCache")
		}
		return cached.PassphraseStream(), nil
	}
	return e.G().LoginState().GetPassphraseStreamForUser(ctx.SecretUI, e.arg.User.GetName())
}

// deviceName gets a new device name from the user.
func (e *loginProvision) deviceName(ctx *Context) (string, error) {
	names, err := e.arg.User.DeviceNames()
	if err != nil {
		e.G().Log.Debug("error getting device names: %s", err)
		e.G().Log.Debug("proceeding to ask user for a device name despite error...")
	}
	arg := keybase1.PromptNewDeviceNameArg{
		ExistingDevices: names,
	}

	for i := 0; i < 10; i++ {
		devname, err := ctx.ProvisionUI.PromptNewDeviceName(ctx.GetNetContext(), arg)
		if err != nil {
			return "", err
		}
		if !libkb.CheckDeviceName.F(devname) {
			arg.ErrorMessage = "Invalid device name. Device names should be " + libkb.CheckDeviceName.Hint
			continue
		}
		duplicate := false
		for _, name := range names {
			if devname == name {
				duplicate = true
				break
			}
		}

		if duplicate {
			arg.ErrorMessage = fmt.Sprintf("Device name %q already used", devname)
			continue
		}

		return devname, nil
	}

	return "", libkb.RetryExhaustedError{}
}

// makeDeviceKeys uses DeviceWrap to generate device keys.
func (e *loginProvision) makeDeviceKeys(ctx *Context, args *DeviceWrapArgs) error {
	eng := NewDeviceWrap(args, e.G())
	if err := RunEngine(eng, ctx); err != nil {
		return err
	}

	e.signingKey = eng.SigningKey()
	e.encryptionKey = eng.EncryptionKey()
	return nil
}

// syncedPGPKey looks for a synced pgp key for e.user.  If found,
// it unlocks it.
func (e *loginProvision) syncedPGPKey(ctx *Context) (libkb.GenericKey, error) {
	key, err := e.arg.User.SyncedSecretKey(ctx.LoginContext)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, libkb.NoSyncedPGPKeyError{}
	}

	e.G().Log.Debug("got synced secret key")

	// unlock it
	// XXX improve this prompt
	parg := ctx.SecretKeyPromptArg(libkb.SecretKeyArg{}, "sign new device")
	unlocked, err := key.PromptAndUnlock(parg, "keybase", nil, e.arg.User)
	if err != nil {
		return nil, err
	}

	e.G().Log.Debug("unlocked secret key")
	return unlocked, nil
}

// gpgPrivateIndex returns an index of the private gpg keys.
func (e *loginProvision) gpgPrivateIndex() (*libkb.GpgKeyIndex, error) {
	cli, err := e.gpgClient()
	if err != nil {
		return nil, err
	}

	// get an index of all the secret keys
	index, _, err := cli.Index(true, "")
	if err != nil {
		return nil, err
	}

	return index, nil
}

// gpgClient returns a gpg client.
func (e *loginProvision) gpgClient() (gpgInterface, error) {
	if e.gpgCli != nil {
		return e.gpgCli, nil
	}

	gpg := e.G().GetGpgClient()
	if err := gpg.Configure(); err != nil {
		return nil, err
	}
	e.gpgCli = gpg
	return e.gpgCli, nil
}

// checkArg checks loginProvisionArg for sane arguments.
func (e *loginProvision) checkArg() error {
	// check we have a good device type:
	if e.arg.DeviceType != libkb.DeviceTypeDesktop && e.arg.DeviceType != libkb.DeviceTypeMobile {
		return libkb.InvalidArgumentError{Msg: fmt.Sprintf("device type must be %q or %q, not %q", libkb.DeviceTypeDesktop, libkb.DeviceTypeMobile, e.arg.DeviceType)}
	}

	if e.arg.User == nil {
		return libkb.InvalidArgumentError{Msg: "User cannot be nil"}
	}

	return nil
}

func (e *loginProvision) route(ctx *Context) error {
	// check if User has any pgp keys, active devices
	ckf := e.arg.User.GetComputedKeyFamily()
	if ckf != nil {
		e.hasPGP = len(ckf.GetActivePGPKeys(false)) > 0
		e.hasDevice = ckf.HasActiveDevice()
	}

	if e.hasDevice {
		return e.chooseDevice(ctx, e.hasPGP)
	}

	if e.hasPGP {
		return e.tryPGP(ctx)
	}

	// User has no existing devices or pgp keys, so create
	// the eldest device.
	return e.makeEldestDevice(ctx)
}

func (e *loginProvision) chooseDevice(ctx *Context, pgp bool) error {
	ckf := e.arg.User.GetComputedKeyFamily()
	devices := partitionDeviceList(ckf.GetAllActiveDevices())
	sort.Sort(devices)

	expDevices := make([]keybase1.Device, len(devices))
	idMap := make(map[keybase1.DeviceID]*libkb.Device)
	for i, d := range devices {
		expDevices[i] = *d.ProtExportFriendly()
		idMap[d.ID] = d
	}

	arg := keybase1.ChooseDeviceArg{
		Devices: expDevices,
	}
	id, err := ctx.ProvisionUI.ChooseDevice(ctx.GetNetContext(), arg)
	if err != nil {
		return err
	}

	if len(id) == 0 {
		// they chose not to use a device
		e.G().Log.Debug("user has devices, but chose not to use any of them")
		if pgp {
			// they have pgp keys, so try that:
			return e.tryPGP(ctx)
		}
		// tell them they need to reset their account
		return libkb.ProvisionUnavailableError{}
	}

	e.G().Log.Debug("user selected device %s", id)
	selected, ok := idMap[id]
	if !ok {
		return fmt.Errorf("selected device %s not in local device map", id)
	}
	e.G().Log.Debug("device details: %+v", selected)

	switch selected.Type {
	case libkb.DeviceTypePaper:
		return e.paper(ctx, selected)
	case libkb.DeviceTypeDesktop:
		return e.deviceWithType(ctx, keybase1.DeviceType_DESKTOP)
	case libkb.DeviceTypeMobile:
		return e.deviceWithType(ctx, keybase1.DeviceType_MOBILE)
	default:
		return fmt.Errorf("unknown device type: %v", selected.Type)
	}
}

func (e *loginProvision) tryPGP(ctx *Context) error {
	err := e.pgpProvision(ctx)
	if err == nil {
		return nil
	}

	if _, ok := err.(libkb.NoSyncedPGPKeyError); !ok {
		// error during pgpProvision was not about no synced pgp key,
		// so return it
		return err
	}

	e.G().Log.Debug("no synced pgp key found, trying GPG")
	return e.tryGPG(ctx)
}

func (e *loginProvision) tryGPG(ctx *Context) error {
	key, method, err := e.chooseGPGKeyAndMethod(ctx)
	if err != nil {
		return err
	}

	// depending on the method, get a signing key
	var signingKey libkb.GenericKey
	switch method {
	case keybase1.GPGMethod_GPG_IMPORT:
		signingKey, err = e.gpgImportKey(ctx, key.GetFingerprint())
		if err != nil {
			// there was an error importing the key.
			// so offer to switch to using gpg to sign
			// the provisioning statement:
			signingKey, err = e.switchToGPGSign(ctx, key, err)
			if err != nil {
				return err
			}
			method = keybase1.GPGMethod_GPG_SIGN
		}
	case keybase1.GPGMethod_GPG_SIGN:
		signingKey, err = e.gpgSignKey(ctx, key.GetFingerprint())
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid gpg provisioning method: %v", method)
	}

	// After obtaining login session, this will be called before the login state is released.
	// It signs this new device with the selected gpg key.
	var afterLogin = func(lctx libkb.LoginContext) error {
		ctx.LoginContext = lctx
		if err := e.makeDeviceKeysWithSigner(ctx, signingKey); err != nil {
			return err
		}
		if err := lctx.LocalSession().SetDeviceProvisioned(e.G().Env.GetDeviceID()); err != nil {
			// not a fatal error, session will stay in memory
			e.G().Log.Warning("error saving session file: %s", err)
		}

		if method == keybase1.GPGMethod_GPG_IMPORT {
			// store the key in lksec
			_, err := libkb.WriteLksSKBToKeyring(e.G(), signingKey, e.lks, lctx)
			if err != nil {
				e.G().Log.Warning("error saving exported gpg key in lksec: %s", err)
				return err
			}
		}

		return nil
	}

	// need a session to continue to provision
	return e.G().LoginState().LoginWithPrompt(e.arg.User.GetName(), ctx.LoginUI, ctx.SecretUI, afterLogin)
}

func (e *loginProvision) chooseGPGKeyAndMethod(ctx *Context) (*libkb.GpgPrimaryKey, keybase1.GPGMethod, error) {
	nilMethod := keybase1.GPGMethod_GPG_NONE
	// find any local private gpg keys that are in user's key family
	matches, err := e.matchingGPGKeys()
	if err != nil {
		if _, ok := err.(libkb.NoSecretKeyError); ok {
			// no match found
			// tell the user they need to get a gpg
			// key onto this device.
		}
		return nil, nilMethod, err
	}

	// have a match
	for _, m := range matches {
		e.G().Log.Debug("matching gpg key: %+v", m)
	}

	// create protocol array of keys
	var gks []keybase1.GPGKey
	gkmap := make(map[string]*libkb.GpgPrimaryKey)
	for _, key := range matches {
		gk := keybase1.GPGKey{
			Algorithm:  key.AlgoString(),
			KeyID:      key.ID64,
			Creation:   key.CreatedString(),
			Identities: key.GetPGPIdentities(),
		}
		gks = append(gks, gk)
		gkmap[key.ID64] = key
	}

	// ask if they want to import or sign
	arg := keybase1.ChooseGPGMethodArg{
		Keys: gks,
	}
	method, err := ctx.ProvisionUI.ChooseGPGMethod(ctx.GetNetContext(), arg)
	if err != nil {
		return nil, nilMethod, err
	}

	// select the key to use
	var key *libkb.GpgPrimaryKey
	if len(matches) == 1 {
		key = matches[0]
	} else {
		// if more than one match, show the user the matching keys, ask for selection
		keyid, err := ctx.GPGUI.SelectKey(ctx.GetNetContext(), keybase1.SelectKeyArg{Keys: gks})
		if err != nil {
			return nil, nilMethod, err
		}

		var ok bool
		key, ok = gkmap[keyid]
		if !ok {
			return nil, nilMethod, fmt.Errorf("key id %v from select key not in local gpg key map", keyid)
		}
	}

	e.G().Log.Debug("using gpg key %v for provisioning", key)

	return key, method, nil
}

func (e *loginProvision) switchToGPGSign(ctx *Context, key *libkb.GpgPrimaryKey, importError error) (libkb.GenericKey, error) {
	gk := keybase1.GPGKey{
		Algorithm:  key.AlgoString(),
		KeyID:      key.ID64,
		Creation:   key.CreatedString(),
		Identities: key.GetPGPIdentities(),
	}
	arg := keybase1.SwitchToGPGSignOKArg{
		Key:         gk,
		ImportError: importError.Error(),
	}
	ok, err := ctx.ProvisionUI.SwitchToGPGSignOK(ctx.GetNetContext(), arg)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("user chose not to switch to GPG sign, original import error: %s", importError)
	}

	e.G().Log.Debug("switching to GPG sign")
	return e.gpgSignKey(ctx, key.GetFingerprint())
}

func (e *loginProvision) matchingGPGKeys() ([]*libkb.GpgPrimaryKey, error) {
	index, err := e.gpgPrivateIndex()
	if err != nil {
		return nil, err
	}

	kfKeys := e.arg.User.GetComputedKeyFamily().GetActivePGPKeys(false)

	if index.Len() == 0 {
		e.G().Log.Debug("no private gpg keys found")
		return nil, e.newGPGMatchErr(kfKeys)
	}

	// iterate through pgp keys in keyfamily
	var matches []*libkb.GpgPrimaryKey
	for _, kfKey := range kfKeys {
		// find matches in gpg index
		gpgKeys := index.Fingerprints.Get(kfKey.GetFingerprint().String())
		if len(gpgKeys) > 0 {
			matches = append(matches, gpgKeys...)
		}
	}

	if len(matches) == 0 {
		// if none exist, then abort with error that they need to get
		// the private key for one of the pgp keys in the keyfamily
		// onto this device.
		e.G().Log.Debug("no matching private gpg keys found")
		return nil, e.newGPGMatchErr(kfKeys)
	}

	return matches, nil
}

func (e *loginProvision) newGPGMatchErr(keys []*libkb.PGPKeyBundle) error {
	fps := make([]string, len(keys))
	for i, k := range keys {
		fps[i] = k.GetFingerprint().ToQuads()
	}
	return libkb.NoMatchingGPGKeysError{Fingerprints: fps, HasActiveDevice: e.hasDevice}
}

func (e *loginProvision) gpgSignKey(ctx *Context, fp *libkb.PGPFingerprint) (libkb.GenericKey, error) {
	kf := e.arg.User.GetComputedKeyFamily()
	if kf == nil {
		return nil, libkb.KeyFamilyError{Msg: "no key family for user"}
	}
	kid, err := kf.FindKIDFromFingerprint(*fp)
	if err != nil {
		return nil, err
	}

	// create a GPGKey shell around gpg cli with fp, kid
	return libkb.NewGPGKey(e.G(), fp, kid, ctx.GPGUI, e.arg.ClientType), nil
}

func (e *loginProvision) gpgImportKey(ctx *Context, fp *libkb.PGPFingerprint) (libkb.GenericKey, error) {
	// import it with gpg
	cli, err := e.gpgClient()
	if err != nil {
		return nil, err
	}
	bundle, err := cli.ImportKey(true, *fp)
	if err != nil {
		return nil, err
	}

	// unlock it
	if err := bundle.Unlock("sign new device", ctx.SecretUI); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (e *loginProvision) makeEldestDevice(ctx *Context) error {
	if !e.arg.User.GetEldestKID().IsNil() {
		// this shouldn't happen, but make sure
		return errors.New("eldest called on user with existing eldest KID")
	}

	args, err := e.makeDeviceWrapArgs(ctx)
	if err != nil {
		return err
	}
	args.IsEldest = true

	if err := e.makeDeviceKeys(ctx, args); err != nil {
		return err
	}

	// save provisioned device id in the session
	return e.setSessionDeviceID(e.G().Env.GetDeviceID())
}

// ensurePaperKey checks to see if e.user has any paper keys.  If
// not, it makes one.
func (e *loginProvision) ensurePaperKey(ctx *Context) error {
	// see if they have a paper key already
	cki := e.arg.User.GetComputedKeyInfos()
	if cki != nil {
		if len(cki.PaperDevices()) > 0 {
			return nil
		}
	}

	// make one
	args := &PaperKeyPrimaryArgs{
		SigningKey: e.signingKey,
		Me:         e.arg.User,
	}
	eng := NewPaperKeyPrimary(e.G(), args)
	return RunEngine(eng, ctx)
}

func (e *loginProvision) getPaperKey(ctx *Context) (*keypair, error) {
	passphrase, err := libkb.GetPaperKeyPassphrase(ctx.SecretUI, "")
	if err != nil {
		return nil, err
	}

	paperPhrase, err := libkb.NewPaperKeyPhraseCheckVersion(e.G(), passphrase)
	if err != nil {
		return nil, err
	}

	bkarg := &PaperKeyGenArg{
		Passphrase: paperPhrase,
		SkipPush:   true,
	}
	bkeng := NewPaperKeyGen(bkarg, e.G())
	if err := RunEngine(bkeng, ctx); err != nil {
		return nil, err
	}

	kp := &keypair{sigKey: bkeng.SigKey(), encKey: bkeng.EncKey()}
	if err := e.G().LoginState().Account(func(a *libkb.Account) {
		a.SetUnlockedPaperKey(kp.sigKey, kp.encKey)
	}, "UnlockedPaperKey"); err != nil {
		return nil, err
	}

	return kp, nil
}

func (e *loginProvision) uidByKID(kid keybase1.KID) (keybase1.UID, error) {
	var nilUID keybase1.UID
	arg := libkb.APIArg{
		Endpoint:     "key/owner",
		NeedSession:  false,
		Contextified: libkb.NewContextified(e.G()),
		Args:         libkb.HTTPArgs{"kid": libkb.S{Val: kid.String()}},
	}
	res, err := e.G().API.Get(arg)
	if err != nil {
		return nilUID, err
	}
	suid, err := res.Body.AtPath("uid").GetString()
	if err != nil {
		return nilUID, err
	}
	return keybase1.UIDFromString(suid)
}

func (e *loginProvision) fetchLKS(ctx *Context, encKey libkb.GenericKey) error {
	gen, clientLKS, err := fetchLKS(ctx, e.G(), encKey)
	if err != nil {
		return err
	}
	e.lks = libkb.NewLKSecWithClientHalf(clientLKS, gen, e.arg.User.GetUID(), e.G())
	return nil
}

func (e *loginProvision) setSessionDeviceID(id keybase1.DeviceID) error {
	var serr error
	if err := e.G().LoginState().LocalSession(func(s *libkb.Session) {
		serr = s.SetDeviceProvisioned(id)
	}, "loginProvision - device"); err != nil {
		return err
	}
	return serr
}

func (e *loginProvision) displaySuccess(ctx *Context) error {
	if len(e.username) == 0 && e.arg.User != nil {
		e.username = e.arg.User.GetName()
	}
	sarg := keybase1.ProvisioneeSuccessArg{
		Username:   e.username,
		DeviceName: e.devname,
	}
	return ctx.ProvisionUI.ProvisioneeSuccess(ctx.GetNetContext(), sarg)
}

func (e *loginProvision) cleanup() {
	if !e.cleanupOnErr {
		return
	}

	// the best way to cleanup is to logout...
	e.G().Log.Debug("an error occurred during provisioning, logging out")
	e.G().Logout()
}

var devtypeSortOrder = map[string]int{libkb.DeviceTypeMobile: 0, libkb.DeviceTypeDesktop: 1, libkb.DeviceTypePaper: 2}

type partitionDeviceList []*libkb.Device

func (p partitionDeviceList) Len() int {
	return len(p)
}

func (p partitionDeviceList) Less(a, b int) bool {
	if p[a].Type != p[b].Type {
		return devtypeSortOrder[p[a].Type] < devtypeSortOrder[p[b].Type]
	}
	return *p[a].Description < *p[b].Description
}

func (p partitionDeviceList) Swap(a, b int) {
	p[a], p[b] = p[b], p[a]
}
