// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package engine

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"golang.org/x/net/context"

	"github.com/keybase/client/go/kex2"
	"github.com/keybase/client/go/libkb"
	keybase1 "github.com/keybase/client/go/protocol"
)

func TestLoginLogoutLogin(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := CreateAndSignupFakeUser(tc, "login")
	Logout(tc)
	u1.LoginOrBust(tc)
}

// Test login switching between two different users.
func TestLoginAndSwitch(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := CreateAndSignupFakeUser(tc, "first")
	Logout(tc)
	u2 := CreateAndSignupFakeUser(tc, "secon")
	Logout(tc)
	t.Logf("first logging back in")
	u1.LoginOrBust(tc)
	Logout(tc)
	t.Logf("second logging back in")
	u2.LoginOrBust(tc)

	return
}

func TestCreateFakeUserNoKeys(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	createFakeUserWithNoKeys(tc)

	me, err := libkb.LoadMe(libkb.NewLoadUserPubOptionalArg(tc.G))
	if err != nil {
		t.Fatal(err)
	}

	kf := me.GetKeyFamily()
	if kf == nil {
		t.Fatal("user has a nil key family")
	}
	if me.GetEldestKID().Exists() {
		t.Fatalf("user has an eldest key, they should have no keys: %s", me.GetEldestKID())
	}

	ckf := me.GetComputedKeyFamily()
	if ckf.HasActiveKey() {
		t.Errorf("user has an active key, but they should have no keys")
	}
}

func testUserHasDeviceKey(tc libkb.TestContext) {
	me, err := libkb.LoadMe(libkb.NewLoadUserPubOptionalArg(tc.G))
	if err != nil {
		tc.T.Fatal(err)
	}

	kf := me.GetKeyFamily()
	if kf == nil {
		tc.T.Fatal("user has a nil key family")
	}
	if me.GetEldestKID().IsNil() {
		tc.T.Fatal("user has no eldest key")
	}

	ckf := me.GetComputedKeyFamily()
	if ckf == nil {
		tc.T.Fatalf("user has no computed key family")
	}

	active := ckf.HasActiveKey()
	if !active {
		tc.T.Errorf("user has no active key")
	}

	subkey, err := me.GetDeviceSubkey()
	if err != nil {
		tc.T.Fatal(err)
	}
	if subkey == nil {
		tc.T.Fatal("nil subkey")
	}
}

func TestUserInfo(t *testing.T) {
	t.Skip()
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u := CreateAndSignupFakeUser(tc, "login")
	var username libkb.NormalizedUsername
	var err error
	aerr := tc.G.LoginState().Account(func(a *libkb.Account) {
		_, username, _, _, _, err = a.UserInfo()
	}, "TestUserInfo")
	if aerr != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !username.Eq(libkb.NewNormalizedUsername(u.Username)) {
		t.Errorf("userinfo username: %q, expected %q", username, u.Username)
	}
}

func TestProvisionDesktop(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "kex2provision")
	defer tcX.Cleanup()

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "template")
	defer tcY.Cleanup()

	// provisioner needs to be logged in
	userX := CreateAndSignupFakeUser(tcX, "login")
	var secretX kex2.Secret
	if _, err := rand.Read(secretX[:]); err != nil {
		t.Fatal(err)
	}

	secretCh := make(chan kex2.Secret)

	// provisionee calls login:
	ctx := &Context{
		ProvisionUI: newTestProvisionUISecretCh(secretCh),
		LoginUI:     &libkb.TestLoginUI{Username: userX.Username},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)

	var wg sync.WaitGroup

	// start provisionee
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := RunEngine(eng, ctx); err != nil {
			t.Errorf("login error: %s", err)
			return
		}
	}()

	// start provisioner
	provisioner := NewKex2Provisioner(tcX.G, secretX, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx := &Context{
			SecretUI:    userX.NewSecretUI(),
			ProvisionUI: newTestProvisionUI(),
		}
		if err := RunEngine(provisioner, ctx); err != nil {
			t.Errorf("provisioner error: %s", err)
			return
		}
	}()
	secretFromY := <-secretCh
	provisioner.AddSecret(secretFromY)

	wg.Wait()

	if err := AssertProvisioned(tcY); err != nil {
		t.Fatal(err)
	}

	// make sure that the provisioned device can use
	// the passphrase stream cache (use an empty secret ui)
	arg := &TrackEngineArg{
		UserAssertion: "t_alice",
		Options:       keybase1.TrackOptions{BypassConfirm: true},
	}
	ctx = &Context{
		LogUI:      tcY.G.UI.GetLogUI(),
		IdentifyUI: &FakeIdentifyUI{},
		SecretUI:   &libkb.TestSecretUI{},
	}

	teng := NewTrackEngine(arg, tcY.G)
	if err := RunEngine(teng, ctx); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionMobile(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "kex2provision")
	defer tcX.Cleanup()

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "template")
	defer tcY.Cleanup()

	// provisioner needs to be logged in
	userX := CreateAndSignupFakeUser(tcX, "login")
	var secretX kex2.Secret
	if _, err := rand.Read(secretX[:]); err != nil {
		t.Fatal(err)
	}

	secretCh := make(chan kex2.Secret)

	// provisionee calls login:
	ctx := &Context{
		ProvisionUI: newTestProvisionUISecretCh(secretCh),
		LoginUI:     &libkb.TestLoginUI{Username: userX.Username},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeMobile, "", keybase1.ClientType_CLI)

	var wg sync.WaitGroup

	// start provisionee
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := RunEngine(eng, ctx); err != nil {
			t.Errorf("login error: %s", err)
			return
		}
	}()

	// start provisioner
	provisioner := NewKex2Provisioner(tcX.G, secretX, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx := &Context{
			SecretUI:    userX.NewSecretUI(),
			ProvisionUI: newTestProvisionUI(),
		}
		if err := RunEngine(provisioner, ctx); err != nil {
			t.Errorf("provisioner error: %s", err)
			return
		}
	}()
	secretFromY := <-secretCh
	provisioner.AddSecret(secretFromY)

	wg.Wait()

	if err := AssertProvisioned(tcY); err != nil {
		t.Fatal(err)
	}
}

// If a user has device keys and no pgp keys,
// not selecting a device should result in
// ProvisionUnavailable.
func TestProvisionChooseNoDeviceWithoutPGP(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "provision_x")
	defer tcX.Cleanup()

	// create user (and device X)
	userX := CreateAndSignupFakeUser(tcX, "login")

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "provision_y")
	defer tcY.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIChooseNoDevice(),
		LoginUI:     &libkb.TestLoginUI{Username: userX.Username},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	err := RunEngine(eng, ctx)
	if err == nil {
		t.Fatal("expected login to fail, but it ran without error")
	}
	if _, ok := err.(libkb.ProvisionUnavailableError); !ok {
		t.Fatalf("expected ProvisionUnavailableError, got %T (%s)", err, err)
	}

	if err := AssertLoggedIn(tcY); err == nil {
		t.Fatal("should not be logged in")
	}
}

// If a user has no keys, provision via passphrase should work.
func TestProvisionPassphraseNoKeysSolo(t *testing.T) {
	tcWeb := SetupEngineTest(t, "web")
	defer tcWeb.Cleanup()

	username, passphrase := createFakeUserWithNoKeys(tcWeb)

	Logout(tcWeb)

	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any keys, login should have fixed that:
	testUserHasDeviceKey(tc)

	// and they should have a paper backup key
	hasOnePaperDev(tc, &FakeUser{Username: username, Passphrase: passphrase})

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

// Test bad name input (not valid username or email address).
func TestProvisionPassphraseBadName(t *testing.T) {
	tcWeb := SetupEngineTest(t, "web")
	defer tcWeb.Cleanup()

	_, passphrase := createFakeUserWithNoKeys(tcWeb)

	Logout(tcWeb)

	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: strings.Repeat("X", 20)},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	err := RunEngine(eng, ctx)
	if err == nil {
		t.Fatal("Provision via passphrase should have failed with bad name.")
	}
	if _, ok := err.(libkb.BadNameError); !ok {
		t.Fatalf("Provision via passphrase err type: %T, expected libkb.BadNameError", err)
	}
}

// If a user has (only) a synced pgp key, provision via passphrase
// should work.
func TestProvisionPassphraseSyncedPGP(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPOnly(t, tc)
	t.Log("Created fake user")
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc = SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any device keys, login should have fixed that:
	testUserHasDeviceKey(tc)

	// and they should have a paper backup key
	hasOnePaperDev(tc, u1)

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

// If a user has (only) a synced pgp key, provision via passphrase
// should work, if they specify email address as username.
func TestProvisionPassphraseSyncedPGPEmail(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPOnly(t, tc)
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc = SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Email},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any device keys, login should have fixed that:
	testUserHasDeviceKey(tc)

	// and they should have a paper backup key
	hasOnePaperDev(tc, u1)

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

// Check that a bad passphrase fails to unlock a synced pgp key
func TestProvisionSyncedPGPBadPassphrase(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPOnly(t, tc)
	t.Log("Created fake user")
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc = SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: u1.Passphrase + u1.Passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("sync pgp provision worked with bad passphrase")
	} else if _, ok := err.(libkb.PassphraseError); !ok {
		t.Errorf("error: %T, expected libkb.PassphraseError", err)
	}
}

// If a user is logged in as alice, then logs in as bob (who has
// no keys), provision via passphrase should work.
// Bug https://keybase.atlassian.net/browse/CORE-2605
func TestProvisionPassphraseNoKeysSwitchUser(t *testing.T) {
	// this is the web user
	tcWeb := SetupEngineTest(t, "web")
	username, passphrase := createFakeUserWithNoKeys(tcWeb)
	Logout(tcWeb)
	tcWeb.Cleanup()

	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	// this is a provisioned user.  stay logged in as this user
	// and start login process for web user.
	CreateAndSignupFakeUser(tc, "alice")

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, username, keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any keys, login should have fixed that:
	testUserHasDeviceKey(tc)

	t.Logf("user has device key")

	// and they should have a paper backup key
	hasOnePaperDev(tc, &FakeUser{Username: username, Passphrase: passphrase})

	t.Logf("user has paper device")

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionPaper(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()
	fu := NewFakeUserOrBust(t, "paper")
	arg := MakeTestSignupEngineRunArg(fu)
	loginUI := &paperLoginUI{Username: fu.Username}
	ctx := &Context{
		LogUI:    tc.G.UI.GetLogUI(),
		GPGUI:    &gpgtestui{},
		SecretUI: fu.NewSecretUI(),
		LoginUI:  loginUI,
	}
	s := NewSignupEngine(&arg, tc.G)
	err := RunEngine(s, ctx)
	if err != nil {
		tc.T.Fatal(err)
	}

	assertNumDevicesAndKeys(tc, fu, 2, 4)

	Logout(tc)

	if len(loginUI.PaperPhrase) == 0 {
		t.Fatal("login ui has no paper key phrase")
	}

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	fakeClock := clockwork.NewFakeClockAt(time.Now())
	tc2.G.Clock = fakeClock
	// to pick up the new clock...
	tc2.G.ResetLoginState()
	defer tc2.Cleanup()

	secUI := fu.NewSecretUI()
	secUI.Passphrase = loginUI.PaperPhrase
	provUI := newTestProvisionUIPaper()
	provLoginUI := &libkb.TestLoginUI{Username: fu.Username}
	ctx = &Context{
		ProvisionUI: provUI,
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    secUI,
		LoginUI:     provLoginUI,
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	assertNumDevicesAndKeys(tc, fu, 3, 6)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}

	if provUI.calledChooseDeviceType != 0 {
		t.Errorf("expected 0 calls to ChooseDeviceType, got %d", provUI.calledChooseDeviceType)
	}
	if provLoginUI.CalledGetEmailOrUsername != 1 {
		t.Errorf("expected 1 call to GetEmailOrUsername, got %d", provLoginUI.CalledGetEmailOrUsername)
	}
	var key libkb.GenericKey

	ch := make(chan struct{})
	pch := func() {
		ch <- struct{}{}
	}

	tc2.G.LoginState().Account(func(a *libkb.Account) {
		key = a.GetUnlockedPaperEncKey()
		a.SetTestPostCleanHook(pch)
	}, "GetUnlockedPaperEncKey")
	if key == nil {
		t.Errorf("Got a null paper encryption key")
	}

	fakeClock.Advance(libkb.PaperKeyMemoryTimeout + 1*time.Minute)
	<-ch

	tc2.G.LoginState().Account(func(a *libkb.Account) {
		key = a.GetUnlockedPaperEncKey()
	}, "GetUnlockedPaperEncKey")
	if key != nil {
		t.Errorf("Got a non-null paper encryption key after timeout")
	}
}

// Provision device using a private GPG key (not synced to keybase
// server), import private key to lksec.
func TestProvisionGPGImportOK(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGImport(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	// highly possible they didn't have a paper key, so make sure they have one now:
	hasOnePaperDev(tc2, u1)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}

	// since they imported their pgp key, they should be able to pgp sign something:
	if err := signString(tc2, "sign me", u1.NewSecretUI()); err != nil {
		t.Error("pgp sign failed after gpg provision w/ import")
		t.Fatal(err)
	}
}

// Provision device using a private GPG key (not synced to keybase
// server), import private key to lksec.  User selects key from
// several matching keys.
func TestProvisionGPGImportMultiple(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPMult(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGImport(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	// highly possible they didn't have a paper key, so make sure they have one now:
	hasOnePaperDev(tc2, u1)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}

	// since they imported their pgp key, they should be able to pgp sign something:
	if err := signString(tc2, "sign me", u1.NewSecretUI()); err != nil {
		t.Error("pgp sign failed after gpg provision w/ import")
		t.Fatal(err)
	}
}

// Provision device using a private GPG key (not synced to keybase
// server), use gpg to sign (no private key import).
func TestProvisionGPGSign(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGSign(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	// highly possible they didn't have a paper key, so make sure they have one now:
	hasOnePaperDev(tc2, u1)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}

	// since they *did not* import a pgp key, they should *not* be able to pgp sign something:
	if err := signString(tc2, "sign me", u1.NewSecretUI()); err == nil {
		t.Error("pgp sign worked after gpg provision w/o import")
		t.Fatal(err)
	}
}

func TestProvisionGPGSignFailedSign(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGSign(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgTestUIBadSign{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("expected a failure in login")
	}

	cf := tc2.G.Env.GetConfigFilename()
	jf := libkb.NewJSONConfigFile(tc2.G, cf)
	if err := jf.Load(true); err != nil {
		t.Fatal(err)
	}
	devid := jf.GetDeviceID()
	if !devid.IsNil() {
		t.Fatalf("got a non-nil Device ID after failed GPG provision (%v)", devid)
	}
}

// Provision device using a private GPG key (not synced to keybase
// server), use gpg to sign (no private key import).
// Enable secret storage.  keybase-issues#1822
func TestProvisionGPGSignSecretStore(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// create a secret UI that stores the secret
	secUI := u1.NewSecretUI()
	secUI.StoreSecret = true

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGSign(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    secUI,
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	// highly possible they didn't have a paper key, so make sure they have one now:
	hasOnePaperDev(tc2, u1)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}
}

// Provision device using a private GPG key (not synced to keybase
// server). Import private key to lksec fails, switches to gpg
// sign, which works.
func TestProvisionGPGSwitchToSign(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// load the user (bypassing LoginUsername for this test...)
	user, err := libkb.LoadUser(libkb.NewLoadUserByNameArg(tc2.G, u1.Username))
	if err != nil {
		t.Fatal(err)
	}

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGImport(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}

	arg := loginProvisionArg{
		DeviceType: libkb.DeviceTypeDesktop,
		ClientType: keybase1.ClientType_CLI,
		User:       user,
	}

	eng := newLoginProvision(tc2.G, &arg)
	// use a gpg client that will fail to import any gpg key
	eng.gpgCli = newGPGImportFailer(tc2.G)

	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	// highly possible they didn't have a paper key, so make sure they have one now:
	hasOnePaperDev(tc2, u1)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}

	// since they did not import their pgp key, they should not be able
	// to pgp sign something:
	if err := signString(tc2, "sign me", u1.NewSecretUI()); err == nil {
		t.Fatal("pgp sign worked after gpg sign provisioning")
	}
}

// Try provision device using a private GPG key (not synced to keybase
// server). Import private key to lksec fails, user does not want
// to switch to gpg sign, so provisioning fails.
func TestProvisionGPGNoSwitchToSign(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// load the user (bypassing LoginUsername for this test...)
	user, err := libkb.LoadUser(libkb.NewLoadUserByNameArg(tc2.G, u1.Username))
	if err != nil {
		t.Fatal(err)
	}

	// instruct provisioning ui to not allow the switch to gpg sign:
	provUI := newTestProvisionUIGPGImport()
	provUI.abortSwitchToGPGSign = true

	// run login on new device
	ctx := &Context{
		ProvisionUI: provUI,
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}

	arg := loginProvisionArg{
		DeviceType: libkb.DeviceTypeDesktop,
		ClientType: keybase1.ClientType_CLI,
		User:       user,
	}

	eng := newLoginProvision(tc2.G, &arg)
	// use a gpg client that will fail to import any gpg key
	eng.gpgCli = newGPGImportFailer(tc2.G)

	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("provisioning worked despite not allowing switch to gpg sign")
	}
}

// User with pgp keys, but on a device without any gpg keyring.
func TestProvisionGPGNoKeyring(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGImport(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("provision worked without gpg keyring")
	} else if _, ok := err.(libkb.NoMatchingGPGKeysError); !ok {
		t.Errorf("error %T, expected libkb.NoMatchingGPGKeysError", err)
	}
}

// User with pgp keys, but on a device with gpg keys that don't
// match.
func TestProvisionGPGNoMatch(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// make a new keyring, not associated with keybase
	if err := tc2.GenerateGPGKeyring(u1.Email); err != nil {
		t.Fatal(err)
	}

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGImport(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("provision worked without matching gpg key")
	} else if _, ok := err.(libkb.NoMatchingGPGKeysError); !ok {
		t.Errorf("error %T, expected libkb.NoMatchingGPGKeysError", err)
	}
}

// User with pgp keys, but on a device without gpg.
func TestProvisionGPGNoGPGInstalled(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// this should make it unable to find gpg
	tc2.G.Env.Test.GPG = "/dev/null"

	// run login on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPGImport(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("provision worked without gpg")
	}
	// XXX test a specific error, currently it's just returning an error string
}

func TestProvisionDupDevice(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "kex2provision")
	defer tcX.Cleanup()

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "template")
	defer tcY.Cleanup()

	// provisioner needs to be logged in
	userX := CreateAndSignupFakeUser(tcX, "login")
	var secretX kex2.Secret
	if _, err := rand.Read(secretX[:]); err != nil {
		t.Fatal(err)
	}

	secretCh := make(chan kex2.Secret)

	provui := &testProvisionDupDeviceUI{newTestProvisionUISecretCh(secretCh)}

	// provisionee calls login:
	ctx := &Context{
		ProvisionUI: provui,
		LoginUI:     &libkb.TestLoginUI{Username: userX.Username},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)

	var wg sync.WaitGroup

	// start provisionee
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := RunEngine(eng, ctx); err == nil {
			t.Errorf("login ran without error")
			return
		}
	}()

	// start provisioner
	provisioner := NewKex2Provisioner(tcX.G, secretX, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx := &Context{
			SecretUI:    userX.NewSecretUI(),
			ProvisionUI: newTestProvisionUI(),
		}
		if err := RunEngine(provisioner, ctx); err == nil {
			t.Errorf("provisioner ran without error")
			return
		}
	}()
	secretFromY := <-secretCh
	provisioner.AddSecret(secretFromY)

	wg.Wait()

	if err := AssertProvisioned(tcY); err == nil {
		t.Fatal("device provisioned using existing name")
	}
}

// If a user has no keys, provision via passphrase should work.
// This tests when they have another account on the same machine.
func TestProvisionPassphraseNoKeysMultipleAccounts(t *testing.T) {
	tcWeb := SetupEngineTest(t, "login")

	// create a "web" user with no keys
	username, passphrase := createFakeUserWithNoKeys(tcWeb)
	Logout(tcWeb)
	tcWeb.Cleanup()

	// create a new test context
	tc := SetupEngineTest(t, "fake")
	defer tc.Cleanup()

	// create a user to fill up config with something
	CreateAndSignupFakeUser(tc, "fake")
	Logout(tc)

	// now try to log in as the web user
	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, username, keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any keys, login should have fixed that:
	testUserHasDeviceKey(tc)

	// and they should have a paper backup key
	hasOnePaperDev(tc, &FakeUser{Username: username, Passphrase: passphrase})

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

// We have obviated the unlock command by combining it with login.
func TestLoginStreamCache(t *testing.T) {
	t.Skip()

	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := CreateAndSignupFakeUser(tc, "login")

	if !assertStreamCache(tc, true) {
		t.Fatal("expected valid stream cache after signup")
	}

	tc.G.LoginState().Account(func(a *libkb.Account) {
		a.ClearStreamCache()
	}, "clear stream cache")

	if !assertStreamCache(tc, false) {
		t.Fatal("expected invalid stream cache after clear")
	}

	// This should now unlock the stream cache too
	u1.LoginOrBust(tc)

	if !assertStreamCache(tc, true) {
		t.Fatal("expected valid stream cache after login")
	}
}

// Check the device type
func TestLoginInvalidDeviceType(t *testing.T) {
	tcWeb := SetupEngineTest(t, "web")
	defer tcWeb.Cleanup()

	username, passphrase := createFakeUserWithNoKeys(tcWeb)

	Logout(tcWeb)

	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypePaper, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("login with paper device type worked")
	} else if _, ok := err.(libkb.InvalidArgumentError); !ok {
		t.Errorf("err type: %T, expected libkb.InvalidArgumentError", err)
	}
}

// Test that login provision checks for nil user in argument.
func TestProvisionNilUser(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	arg := loginProvisionArg{
		DeviceType: libkb.DeviceTypeDesktop,
		ClientType: keybase1.ClientType_CLI,
		User:       nil,
	}
	eng := newLoginProvision(tc.G, &arg)
	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("loginprovision with nil user worked")
	} else if _, ok := err.(libkb.InvalidArgumentError); !ok {
		t.Errorf("err type: %T, expected libkb.InvalidArgumentError", err)
	}
}

func userPlusPaper(t *testing.T) (*FakeUser, string) {
	tc := SetupEngineTest(t, "fake")
	defer tc.Cleanup()
	fu := NewFakeUserOrBust(t, "fake")
	arg := MakeTestSignupEngineRunArg(fu)
	loginUI := &paperLoginUI{Username: fu.Username}
	ctx := &Context{
		LogUI:    tc.G.UI.GetLogUI(),
		GPGUI:    &gpgtestui{},
		SecretUI: fu.NewSecretUI(),
		LoginUI:  loginUI,
	}
	s := NewSignupEngine(&arg, tc.G)
	if err := RunEngine(s, ctx); err != nil {
		t.Fatal(err)
	}
	Logout(tc)
	return fu, loginUI.PaperPhrase
}

func TestProvisionPaperFailures(t *testing.T) {
	// create two users
	ux, uxPaper := userPlusPaper(t)
	_, uyPaper := userPlusPaper(t)

	// try provision as ux on a new device with uy's paper key
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	secUI := ux.NewSecretUI()
	secUI.Passphrase = uyPaper
	provUI := newTestProvisionUIPaper()
	provLoginUI := &libkb.TestLoginUI{Username: ux.Username}
	ctx := &Context{
		ProvisionUI: provUI,
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    secUI,
		LoginUI:     provLoginUI,
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("provision with another user's paper key worked")
	}

	// try provision as ux on a new device first with fu's paper key
	// then with ux's paper key (testing retry works)
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	retrySecUI := &testRetrySecretUI{
		Passphrases: []string{uyPaper, uxPaper},
	}
	provUI = newTestProvisionUIPaper()
	provLoginUI = &libkb.TestLoginUI{Username: ux.Username}
	ctx = &Context{
		ProvisionUI: provUI,
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    retrySecUI,
		LoginUI:     provLoginUI,
		GPGUI:       &gpgtestui{},
	}
	eng = NewLogin(tc2.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}
	if retrySecUI.index != len(retrySecUI.Passphrases) {
		t.Errorf("retry sec ui index: %d, expected %d", retrySecUI.index, len(retrySecUI.Passphrases))
	}

	// try provision as ux on a new device first with garbage paper key
	// then with ux's paper key (testing retry works)
	tc3 := SetupEngineTest(t, "login")
	defer tc3.Cleanup()

	retrySecUI = &testRetrySecretUI{
		Passphrases: []string{"garbage garbage garbage", uxPaper},
	}
	provUI = newTestProvisionUIPaper()
	provLoginUI = &libkb.TestLoginUI{Username: ux.Username}
	ctx = &Context{
		ProvisionUI: provUI,
		LogUI:       tc3.G.UI.GetLogUI(),
		SecretUI:    retrySecUI,
		LoginUI:     provLoginUI,
		GPGUI:       &gpgtestui{},
	}
	eng = NewLogin(tc3.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}
	if retrySecUI.index != len(retrySecUI.Passphrases) {
		t.Errorf("retry sec ui index: %d, expected %d", retrySecUI.index, len(retrySecUI.Passphrases))
	}

	// try provision as ux on a new device first with invalid version paper key
	// then with ux's paper key (testing retry works)
	tc4 := SetupEngineTest(t, "login")
	defer tc4.Cleanup()

	paperNextVer, err := libkb.MakePaperKeyPhrase(libkb.PaperKeyVersion + 1)
	if err != nil {
		t.Fatal(err)
	}
	retrySecUI = &testRetrySecretUI{
		Passphrases: []string{paperNextVer.String(), uxPaper},
	}
	provUI = newTestProvisionUIPaper()
	provLoginUI = &libkb.TestLoginUI{Username: ux.Username}
	ctx = &Context{
		ProvisionUI: provUI,
		LogUI:       tc4.G.UI.GetLogUI(),
		SecretUI:    retrySecUI,
		LoginUI:     provLoginUI,
		GPGUI:       &gpgtestui{},
	}
	eng = NewLogin(tc4.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}
	if retrySecUI.index != len(retrySecUI.Passphrases) {
		t.Errorf("retry sec ui index: %d, expected %d", retrySecUI.index, len(retrySecUI.Passphrases))
	}

}

// After kex provisioning, try using a synced pgp key to sign
// something.
func TestProvisionKexUseSyncPGP(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "kex2provision")
	defer tcX.Cleanup()

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "template")
	defer tcY.Cleanup()

	// create provisioner with synced pgp key
	userX := createFakeUserWithPGPSibkeyPushed(tcX)
	var secretX kex2.Secret
	if _, err := rand.Read(secretX[:]); err != nil {
		t.Fatal(err)
	}

	secretCh := make(chan kex2.Secret)

	// provisionee calls login:
	ctx := &Context{
		ProvisionUI: newTestProvisionUISecretCh(secretCh),
		LoginUI:     &libkb.TestLoginUI{Username: userX.Username},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)

	var wg sync.WaitGroup

	// start provisionee
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := RunEngine(eng, ctx); err != nil {
			t.Errorf("login error: %s", err)
			return
		}
	}()

	// start provisioner
	provisioner := NewKex2Provisioner(tcX.G, secretX, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx := &Context{
			SecretUI:    userX.NewSecretUI(),
			ProvisionUI: newTestProvisionUI(),
		}
		if err := RunEngine(provisioner, ctx); err != nil {
			t.Errorf("provisioner error: %s", err)
			return
		}
	}()
	secretFromY := <-secretCh
	provisioner.AddSecret(secretFromY)

	wg.Wait()

	if err := AssertProvisioned(tcY); err != nil {
		t.Fatal(err)
	}

	t.Logf(strings.Repeat("*", 100))
	t.Logf("provisioned")
	t.Logf(strings.Repeat("*", 100))

	// make sure that the provisioned device can use
	// the passphrase stream cache (use an empty secret ui)
	arg := &TrackEngineArg{
		UserAssertion: "t_alice",
		Options:       keybase1.TrackOptions{BypassConfirm: true},
	}
	ctx = &Context{
		LogUI:      tcY.G.UI.GetLogUI(),
		IdentifyUI: &FakeIdentifyUI{},
		SecretUI:   &libkb.TestSecretUI{},
	}

	teng := NewTrackEngine(arg, tcY.G)
	if err := RunEngine(teng, ctx); err != nil {
		t.Fatal(err)
	}

	// tsec isn't cached on device Y, so this should fail since the
	// secret ui doesn't know the passphrase:
	if err := signString(tcY, "sign me", &libkb.TestSecretUI{}); err == nil {
		t.Fatal("sign worked on device Y after provisioning without knowing passphrase")
	}

	// but if we know the passphrase, it should prompt for it
	// and use it
	if err := signString(tcY, "sign me", userX.NewSecretUI()); err != nil {
		t.Fatalf("sign failed on device Y with passphrase in secret ui: %s", err)
	}
}

// Provision one (physical) device with multiple users.
func TestProvisionMultipleUsers(t *testing.T) {
	// make some users with synced pgp keys
	users := make([]*FakeUser, 3)
	for i := 0; i < len(users); i++ {
		tc := SetupEngineTest(t, "login")
		users[i] = createFakeUserWithPGPOnly(t, tc)
		Logout(tc)
		tc.Cleanup()
	}

	// provision user[0] on a new device
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: users[0].Email},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    users[0].NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "", keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc)
	hasOnePaperDev(tc, users[0])
	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}

	// provision user[1] on the same device, specifying username
	ctx = &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    users[1].NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng = NewLogin(tc.G, libkb.DeviceTypeDesktop, users[1].Username, keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc)
	hasOnePaperDev(tc, users[1])
	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}

	// provision user[2] on the same device, specifying email
	ctx = &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    users[2].NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng = NewLogin(tc.G, libkb.DeviceTypeDesktop, users[2].Email, keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc)
	hasOnePaperDev(tc, users[2])
	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}

	// when you specify an email address, you are forcing provisioning
	// to happen, so make sure that it detects that the device is already
	// registered for this user.
	ctx = &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    users[2].NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng = NewLogin(tc.G, libkb.DeviceTypeDesktop, users[2].Email, keybase1.ClientType_CLI)
	if err := RunEngine(eng, ctx); err == nil {
		t.Fatal("login provision via email successful for already provisioned device/user combo")
	} else if _, ok := err.(libkb.DeviceAlreadyProvisionedError); !ok {
		t.Fatalf("err: %T, expected libkb.DeviceAlreadyProvisionedError", err)
	}
}

type testProvisionUI struct {
	secretCh               chan kex2.Secret
	method                 keybase1.ProvisionMethod
	gpgMethod              keybase1.GPGMethod
	chooseDevice           string
	verbose                bool
	calledChooseDeviceType int
	abortSwitchToGPGSign   bool
}

func newTestProvisionUI() *testProvisionUI {
	ui := &testProvisionUI{method: keybase1.ProvisionMethod_DEVICE}
	if len(os.Getenv("KB_TEST_VERBOSE")) > 0 {
		ui.verbose = true
	}
	ui.gpgMethod = keybase1.GPGMethod_GPG_IMPORT
	return ui
}

func newTestProvisionUISecretCh(ch chan kex2.Secret) *testProvisionUI {
	ui := newTestProvisionUI()
	ui.secretCh = ch
	ui.chooseDevice = "computer"
	return ui
}

func newTestProvisionUIPassphrase() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_PASSPHRASE
	return ui
}

func newTestProvisionUIChooseNoDevice() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.chooseDevice = "none"
	return ui
}

func newTestProvisionUIPaper() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_PAPER_KEY
	ui.chooseDevice = "paper key"
	return ui
}

func newTestProvisionUIGPGImport() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_GPG_IMPORT
	ui.gpgMethod = keybase1.GPGMethod_GPG_IMPORT
	return ui
}

func newTestProvisionUIGPGSign() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_GPG_SIGN
	ui.gpgMethod = keybase1.GPGMethod_GPG_SIGN
	return ui
}

func (u *testProvisionUI) printf(format string, a ...interface{}) {
	if !u.verbose {
		return
	}
	fmt.Printf("testProvisionUI: "+format+"\n", a...)
}

func (u *testProvisionUI) ChooseProvisioningMethod(_ context.Context, _ keybase1.ChooseProvisioningMethodArg) (keybase1.ProvisionMethod, error) {
	panic("ChooseProvisioningMethod deprecated")
}

func (u *testProvisionUI) ChooseGPGMethod(_ context.Context, _ keybase1.ChooseGPGMethodArg) (keybase1.GPGMethod, error) {
	u.printf("ChooseGPGMethod")
	return u.gpgMethod, nil
}

func (u *testProvisionUI) SwitchToGPGSignOK(ctx context.Context, arg keybase1.SwitchToGPGSignOKArg) (bool, error) {
	if u.abortSwitchToGPGSign {
		return false, nil
	}
	return true, nil
}

func (u *testProvisionUI) ChooseDevice(_ context.Context, arg keybase1.ChooseDeviceArg) (keybase1.DeviceID, error) {
	u.printf("ChooseDevice")
	if len(arg.Devices) == 0 {
		return "", nil
	}

	if u.chooseDevice == "none" {
		return "", nil
	}

	if len(u.chooseDevice) > 0 {
		for _, d := range arg.Devices {
			if d.Type == u.chooseDevice {
				return d.DeviceID, nil
			}
		}
	}
	return "", nil
}

func (u *testProvisionUI) ChooseDeviceType(_ context.Context, _ keybase1.ChooseDeviceTypeArg) (keybase1.DeviceType, error) {
	u.printf("ChooseDeviceType")
	u.calledChooseDeviceType++
	return keybase1.DeviceType_DESKTOP, nil
}

func (u *testProvisionUI) DisplayAndPromptSecret(_ context.Context, arg keybase1.DisplayAndPromptSecretArg) (keybase1.SecretResponse, error) {
	u.printf("DisplayAndPromptSecret")
	var ks kex2.Secret
	copy(ks[:], arg.Secret)
	u.secretCh <- ks
	var sr keybase1.SecretResponse
	return sr, nil
}

func (u *testProvisionUI) PromptNewDeviceName(_ context.Context, arg keybase1.PromptNewDeviceNameArg) (string, error) {
	u.printf("PromptNewDeviceName")
	return libkb.RandString("device", 5)
}

func (u *testProvisionUI) DisplaySecretExchanged(_ context.Context, _ int) error {
	u.printf("DisplaySecretExchanged")
	return nil
}

func (u *testProvisionUI) ProvisioneeSuccess(_ context.Context, _ keybase1.ProvisioneeSuccessArg) error {
	u.printf("ProvisioneeSuccess")
	return nil
}

func (u *testProvisionUI) ProvisionerSuccess(_ context.Context, _ keybase1.ProvisionerSuccessArg) error {
	u.printf("ProvisionerSuccess")
	return nil
}

type testProvisionDupDeviceUI struct {
	*testProvisionUI
}

// return an existing device name
func (u *testProvisionDupDeviceUI) PromptNewDeviceName(_ context.Context, arg keybase1.PromptNewDeviceNameArg) (string, error) {
	return arg.ExistingDevices[0], nil
}

type paperLoginUI struct {
	Username    string
	PaperPhrase string
}

func (p *paperLoginUI) GetEmailOrUsername(_ context.Context, _ int) (string, error) {
	return p.Username, nil
}

func (p *paperLoginUI) PromptRevokePaperKeys(_ context.Context, arg keybase1.PromptRevokePaperKeysArg) (bool, error) {
	return false, nil
}

func (p *paperLoginUI) DisplayPaperKeyPhrase(_ context.Context, arg keybase1.DisplayPaperKeyPhraseArg) error {
	return nil
}

func (p *paperLoginUI) DisplayPrimaryPaperKey(_ context.Context, arg keybase1.DisplayPrimaryPaperKeyArg) error {
	p.PaperPhrase = arg.Phrase
	return nil
}

func signString(tc libkb.TestContext, input string, secUI libkb.SecretUI) error {
	var sink bytes.Buffer

	earg := PGPSignArg{
		Sink:   libkb.NopWriteCloser{W: &sink},
		Source: ioutil.NopCloser(bytes.NewBufferString(input)),
		Opts: keybase1.PGPSignOptions{
			Mode: keybase1.SignMode_ATTACHED,
		},
	}

	eng := NewPGPSignEngine(&earg, tc.G)
	ctx := Context{
		SecretUI: secUI,
	}

	return RunEngine(eng, &ctx)
}

type testRetrySecretUI struct {
	Passphrases []string
	StoreSecret bool
	index       int
}

func (t *testRetrySecretUI) GetPassphrase(p keybase1.GUIEntryArg, terminal *keybase1.SecretEntryArg) (keybase1.GetPassphraseRes, error) {
	n := t.index
	if n >= len(t.Passphrases) {
		n = len(t.Passphrases) - 1
	}
	t.index++
	return keybase1.GetPassphraseRes{
		Passphrase:  t.Passphrases[n],
		StoreSecret: p.Features.StoreSecret.Allow && t.StoreSecret,
	}, nil
}

type gpgImportFailer struct {
	g *libkb.GlobalContext
}

func newGPGImportFailer(g *libkb.GlobalContext) *gpgImportFailer {
	return &gpgImportFailer{g: g}
}

func (g *gpgImportFailer) ImportKey(secret bool, fp libkb.PGPFingerprint) (*libkb.PGPKeyBundle, error) {
	return nil, errors.New("failed to import key")
}

func (g *gpgImportFailer) Index(secret bool, query string) (ki *libkb.GpgKeyIndex, w libkb.Warnings, err error) {
	// use real gpg for this part
	gpg := g.g.GetGpgClient()
	if err := gpg.Configure(); err != nil {
		return nil, w, err
	}
	return gpg.Index(secret, query)
}
