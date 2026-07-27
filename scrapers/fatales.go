package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"project/models"
	"project/services"
	"project/types"
)

const fatalesSearchBase = "https://www.fatales.tn/recherche"
const fatalesMediaBase = "https://www.fatales.tn"

type FatalesSearchScraper struct{}

func (f FatalesSearchScraper) ScrapeSearch(ctx context.Context, query string, resultsChan chan<- models.Product) {
	query = SanitizeQuery(query)
	encodedQuery := neturl.QueryEscape(query)
	emptyCount := 0
	rank := 0

	for page := 1; page <= 3; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[fatales-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		searchURL := fatalesSearchBase + "?controller=search&s=" + encodedQuery
		if page > 1 {
			searchURL += "&page=" + strconv.Itoa(page)
		}

		log.Printf("[fatales-search] scraping page %d: %s", page, searchURL)
		pageProducts, hasNext := scrapeFatalesSearchPage(ctx, searchURL, resultsChan, &rank)

		if !hasNext {
			break
		}
		if pageProducts == 0 {
			emptyCount++
			if emptyCount >= 2 {
				log.Printf("[fatales-search] stopping after %d consecutive empty pages", emptyCount)
				break
			}
			continue
		}
		emptyCount = 0
	}

	log.Printf("[fatales-search] total products for %q: %d (ranks assigned)", query, rank)
}

type fatalesProductData struct {
	Reference string            `json:"reference"`
	Ean13     string            `json:"ean13"`
	Features  []fatalesFeature `json:"features"`
}

type fatalesFeature struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func fatalesParsePrice(raw string) (float64, string) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "TND")
	s = strings.TrimSuffix(s, "DT")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ""
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, ""
	}
	return v, raw
}

func scrapeFatalesSearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (pageProductCount int, hasNext bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.fatales.tn", "fatales.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)

	visited := make(map[string]bool)
	skippedCount := 0
	pageStartRank := *rank

	c.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})

	c.OnHTML("a[rel=next]", func(e *colly.HTMLElement) {
		hasNext = true
	})

	c.OnHTML("article.product-miniature.js-product-miniature", func(e *colly.HTMLElement) {
		productURL := e.ChildAttr("a.product_img_link", "href")
		if productURL == "" {
			productURL = e.ChildAttr("a.product-thumbnail", "href")
		}
		if productURL == "" {
			productURL = e.ChildAttr("a.thumbnail", "href")
		}
		if productURL == "" {
			return
		}
		normalized := normalizeURL(productURL)
		if visited[normalized] {
			skippedCount++
			return
		}
		visited[normalized] = true

		name := cleanText(e.ChildText("h2[itemprop=name] a.product-name"))
		if name == "" {
			name = cleanText(e.ChildText("h2.product-title a, h3.product-title a"))
		}
		if name == "" {
			return
		}
		name = normalizeName(name)

		var price float64
		var priceText string

		priceRaw := cleanText(e.ChildText("span.price.product-price"))
		if priceRaw != "" {
			price, priceText = ParseTNDPrice(priceRaw)
		}

		if price > 1_000_000 {
			log.Printf("[fatales-search] WARNING: sanity check failed — price %.2f for %q looks invalid, defaulting to 0", price, name)
			price = 0
			priceText = ""
		}

		imageURL := e.ChildAttr("img.replace-2x.img-responsive", "src")
		if imageURL == "" {
			imageURL = e.ChildAttr("img.img-responsive", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "data-src")
		}
		imageURL = AbsImageURL(fatalesMediaBase, imageURL)

		descRaw := e.ChildText(".product-description, .product-description-short, .description")
		description := cleanText(cleanDescription(descRaw))

		inStock := true
		e.DOM.Find("button[disabled], .add-to-cart button[disabled], .product-unavailable, .out-of-stock").Each(func(_ int, s *goquery.Selection) {
			inStock = false
		})
		if inStock {
			itemText := strings.ToLower(e.Text)
			if strings.Contains(itemText, "rupture") || strings.Contains(itemText, "épuisé") || strings.Contains(itemText, "indisponible") || strings.Contains(itemText, "hors stock") {
				inStock = false
			}
		}

		*rank++
		p := models.Product{
			ID:            GenerateScraperID("fa", productURL),
			Name:          name,
			Price:         price,
			PriceText:     cleanText(priceText),
			Category:      "",
			Subcategory:   "",
			Description:   description,
			Image:         imageURL,
			ProductURL:    productURL,
			Source:        "Fatales",
			Reference:     "",
			BaseReference: "",
			InStock:       inStock,
			SearchRank:    *rank,
			Specs:         make(map[string]string),
		}

		deepC := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
			colly.AllowedDomains("www.fatales.tn", "fatales.tn"),
			colly.AllowURLRevisit(),
		)
		deepC.SetRequestTimeout(10 * time.Second)

		var breadcrumbs []string
		var catOnce sync.Once

		deepC.OnRequest(func(r *colly.Request) {
			select {
			case <-ctx.Done():
				r.Abort()
			default:
			}
		})

		deepC.OnHTML("nav.breadcrumb ol li, .breadcrumb li, ul.breadcrumb li", func(d *colly.HTMLElement) {
			text := strings.TrimSpace(d.Text)
			if text != "" && !strings.EqualFold(text, "Accueil") && !strings.EqualFold(text, "Home") {
				breadcrumbs = append(breadcrumbs, text)
			}
		})

		deepC.OnHTML("#product-details", func(d *colly.HTMLElement) {
			dataJSON := d.Attr("data-product")
			if dataJSON == "" {
				return
			}
			var pd fatalesProductData
			if err := json.Unmarshal([]byte(dataJSON), &pd); err != nil {
				log.Printf("[fatales-deep] error parsing product data JSON: %v", err)
				return
			}

			if p.Reference == "" && pd.Reference != "" {
				p.Reference = cleanReference(pd.Reference)
				p.BaseReference = getBaseReference(p.Reference)
			}

			if pd.Ean13 != "" {
				p.EAN = cleanEAN13(pd.Ean13)
			}

			for _, feat := range pd.Features {
				key := strings.TrimSpace(feat.Name)
				val := strings.TrimSpace(feat.Value)
				if key != "" && val != "" {
					normKey := services.NormalizeSpecKey(key)
					if normKey != "" {
						p.Specs[normKey] = val
					}
				}
			}
		})

		if err := deepC.Visit(productURL); err != nil {
			log.Printf("[fatales-deep] error scraping %s: %v", productURL, err)
		}

		p.Subcategory = mapBeautyStoreCategory(breadcrumbs, productURL)
		p.Category = string(types.CategorySanteBeaute)
		catOnce.Do(func() {
			log.Printf("[fatales-cat] %q → subcategory=%s (breadcrumbs=%v)", name, p.Subcategory, breadcrumbs)
		})

		services.EnrichProduct(&p)

		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[fatales-search] error visiting %s: %v", url, err)
		return 0, false
	}

	pageProductCount = *rank - pageStartRank
	log.Printf("[fatales-search] page scraped, skipped (visited): %d", skippedCount)
	return
}

func SyncFatales(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.fatales.tn", "fatales.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML("button[disabled], .product-unavailable, .out-of-stock, #product-availability", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") || strings.Contains(text, "hors stock") {
			result.InStock = false
		}
	})

	c.OnHTML("span.price.product-price", func(e *colly.HTMLElement) {
		if !found {
			price, priceText := ParseTNDPrice(e.Text)
			if price > 0 {
				result.Price = price
				result.PriceText = priceText
				found = true
			}
		}
	})

	c.OnHTML("meta[itemprop=price]", func(e *colly.HTMLElement) {
		if !found {
			content := e.Attr("content")
			if content != "" {
				if v, err := strconv.ParseFloat(content, 64); err == nil && v > 0 {
					result.Price = v
					result.PriceText = fmt.Sprintf("%.3f TND", v)
					found = true
				}
			}
		}
	})

	err := c.Visit(original.ProductURL)
	if err != nil {
		log.Printf("[sync-fatales] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-fatales] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-fatales] synced %s (%s): %.3f TND inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}
