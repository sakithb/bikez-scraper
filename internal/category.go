package internal

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type CategoryScraper struct {
	inputs  chan string
	results chan string
	cwg     sync.WaitGroup
	wwg     sync.WaitGroup
}

func NewCategoryScraper() *CategoryScraper {
	s := CategoryScraper{}
	s.inputs = make(chan string, 15)
	s.results = make(chan string, 15)
	s.cwg = sync.WaitGroup{}
	s.wwg = sync.WaitGroup{}

	return &s
}

func (s *CategoryScraper) Start(workers int) {
	go s.Writer()

	for range workers {
		go s.Scraper()
	}

	for i := range 15 {
		s.inputs <- fmt.Sprintf("https://bikez.com/category/index.php?category=%d", i+1)
	}

	s.cwg.Add(15)
	s.wwg.Add(workers + 1)

	s.cwg.Wait()

	close(s.inputs)
	close(s.results)

	s.wwg.Wait()
}

func (s *CategoryScraper) Writer() {
	defer s.wwg.Done()

	f, err := os.Create("bikes.list")
	if err != nil {
		panic(err)
	}

	defer f.Close()

	count := 0
	start := time.Now().Unix()
	m := 0

	for r := range s.results {
		_, err := f.WriteString(r + "\n")
		if err != nil {
			panic(err)
		}

		count++
		speed := float64(count) / float64((time.Now().Unix() - start))

		line := fmt.Sprintf("%d URLs scraped - %.1f URLS/s", count, speed)
		
		if len(line) > m {
			m = len(line)
		}

		fmt.Printf("\r%-*v", m, line)
	}

	err = f.Sync()
	if err != nil {
		panic(err)
	}

	fmt.Print("\n")
}

func (s *CategoryScraper) Scraper() {
	defer s.wwg.Done()

	cl := colly.NewCollector()

	cl.OnHTML("table.zebra>tbody>tr>td:first-child>a:only-child:not(:has(*))", func(h *colly.HTMLElement) {
		s.results <- h.Request.AbsoluteURL(h.Attr("href"))
	})

	cl.OnHTML("table.zebra>tbody>tr:last-child>td>*:last-child", func(h *colly.HTMLElement) {
		if h.Name == "a" {
			s.inputs <- h.Request.AbsoluteURL(h.Attr("href"))
		} else {
			s.cwg.Done()
		}
	})

	for url := range s.inputs {
		cl.Visit(url)
		cl.Wait()
	}
}
