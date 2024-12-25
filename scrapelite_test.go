package scrapelite

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestNewScraper(t *testing.T) {
	tests := []struct {
		name string
		want *Scraper
	}{
		{name: "Test scraper default values are fine", want: &Scraper{capturedHrefLinkFilter: nil, HrefLinks: make(chan string), CapturedDomainDocuments: make(chan *goquery.Document)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New()
			assert.Nil(t, got.capturedHrefLinkFilter)
			assert.NotNil(t, got.CapturedDomainDocuments)
			assert.NotNil(t, got.HrefLinks)
		})
	}
}

func TestScraper_InitWithValues(t *testing.T) {
	var allowedDomain allowedDomainCallBack = func(url string) bool {
		return true
	}

	wantScraper := &Scraper{capturedHrefLinkFilter: allowedDomain, HrefLinks: nil}

	s := New()

	s.SetCapturedDocumentsFilter(wantScraper.capturedHrefLinkFilter)
	s.SetHrefLinkCaptureFilter(wantScraper.capturedHrefLinkFilter)

	assert.NotNil(t, s.capturedHrefLinkFilter)
	assert.NotNil(t, s.captureDomainFilter)
}

var TestBaseUrl = "https://example.com"

var TestHtml = `
	<!DOCTYPE html>
	<html>
	<body>
	   <a href="https://example.com">Home</a>
	   <a href="/relative/path">Relative Link</a>
	   <a href="https://google.com">Google</a>
	   <div class="nested">
		   <a href="../parent/path">Parent Path</a>
		   <a>No Href</a>
	   </div>
	</body>
	</html>
`

// MockHttpClient is a mock *http.Client
// created to return a local HTML file instead
// of retrieving the HTML from an external source.
type MockHttpClient struct {
}

func (m *MockHttpClient) Get(url string) (resp *http.Response, err error) {
	// h contains example HTML for testing purposes.
	res := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(TestHtml))}
	return res, nil
}

func TestScraperGetHrefs(t *testing.T) {
	os.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	s := New()
	s.httpClient = &MockHttpClient{}

	s.Go(TestBaseUrl)

	// Waiting until the first document arrives because
	// we're not interested in testing crawling.
	select {
	case l := <-s.HrefLinks:
		// We have an array of candidates because
		// we don't guarantee the order of the
		// hrefs in the channel. On top of
		// this, the order does not really matter
		// only the fact that they exist matter.
		// They will all be crawled eventually either way.
		candidates := []string{"https://example.com", "https://google.com", "https://example.com/relative/path", "https://example.com/parent/path"}
		var found bool
		for _, c := range candidates {
			if c == l {
				found = true
			}
		}
		assert.True(t, found, "expected to find a matching link")
	}
}

func TestScraperGetDocuments(t *testing.T) {
	os.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	s := New()
	s.httpClient = &MockHttpClient{}

	s.Go(TestBaseUrl)

	// Waiting until the first document arrives because
	// we're not interested in testing crawling.
	for i := 0; i < 10000; i++ {

		select {
		case l := <-s.CapturedDomainDocuments:
			reader := strings.NewReader(TestHtml)
			d, _ := goquery.NewDocumentFromReader(reader)
			assert.Equal(t, l, d)
		}
	}
}
