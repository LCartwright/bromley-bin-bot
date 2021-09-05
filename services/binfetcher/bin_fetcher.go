package binfetcher

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"
)

type BinFetcherer interface {
	UpdateBins()
	NextCollection() *BinCollections
}

type BinFetcher struct {
	rwmu        sync.RWMutex
	fetchURL    bool
	collections *BinCollections
}

func (b *BinFetcher) UpdateBins() {
	b.rwmu.Lock()
	defer b.rwmu.Unlock()
	b.update()
}

func (b *BinFetcher) NextCollection() *BinCollections {
	b.rwmu.RLock()
	defer b.rwmu.RUnlock()
	return b.collections
}

type BinCollections struct {
	Date            time.Time
	CollectionTypes []string
}

func NewBinFetcher(fetchURL bool) *BinFetcher {
	return &BinFetcher{
		fetchURL: fetchURL,
	}
}

type binfo struct {
	Heading            string
	Frequency          string
	NextCollection     string
	NextCollectionTime time.Time
	LastCollection     string
}

var (
	logger, _ = zap.NewDevelopment()
)

func (b *BinFetcher) update() {
	logger.Info("Fetching bin update")
	var file io.ReadCloser
	var err error
	if b.fetchURL {
		logger.Info("Fetching file from URL")
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		req, err := http.NewRequest("GET", "https://recyclingservices.bromley.gov.uk/waste/8151525", nil)
		if err != nil {
			logger.Fatal("error creating request", zap.Error(err))
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			logger.Fatal("error creating request", zap.Error(err))
		}
		file = resp.Body
	} else {
		logger.Info("Fetching file from local")
		file, err = os.Open("resources/bins.html")
	}

	if err != nil {
		logger.Fatal("error opening file", zap.Error(err))
	}

	defer func() {
		err := file.Close()
		if err != nil {
			logger.Error("error closing file", zap.Error(err))
		}
	}()
	binsHTML, err := goquery.NewDocumentFromReader(file)
	if err != nil {
		logger.Fatal("error reading file", zap.Error(err))
	}
	allBins := make([]*binfo, 0)
	binsHTML.Find("body > div > div > div.container > div > div.waste__collections > div > div.govuk-grid-column-two-thirds > h3").Each(func(i int, selection *goquery.Selection) {
		sectionName := selection.Text()
		next := selection.Next().ChildrenFiltered("div:nth-child(2)").ChildrenFiltered("dl").ChildrenFiltered("div")
		b := &binfo{}
		b.Heading = sectionName
		next.Each(func(i2 int, selection2 *goquery.Selection) {
			dt := strings.TrimSpace(selection2.ChildrenFiltered("dt").Clone().Children().Remove().End().Text())
			dd := strings.TrimSpace(selection2.ChildrenFiltered("dd").Clone().Children().Remove().End().Text())
			key := strings.ToLower(dt)
			switch key {
			case "frequency":
				b.Frequency = dd
			case "next collection":
				b.NextCollection = strings.TrimSpace(dd)
				b.NextCollectionTime = getTimeFromDateStr(b.NextCollection)
			case "last collection":
				b.LastCollection = dd
			default:
			}
		})
		allBins = append(allBins, b)
	})

	collections := make(map[time.Time]*BinCollections, 0)

	for _, b := range allBins {
		if b.NextCollectionTime.IsZero() {
			continue
		}
		c, ok := collections[b.NextCollectionTime]
		if !ok {
			c = &BinCollections{
				Date: b.NextCollectionTime,
			}
		}
		c.CollectionTypes = append(c.CollectionTypes, b.Heading)
		sort.Strings(c.CollectionTypes)
		collections[c.Date] = c
	}

	var nextCollection = &BinCollections{}
	for k, v := range collections {
		if nextCollection.Date.IsZero() || k.Before(nextCollection.Date) {
			nextCollection = v
			continue
		}
	}
	b.collections = nextCollection
}

func getTimeFromDateStr(dateStr string) time.Time {
	r := regexp.MustCompile(`(\w{6,9}),\s(\d{1,2})\w{1,2}\s(\w{3,9})`)
	matches := r.FindStringSubmatch(dateStr)
	if len(matches) != 4 {
		return time.Time{}
	}
	t, err := time.Parse("2 January 2006", fmt.Sprintf("%s %s %d", matches[2], matches[3], time.Now().Year()))
	if err != nil {
		return time.Time{}
	}
	return t
}
