// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package libkb

import (
	"os"
	"runtime"
	"time"

	keybase1 "github.com/keybase/client/go/protocol"
)

const (
	DevelServerURI      = "http://localhost:3000"
	StagingServerURI    = "https://stage0.keybase.io"
	ProductionServerURI = "https://keybase.io"
	TorServerURI        = "http://fncuwbiisyh6ak3i.onion"
)

type RunMode string

var TorProxy = "localhost:9050"

const (
	DevelRunMode      RunMode = "devel"
	StagingRunMode            = "staging"
	ProductionRunMode         = "prod"
	RunModeError              = "error"
	NoRunMode                 = ""
)

var RunModes = []RunMode{DevelRunMode, StagingRunMode, ProductionRunMode}

var ServerLookup = map[RunMode]string{
	DevelRunMode:      DevelServerURI,
	StagingRunMode:    StagingServerURI,
	ProductionRunMode: ProductionServerURI,
}

const (
	ConfigFile  = "config.json"
	SessionFile = "session.json"
	DBFile      = "keybase.leveldb"
	SocketFile  = "keybased.sock"
	PIDFile     = "keybased.pid"

	SecretKeyringTemplate = "secretkeys.%u.mpack"

	APIVersion           = "1.0"
	APIURIPathPrefix     = "/_/api/" + APIVersion
	DaemonPort           = 40933
	GoClientID           = "keybase.io go client"
	KeybaseSaltpackBrand = "KEYBASE"
)

// Right now reddit is the only site that seems to have any requirements for
// our User-Agent string. (See https://github.com/reddit/reddit/wiki/API.)If
// something else comes up, we'll want to make this more configurable.
var UserAgent = runtime.GOOS + ":" + "Keybase CLI (" + runtime.Version() + "):" + Version

const (
	PermFile          os.FileMode = 0600
	PermDir           os.FileMode = 0700
	UmaskablePermFile os.FileMode = 0666
)

const (
	UserCacheMaxAge      = 5 * time.Minute
	PGPFingerprintHexLen = 40

	ProofCacheSize             = 0x1000
	ProofCacheLongDur          = 48 * time.Hour
	ProofCacheMediumDur        = 6 * time.Hour
	ProofCacheShortDur         = 30 * time.Minute
	Identify2CacheLongTimeout  = 6 * time.Hour
	Identify2CacheShortTimeout = 1 * time.Minute
	LinkCacheSize              = 0x10000
	LinkCacheCleanDur          = 1 * time.Minute

	SigShortIDBytes  = 27
	LocalTrackMaxAge = 48 * time.Hour
)

var MerkleProdKIDs = []string{
	"010159baae6c7d43c66adf8fb7bb2b8b4cbe408c062cfc369e693ccb18f85631dbcd0a",
}
var MerkleTestKIDs = []string{
	"0101be58b6c82db64f6ccabb05088db443c69f87d5d48857d709ed6f73948dabe67d0a",
}
var MerkleStagingKIDs = []string{
	"0101bed85ce72cc315828367c28b41af585b6b7d95646a62ca829691d70f49184fa70a",
}

var CodeSigningProdKIDs = []string{
// Enable codesigning by having valid KIDs below
// "01209092ae4e790763dc7343851b977930f35b16cf43ab0ad900a2af3d3ad5cea1a10a", // Keybot (build machine)
// "0120ad6ec4c0132ca7627b3c4d72c650323abec004da51dc086fd0ec2b4f82e6e4860a", // Gabriel's Macbook
}
var CodeSigningTestKIDs = []string{}
var CodeSigningStagingKIDs = []string{}

const (
	KeybaseKIDV1       = 1 // Uses SHA-256
	KeybaseSignatureV1 = 1
	OneYearInSeconds   = 24 * 60 * 60 * 365

	SigExpireIn       = OneYearInSeconds * 16 // 16 years
	NaclEdDSAExpireIn = OneYearInSeconds * 16 // 16 years
	NaclDHExpireIn    = OneYearInSeconds * 16 // 16 years
	KeyExpireIn       = OneYearInSeconds * 16 // 16 years
	SubkeyExpireIn    = OneYearInSeconds * 16 // 16 years
	AuthExpireIn      = OneYearInSeconds      // 1 year

	PaperKeyMemoryTimeout = time.Hour
)

// Status codes.  This list should match keybase/lib/status_codes.iced.
const (
	SCOk                     = int(keybase1.StatusCode_SCOk)
	SCLoginRequired          = int(keybase1.StatusCode_SCLoginRequired)
	SCBadSession             = int(keybase1.StatusCode_SCBadSession)
	SCBadLoginUserNotFound   = int(keybase1.StatusCode_SCBadLoginUserNotFound)
	SCBadLoginPassword       = int(keybase1.StatusCode_SCBadLoginPassword)
	SCNotFound               = int(keybase1.StatusCode_SCNotFound)
	SCGeneric                = int(keybase1.StatusCode_SCGeneric)
	SCAlreadyLoggedIn        = int(keybase1.StatusCode_SCAlreadyLoggedIn)
	SCCanceled               = int(keybase1.StatusCode_SCCanceled)
	SCInputCanceled          = int(keybase1.StatusCode_SCInputCanceled)
	SCReloginRequired        = int(keybase1.StatusCode_SCReloginRequired)
	SCResolutionFailed       = int(keybase1.StatusCode_SCResolutionFailed)
	SCProfileNotPublic       = int(keybase1.StatusCode_SCProfileNotPublic)
	SCBadSignupUsernameTaken = int(keybase1.StatusCode_SCBadSignupUsernameTaken)
	SCBadInvitationCode      = int(keybase1.StatusCode_SCBadInvitationCode)
	SCMissingResult          = int(keybase1.StatusCode_SCMissingResult)
	SCKeyNotFound            = int(keybase1.StatusCode_SCKeyNotFound)
	SCKeyInUse               = int(keybase1.StatusCode_SCKeyInUse)
	SCKeyBadGen              = int(keybase1.StatusCode_SCKeyBadGen)
	SCKeyNoSecret            = int(keybase1.StatusCode_SCKeyNoSecret)
	SCKeyBadUIDs             = int(keybase1.StatusCode_SCKeyBadUIDs)
	SCKeyNoActive            = int(keybase1.StatusCode_SCKeyNoActive)
	SCKeyNoSig               = int(keybase1.StatusCode_SCKeyNoSig)
	SCKeyBadSig              = int(keybase1.StatusCode_SCKeyBadSig)
	SCKeyBadEldest           = int(keybase1.StatusCode_SCKeyBadEldest)
	SCKeyNoEldest            = int(keybase1.StatusCode_SCKeyNoEldest)
	SCKeyDuplicateUpdate     = int(keybase1.StatusCode_SCKeyDuplicateUpdate)
	SCKeySyncedPGPNotFound   = int(keybase1.StatusCode_SCKeySyncedPGPNotFound)
	SCKeyNoMatchingGPG       = int(keybase1.StatusCode_SCKeyNoMatchingGPG)
	SCSibkeyAlreadyExists    = int(keybase1.StatusCode_SCSibkeyAlreadyExists)
	SCDecryptionKeyNotFound  = int(keybase1.StatusCode_SCDecryptionKeyNotFound)
	SCBadTrackSession        = int(keybase1.StatusCode_SCBadTrackSession)
	SCDeviceNotFound         = int(keybase1.StatusCode_SCDeviceNotFound)
	SCDeviceMismatch         = int(keybase1.StatusCode_SCDeviceMismatch)
	SCDeviceRequired         = int(keybase1.StatusCode_SCDeviceRequired)
	SCDevicePrevProvisioned  = int(keybase1.StatusCode_SCDevicePrevProvisioned)
	SCDeviceNoProvision      = int(keybase1.StatusCode_SCDeviceNoProvision)
	SCStreamExists           = int(keybase1.StatusCode_SCStreamExists)
	SCStreamNotFound         = int(keybase1.StatusCode_SCStreamNotFound)
	SCStreamWrongKind        = int(keybase1.StatusCode_SCStreamWrongKind)
	SCStreamEOF              = int(keybase1.StatusCode_SCStreamEOF)
	SCAPINetworkError        = int(keybase1.StatusCode_SCAPINetworkError)
	SCTimeout                = int(keybase1.StatusCode_SCTimeout)
	SCProofError             = int(keybase1.StatusCode_SCProofError)
	SCIdentificationExpired  = int(keybase1.StatusCode_SCIdentificationExpired)
	SCSelfNotFound           = int(keybase1.StatusCode_SCSelfNotFound)
	SCBadKexPhrase           = int(keybase1.StatusCode_SCBadKexPhrase)
	SCNoUI                   = int(keybase1.StatusCode_SCNoUI)
	SCNoUIDelegation         = int(keybase1.StatusCode_SCNoUIDelegation)
	SCIdentifyFailed         = int(keybase1.StatusCode_SCIdentifyFailed)
	SCTrackingBroke          = int(keybase1.StatusCode_SCTrackingBroke)
	SCKeyNoPGPEncryption     = int(keybase1.StatusCode_SCKeyNoPGPEncryption)
	SCKeyNoNaClEncryption    = int(keybase1.StatusCode_SCKeyNoNaClEncryption)
	SCWrongCryptoFormat      = int(keybase1.StatusCode_SCWrongCryptoFormat)
)

const (
	IDSuffixKID = 0x0a
)

const (
	MerkleTreeNode = 1
	MerkleTreeLeaf = 2
)

type LinkType string
type DelegationType LinkType

const (
	AuthenticationType    LinkType = "auth"
	CryptocurrencyType             = "cryptocurrency"
	RevokeType                     = "revoke"
	TrackType                      = "track"
	UntrackType                    = "untrack"
	UpdatePassphraseType           = "update_passphrase_hash"
	WebServiceBindingType          = "web_service_binding"

	EldestType    DelegationType = "eldest"
	PGPUpdateType                = "pgp_update"
	SibkeyType                   = "sibkey"
	SubkeyType                   = "subkey"
)

const (
	SigTypeNone           = 0
	SigTypeSelfSig        = 1
	SigTypeRemoteProof    = 2
	SigTypeTrack          = 3
	SigTypeUntrack        = 4
	SigTypeRevoke         = 5
	SigTypeCryptocurrency = 6
	SigTypeAnnouncement   = 7
)

type KeyType int

const (
	KeyTypeNone                  KeyType = 0
	KeyTypeOpenPGPPublic                 = 1
	KeyTypeP3skbPrivate                  = 2
	KeyTypeKbNaclEddsa                   = 3
	KeyTypeKbNaclDH                      = 4
	KeyTypeKbNaclEddsaServerHalf         = 5
	KeyTypeKbNaclDHServerHalf            = 6
)

const (
	DeviceStatusNone    = 0
	DeviceStatusActive  = 1
	DeviceStatusDefunct = 2
)

// these strings need to match the keys in
// keybase/lib_public/public_constants.iced ->
// public_constants.device.type
const (
	DeviceTypeDesktop = "desktop"
	DeviceTypeMobile  = "mobile"
	DeviceTypePaper   = "backup"
)

const DownloadURL = "https://keybase.io/download"

var PGPVersion = "Keybase Go " + Version + " (" + runtime.GOOS + ")"

var PGPArmorHeaders = map[string]string{
	"Version": PGPVersion,
	"Comment": DownloadURL,
}

var RemoteServiceTypes = map[string]keybase1.ProofType{
	"keybase":    keybase1.ProofType_KEYBASE,
	"twitter":    keybase1.ProofType_TWITTER,
	"github":     keybase1.ProofType_GITHUB,
	"reddit":     keybase1.ProofType_REDDIT,
	"coinbase":   keybase1.ProofType_COINBASE,
	"hackernews": keybase1.ProofType_HACKERNEWS,
	"https":      keybase1.ProofType_GENERIC_WEB_SITE,
	"http":       keybase1.ProofType_GENERIC_WEB_SITE,
	"dns":        keybase1.ProofType_DNS,
	"rooter":     keybase1.ProofType_ROOTER,
}

var RemoteServiceOrder = []keybase1.ProofType{
	keybase1.ProofType_KEYBASE,
	keybase1.ProofType_TWITTER,
	keybase1.ProofType_GITHUB,
	keybase1.ProofType_REDDIT,
	keybase1.ProofType_COINBASE,
	keybase1.ProofType_HACKERNEWS,
	keybase1.ProofType_GENERIC_WEB_SITE,
	keybase1.ProofType_ROOTER,
}

const CanonicalHost = "keybase.io"

const (
	HTTPDefaultTimeout = 60 * time.Second
	HTTPPollMaximum    = 5 * time.Second
)

// Packet tags for OpenPGP and also Keybase packets
const (
	KeybasePacketV1 = 1
	TagP3skb        = 513
	TagSignature    = 514
	TagEncryption   = 515
)

const (
	KIDPGPBase    AlgoType = 0x00
	KIDPGPRsa              = 0x1
	KIDPGPElgamal          = 0x10
	KIDPGPDsa              = 0x11
	KIDPGPEcdh             = 0x12
	KIDPGPEcdsa            = 0x13
	KIDNaclEddsa           = 0x20
	KIDNaclDH              = 0x21
)

// OpenPGP hash IDs, taken from http://tools.ietf.org/html/rfc4880#section-9.4
const (
	HashPGPMd5       = 1
	HashPGPSha1      = 2
	HashPGPRipemd160 = 3
	HashPGPSha256    = 8
	HashPGPSha384    = 9
	HashPGPSha512    = 10
	HashPGPSha224    = 11
)

const (
	SigKbEddsa = KIDNaclEddsa
)

const (
	ServerUpdateLag = time.Minute
)

// key_revocation_types
const (
	RevSimpleDelete = 0
	RevFull         = 1
	RevDated        = 2
)

type KeyStatus int

const (
	KeyUncancelled KeyStatus = iota
	KeyRevoked
	KeyDeleted
	KeySuperseded
)

type KeyRole int

const (
	DLGNone KeyRole = iota
	DLGSibkey
	DLGSubkey
)

const (
	KexScryptCost       = 32768
	KexScryptR          = 8
	KexScryptP          = 1
	KexScryptKeylen     = 32
	KexSessionIDEntropy = 65 // kex doc specifies 65 bits of entropy
)

const (
	Kex2PhraseEntropy = 88
	Kex2ScryptCost    = 1 << 17
	Kex2ScryptR       = 8
	Kex2ScryptP       = 1
	Kex2ScryptKeylen  = 32
)

const (
	PaperKeyScryptCost    = 32768
	PaperKeyScryptR       = 8
	PaperKeyScryptP       = 1
	PaperKeyScryptKeylen  = 128
	PaperKeySecretEntropy = 117
	PaperKeyIDBits        = 22
	PaperKeyVersionBits   = 4
	PaperKeyVersion       = 0
)

const UserSummaryLimit = 500 // max number of user summaries in one request

const MinPassphraseLength = 12

const TrackingRateLimitSeconds = 50

type KexRole int

const (
	KexRoleProvisioner KexRole = iota
	KexRoleProvisionee
)

const (
	IdentifySourceKBFS = "kbfs"
	TestInvitationCode = "202020202020202020202020"
)

const (
	SecretPromptCancelDuration = 5 * time.Minute
)

const ServiceLogFileName = "keybase.service.log"
const KBFSLogFileName = "keybase.kbfs.log"
const DesktopLogFileName = "Keybase.app.log"

const (
	PGPAssertionKey = "pgp"
)

type AppStartMode string

const (
	AppStartModeDisabled AppStartMode = ""
	AppStartModeService  AppStartMode = "service" // Open app after service start
)
