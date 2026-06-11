package main

import (
	"fmt"
	"strings"
	"time"
)

// Automatic cueing is the reflex arc: an observation that matches a
// graduated cue triggers the decision loop with no human typing the cue.
// Only graduated pairs participate, the ACC conflict veto still holds, and
// the three execution gates still decide whether anything actually runs.
// A per-cue cooldown keeps a storm of matching lines from firing the same
// habit over and over.

// containsPhrase reports whether phrase occurs in s as a whole-word
// substring, case-insensitively: "go test" is found in "run: go test ./..."
// but "ls" is not found in "also". Word characters are ASCII letters,
// digits, and underscore, so placeholder punctuation (<n>, <path>) and
// spaces count as boundaries and templates match templates naturally.
func containsPhrase(s, phrase string) bool {
	if phrase == "" {
		return false
	}
	s, phrase = strings.ToLower(s), strings.ToLower(phrase)
	for from := 0; from+len(phrase) <= len(s); {
		i := strings.Index(s[from:], phrase)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(phrase)
		if (start == 0 || !isWordByte(s[start-1])) && (end == len(s) || !isWordByte(s[end])) {
			return true
		}
		from = start + 1
	}
	return false
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}

// graduatedCues returns the trigger set for automatic cueing, cached until
// a habit changes. The cache spares the store a query per observation.
func (d *Daemon) graduatedCues() []string {
	if !d.gradCuesLoaded {
		cues, err := d.store.GraduatedCues()
		if err != nil {
			fmt.Fprintf(d.out, "graduated cue lookup failed: %v\n", err)
			return nil
		}
		d.gradCues = cues
		d.gradCuesLoaded = true
	}
	return d.gradCues
}

// habitsChanged invalidates the graduated-cue cache. Every path that saves
// a habit must call it, or a demoted pair could keep firing from memory.
func (d *Daemon) habitsChanged() {
	d.gradCuesLoaded = false
}

// autoCue checks one observation against every graduated cue and runs what
// matches. It is called before the consciousness gate, because routine is
// exactly what graduates: the observation may be predicted away while the
// habit still has to fire. The caller has already excluded forced
// observations and action echoes.
func (d *Daemon) autoCue(o Observation, tpl string) {
	cues := d.graduatedCues()
	if len(cues) == 0 {
		return
	}
	if d.lastAutoCue == nil {
		d.lastAutoCue = make(map[string]time.Time)
	}
	for _, cue := range cues {
		if !containsPhrase(tpl, cue) {
			continue
		}
		if last, ok := d.lastAutoCue[cue]; ok && o.At.Sub(last) < d.cfg.AutoCueCooldown() {
			continue
		}
		// The stamp lands on every match, acted on or vetoed, so a storm
		// cannot hammer the decision loop either.
		d.lastAutoCue[cue] = o.At

		dec, err := d.decide(cue)
		if err != nil || dec.Proposed == nil {
			continue
		}
		if dec.Conflict {
			fmt.Fprintf(d.out, "      ? observed cue %q is ambiguous; staying still. %s\n", cue, dec.Question)
			continue
		}
		// The winning command for this cue may have shifted since
		// graduation; only a pair that is still graduated may fire.
		if dec.Status != HabitGraduated {
			continue
		}
		if allowed, reason := d.mayExecute(dec.Proposed.Command); !allowed {
			fmt.Fprintf(d.out, "      . observed cue %q would run %q, held back: %s\n",
				cue, dec.Proposed.Command, reason)
			continue
		}
		outcome, demoted := d.autorun(dec, "observed", o.At)
		fmt.Fprintf(d.out, "      > observed cue %q ran %q -> %s\n", cue, dec.Proposed.Command, outcome)
		if demoted {
			fmt.Fprintf(d.out, "        the run failed; the pair is demoted until reconfirmed.\n")
		}
	}
}
