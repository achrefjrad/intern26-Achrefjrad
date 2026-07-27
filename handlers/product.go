package handlers

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"
	"project/models"
	"project/storage"
)

func GetProduct(c echo.Context) error {
	id := c.Param("id")

	product, err := storage.GetES().GetProductByID(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if product == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "product not found"})
	}

	resp := models.ProductDetailResponse{
		Product: *product,
		Offers:  []models.Product{},
	}

	offers := []models.Product{}

	// Primary search: match on reference, base_reference, or ean
	// only if at least one identifier is present
	if product.EAN != "" || product.Reference != "" || product.BaseReference != "" {
		offersResult, err := storage.GetES().SearchOffers(
			product.Reference,
			product.BaseReference,
			product.EAN,
			product.ID,
			1, 100,
		)
		if err == nil && offersResult != nil && len(offersResult.Data) > 0 {
			offers = offersResult.Data
		}
	}

	// Fallback: if fewer than 2 offers, try title-based similarity search
	if len(offers) < 2 {
		titleResult, titleErr := storage.GetES().SearchOffersByTitle(
			product.Name,
			product.ID,
			product.Category,
			product.Subcategory,
			1, 100,
		)
		if titleErr == nil && titleResult != nil && len(titleResult.Data) > 0 {
			offers = titleResult.Data
		}
	}

	resp.Offers = offers

	sort.Slice(resp.Offers, func(i, j int) bool {
		return resp.Offers[i].Price < resp.Offers[j].Price
	})

	return c.JSON(http.StatusOK, resp)
}