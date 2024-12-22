package scrapelite

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"net/url"

	"time"
)

// HttpClient this interface is created to test the whole
// scraping functionality without sending an actual request.
// The main function does *http.Client.Get and in tests
// we obviously don't want to do that.
type HttpClient interface {
	Get(url string) (resp *http.Response, err error)
}
type allowedDomainCallBack func(url string) bool
type Scraper struct {
	capturedHrefLinkFilter  allowedDomainCallBack
	captureDomainFilter     allowedDomainCallBack
	hrefLinks               chan string
	scrapeReady             chan struct{}
	capturedDomainDocuments chan *goquery.Document

	httpClient HttpClient
}

func New() *Scraper {
	c := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
	s := &Scraper{httpClient: c, hrefLinks: make(chan string), capturedDomainDocuments: make(chan *goquery.Document)}
	return s
}

func (s *Scraper) SetCapturedDocumentsFilter(allowedDomainFilter allowedDomainCallBack) *Scraper {
	s.captureDomainFilter = allowedDomainFilter
	return s
}

func (s *Scraper) SetHrefLinkCaptureFilter(allowedDomainFilter allowedDomainCallBack) *Scraper {
	s.capturedHrefLinkFilter = allowedDomainFilter
	return s
}

func (s *Scraper) SetHttpClient(httpClient *http.Client) *Scraper {
	s.httpClient = httpClient
	return s
}

func (s *Scraper) Go(baseUrl string) {
	parsedBaseUrl, err := url.Parse(baseUrl)
	if err != nil {
		log.Fatalln("Error parsing initial url: ", parsedBaseUrl)
	}
	go func() {
		// Adding the initial urls into the hrefLinks chan so that
		// we can start from somewhere. Else the chan would be empty,
		// and we would be blocking forever in the receiving side
		// which is the hot loop in this function.
		s.hrefLinks <- baseUrl
	}()
	for i := 0; i < 15; i++ {
		go s.ScrapeDocumentsAndHrefLinks(parsedBaseUrl)
	}
}
func (s *Scraper) Wait() {
	<-s.scrapeReady
}

// ScrapeDocumentsAndHrefLinks scrapes the initial url
// and creates more links to scrape by worming through
// the page and its various a[href] links.
// The user can provide a closure that is matched on the urls
// to make sure that the correct urls are being stored.
func (s *Scraper) ScrapeDocumentsAndHrefLinks(baseUrl *url.URL) {
	for l := range s.hrefLinks {
		// Here we are sending the link back to the channel
		// because the link we're using to crawl is also going
		// to be used by the HTML parser that will be receiving
		// from the hrefLinks channel. AKA we don't want to
		// get rid of it forever. We send it from its own
		// goroutine to avoid blocking the main goroutine
		// thread in this comment scope.
		go func() {
			// Checking if no filter set first to not cause
			// a nil reference
			if s.capturedHrefLinkFilter == nil || s.capturedHrefLinkFilter(l) {
				s.hrefLinks <- l
			}
		}()
		fmt.Println("Visiting:", l)
		r, err := s.httpClient.Get(l)
		if err != nil {
			log.Println(err)
			return
		}
		d, err := goquery.NewDocumentFromReader(r.Body)
		if err != nil {
			log.Println(err)
			return
		}
		// Checking if no filter set first to not cause
		// a nil reference
		if s.captureDomainFilter == nil || s.captureDomainFilter(l) {
			go func() {
				select {
				case s.capturedDomainDocuments <- d:
				case <-time.After(5 * time.Second):
					log.Fatalln("Channel send to capturedDomainDocuments blocked for 5 seconds. Something is wrong with your code. Make sure you receive the documents.")
				}
			}()
		}
		d.Find("a").Each(func(i int, sel *goquery.Selection) {
			href, ok := sel.Attr("href")
			if !ok {
				return
			}
			hrefUrl, err := url.Parse(href)
			if err != nil {
				log.Println(err)
				return
			}
			// a contains the absolute url from the href.
			// we do this by combining the initial url
			// with the href.
			a := baseUrl.ResolveReference(hrefUrl)

			// Here we capture the a[href] and put
			// it to the hrefLinks channel which will
			// be consumed by both the href crawler
			// goroutines and also the goroutines
			// that parse the HTML contents and
			// extract useful data from the page
			go func() {
				// Checking if no filter set first to not cause
				// a nil reference
				if s.capturedHrefLinkFilter == nil || s.capturedHrefLinkFilter(a.String()) {
					s.hrefLinks <- a.String()
				}
			}()
		})
	}
	s.scrapeReady <- struct{}{}
}
