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

const bigDepotSearchBase = "https://bigdepot.tn/recherche"
const bigDepotMediaBase = "https://bigdepot.tn"

type BigDepotSearchScraper struct{}

func (b BigDepotSearchScraper) ScrapeSearch(ctx context.Context, query string, resultsChan chan<- models.Product) {
	query = SanitizeQuery(query)
	encodedQuery := neturl.QueryEscape(query)
	rank := 0

	for page := 1; page <= 5; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[bigdepot-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		searchURL := bigDepotSearchBase + "?controller=search&s=" + encodedQuery
		if page > 1 {
			searchURL += "&page=" + strconv.Itoa(page)
		}

		log.Printf("[bigdepot-search] scraping page %d: %s", page, searchURL)
		count, hasNext := bigDepotScrapeSearchPage(ctx, searchURL, resultsChan, &rank)

		if !hasNext || count == 0 {
			break
		}
	}

	log.Printf("[bigdepot-search] total products for %q: %d", query, rank)
}

func bigDepotScrapeSearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (int, bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.bigdepot.tn", "bigdepot.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*bigdepot.tn",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	hasNext := false
	visited := make(map[string]bool)
	pageStartRank := *rank
	matchedCards := 0

	c.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	c.OnResponse(func(r *colly.Response) {
		log.Printf("[bigdepot-search] received %d from %s", r.StatusCode, r.Request.URL)
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("[bigdepot-search] request error: %v (status: %d)", err, r.StatusCode)
	})

	c.OnHTML("a[rel=next], .pagination a.next, li.pagination-next a", func(e *colly.HTMLElement) {
		hasNext = true
	})

	c.OnHTML("article.product-miniature, .product-miniature, .ajax_block_product, .product-container, .wb-product-block, .product-block, li.product", func(e *colly.HTMLElement) {
		matchedCards++
		productURL := e.ChildAttr("a.product_img_link, a.product-thumbnail, a.thumbnail, a.product-name", "href")
		if productURL == "" {
			productURL = e.ChildAttr("a", "href")
		}
		if productURL == "" {
			return
		}
		normalized := normalizeURL(productURL)
		if visited[normalized] {
			return
		}
		visited[normalized] = true

		name := cleanText(e.ChildText("h2[itemprop=name] a.product-name, h2.product-title a, h3.product-title a, h5 a, h5.product-name a, .product-title a, .product-name"))
		if name == "" {
			name = cleanText(e.ChildText("h2 a, h3 a, h4 a, h5 a"))
		}
		if name == "" {
			return
		}
		name = normalizeName(name)

		var price float64
		var priceText string
		if metaContent := e.ChildAttr("meta[itemprop=price]", "content"); metaContent != "" {
			if v, err := strconv.ParseFloat(metaContent, 64); err == nil {
				price = v
			}
		}
		if price == 0 {
			priceRaw := cleanText(e.ChildText("span.price, .product-price, .price"))
			if priceRaw != "" {
				price, priceText = ParseTNDPrice(priceRaw)
			}
		}
		if price > 1_000_000 {
			log.Printf("[bigdepot-search] WARNING: sanity check failed — price %.2f for %q", price, name)
			price = 0
			priceText = ""
		}

		imageURL := e.ChildAttr("img[itemprop=image], img.primary-image, img.img-responsive, img.replace-2x", "src")
		if imageURL == "" {
			imageURL = e.ChildAttr("img[itemprop=image], img.primary-image, img.img-responsive, img.replace-2x", "data-src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "data-src")
		}
		imageURL = AbsImageURL(bigDepotMediaBase, imageURL)

		descRaw := e.ChildText(".product-description, .product-description-short, .description")
		description := cleanText(cleanDescription(descRaw))

		inStock := true
		e.DOM.Find(".product-unavailable, .out-of-stock, button[disabled]").Each(func(_ int, s *goquery.Selection) {
			inStock = false
		})
		if inStock {
			itemText := strings.ToLower(e.Text)
			if strings.Contains(itemText, "rupture") || strings.Contains(itemText, "epuise") || strings.Contains(itemText, "indisponible") || strings.Contains(itemText, "hors stock") {
				inStock = false
			}
		}

		*rank++
		p := models.Product{
			ID:            GenerateScraperID("bdp", productURL),
			Name:          name,
			Price:         price,
			PriceText:     cleanText(priceText),
			Category:      string(types.CategoryJardinageBricolage),
			Subcategory:   "",
			Description:   description,
			Image:         imageURL,
			ProductURL:    productURL,
			Source:        "BigDepot",
			Reference:     "",
			BaseReference: "",
			InStock:       inStock,
			SearchRank:    *rank,
			Specs:         make(map[string]string),
		}

		bigDepotDeepScrape(ctx, &p, productURL)

		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[bigdepot-search] error visiting %s: %v", url, err)
		return 0, false
	}

	c.Wait()
	count := *rank - pageStartRank
	return count, hasNext
}

type bigDepotProductData struct {
	Reference string `json:"reference"`
	Ean13     string `json:"ean13"`
}

func bigDepotDeepScrape(ctx context.Context, p *models.Product, productURL string) {
	deepC := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.bigdepot.tn", "bigdepot.tn"),
		colly.AllowURLRevisit(),
	)
	deepC.SetRequestTimeout(10 * time.Second)
	deepC.Limit(&colly.LimitRule{
		DomainGlob:  "*bigdepot.tn",
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
		var pd bigDepotProductData
		if err := json.Unmarshal([]byte(dataJSON), &pd); err != nil {
			log.Printf("[bigdepot-deep] error parsing data-product JSON: %v", err)
			return
		}
		if p.Reference == "" && pd.Reference != "" {
			p.Reference = cleanReference(pd.Reference)
			p.BaseReference = getBaseReference(p.Reference)
		}
		if pd.Ean13 != "" {
			p.EAN = cleanEAN13(pd.Ean13)
		}
	})

	deepC.OnHTML("section.product-features dl.data-sheet, .product-features dl.data-sheet", func(d *colly.HTMLElement) {
		d.DOM.Find("dt.name").Each(func(i int, sel *goquery.Selection) {
			key := strings.TrimSpace(sel.Text())
			if key == "" {
				return
			}
			normalizedKey := services.NormalizeSpecKey(key)
			val := strings.TrimSpace(sel.NextFiltered("dd.value").Text())
			if val != "" && normalizedKey != "" {
				p.Specs[normalizedKey] = val
			}
		})
	})

	deepC.OnHTML("meta[itemprop=gtin13], meta[itemprop=gtin12]", func(d *colly.HTMLElement) {
		if p.EAN == "" {
			content := d.Attr("content")
			if ean := cleanEAN13(content); ean != "" {
				p.EAN = ean
			}
		}
	})

	deepC.OnHTML("span[itemprop=sku], .product-reference span", func(d *colly.HTMLElement) {
		if p.Reference == "" {
			ref := cleanReference(d.Text)
			if ref != "" {
				p.Reference = ref
				p.BaseReference = getBaseReference(ref)
			}
		}
	})

	if err := deepC.Visit(productURL); err != nil {
		log.Printf("[bigdepot-deep] error scraping %s: %v", productURL, err)
	}

	p.Subcategory = MapJardinageBricolageCategory(breadcrumbs, productURL)
	catOnce.Do(func() {
		log.Printf("[bigdepot-cat] %q -> subcategory=%s (breadcrumbs=%v)", p.Name, p.Subcategory, breadcrumbs)
	})

	services.EnrichProduct(p)
}

func SyncBigDepot(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.bigdepot.tn", "bigdepot.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".product-unavailable, .out-of-stock, button[disabled], #product-availability", func(e *colly.HTMLElement) {
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

	c.OnHTML("span.price, .product-price, .current-price span", func(e *colly.HTMLElement) {
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
		log.Printf("[sync-bigdepot] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-bigdepot] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-bigdepot] synced %s (%s): %.3f TND inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}
