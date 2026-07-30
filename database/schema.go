package database

import (
	"context"
	"fmt"
	"log"
	"time"
)

const createProductsTable = `
CREATE TABLE IF NOT EXISTS products (
    id          VARCHAR(20)    PRIMARY KEY,
    name        TEXT           NOT NULL,
    description TEXT,
    price       NUMERIC(12,2)  NOT NULL DEFAULT 0,
    image_url   TEXT,
    product_url TEXT,
    category    VARCHAR(100),
    subcategory VARCHAR(100),
    source      VARCHAR(50),
    reference   VARCHAR(100),
    ean         VARCHAR(13),
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
`

const migrationAddReference = `
ALTER TABLE products ADD COLUMN IF NOT EXISTS reference VARCHAR(100);
`

const migrationAddSpecs = `
ALTER TABLE products ADD COLUMN IF NOT EXISTS specs JSONB DEFAULT '{}'::jsonb;
`

const migrationAddEAN = `
ALTER TABLE products ADD COLUMN IF NOT EXISTS ean VARCHAR(13);
`

const createIndexCategory = `
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category);
`

const createIndexSubcategory = `
CREATE INDEX IF NOT EXISTS idx_products_subcategory ON products(subcategory);
`

const createIndexSource = `
CREATE INDEX IF NOT EXISTS idx_products_source ON products(source);
`

const createIndexProductURL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_products_product_url ON products(product_url);
`

func CreateSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, createProductsTable)
	if err != nil {
		return fmt.Errorf("creating products table: %w", err)
	}

	if _, err := pool.Exec(ctx, createIndexCategory); err != nil {
		return fmt.Errorf("creating index on category: %w", err)
	}
	if _, err := pool.Exec(ctx, createIndexSubcategory); err != nil {
		return fmt.Errorf("creating index on subcategory: %w", err)
	}
	if _, err := pool.Exec(ctx, createIndexSource); err != nil {
		return fmt.Errorf("creating index on source: %w", err)
	}

	if _, err := pool.Exec(ctx, createIndexProductURL); err != nil {
		return fmt.Errorf("creating unique index on product_url: %w", err)
	}

	if _, err := pool.Exec(ctx, migrationAddReference); err != nil {
		return fmt.Errorf("migration - adding reference column: %w", err)
	}

	if _, err := pool.Exec(ctx, migrationAddSpecs); err != nil {
		return fmt.Errorf("migration - adding specs column: %w", err)
	}

	if _, err := pool.Exec(ctx, migrationAddEAN); err != nil {
		return fmt.Errorf("migration - adding ean column: %w", err)
	}

	log.Println("Products table initialised successfully")

	if err := createFilterSchemaTable(); err != nil {
		return err
	}

	return createPriceHistoryTable()
}

func createFilterSchemaTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sql := `
	CREATE TABLE IF NOT EXISTS filter_schemas (
	    subcategory VARCHAR(100) PRIMARY KEY,
	    filters     JSONB NOT NULL DEFAULT '{}'::jsonb,
	    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("creating filter_schemas table: %w", err)
	}
	log.Println("Filter schemas table initialised successfully")
	return nil
}

func createPriceHistoryTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const sql = `
	CREATE TABLE IF NOT EXISTS price_history (
	    id          SERIAL PRIMARY KEY,
	    product_id  VARCHAR(20) NOT NULL REFERENCES products(id),
	    price       NUMERIC(12,2) NOT NULL,
	    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_price_history_product_recorded ON price_history(product_id, recorded_at);
	`
	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("creating price_history table: %w", err)
	}
	log.Println("Price history table initialised successfully")
	return nil
}
