// Copyright 2015 Keybase, Inc. All rights reserved. Use of
// this source code is governed by the included BSD license.

package libkb

import (
	"bytes"
	"crypto/hmac"
	"io"

	"github.com/keybase/saltpack"
)

func SaltpackVerify(g *GlobalContext, source io.Reader, sink io.WriteCloser, checkSender func(saltpack.SigningPublicKey) error) error {
	sc, newSource, err := ClassifyStream(source)
	if err != nil {
		return err
	}
	if sc.Format != CryptoMessageFormatSaltpack {
		return WrongCryptoFormatError{
			Wanted:    CryptoMessageFormatSaltpack,
			Received:  sc.Format,
			Operation: "verify",
		}
	}
	source = newSource

	kr := echoKeyring{Contextified: NewContextified(g)}

	var skey saltpack.SigningPublicKey
	var vs io.Reader
	var frame saltpack.Frame
	if sc.Armored {
		skey, vs, frame, err = saltpack.NewDearmor62VerifyStream(source, kr)
	} else {
		skey, vs, err = saltpack.NewVerifyStream(source, kr)
	}
	if err != nil {
		g.Log.Debug("saltpack.NewDearmor62VerifyStream error: %s", err)
		return err
	}

	if checkSender != nil {
		if err = checkSender(skey); err != nil {
			return err
		}
	}

	n, err := io.Copy(sink, vs)
	if err != nil {
		return err
	}

	if sc.Armored {
		if brand, err := saltpack.CheckArmor62Frame(frame, saltpack.MessageTypeAttachedSignature); err != nil {
			return err
		} else if err = checkSaltpackBrand(brand); err != nil {
			return err
		}
	}

	g.Log.Debug("Verify: read %d bytes", n)

	if err := sink.Close(); err != nil {
		return err
	}

	return nil
}

func SaltpackVerifyDetached(g *GlobalContext, message io.Reader, signature []byte, checkSender func(saltpack.SigningPublicKey) error) error {
	sc, _, err := ClassifyStream(bytes.NewReader(signature))
	if err != nil {
		return err
	}
	if sc.Format != CryptoMessageFormatSaltpack {
		return WrongCryptoFormatError{
			Wanted:    CryptoMessageFormatSaltpack,
			Received:  sc.Format,
			Operation: "verify detached",
		}
	}

	kr := echoKeyring{Contextified: NewContextified(g)}

	var skey saltpack.SigningPublicKey
	if sc.Armored {
		var brand string
		skey, brand, err = saltpack.Dearmor62VerifyDetachedReader(message, string(signature), kr)
		if err != nil {
			g.Log.Debug("saltpack.Dearmor62VerifyDetachedReader error: %s", err)
			return err
		}
		if err = checkSaltpackBrand(brand); err != nil {
			return err
		}
	} else {
		skey, err = saltpack.VerifyDetachedReader(message, signature, kr)
		if err != nil {
			g.Log.Debug("saltpack.VerifyDetachedReader error: %s", err)
			return err
		}
	}

	if checkSender != nil {
		if err = checkSender(skey); err != nil {
			return err
		}
	}

	return nil
}

type echoKeyring struct {
	Contextified
}

func (e echoKeyring) LookupSigningPublicKey(kid []byte) saltpack.SigningPublicKey {
	var k NaclSigningKeyPublic
	copy(k[:], kid)
	return saltSignerPublic{key: k}
}

type sigKeyring struct {
	saltSigner
}

func (s sigKeyring) LookupSigningPublicKey(kid []byte) saltpack.SigningPublicKey {
	if s.GetPublicKey() == nil {
		return nil
	}

	if hmac.Equal(s.GetPublicKey().ToKID(), kid) {
		return s.GetPublicKey()
	}

	return nil
}
