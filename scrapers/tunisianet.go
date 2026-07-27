package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"project/models"
	"project/services"
)

var uiNoisePatterns = []string{
	"Ajouter au panier",
	"Ajouter au comparateur",
	"Comparer",
	"Favoris",
	"Mes favoris",
	"En savoir plus",
	"Voir le produit",
	"Quick view",
	"Aperçu rapide",
	"En stock",
	"Rupture",
	"Disponible",
	"Promo",
}

func ExtractPrice(s string) (float64, string) {
	return ParseTNDPrice(s)
}

func cleanDescription(text string) string {
	if text == "" {
		return ""
	}

	for _, pattern := range uiNoisePatterns {
		text = strings.ReplaceAll(text, pattern, "")
	}

	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	text = strings.TrimSpace(text)

	return text
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

var configSuffixPattern = regexp.MustCompile(`(-(?:8|12|16|24|32|36|48|64)(?:G|GB|GO)?|-W11P|-WIN11PRO|-W11|-WIN11|-DOS|-UBUNTU|-UB|-(?:256|512)(?:G|GB|GO|SSD)|-1(?:T|TB))*$`)
var regionalSuffixPattern = regexp.MustCompile(`(?:/|-)(?:TN|FR)$`)

func getBaseReference(rawRef string) string {
	if rawRef == "" {
		return ""
	}
	result := regionalSuffixPattern.ReplaceAllString(rawRef, "")
	result = configSuffixPattern.ReplaceAllString(result, "")
	return strings.ToUpper(result)
}

func cleanReference(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.Trim(ref, "[]")
	ref = strings.TrimSpace(ref)
	prefixes := []string{"R\u00e9f :", "Ref :", "R\u00e9f\u00e9rence :", "R\u00e9f\u00e9rence"}
	for _, p := range prefixes {
		ref = strings.TrimPrefix(ref, p)
	}
	return strings.TrimSpace(ref)
}

func cleanEAN13(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "ean13:")
	s = strings.TrimPrefix(s, "EAN13:")
	s = strings.TrimPrefix(s, "ean:")
	s = strings.TrimPrefix(s, "EAN:")
	var digits strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	result := digits.String()
	if len(result) != 13 {
		return ""
	}
	return result
}

func normalizeName(name string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(name, " "))
}

const tunisianetSearchBase = "https://www.tunisianet.com.tn/recherche"

type TunisianetSearchScraper struct{}

func (t TunisianetSearchScraper) ScrapeSearch(ctx context.Context, query string, resultsChan chan<- models.Product) {
	query = SanitizeQuery(query)
	encodedQuery := url.QueryEscape(query)
	emptyCount := 0
	rank := 0

	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[tunisianet-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		searchURL := fmt.Sprintf("%s?controller=search&s=%s", tunisianetSearchBase, encodedQuery)
		if page > 1 {
			searchURL = fmt.Sprintf("%s&page=%d", searchURL, page)
		}

		log.Printf("[tunisianet-search] scraping page %d: %s", page, searchURL)
		pageProducts, hasNext := scrapeTunisianetSearchPage(ctx, searchURL, resultsChan, &rank)

		if !hasNext {
			break
		}
		if pageProducts == 0 {
			emptyCount++
			if emptyCount >= 2 {
				log.Printf("[tunisianet-search] stopping after %d consecutive empty pages", emptyCount)
				break
			}
			continue
		}
		emptyCount = 0
	}

	log.Printf("[tunisianet-search] total products for %q: %d (ranks assigned)", query, rank)
}

func scrapeTunisianetSearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (pageProductCount int, hasNext bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.tunisianet.com.tn", "tunisianet.com.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)

	skippedCount := 0
	pageStartRank := *rank
	visited := make(map[string]bool)

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

	c.OnHTML("a[rel=next], a.next, .next a, li.pagination-next a", func(e *colly.HTMLElement) {
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
		var priceHTML string

		if metaContent := e.ChildAttr("meta[itemprop=price]", "content"); metaContent != "" {
			priceHTML = fmt.Sprintf("<meta itemprop=\"price\" content=\"%s\">", metaContent)
			if v, err := strconv.ParseFloat(metaContent, 64); err == nil {
				price = v
			}
			priceText = cleanText(e.ChildText("span[itemprop=price]"))
		}

		if priceHTML == "" {
			if sel := e.DOM.Find("span[itemprop=price]").First(); sel.Length() > 0 {
				priceHTML, _ = sel.Html()
				price, priceText = ExtractPrice(strings.TrimSpace(sel.Text()))
			}
		}
		if priceHTML == "" {
			if sel := e.DOM.Find("span.price").First(); sel.Length() > 0 {
				priceHTML, _ = sel.Html()
				price, priceText = ExtractPrice(strings.TrimSpace(sel.Text()))
			}
		}
		if priceHTML == "" {
			if sel := e.DOM.Find(".price").First(); sel.Length() > 0 {
				priceHTML, _ = sel.Html()
				price, priceText = ExtractPrice(strings.TrimSpace(sel.Text()))
			}
		}

		imageURL := e.ChildAttr(".product-thumbnail img, img.img-responsive, img.primary-image", "src")
		if imageURL == "" {
			imageURL = e.ChildAttr(".product-thumbnail img, img.img-responsive, img.primary-image", "data-src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "data-src")
		}
		imageURL = AbsImageURL("https://www.tunisianet.com.tn", imageURL)

		descRaw := e.ChildText(".product-description, .product-description-short, .description")
		description := cleanText(cleanDescription(descRaw))

		hashID := GenerateScraperID("tn", productURL)

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
			ID:            hashID,
			Name:          name,
			Price:         price,
			PriceText:     cleanText(priceText),
			Category:      "",
			Subcategory:   "",
			Description:   description,
			Image:         imageURL,
			ProductURL:    productURL,
			Source:        "Tunisianet",
			Reference:     reference,
			BaseReference: getBaseReference(reference),
			InStock:       inStock,
			SearchRank:    *rank,
			Specs:         make(map[string]string),
		}

		// Deep scrape product page for technical specs
		deepC := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
			colly.AllowedDomains("www.tunisianet.com.tn", "tunisianet.com.tn"),
			colly.AllowURLRevisit(),
		)
		deepC.SetRequestTimeout(5 * time.Second)
		deepC.OnHTML("section.product-features dl.data-sheet", func(d *colly.HTMLElement) {
			d.DOM.Find("dt.name").Each(func(i int, sel *goquery.Selection) {
				key := services.NormalizeSpecKey(sel.Text())
				if key == "" {
					return
				}
				val := strings.TrimSpace(sel.NextFiltered("dd.value").Text())
				if val != "" {
					p.Specs[key] = val
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
		if err := deepC.Visit(productURL); err != nil {
			log.Printf("[tunisianet-deep] error scraping %s: %v", productURL, err)
		}

		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	c.OnHTML("h1[itemprop=name], h1.product-title", func(e *colly.HTMLElement) {
		spFound = true
		spName = normalizeName(cleanText(e.Text))
	})

	c.OnHTML("meta[itemprop=price]", func(e *colly.HTMLElement) {
		if content := e.Attr("content"); content != "" {
			if v, err := strconv.ParseFloat(content, 64); err == nil {
				spPrice = v
			}
		}
	})

	c.OnHTML("span[itemprop=price]", func(e *colly.HTMLElement) {
		if spPrice == 0 {
			price, priceText := ExtractPrice(e.Text)
			if price > 0 {
				spPrice = price
				spPriceText = priceText
			}
		}
	})

	c.OnHTML("span.price, .current-price span", func(e *colly.HTMLElement) {
		if spPrice == 0 {
			price, priceText := ExtractPrice(e.Text)
			if price > 0 {
				spPrice = price
				spPriceText = priceText
			}
		}
	})

	c.OnHTML(".product-cover img, img[itemprop=image], .product-image img", func(e *colly.HTMLElement) {
		if spImage == "" {
			src := e.Attr("src")
			if src == "" {
				src = e.Attr("data-src")
			}
			spImage = AbsImageURL("https://www.tunisianet.com.tn", src)
		}
	})

	c.OnHTML(".product-description, .product-information", func(e *colly.HTMLElement) {
		if spDesc == "" {
			spDesc = cleanText(cleanDescription(e.Text))
		}
	})

	c.OnHTML("span.product-reference, span[itemprop=sku]", func(e *colly.HTMLElement) {
		if spReference == "" {
			spReference = cleanReference(e.Text)
		}
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[tunisianet-search] error visiting %s: %v", url, err)
		return 0, false
	}

	if spFound && spName != "" {
		prodURL := finalURL
		if prodURL == "" || prodURL == url {
			prodURL = url
		}
		*rank++
		p := models.Product{
			ID:            GenerateScraperID("tn", prodURL),
			Name:          spName,
			Price:         spPrice,
			PriceText:     cleanText(spPriceText),
			Category:      "",
			Subcategory:   "",
			Description:   spDesc,
			Image:         spImage,
			ProductURL:    prodURL,
			Source:        "Tunisianet",
			Reference:     spReference,
			BaseReference: getBaseReference(spReference),
			SearchRank:    *rank,
			Specs:         make(map[string]string),
		}
		deepC := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
			colly.AllowedDomains("www.tunisianet.com.tn", "tunisianet.com.tn"),
			colly.AllowURLRevisit(),
		)
		deepC.SetRequestTimeout(5 * time.Second)
		deepC.OnHTML("section.product-features dl.data-sheet", func(d *colly.HTMLElement) {
			d.DOM.Find("dt.name").Each(func(i int, sel *goquery.Selection) {
				key := services.NormalizeSpecKey(sel.Text())
				if key == "" {
					return
				}
				val := strings.TrimSpace(sel.NextFiltered("dd.value").Text())
				if val != "" {
					p.Specs[key] = val
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
		if err := deepC.Visit(prodURL); err != nil {
			log.Printf("[tunisianet-deep] error scraping %s: %v", prodURL, err)
		}

		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
		log.Printf("[tunisianet-search] single-product redirect detected: %q (%.3f DT)", spName, spPrice)
	}

	pageProductCount = *rank - pageStartRank
	log.Printf("[tunisianet-search] page scraped, skipped (visited): %d", skippedCount)
	return
}

func SyncTunisianet(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.tunisianet.com.tn", "tunisianet.com.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".stock-status, p.stock, span.stock, .in-stock, .out-of-stock, button[disabled]", func(e *colly.HTMLElement) {
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
		log.Printf("[sync-tunisianet] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-tunisianet] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-tunisianet] synced %s (%s): %.3f DT inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}