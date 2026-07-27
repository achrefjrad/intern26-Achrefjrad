package scrapers

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"

	"project/models"
)

func SanitizeQuery(query string) string {
	s := strings.TrimSpace(query)
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}

func RunSearchScrapersStream(ctx context.Context, query string, ch chan<- models.Product, limitN int) {
	q := SanitizeQuery(query)
	var wg sync.WaitGroup

	n := limitN
	if n > len(Registry) {
		n = len(Registry)
	}

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(fn ScraperFunc) {
			defer wg.Done()
			if err := fn(ctx, q, ch); err != nil {
				log.Printf("[manager] scraper error: %v", err)
			}
		}(Registry[i])
	}

	wg.Wait()
}
