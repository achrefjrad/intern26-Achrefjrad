package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	neturl "net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"project/models"
	"project/services"
)

const sbsSearchBase = "https://www.sbsinformatique.com/recherche"
const sbsMediaBase = "https://www.sbsinformatique.com"

type SBSSearchScraper struct{}

func (s SBSSearchScraper) ScrapeSearch(ctx context.Context, query string, resultsChan chan<- models.Product) {
	query = SanitizeQuery(query)
	encodedQuery := strings.ReplaceAll(neturl.QueryEscape(query), "%2B", "+")
	emptyCount := 0
	rank := 0

	for page := 1; page <= 3; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[sbs-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		searchURL := sbsSearchBase + "?s=" + encodedQuery
		if page > 1 {
			searchURL += "&page=" + strconv.Itoa(page)
		}

		log.Printf("[sbs-search] scraping page %d: %s", page, searchURL)
		pageProducts, hasNext := scrapeSBSSearchPage(ctx, searchURL, resultsChan, &rank)

		if !hasNext {
			break
		}
		if pageProducts == 0 {
			emptyCount++
			if emptyCount >= 2 {
				log.Printf("[sbs-search] stopping after %d consecutive empty pages", emptyCount)
				break
			}
			continue
		}
		emptyCount = 0
	}

	log.Printf("[sbs-search] total products for %q: %d (ranks assigned)", query, rank)
}

func deepScrapeSBSSpecs(ctx context.Context, parent *colly.Collector, productURL string, p *models.Product) {
	deepC := parent.Clone()
	deepC.SetRequestTimeout(5 * time.Second)
	deepC.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})
	deepC.OnHTML("section.product-features dl.data-sheet, .table-data-sheet tr", func(e *colly.HTMLElement) {
		key := services.NormalizeSpecKey(e.ChildText("dt.name, th"))
		if key == "" {
			return
		}
		val := strings.TrimSpace(e.ChildText("dd.value, td"))
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
		log.Printf("[sbs-deep] error scraping %s: %v", productURL, err)
	}
}

func scrapeSBSSearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (pageProductCount int, hasNext bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.sbsinformatique.com", "sbsinformatique.com"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)

	visited := make(map[string]bool)
	skippedCount := 0
	pageStartRank := *rank

	referenceMap := make(map[string]string)

	// Single-product redirect capture
	var finalURL string
	var spName, spImage, spDesc string
	var spPrice float64
	var spPriceText string
	var spReference string
	spFound := false

	c.OnResponse(func(r *colly.Response) {
		finalURL = r.Request.URL.String()
	})

	c.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})

	c.OnHTML("a.next[rel=next], a[rel=next], a.next, .next a, li.pagination-next a", func(e *colly.HTMLElement) {
		hasNext = true
	})

	c.OnHTML("#js-rcpgtm-tracking-data", func(e *colly.HTMLElement) {
		encoded := e.Attr("data-tracking-data")
		if encoded == "" {
			return
		}
		decoded, err := neturl.QueryUnescape(encoded)
		if err != nil {
			log.Printf("[sbs-search] failed to decode tracking data: %v", err)
			return
		}
		var tracking struct {
			DetailProductsList []struct {
				Name      string `json:"name"`
				Reference string `json:"reference"`
			} `json:"detail_products_list"`
		}
		if err := json.Unmarshal([]byte(decoded), &tracking); err != nil {
			log.Printf("[sbs-search] failed to parse tracking data JSON: %v", err)
			return
		}
		for _, item := range tracking.DetailProductsList {
			if item.Reference != "" {
				referenceMap[normalizeSBSKey(item.Name)] = item.Reference
			}
		}
		log.Printf("[sbs-search] built reference map with %d entries from tracking data", len(referenceMap))
	})

	c.OnHTML("article.product-miniature", func(e *colly.HTMLElement) {
		productURL := e.ChildAttr("a.thumbnail[itemprop=url], a.thumbnail.product-thumbnail, a.product-thumbnail", "href")
		if productURL == "" {
			productURL = e.ChildAttr("a.thumbnail", "href")
		}
		if productURL == "" {
			productURL = e.ChildAttr("a", "href")
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

		nameSelection := e.DOM.Find("div.tvproduct-name.product-title a h6, .product-title h6, .tvproduct-name h6, div.tvproduct-name.product-title a, .product-title a, h6[itemprop=name]")
		name := cleanText(nameSelection.First().Text())
		if name == "" {
			return
		}

		var price float64
		var priceText string

		priceSelection := e.DOM.Find(".tvproduct-wrapper.grid span.price, .product-price-and-shipping span.price, span.price")
		priceRaw := strings.TrimSpace(priceSelection.First().Text())
		if priceRaw != "" {
			price, priceText = ParseTNDPrice(priceRaw)
		}
		if price > 1_000_000 {
			log.Printf("[sbs-search] WARNING: sanity check failed — price %.2f for %q looks invalid, defaulting to 0", price, name)
			price = 0
			priceText = ""
		}

		imageURL := e.ChildAttr("img.tvproduct-defult-img", "src")
		if imageURL == "" {
			imageURL = e.ChildAttr("img[itemprop=image]", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr(".thumbnail img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "data-src")
		}
		imageURL = AbsImageURL(sbsMediaBase, imageURL)

		descRaw := cleanText(e.ChildText(".tv-product-desc, .product-description"))
		description := cleanText(descRaw)

		reference := referenceMap[normalizeSBSKey(name)]
		if reference == "" {
			reference = cleanReference(e.ChildText("span[itemprop=sku]"))
		}
		if reference == "" {
			reference = cleanReference(e.ChildText(".product-reference span"))
		}

		hashID := GenerateScraperID("sbs", productURL)

		inStock := true
		if e.DOM.Find(".out-of-stock, .product-unavailable, .unavailable").Length() > 0 {
			inStock = false
		}
		if inStock {
			stockText := strings.ToLower(e.ChildText(".availability, .stock-status, .stock, #product-availability"))
			if strings.Contains(stockText, "rupture") || strings.Contains(stockText, "épuisé") || strings.Contains(stockText, "indisponible") {
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
			Description:    description,
			Image:          imageURL,
			ProductURL:     productURL,
			Source:         "SBS Informatique",
			Reference:      reference,
			BaseReference:  getBaseReference(reference),
			InStock:        inStock,
			SearchRank:     *rank,
			Specs:          make(map[string]string),
		}
		deepScrapeSBSSpecs(ctx, c, productURL, &p)
		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	// Single-product page detection (PrestaShop product page)
	isProductPage := false
	c.OnHTML("form#add-to-cart-or-refresh, div.product-information", func(e *colly.HTMLElement) {
		isProductPage = true
	})

	c.OnHTML("h1[itemprop=name], h1", func(e *colly.HTMLElement) {
		spFound = true
		spName = cleanText(e.Text)
	})

	c.OnHTML("meta[itemprop=price]", func(e *colly.HTMLElement) {
		if content := e.Attr("content"); content != "" {
			if v, err := strconv.ParseFloat(content, 64); err == nil {
				spPrice = v
			}
		}
	})

	c.OnHTML("span.price", func(e *colly.HTMLElement) {
		if spPrice == 0 {
			price, priceText := ExtractPrice(e.Text)
			if price > 0 {
				spPrice = price
				spPriceText = priceText
			}
		}
	})

	c.OnHTML("img[itemprop=image], .product-cover img", func(e *colly.HTMLElement) {
		if spImage == "" {
			src := e.Attr("src")
			if src == "" {
				src = e.Attr("data-src")
			}
			spImage = AbsImageURL(sbsMediaBase, src)
		}
	})

	c.OnHTML(".product-description, .product-information", func(e *colly.HTMLElement) {
		if spDesc == "" {
			spDesc = cleanText(e.Text)
		}
	})

	c.OnHTML("span[itemprop=sku], .product-reference span", func(e *colly.HTMLElement) {
		if spReference == "" {
			spReference = cleanReference(e.Text)
		}
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[sbs-search] error visiting %s: %v", url, err)
		return 0, false
	}

	// If grid found nothing but we detected a single product page, extract it
	isSearchURL := strings.Contains(finalURL, "/recherche") || strings.Contains(finalURL, "?s=")
	hasRechercher := strings.Contains(spName, "Rechercher")
	if spFound && spName != "" && isProductPage && !isSearchURL && !hasRechercher {
		if spPrice > 1_000_000 {
			log.Printf("[sbs-search] WARNING: sanity check failed — single-product price %.2f for %q looks invalid, defaulting to 0", spPrice, spName)
			spPrice = 0
			spPriceText = ""
		}
		prodURL := finalURL
		if prodURL == "" || prodURL == url {
			prodURL = url
		}
		*rank++
		p := models.Product{
			ID:             GenerateScraperID("sbs", prodURL),
			Name:           spName,
			Price:          spPrice,
			PriceText:      spPriceText,
			Category:       "",
			Subcategory:    "",
			Description:    spDesc,
			Image:          spImage,
			ProductURL:     prodURL,
			Source:         "SBS Informatique",
			Reference:      spReference,
			BaseReference:  getBaseReference(spReference),
			InStock:        true,
			SearchRank:  *rank,
			Specs:          make(map[string]string),
		}
		deepScrapeSBSSpecs(ctx, c, prodURL, &p)
		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
		log.Printf("[sbs-search] single-product redirect detected: %q (%.3f DT)", spName, spPrice)
	}

	pageProductCount = *rank - pageStartRank
	log.Printf("[sbs-search] page scraped, skipped (visited): %d", skippedCount)
	return
}

var sbsKeyNorm = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func normalizeSBSKey(s string) string {
	return strings.ToLower(sbsKeyNorm.ReplaceAllString(s, ""))
}

func extractSBSPrice(s string) (float64, string) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, ""
	}

	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	raw = strings.ReplaceAll(raw, "\t", " ")
	reSpace := regexp.MustCompile(`\s+`)
	raw = strings.TrimSpace(reSpace.ReplaceAllString(raw, " "))

	cleaned := raw

	cleaned = strings.ReplaceAll(cleaned, "DT", "")
	cleaned = strings.ReplaceAll(cleaned, "TND", "")
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", "")
	cleaned = strings.TrimSpace(cleaned)

	cleaned = strings.ReplaceAll(cleaned, " ", "")

	if len(cleaned) >= 2 && len(cleaned)%2 == 0 {
		half := len(cleaned) / 2
		if cleaned[:half] == cleaned[half:] {
			log.Printf("[sbs-search] WARNING: duplicated numeric pattern in %q, keeping first half", cleaned)
			cleaned = cleaned[:half]
		}
	}

	if idx := strings.LastIndex(cleaned, ","); idx != -1 {
		cleaned = cleaned[:idx] + "." + cleaned[idx+1:]
	}

	reDigits := regexp.MustCompile(`[^\d.]`)
	cleaned = reDigits.ReplaceAllString(cleaned, "")

	if cleaned == "" {
		log.Printf("[sbs-search] WARNING: could not parse price from %q", raw)
		return 0, raw
	}

	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		log.Printf("[sbs-search] WARNING: strconv.ParseFloat failed for %q (cleaned: %q): %v", raw, cleaned, err)
		return 0, raw
	}

	return val, raw
}

func SyncSBS(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.sbsinformatique.com", "sbsinformatique.com"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".out-of-stock, .product-unavailable, .unavailable, .stock-status, #product-availability", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") {
			result.InStock = false
		}
	})

	c.OnHTML("meta[itemprop=price]", func(e *colly.HTMLElement) {
		content := e.Attr("content")
		if content != "" {
			if v, err := strconv.ParseFloat(content, 64); err == nil && v > 0 {
				result.Price = v
				result.PriceText = fmt.Sprintf("%.3f DT", v)
				found = true
			}
		}
	})

	c.OnHTML("span.price", func(e *colly.HTMLElement) {
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
		log.Printf("[sync-sbs] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-sbs] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-sbs] synced %s (%s): %.3f DT inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}
