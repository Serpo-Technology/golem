package senses

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// WebSensor is the web-text sense: each watched URL is fetched on the
// poll interval and the page's stripped text arrives line by line on
// the web channel. The text is already symbolic, so no transducer
// stands at the edge; what does stand there is distrust. The web
// channel ships untrusted: its moments encode at reduced weight and are
// barred from cueing habits and from anchoring the grounding of an
// action, so an adversarial page can be remembered without ever
// becoming a reflex.
const (
	webChannel   = "web"
	webLineCap   = 240
	webLineMin   = 3
	webLinesMax  = 200     // lines per fetch; the gate and governor do the rest
	webBodyCap   = 1 << 20 // bytes read per page
	webFetchWait = 20 * time.Second
)

func WebSensor(urls []string, poll time.Duration, out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	client := &http.Client{Timeout: webFetchWait}
	fetchAll := func() {
		for _, url := range urls {
			select {
			case <-stop:
				return
			default:
			}
			fetchOne(client, url, out, dropped)
		}
	}
	fetchAll()
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			fetchAll()
		}
	}
}

func fetchOne(client *http.Client, url string, out chan Observation, dropped *atomic.Int64) {
	resp, err := client.Get(url)
	if err != nil {
		emit(out, dropped, Observation{Channel: webChannel,
			Content: "web fetch failed: " + err.Error(), At: time.Now()})
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, webBodyCap))
	if err != nil {
		return
	}
	lines := StripHTML(string(body))
	if len(lines) > webLinesMax {
		lines = lines[:webLinesMax]
	}
	for _, line := range lines {
		emit(out, dropped, Observation{Channel: webChannel, Content: line, At: time.Now()})
	}
}

// StripHTML reduces a page to its visible text lines: script and style
// bodies dropped, tags removed, the basic entities decoded, whitespace
// collapsed, and the very short fragments (menu glyphs, separators)
// discarded. Block-level tags break lines, so a paragraph arrives as
// one observation.
func StripHTML(page string) []string {
	var b strings.Builder
	lower := strings.ToLower(page)
	i := 0
	for i < len(page) {
		if page[i] != '<' {
			b.WriteByte(page[i])
			i++
			continue
		}
		// Script and style bodies are invisible: skip to the close tag.
		skipped := false
		for _, tag := range []string{"script", "style"} {
			if strings.HasPrefix(lower[i+1:], tag) {
				if end := strings.Index(lower[i:], "</"+tag); end >= 0 {
					i += end
				} else {
					i = len(page)
				}
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		end := strings.IndexByte(page[i:], '>')
		if end < 0 {
			break
		}
		// A block-level boundary ends the current line of text.
		tag := strings.TrimLeft(lower[i+1:i+end], "/")
		for _, block := range []string{"p", "div", "br", "li", "tr", "h1", "h2", "h3", "h4", "title", "section", "article"} {
			if tag == block || strings.HasPrefix(tag, block+" ") {
				b.WriteByte('\n')
				break
			}
		}
		i += end + 1
	}

	text := b.String()
	for entity, plain := range map[string]string{
		"&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": `"`,
		"&#39;": "'", "&apos;": "'", "&nbsp;": " ",
	} {
		text = strings.ReplaceAll(text, entity, plain)
	}

	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if len(line) < webLineMin {
			continue
		}
		if len(line) > webLineCap {
			line = line[:webLineCap]
		}
		lines = append(lines, line)
	}
	return lines
}
