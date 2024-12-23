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
	s := scrapelite.New()
	s.SetCapturedDocumentsFilter(func(url string) bool {
		// capturedHrefLinkFilter contains a closure that determines what scraped
		// domain links should be captured into the *goquery.Document channel
		// for later inspection
		return true
	})

	s.SetHrefLinkCaptureFilter(func(url string) bool {
		// hrefLinkFilter is a closure that determines a set of rules
		// that a href link has to contain for it to be captured into
		// later crawled links.
		// For example, if you're scraping facebook.com, You could add
		// a rule that specifies that links pointing to other sites
		// should be discarded.
		return true
	})
	s.Go("https://example.com")

	// Wait for the scraper to finish or alternatively consume the channels in real-time
	// NOTE: if you don't consume the documents fast enough, there will be a memory leak 
    // with goroutines because sending to a channel from a new goroutine creates a stack
    // for the goroutine etc. which takes up space and that goroutine is blocking on send.
	// I'm going to think about some good solution for this. Personally I just make sure
    // there are more receiving goroutines than sending ones.
	for d := range s.CapturedDomainDocuments {
			// Parse the documents here.
	}
```
