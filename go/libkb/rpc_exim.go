// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

// Export-Import for RPC stubs

package libkb

import (
	"errors"
	"io"
	"sort"
	"strings"

	keybase1 "github.com/keybase/client/go/protocol"
	"github.com/keybase/go-crypto/openpgp"
	pgpErrors "github.com/keybase/go-crypto/openpgp/errors"
)

func (sh SigHint) Export() *keybase1.SigHint {
	return &keybase1.SigHint{
		RemoteId:  sh.remoteID,
		ApiUrl:    sh.apiURL,
		HumanUrl:  sh.humanURL,
		CheckText: sh.checkText,
	}
}

func (l LinkCheckResult) ExportToIdentifyRow(i int) keybase1.IdentifyRow {
	return keybase1.IdentifyRow{
		RowId:     i,
		Proof:     ExportRemoteProof(l.link),
		TrackDiff: ExportTrackDiff(l.diff),
	}
}

func (l LinkCheckResult) Export() keybase1.LinkCheckResult {
	ret := keybase1.LinkCheckResult{
		ProofId:       l.position,
		ProofResult:   ExportProofError(l.err),
		SnoozedResult: ExportProofError(l.snoozedErr),
		TorWarning:    l.torWarning,
	}
	if l.cached != nil {
		ret.Cached = l.cached.Export()
	}
	bt := false
	if l.diff != nil {
		ret.Diff = ExportTrackDiff(l.diff)
		if l.diff.BreaksTracking() {
			bt = true
		}
	}
	if l.remoteDiff != nil {
		ret.RemoteDiff = ExportTrackDiff(l.remoteDiff)
		if l.diff.BreaksTracking() {
			bt = true
		}
	}
	if l.hint != nil {
		ret.Hint = l.hint.Export()
	}
	ret.TmpTrackExpireTime = keybase1.ToTime(l.tmpTrackExpireTime)
	ret.BreaksTracking = bt
	return ret
}

func (cr CheckResult) Export() *keybase1.CheckResult {
	return &keybase1.CheckResult{
		ProofResult: ExportProofError(cr.Status),
		Time:        keybase1.ToTime(cr.Time),
		Freshness:   cr.Freshness(),
	}
}

func ExportRemoteProof(p RemoteProofChainLink) keybase1.RemoteProof {
	k, v := p.ToKeyValuePair()
	return keybase1.RemoteProof{
		ProofType:     p.GetProofType(),
		Key:           k,
		Value:         v,
		DisplayMarkup: v,
		SigID:         p.GetSigID(),
		MTime:         keybase1.ToTime(p.GetCTime()),
	}
}

func (is IdentifyState) ExportToUncheckedIdentity() *keybase1.Identity {
	return is.res.ExportToUncheckedIdentity()
}

func (ir IdentifyOutcome) ExportToUncheckedIdentity() *keybase1.Identity {
	tmp := keybase1.Identity{
		Status: ExportErrorAsStatus(ir.Error),
	}
	if ir.TrackUsed != nil {
		tmp.WhenLastTracked = keybase1.ToTime(ir.TrackUsed.GetCTime())
	}

	pc := ir.ProofChecksSorted()
	tmp.Proofs = make([]keybase1.IdentifyRow, len(pc))
	for j, p := range pc {
		tmp.Proofs[j] = p.ExportToIdentifyRow(j)
	}

	tmp.Revoked = make([]keybase1.TrackDiff, len(ir.Revoked))
	for j, d := range ir.Revoked {
		// Should have all non-nil elements...
		tmp.Revoked[j] = *ExportTrackDiff(d)
		tmp.BreaksTracking = true
	}
	return &tmp
}

type ExportableError interface {
	error
	ToStatus() keybase1.Status
}

func ExportProofError(pe ProofError) (ret keybase1.ProofResult) {
	if pe == nil {
		ret.State = keybase1.ProofState_OK
		ret.Status = keybase1.ProofStatus_OK
	} else {
		ret.Status = pe.GetProofStatus()
		ret.State = ProofErrorToState(pe)
		ret.Desc = pe.GetDesc()
	}
	return
}

func ImportProofError(e keybase1.ProofResult) ProofError {
	ps := keybase1.ProofStatus(e.Status)
	if ps == keybase1.ProofStatus_OK {
		return nil
	}
	return NewProofError(ps, e.Desc)
}

func ExportErrorAsStatus(e error) (ret *keybase1.Status) {
	if e == nil {
		return nil
	}

	if e == io.EOF {
		return &keybase1.Status{
			Code: SCStreamEOF,
			Name: "STREAM_EOF",
		}
	}

	if e == pgpErrors.ErrKeyIncorrect {
		return &keybase1.Status{
			Code: SCKeyNoActive,
			Name: "SC_KEY_NO_ACTIVE",
			Desc: "No PGP key found",
		}
	}

	if ee, ok := e.(ExportableError); ok {
		tmp := ee.ToStatus()
		return &tmp
	}

	if G.Env.GetRunMode() != ProductionRunMode {
		G.Log.Warning("not exportable error: %v (%T)", e, e)
	}

	return &keybase1.Status{
		Name: "GENERIC",
		Code: SCGeneric,
		Desc: e.Error(),
	}
}

//=============================================================================

func WrapError(e error) interface{} {
	return ExportErrorAsStatus(e)
}

type ErrorUnwrapper struct{}

func (eu ErrorUnwrapper) MakeArg() interface{} {
	return &keybase1.Status{}
}

func (eu ErrorUnwrapper) UnwrapError(arg interface{}) (appError error, dispatchError error) {
	targ, ok := arg.(*keybase1.Status)
	if !ok {
		dispatchError = errors.New("Error converting status to keybase1.Status object")
		return
	}
	appError = ImportStatusAsError(targ)
	return
}

//=============================================================================

func ImportStatusAsError(s *keybase1.Status) error {
	if s == nil {
		return nil
	}
	switch s.Code {
	case SCOk:
		return nil
	case SCGeneric:
		return errors.New(s.Desc)
	case SCBadLoginPassword:
		return PassphraseError{s.Desc}
	case SCKeyBadGen:
		return KeyGenError{s.Desc}
	case SCAlreadyLoggedIn:
		return LoggedInError{}
	case SCCanceled:
		return CanceledError{s.Desc}
	case SCInputCanceled:
		return InputCanceledError{}
	case SCKeyNoSecret:
		return NoSecretKeyError{}
	case SCLoginRequired:
		return LoginRequiredError{s.Desc}
	case SCKeyInUse:
		var fp *PGPFingerprint
		if len(s.Desc) > 0 {
			fp, _ = PGPFingerprintFromHex(s.Desc)
		}
		return KeyExistsError{fp}
	case SCStreamExists:
		return StreamExistsError{}
	case SCBadInvitationCode:
		return BadInvitationCodeError{}
	case SCStreamNotFound:
		return StreamNotFoundError{}
	case SCStreamWrongKind:
		return StreamWrongKindError{}
	case SCStreamEOF:
		return io.EOF
	case SCSelfNotFound:
		return SelfNotFoundError{msg: s.Desc}
	case SCDeviceNotFound:
		return NoDeviceError{Reason: s.Desc}
	case SCDecryptionKeyNotFound:
		return NoDecryptionKeyError{Msg: s.Desc}
	case SCTimeout:
		return TimeoutError{}
	case SCDeviceMismatch:
		return ReceiverDeviceError{Msg: s.Desc}
	case SCBadKexPhrase:
		return InvalidKexPhraseError{}
	case SCReloginRequired:
		return ReloginRequiredError{}
	case SCDeviceRequired:
		return DeviceRequiredError{}
	case SCMissingResult:
		return IdentifyDidNotCompleteError{}
	case SCSibkeyAlreadyExists:
		return SibkeyAlreadyExistsError{}
	case SCNoUIDelegation:
		return UIDelegationUnavailableError{}
	case SCNoUI:
		return NoUIError{Which: s.Desc}
	case SCProfileNotPublic:
		return ProfileNotPublicError{msg: s.Desc}
	case SCIdentifyFailed:
		var assertion string
		if len(s.Fields) > 0 && s.Fields[0].Key == "assertion" {
			assertion = s.Fields[0].Value
		}
		return IdentifyFailedError{Assertion: assertion, Reason: s.Desc}
	case SCTrackingBroke:
		return TrackingBrokeError{}
	case SCResolutionFailed:
		var input string
		if len(s.Fields) > 0 && s.Fields[0].Key == "input" {
			input = s.Fields[0].Value
		}
		return ResolutionError{Msg: s.Desc, Input: input}
	case SCKeyNoPGPEncryption:
		ret := NoPGPEncryptionKeyError{User: s.Desc}
		for _, field := range s.Fields {
			switch field.Key {
			case "device":
				ret.HasDeviceKey = true
			}
		}
		return ret
	case SCKeyNoNaClEncryption:
		ret := NoNaClEncryptionKeyError{User: s.Desc}
		for _, field := range s.Fields {
			switch field.Key {
			case "pgp":
				ret.HasPGPKey = true
			}
		}
		return ret
	case SCWrongCryptoFormat:
		ret := WrongCryptoFormatError{Operation: s.Desc}
		for _, field := range s.Fields {
			switch field.Key {
			case "wanted":
				ret.Wanted = CryptoMessageFormat(field.Value)
			case "received":
				ret.Received = CryptoMessageFormat(field.Value)
			}
		}
		return ret
	case SCKeySyncedPGPNotFound:
		return NoSyncedPGPKeyError{}
	case SCKeyNoMatchingGPG:
		ret := NoMatchingGPGKeysError{}
		for _, field := range s.Fields {
			switch field.Key {
			case "fingerprints":
				ret.Fingerprints = strings.Split(field.Value, ",")
			case "has_active_device":
				ret.HasActiveDevice = true
			}
		}
		return ret
	case SCDevicePrevProvisioned:
		return DeviceAlreadyProvisionedError{}
	case SCDeviceNoProvision:
		return ProvisionUnavailableError{}
	default:
		ase := AppStatusError{
			Code:   s.Code,
			Name:   s.Name,
			Desc:   s.Desc,
			Fields: make(map[string]string),
		}
		for _, f := range s.Fields {
			ase.Fields[f.Key] = f.Value
		}
		return ase
	}
}

//=============================================================================

func (a AppStatusError) ToStatus() keybase1.Status {
	var fields []keybase1.StringKVPair
	for k, v := range a.Fields {
		fields = append(fields, keybase1.StringKVPair{Key: k, Value: v})
	}

	return keybase1.Status{
		Code:   a.Code,
		Name:   a.Name,
		Desc:   a.Desc,
		Fields: fields,
	}
}

//=============================================================================

func ExportTrackDiff(d TrackDiff) (res *keybase1.TrackDiff) {
	if d != nil {
		res = &keybase1.TrackDiff{
			Type:          keybase1.TrackDiffType(d.GetTrackDiffType()),
			DisplayMarkup: d.ToDisplayString(),
		}
	}
	return
}

//=============================================================================

func ImportPGPFingerprintSlice(fp []byte) (ret *PGPFingerprint) {
	if fp == nil {
		return nil
	}
	if len(fp) != PGPFingerprintLen {
		return nil
	}

	var tmp PGPFingerprint
	copy(tmp[:], fp)
	return &tmp
}

//=============================================================================

func (s TrackSummary) Export(username string) (ret keybase1.TrackSummary) {
	ret.Time = keybase1.ToTime(s.time)
	ret.IsRemote = s.isRemote
	ret.Username = username
	return
}

func ImportTrackSummary(s *keybase1.TrackSummary) *TrackSummary {
	if s == nil {
		return nil
	}

	ret := &TrackSummary{
		time:     keybase1.FromTime(s.Time),
		isRemote: s.IsRemote,
		username: s.Username,
	}
	return ret
}

func ExportTrackSummary(l *TrackLookup, username string) *keybase1.TrackSummary {
	if l == nil {
		return nil
	}

	tmp := l.ToSummary().Export(username)
	return &tmp
}

//=============================================================================

func (ir *IdentifyOutcome) Export() *keybase1.IdentifyOutcome {
	v := make([]string, len(ir.Warnings))
	for i, w := range ir.Warnings {
		v[i] = w.Warning()
	}
	del := make([]keybase1.TrackDiff, len(ir.Revoked))
	for i, d := range ir.Revoked {
		del[i] = *ExportTrackDiff(d)
	}
	ret := &keybase1.IdentifyOutcome{
		Username:          ir.Username,
		Status:            ExportErrorAsStatus(ir.Error),
		Warnings:          v,
		TrackUsed:         ExportTrackSummary(ir.TrackUsed, ir.Username),
		TrackStatus:       ir.TrackStatus(),
		NumTrackFailures:  ir.NumTrackFailures(),
		NumTrackChanges:   ir.NumTrackChanges(),
		NumProofFailures:  ir.NumProofFailures(),
		NumRevoked:        ir.NumRevoked(),
		NumProofSuccesses: ir.NumProofSuccesses(),
		Revoked:           del,
		TrackOptions:      ir.TrackOptions,
		Reason:            ir.Reason,
	}
	return ret
}

//=============================================================================

func DisplayTrackArg(sessionID int, stmt string) *keybase1.DisplayTrackStatementArg {
	return &keybase1.DisplayTrackStatementArg{
		SessionID: sessionID,
		Stmt:      stmt,
	}
}

//=============================================================================

func ImportWarnings(v []string) Warnings {
	w := make([]Warning, len(v))
	for i, s := range v {
		w[i] = StringWarning(s)
	}
	return Warnings{w}
}

//=============================================================================

func (c CryptocurrencyChainLink) Export() (ret keybase1.Cryptocurrency) {
	ret.Pkhash = c.pkhash
	ret.Address = c.address
	return
}

//=============================================================================

func (c CurrentStatus) Export() (ret keybase1.GetCurrentStatusRes) {
	ret.Configured = c.Configured
	ret.Registered = c.Registered
	ret.LoggedIn = c.LoggedIn
	ret.SessionIsValid = c.SessionIsValid
	if c.User != nil {
		ret.User = c.User.Export()
	}
	// ret.ServerUri = G.Env.GetServerUri();
	return
}

//=============================================================================

func (p PassphraseError) ToStatus() (s keybase1.Status) {
	s.Code = SCBadLoginPassword
	s.Name = "BAD_LOGIN_PASSWORD"
	s.Desc = p.Msg
	return
}

func (m Markup) Export() (ret keybase1.Text) {
	ret.Data = m.data
	ret.Markup = true
	return
}

//=============================================================================

func (e LoggedInError) ToStatus() (s keybase1.Status) {
	s.Code = SCAlreadyLoggedIn
	s.Name = "ALREADY_LOGGED_IN"
	s.Desc = "Already logged in as a different user"
	return
}

//=============================================================================

func (e LoggedInWrongUserError) ToStatus() (s keybase1.Status) {
	s.Code = SCAlreadyLoggedIn
	s.Name = "ALREADY_LOGGED_IN"
	s.Desc = e.Error()
	return
}

//=============================================================================

func (e KeyGenError) ToStatus() (s keybase1.Status) {
	s.Code = SCKeyBadGen
	s.Name = "KEY_BAD_GEN"
	s.Desc = e.Msg
	return
}

//=============================================================================

func (c CanceledError) ToStatus() (s keybase1.Status) {
	s.Code = SCCanceled
	s.Name = "CANCELED"
	s.Desc = c.M
	return
}

//=============================================================================

func (e InputCanceledError) ToStatus() (s keybase1.Status) {
	s.Code = SCInputCanceled
	s.Name = "CANCELED"
	s.Desc = "Input canceled"
	return
}

//=============================================================================

func (e SkipSecretPromptError) ToStatus() (s keybase1.Status) {
	s.Code = SCInputCanceled
	s.Name = "CANCELED"
	s.Desc = "Input canceled due to skip secret prompt"
	return
}

//=============================================================================

func (c KeyExistsError) ToStatus() (s keybase1.Status) {
	s.Code = SCKeyInUse
	s.Name = "KEY_IN_USE"
	if c.Key != nil {
		s.Desc = c.Key.String()
	}
	return
}

//=============================================================================

func (c NoActiveKeyError) ToStatus() (s keybase1.Status) {
	s.Code = SCKeyNoActive
	s.Name = "KEY_NO_ACTIVE"
	s.Desc = c.Error()
	return
}

//=============================================================================

func (ids Identities) Export() (res []keybase1.PGPIdentity) {
	var n int
	if ids == nil {
		n = 0
	} else {
		n = len(ids)
	}
	res = make([]keybase1.PGPIdentity, n)
	for i, id := range ids {
		res[i] = id.Export()
	}
	return
}

func ImportPGPIdentities(ids []keybase1.PGPIdentity) (ret Identities) {
	ret = Identities(make([]Identity, len(ids)))
	for i, id := range ids {
		ret[i] = ImportPGPIdentity(id)
	}
	return
}

//=============================================================================

func (id Identity) Export() (ret keybase1.PGPIdentity) {
	ret.Username = id.Username
	ret.Email = id.Email
	ret.Comment = id.Comment
	return
}

func ImportPGPIdentity(arg keybase1.PGPIdentity) (ret Identity) {
	ret.Username = arg.Username
	ret.Email = arg.Email
	ret.Comment = arg.Comment
	return
}

//=============================================================================

// Interface for sorting a list of PublicKeys

type PublicKeyList []keybase1.PublicKey

func (l PublicKeyList) Len() int { return len(l) }
func (l PublicKeyList) Less(i, j int) bool {
	// Keys created first come first.
	if l[i].CTime != l[j].CTime {
		return l[i].CTime < l[j].CTime
	}
	// For keys created at the same time, if one of them's the eldest key, it comes first.
	if l[i].IsEldest != l[j].IsEldest {
		return l[i].IsEldest
	}
	// Otherwise just sort by KID.
	return l[i].KID < l[j].KID
}
func (l PublicKeyList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func ExportPGPIdentity(identity *openpgp.Identity) keybase1.PGPIdentity {
	if identity == nil || identity.UserId == nil {
		return keybase1.PGPIdentity{}
	}
	return keybase1.PGPIdentity{
		Username: identity.UserId.Name,
		Email:    identity.UserId.Email,
		Comment:  identity.UserId.Comment,
	}
}

func (bundle *PGPKeyBundle) Export() keybase1.PublicKey {
	kid := bundle.GetKID()
	fingerprintStr := ""
	identities := []keybase1.PGPIdentity{}
	fingerprintStr = bundle.GetFingerprint().String()
	for _, identity := range bundle.Identities {
		identities = append(identities, ExportPGPIdentity(identity))
	}
	return keybase1.PublicKey{
		KID:            kid,
		PGPFingerprint: fingerprintStr,
		PGPIdentities:  identities,
	}
}

func (ckf ComputedKeyFamily) exportPublicKey(key GenericKey) (pk keybase1.PublicKey) {
	pk.KID = key.GetKID()
	if pgpBundle, isPGP := key.(*PGPKeyBundle); isPGP {
		pk.PGPFingerprint = pgpBundle.GetFingerprint().String()
		ids := make([]keybase1.PGPIdentity, len(pgpBundle.Identities))
		i := 0
		for _, identity := range pgpBundle.Identities {
			ids[i] = ExportPGPIdentity(identity)
			i++
		}
		pk.PGPIdentities = ids
	}
	pk.DeviceID = ckf.cki.KIDToDeviceID[pk.KID]
	device := ckf.cki.Devices[pk.DeviceID]
	if device != nil {
		if device.Description != nil {
			pk.DeviceDescription = *device.Description
		}
		pk.DeviceType = device.Type
	}
	cki, ok := ckf.cki.Infos[pk.KID]
	if ok && cki != nil {
		if cki.Parent.IsValid() {
			pk.ParentID = cki.Parent.String()
		}
		pk.IsSibkey = cki.Sibkey
		pk.IsEldest = cki.Eldest
		pk.CTime = keybase1.TimeFromSeconds(cki.CTime)
		pk.ETime = keybase1.TimeFromSeconds(cki.ETime)
	}
	return pk
}

// Export is used by IDRes.  It includes PGP keys.
func (ckf ComputedKeyFamily) Export() []keybase1.PublicKey {
	var exportedKeys []keybase1.PublicKey
	for _, key := range ckf.GetAllActiveSibkeys() {
		exportedKeys = append(exportedKeys, ckf.exportPublicKey(key))
	}
	for _, key := range ckf.GetAllActiveSubkeys() {
		exportedKeys = append(exportedKeys, ckf.exportPublicKey(key))
	}
	sort.Sort(PublicKeyList(exportedKeys))
	return exportedKeys
}

// ExportDeviceKeys is used by LoadUserPlusKeys.  The key list
// only contains device keys.  It also returns the number of PGP
// keys in the key family.
func (ckf ComputedKeyFamily) ExportDeviceKeys() (exportedKeys []keybase1.PublicKey, pgpKeyCount int) {
	for _, key := range ckf.GetAllActiveSibkeys() {
		if _, isPGP := key.(*PGPKeyBundle); isPGP {
			pgpKeyCount++
			continue
		}
		exportedKeys = append(exportedKeys, ckf.exportPublicKey(key))
	}
	for _, key := range ckf.GetAllActiveSubkeys() {
		if _, isPGP := key.(*PGPKeyBundle); isPGP {
			pgpKeyCount++
			continue
		}
		exportedKeys = append(exportedKeys, ckf.exportPublicKey(key))
	}
	sort.Sort(PublicKeyList(exportedKeys))
	return exportedKeys, pgpKeyCount
}

func (ckf ComputedKeyFamily) ExportRevokedDeviceKeys() []keybase1.RevokedKey {
	var ex []keybase1.RevokedKey
	for _, key := range ckf.GetRevokedKeys() {
		if _, isPGP := key.Key.(*PGPKeyBundle); isPGP {
			continue
		}
		rkey := keybase1.RevokedKey{
			Key: ckf.exportPublicKey(key.Key),
			Time: keybase1.KeybaseTime{
				Unix:  keybase1.TimeFromSeconds(key.RevokedAt.Unix),
				Chain: key.RevokedAt.Chain,
			},
		}
		ex = append(ex, rkey)
	}

	return ex
}

func (u *User) Export() *keybase1.User {
	return &keybase1.User{
		Uid:      u.GetUID(),
		Username: u.GetName(),
	}
}

func (u *User) ExportToVersionVector(idTime keybase1.Time) keybase1.UserVersionVector {
	idv, _ := u.GetIDVersion()
	return keybase1.UserVersionVector{
		Id:               idv,
		SigHints:         u.GetSigHintsVersion(),
		SigChain:         int64(u.GetSigChainLastKnownSeqno()),
		LastIdentifiedAt: idTime,
	}
}

func (u *User) ExportToUserPlusKeys(idTime keybase1.Time) keybase1.UserPlusKeys {
	ret := keybase1.UserPlusKeys{
		Uid:      u.GetUID(),
		Username: u.GetName(),
	}
	ckf := u.GetComputedKeyFamily()
	if ckf != nil {
		ret.DeviceKeys, ret.PGPKeyCount = ckf.ExportDeviceKeys()
		ret.RevokedDeviceKeys = ckf.ExportRevokedDeviceKeys()
	}

	ret.Uvv = u.ExportToVersionVector(idTime)
	return ret
}

//=============================================================================

func (a PGPGenArg) ExportTo(ret *keybase1.PGPKeyGenArg) {
	ret.PrimaryBits = a.PrimaryBits
	ret.SubkeyBits = a.SubkeyBits
	ret.CreateUids = keybase1.PGPCreateUids{Ids: a.Ids.Export()}
	return
}

//=============================================================================

func ImportKeyGenArg(a keybase1.PGPKeyGenArg) (ret PGPGenArg) {
	ret.PrimaryBits = a.PrimaryBits
	ret.SubkeyBits = a.SubkeyBits
	ret.Ids = ImportPGPIdentities(a.CreateUids.Ids)
	return
}

//=============================================================================

func (t Tracker) Export() keybase1.Tracker { return keybase1.Tracker(t) }

//=============================================================================

func (e BadInvitationCodeError) ToStatus(s keybase1.Status) {
	s.Code = SCBadInvitationCode
	s.Name = "BAD_INVITATION_CODE"
	return
}

//=============================================================================

func (e StreamExistsError) ToStatus(s keybase1.Status) {
	s.Code = SCStreamExists
	s.Name = "STREAM_EXISTS"
	return
}

func (e StreamNotFoundError) ToStatus(s keybase1.Status) {
	s.Code = SCStreamNotFound
	s.Name = "SC_STREAM_NOT_FOUND"
	return
}

func (e StreamWrongKindError) ToStatus(s keybase1.Status) {
	s.Code = SCStreamWrongKind
	s.Name = "STREAM_WRONG_KIND"
	return
}

//=============================================================================

func (u NoSecretKeyError) ToStatus() (s keybase1.Status) {
	s.Code = SCKeyNoSecret
	s.Name = "KEY_NO_SECRET"
	return
}

//=============================================================================

func (u LoginRequiredError) ToStatus() (s keybase1.Status) {
	s.Code = SCLoginRequired
	s.Name = "LOGIN_REQUIRED"
	s.Desc = u.Context
	return
}

//=============================================================================

func (e APINetError) ToStatus() (s keybase1.Status) {
	s.Code = SCAPINetworkError
	s.Name = "API_NETWORK_ERROR"
	s.Desc = e.Error()
	return
}

func (e ProofNotFoundForServiceError) ToStatus() (s keybase1.Status) {
	s.Code = SCProofError
	s.Name = "PROOF_ERROR"
	s.Desc = e.Error()
	return
}

func (e ProofNotFoundForUsernameError) ToStatus() (s keybase1.Status) {
	s.Code = SCProofError
	s.Name = "PROOF_ERROR"
	s.Desc = e.Error()
	return
}

func (e NoDecryptionKeyError) ToStatus() (s keybase1.Status) {
	s.Code = SCDecryptionKeyNotFound
	s.Name = "KEY_NOT_FOUND_DECRYPTION"
	s.Desc = e.Msg
	return
}

func (e NoKeyError) ToStatus() (s keybase1.Status) {
	s.Code = SCKeyNotFound
	s.Name = "KEY_NOT_FOUND"
	s.Desc = e.Msg
	return
}

func (e NoSyncedPGPKeyError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCKeySyncedPGPNotFound,
		Name: "KEY_NOT_FOUND_SYNCED_PGP",
		Desc: e.Error(),
	}
}

func (e IdentifyTimeoutError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCIdentificationExpired,
		Name: "IDENTIFICATION_EXPIRED",
		Desc: e.Error(),
	}
}

func (e SelfNotFoundError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCSelfNotFound,
		Name: "SELF_NOT_FOUND",
		Desc: e.Error(),
	}
}

func (e NoDeviceError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCDeviceNotFound,
		Name: "DEVICE_NOT_FOUND",
		Desc: e.Reason,
	}
}

func (e TimeoutError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCTimeout,
		Name: "SC_TIMEOUT",
		Desc: e.Error(),
	}
}

func (e ReceiverDeviceError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCDeviceMismatch,
		Name: "SC_DEVICE_MISMATCH",
		Desc: e.Error(),
	}
}

func (e InvalidKexPhraseError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCBadKexPhrase,
		Name: "SC_BAD_KEX_PHRASE",
		Desc: e.Error(),
	}
}

func (e ReloginRequiredError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCReloginRequired,
		Name: "SC_RELOGIN_REQUIRED",
		Desc: e.Error(),
	}
}

func (e DeviceRequiredError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCDeviceRequired,
		Name: "SC_DEVICE_REQUIRED",
		Desc: e.Error(),
	}
}

func (e IdentifyDidNotCompleteError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCMissingResult,
		Name: "SC_MISSING_RESULT",
		Desc: e.Error(),
	}
}

func (e SibkeyAlreadyExistsError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCSibkeyAlreadyExists,
		Name: "SC_SIBKEY_ALREADY_EXISTS",
		Desc: e.Error(),
	}
}

func (e UIDelegationUnavailableError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCNoUIDelegation,
		Name: "SC_UI_DELEGATION_UNAVAILABLE",
		Desc: e.Error(),
	}
}

func (e NoUIError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCNoUI,
		Name: "SC_NO_UI",
		Desc: e.Which,
	}
}

func (e ResolutionError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCResolutionFailed,
		Name: "SC_RESOLUTION_FAILED",
		Desc: e.Msg,
		Fields: []keybase1.StringKVPair{
			{"input", e.Input},
		},
	}
}

func (e IdentifyFailedError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCIdentifyFailed,
		Name: "SC_IDENTIFY_FAILED",
		Desc: e.Reason,
		Fields: []keybase1.StringKVPair{
			{"assertion", e.Assertion},
		},
	}
}

func (e ProfileNotPublicError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCProfileNotPublic,
		Name: "SC_PROFILE_NOT_PUBLIC",
		Desc: e.msg,
	}
}

func (e TrackingBrokeError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCTrackingBroke,
		Name: "SC_TRACKING_BROKE",
	}
}

func (e NoPGPEncryptionKeyError) ToStatus() keybase1.Status {
	ret := keybase1.Status{
		Code: SCKeyNoPGPEncryption,
		Name: "SC_KEY_NO_PGP_ENCRYPTION",
		Desc: e.User,
	}
	if e.HasDeviceKey {
		ret.Fields = []keybase1.StringKVPair{
			{"device", "1"},
		}
	}
	return ret
}

func (e NoNaClEncryptionKeyError) ToStatus() keybase1.Status {
	ret := keybase1.Status{
		Code: SCKeyNoNaClEncryption,
		Name: "SC_KEY_NO_NACL_ENCRYPTION",
		Desc: e.User,
	}
	if e.HasPGPKey {
		ret.Fields = []keybase1.StringKVPair{
			{"pgp", "1"},
		}
	}
	return ret
}

func (e WrongCryptoFormatError) ToStatus() keybase1.Status {
	ret := keybase1.Status{
		Code: SCWrongCryptoFormat,
		Name: "SC_WRONG_CRYPTO_FORMAT",
		Desc: e.Operation,
		Fields: []keybase1.StringKVPair{
			{"wanted", string(e.Wanted)},
			{"received", string(e.Received)},
		},
	}
	return ret
}

func (e NoMatchingGPGKeysError) ToStatus() keybase1.Status {
	s := keybase1.Status{
		Code: SCKeyNoMatchingGPG,
		Name: "SC_KEY_NO_MATCHING_GPG",
		Fields: []keybase1.StringKVPair{
			{"fingerprints", strings.Join(e.Fingerprints, ",")},
		},
	}
	if e.HasActiveDevice {
		s.Fields = append(s.Fields, keybase1.StringKVPair{Key: "has_active_device", Value: "1"})
	}
	return s
}

func (e DeviceAlreadyProvisionedError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCDevicePrevProvisioned,
		Name: "SC_DEVICE_PREV_PROVISIONED",
	}
}

func (e ProvisionUnavailableError) ToStatus() keybase1.Status {
	return keybase1.Status{
		Code: SCDeviceNoProvision,
		Name: "SC_DEVICE_NO_PROVISION",
	}
}
