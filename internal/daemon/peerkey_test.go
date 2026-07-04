package daemon

import (
	"strings"
	"testing"

	"golem/internal/authkey"
	"golem/wire"
)

// enroll binds a peer's public key on the daemon, the architect action a peer's
// self-signing depends on.
func enroll(t *testing.T, d *Daemon, name, pubB64 string) {
	t.Helper()
	if err := d.setSourceKey(name, pubB64); err != nil {
		t.Fatal(err)
	}
}

// A peer that signs with its own enrolled key is authenticated as exactly that
// source, over the network transport, without the architect's key.
func TestEnrolledPeerProvesItsIdentity(t *testing.T) {
	d, _ := keyedDaemon(t)
	pub, priv, _ := authkey.Generate()
	enroll(t, d, "llm", pub)

	req := signed(t, priv, wire.Request{Cmd: "tell", Source: "llm", Content: "a zebra is an animal"})
	pr, err := d.principalOf(req, "tcp")
	if err != nil {
		t.Fatalf("an enrolled peer's own signature should authenticate over tcp: %v", err)
	}
	if pr.architectSigned {
		t.Fatal("a peer must not be seen as the architect")
	}
	if pr.peerSource != "llm" {
		t.Fatalf("the peer should be carried as its source: %q", pr.peerSource)
	}
}

// A peer cannot borrow another registered source's name: its signature verifies
// only under its own key, so declaring a different source fails.
func TestPeerCannotBorrowAnotherSource(t *testing.T) {
	d, _ := keyedDaemon(t)
	pubA, privA, _ := authkey.Generate()
	pubB, _, _ := authkey.Generate()
	enroll(t, d, "alice", pubA)
	enroll(t, d, "bob", pubB)

	// Alice's key signs, but the request claims to be bob.
	req := signed(t, privA, wire.Request{Cmd: "tell", Source: "bob", Content: "a cat is an animal"})
	if _, err := d.principalOf(req, "tcp"); err == nil {
		t.Fatal("a peer signing as itself must not be heard as another enrolled source")
	}
}

// A peer cannot claim to be the architect: signing with its own key and
// declaring the architect source verifies under neither key.
func TestPeerCannotClaimArchitect(t *testing.T) {
	d, _ := keyedDaemon(t)
	pub, priv, _ := authkey.Generate()
	enroll(t, d, "llm", pub)

	req := signed(t, priv, wire.Request{Cmd: "tell", Source: architectSource, Content: "x"})
	if _, err := d.principalOf(req, "tcp"); err == nil {
		t.Fatal("a peer must not authenticate as the architect")
	}
	// And a peer signature never satisfies an authority gate.
	req2 := signed(t, priv, wire.Request{Cmd: "trust", Source: "llm", Query: "llm", Weight: 0.9})
	pr, err := d.principalOf(req2, "tcp")
	if err != nil {
		t.Fatalf("the peer request itself authenticates as the peer: %v", err)
	}
	if pr.architectSigned {
		t.Fatal("a peer signature must never be architect-signed")
	}
}

// An unenrolled source cannot self-authenticate: with no key on file, its own
// signature verifies under nothing and is refused over the network.
func TestUnenrolledSourceRefusedOnNetwork(t *testing.T) {
	d, _ := keyedDaemon(t)
	_, priv, _ := authkey.Generate()
	req := signed(t, priv, wire.Request{Cmd: "tell", Source: "stranger", Content: "x"})
	if _, err := d.principalOf(req, "tcp"); err == nil {
		t.Fatal("an unenrolled source must not authenticate over the network")
	}
}

// Enrollment survives a restart: keys written to the store are decoded back
// into the map a fresh daemon checks.
func TestEnrolledKeysLoadFromStore(t *testing.T) {
	d, s := testDaemon(t)
	pub, priv, _ := authkey.Generate()
	if err := d.setSourceKey("llm", pub); err != nil {
		t.Fatal(err)
	}
	// A fresh daemon over the same store loads the enrolled key the way a boot
	// does, so the peer is honored again after a restart.
	fresh := testDaemonOn(t, s)
	if err := fresh.loadSourceKeys(); err != nil {
		t.Fatal(err)
	}
	apub, _, _ := authkey.Generate()
	fresh.architectPub, _ = authkey.DecodePublic(apub)
	req := signed(t, priv, wire.Request{Cmd: "tell", Source: "llm", Content: "a zebra is an animal"})
	pr, err := fresh.principalOf(req, "tcp")
	if err != nil || pr.peerSource != "llm" {
		t.Fatalf("a restarted daemon should honor an enrolled key: %+v %v", pr, err)
	}
}

// Revoking a peer's key removes its ability to prove that identity.
func TestRevokedKeyCannotAuthenticate(t *testing.T) {
	d, _ := keyedDaemon(t)
	pub, priv, _ := authkey.Generate()
	enroll(t, d, "llm", pub)
	if err := d.setSourceKey("llm", ""); err != nil { // revoke
		t.Fatal(err)
	}
	req := signed(t, priv, wire.Request{Cmd: "tell", Source: "llm", Content: "x"})
	if _, err := d.principalOf(req, "tcp"); err == nil {
		t.Fatal("a revoked key must not authenticate")
	}
}

// The enroll verb takes the architect's signature: a peer cannot enroll itself
// or anyone else.
func TestEnrollNeedsArchitectSignature(t *testing.T) {
	d, _ := keyedDaemon(t)
	pub, _, _ := authkey.Generate()
	d.reqPrincipal = principal{peerSource: "llm"} // a peer, not the architect
	resp, _, _ := d.handle(wire.Request{Cmd: "enroll", Query: "llm", Content: pub}, nil)
	if !strings.Contains(resp.Error, "architect") {
		t.Fatalf("a peer must not enroll a key: %+v", resp)
	}
	d.reqPrincipal = principal{architectSigned: true}
	resp, _, _ = d.handle(wire.Request{Cmd: "enroll", Query: "llm", Content: pub}, nil)
	if !resp.OK {
		t.Fatalf("the architect should enroll a key: %+v", resp)
	}
}

// mustPub returns a valid base64 public key (identity helper for readability).
func mustPub(t *testing.T, pub string) string {
	t.Helper()
	if _, err := authkey.DecodePublic(pub); err != nil {
		t.Fatal(err)
	}
	return pub
}
