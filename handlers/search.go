package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"project/models"
	"project/scrapers"
	"project/services"
	"project/storage"
)

func SearchProducts(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 {
		limit = 24
	}
	if page < 1 {
		page = 1
	}

	q := scrapers.SanitizeQuery(c.QueryParam("q"))
	category := c.QueryParam("category")
	subcategory := c.QueryParam("subcategory")
	if subcategory == "" {
		subcategory = c.QueryParam("sub")
		if subcategory == "" {
			subcategory = c.QueryParam("subcategories")
		}
	}
	sort := c.QueryParam("sort")

	minPrice, _ := strconv.ParseFloat(c.QueryParam("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(c.QueryParam("max_price"), 64)

	specsFilter := make(map[string]string)
	for key, vals := range c.QueryParams() {
		if strings.HasPrefix(key, "spec_") && len(vals) > 0 {
			specsFilter[strings.ToLower(key[5:])] = vals[0]
		}
	}

	if q != "" {
		// ---- Phase 1: Reference VIP Lane (Exact Match) ----
		if services.IsReferenceQuery(q) {
			result, err := storage.GetES().SearchReference(strings.ToUpper(strings.TrimSpace(q)), page, limit)
			if err == nil && result != nil && result.Total > 0 {
				for i := range result.Data {
					result.Data[i].Score = 100.0
				}
				return c.JSON(http.StatusOK, result)
			}
			if err != nil {
				log.Printf("[search] reference query failed: %v", err)
			}
		}

		// ---- Phase 2: Check scrape history ----
		if storage.GetES().HasScrapedRecently(context.Background(), q, 24*time.Hour) {
			candidates, err := storage.GetES().SearchBroad(q, sort, 200)
			if err == nil && len(candidates) > 0 {
				total := len(candidates)
				totalPages := total / limit
				if total%limit > 0 {
					totalPages++
				}
				if totalPages < 1 {
					totalPages = 1
				}
				if page > totalPages {
					page = totalPages
				}

				start := (page - 1) * limit
				end := start + limit
				if start > total {
					return c.JSON(http.StatusOK, models.PaginatedResponse{
						Data:       []models.Product{},
						Page:       page,
						Limit:      limit,
						Total:      total,
						TotalPages: totalPages,
					})
				}
				if end > total {
					end = total
				}

				return c.JSON(http.StatusOK, models.PaginatedResponse{
					Data:       candidates[start:end],
					Page:       page,
					Limit:      limit,
					Total:      total,
					TotalPages: totalPages,
				})
			}
			if err != nil {
				log.Printf("[search] broad elasticsearch query failed: %v", err)
			}

			return c.JSON(http.StatusOK, models.PaginatedResponse{
				Data:       []models.Product{},
				Page:       page,
				Limit:      limit,
				Total:      0,
				TotalPages: 1,
			})
		}

		// ---- Phase 3: Not scraped recently — trigger on-demand scraping ----
		log.Printf("[search] query %q not scraped recently, triggering on-demand scrape", q)

		scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer scrapeCancel()

		ch := make(chan models.Product, 100)
		go func() {
			scrapers.RunSearchScrapersStream(scrapeCtx, q, ch, 8)
			close(ch)
		}()

		persistCtx, persistCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer persistCancel()

		batch := make([]models.Product, 0, 10)
		var totalScraped int

		for p := range ch {
			services.EnrichProduct(&p)
			batch = append(batch, p)
			if len(batch) >= 10 {
				totalScraped += len(batch)
				if err := storage.GetStorage().AppendProducts(persistCtx, batch); err != nil {
					log.Printf("[search-on-demand] batch PG persist error: %v", err)
				}
				if err := storage.GetES().AppendProducts(persistCtx, batch); err != nil {
					log.Printf("[search-on-demand] batch ES index error: %v", err)
				}
				batch = batch[:0]
			}
		}

		if len(batch) > 0 {
			totalScraped += len(batch)
			if err := storage.GetStorage().AppendProducts(persistCtx, batch); err != nil {
				log.Printf("[search-on-demand] final batch PG persist error: %v", err)
			}
			if err := storage.GetES().AppendProducts(persistCtx, batch); err != nil {
				log.Printf("[search-on-demand] final batch ES index error: %v", err)
			}
		}

		if totalScraped > 0 {
			if err := storage.GetES().RefreshIndex(persistCtx); err != nil {
				log.Printf("[search-on-demand] ES refresh error: %v", err)
			}
		}

		storage.GetES().MarkAsScraped(persistCtx, q)

		// ---- Phase 4: Re-query after scrape ----
		candidates, err := storage.GetES().SearchBroad(q, sort, 200)
		if err == nil && len(candidates) > 0 {
			total := len(candidates)
			totalPages := total / limit
			if total%limit > 0 {
				totalPages++
			}
			if totalPages < 1 {
				totalPages = 1
			}
			if page > totalPages {
				page = totalPages
			}
			start := (page - 1) * limit
			end := start + limit
			if start > total {
				return c.JSON(http.StatusOK, models.PaginatedResponse{
					Data:       []models.Product{},
					Page:       page,
					Limit:      limit,
					Total:      total,
					TotalPages: totalPages,
				})
			}
			if end > total {
				end = total
			}
			return c.JSON(http.StatusOK, models.PaginatedResponse{
				Data:       candidates[start:end],
				Page:       page,
				Limit:      limit,
				Total:      total,
				TotalPages: totalPages,
			})
		}
		if err != nil {
			log.Printf("[search] broad post-scrape ES query failed: %v", err)
		}

		return c.JSON(http.StatusOK, models.PaginatedResponse{
			Data:       []models.Product{},
			Page:       page,
			Limit:      limit,
			Total:      0,
			TotalPages: 1,
		})
	}

	// ---- No search term — browse mode ----
	result, err := storage.GetES().Search("", category, subcategory, sort, page, limit, specsFilter, minPrice, maxPrice)
	if err != nil {
		log.Printf("[search] ES search error: %v", err)
		return c.JSON(http.StatusOK, models.PaginatedResponse{
			Data:       []models.Product{},
			Page:       page,
			Limit:      limit,
			Total:      0,
			TotalPages: 1,
		})
	}
	return c.JSON(http.StatusOK, result)
}

func filterProducts(products []models.Product, category, subcategory string) []models.Product {
	if category == "" && subcategory == "" {
		return products
	}
	filtered := make([]models.Product, 0, len(products))
	for _, p := range products {
		if category != "" && p.Category != category {
			continue
		}
		if subcategory != "" && p.Subcategory != subcategory {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

// productListKey returns a deterministic string fingerprint of a product slice
// by concatenating all product IDs in order. Used to detect whether an updated
// SearchBroad result is identical to the previously sent list, avoiding
// redundant full-grid replacements on the frontend.
func productListKey(products []models.Product) string {
	if len(products) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(products) * 12)
	for _, p := range products {
		b.WriteString(p.ID)
		b.WriteByte(',')
	}
	return b.String()
}

func flushScoredBatch(q, sortOrder, category, subcategory string, batch []models.Product) []models.Product {
	if len(batch) == 0 {
		return nil
	}

	bgCtx := context.Background()

	if err := storage.GetES().AppendProducts(bgCtx, batch); err != nil {
		log.Printf("[sse] ES index error during flush: %v", err)
	}

	if err := storage.GetES().RefreshIndex(bgCtx); err != nil {
		log.Printf("[sse] ES refresh error: %v", err)
	}

	candidates, err := storage.GetES().SearchBroad(q, sortOrder, 200)
	if err != nil {
		log.Printf("[sse] SearchBroad error during flush: %v", err)
		return nil
	}

	return filterProducts(candidates, category, subcategory)
}

func SearchStream(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	if q == "" {
		http.Error(c.Response().Writer, "query parameter q is required", http.StatusBadRequest)
		return nil
	}
	category := c.QueryParam("category")
	subcategory := c.QueryParam("subcategory")
	sortOrder := c.QueryParam("sort")

	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no")

	ctx := c.Request().Context()

	sseChan := make(chan []models.Product, 5)

	go func() {
		defer close(sseChan)

		bgCtx := context.Background()
		var lastSentKey string

		sendIfChanged := func(candidates []models.Product) {
			key := productListKey(candidates)
			if len(candidates) > 0 && key == lastSentKey {
				return
			}
			lastSentKey = key
			select {
			case sseChan <- candidates:
			default:
			}
		}

		// ---- Reference VIP Lane ----
		if services.IsReferenceQuery(q) {
			result, err := storage.GetES().SearchReference(strings.ToUpper(strings.TrimSpace(q)), 1, 100)
			if err == nil && result != nil && result.Total > 0 {
				sendIfChanged(filterProducts(result.Data, category, subcategory))
				return
			}
			if err != nil {
				log.Printf("[sse] reference query failed: %v", err)
			}
		}

		// ---- Check scrape history BEFORE sending old data ----
		if storage.GetES().HasScrapedRecently(bgCtx, q, 24*time.Hour) {
			log.Printf("[sse] Query %q recently scraped, serving from ES", q)
			candidates, err := storage.GetES().SearchBroad(q, sortOrder, 200)
			if err == nil && len(candidates) > 0 {
				sendIfChanged(filterProducts(candidates, category, subcategory))
			}
			return
		}

		log.Printf("[sse] Query history not found or expired for %q, triggering live scrape", q)

		// ---- Live scrape with detached background context ----
		scrapeOutput := make(chan models.Product, 100)
		go func() {
			scrapers.RunSearchScrapersStream(bgCtx, q, scrapeOutput, 8)
			close(scrapeOutput)
		}()

		batch := make([]models.Product, 0, 24)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		flush := func() {
			if len(batch) == 0 {
				return
			}
			batchCopy := make([]models.Product, len(batch))
			copy(batchCopy, batch)
			batch = batch[:0]
			sendIfChanged(flushScoredBatch(q, sortOrder, category, subcategory, batchCopy))
		}

		for {
			select {
			case p, ok := <-scrapeOutput:
				if !ok {
					flush()
					storage.GetES().MarkAsScraped(bgCtx, q)
					return
				}
				services.EnrichProduct(&p)
				batch = append(batch, p)

				go func(prod models.Product) {
					if err := storage.GetStorage().SaveProduct(context.Background(), &prod); err != nil {
						log.Printf("[sse] PG save error: %v", err)
					}
				}(p)

				if len(batch) >= 24 {
					flush()
				}

			case <-ticker.C:
				flush()
			}
		}
	}()

loop:
	for {
		select {
		case products, ok := <-sseChan:
			if !ok {
				break loop
			}
			data, err := json.Marshal(products)
			if err != nil {
				log.Printf("[sse] marshal error: %v", err)
				continue
			}
			_, err = fmt.Fprintf(resp.Writer, "data: %s\n\n", data)
			if err != nil {
				break loop
			}
			resp.Flush()

		case <-ctx.Done():
			break loop
		}
	}

	if ctx.Err() == nil {
		_, err := fmt.Fprintf(resp.Writer, "event: close\ndata: {}\n\n")
		if err == nil {
			resp.Flush()
		}
	}

	return nil
}
