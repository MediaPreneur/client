// Auto-generated by avdl-compiler v1.1.1 (https://github.com/keybase/node-avdl-compiler)
//   Input file: avdl/constants.avdl

package keybase1

import (
	rpc "github.com/keybase/go-framed-msgpack-rpc"
)

type StatusCode int

const (
	StatusCode_SCOk                     StatusCode = 0
	StatusCode_SCLoginRequired          StatusCode = 201
	StatusCode_SCBadSession             StatusCode = 202
	StatusCode_SCBadLoginUserNotFound   StatusCode = 203
	StatusCode_SCBadLoginPassword       StatusCode = 204
	StatusCode_SCNotFound               StatusCode = 205
	StatusCode_SCGeneric                StatusCode = 218
	StatusCode_SCAlreadyLoggedIn        StatusCode = 235
	StatusCode_SCCanceled               StatusCode = 237
	StatusCode_SCInputCanceled          StatusCode = 239
	StatusCode_SCReloginRequired        StatusCode = 274
	StatusCode_SCResolutionFailed       StatusCode = 275
	StatusCode_SCProfileNotPublic       StatusCode = 276
	StatusCode_SCIdentifyFailed         StatusCode = 277
	StatusCode_SCTrackingBroke          StatusCode = 278
	StatusCode_SCWrongCryptoFormat      StatusCode = 279
	StatusCode_SCBadSignupUsernameTaken StatusCode = 701
	StatusCode_SCBadInvitationCode      StatusCode = 707
	StatusCode_SCMissingResult          StatusCode = 801
	StatusCode_SCKeyNotFound            StatusCode = 901
	StatusCode_SCKeyInUse               StatusCode = 907
	StatusCode_SCKeyBadGen              StatusCode = 913
	StatusCode_SCKeyNoSecret            StatusCode = 914
	StatusCode_SCKeyBadUIDs             StatusCode = 915
	StatusCode_SCKeyNoActive            StatusCode = 916
	StatusCode_SCKeyNoSig               StatusCode = 917
	StatusCode_SCKeyBadSig              StatusCode = 918
	StatusCode_SCKeyBadEldest           StatusCode = 919
	StatusCode_SCKeyNoEldest            StatusCode = 920
	StatusCode_SCKeyDuplicateUpdate     StatusCode = 921
	StatusCode_SCSibkeyAlreadyExists    StatusCode = 922
	StatusCode_SCDecryptionKeyNotFound  StatusCode = 924
	StatusCode_SCKeyNoPGPEncryption     StatusCode = 927
	StatusCode_SCKeyNoNaClEncryption    StatusCode = 928
	StatusCode_SCKeySyncedPGPNotFound   StatusCode = 929
	StatusCode_SCKeyNoMatchingGPG       StatusCode = 930
	StatusCode_SCBadTrackSession        StatusCode = 1301
	StatusCode_SCDeviceNotFound         StatusCode = 1409
	StatusCode_SCDeviceMismatch         StatusCode = 1410
	StatusCode_SCDeviceRequired         StatusCode = 1411
	StatusCode_SCDevicePrevProvisioned  StatusCode = 1413
	StatusCode_SCDeviceNoProvision      StatusCode = 1414
	StatusCode_SCStreamExists           StatusCode = 1501
	StatusCode_SCStreamNotFound         StatusCode = 1502
	StatusCode_SCStreamWrongKind        StatusCode = 1503
	StatusCode_SCStreamEOF              StatusCode = 1504
	StatusCode_SCAPINetworkError        StatusCode = 1601
	StatusCode_SCTimeout                StatusCode = 1602
	StatusCode_SCProofError             StatusCode = 1701
	StatusCode_SCIdentificationExpired  StatusCode = 1702
	StatusCode_SCSelfNotFound           StatusCode = 1703
	StatusCode_SCBadKexPhrase           StatusCode = 1704
	StatusCode_SCNoUIDelegation         StatusCode = 1705
	StatusCode_SCNoUI                   StatusCode = 1706
	StatusCode_SCInvalidVersionError    StatusCode = 1800
	StatusCode_SCOldVersionError        StatusCode = 1801
	StatusCode_SCInvalidLocationError   StatusCode = 1802
	StatusCode_SCServiceStatusError     StatusCode = 1803
	StatusCode_SCInstallError           StatusCode = 1804
)

type ConstantsInterface interface {
}

func ConstantsProtocol(i ConstantsInterface) rpc.Protocol {
	return rpc.Protocol{
		Name:    "keybase.1.constants",
		Methods: map[string]rpc.ServeHandlerDescription{},
	}
}

type ConstantsClient struct {
	Cli rpc.GenericClient
}
