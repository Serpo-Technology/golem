package daemon

import "testing"

// A bulk channel takes its per-channel encode burst while a trickle channel
// keeps the default, so the web can bank several sources in one cycle instead of
// starving every page after the first.
func TestPerChannelEncodeBurst(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.EncodeBurst = 6
	d.cfg.EncodeBursts = map[string]float64{"web": 50}

	if got := d.governor("web").max; got != 50 {
		t.Fatalf("the web governor should take its per-channel burst: %v", got)
	}
	if got := d.governor("syslog").max; got != 6 {
		t.Fatalf("an unlisted channel should keep the default burst: %v", got)
	}
}
