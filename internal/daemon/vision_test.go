package daemon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// redBall draws a filled red disc on white, the shape and colour "a round red ball" is
// captioned with, so the eye is tested against a percept the curriculum knows.
func discPNG(t *testing.T, w, h int, cx, cy, r int, fg color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, fg)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// GOLEM names a colour with the innate word at first glance, then, shown the thing and
// told a colour term, learns to speak that term for what it measured, in whichever
// tongue it was taught, over the born-in one.
func TestLearnedColorNameGroundsAndSpeaks(t *testing.T) {
	d, _ := testDaemon(t)
	red := discPNG(t, 200, 200, 100, 100, 70, color.RGBA{200, 30, 30, 255})

	if cap0, _, _ := d.perceive(red); !strings.Contains(cap0, "red") {
		t.Fatalf("the innate eye should say red before anything is taught, got %q", cap0)
	}
	s, ok := d.measure(red)
	if !ok {
		t.Fatal("the disc should be measurable")
	}
	if !d.groundColor("crimson", "en", s.cell) {
		t.Fatal("the colour word should ground")
	}
	if got := d.render(s, "en"); !strings.Contains(got, "crimson") {
		t.Fatalf("the learned colour word should speak over the innate one, got %q", got)
	}
	// Cross-tongue: the Turkish term grounds to the same cell and wins under its tongue.
	if !d.groundColor("kırmızı", "tr", s.cell) {
		t.Fatal("the Turkish colour word should ground")
	}
	if got := d.render(s, "tr"); !strings.Contains(got, "kırmızı") {
		t.Fatalf("the tongue of the moment should win, got %q", got)
	}
	if got := d.render(s, "en"); !strings.Contains(got, "crimson") {
		t.Fatalf("English should still say crimson, got %q", got)
	}
}

// A colour is taught by talking, not a command: GOLEM sees a thing, and a colour told
// right after ("its colour is crimson") grounds the word to what its eye measured, and
// the eye then speaks it. With nothing in view, a colour remark grounds nothing.
func TestColorTaughtByTalkingAfterSeeing(t *testing.T) {
	d, _ := testDaemon(t)
	s, ok := d.measure(discPNG(t, 200, 200, 100, 100, 70, color.RGBA{200, 30, 30, 255}))
	if !ok {
		t.Fatal("the disc should be measurable")
	}

	say(t, d, "its colour is teal") // nothing seen yet
	if d.learnedColorFor(s.cell, "en") == "teal" {
		t.Fatal("a colour told with nothing in view should ground nothing")
	}

	d.sawCell, d.sawAt = s.cell, time.Now() // GOLEM has just seen the red thing
	say(t, d, "its colour is crimson")
	if got := d.learnedColorFor(s.cell, "en"); got != "crimson" {
		t.Fatalf("a colour told after seeing should ground to what was measured; got %q", got)
	}
	if cap := d.render(s, "en"); !strings.Contains(cap, "crimson") {
		t.Fatalf("the eye should then speak the taught colour; got %q", cap)
	}
}

// A learned colour term is a synonym of the born-in word for the same cell, so a percept
// described with the taught word still recognizes a concept described with the born-in one:
// recognition matches through the shared colour sense, not the surface token.
func TestSynonymRecognitionThroughSharedSense(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "vision", "a round red ball", 5, time.Now().Add(-3*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	if err := d.ensureColourSenses(); err != nil { // the born-in colour words name their cells, as at boot
		t.Fatal(err)
	}
	sight, ok := d.measure(discPNG(t, 200, 200, 100, 100, 70, color.RGBA{200, 30, 30, 255}))
	if !ok {
		t.Fatal("the disc should be measurable")
	}
	if !d.groundColor("crimson", "en", sight.cell) {
		t.Fatal("crimson should ground to the red cell")
	}

	syns := d.senseSynonyms("crimson")
	sharesRed := false
	for _, w := range syns {
		if w == "red" {
			sharesRed = true
		}
	}
	if !sharesRed {
		t.Fatalf("crimson and red share a colour cell, so they should be sense synonyms; got %v", syns)
	}
	if got := d.senseSynonyms("wobblexyz"); len(got) != 0 {
		t.Fatalf("an ungrounded word has no sense synonyms; got %v", got)
	}

	cap := d.render(sight, "en")
	if !strings.Contains(cap, "crimson") {
		t.Fatalf("the eye should speak the taught colour; got %q", cap)
	}
	found := false
	for _, c := range d.recognize(cap) {
		if strings.Contains(c.Content, "ball") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a crimson percept should recognize the red ball through the shared colour sense; cap=%q", cap)
	}
}

// The vector layer complements graph co-reference: two words the graph never joined but
// whose concepts keep the same company (model and llm, taught over the same relations)
// are found as synonyms through their near-identical usage vectors.
func TestUsageVectorSynonymsComplementCoReference(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(gloss, tmpl string) uint64 {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss, Template: tmpl, Category: "noun", Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	name := func(form string, cid uint64) {
		w, _, err := s.WordNode(form, "en", limbic.State{}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(w.ID, cid, memory.RelNames, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	model := mk("model", "concept:en:model")
	llm := mk("llm", "concept:en:llm")
	name("model", model)
	name("llm", llm)
	// Near-identical usage: they fill the same roles of the same relations.
	if err := s.SetUsageVector(model, usageSelf, []float32{0.6, 0.5, 0.4, 0.3}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUsageVector(llm, usageSelf, []float32{0.61, 0.49, 0.41, 0.29}); err != nil {
		t.Fatal(err)
	}
	syns := d.senseSynonyms("model")
	found := false
	for _, w := range syns {
		if w == "llm" {
			found = true
		}
	}
	if !found {
		t.Fatalf("model and llm keep the same company, so vector synonymy should link them; got %v", syns)
	}
}

// Colour boundaries are learned, not fixed: a colour that starts as orange comes to
// classify as red once GOLEM is told, several times, that such a colour is red, because
// the red category's centre moves toward what it is shown.
func TestColourBoundaryLearnedFromTelling(t *testing.T) {
	d, _ := testDaemon(t)
	border := [3]float64{0.92, 0.42, 0.12} // on the red/orange border, innately orange
	if cell := d.classifyColour(border); cell != "col:orange" {
		t.Fatalf("the border colour should start as orange from the born-in prior; got %q", cell)
	}
	for i := 0; i < 6; i++ {
		d.learnColour("col:red", border)
	}
	if cell := d.classifyColour(border); cell != "col:red" {
		t.Fatalf("after being told it is red, the border colour should classify as red; got %q", cell)
	}
}

// The eye reads a coarse pattern: a solid surface is plain, bands along one axis are
// striped, and scatter across both is spotted, all from the pixels.
func TestTextureWordReadsPattern(t *testing.T) {
	grid := func(f func(x, y int) float64) []pixel {
		var px []pixel
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				v := f(x, y)
				px = append(px, pixel{x: x, y: y, r: v, g: v, b: v})
			}
		}
		return px
	}
	solid := grid(func(x, y int) float64 { return 0.5 })
	stripes := grid(func(x, y int) float64 {
		if y%2 == 0 {
			return 0.05
		}
		return 0.95
	})
	spots := grid(func(x, y int) float64 {
		if (x+y)%2 == 0 {
			return 0.05
		}
		return 0.95
	})
	if got := textureWord(solid); got != "" {
		t.Fatalf("a solid surface should be plain; got %q", got)
	}
	if got := textureWord(stripes); got != "striped" {
		t.Fatalf("bands along one axis should read striped; got %q", got)
	}
	if got := textureWord(spots); got != "spotted" {
		t.Fatalf("scatter across both axes should read spotted; got %q", got)
	}
}

// stripedRectPNG draws a subject filled with horizontal bands of a given pitch on white,
// so the marks can be made finer than the colour/shape grid resolves.
func stripedRectPNG(t *testing.T, w, h, x0, y0, x1, y1, pitch int, a, b color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if x >= x0 && x < x1 && y >= y0 && y < y1 {
				if (y/pitch)%2 == 0 {
					c = a
				} else {
					c = b
				}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The eye reads fine bands as striped end to end: a subject striped at a pitch finer than
// the colour/shape grid resolves is blurred to a flat mean by that coarse sample, but the
// finer texture grid still catches the bands. A solid subject stays plain.
func TestMeasureReadsFineTexture(t *testing.T) {
	d, _ := testDaemon(t)
	dark, light := color.RGBA{40, 40, 40, 255}, color.RGBA{160, 160, 160, 255} // both clearly the subject, not the white ground
	striped := stripedRectPNG(t, 200, 200, 40, 40, 160, 160, 2, dark, light)   // 4px period, below the 48-grid
	s, ok := d.measure(striped)
	if !ok {
		t.Fatal("measure should read the striped subject")
	}
	if s.texture != "striped" {
		t.Fatalf("fine bands should read striped on the finer grid; got %q", s.texture)
	}
	solid := rectPNG(t, 200, 200, 40, 40, 160, 160, color.RGBA{40, 40, 40, 255})
	if s2, ok := d.measure(solid); !ok || s2.texture != "" {
		t.Fatalf("a solid subject should stay plain; got %q (ok=%v)", s2.texture, ok)
	}
}

func rectPNG(t *testing.T, w, h, x0, y0, x1, y1 int, fg color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if x >= x0 && x < x1 && y >= y0 && y < y1 {
				c = fg
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// GOLEM's eye reads a red disc as a round red thing and a tall green bar as a tall green
// one: the colour and gross shape it captions with are measured from the pixels, in its
// own words, not handed in.
func TestPerceiveReadsColorAndShape(t *testing.T) {
	d, _ := testDaemon(t)

	cap, mime, ok := d.perceive(discPNG(t, 200, 200, 100, 100, 70, color.RGBA{200, 30, 30, 255}))
	if !ok {
		t.Fatal("a red disc should be perceivable")
	}
	if mime != "image/png" {
		t.Fatalf("mime should follow the codec, got %q", mime)
	}
	if !strings.Contains(cap, "red") {
		t.Fatalf("a red disc should be seen as red, got %q", cap)
	}
	if !strings.Contains(cap, "round") {
		t.Fatalf("a filled disc should be seen as round, got %q", cap)
	}

	tall, _, ok := d.perceive(rectPNG(t, 200, 200, 85, 20, 115, 180, color.RGBA{40, 160, 40, 255}))
	if !ok {
		t.Fatal("a green bar should be perceivable")
	}
	if !strings.Contains(tall, "green") {
		t.Fatalf("a green bar should be seen as green, got %q", tall)
	}
	if !strings.Contains(tall, "tall") {
		t.Fatalf("a bar taller than it is wide should be seen as tall, got %q", tall)
	}
}

// A percept the eye made out is recognized by resemblance to what GOLEM already knows:
// a red round thing it has never been handed still lands on the red round thing it was
// taught, because the colour and shape it measured are the features the walk shares.
func TestPerceivedPerceptRecognizedByResemblance(t *testing.T) {
	d, s := testDaemon(t)
	// Teach a "round red ball" the ordinary way, so it carries a mined signature.
	feedShape(t, s, "vision", "a round red ball", 5, time.Now().Add(-3*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()

	cap, _, ok := d.perceive(discPNG(t, 200, 200, 100, 100, 70, color.RGBA{200, 30, 30, 255}))
	if !ok {
		t.Fatal("the disc should be perceivable")
	}
	found := false
	var got []string
	for _, c := range d.recognize(cap) {
		got = append(got, c.Content)
		if strings.Contains(c.Content, "ball") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a perceived round red form should resemble the taught round red ball; got %v from %q", got, cap)
	}
}
