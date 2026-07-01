package daemon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
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
