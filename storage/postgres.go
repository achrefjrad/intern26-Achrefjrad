package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"project/database"
	"project/models"
)

const upsertProductSQL = `
INSERT INTO products (id, name, description, price, image_url, product_url, category, subcategory, source, reference, ean, specs, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
ON CONFLICT (product_url) DO UPDATE SET
    id          = EXCLUDED.id,
    name        = EXCLUDED.name,
    description = EXCLUDED.description,
    price       = EXCLUDED.price,
    image_url   = EXCLUDED.image_url,
    category    = EXCLUDED.category,
    subcategory = EXCLUDED.subcategory,
    source      = EXCLUDED.source,
    reference   = EXCLUDED.reference,
    ean         = EXCLUDED.ean,
    specs       = EXCLUDED.specs,
    updated_at  = NOW()
`

type PostgresStorage struct{}

func (p *PostgresStorage) SaveProduct(ctx context.Context, product *models.Product) error {
	pool := database.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialised")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	specsJSON, err := json.Marshal(product.Specs)
	if err != nil {
		return fmt.Errorf("marshalling specs for product %s: %w", product.ID, err)
	}

	_, err = pool.Exec(ctx, upsertProductSQL,
		product.ID,
		product.Name,
		product.Description,
		product.Price,
		product.Image,
		product.ProductURL,
		product.Category,
		product.Subcategory,
		product.Source,
		product.Reference,
		product.EAN,
		specsJSON,
	)
	if err != nil {
		return fmt.Errorf("upserting product %s: %w", product.ID, err)
	}
	return nil
}

func (p *PostgresStorage) SaveProducts(ctx context.Context, products []models.Product) error {
	pool := database.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialised")
	}

	if len(products) == 0 {
		return nil
	}

	writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx, err := pool.Begin(writeCtx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(writeCtx)

	batch := &pgx.Batch{}
	for _, product := range products {
		specsJSON, err := json.Marshal(product.Specs)
		if err != nil {
			return fmt.Errorf("marshalling specs for product %s: %w", product.ID, err)
		}
		batch.Queue(upsertProductSQL,
			product.ID,
			product.Name,
			product.Description,
			product.Price,
			product.Image,
			product.ProductURL,
			product.Category,
			product.Subcategory,
			product.Source,
			product.Reference,
			product.EAN,
			specsJSON,
		)
	}

	br := tx.SendBatch(writeCtx, batch)
	upserted := 0
	for i := 0; i < len(products); i++ {
		_, err := br.Exec()
		if err != nil {
			br.Close()
			return fmt.Errorf("upserting product at index %d: %w", i, err)
		}
		upserted++
	}
	br.Close()

	if err := tx.Commit(writeCtx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (p *PostgresStorage) AppendProducts(ctx context.Context, products []models.Product) error {
	return p.SaveProducts(ctx, products)
}

const selectAllProductsSQL = `
SELECT id, name, description, price, image_url, product_url, category, subcategory, source, reference, ean, specs
FROM products
ORDER BY source, id
`

func LoadProductsFromPostgres() ([]models.Product, error) {
	pool := database.GetPool()
	if pool == nil {
		return nil, fmt.Errorf("database not initialised")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, selectAllProductsSQL)
	if err != nil {
		return nil, fmt.Errorf("querying products: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		var specsBytes []byte
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.Image,
			&p.ProductURL,
			&p.Category,
			&p.Subcategory,
			&p.Source,
			&p.Reference,
			&p.EAN,
			&specsBytes,
		); err != nil {
			return nil, fmt.Errorf("scanning product row: %w", err)
		}
		if len(specsBytes) > 0 {
			if err := json.Unmarshal(specsBytes, &p.Specs); err != nil {
				return nil, fmt.Errorf("unmarshalling specs for product %s: %w", p.ID, err)
			}
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating product rows: %w", err)
	}

	return products, nil
}

func SaveFilterSchema(ctx context.Context, schema *models.FilterSchema) error {
	pool := database.GetPool()
	if pool == nil {
		return fmt.Errorf("database not initialised")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filtersJSON, err := json.Marshal(schema.Filters)
	if err != nil {
		return fmt.Errorf("marshalling filters for %s: %w", schema.Subcategory, err)
	}

	const sql = `
	INSERT INTO filter_schemas (subcategory, filters, updated_at)
	VALUES ($1, $2, NOW())
	ON CONFLICT (subcategory) DO UPDATE SET
	    filters    = EXCLUDED.filters,
	    updated_at = NOW()
	`
	_, err = pool.Exec(ctx, sql, schema.Subcategory, filtersJSON)
	if err != nil {
		return fmt.Errorf("saving filter schema for %s: %w", schema.Subcategory, err)
	}
	return nil
}

func GetFilterSchema(ctx context.Context, subcategory string) (*models.FilterSchema, error) {
	pool := database.GetPool()
	if pool == nil {
		return nil, fmt.Errorf("database not initialised")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	const sql = `SELECT subcategory, filters, updated_at FROM filter_schemas WHERE subcategory = $1`

	var s models.FilterSchema
	var filtersBytes []byte
	err := pool.QueryRow(ctx, sql, subcategory).Scan(&s.Subcategory, &filtersBytes, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying filter schema for %s: %w", subcategory, err)
	}
	if len(filtersBytes) > 0 {
		if err := json.Unmarshal(filtersBytes, &s.Filters); err != nil {
			return nil, fmt.Errorf("unmarshalling filters for %s: %w", subcategory, err)
		}
	}
	return &s, nil
}
