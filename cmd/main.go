package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/sakithb/bikez-scraper/internal"
)

func main() {
	a := os.Args[1]

	if strings.Contains(a, "c") {
		fmt.Println("Running category scraper")
		s := internal.NewCategoryScraper()
		s.Start(8)
	}
	
	if strings.Contains(a, "b") {
		fmt.Println("Running bike scraper")
		s := internal.NewBikeScraper()
		s.Start(8)
	}
}
