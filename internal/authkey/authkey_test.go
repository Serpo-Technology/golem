package authkey

import (
	"strings"
	"testing"
)

// A generated keypair signs and verifies its own messages, and the public half
// derives from the private half, so a signer can hand out its public key.
func TestGenerateSignVerify(t *testing.T) {
	pubB64, privB64, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("retune experimentConf to 0.5")
	sig, err := Sign(privB64, msg)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := DecodePublic(pubB64)
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(pub, msg, sig) {
		t.Fatal("a fresh signature should verify against its own public key")
	}
	derived, err := PublicFromPrivate(privB64)
	if err != nil || derived != pubB64 {
		t.Fatalf("the public key should derive from the private key: %q vs %q (%v)", derived, pubB64, err)
	}
}

// A signature does not verify against a different key, over a tampered message,
// or when the signature itself is garbled.
func TestVerifyRejectsForgeryAndTampering(t *testing.T) {
	pubB64, privB64, _ := Generate()
	otherPub, _, _ := Generate()
	msg := []byte("a wolf chased a rabbit")
	sig, _ := Sign(privB64, msg)

	pub, _ := DecodePublic(pubB64)
	wrong, _ := DecodePublic(otherPub)
	if Verify(wrong, msg, sig) {
		t.Fatal("a signature must not verify against another key")
	}
	if Verify(pub, []byte("a wolf chased a deer"), sig) {
		t.Fatal("a tampered message must not verify")
	}
	if Verify(pub, msg, "not-base64-!!") {
		t.Fatal("a garbled signature must not verify, and must not panic")
	}
	if Verify(pub, msg, "") {
		t.Fatal("an empty signature must not verify")
	}
}

// A malformed key is refused rather than accepted or panicking, and an empty
// public key decodes to nil so the daemon can read it as "no key set".
func TestKeyDecodingIsStrict(t *testing.T) {
	if _, err := Sign("not-base64", []byte("x")); err == nil {
		t.Fatal("a malformed private key should error")
	}
	if _, err := DecodePublic("aGVsbG8="); err == nil {
		t.Fatal("a wrong-size public key should error")
	}
	pub, err := DecodePublic("")
	if err != nil || pub != nil {
		t.Fatalf("an empty public key should decode to nil, no error: %v %v", pub, err)
	}
}

// PrivateFromEnv reads the key from the environment and reports a clear error
// when none is set.
func TestPrivateFromEnv(t *testing.T) {
	t.Setenv("GOLEM_PRIVATE_KEY", "")
	t.Setenv("GOLEM_PRIVATE_KEY_FILE", "")
	if _, err := PrivateFromEnv(); err == nil || !strings.Contains(err.Error(), "no signing key") {
		t.Fatalf("a missing key should be a clear error: %v", err)
	}
	_, priv, _ := Generate()
	t.Setenv("GOLEM_PRIVATE_KEY", priv)
	if got, err := PrivateFromEnv(); err != nil || got != priv {
		t.Fatalf("the key should be read from the environment: %v", err)
	}
}
