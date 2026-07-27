package scrapers

import (
	"context"
	"log"

	"project/models"
)

type ScraperFunc func(ctx context.Context, query string, ch chan<- models.Product) error

var Registry []ScraperFunc

func init() {
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		TunisianetSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		MyTechSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		WikiSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		SBSSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		BeautyStoreSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		FatalesSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		BigDepotSearchScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})
	Registry = append(Registry, func(ctx context.Context, query string, ch chan<- models.Product) error {
		EGMScraper{}.ScrapeSearch(ctx, query, ch)
		return nil
	})

	log.Printf("[registry] registered %d scrapers", len(Registry))
}
