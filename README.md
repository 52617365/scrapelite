Scrapelite is a lightweight and undemanding scraping library for Go. It's undemanding because all it does it retrieve goquery.Documents for you.
You can do whatever you want with the resulting goquery Documents. Scrapelite is minimal and doesn't try to implement every feature.
The reason why I made this was because I was using a popular Go scraping library, and it used 4GB of ram and had a bunch of leaks. 
On top of this, I like small and lightweight. I did not need all those features. 

Usage:
```go

package main

import (
	"github.com/52617365/scrapelite"
)

func main() {
	// capturedHrefLinkFilter contains a closure that determines what scraped
	// domain links should be captured into the *goquery.Document channel
	// for later inspection
	capturedDocumentsFilter := func(url string) bool {
		return true
	}
	// hrefLinkFilter is a closure that determines a set of rules
	// that a href link has to contain for it to be captured into
	// later crawled links.
	// For example, if you're scraping facebook.com, You could add
	// a rule that specifies that links pointing to other sites
	// should be discarded.
	hrefLinkFilter := func(url string) bool {
		return true
	}

    // You are free to modify the [scrapelite.Scraper] as you want.
	s := scrapelite.New()
	s.CaptureDomainFilter = capturedDocumentsFilter
	s.CapturedHrefLinkFilter = hrefLinkFilter
    // ConcurrentRequests rate limits requests to 5 at a time.
    // Please use this to not cause trouble.
	s.ConcurrentRequests = 5
	s.Workers = 2

	s.Go("https://example.com")

    // Consuming the channel of documents. You should consume this ASAP if you don't want to lose scraped data.
    // This is because the senders are using a select and timing out after 1 - 2 seconds if nobody is receiving from channel.
    // This is done to avoid memory leaks.
	for d := range s.CapturedDomainDocuments {
			// Parse the documents here.
	}
```
