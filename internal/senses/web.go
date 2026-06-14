package senses

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
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
	webLinesMax  = 50      // lines per fetch: a page's lead and early body, where the summary sits
	webBodyCap   = 1 << 20 // bytes read per page
	webFetchWait = 20 * time.Second
	// webUserAgent identifies the reader plainly so a polite source serves its
	// page instead of a robot-policy notice. The web sense reads named pages on a
	// slow poll, so it names itself rather than masquerading as a browser.
	webUserAgent = "GOLEM/0.1 (grounded learning daemon)"
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		emit(out, dropped, Observation{Channel: webChannel,
			Content: "web fetch failed: " + err.Error(), At: time.Now()})
		return
	}
	req.Header.Set("User-Agent", webUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		emit(out, dropped, Observation{Channel: webChannel,
			Content: "web fetch failed: " + err.Error(), At: time.Now()})
		return
	}
	defer resp.Body.Close()
	// An error response is not a page to read from: its body is a notice, not
	// the source, so the status is recorded and the fetch ends.
	if resp.StatusCode >= 400 {
		emit(out, dropped, Observation{Channel: webChannel,
			Content: fmt.Sprintf("web fetch got status %d from %s", resp.StatusCode, url), At: time.Now()})
		return
	}
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

// skipTags name subtrees that carry the page's chrome and machinery rather than
// its text, so their whole contents are passed over.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"nav": true, "header": true, "footer": true, "aside": true,
	"form": true, "button": true, "svg": true,
}

// blockTags name elements whose end is a line break, so a paragraph or a list
// item arrives as one line rather than running into the next.
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "td": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"title": true, "section": true, "article": true, "blockquote": true,
	"figcaption": true, "pre": true,
}

// StripHTML reduces a page to its readable text lines. The page is parsed into a
// document, the main content located, the chrome and machinery subtrees passed
// over, and the remaining text gathered block by block, so an article arrives as
// its paragraphs and not its menu. The parser decodes entities; whitespace is
// collapsed and the very short fragments are dropped.
func StripHTML(page string) []string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockTags[n.Data] {
			b.WriteByte('\n')
		}
	}
	walk(mainContent(doc))
	return cleanLines(b.String())
}

// mainContent returns the element a reader would call the body of the page: the
// first <main> the page marks, else the first <article>, else the whole
// document. A page with neither still sheds its chrome through the skipped tags.
func mainContent(doc *html.Node) *html.Node {
	var main, article *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "main":
				if main == nil {
					main = n
				}
			case "article":
				if article == nil {
					article = n
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	switch {
	case main != nil:
		return main
	case article != nil:
		return article
	default:
		return doc
	}
}

// cleanLines collapses each block's whitespace, drops the very short fragments
// (menu glyphs, separators), and caps a long line.
func cleanLines(text string) []string {
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
