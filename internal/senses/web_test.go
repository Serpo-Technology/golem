package senses

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// A fetch identifies itself with a descriptive user-agent and reads the page's
// visible text in line by line, so a polite source serves its content.
func TestWebFetchSendsUserAgentAndReadsText(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, "<html><body><p>The cat sat on the mat.</p><p>Cats are mammals.</p></body></html>")
	}))
	defer srv.Close()

	out := make(chan Observation, 16)
	var dropped atomic.Int64
	fetchOne(&http.Client{}, srv.URL, out, &dropped)
	close(out)

	var lines []string
	for o := range out {
		if o.Channel != webChannel {
			t.Fatalf("web text should arrive on the web channel, got %q", o.Channel)
		}
		lines = append(lines, o.Content)
	}
	if !strings.Contains(gotUA, "GOLEM") {
		t.Fatalf("the fetch should send a descriptive user-agent, got %q", gotUA)
	}
	if len(lines) != 2 || lines[0] != "The cat sat on the mat." {
		t.Fatalf("the page text should arrive as lines: %v", lines)
	}
}

// The parser keeps the article and sheds the chrome: a page's main content is
// read while its nav, header, footer, and scripts are passed over.
func TestStripHTMLExtractsContentNotChrome(t *testing.T) {
	page := `<html><head><title>Cat</title><style>.x{color:red}</style></head><body>
		<nav><a>Home</a><a>Menu</a></nav>
		<header>Site banner here</header>
		<main>
			<h1>Cats</h1>
			<p>A cat is a small animal that people keep.</p>
			<p>Cats purr when they are content.</p>
		</main>
		<footer>Copyright notice text</footer>
		<script>trackUser()</script>
	</body></html>`

	joined := strings.Join(StripHTML(page), " | ")
	if !strings.Contains(joined, "A cat is a small animal that people keep.") ||
		!strings.Contains(joined, "Cats purr when they are content.") {
		t.Fatalf("the article text should be extracted: %q", joined)
	}
	for _, chrome := range []string{"Home", "Menu", "Site banner", "Copyright", "trackUser", "color:red"} {
		if strings.Contains(joined, chrome) {
			t.Fatalf("chrome %q should be dropped: %q", chrome, joined)
		}
	}
}

// An error response is a notice, not a page: its body is never read as content,
// only the status is recorded.
func TestWebFetchSkipsErrorResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<html><body><p>forbidden secret text</p></body></html>")
	}))
	defer srv.Close()

	out := make(chan Observation, 16)
	var dropped atomic.Int64
	fetchOne(&http.Client{}, srv.URL, out, &dropped)
	close(out)

	for o := range out {
		if strings.Contains(o.Content, "secret") {
			t.Fatalf("an error page must not be read as content: %q", o.Content)
		}
		if !strings.Contains(o.Content, "status 403") {
			t.Fatalf("the status should be noted instead: %q", o.Content)
		}
	}
}
