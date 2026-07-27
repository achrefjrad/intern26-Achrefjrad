package workers

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"project/database"
	"project/models"
	"project/scrapers"
	"project/services"
	"project/storage"
)

var syncingActive atomic.Bool

const batchSize = 50

func RunProductSync() (int, time.Duration) {
	start := time.Now()
	pool := database.GetPool()
	if pool == nil {
		log.Println("[product-sync] database not available")
		return 0, time.Since(start)
	}

	totalUpdated := 0
	offset := 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		rows, err := pool.Query(ctx, `
			SELECT id, name, description, price, image_url, product_url, category, subcategory, source, reference
			FROM products
			ORDER BY id
			LIMIT $1 OFFSET $2
		`, batchSize, offset)
		cancel()
		if err != nil {
			log.Printf("[product-sync] query error: %v", err)
			break
		}

		var batch []models.Product
		for rows.Next() {
			var p models.Product
			if err := rows.Scan(
				&p.ID, &p.Name, &p.Description, &p.Price,
				&p.Image, &p.ProductURL, &p.Category,
				&p.Subcategory, &p.Source, &p.Reference,
			); err != nil {
				log.Printf("[product-sync] scan error: %v", err)
				continue
			}
			batch = append(batch, p)
		}
		rows.Close()

		if len(batch) == 0 {
			break
		}

		updated := syncProductBatch(batch)
		if len(updated) > 0 {
			if err := storage.GetStorage().SaveProducts(context.Background(), updated); err != nil {
				log.Printf("[product-sync] save to PG error: %v", err)
			}
			if err := storage.GetES().AppendProducts(context.Background(), updated); err != nil {
				log.Printf("[product-sync] save to ES error: %v", err)
			}
			totalUpdated += len(updated)
			log.Printf("[product-sync] batch offset=%d: %d updated", offset, len(updated))
		}

		offset += batchSize

		if offset > 10000 {
			log.Printf("[product-sync] safety limit of 10000 products reached")
			break
		}
	}

	log.Printf("[product-sync] finished: %d products updated in %v", totalUpdated, time.Since(start))
	return totalUpdated, time.Since(start)
}

func StartProductSync(interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if !syncingActive.CompareAndSwap(false, true) {
				log.Println("[product-sync] previous sync still running, skipping this cycle")
				continue
			}
			log.Println("[product-sync] automatic sync started")
			count, duration := RunProductSync()
			log.Printf("[product-sync] sync completed: products=%d duration=%s", count, duration)
			syncingActive.Store(false)
		}
	}()
}

func syncProductBatch(products []models.Product) []models.Product {
	var mu sync.Mutex
	var updated []models.Product
	var wg sync.WaitGroup

	for i := 0; i < len(products); i += 5 {
		end := i + 5
		if end > len(products) {
			end = len(products)
		}

		for _, p := range products[i:end] {
			wg.Add(1)
			go func(original models.Product) {
				defer wg.Done()
				result := rescrapeProductPage(original)
				if result != nil {
					services.EnrichProduct(result)
					mu.Lock()
					updated = append(updated, *result)
					mu.Unlock()
				}
			}(p)
		}
		wg.Wait()

		if end < len(products) {
			time.Sleep(1 * time.Second)
		}
	}

	return updated
}

func rescrapeProductPage(original models.Product) *models.Product {
	switch {
	case original.Source == "Tunisianet":
		return scrapers.SyncTunisianet(original)
	case original.Source == "MyTech":
		return scrapers.SyncMyTech(original)
	case original.Source == "wiki.tn":
		return scrapers.SyncWiki(original)
	case original.Source == "SBS Informatique":
		return scrapers.SyncSBS(original)
	case original.Source == "BeautyStore":
		return scrapers.SyncBeautyStore(original)
	case original.Source == "Fatales":
		return scrapers.SyncFatales(original)
	case original.Source == "BigDepot":
		return scrapers.SyncBigDepot(original)
	case original.Source == "EGM":
		return scrapers.SyncEGM(original)
	}
	return nil
}


