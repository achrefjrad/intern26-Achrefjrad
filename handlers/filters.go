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
	"taille ecran":           "taille_de_l_ecran",
	"taille de l'ecran":      "taille_de_l_ecran",
	"capacite de stockage":   "stockage",
	"disque dur":             "stockage",
	"systeme":                "os",
	"systeme d'exploitation": "os",
	"carte graphique":        "carte_graphique",
	// Santé & Beauté mappings
	"texture de soin":        "texture",
	"texture":                "texture",
	"ingredients actifs":     "ingredients_actifs",
	"type de peaux":          "type_de_peau",
	"type de peau":           "type_de_peau",
	"besoin cheveux":         "besoin",
	"par besoin":             "besoin",
	"type de cheveux":        "type_de_cheveux",
	"type de produits cheveux": "type_de_produit",
	"type de produit":        "type_de_produit",
	"contenance":             "contenance",
	"zone":                   "zone",
	"finition":               "finition",
	"categories":             "categorie",
	"categorie":              "categorie",
	"outils cheveux":         "outils_cheveux",
	"puissance":              "puissance",
	// Accessoires
	"type":                   "type",
	"type d'accessoire":      "type",
	"capacite":               "capacite",
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

	// Build lowercase-indexed ES counts for case-insensitive matching
	esCountsLower := make(map[string]map[string]int, len(esCounts))
	for esKey, valCounts := range esCounts {
		lowerMap := make(map[string]int, len(valCounts))
		for val, count := range valCounts {
			lowerMap[strings.ToLower(val)] = count
		}
		esCountsLower[esKey] = lowerMap
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
		fieldCounts := esCountsLower[esKey]

		group := models.FilterGroup{
			Name:   groupName,
			Key:    esKey,
			Values: make([]models.FilterValue, 0, len(values)),
		}

		for _, val := range values {
			count := 0
			if fieldCounts != nil {
				count = fieldCounts[strings.ToLower(val)]
			}
			group.Values = append(group.Values, models.FilterValue{
				Value: val,
				Count: count,
			})
		}
		enriched.Filters = append(enriched.Filters, group)
	}

	// Ensure "marque" is at index 0
	marqueIdx := -1
	for i, g := range enriched.Filters {
		if strings.ToLower(g.Key) == "marque" {
			marqueIdx = i
			break
		}
	}
	if marqueIdx > 0 {
		g := enriched.Filters[marqueIdx]
		enriched.Filters = append(enriched.Filters[:marqueIdx], enriched.Filters[marqueIdx+1:]...)
		enriched.Filters = append([]models.FilterGroup{g}, enriched.Filters...)
	}

	return c.JSON(http.StatusOK, enriched)
}
