package daemon

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // register the decoders so a percept can arrive in any common form
	_ "image/jpeg" // the eye reads the bytes; it does not care which codec wrote them
	_ "image/png"
	"math"
	"sort"
	"strings"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// colourFocusWindow is how long the thing just seen stays in focus for a colour to be
// told about it: a conversational moment, so "its colour is crimson" a breath after
// seeing lands, while a stray colour word much later does not.
const colourFocusWindow = 2 * time.Minute

var colourAnchors = map[string]bool{"colour": true, "color": true, "renk": true, "rengi": true, "colorem": true}
var colourDeictics = map[string]bool{"it": true, "that": true, "this": true, "bu": true, "şu": true, "o": true, "id": true, "hoc": true}

// planColorNaming grounds a colour word told right after GOLEM has seen something: "its
// colour is crimson", "it is crimson", "bu renk kırmızı". The word binds to the
// perceptual cell the eye just measured, so a colour term is learned by being shown a
// thing and told its colour, dissolving the teaching into conversation rather than a
// command. It declines when nothing was freshly seen, or when the remark names a known
// kind ("it is a ball"), leaving those to the ordinary paths.
func (d *Daemon) planColorNaming(frame grammar.Frame, raw string) (string, bool) {
	if frame.Force != grammar.ForceStatement || frame.Negate || frame.Comparative {
		return "", false
	}
	if d.sawCell == "" || time.Since(d.sawAt) > colourFocusWindow {
		return "", false // nothing freshly seen to attach a colour to
	}
	words := strings.Fields(strings.ToLower(strings.Trim(raw, " .!?")))
	if len(words) < 2 {
		return "", false
	}
	colour := words[len(words)-1] // the colour word closes the clause in every form
	if colourAnchors[colour] || colourDeictics[colour] {
		return "", false // the clause named no colour, only the anchor or the deictic
	}
	anchored := false
	for _, w := range words {
		if colourAnchors[w] {
			anchored = true
			break
		}
	}
	// An explicit "colour is X" names a colour outright; a deictic "it is X" does when X
	// is a new word rather than a known kind, so "it is a ball" is left to naming.
	deictic := colourDeictics[words[0]] && d.resolveConcept(colour) == nil
	if !anchored && !deictic {
		return "", false
	}
	lang := frame.Lang
	if lang == "" {
		lang = "en"
	}
	if !d.groundColor(colour, lang, d.sawCell) {
		return "", false
	}
	return fmt.Sprintf("I see, that colour is %s.", colour), true
}

// GOLEM's own eye. Perception is not a caption handed in from outside; it is measured
// here, from the pixels, in the same interpretable attributes its lessons are captioned
// with -- a dominant colour, a gross shape, a lightness -- so what it sees is described
// in its own vocabulary and can be reasoned over and recognized by resemblance like
// anything it was shown by hand. Every step is a plain rule over the bytes, auditable
// the way the rest of the substrate is, not a foreign model's verdict.

const (
	visionGrid      = 48   // the eye samples the image down to a grid this wide, enough for colour and gross shape
	visionFgDist    = 0.22 // a sample this far from the background colour belongs to the subject, not the ground
	visionRoundFill = 0.62 // a subject filling this much of its box but leaving its corners empty reads as round
	visionSquareCap = 0.90 // above this its box is nearly full, so it is a block, not round
)

// sighting is what the eye measured off an image before any word is put to it: the raw
// colour and the perceptual cell it falls in, the gross shape, and the codec, all plain
// numbers so naming and learning can work over the same measurement.
type sighting struct {
	color [3]float64
	cell  string // the perceptual colour cell the measurement falls in, the key a learned word grounds to
	shape string
	mime  string
}

// measure reads an image into a sighting: the interpretable attributes worked out from
// the pixels by plain rule, with no word attached yet. ok is false when the bytes are
// not an image it can decode.
func (d *Daemon) measure(data []byte) (sighting, bool) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return sighting{}, false
	}
	px := sampleGrid(img, visionGrid)
	if len(px) == 0 {
		return sighting{}, false
	}
	bg := borderColor(px)
	fg, box, fill := foreground(px, bg)
	// A picture with no subject standing out from its ground (a flat swatch, a texture)
	// is taken whole: its colour is the colour of the field.
	if len(fg) == 0 {
		fg, box, fill = allPixels(px)
	}
	c := meanColor(fg)
	return sighting{color: c, cell: colorCell(c), shape: shapeWord(box, fill), mime: "image/" + format}, true
}

// render puts GOLEM's own words to a sighting: a noun phrase of the attributes it made
// out ("a round red form"), the head left generic because naming the kind is
// recognition's job, not the eye's. The colour word is the one GOLEM has learned for
// that cell if it has learned one, else the innate word it is born able to say.
func (d *Daemon) render(s sighting, lang string) string {
	adjs := make([]string, 0, 3)
	if s.shape != "" {
		adjs = append(adjs, s.shape)
	}
	adjs = append(adjs, d.colorFor(s, lang))
	return "a " + strings.Join(adjs, " ") + " form"
}

// perceive is the whole eye in one step for callers that just want a caption: measure,
// then render in GOLEM's own words.
func (d *Daemon) perceive(data []byte) (caption, mime string, ok bool) {
	s, ok := d.measure(data)
	if !ok {
		return "", "", false
	}
	return d.render(s, ""), s.mime, true
}

// colorFor is the colour word GOLEM puts to a sighting: the word it has learned names
// this cell, preferring the tongue asked for, and failing that the innate word. So a
// colour term taught by grounding (in any tongue) speaks over the born-in one, and the
// born-in one is there from the first glance before anything is taught.
func (d *Daemon) colorFor(s sighting, lang string) string {
	if w := d.learnedColorFor(s.cell, lang); w != "" {
		return w
	}
	return colorWord(s.color)
}

// colorCell is the perceptual cell a colour falls in, the key a learned word grounds
// to: an unsaturated colour by its lightness, a saturated one by a coarse hue band.
// The cells are the raw measurement, finer than the innate colour words and prior to
// them, so a word is learned against what the eye sees, not against another word.
func colorCell(c [3]float64) string {
	r, g, b := c[0], c[1], c[2]
	maxc := math.Max(r, math.Max(g, b))
	minc := math.Min(r, math.Min(g, b))
	var sat float64
	if maxc > 0 {
		sat = (maxc - minc) / maxc
	}
	if sat < 0.16 {
		switch {
		case maxc < 0.25:
			return "shade:dark"
		case maxc < 0.72:
			return "shade:gray"
		default:
			return "shade:light"
		}
	}
	return fmt.Sprintf("hue:%d", int(hue(r, g, b, maxc, minc))/30)
}

// groundColor learns a colour word by binding it to the perceptual cell the eye just
// measured: shown a thing and told its colour, the word comes to name that region of
// what GOLEM sees. The cell is a spared perceptual class, so the grounding stands
// across the night, and the bond is at deliberate-label strength, a told colour taken
// on GOLEM's word.
func (d *Daemon) groundColor(word, lang, cell string) bool {
	id := d.ensureClassConcept("percept:"+cell, cell)
	if id == 0 {
		return false
	}
	w, _, err := d.store.WordNode(word, lang, d.chem.Snapshot(), time.Now())
	if err != nil {
		return false
	}
	return d.store.LinkRel(w.ID, id, memory.RelNames, deliberateName) == nil
}

// learnedColorFor is the colour word GOLEM has learned for a cell, the strongest namer
// of that perceptual class, preferring the tongue asked for so a bilingual eye answers
// in the tongue of the moment. Empty when nothing has been grounded there yet.
func (d *Daemon) learnedColorFor(cell, lang string) string {
	c, err := d.store.ConceptByTemplate("percept:" + cell)
	if err != nil || c == nil {
		return ""
	}
	namers, err := d.store.LinksToRel([]uint64{c.ID}, memory.RelNames)
	if err != nil || len(namers) == 0 {
		return ""
	}
	ids := make([]uint64, 0, len(namers))
	weight := make(map[uint64]float64, len(namers))
	for _, e := range namers {
		ids = append(ids, e.From)
		weight[e.From] = e.Weight
	}
	words, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return ""
	}
	best, bestScore := "", -1.0
	for _, w := range words {
		score := weight[w.ID]
		if lang != "" && w.Lang == lang {
			score += 1 // the tongue of the moment wins a tie, and outranks a firmer word of another tongue
		}
		if score > bestScore {
			best, bestScore = w.Content, score
		}
	}
	return best
}

// pixel is one sampled point: its position on the sampling grid and its colour.
type pixel struct {
	x, y    int
	r, g, b float64
}

// sampleGrid reduces an image to a small grid of colour samples, so colour and gross
// shape are read at a steady cost whatever the image's true resolution.
func sampleGrid(img image.Image, grid int) []pixel {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	cols, rows := grid, grid
	if w < cols {
		cols = w
	}
	if h < rows {
		rows = h
	}
	out := make([]pixel, 0, cols*rows)
	for gy := 0; gy < rows; gy++ {
		for gx := 0; gx < cols; gx++ {
			sx := b.Min.X + gx*w/cols
			sy := b.Min.Y + gy*h/rows
			r16, g16, b16, _ := img.At(sx, sy).RGBA()
			out = append(out, pixel{
				x: gx, y: gy,
				r: float64(r16) / 65535, g: float64(g16) / 65535, b: float64(b16) / 65535,
			})
		}
	}
	return out
}

// borderColor is the mean colour of the grid's edge, taken as the background the
// subject sits on: a photographed or drawn thing usually leaves its ground at the frame.
func borderColor(px []pixel) [3]float64 {
	if len(px) == 0 {
		return [3]float64{}
	}
	maxX, maxY := 0, 0
	for _, p := range px {
		if p.x > maxX {
			maxX = p.x
		}
		if p.y > maxY {
			maxY = p.y
		}
	}
	var sr, sg, sb float64
	n := 0
	for _, p := range px {
		if p.x == 0 || p.y == 0 || p.x == maxX || p.y == maxY {
			sr, sg, sb, n = sr+p.r, sg+p.g, sb+p.b, n+1
		}
	}
	if n == 0 {
		return [3]float64{}
	}
	return [3]float64{sr / float64(n), sg / float64(n), sb / float64(n)}
}

// foreground picks the samples that stand out from the background, and reports their
// bounding box and how much of that box they fill, the two numbers gross shape reads.
func foreground(px []pixel, bg [3]float64) (fg []pixel, box [4]int, fill float64) {
	minX, minY, maxX, maxY := math.MaxInt32, math.MaxInt32, 0, 0
	for _, p := range px {
		if colorDist(p.r, p.g, p.b, bg[0], bg[1], bg[2]) < visionFgDist {
			continue
		}
		fg = append(fg, p)
		if p.x < minX {
			minX = p.x
		}
		if p.y < minY {
			minY = p.y
		}
		if p.x > maxX {
			maxX = p.x
		}
		if p.y > maxY {
			maxY = p.y
		}
	}
	if len(fg) == 0 {
		return nil, [4]int{}, 0
	}
	box = [4]int{minX, minY, maxX, maxY}
	area := float64((maxX - minX + 1) * (maxY - minY + 1))
	if area > 0 {
		fill = float64(len(fg)) / area
	}
	return fg, box, fill
}

// allPixels treats the whole frame as the subject, for an image with no figure against
// a ground.
func allPixels(px []pixel) (fg []pixel, box [4]int, fill float64) {
	maxX, maxY := 0, 0
	for _, p := range px {
		if p.x > maxX {
			maxX = p.x
		}
		if p.y > maxY {
			maxY = p.y
		}
	}
	return px, [4]int{0, 0, maxX, maxY}, 1
}

func colorDist(r1, g1, b1, r2, g2, b2 float64) float64 {
	dr, dg, db := r1-r2, g1-g2, b1-b2
	return math.Sqrt((dr*dr + dg*dg + db*db) / 3)
}

func meanColor(px []pixel) [3]float64 {
	if len(px) == 0 {
		return [3]float64{}
	}
	var sr, sg, sb float64
	for _, p := range px {
		sr, sg, sb = sr+p.r, sg+p.g, sb+p.b
	}
	n := float64(len(px))
	return [3]float64{sr / n, sg / n, sb / n}
}

// shapeWord reads gross shape from the subject's box and how it fills it: a tall or long
// box by its sides, a plump box that leaves its corners empty as round, a nearly full
// box as a block. A subject with no clear shape gets no word rather than a guessed one.
func shapeWord(box [4]int, fill float64) string {
	w := float64(box[2]-box[0]) + 1
	h := float64(box[3]-box[1]) + 1
	if w <= 1 || h <= 1 {
		return ""
	}
	switch {
	case h >= 1.5*w:
		return "tall"
	case w >= 1.5*h:
		return "long"
	case fill >= visionRoundFill && fill < visionSquareCap:
		return "round"
	}
	return ""
}

// namedColor pairs a colour with the word GOLEM knows it by. The table is a small,
// readable sensory given, the visible band cut where the common colour words fall, the
// way the closed-class words are given; it is the one built-in the eye leans on, and it
// can be refined by grounding later.
type namedColor struct {
	name    string
	hMin    float64
	hMax    float64
}

var hueBands = []namedColor{
	{"red", 345, 360}, {"red", 0, 15}, {"orange", 15, 45}, {"yellow", 45, 70},
	{"green", 70, 170}, {"blue", 170, 255}, {"purple", 255, 300}, {"pink", 300, 345},
}

// colorWord names a colour the way a caption would: an unsaturated colour by its
// lightness (black, gray, white), a dark warm one as brown, otherwise by the hue band
// it falls in. These are the colour words the lessons use, so a percept's colour is a
// feature the resemblance walk already understands.
func colorWord(c [3]float64) string {
	r, g, b := c[0], c[1], c[2]
	maxc := math.Max(r, math.Max(g, b))
	minc := math.Min(r, math.Min(g, b))
	var sat float64
	if maxc > 0 {
		sat = (maxc - minc) / maxc
	}
	if sat < 0.16 { // no colour to speak of, only lightness
		switch {
		case maxc < 0.25:
			return "black"
		case maxc < 0.72:
			return "gray"
		default:
			return "white"
		}
	}
	h := hue(r, g, b, maxc, minc)
	// A dark, warm, muted colour reads as brown rather than a dim orange.
	if h < 45 && maxc < 0.5 {
		return "brown"
	}
	for _, band := range hueBands {
		if h >= band.hMin && h < band.hMax {
			return band.name
		}
	}
	return "gray"
}

func hue(r, g, b, maxc, minc float64) float64 {
	d := maxc - minc
	if d == 0 {
		return 0
	}
	var h float64
	switch maxc {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}

// recognize reports the known concepts a just-seen percept most resembles, by the
// features its caption shares with them: the eye made out the attributes, the
// resemblance walk says what thing tends to carry them. It is GOLEM's own recognition,
// not a label read off the image.
func (d *Daemon) recognize(caption string) []*memory.Fragment {
	resembled, _ := d.featureSeeds(caption)
	sort.SliceStable(resembled, func(i, j int) bool { return resembled[i].Weight > resembled[j].Weight })
	return resembled
}
