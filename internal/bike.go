package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type BikeScraper struct {
	inputs  chan string
	results chan Bike
	bwg     sync.WaitGroup
	wwg     sync.WaitGroup
}

type Bike struct {
	Model    string `json:"model"`
	Brand    string `json:"brand"`
	Year     int    `json:"year"`
	Category string `json:"type"`
}

type InputBike struct {
	Model    string `json:"model"`
	Category string `json:"category"`

	Brand struct {
		Name string `json:"name"`
	} `json:"brand"`

	Engine struct {
		Displacement struct {
			Value string `json:"value"`
		} `json:"engineDisplacement"`
	} `json:"vehicleEngine"`
}

func NewBikeScraper() *BikeScraper {
	s := BikeScraper{}
	s.inputs = make(chan string, 10)
	s.results = make(chan Bike, 10)
	s.bwg = sync.WaitGroup{}
	s.wwg = sync.WaitGroup{}

	return &s
}

func (s *BikeScraper) Start(workers int) {
	go s.Writer()

	for range workers {
		go s.Scraper()
	}

	s.wwg.Add(workers + 1)

	f, err := os.Open("bikes.list")
	if err != nil {
		panic("Could not open bikes.list")
	}

	defer f.Close()

	sc := bufio.NewReader(f)

	for {
		line, err := sc.ReadString('\n')
		if err != nil && err != io.EOF {
			panic("Error while reading bikes.list")
		}

		if len(line) == 0 {
			panic("Invalid line")
		}

		s.bwg.Add(1)
		s.inputs <- line[:len(line)-1]

		if err == io.EOF {
			break
		}
	}

	s.bwg.Wait()

	close(s.inputs)
	close(s.results)

	s.wwg.Wait()
}

func (s *BikeScraper) Writer() {
	defer s.wwg.Done()

	f, err := os.Create("models.json")
	if err != nil {
		panic(err)
	}

	defer f.Close()

	_, err = f.Write([]byte{'['})
	if err != nil {
		panic(err)
	}

	count := 0
	start := time.Now().Unix()
	m := 0

	for b := range s.results {
		j, err := json.Marshal(b)
		if err != nil {
			panic(err)
		}

		_, err = f.Write(append(j, ','))
		if err != nil {
			panic(err)
		}

		count++
		speed := float64(count) / float64((time.Now().Unix() - start))
		line := fmt.Sprintf("\r%d Bikes scraped - %.1f URLS/s", count, speed)

		if len(line) > m {
			m = len(line)
		}

		fmt.Printf("\r%-*v", m, line)
	}

	f.Seek(-1, 1)

	_, err = f.Write([]byte{']'})
	if err != nil {
		panic(err)
	}

	err = f.Sync()
	if err != nil {
		panic(err)
	}

	fmt.Printf("\r%05d Bikes scraped - Done      \n", count)
}

func (s *BikeScraper) Scraper() {
	defer s.wwg.Done()

	cl := colly.NewCollector()
	skip := false

	cl.OnHTML("script[type='application/ld+json']", func(h *colly.HTMLElement) {
		if !strings.HasPrefix(h.Text, " \n{\"@type\":\"M") {
			skip = !strings.Contains(h.Text, "aggregateRating")
			return
		}

		defer s.bwg.Done()

		if skip {
			return
		}

		i := InputBike{}

		t := []byte(h.Text)
		for i, v := range t {
			if v == '\n' || v == '\t' {
				t[i] = ' '
			}
		}

		err := json.Unmarshal(t, &i)
		if err != nil {
			panic(err.Error() + " " + h.Request.URL.String())
		}

		if i.Model == "" || i.Brand.Name == "" || i.Category == "" || i.Engine.Displacement.Value == "" {
			return
		}

		e, err := strconv.ParseFloat(i.Engine.Displacement.Value, 64)
		if err != nil {
			panic(err.Error() + " " + h.Request.URL.String())
		}

		d := int(math.Round(e/10) * 10)

		if d < 150 {
			return
		}

		bef, aft, _ := strings.Cut(i.Model, " ")

		y, err := strconv.Atoi(bef)
		if err != nil {
			panic(err.Error() + " " + h.Request.URL.String())
		}

		b := Bike{}

		b.Model = aft
		b.Brand = i.Brand.Name
		b.Year = y
		b.Category = strings.ToLower(i.Category)

		s.results <- b
	})

	for url := range s.inputs {
		cl.Visit(url)
		cl.Wait()
	}
}
