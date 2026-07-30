package handlers

import (
	"math"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"project/database"
	"project/models"
	"project/storage"
)

func GetPriceHistory(c echo.Context) error {
	productID := c.Param("id")

	product, err := storage.GetES().GetProductByID(productID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if product == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "product not found"})
	}

	pool := database.GetPool()
	if pool == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "database unavailable"})
	}

	rows, err := pool.Query(c.Request().Context(),
		`SELECT price, recorded_at FROM price_history WHERE product_id = $1 ORDER BY recorded_at ASC`,
		productID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query failed"})
	}
	defer rows.Close()

	var history []models.PriceHistoryPoint
	for rows.Next() {
		var price float64
		var recordedAt time.Time
		if err := rows.Scan(&price, &recordedAt); err != nil {
			continue
		}
		history = append(history, models.PriceHistoryPoint{
			Date:  recordedAt.Format("2006-01-02"),
			Price: price,
		})
	}

	var changePct float64
	if len(history) >= 2 {
		first := history[0].Price
		last := history[len(history)-1].Price
		if first != 0 {
			changePct = math.Round((last-first)/first*10000) / 100
		}
	}

	resp := models.PriceHistoryResponse{
		CurrentPrice:     product.Price,
		ChangePercentage: changePct,
		History:          history,
	}

	return c.JSON(http.StatusOK, resp)
}
