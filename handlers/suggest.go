package handlers

import (
	"log"
	"net/http"

	"project/storage"

	"github.com/labstack/echo/v4"
)

func SuggestProducts(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return c.JSON(http.StatusOK, []string{})
	}

	suggestions, err := storage.GetES().Suggest(q)
	if err != nil {
		log.Printf("[suggest] ES suggest error: %v", err)
		return c.JSON(http.StatusOK, []string{})
	}

	return c.JSON(http.StatusOK, suggestions)
}