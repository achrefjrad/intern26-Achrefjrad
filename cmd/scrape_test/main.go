package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"project/models"
	"project/scrapers"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run cmd/scrape_test/main.go <query>\nExample: go run cmd/scrape_test/main.go tondeuse")
	}
	query := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ch := make(chan models.Product, 100)
	var results []models.Product

	go func() {
		scrapers.RunSearchScrapersStream(ctx, query, ch, 8)
		close(ch)
	}()

	for p := range ch {
		results = append(results, p)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SearchRank < results[j].SearchRank
	})

	fmt.Printf("\n=== Scrape Test Results for %q ===\n", query)
	fmt.Printf("Total products: %d\n\n", len(results))

	for _, p := range results {
		fmt.Printf("  [%s] %s\n", p.Source, p.Name)
		fmt.Printf("         Price: %.3f TND\n", p.Price)
		fmt.Printf("         Category: %s\n", p.Category)
		fmt.Printf("         Subcategory: %s\n", p.Subcategory)
		if p.Reference != "" {
			fmt.Printf("         Ref: %s\n", p.Reference)
		}
		if p.InStock {
			fmt.Printf("         In stock: yes\n")
		} else {
			fmt.Printf("         In stock: no\n")
		}
		if len(p.Specs) > 0 {
			fmt.Printf("         Specs:\n")
			keys := make([]string, 0, len(p.Specs))
			for k := range p.Specs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("           %s: %s\n", k, p.Specs[k])
			}
		}
		fmt.Println()
	}

	fmt.Printf("=== End (%d products) ===\n", len(results))
}
