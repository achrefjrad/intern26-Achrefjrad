package routes

import (
	"github.com/labstack/echo/v4"
	"project/handlers"
)

func SetupRoutes(e *echo.Echo) {
	e.GET("/health", handlers.HealthCheck)
	e.GET("/search", handlers.SearchProducts)
	e.GET("/search/stream", handlers.SearchStream)
	e.GET("/products/:id", handlers.GetProduct)
	e.GET("/suggest", handlers.SuggestProducts)
	e.GET("/api/filters/schema/:subcategory", handlers.GetFilterSchema)
}
