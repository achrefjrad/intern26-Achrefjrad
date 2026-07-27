package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"project/models"
	"project/services"
)

const mytekMediaBase = "https://mk-media.mytek.tn/media/catalog/product"

const mytekSearchBase = "https://www.mytek.tn/catalogsearch/result/"

type MyTechSearchScraper struct{}

func (m MyTechSearchScraper) ScrapeSearch(ctx context.Context, query string, ch chan<- models.Product) {
	query = SanitizeQuery(query)
	encodedQuery := url.QueryEscape(query) + "&product_list_order=relevance"
	rank := 0
	emptyCount := 0

	for page := 1; page <= 3; page++ {
		select {
		case <-ctx.Done():
			log.Printf("[mytech-search] context cancelled after %d pages", page-1)
			return
		default:
		}

		searchURL := fmt.Sprintf("%s?q=%s", mytekSearchBase, encodedQuery)
		if page > 1 {
			searchURL = fmt.Sprintf("%s&p=%d", searchURL, page)
		}

		log.Printf("[mytech-search] scraping page %d: %s", page, searchURL)
		pageProductCount, hasNext := scrapeMyTechSearchPage(ctx, searchURL, ch, &rank)
		log.Printf("[mytech-search] page %d products found: %d, hasNext: %v", page, pageProductCount, hasNext)

		if !hasNext {
			break
		}
		if pageProductCount == 0 {
			emptyCount++
			if emptyCount >= 2 {
				log.Printf("[mytech-search] stopping after %d consecutive empty pages", emptyCount)
				break
			}
			continue
		}
		emptyCount = 0
	}

	log.Printf("[mytech-search] total products for %q: %d (ranks assigned)", query, rank)
}

func extractMytechFallbackProduct(e *colly.HTMLElement, visited map[string]bool, skippedCount *int) *models.Product {
	productURL := e.ChildAttr("a.product-item-link, a.product-item-photo, a.product.photo, a", "href")
	if productURL == "" {
		return nil
	}
	normalized := normalizeURL(productURL)
	if visited[normalized] {
		*skippedCount++
		return nil
	}
	visited[normalized] = true

	name := strings.TrimSpace(e.ChildText("a.product-item-link, .product-name, .product-item-name a, .product-item-link"))
	if name == "" {
		name = strings.TrimSpace(e.ChildText("h2 a, h3 a, strong a"))
	}
	if name == "" {
		return nil
	}

	price := -1.0
	var priceText string

	priceRaw := strings.TrimSpace(e.ChildAttr(".price-wrapper", "data-price-amount"))
	if priceRaw == "" {
		priceRaw = strings.TrimSpace(e.ChildAttr("[data-price-amount]", "data-price-amount"))
	}
	if priceRaw != "" {
		if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
			price = v
			priceText = priceRaw + " DT"
		}
	}

	if price == -1.0 {
		priceRaw = strings.TrimSpace(e.ChildText(".final-price"))
		if priceRaw == "" {
			priceSelection := e.DOM.Find(".price-wrapper .price, .price-box .price, span.price, .special-price .price, .price")
			priceRaw = strings.TrimSpace(priceSelection.First().Text())
		}
		if priceRaw != "" {
			priceRaw = strings.ReplaceAll(priceRaw, "\u00a0", " ")
			priceRaw = strings.ReplaceAll(priceRaw, "DT", "")
			priceRaw = strings.ReplaceAll(priceRaw, "TND", "")
			priceRaw = strings.TrimSpace(priceRaw)
			priceRaw = strings.ReplaceAll(priceRaw, " ", "")
			priceRaw = strings.ReplaceAll(priceRaw, ",", ".")
			if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
				price = v
				priceText = priceRaw + " DT"
			}
		}
	}

	if price == -1.0 {
		priceStr := strings.TrimSpace(e.Attr("data-final-price"))
		if priceStr == "" {
			priceStr = e.Attr("data-price")
		}
		if priceStr != "" {
			if v, err := strconv.ParseFloat(priceStr, 64); err == nil {
				price = v
				priceText = priceStr + " DT"
			}
		}
	}

	if price == -1.0 {
		log.Printf("[mytech-search] WARNING: could not extract price for %s from any source", name)
		priceText = ""
	}

	imageURL := e.ChildAttr("img.product-image-photo", "src")
	if imageURL == "" {
		imageURL = e.ChildAttr("img.product-image-photo", "data-src")
	}
	if imageURL == "" {
		imageURL = e.ChildAttr("img.product-image-photo", "data-lazy-src")
	}
	if imageURL == "" {
		imageURL = e.ChildAttr("img[src*='media/catalog']", "src")
	}
	if imageURL == "" {
		srcset := e.ChildAttr("img", "srcset")
		if srcset != "" {
			parts := strings.Fields(srcset)
			if len(parts) > 0 {
				imageURL = parts[0]
			}
		}
	}
	if imageURL == "" {
		imageURL = e.ChildAttr("img", "src")
	}
	if imageURL == "" {
		imageURL = e.ChildAttr("img", "data-src")
	}
	if imageURL != "" && !strings.HasPrefix(imageURL, "http") && !strings.HasPrefix(imageURL, "//") {
		imageURL = mytekMediaBase + "/" + strings.TrimPrefix(imageURL, "/")
	}

	descRaw := strings.TrimSpace(e.ChildText(".product-item-description, .description, .product.description"))
	description := cleanText(descRaw)

		refSel := e.DOM.Find("div.sku, span[itemprop=sku], .sku")
		reference := cleanReference(refSel.First().Text())
		if reference == "" {
			reference = cleanReference(e.Attr("data-sku"))
		}

		hashID := GenerateScraperID("my", productURL)

		inStock := true
		if e.DOM.Find(".stock.unavailable, .product-item.out-of-stock, .unavailable").Length() > 0 {
			inStock = false
		}
		if inStock {
			itemText := strings.ToLower(e.Text)
			if strings.Contains(itemText, "épuisé") || strings.Contains(itemText, "rupture") || strings.Contains(itemText, "indisponible") || strings.Contains(itemText, "en arrivage") {
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
			Description:    description,
			Image:          imageURL,
			ProductURL:     productURL,
			Source:         "MyTech",
			Reference:      reference,
			BaseReference:  getBaseReference(reference),
			InStock:        inStock,
			Specs:          make(map[string]string),
		}
}

func deepScrapeMyTechSpecs(ctx context.Context, parent *colly.Collector, productURL string, p *models.Product) {
	deepC := parent.Clone()
	deepC.SetRequestTimeout(5 * time.Second)
	deepC.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})
	deepC.OnHTML("table#product-attribute-specs-table tr, .data.table.additional-attributes tr", func(e *colly.HTMLElement) {
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
		log.Printf("[mytech-deep] error scraping %s: %v", productURL, err)
	}
}

func scrapeMyTechSearchPage(ctx context.Context, url string, ch chan<- models.Product, rank *int) (pageProductCount int, hasNext bool) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.mytek.tn", "mytek.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(30 * time.Second)

	hasNext = false
	visited := make(map[string]bool)
	skippedCount := 0
	countSeoData := 0
	countFallback := 0
	pageStartRank := *rank

	var finalURL string
	var spName, spImage, spDesc string
	var spPrice float64
	var spPriceText string
	spFound := false
	spPageBody := false
	var spReference string

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

	c.OnHTML("a[rel=next]", func(e *colly.HTMLElement) {
		hasNext = true
	})

	c.OnHTML("div[id=seo-product-data] div[data-id]", func(e *colly.HTMLElement) {
		countSeoData++
		productURL := strings.TrimSpace(e.Attr("data-url"))
		if productURL == "" {
			return
		}
		normalized := normalizeURL(productURL)
		if visited[normalized] {
			skippedCount++
			return
		}
		visited[normalized] = true

		name := strings.TrimSpace(e.Attr("data-name"))
		if name == "" {
			return
		}

		price := -1.0
		var priceText string
		var priceRaw string

		priceStr := e.Attr("data-final-price")
		if priceStr == "" {
			priceStr = e.Attr("data-price")
		}
		if priceStr != "" {
			v, err := strconv.ParseFloat(priceStr, 64)
			if err == nil {
				price = v
				priceText = priceStr + " DT"
			} else {
				log.Printf("[mytech-search] WARNING: could not parse data-attribute price %q for %s: %v", priceStr, name, err)
			}
		}

		if price == -1.0 {
			priceRaw = strings.TrimSpace(e.ChildAttr(".price-wrapper", "data-price-amount"))
			if priceRaw == "" {
				priceRaw = strings.TrimSpace(e.ChildAttr("[data-price-amount]", "data-price-amount"))
			}
			if priceRaw != "" {
				if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
					price = v
					priceText = priceRaw + " DT"
				}
			}
		}

		if price == -1.0 {
			priceRaw = strings.TrimSpace(e.ChildText(".final-price"))
			if priceRaw == "" {
				priceRaw = strings.TrimSpace(e.ChildText(".price-wrapper .price, .price-box .price, span.price, .special-price .price"))
			}
			if priceRaw != "" {
				priceRaw = strings.ReplaceAll(priceRaw, "\u00a0", " ")
				priceRaw = strings.ReplaceAll(priceRaw, "DT", "")
				priceRaw = strings.ReplaceAll(priceRaw, "TND", "")
				priceRaw = strings.TrimSpace(priceRaw)
				priceRaw = strings.ReplaceAll(priceRaw, " ", "")
				priceRaw = strings.ReplaceAll(priceRaw, ",", ".")
				if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
					price = v
					priceText = priceRaw + " DT"
				}
			}
		}

		if price == -1.0 {
			log.Printf("[mytech-search] WARNING: could not extract price for %s from any source", name)
			priceText = ""
		}

		imagePath := strings.TrimSpace(e.Attr("data-image"))
		imageURL := ""
		if imagePath != "" {
			if strings.HasPrefix(imagePath, "http") {
				imageURL = imagePath
			} else {
				imagePath = strings.TrimPrefix(imagePath, "/")
				imageURL = mytekMediaBase + "/" + imagePath
			}
		}

		if imageURL == "" {
			imageURL = e.ChildAttr("img.product-image-photo", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img.product-image-photo", "data-src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img.product-image-photo", "data-lazy-src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "src")
		}
		if imageURL == "" {
			imageURL = e.ChildAttr("img", "data-src")
		}
		if imageURL == "" {
			srcset := e.ChildAttr("img", "srcset")
			if srcset != "" {
				parts := strings.Fields(srcset)
				if len(parts) > 0 {
					imageURL = parts[0]
				}
			}
		}

		descRaw := strings.TrimSpace(e.Attr("data-description"))
		description := cleanText(descRaw)

		reference := cleanReference(e.Attr("data-sku"))
		if reference == "" {
			reference = cleanReference(e.ChildText("span[itemprop=sku]"))
		}
		if reference == "" {
			reference = cleanReference(e.ChildText("div.sku"))
		}

		hashID := GenerateScraperID("my", productURL)

		inStock := true
		if e.DOM.Find(".stock.unavailable, .product-item.out-of-stock, .unavailable").Length() > 0 {
			inStock = false
		}
		if inStock {
			itemText := strings.ToLower(e.Text)
			if strings.Contains(itemText, "épuisé") || strings.Contains(itemText, "rupture") || strings.Contains(itemText, "indisponible") || strings.Contains(itemText, "en arrivage") {
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
			Source:         "MyTech",
			Reference:      reference,
			BaseReference:  getBaseReference(reference),
			InStock:        inStock,
			SearchRank:     *rank,
			Specs:          make(map[string]string),
		}
		deepScrapeMyTechSpecs(ctx, c, productURL, &p)
		services.EnrichProduct(&p)
		select {
		case ch <- p:
		case <-ctx.Done():
		}
	})

	c.OnHTML("li.item.product", func(e *colly.HTMLElement) {
		countFallback++
		p := extractMytechFallbackProduct(e, visited, &skippedCount)
		if p != nil {
			*rank++
			p.SearchRank = *rank
			deepScrapeMyTechSpecs(ctx, c, p.ProductURL, p)
			services.EnrichProduct(p)
			select {
			case ch <- *p:
			case <-ctx.Done():
			}
		}
	})

	c.OnHTML("body.catalog-product-view", func(e *colly.HTMLElement) {
		spPageBody = true
	})

	c.OnHTML("h1.page-title, span[itemprop=name]", func(e *colly.HTMLElement) {
		spFound = true
		spName = strings.TrimSpace(e.Text)
	})

	c.OnHTML("meta[itemprop=price]", func(e *colly.HTMLElement) {
		if content := e.Attr("content"); content != "" {
			if v, err := strconv.ParseFloat(content, 64); err == nil {
				spPrice = v
			}
		}
	})

	c.OnHTML(".product-info-price span.price, span.price", func(e *colly.HTMLElement) {
		if spPrice == 0 {
			priceRaw := strings.TrimSpace(e.Text)
			priceRaw = strings.ReplaceAll(priceRaw, "\u00a0", " ")
			priceRaw = strings.ReplaceAll(priceRaw, "DT", "")
			priceRaw = strings.ReplaceAll(priceRaw, "TND", "")
			priceRaw = strings.TrimSpace(priceRaw)
			priceRaw = strings.ReplaceAll(priceRaw, " ", "")
			priceRaw = strings.ReplaceAll(priceRaw, ",", ".")
			if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
				spPrice = v
				spPriceText = priceRaw + " DT"
			}
		}
	})

	c.OnHTML(".final-price", func(e *colly.HTMLElement) {
		if spPrice == 0 {
			priceRaw := strings.TrimSpace(e.Text)
			priceRaw = strings.ReplaceAll(priceRaw, "\u00a0", " ")
			priceRaw = strings.ReplaceAll(priceRaw, "DT", "")
			priceRaw = strings.ReplaceAll(priceRaw, "TND", "")
			priceRaw = strings.TrimSpace(priceRaw)
			priceRaw = strings.ReplaceAll(priceRaw, " ", "")
			priceRaw = strings.ReplaceAll(priceRaw, ",", ".")
			if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
				spPrice = v
				spPriceText = priceRaw + " DT"
			}
		}
	})

	c.OnHTML("[data-price-amount]", func(e *colly.HTMLElement) {
		if spPrice == 0 {
			priceRaw := strings.TrimSpace(e.Attr("data-price-amount"))
			if priceRaw != "" {
				if v, err := strconv.ParseFloat(priceRaw, 64); err == nil {
					spPrice = v
					spPriceText = priceRaw + " DT"
				}
			}
		}
	})

	c.OnHTML(".product-image-photo, .gallery-placeholder img, img[itemprop=image]", func(e *colly.HTMLElement) {
		if spImage == "" {
			spImage = e.Attr("src")
			if spImage == "" {
				spImage = e.Attr("data-src")
			}
			if spImage != "" && !strings.HasPrefix(spImage, "http") {
				spImage = mytekMediaBase + "/" + strings.TrimPrefix(spImage, "/")
			}
		}
	})

	c.OnHTML(".product-info-main .product.attribute.description", func(e *colly.HTMLElement) {
		if spDesc == "" {
			spDesc = cleanText(e.Text)
		}
	})

	c.OnHTML(".product-info-main .sku, div.sku, span[itemprop=sku]", func(e *colly.HTMLElement) {
		if spReference == "" {
			spReference = cleanReference(e.Text)
		}
	})

	err := c.Visit(url)
	if err != nil {
		log.Printf("[mytech-search] error visiting %s: %v", url, err)
		return 0, false
	}

	if *rank == pageStartRank && (spPageBody || spFound) && spName != "" {
		prodURL := finalURL
		if prodURL == "" || prodURL == url {
			prodURL = url
		}
		*rank++
		p := models.Product{
			ID:             GenerateScraperID("my", prodURL),
			Name:           spName,
			Price:          spPrice,
			PriceText:      spPriceText,
			Category:       "",
			Subcategory:    "",
			Description:    spDesc,
			Image:          spImage,
			ProductURL:     prodURL,
			Source:         "MyTech",
			Reference:      spReference,
			BaseReference:  getBaseReference(spReference),
			SearchRank:     *rank,
		}
		if spPrice <= 0 {
			p.PriceText = ""
		}
		select {
		case ch <- p:
		case <-ctx.Done():
		}
		log.Printf("[mytech-search] single-product redirect detected: %q (%.3f DT)", spName, spPrice)
	}

	pageProductCount = *rank - pageStartRank
	if countSeoData == 0 && countFallback == 0 {
		log.Printf("[mytech-search] empty DOM: no product selectors matched on page %s", url)
	}
	return
}

func SyncMyTech(original models.Product) *models.Product {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		colly.AllowedDomains("www.mytek.tn", "mytek.tn"),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(15 * time.Second)

	result := original
	result.InStock = true
	found := false

	c.OnHTML(".stock.unavailable, .product-info-stock-sku .unavailable, .stock-status, .product-item-inner span, .price-box span", func(e *colly.HTMLElement) {
		text := strings.ToLower(strings.TrimSpace(e.Text))
		if strings.Contains(text, "rupture") || strings.Contains(text, "épuisé") || strings.Contains(text, "indisponible") || strings.Contains(text, "en arrivage") {
			result.InStock = false
		}
	})

	c.OnHTML("div[id=seo-product-data]", func(e *colly.HTMLElement) {
		priceStr := e.Attr("data-final-price")
		if priceStr == "" {
			priceStr = e.Attr("data-price")
		}
		if priceStr != "" {
			if v, err := strconv.ParseFloat(priceStr, 64); err == nil && v > 0 {
				result.Price = v
				result.PriceText = fmt.Sprintf("%.3f DT", v)
				found = true
			}
		}
	})

	c.OnHTML("span.price-wrapper .price, span.price", func(e *colly.HTMLElement) {
		if !found {
			raw := strings.TrimSpace(e.Text)
			raw = strings.ReplaceAll(raw, "\u00a0", " ")
			raw = strings.ReplaceAll(raw, "DT", "")
			raw = strings.ReplaceAll(raw, "TND", "")
			raw = strings.TrimSpace(raw)
			raw = strings.ReplaceAll(raw, " ", "")
			raw = strings.ReplaceAll(raw, ",", ".")
			if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
				result.Price = v
				result.PriceText = raw + " DT"
				found = true
			}
		}
	})

	err := c.Visit(original.ProductURL)
	if err != nil {
		log.Printf("[sync-mytech] error visiting %s: %v", original.ProductURL, err)
		return nil
	}

	if !found {
		log.Printf("[sync-mytech] no price found for %s (%s)", original.ID, original.Name)
		result.InStock = false
	}

	log.Printf("[sync-mytech] synced %s (%s): %.3f DT inStock=%v", original.ID, original.Name, result.Price, result.InStock)
	return &result
}