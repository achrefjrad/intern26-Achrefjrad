package main

import (
	"context"
	"log"
	"os"
	"time"

	"project/database"
	"project/models"
	"project/routes"
	"project/services"
	"project/storage"
	"project/workers"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func initESIndex(es *storage.ElasticsearchStorage) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := es.InitializeProductIndex(ctx); err != nil {
		log.Printf("Failed to initialize products index: %v", err)
	}
}

func main() {
	e := echo.New()

	// 1. DYNAMIC FRONTEND URL FOR CORS
	frontendURL := os.Getenv("FRONTEND_URL")

	// Explicitly list allowed origins to ensure Render and localhost both work
	allowedOrigins := []string{
		"http://localhost:3000",              // Local development
		"https://frontend-f5ag.onrender.com", // Your live Render frontend
	}

	if frontendURL != "" {
		allowedOrigins = append(allowedOrigins, frontendURL)
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Cache-Control"},
		AllowCredentials: true,
	}))

	if err := database.Init(); err != nil {
		log.Fatalf("Database initialisation failed: %v", err)
	}
	defer database.Close()

	if err := database.CreateSchema(); err != nil {
		log.Fatalf("Schema initialisation failed: %v", err)
	}

	pgStore := &storage.PostgresStorage{}
	esClient := database.GetESClient()
	esStore := storage.NewElasticsearchStorage(esClient)
	storage.Init(pgStore)
	storage.InitES(esStore)
	initESIndex(esStore)

	if err := esStore.InitializeScrapeHistoryIndex(); err != nil {
		e.Logger.Warnf("Failed to initialize scrape_history index: %v", err)
	}

	routes.SetupRoutes(e)

	workers.StartProductSync(6 * time.Hour)
	startFilterSchemaSync()

	// 2. DYNAMIC PORT BINDING
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Fallback for local development
	}

	e.Logger.Info("Server running on port " + port)
	e.Logger.Fatal(e.Start(":" + port))
}

func startFilterSchemaSync() {
	go func() {
		log.Println("[filter-schema] initial scrape on startup...")
		runFilterSchemaScrape()

		ticker := time.NewTicker(7 * 24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("[filter-schema] scheduled weekly scrape...")
			runFilterSchemaScrape()
		}
	}()
}

func runFilterSchemaScrape() {
	results := services.ScrapeFilterSchemas()
	if len(results) == 0 {
		log.Println("[filter-schema] no filter schemas scraped")
		return
	}

	ctx := context.Background()
	synced := 0
	for subcat, filters := range results {
		if err := storage.SaveFilterSchema(ctx, &models.FilterSchema{
			Subcategory: string(subcat),
			Filters:     filters,
		}); err != nil {
			log.Printf("[filter-schema] failed to save %s: %v", subcat, err)
			continue
		}
		synced++
	}
	log.Printf("[filter-schema] synced %d / %d schemas", synced, len(results))
}
