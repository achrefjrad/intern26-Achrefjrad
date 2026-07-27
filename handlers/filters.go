package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"project/storage"
)

func GetFilterSchema(c echo.Context) error {
	subcategory := c.Param("subcategory")
	if subcategory == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "subcategory is required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schema, err := storage.GetFilterSchema(ctx, subcategory)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"subcategory": subcategory,
				"filters":     map[string][]string{},
			})
		}
		log.Printf("[filters] error fetching schema for %s: %v", subcategory, err)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"subcategory": subcategory,
			"filters":     map[string][]string{},
		})
	}

	return c.JSON(http.StatusOK, schema)
}
