package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"project/database"
	"project/services"
	"project/storage"
)

const updateSpecsSQL = `UPDATE products SET specs = $1, updated_at = NOW() WHERE id = $2`

func main() {
	log.Println("=== Specs Migration ===")

	if err := database.Init(); err != nil {
		log.Fatalf("Database init: %v", err)
	}
	dbPool := database.GetPool()
	if dbPool == nil {
		log.Fatalf("database pool is nil")
	}

	esClient := database.GetESClient()
	if esClient == nil {
		log.Fatal("elasticsearch client not available — ensure ES_URL is set and ES is running")
	}

	products, err := storage.LoadProductsFromPostgres()
	if err != nil {
		log.Fatalf("Loading products from PG: %v", err)
	}
	log.Printf("Loaded %d products", len(products))

	ctx := context.Background()
	updated := 0

	for i := range products {
		p := &products[i]

		services.EnrichProduct(p)

		if len(p.Specs) == 0 {
			continue
		}

		specsJSON, err := json.Marshal(p.Specs)
		if err != nil {
			log.Printf("Error marshalling specs for %s: %v", p.ID, err)
			continue
		}

		_, err = dbPool.Exec(ctx, updateSpecsSQL, specsJSON, p.ID)
		if err != nil {
			log.Printf("Error updating specs for %s: %v", p.ID, err)
			continue
		}

		updated++
	}

	log.Printf("Updated specs for %d / %d products in PostgreSQL", updated, len(products))

	if updated == 0 {
		log.Println("No products to re-index — exiting")
		return
	}

	log.Println("Re-indexing all products to Elasticsearch...")
	if err := storage.BulkIndexProducts(ctx, esClient, products); err != nil {
		log.Fatalf("ES bulk index error: %v", err)
	}

	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := esClient.Indices.Refresh(
		esClient.Indices.Refresh.WithContext(refreshCtx),
		esClient.Indices.Refresh.WithIndex("products"),
	)
	if err != nil {
		log.Printf("ES refresh error (non-fatal): %v", err)
	} else {
		res.Body.Close()
	}

	log.Println("=== Migration complete ===")
}
