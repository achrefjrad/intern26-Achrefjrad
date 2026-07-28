package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"project/models"
	"project/storage"
	"project/types"
)

var schemaKeyMapping = map[string]string{
	"memoire":                "ram",
	"memoire vive (ram)":     "ram",
	"taille ecran":           "ecran",
	"taille de l'ecran":      "ecran",
	"capacite de stockage":   "stockage",
	"disque dur":             "stockage",
	"systeme d'exploitation": "os",
	"carte graphique":        "carte_graphique",
}

func translateSchemaKey(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.NewReplacer(
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"à", "a", "â", "a", "ä", "a",
		"ù", "u", "û", "u", "ü", "u",
		"ô", "o", "ö", "o",
		"î", "i", "ï", "i",
		"ç", "c",
	).Replace(lower)
	if mapped, ok := schemaKeyMapping[lower]; ok {
		return mapped
	}
	key := strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	return strings.Join(key, "_")
}

func GetFilterSchema(c echo.Context) error {
	subcategory := c.Param("subcategory")
	if subcategory == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "subcategory is required"})
	}

	category := c.QueryParam("category")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schema, err := storage.GetFilterSchema(ctx, subcategory)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[filters] error fetching schema for %s: %v", subcategory, err)
		}
		schema = &models.FilterSchema{
			Subcategory: subcategory,
			Filters:     map[string][]string{},
		}
	}

	// Determine category from query param or subcategory lookup
	if category == "" {
		if cat, ok := types.SubcategoryToCategory[types.Subcategory(subcategory)]; ok {
			category = string(cat)
		}
	}

	// Get ES counts
	esCounts := make(map[string]map[string]int)
	if category != "" {
		es := storage.GetES()
		if es != nil {
			counts, err := es.GetFilterCounts(category, subcategory)
			if err != nil {
				log.Printf("[filters] ES count error for %s/%s: %v", category, subcategory, err)
			} else {
				esCounts = counts
			}
		}
	}

	// Build enriched response
	enriched := models.EnrichedFilterSchema{
		Subcategory: subcategory,
		Category:    category,
		UpdatedAt:   schema.UpdatedAt,
	}

	for groupName, values := range schema.Filters {
		if len(values) == 0 {
			continue
		}
		esKey := translateSchemaKey(groupName)
		fieldCounts := esCounts[esKey]

		group := models.FilterGroup{
			Name:   groupName,
			Key:    esKey,
			Values: make([]models.FilterValue, 0, len(values)),
		}

		for _, val := range values {
			count := 0
			if fieldCounts != nil {
				count = fieldCounts[val]
			}
			group.Values = append(group.Values, models.FilterValue{
				Value: val,
				Count: count,
			})
		}
		enriched.Filters = append(enriched.Filters, group)
	}

	return c.JSON(http.StatusOK, enriched)
}
