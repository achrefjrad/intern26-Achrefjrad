package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"project/models"
	"project/services"
)

type EGMScraper struct{}

type egmRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
	Page  int    `json:"page"`
}

type EGMResponse struct {
	Products   []EGMProduct `json:"products"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	TotalPages int          `json:"totalPages"`
}

type EGMProduct struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	SKU         string   `json:"sku"`
	Price       float64  `json:"price"`
	SalePrice   *float64 `json:"salePrice"`
	InStock     bool     `json:"inStock"`
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory"`
	Image       string   `json:"image"`
	Description string   `json:"description"`
	Slug        string   `json:"slug"`
}

func (e EGMScraper) ScrapeSearch(ctx context.Context, query string, resultsChan chan<- models.Product) {
	query = SanitizeQuery(query)
	rank := 0

	for page := 1; page <= 5; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[egm-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		count, hasNext := egmScrapeSearchPage(ctx, query, page, resultsChan, &rank)
		if !hasNext || count == 0 {
			break
		}
	}

	log.Printf("[egm-search] total products for %q: %d", query, rank)
}

func egmScrapeSearchPage(ctx context.Context, query string, page int, ch chan<- models.Product, rank *int) (int, bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("egm.tn", "www.egm.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*egm.tn",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	cleanQuery := strings.ToLower(strings.TrimSpace(query))
	hasNext := false
	matchedCards := 0
	pageStartRank := *rank

	c.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
		r.Headers.Set("Content-Type", "application/json")
		r.Headers.Set("Accept", "application/json")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	c.OnResponse(func(r *colly.Response) {
		var resp EGMResponse
		if err := json.Unmarshal(r.Body, &resp); err != nil {
			log.Printf("[egm-search] failed to unmarshal response: %v", err)
			return
		}

		log.Printf("[egm-search] page %d: got %d products (total=%d)", page, len(resp.Products), resp.Total)

		if resp.Total == 0 {
			log.Printf("[egm-search] Normalized query %q returned 0 products. Raw response: %s", cleanQuery, string(r.Body))
		}

		for _, prod := range resp.Products {
			matchedCards++

			name := normalizeName(cleanText(prod.Name))
			if name == "" {
				continue
			}

			productURL := ""
			if prod.Slug != "" {
				productURL = fmt.Sprintf("https://egm.tn/produit/%s", prod.Slug)
			} else {
				productURL = fmt.Sprintf("https://egm.tn/product/%d", prod.ID)
			}

			price := prod.Price
			if prod.SalePrice != nil && *prod.SalePrice > 0 {
				price = *prod.SalePrice
			}

			*rank++
			p := models.Product{
				ID:            GenerateScraperID("egm", fmt.Sprintf("%d", prod.ID)),
				Name:          name,
				Price:         price,
				PriceText:     "",
				Category:      prod.Category,
				Subcategory:   "",
				Description:   prod.Description,
				Image:         prod.Image,
				ProductURL:    productURL,
				Source:        "EGM",
				Reference:     prod.SKU,
				BaseReference: "",
				InStock:       prod.InStock,
				SearchRank:    *rank,
				Specs:         make(map[string]string),
			}

			if price > 0 {
				p.PriceText = fmt.Sprintf("%.3f TND", price)
			}

			if prod.SKU != "" {
				p.BaseReference = getBaseReference(prod.SKU)
			}

			egmDeepScrape(ctx, &p, productURL)

			select {
			case ch <- p:
			case <-ctx.Done():
			}
		}

		if page < resp.TotalPages {
			hasNext = true
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[egm-search] request error: %v (status: %d)", err, r.StatusCode)
	})

	payload := egmRequest{
		Query: cleanQuery,
		Limit: 40,
		Page:  page,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[egm-search] failed to marshal payload: %v", err)
		return 0, false
	}

	err = c.PostRaw("https://egm.tn/api/search-typesense", jsonData)
	if err != nil {
		log.Printf("[egm-search] error posting to API: %v", err)
		return 0, false
	}

	c.Wait()
	count := *rank - pageStartRank
	return count, hasNext
}

func egmDeepScrape(ctx context.Context, p *models.Product, productURL string) {
	deepC := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("egm.tn", "www.egm.tn"),
		colly.AllowURLRevisit(),
	)
	deepC.SetRequestTimeout(10 * time.Second)
	deepC.Limit(&colly.LimitRule{
		DomainGlob:  "*egm.tn",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	var breadcrumbs []string
	var catOnce sync.Once

	deepC.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	deepC.OnHTML("nav.breadcrumb ol li, .breadcrumb li, ul.breadcrumb li, nav.woocommerce-breadcrumb li, .woocommerce-breadcrumb span", func(d *colly.HTMLElement) {
		text := strings.TrimSpace(d.Text)
		if text != "" && !strings.EqualFold(text, "Accueil") && !strings.EqualFold(text, "Home") {
			breadcrumbs = append(breadcrumbs, text)
		}
	})

	deepC.OnHTML("table.woocommerce-product-attributes tr, .shop_attributes tr, .woocommerce-product-attributes tr", func(d *colly.HTMLElement) {
		key := strings.TrimSpace(d.ChildText("th"))
		if key == "" {
			return
		}
		val := strings.TrimSpace(d.ChildText("td, td p"))
		if val != "" {
			p.Specs[services.NormalizeSpecKey(key)] = val
		}
	})

	deepC.OnHTML("meta[itemprop=gtin13], meta[itemprop=gtin12]", func(d *colly.HTMLElement) {
		if p.EAN == "" {
			content := d.Attr("content")
			if ean := cleanEAN13(content); ean != "" {
				p.EAN = ean
			}
		}
	})

	deepC.OnHTML("span[itemprop=sku], .sku", func(d *colly.HTMLElement) {
		if p.Reference == "" {
			ref := cleanReference(d.Text)
			if ref != "" {
				p.Reference = ref
				p.BaseReference = getBaseReference(ref)
			}
		}
	})

	if err := deepC.Visit(productURL); err != nil {
		log.Printf("[egm-deep] error scraping %s: %v", productURL, err)
	}

	p.Subcategory = MapJardinageBricolageCategory(breadcrumbs, productURL)
	catOnce.Do(func() {
		log.Printf("[egm-cat] %q -> subcategory=%s (breadcrumbs=%v)", p.Name, p.Subcategory, breadcrumbs)
	})

	services.EnrichProduct(p)
}

func SyncEGM(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("egm.tn", "www.egm.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".out-of-stock, .stock.out-of-stock, button[disabled]", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "epuise") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") || strings.Contains(text, "hors stock") {
			result.InStock = false
		}
	})

	c.OnHTML("meta[itemprop=price]", func(e *colly.HTMLElement) {
		content := e.Attr("content")
		if content != "" {
			if v, err := strconv.ParseFloat(content, 64); err == nil && v > 0 {
				result.Price = v
				result.PriceText = fmt.Sprintf("%.3f TND", v)
				found = true
			}
		}
	})

	c.OnHTML("span.price, .price .woocommerce-Price-amount, .product-price", func(e *colly.HTMLElement) {
		if !found {
			raw, _ := ExtractPrice(e.Text)
			if raw > 0 {
				result.Price = raw
				result.PriceText = e.Text
				found = true
			}
		}
	})

	err := c.Visit(original.ProductURL)
	if err != nil {
		log.Printf("[sync-egm] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-egm] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-egm] synced %s (%s): %.3f TND inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}


