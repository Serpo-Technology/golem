// Package authkey is the daemon's proof of who is speaking to it. The architect
// holds a private key and signs the requests that carry authority; the daemon
// holds only the matching public key and verifies the signature. Nothing the
// daemon stores can impersonate the architect, and no network party can act as
// the architect without the private key.
//
// The primitive is Ed25519 from the standard library: a 32-byte public key, a
// 64-byte signature, deterministic and fast, with no external dependency. Keys
// travel and rest as base64 text so they fit a config field or an environment
// variable.
package authkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

// Generate makes a fresh architect keypair, returning both halves base64
// encoded: the public key to place in the daemon's config, the private key to
// keep on the architect's machine.
func Generate() (pub, priv string, err error) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pk),
		base64.StdEncoding.EncodeToString(sk), nil
}

// Sign signs a message with a base64 private key, returning the base64
// signature.
func Sign(privB64 string, msg []byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privB64))
	if err != nil {
		return "", err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return "", errors.New("the private key is the wrong size")
	}
	sig := ed25519.Sign(ed25519.PrivateKey(raw), msg)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify reports whether a base64 signature over a message was made by the
// private key matching the given public key. Any decoding failure is a
// verification failure, never a panic.
func Verify(pub ed25519.PublicKey, msg []byte, sigB64 string) bool {
	if len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, msg, sig)
}

// NormalizeSig returns a signature's canonical base64 form: the base64 of its
// decoded bytes, so two encodings of the same signature (extra whitespace,
// padding differences) collapse to one string. ok is false when it is not a
// well-formed signature. The daemon keys its one-shot replay cache on this form
// so a permuted copy of an accepted signature cannot pass as a new one.
func NormalizeSig(sigB64 string) (string, bool) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return "", false
	}
	return base64.StdEncoding.EncodeToString(raw), true
}

// DecodePublic turns a base64 public key into the key the daemon verifies
// against. An empty string yields a nil key and no error: the daemon reads that
// as "no architect key configured".
func DecodePublic(pubB64 string) (ed25519.PublicKey, error) {
	pubB64 = strings.TrimSpace(pubB64)
	if pubB64 == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("the public key is the wrong size")
	}
	return ed25519.PublicKey(raw), nil
}

// PublicFromPrivate derives the public half of a base64 private key, so a
// signer can print the public key to register without a separate file.
func PublicFromPrivate(privB64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privB64))
	if err != nil {
		return "", err
	}
	if len(raw) != ed25519.PrivateKeySize {
		return "", errors.New("the private key is the wrong size")
	}
	pub := ed25519.PrivateKey(raw).Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub), nil
}

// PrivateFromEnv reads the architect's signing key from the environment: the
// key itself in GOLEM_PRIVATE_KEY, or a file holding it in
// GOLEM_PRIVATE_KEY_FILE. A missing key is an error the caller can explain.
func PrivateFromEnv() (string, error) {
	if k := strings.TrimSpace(os.Getenv("GOLEM_PRIVATE_KEY")); k != "" {
		return k, nil
	}
	if f := strings.TrimSpace(os.Getenv("GOLEM_PRIVATE_KEY_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", errors.New("no signing key: set GOLEM_PRIVATE_KEY or GOLEM_PRIVATE_KEY_FILE")
}
