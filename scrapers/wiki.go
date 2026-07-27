package scrapers

import (
	"context"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/PuerkitoBio/goquery"
	"project/models"
	"project/services"
)

func cleanWikiText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

const wikiSearchBase = "https://wiki.tn"

type WikiSearchScraper struct{}

func (w WikiSearchScraper) ScrapeSearch(ctx context.Context, query string, ch chan<- models.Product) {
	query = SanitizeQuery(query)
	rank := 0
	emptyCount := 0

	for page := 1; page <= 3; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[wiki-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		u, err := url.Parse(wikiSearchBase)
		if err != nil {
			log.Printf("[wiki-search] failed to parse base URL: %v", err)
			continue
		}
		q := u.Query()
		q.Set("s", query)
		q.Set("post_type", "product")
		q.Set("dgwt_wcas", "1")
		if page > 1 {
			q.Set("paged", strconv.Itoa(page))
		}
		u.RawQuery = q.Encode()
		searchURL := u.String()

		log.Printf("[wiki-search] scraping page %d: %s", page, searchURL)
		pageProductCount, hasNext := scrapeWikiSearchPage(ctx, searchURL, ch, &rank)
		log.Printf("[wiki-search] page %d products found: %d, hasNext: %v", page, pageProductCount, hasNext)

		if !hasNext {
			break
		}
		if pageProductCount == 0 {
			emptyCount++
			if emptyCount >= 2 {
				log.Printf("[wiki-search] stopping after %d consecutive empty pages", emptyCount)
				break
			}
			continue
		}
		emptyCount = 0
	}

	log.Printf("[wiki-search] total products for %q: %d (ranks assigned)", query, rank)
}

func extractWikiFallbackProduct(e *colly.HTMLElement, visited map[string]bool, skippedCount *int) *models.Product {
	productURL := e.ChildAttr("a.woocommerce-loop-product__link", "href")
	if productURL == "" {
		productURL = e.ChildAttr("h3.product-card__title a", "href")
	}
	if productURL == "" {
		productURL = e.ChildAttr("figure.product-card__image a", "href")
	}
	if productURL == "" {
		productURL = e.ChildAttr("a", "href")
	}
	if productURL == "" {
		return nil
	}
	normalized := normalizeURL(productURL)
	if visited[normalized] {
		*skippedCount++
		return nil
	}
	visited[normalized] = true

	name := cleanWikiText(e.ChildText("h2.woocommerce-loop-product__title, h3.product-card__title a, .woocommerce-loop-product__title"))
	if name == "" {
		name = cleanWikiText(e.ChildText("h2 a, h3 a, .product-title a"))
	}
	if name == "" {
		return nil
	}

	var price float64
	var priceText string

	priceRaw := e.ChildText(".price ins .woocommerce-Price-amount bdi, .price .woocommerce-Price-amount bdi, div.product-card__price ins bdi, div.product-card__price .woocommerce-Price-amount bdi")
	if priceRaw != "" {
		price, priceText = ParseTNDPrice(priceRaw)
	}

	imageURL := ""
	e.DOM.Find("img").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, attr := range []string{"data-large_image", "data-src", "data-lazy-src", "srcset", "src"} {
			val, _ := s.Attr(attr)
			val = strings.TrimSpace(val)
			if attr == "srcset" && val != "" {
				parts := strings.Fields(val)
				if len(parts) > 0 {
					val = parts[0]
				}
			}
			if strings.HasPrefix(val, "data:image") {
				continue
			}
			if strings.Contains(val, "wp-content/uploads/") {
				imageURL = val
				return false
			}
		}
		return true
	})
	if imageURL == "" {
		log.Printf("[wiki-search] missing image for product: %s", name)
	}
	imageURL = AbsImageURL("https://www.wiki.tn", imageURL)

		reference := cleanReference(e.ChildText("span.sku, .product-card__sku span.sku"))

		hashID := GenerateScraperID("wt", productURL)

	inStock := true
	if e.DOM.Find(".out-of-stock, .stock.out-of-stock, .product.outofstock, [data-stock-status='outofstock']").Length() > 0 {
		inStock = false
	}
	if inStock {
		stockText := strings.ToLower(e.ChildText(".stock-status, .availability, .stock, .brxe-shortcode-dispo, .stock-status-badge"))
		if strings.Contains(stockText, "rupture") || strings.Contains(stockText, "épuisé") || strings.Contains(stockText, "indisponible") || strings.Contains(stockText, "hors stock") {
			inStock = false
		}
	}

	return &models.Product{
			ID:             hashID,
			Name:           name,
			Price:          price,
			PriceText:      priceText,
			Category:       "",
			Subcategory:    "",
			Image:          imageURL,
			ProductURL:     productURL,
			Source:         "wiki.tn",
			Reference:      reference,
			BaseReference:  getBaseReference(reference),
			InStock:        inStock,
			Specs:          make(map[string]string),
		}
}

func deepScrapeWikiSpecs(ctx context.Context, parent *colly.Collector, productURL string, p *models.Product) {
	deepC := parent.Clone()
	deepC.SetRequestTimeout(5 * time.Second)
	deepC.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})
	deepC.OnHTML("#tab-description, .woocommerce-product-details__short-description", func(e *colly.HTMLElement) {
		if p.Description == "" {
			p.Description = cleanWikiText(e.Text)
		}
	})
	deepC.OnHTML("table.woocommerce-product-attributes tr, .shop_attributes tr", func(e *colly.HTMLElement) {
		key := services.NormalizeSpecKey(e.ChildText("th"))
		if key == "" {
			return
		}
		val := strings.TrimSpace(e.ChildText("td"))
		if val != "" {
			p.Specs[key] = val
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
	if err := deepC.Visit(productURL); err != nil {
		log.Printf("[wiki-deep] error scraping %s: %v", productURL, err)
	}
}

func scrapeWikiSearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (pageProductCount int, hasNext bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.wiki.tn", "wiki.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)

	hasNext = false
	visited := make(map[string]bool)
	skippedCount := 0
	countCardGrid := 0
	countWooProduct := 0
	pageStartRank := *rank

	var spName, spImage, spURL string
	var spPrice float64
	var spPriceText string
	spFound := false
	var spReference string

	c.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})

	c.OnHTML("a.next.page-numbers, a.next, .next a, .pagination a.next", func(e *colly.HTMLElement) {
		hasNext = true
	})

	c.OnHTML("div.product-card--grid", func(e *colly.HTMLElement) {
		countCardGrid++
		productURL := e.ChildAttr("h3.product-card__title a", "href")
		if productURL == "" {
			productURL = e.ChildAttr("figure.product-card__image a", "href")
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

		name := cleanWikiText(e.ChildText("h3.product-card__title a"))
		if name == "" {
			return
		}

		var price float64
		var priceText string

		priceRaw := e.ChildText("div.product-card__price ins bdi")
		if priceRaw == "" {
			priceRaw = e.ChildText("div.product-card__price .woocommerce-Price-amount bdi")
		}
		if priceRaw != "" {
			price, priceText = ParseTNDPrice(priceRaw)
		}

		imageURL := ""
		e.DOM.Find("img").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			for _, attr := range []string{"data-large_image", "data-src", "data-lazy-src", "srcset", "src"} {
				val, _ := s.Attr(attr)
				val = strings.TrimSpace(val)
				if attr == "srcset" && val != "" {
					parts := strings.Fields(val)
					if len(parts) > 0 {
						val = parts[0]
					}
				}
				if strings.HasPrefix(val, "data:image") {
					continue
				}
				if strings.Contains(val, "wp-content/uploads/") {
					imageURL = val
					return false
				}
			}
			return true
		})
		if imageURL == "" {
			log.Printf("[wiki-search] missing image for product: %s", name)
		}
		imageURL = AbsImageURL("https://www.wiki.tn", imageURL)

	reference := cleanReference(e.ChildText("span.sku, .product-card__sku span.sku"))

		hashID := GenerateScraperID("wt", productURL)

		inStock := true
		if e.DOM.Find(".out-of-stock, .stock.out-of-stock, .product.outofstock, [data-stock-status='outofstock']").Length() > 0 {
			inStock = false
		}
		if inStock {
			stockText := strings.ToLower(e.ChildText(".stock-status, .availability, .stock, .brxe-shortcode-dispo, .stock-status-badge"))
			if strings.Contains(stockText, "rupture") || strings.Contains(stockText, "épuisé") || strings.Contains(stockText, "indisponible") || strings.Contains(stockText, "hors stock") {
				inStock = false
			}
		}

		*rank++
		p := models.Product{
			ID:             hashID,
			Name:           name,
			Price:          price,
			PriceText:      priceText,
			Category:       "",
			Subcategory:    "",
			Image:          imageURL,
			ProductURL:     productURL,
			Source:         "wiki.tn",
			Reference:      reference,
			BaseReference:  getBaseReference(reference),
			InStock:        inStock,
			SearchRank:     *rank,
			Specs:          make(map[string]string),
		}
		deepScrapeWikiSpecs(ctx, c, productURL, &p)
		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	c.OnHTML("li.product", func(e *colly.HTMLElement) {
		countWooProduct++
		p := extractWikiFallbackProduct(e, visited, &skippedCount)
		if p != nil {
			*rank++
			p.SearchRank = *rank
			deepScrapeWikiSpecs(ctx, c, p.ProductURL, p)
			services.EnrichProduct(p)
			select {
			case ch <- *p:
			case <-ctx.Done():
			}
		}
	})

	c.OnHTML("body.single-product", func(e *colly.HTMLElement) {
		spFound = true

		spName = cleanWikiText(e.ChildText("h1.brxe-product-title"))

		spURL = e.Request.URL.String()
		if spURL == "" {
			spURL = url
		}

		priceRaw := e.ChildText("ins .woocommerce-Price-amount bdi")
		if priceRaw == "" {
			priceRaw = e.ChildText(".woocommerce-Price-amount bdi")
		}
		if priceRaw != "" {
			cleaned := priceRaw
			cleaned = strings.ReplaceAll(cleaned, ",", ".")
			cleaned = strings.ReplaceAll(cleaned, "TND", "")
			cleaned = strings.ReplaceAll(cleaned, "DT", "")
			reNonNum := regexp.MustCompile(`[^\d.]`)
			cleaned = reNonNum.ReplaceAllString(cleaned, "")
			if cleaned != "" {
				if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
					spPrice = v
					spPriceText = priceRaw
				} else {
					log.Printf("[wiki-search] failed to parse single-product price %q (cleaned: %q): %v", priceRaw, cleaned, err)
				}
			} else {
				log.Printf("[wiki-search] price text %q produced empty cleaned string", priceRaw)
			}
		} else {
			log.Printf("[wiki-search] no price element found on single-product page")
		}

		e.DOM.Find("img").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			for _, attr := range []string{"data-large_image", "data-src", "data-lazy-src", "srcset", "src"} {
				val, _ := s.Attr(attr)
				val = strings.TrimSpace(val)
				if attr == "srcset" && val != "" {
					parts := strings.Fields(val)
					if len(parts) > 0 {
						val = parts[0]
					}
				}
				if strings.HasPrefix(val, "data:image") {
					continue
				}
				if strings.Contains(val, "wp-content/uploads/") {
					spImage = val
					return false
				}
			}
			return true
		})

		refRaw := e.ChildText("span.sku, .product-card__sku span.sku")
		spReference = cleanReference(refRaw)
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[wiki-search] error visiting %s: %v", url, err)
		return 0, false
	}

	if spFound && spName != "" {
		*rank++
		p := models.Product{
			ID:             GenerateScraperID("wt", spURL),
			Name:           spName,
			Price:          spPrice,
			PriceText:      spPriceText,
			Category:       "",
			Subcategory:    "",
			Image:          spImage,
			ProductURL:     spURL,
			Source:         "wiki.tn",
			Reference:      spReference,
			BaseReference:  getBaseReference(spReference),
			SearchRank:     *rank,
			Specs:          make(map[string]string),
		}
		deepScrapeWikiSpecs(ctx, c, spURL, &p)
		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
		log.Printf("[wiki-search] single-product redirect detected: %q (%.3f DT)", spName, spPrice)
		pageProductCount = 1
		return
	}

	pageProductCount = *rank - pageStartRank
	return
}

func SyncWiki(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.wiki.tn", "wiki.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".out-of-stock, p.stock.out-of-stock, .stock.out-of-stock, [data-stock-status='outofstock'], .stock-status-badge", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") || strings.Contains(text, "hors stock") {
			result.InStock = false
		}
	})

	c.OnHTML(".brxe-shortcode-dispo", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") || strings.Contains(text, "hors stock") {
			result.InStock = false
		}
	})

	c.OnHTML(".price ins .woocommerce-Price-amount bdi", func(e *colly.HTMLElement) {
		if !found {
			price, priceText := ParseTNDPrice(e.Text)
			if price > 0 {
				result.Price = price
				result.PriceText = priceText
				found = true
			}
		}
	})

	c.OnHTML(".price .woocommerce-Price-amount bdi", func(e *colly.HTMLElement) {
		if !found {
			price, priceText := ParseTNDPrice(e.Text)
			if price > 0 {
				result.Price = price
				result.PriceText = priceText
				found = true
			}
		}
	})

	err := c.Visit(original.ProductURL)
	if err != nil {
		log.Printf("[sync-wiki] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-wiki] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-wiki] synced %s (%s): %.3f DT inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}