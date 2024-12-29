package scrapelite

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// CustomHttpClient this interface is created to test the whole
// scraping functionality without sending an actual request.
// The main function does *http.Client.Get and in tests
// we obviously don't want to do that.
type CustomHttpClient interface {
	Get(url string) (resp *http.Response, err error)
}

type (
	// allowedDomainCallBack is the type of function that will be used
	// to check if a domain should be visited in certain cases.
	// it contains conditional/s that will be matched.
	allowedDomainCallBack func(url string) bool
	Scraper               struct {
		capturedHrefLinkFilter  allowedDomainCallBack
		captureDomainFilter     allowedDomainCallBack
		HrefLinks               chan string
		scrapeReady             chan struct{}
		CapturedDomainDocuments chan *goquery.Document
		workers                 int
		// visitedUrls will be populated with contents
		// if the user decides to use VisitDuplicates.
		// It is simply a map that contains an url as
		// a key and then a boolean that is set to true
		// if said url is already visited.
		visitedUrls sync.Map

		// visitedUrlsChan is the internal channel that is
		// used to sync the visited urls into the VisitedUrls map
		// visitedUrlsChan chan string

		// visitDuplicates if this is set to true, duplicate links will be
		// visited. The default value is false because I don't really want to scrape
		// the same sites multiple times.
		visitDuplicates bool

		// showVisitingMessages contains a boolean that if set will print out
		// every website that is being visited. This is handy in debugging
		// situations and I usually keep it on.
		showVisitingMessages bool

		// verbose provides some nice printing to diagnose issues.
		verbose bool

		httpClient CustomHttpClient
	}
)

func New() *Scraper {
	c := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
	s := &Scraper{httpClient: c, HrefLinks: make(chan string), CapturedDomainDocuments: make(chan *goquery.Document), scrapeReady: make(chan struct{}), visitedUrls: sync.Map{}}
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

func (s *Scraper) SetVerbose(v bool) *Scraper {
	s.verbose = v
	return s
}

func (s *Scraper) SetHttpClient(httpClient *http.Client) *Scraper {
	s.httpClient = httpClient
	return s
}

func (s *Scraper) SetWorkerAmount(workers int) *Scraper {
	s.workers = workers
	return s
}

func (s *Scraper) SetVisitDuplicates(b bool) *Scraper {
	s.visitDuplicates = b
	return s
}

func (s *Scraper) SetVisitingMessages(b bool) *Scraper {
	s.showVisitingMessages = b
	return s
}

func (s *Scraper) verbosePrint(message string) {
	_, file, line, _ := runtime.Caller(0)
	fmt.Printf("[VERBOSE %s:%d]: %s\n", file, line, message)
}

func (s *Scraper) Go(baseUrl string) {
	parsedBaseUrl, err := url.Parse(baseUrl)
	if err != nil {
		log.Fatalln("Error parsing initial url: ", parsedBaseUrl)
	}

	if s.verbose {
		s.verbosePrint(fmt.Sprintf("Started scraper with base url: %s", baseUrl))
	}

	go func() {
		// Adding the initial urls into the HrefLinks chan so that
		// we can start from somewhere. Else the chan would be empty,
		// and we would be blocking forever in the receiving side
		// which is the hot loop in this function.
		s.HrefLinks <- baseUrl
		s.verbosePrint(fmt.Sprintf("Added %s to the s.HrefLinks channel queue", baseUrl))
	}()

	if s.workers == 0 {
		if s.verbose {
			s.verbosePrint("workers was set to 0 it was reassigned to 1")
		}
		s.workers = 1
	}

	for i := 0; i < s.workers; i++ {
		go s.ScrapeDocumentsAndHrefLinks(parsedBaseUrl)
	}
	if s.verbose {
		s.verbosePrint(fmt.Sprintf("Created %d workers for scraper", s.workers))
	}
}

func (s *Scraper) Wait() {
	<-s.scrapeReady
}

// syncVisitedUrls note that this solution is not bullet-proof
// because a goroutine could be reading an "outdated" map of visited urls.
// if we wanted this to be bullet-proof we would be using a RW Mutex but
// using that would slow down the whole thing by a lot so I decided to be
// mediocre with the visited urls by using a channel.
// func (s *Scraper) syncVisitedUrls() {
// 	for u := range s.visitedUrlsChan {
// 		s.VisitedUrls[u] = true
// 	}
// }

func (s *Scraper) isVisitedUrl(url string) bool {
	if s.verbose {
		s.verbosePrint(fmt.Sprintf("Trying to take lock to load %s from the visitedUrls map", url))
	}
	_, ok := s.visitedUrls.Load(url)
	if s.verbose {
		s.verbosePrint(fmt.Sprintf("Successfully took the lock and got the result %s:%v", url, ok))
	}
	return ok
}

func (s *Scraper) addVisitedUrl(url string) {
	if s.verbose {
		s.verbosePrint(fmt.Sprintf("Trying to take the lock to add %s to the visitedUrls map", url))
	}
	s.visitedUrls.Store(url, true)
	if s.verbose {
		s.verbosePrint(fmt.Sprintf("Successfully added %s to the visitedUrls map", url))
	}
}

// ScrapeDocumentsAndHrefLinks scrapes the initial url
// and creates more links to scrape by worming through
// the page and its various a[href] links.
// The user can provide a closure that is matched on the urls
// to make sure that the correct urls are being stored.
func (s *Scraper) ScrapeDocumentsAndHrefLinks(baseUrl *url.URL) {
	for l := range s.HrefLinks {
		// Wrapping the whole loop iteration in a function
		// to form a "scope" to defer close the res body
		// correctly. I did this because I was running into
		// memory leaking issues and could not really figure
		// out why it was happening. I got the solution from
		// here: https://stackoverflow.com/questions/45617758/proper-way-to-release-resources-with-defer-in-a-loop
		func() {
			if s.showVisitingMessages {
				fmt.Println("Visiting:", l)
			}

			r, err := s.httpClient.Get(l)
			if err != nil {
				log.Println(err)
				return
			}
			if s.verbose {
				s.verbosePrint(fmt.Sprintf("We sent a request to %s and received status %d", l, r.StatusCode))
			}
			defer r.Body.Close()

			if !s.visitDuplicates {
				if s.verbose {
					s.verbosePrint(fmt.Sprintf("Added %s to the cache of visited urls", l))
				}
				s.addVisitedUrl(l)
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
					case s.CapturedDomainDocuments <- d:
						if s.verbose {
							s.verbosePrint(fmt.Sprintf("Sent %v to documents channel", l))
						}
					case <-time.After(1 * time.Second):
						if s.verbose {
							s.verbosePrint(fmt.Sprintf("Tried to send %v to documents channel but it timed out", l))
						}
					}
				}()
			}

			d.Find("a").Each(func(i int, sel *goquery.Selection) {
				href, ok := sel.Attr("href")
				if !ok {
					if s.verbose {
						s.verbosePrint("Tried to get the href attribute from the a selector but it failed, we are now going to return")
					}
					return
				}
				if s.verbose {
					s.verbosePrint(fmt.Sprintf("We got the %s href selector from the a tag successfully", href))
				}

				hrefUrl, err := url.Parse(href)
				if err != nil {
					if s.verbose {
						s.verbosePrint(fmt.Sprintf("Tried to parse %s but we got an error", href))
					}
					log.Println(err)
					return
				}
				if s.verbose {
					s.verbosePrint(fmt.Sprintf("Successfully parsed %s into an url.Url", href))
				}
				// a contains the absolute url from the href.
				// we do this by combining the initial url
				// with the href.
				a := baseUrl.ResolveReference(hrefUrl)

				// If the url is already visited, let's not add it to the s.HrefLinks queue.
				if !s.visitDuplicates {
					if s.isVisitedUrl(a.String()) {
						if s.verbose {
							s.verbosePrint(fmt.Sprintf("%s was already visited so we skipped it completely", a.String()))
						}
						return
					}
				}
				// Here we capture the a[href] and put
				// it to the HrefLinks channel which will
				// be consumed by this same loop.
				go func() {
					// Checking if no filter set first to not cause
					// a nil reference
					if s.capturedHrefLinkFilter == nil || s.capturedHrefLinkFilter(a.String()) {
						select {
						case s.HrefLinks <- a.String():
							if s.verbose {
								s.verbosePrint(fmt.Sprintf("Added %s to the channel of href links", a.String()))
							}
						case <-time.After(1 * time.Second):
							if s.verbose {
								s.verbosePrint(fmt.Sprintf("Tried to send %s to HrefLinks channel but it timed out", a.String()))
							}
						}
					}
				}()
			})
		}()
	}
	s.scrapeReady <- struct{}{}

	if s.verbose {
		s.verbosePrint("Successfully notified the scrapeReady channel that we are ready scraping")
	}
	close(s.scrapeReady)
	close(s.HrefLinks)
	close(s.CapturedDomainDocuments)
	if s.verbose {
		s.verbosePrint("Successfully closed the scrapeReady, Hreflinks, CapturedDomainDocuments channels")
	}
}
