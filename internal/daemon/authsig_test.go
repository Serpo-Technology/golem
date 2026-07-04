package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/authkey"
	"golem/wire"
)

// keyedDaemon boots a loopless daemon that holds a fresh architect public key,
// returning the private key a test signs requests with.
func keyedDaemon(t *testing.T) (*Daemon, string) {
	t.Helper()
	pub, priv, err := authkey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	d, _ := testDaemon(t)
	key, err := authkey.DecodePublic(pub)
	if err != nil {
		t.Fatal(err)
	}
	d.architectPub = key
	return d, priv
}

func signed(t *testing.T, priv string, req wire.Request) wire.Request {
	t.Helper()
	if err := wire.SignRequest(priv, &req); err != nil {
		t.Fatal(err)
	}
	return req
}

// A validly signed request authenticates as the architect over any transport,
// and the same request refused over the network when unsigned still passes on
// the local socket as the operator.
func TestPrincipalOfSignedAndUnsigned(t *testing.T) {
	d, priv := keyedDaemon(t)
	req := signed(t, priv, wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1"})

	pr, err := d.principalOf(req, "tcp")
	if err != nil || !pr.architectSigned {
		t.Fatalf("a signed request should authenticate as the architect: %+v %v", pr, err)
	}
	if pr.sigFingerprint == "" {
		t.Fatal("a signed request should carry a fingerprint for the trail")
	}

	bare := wire.Request{Cmd: "tell", Content: "a wolf chased a rabbit"}
	if _, err := d.principalOf(bare, "tcp"); err == nil {
		t.Fatal("the network transport must refuse an unsigned request")
	}
	pr, err = d.principalOf(bare, "unix")
	if err != nil || pr.architectSigned || !pr.local {
		t.Fatalf("the local socket should accept an unsigned request as the operator: %+v %v", pr, err)
	}
}

// A tampered request does not verify: the signature binds every field, so
// changing the value after signing breaks it.
func TestTamperedRequestRefused(t *testing.T) {
	d, priv := keyedDaemon(t)
	req := signed(t, priv, wire.Request{Cmd: "retune", Query: "f", Arg: "experimentConf", Content: "0.5"})
	req.Content = "0.1" // move the value after signing
	if _, err := d.verifyArchitect(req); err == nil || !strings.Contains(err.Error(), "does not verify") {
		t.Fatalf("a tampered request must be refused: %v", err)
	}
}

// A request signed by a different key does not verify against the daemon's
// architect key.
func TestForeignKeyRefused(t *testing.T) {
	d, _ := keyedDaemon(t)
	_, otherPriv, _ := authkey.Generate()
	req := signed(t, otherPriv, wire.Request{Cmd: "trust", Query: "llm", Weight: 0.9})
	if _, err := d.verifyArchitect(req); err == nil {
		t.Fatal("a request signed by a foreign key must not verify")
	}
}

// A signature is one-shot: replaying an accepted request is refused by the
// seen-signature cache.
func TestReplayRefused(t *testing.T) {
	d, priv := keyedDaemon(t)
	req := signed(t, priv, wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1"})
	if ok, err := d.verifyArchitect(req); !ok || err != nil {
		t.Fatalf("the first use should verify: %v", err)
	}
	if _, err := d.verifyArchitect(req); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("a replayed request must be refused: %v", err)
	}
}

// A whitespace-padded copy of a spent signature verifies but shares its
// canonical form, so the one-shot cache catches it as a replay rather than
// letting the permuted string pass as new.
func TestSignatureMalleabilityCaughtAsReplay(t *testing.T) {
	d, priv := keyedDaemon(t)
	req := signed(t, priv, wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1"})
	if ok, err := d.verifyArchitect(req); !ok || err != nil {
		t.Fatalf("the first use should verify: %v", err)
	}
	padded := req
	padded.Sig = " " + req.Sig + "\n"
	if _, err := d.verifyArchitect(padded); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("a padded copy of a spent signature must be caught as a replay: %v", err)
	}
}

// A near-future signature's cache entry expires at Ts plus the window, not at
// insertion time, so it cannot be replayed in the gap between an early sweep
// and its own freshness deadline.
func TestFutureTimestampCacheOutlivesFreshness(t *testing.T) {
	d, priv := keyedDaemon(t)
	future := time.Now().Add(sigWindow - 10*time.Second).UnixNano()
	req := wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1", Ts: future}
	if err := wire.SignRequest(priv, &req); err != nil {
		t.Fatal(err)
	}
	if ok, err := d.verifyArchitect(req); !ok || err != nil {
		t.Fatalf("a near-future signature within the window should verify: %v", err)
	}
	key, _ := authkey.NormalizeSig(req.Sig)
	if exp := d.seenSigs[key]; exp != future+int64(sigWindow) {
		t.Fatalf("the cache entry must expire at Ts+window (%d), got %d", future+int64(sigWindow), exp)
	}
}

// A signature whose timestamp is outside the window is stale, even though the
// cryptography verifies: a captured request cannot be replayed once its moment
// has passed.
func TestStaleTimestampRefused(t *testing.T) {
	d, priv := keyedDaemon(t)
	req := wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1",
		Ts: time.Now().Add(-10 * time.Minute).UnixNano()}
	// Sign with the backdated Ts already set, so the signature is valid but old.
	if err := wire.SignRequest(priv, &req); err != nil {
		t.Fatal(err)
	}
	if _, err := d.verifyArchitect(req); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("a stale signature must be refused: %v", err)
	}
}

// With no architect key configured, a presented signature cannot be verified
// and is refused, and the network transport refuses every request.
func TestNoKeyConfiguredRefusesSignedAndNetwork(t *testing.T) {
	d, _ := testDaemon(t) // no architectPub set
	_, priv, _ := authkey.Generate()
	req := signed(t, priv, wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1"})
	if _, err := d.principalOf(req, "unix"); err == nil {
		t.Fatal("a signature with no configured key must be refused, not ignored")
	}
	if _, err := d.principalOf(wire.Request{Cmd: "tell", Content: "hi"}, "tcp"); err == nil {
		t.Fatal("the network transport must refuse when no key is configured")
	}
}

// The canonical form the signature covers excludes only the signature itself:
// two requests that differ only in Sig share canonical bytes, and any other
// difference changes them.
func TestCanonicalExcludesOnlySignature(t *testing.T) {
	a := wire.Request{Cmd: "retune", Query: "f", Arg: "c", Content: "1", Ts: 42, Sig: "AAAA"}
	b := a
	b.Sig = "ZZZZ"
	if string(wire.CanonicalBytes(a)) != string(wire.CanonicalBytes(b)) {
		t.Fatal("canonical bytes must not depend on the signature field")
	}
	c := a
	c.Content = "2"
	if string(wire.CanonicalBytes(a)) == string(wire.CanonicalBytes(c)) {
		t.Fatal("canonical bytes must change when any other field changes")
	}
	d := a
	d.Ts = 43
	if string(wire.CanonicalBytes(a)) == string(wire.CanonicalBytes(d)) {
		t.Fatal("canonical bytes must include the timestamp")
	}
}

// loopbackAddr reads a bind address the way the daemon's startup note does.
func TestLoopbackAddr(t *testing.T) {
	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:7777", true},
		{"[::1]:7777", true},
		{"localhost:7777", true},
		{"0.0.0.0:7777", false},
		{"192.168.1.5:7777", false},
	} {
		if got := loopbackAddr(tc.addr); got != tc.want {
			t.Fatalf("loopbackAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}
