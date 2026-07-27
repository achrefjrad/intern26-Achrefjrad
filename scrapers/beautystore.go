package scrapers

import (
	"context"
	"fmt"
	"log"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"project/models"
	"project/services"
	"project/types"
)

func mapBeautyStoreCategory(breadcrumbs []string, urlPath string) string {
	check := func(keywords ...string) bool {
		for _, crumb := range breadcrumbs {
			low := strings.ToLower(crumb)
			for _, kw := range keywords {
				if strings.Contains(low, kw) {
					return true
				}
			}
		}
		lowURL := strings.ToLower(urlPath)
		for _, kw := range keywords {
			if strings.Contains(lowURL, kw) {
				return true
			}
		}
		return false
	}

	switch {
	case check("k-beauty", "soin", "solaire", "visage", "fotoprotector", "ecran", "ecran-solaire", "serum", "creme"):
		return string(types.SubcatSoinsVisage)
	case check("yeux", "lèvres", "levres", "teint", "ongles", "maquillage"):
		return string(types.SubcatMaquillage)
	case check("cheveux", "capillaire"):
		return string(types.SubcatCheveux)
	case check("toiletries", "hygiène", "hygiene", "bain"):
		return string(types.SubcatHygieneToilette)
	case check("parapharmacie", "moniteurs de santé", "moniteur de sante", "santé", "sante"):
		return string(types.SubcatParapharmacieSante)
	case check("bébé", "bebe", "enfant", "maman"):
		return string(types.SubcatBebeMaman)
	case check("coffret", "parfum"):
		return string(types.SubcatParfumsCoffrets)
	default:
		if cat, sub := services.AssignProductCategory(urlPath); sub != types.SubcatUncategorized && cat != types.CategoryUncategorized {
			return string(sub)
		}
		return string(types.SubcatSanteBeauteAutres)
	}
}

const beautySearchBase = "https://beautystore.tn/recherche"
const beautyMediaBase = "https://beautystore.tn"

type BeautyStoreSearchScraper struct{}

func (b BeautyStoreSearchScraper) ScrapeSearch(ctx context.Context, query string, resultsChan chan<- models.Product) {
	query = SanitizeQuery(query)
	encodedQuery := neturl.QueryEscape(query)
	emptyCount := 0
	rank := 0

	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[beautystore-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		searchURL := beautySearchBase + "?controller=search&s=" + encodedQuery
		if page > 1 {
			searchURL += "&page=" + strconv.Itoa(page)
		}

		log.Printf("[beautystore-search] scraping page %d: %s", page, searchURL)
		pageProducts, hasNext := scrapeBeautySearchPage(ctx, searchURL, resultsChan, &rank)

		if !hasNext {
			break
		}
		if pageProducts == 0 {
			emptyCount++
			if emptyCount >= 2 {
				log.Printf("[beautystore-search] stopping after %d consecutive empty pages", emptyCount)
				break
			}
			continue
		}
		emptyCount = 0
	}

	log.Printf("[beautystore-search] total products for %q: %d (ranks assigned)", query, rank)
}

func scrapeBeautySearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (pageProductCount int, hasNext bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.beautystore.tn", "beautystore.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*beautystore.tn",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	visited := make(map[string]bool)
	skippedCount := 0
	pageStartRank := *rank

	c.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	c.OnHTML("a.next[rel=next], a[rel=next], a.next, .next a, li.pagination-next a", func(e *colly.HTMLElement) {
		hasNext = true
	})

	c.OnHTML("article.product-miniature", func(e *colly.HTMLElement) {
		productURL := e.ChildAttr("a.product-thumbnail", "href")
		if productURL == "" {
			productURL = e.ChildAttr("a.thumbnail", "href")
		}
		if productURL == "" {
			productURL = e.ChildAttr("a.product-image", "href")
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

		name := cleanText(e.ChildText("h2.product-title a, h3.product-title a, .product-title a, a.product-name"))
		if name == "" {
			name = cleanText(e.ChildText("h2 a, h3 a"))
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
			priceText = cleanText(e.ChildText("span[itemprop=price]"))
		}

		if price == 0 {
			priceRaw := cleanText(e.ChildText("span.price"))
			if priceRaw != "" {
				price, priceText = ExtractPrice(priceRaw)
			}
		}

		if price > 1_000_000 {
			log.Printf("[beautystore-search] WARNING: sanity check failed — price %.2f for %q looks invalid, defaulting to 0", price, name)
			price = 0
			priceText = ""
		}

		imageURL := e.ChildAttr("img[itemprop=image], img.primary-image, img.img-responsive", "src")
		if imageURL == "" {
			imageURL = e.ChildAttr("img[itemprop=image], img.primary-image, img.img-responsive", "data-src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "data-src")
		}
		imageURL = AbsImageURL(beautyMediaBase, imageURL)

		descRaw := e.ChildText(".product-description, .product-description-short, .description")
		description := cleanText(cleanDescription(descRaw))

		inStock := true
		e.DOM.Find(".product-unavailable, .out-of-stock, .strike, .product-actions.out-of-stock, button[disabled]").Each(func(_ int, s *goquery.Selection) {
			inStock = false
		})
		if inStock {
			itemText := strings.ToLower(e.Text)
			if strings.Contains(itemText, "rupture") || strings.Contains(itemText, "épuisé") || strings.Contains(itemText, "indisponible") || strings.Contains(itemText, "hors stock") {
				inStock = false
			}
		}

		reference := cleanReference(e.ChildText("span.product-reference, span[itemprop=sku]"))

		*rank++
		p := models.Product{
			ID:            GenerateScraperID("bs", productURL),
			Name:          name,
			Price:         price,
			PriceText:     cleanText(priceText),
			Category:      "Santé & Beauté",
			Subcategory:   "",
			Description:   description,
			Image:         imageURL,
			ProductURL:    productURL,
			Source:        "BeautyStore",
			Reference:     reference,
			BaseReference: getBaseReference(reference),
			InStock:       inStock,
			SearchRank:    *rank,
			Specs:         make(map[string]string),
		}

		deepC := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
			colly.AllowedDomains("www.beautystore.tn", "beautystore.tn"),
			colly.AllowURLRevisit(),
		)
		deepC.SetRequestTimeout(10 * time.Second)
		deepC.Limit(&colly.LimitRule{
			DomainGlob:  "*beautystore.tn",
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

		deepC.OnHTML("section.product-features dl.data-sheet", func(d *colly.HTMLElement) {
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

		deepC.OnHTML("span[itemprop=sku], .product-reference span", func(d *colly.HTMLElement) {
			if p.Reference == "" {
				ref := cleanReference(d.Text)
				if ref != "" {
					p.Reference = ref
					p.BaseReference = getBaseReference(ref)
				}
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
			log.Printf("[beautystore-deep] error scraping %s: %v", productURL, err)
		}

		p.Subcategory = mapBeautyStoreCategory(breadcrumbs, productURL)
		catOnce.Do(func() {
			log.Printf("[beautystore-cat] %q → subcategory=%s (breadcrumbs=%v)", name, p.Subcategory, breadcrumbs)
		})

		if _, ok := p.Specs["marque"]; !ok {
			if brand := extractBeautyBrandFromTitle(name); brand != "" {
				p.Specs["marque"] = brand
			}
		}

		services.EnrichProduct(&p)

		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[beautystore-search] error visiting %s: %v", url, err)
		return 0, false
	}

	pageProductCount = *rank - pageStartRank
	log.Printf("[beautystore-search] page scraped, skipped (visited): %d", skippedCount)
	return
}

// extractBeautyBrandFromTitle attempts to extract the brand from a product title
// by taking the first fully-capitalized word(s), e.g. "CELIMAX Disques..." → "CELIMAX"
func extractBeautyBrandFromTitle(title string) string {
	fields := strings.Fields(title)
	if len(fields) == 0 {
		return ""
	}
	var brandParts []string
	for _, f := range fields {
		cleaned := strings.Trim(f, ".,;:!?()[]{}«»\"'-")
		if cleaned == "" {
			continue
		}
		isUpper := true
		for _, r := range cleaned {
			if unicode.IsLetter(r) && !unicode.IsUpper(r) {
				isUpper = false
				break
			}
		}
		if !isUpper || len(cleaned) <= 1 {
			break
		}
		brandParts = append(brandParts, cleaned)
	}
	if len(brandParts) > 0 {
		return strings.Join(brandParts, " ")
	}
	return ""
}

func SyncBeautyStore(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.beautystore.tn", "beautystore.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*beautystore.tn",
		Delay:       1 * time.Second,
		RandomDelay: 2 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7")
	})

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".out-of-stock, .product-unavailable, .unavailable, .stock-status, #product-availability", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") || strings.Contains(text, "hors stock") {
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

	c.OnHTML("span[itemprop=price]", func(e *colly.HTMLElement) {
		if !found {
			price, priceText := ExtractPrice(e.Text)
			if price > 0 {
				result.Price = price
				result.PriceText = priceText
				found = true
			}
		}
	})

	c.OnHTML("span.price", func(e *colly.HTMLElement) {
		if !found {
			price, priceText := ExtractPrice(e.Text)
			if price > 0 {
				result.Price = price
				result.PriceText = priceText
				found = true
			}
		}
	})

	err := c.Visit(original.ProductURL)
	if err != nil {
		log.Printf("[sync-beautystore] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-beautystore] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-beautystore] synced %s (%s): %.3f DT inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}
