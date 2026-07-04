package client

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
)

// GenerateKeypair mints a fresh architect keypair for the toolkit, the public
// half to register with a brain and the private half for the architect to keep
// and sign with. The brain never mints a key: generation is the architect's,
// done here in the architect's own instrument, and only the public key is ever
// handed to GOLEM. Both halves are base64.
func GenerateKeypair() (pub, priv string, err error) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pk),
		base64.StdEncoding.EncodeToString(sk), nil
}
