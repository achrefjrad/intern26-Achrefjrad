package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"project/models"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
)

var accessoryKeywords = []string{"cable", "chargeur", "batterie", "housse", "adaptateur", "etui", "ecouteur", "support", "glass", "coque", "film", "protecteur", "verre", "silicone", "sacoche", "refroidisseur", "clavier", "souris", "sac", "tapis", "ventilateur", "sac-a-dos"}

type ElasticsearchStorage struct {
	client *elasticsearch.Client
}

func NewElasticsearchStorage(client *elasticsearch.Client) *ElasticsearchStorage {
	return &ElasticsearchStorage{client: client}
}

func (es *ElasticsearchStorage) clientAvailable() bool {
	return es != nil && es.client != nil
}

type esHit struct {
	Source json.RawMessage `json:"_source"`
}

type esHits struct {
	Total map[string]interface{} `json:"total"`
	Hits  []esHit                `json:"hits"`
}

type esSearchResponse struct {
	Took         int                          `json:"took"`
	TimedOut     bool                         `json:"timed_out"`
	Hits         esHits                       `json:"hits"`
	Aggregations map[string]json.RawMessage   `json:"aggregations,omitempty"`
}

type esDocument struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Price         float64           `json:"price"`
	PriceText     string            `json:"price_text"`
	Category      string            `json:"category"`
	Subcategory   string            `json:"subcategory"`
	Description   string            `json:"description"`
	Image         string            `json:"image"`
	URL           string            `json:"url"`
	Source        string            `json:"source"`
	Reference     string            `json:"reference"`
	BaseReference string            `json:"base_reference"`
	EAN           string            `json:"ean"`
	InStock       bool              `json:"in_stock"`
	Specs         map[string]string `json:"specs"`
}

func (es *ElasticsearchStorage) esHitsToProducts(sr *esSearchResponse) []models.Product {
	products := make([]models.Product, 0, len(sr.Hits.Hits))
	for _, hit := range sr.Hits.Hits {
		var doc esDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			log.Printf("[es] unmarshal hit error: %v", err)
			continue
		}
		products = append(products, models.Product{
			ID:            doc.ID,
			Name:          doc.Title,
			Price:         doc.Price,
			PriceText:     doc.PriceText,
			Category:      doc.Category,
			Subcategory:   doc.Subcategory,
			Description:   doc.Description,
			Image:         doc.Image,
			ProductURL:    doc.URL,
			Source:        doc.Source,
			Reference:     doc.Reference,
			BaseReference: doc.BaseReference,
			EAN:           doc.EAN,
			InStock:       doc.InStock,
			Specs:         doc.Specs,
		})
	}
	if products == nil {
		products = []models.Product{}
	}
	return products
}

func (es *ElasticsearchStorage) SearchReference(reference string, page, limit int) (*models.PaginatedResponse, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}
	if reference == "" {
		return nil, fmt.Errorf("no reference provided")
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"reference": map[string]interface{}{
								"value":            reference,
								"case_insensitive": true,
							},
						},
					},
					{
						"match_phrase": map[string]interface{}{
							"title": map[string]interface{}{
								"query": reference,
							},
						},
					},
				},
				"minimum_should_match": 1,
			},
		},
		"from": (page - 1) * limit,
		"size": limit,
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES reference query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch reference search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch reference search response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES response: %w", err)
	}

	total := 0
	if v, ok := sr.Hits.Total["value"]; ok {
		if f, ok := v.(float64); ok {
			total = int(f)
		}
	}

	products := es.esHitsToProducts(&sr)

	totalPages := total / limit
	if total%limit > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	return &models.PaginatedResponse{
		Data:       products,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (es *ElasticsearchStorage) SearchOffers(reference, baseReference, ean, excludeID string, page, limit int) (*models.PaginatedResponse, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}

	if reference == "" && baseReference == "" && ean == "" {
		return &models.PaginatedResponse{
			Data:       []models.Product{},
			Page:       page,
			Limit:      limit,
			Total:      0,
			TotalPages: 1,
		}, nil
	}

	should := []map[string]interface{}{}
	if reference != "" {
		should = append(should, map[string]interface{}{
			"term": map[string]interface{}{
				"reference": map[string]interface{}{
					"value":            reference,
					"case_insensitive": true,
				},
			},
		})
	}
	if baseReference != "" {
		should = append(should, map[string]interface{}{
			"term": map[string]interface{}{
				"base_reference": map[string]interface{}{
					"value":            baseReference,
					"case_insensitive": true,
				},
			},
		})
	}
	if ean != "" {
		should = append(should, map[string]interface{}{
			"term": map[string]interface{}{
				"ean": map[string]interface{}{
					"value":            ean,
					"case_insensitive": true,
				},
			},
		})
	}

	boolQ := map[string]interface{}{
		"should":               should,
		"minimum_should_match": 1,
	}

	if excludeID != "" {
		boolQ["must_not"] = []map[string]interface{}{
			{
				"term": map[string]interface{}{
					"id": map[string]interface{}{
						"value": excludeID,
					},
				},
			},
		}
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQ,
		},
		"from": (page - 1) * limit,
		"size": limit,
		"sort": []map[string]interface{}{
			{"price": map[string]interface{}{"order": "asc"}},
		},
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES offers query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch offers search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch offers search response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES response: %w", err)
	}

	total := 0
	if v, ok := sr.Hits.Total["value"]; ok {
		if f, ok := v.(float64); ok {
			total = int(f)
		}
	}

	products := es.esHitsToProducts(&sr)

	totalPages := total / limit
	if total%limit > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	return &models.PaginatedResponse{
		Data:       products,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (es *ElasticsearchStorage) SearchOffersByTitle(title, excludeID, category, subcategory string, page, limit int) (*models.PaginatedResponse, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}
	if title == "" {
		return nil, fmt.Errorf("no title provided")
	}

	boolQ := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"match": map[string]interface{}{
					"title": map[string]interface{}{
						"query":               title,
						"minimum_should_match": "75%",
					},
				},
			},
		},
	}

	if excludeID != "" {
		boolQ["must_not"] = []map[string]interface{}{
			{
				"term": map[string]interface{}{
					"id": map[string]interface{}{
						"value": excludeID,
					},
				},
			},
		}
	}

	filters := []map[string]interface{}{}
	if subcategory != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"subcategory": map[string]interface{}{
					"value": subcategory,
				},
			},
		})
	} else if category != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"category": map[string]interface{}{
					"value": category,
				},
			},
		})
	}
	if len(filters) > 0 {
		boolQ["filter"] = filters
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQ,
		},
		"from": (page - 1) * limit,
		"size": limit,
		"sort": []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"price": map[string]interface{}{"order": "asc"}},
		},
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES title offers query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch title offers search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch title offers search response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES response: %w", err)
	}

	total := 0
	if v, ok := sr.Hits.Total["value"]; ok {
		if f, ok := v.(float64); ok {
			total = int(f)
		}
	}

	products := es.esHitsToProducts(&sr)

	totalPages := total / limit
	if total%limit > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	return &models.PaginatedResponse{
		Data:       products,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (es *ElasticsearchStorage) GetProductByID(id string) (*models.Product, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}
	if id == "" {
		return nil, fmt.Errorf("no id provided")
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"id": map[string]interface{}{
					"value": id,
				},
			},
		},
		"size": 1,
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES id query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch id search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch id search response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES response: %w", err)
	}

	total := 0
	if v, ok := sr.Hits.Total["value"]; ok {
		if f, ok := v.(float64); ok {
			total = int(f)
		}
	}

	if total == 0 {
		return nil, nil
	}

	products := es.esHitsToProducts(&sr)
	return &products[0], nil
}

func (es *ElasticsearchStorage) SearchBroad(q, sort string, limit int) ([]models.Product, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}
	if q == "" {
		return nil, fmt.Errorf("no query provided")
	}

	isSearchingAccessory := false
	lowerQ := strings.ToLower(q)
	for _, ak := range accessoryKeywords {
		if strings.Contains(lowerQ, ak) {
			isSearchingAccessory = true
			break
		}
	}

	boolQuery := map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"multi_match": map[string]interface{}{
					"query":     q,
					"fields":    []string{"title^3", "description"},
					"fuzziness": "AUTO",
				},
			},
		},
		"should": []map[string]interface{}{
			{
				"match_phrase_prefix": map[string]interface{}{
					"title": map[string]interface{}{
						"query": q,
						"boost": 3.0,
					},
				},
			},
		},
	}

	baseQuery := map[string]interface{}{"bool": boolQuery}

	var rootQuery map[string]interface{}
	if isSearchingAccessory {
		rootQuery = baseQuery
	} else {
		rootQuery = map[string]interface{}{
			"function_score": map[string]interface{}{
				"query": baseQuery,
				"functions": []map[string]interface{}{
					{
						"filter": map[string]interface{}{
							"match": map[string]interface{}{
								"title": strings.Join(accessoryKeywords, " "),
							},
						},
						"weight": 0.1,
					},
				},
				"boost_mode": "multiply",
			},
		}
	}

	// Build sort criteria:
	//   - Price sort is primary when explicitly requested.
	//   - _score is primary for the default relevance ranking.
	//   - _id is always the final tie-breaker so that identical-score or
	//     identical-price results appear in a deterministic, stable order
	//     across repeated searches.
	body := map[string]interface{}{
		"size":  limit,
		"query": rootQuery,
	}

	switch sort {
	case "price_asc":
		body["sort"] = []map[string]interface{}{
			{"price": map[string]interface{}{"order": "asc"}},
			{"_doc": map[string]interface{}{"order": "asc"}},
		}
	case "price_desc":
		body["sort"] = []map[string]interface{}{
			{"price": map[string]interface{}{"order": "desc"}},
			{"_doc": map[string]interface{}{"order": "asc"}},
		}
	default:
		body["sort"] = []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
			{"_doc": map[string]interface{}{"order": "asc"}},
		}
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES broad query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch broad search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch broad search response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES response: %w", err)
	}

	return es.esHitsToProducts(&sr), nil
}

var categorySpecFields = map[string][]string{
	"Santé & Beauté": {
		"marque", "type_de_peau", "type_de_cheveux", "besoin", "besoin_cheveux",
		"texture_de_soin", "zone", "finition", "indice_spf", "contenance",
		"ingredients_actifs", "parfum", "gamme", "type_de_produit", "probleme_specifique",
	},
	"Informatique": {
		"marque", "processeur", "ram", "stockage", "ecran", "carte_graphique",
		"os", "resolution", "connectivite", "format", "couleur",
	},
	"Téléphonie": {
		"marque", "ram", "stockage", "ecran", "os", "couleur", "batterie",
		"resolution_appareil_photo", "connectivite", "double_sim",
	},
	"Électroménager": {
		"marque", "capacite", "classe_energetique", "couleur", "dimensions",
		"type_de_produit", "puissance", "nombre_de_programmes", "niveau_sonore",
	},
	"Jardinage & Bricolage": {
		"marque", "puissance", "tension", "batterie", "pression", "capacite",
		"largeur_coupe", "type_de_produit",
	},
}

type esAggregationBucket struct {
	Key      string `json:"key"`
	DocCount int    `json:"doc_count"`
}

type esAggregationResult struct {
	Buckets []esAggregationBucket `json:"buckets"`
}

func buildSpecAggregations(category string) map[string]interface{} {
	fields, ok := categorySpecFields[category]
	if !ok {
		return nil
	}
	aggs := make(map[string]interface{})
	for _, field := range fields {
		aggs["spec_"+field] = map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "specs." + field + ".keyword",
				"size":  100,
			},
		}
	}
	return aggs
}

func parseSpecFacets(aggs map[string]json.RawMessage) map[string][]string {
	facets := make(map[string][]string)
	for aggName, raw := range aggs {
		if !strings.HasPrefix(aggName, "spec_") {
			continue
		}
		var result esAggregationResult
		if err := json.Unmarshal(raw, &result); err != nil {
			continue
		}
		key := strings.TrimPrefix(aggName, "spec_")
		var values []string
		for _, bucket := range result.Buckets {
			if bucket.Key != "" {
				values = append(values, bucket.Key)
			}
		}
		if len(values) > 0 {
			facets[key] = values
		}
	}
	return facets
}

func (es *ElasticsearchStorage) Search(query string, category, subcategory, sort string, page, limit int, specsFilter map[string]string, minPrice, maxPrice float64) (*models.PaginatedResponse, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}

	q := strings.TrimSpace(query)
	var queryBody map[string]interface{}
	var postFilter []map[string]interface{}

	if q == "" && category == "" && subcategory == "" && len(specsFilter) == 0 {
		queryBody = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	} else if q != "" {
		isSearchingAccessory := false
		lowerQ := strings.ToLower(q)
		for _, ak := range accessoryKeywords {
			if strings.Contains(lowerQ, ak) {
				isSearchingAccessory = true
				break
			}
		}

		boolClauses := map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"multi_match": map[string]interface{}{
						"query":     q,
						"fields":    []string{"title^4", "title.raw^2", "description"},
						"fuzziness": "AUTO",
					},
				},
			},
			"should": []map[string]interface{}{
				{
					"match_phrase": map[string]interface{}{
						"title.raw": map[string]interface{}{
							"query": q,
							"boost": 3.0,
						},
					},
				},
			},
		}

		filter := []map[string]interface{}{}
		if category != "" {
			filter = append(filter, map[string]interface{}{
				"term": map[string]interface{}{
					"category": category,
				},
			})
		}
		if subcategory != "" {
			filter = append(filter, map[string]interface{}{
				"term": map[string]interface{}{
					"subcategory": subcategory,
				},
			})
		}
		if len(filter) > 0 {
			boolClauses["filter"] = filter
		}

		postFilter = []map[string]interface{}{}
		for sk, sv := range specsFilter {
			values := strings.Split(sv, ",")
			var shouldClauses []map[string]interface{}
			for _, val := range values {
				shouldClauses = append(shouldClauses, map[string]interface{}{
					"term": map[string]interface{}{
						"specs." + sk + ".keyword": val,
					},
				})
			}
			postFilter = append(postFilter, map[string]interface{}{
				"bool": map[string]interface{}{
					"should":               shouldClauses,
					"minimum_should_match": 1,
				},
			})
		}
		priceRange := buildPriceRange(minPrice, maxPrice)
		if priceRange != nil {
			postFilter = append(postFilter, priceRange)
		}

		baseQuery := map[string]interface{}{"bool": boolClauses}
		if isSearchingAccessory {
			queryBody = baseQuery
		} else {
			queryBody = map[string]interface{}{
				"function_score": map[string]interface{}{
					"query": baseQuery,
					"functions": []map[string]interface{}{
						{
							"filter": map[string]interface{}{
								"match": map[string]interface{}{
									"title": strings.Join(accessoryKeywords, " "),
								},
							},
							"weight": 0.1,
						},
					},
					"boost_mode": "multiply",
				},
			}
		}
	} else {
		filter := []map[string]interface{}{}
		if category != "" {
			filter = append(filter, map[string]interface{}{
				"term": map[string]interface{}{
					"category": category,
				},
			})
		}
		if subcategory != "" {
			filter = append(filter, map[string]interface{}{
				"term": map[string]interface{}{
					"subcategory": subcategory,
				},
			})
		}
		boolQuery := map[string]interface{}{}
		if len(filter) > 0 {
			boolQuery["filter"] = filter
		}
		queryBody = map[string]interface{}{"bool": boolQuery}

		postFilter = []map[string]interface{}{}
		for sk, sv := range specsFilter {
			values := strings.Split(sv, ",")
			var shouldClauses []map[string]interface{}
			for _, val := range values {
				shouldClauses = append(shouldClauses, map[string]interface{}{
					"term": map[string]interface{}{
						"specs." + sk + ".keyword": val,
					},
				})
			}
			postFilter = append(postFilter, map[string]interface{}{
				"bool": map[string]interface{}{
					"should":               shouldClauses,
					"minimum_should_match": 1,
				},
			})
		}
		priceRange := buildPriceRange(minPrice, maxPrice)
		if priceRange != nil {
			postFilter = append(postFilter, priceRange)
		}
	}

	body := map[string]interface{}{
		"query": queryBody,
		"from":  (page - 1) * limit,
		"size":  limit,
	}

	if category != "" {
		if aggs := buildSpecAggregations(category); aggs != nil {
			body["aggs"] = aggs
		}
	}

	if sort == "price_asc" {
		body["sort"] = []map[string]interface{}{
			{"price": map[string]interface{}{"order": "asc"}},
		}
	} else if sort == "price_desc" {
		body["sort"] = []map[string]interface{}{
			{"price": map[string]interface{}{"order": "desc"}},
		}
	}

	if len(postFilter) > 0 {
		body["post_filter"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": postFilter,
			},
		}
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES query: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch search response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES response: %w", err)
	}

	total := 0
	if v, ok := sr.Hits.Total["value"]; ok {
		if f, ok := v.(float64); ok {
			total = int(f)
		}
	}

	products := es.esHitsToProducts(&sr)

	facets := parseSpecFacets(sr.Aggregations)

	totalPages := total / limit
	if total%limit > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	return &models.PaginatedResponse{
		Data:       products,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		Facets:     facets,
	}, nil
}

func (es *ElasticsearchStorage) InitializeScrapeHistoryIndex() error {
	if !es.clientAvailable() {
		return fmt.Errorf("elasticsearch client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := es.client.Indices.Exists(
		[]string{"scrape_history"},
		es.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("checking scrape_history index: %w", err)
	}
	defer exists.Body.Close()

	if exists.StatusCode == 200 {
		return nil
	}

	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type": "keyword",
				},
				"last_scraped_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}

	body, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshalling scrape_history mapping: %w", err)
	}

	res, err := es.client.Indices.Create(
		"scrape_history",
		es.client.Indices.Create.WithBody(strings.NewReader(string(body))),
		es.client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating scrape_history index: %s", res.String())
	}

	log.Println("[es] scrape_history index created")
	return nil
}

func (es *ElasticsearchStorage) HasScrapedRecently(ctx context.Context, query string, maxAge time.Duration) bool {
	if !es.clientAvailable() || query == "" {
		return false
	}

	normalizedQuery := strings.ToLower(strings.Join(strings.Fields(query), " "))
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"_id": normalizedQuery,
						},
					},
					{
						"range": map[string]interface{}{
							"last_scraped_at": map[string]interface{}{
								"gte": cutoff,
							},
						},
					},
				},
			},
		},
		"size": 0,
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		log.Printf("[es] HasScrapedRecently marshal error: %v", err)
		return false
	}

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("scrape_history"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		log.Printf("[es] HasScrapedRecently search error: %v", err)
		return false
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[es] HasScrapedRecently response error: %s", res.String())
		return false
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		log.Printf("[es] HasScrapedRecently decode error: %v", err)
		return false
	}

	total := 0
	if v, ok := sr.Hits.Total["value"]; ok {
		if f, ok := v.(float64); ok {
			total = int(f)
		}
	}

	return total > 0
}

func BulkIndexProducts(ctx context.Context, es *elasticsearch.Client, products []models.Product) error {
	if es == nil {
		return fmt.Errorf("elasticsearch client not initialized")
	}
	if len(products) == 0 {
		return nil
	}

	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        es,
		Index:         "products",
		FlushInterval: 1 * time.Second,
		NumWorkers:    2,
	})
	if err != nil {
		return fmt.Errorf("creating bulk indexer: %w", err)
	}

	for _, p := range products {
		doc := map[string]interface{}{
			"id":              p.ID,
			"title":           p.Name,
			"price":           p.Price,
			"price_text":      p.PriceText,
			"category":        p.Category,
			"subcategory":     p.Subcategory,
			"description":     p.Description,
			"image":           p.Image,
			"url":             p.ProductURL,
			"source":          p.Source,
			"reference":       p.Reference,
			"base_reference":  p.BaseReference,
			"ean":             p.EAN,
			"in_stock":        p.InStock,
			"specs":           p.Specs,
			"scraped_at":      time.Now().UTC().Format(time.RFC3339),
		}

		data, err := json.Marshal(doc)
		if err != nil {
			log.Printf("[es] BulkIndexProducts marshal error for %s: %v", p.ID, err)
			continue
		}

		err = bi.Add(ctx, esutil.BulkIndexerItem{
			Action:     "index",
			Body:       strings.NewReader(string(data)),
			DocumentID: p.ID,
			OnFailure: func(_ context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
				if err != nil {
					log.Printf("[es] BulkIndexProducts bulk index error for %s: %v", item.DocumentID, err)
				} else {
					log.Printf("[es] BulkIndexProducts bulk index failure for %s: %s", item.DocumentID, res.Error.Reason)
				}
			},
		})
		if err != nil {
			log.Printf("[es] BulkIndexProducts add error for %s: %v", p.ID, err)
		}
	}

	if err := bi.Close(ctx); err != nil {
		return fmt.Errorf("closing bulk indexer: %w", err)
	}

	return nil
}

func (es *ElasticsearchStorage) MarkAsScraped(ctx context.Context, query string) {
	if !es.clientAvailable() || query == "" {
		return
	}

	normalizedQuery := strings.ToLower(strings.Join(strings.Fields(query), " "))

	doc := map[string]interface{}{
		"query":           normalizedQuery,
		"last_scraped_at": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(doc)
	if err != nil {
		log.Printf("[es] MarkAsScraped marshal error: %v", err)
		return
	}

	docID := url.PathEscape(normalizedQuery)
	res, err := es.client.Index(
		"scrape_history",
		strings.NewReader(string(data)),
		es.client.Index.WithDocumentID(docID),
		es.client.Index.WithContext(ctx),
		es.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		log.Printf("[es] MarkAsScraped index error: %v", err)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("[es] MarkAsScraped response error: %s", res.String())
	}
}

func (es *ElasticsearchStorage) Suggest(query string) ([]string, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}
	if query == "" {
		return nil, nil
	}

	body := map[string]interface{}{
		"_source": []string{"title"},
		"size":    5,
		"query": map[string]interface{}{
			"match": map[string]interface{}{
				"title.suggest": query,
			},
		},
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling suggest query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch suggest search error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch suggest response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES suggest response: %w", err)
	}

	products := es.esHitsToProducts(&sr)
	suggestions := make([]string, 0, len(products))
	seen := make(map[string]struct{})
	for _, p := range products {
		if _, ok := seen[p.Name]; ok {
			continue
		}
		seen[p.Name] = struct{}{}
		suggestions = append(suggestions, p.Name)
		if len(suggestions) >= 5 {
			break
		}
	}
	return suggestions, nil
}

func (es *ElasticsearchStorage) RefreshIndex(ctx context.Context) error {
	if !es.clientAvailable() {
		return fmt.Errorf("elasticsearch client not initialized")
	}
	res, err := es.client.Indices.Refresh(
		es.client.Indices.Refresh.WithIndex("products"),
		es.client.Indices.Refresh.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("refresh error: %s", res.String())
	}
	return nil
}

func (es *ElasticsearchStorage) AppendProducts(ctx context.Context, products []models.Product) error {
	if !es.clientAvailable() || len(products) == 0 {
		return nil
	}

	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:        es.client,
		Index:         "products",
		FlushInterval: 1 * time.Second,
		NumWorkers:    2,
	})
	if err != nil {
		return fmt.Errorf("creating bulk indexer: %w", err)
	}

	for _, p := range products {
		doc := map[string]interface{}{
			"id":              p.ID,
			"title":           p.Name,
			"price":           p.Price,
			"price_text":      p.PriceText,
			"category":        p.Category,
			"subcategory":     p.Subcategory,
			"description":     p.Description,
			"image":           p.Image,
			"url":             p.ProductURL,
			"source":          p.Source,
			"reference":       p.Reference,
			"base_reference":  p.BaseReference,
			"ean":             p.EAN,
			"in_stock":        p.InStock,
			"specs":           p.Specs,
			"scraped_at":      time.Now().UTC().Format(time.RFC3339),
		}

		data, err := json.Marshal(doc)
		if err != nil {
			log.Printf("[es] marshal error for product %s: %v", p.ID, err)
			continue
		}

		err = bi.Add(ctx, esutil.BulkIndexerItem{
			Index:      "products",
			Action:     "index",
			Body:       strings.NewReader(string(data)),
			DocumentID: p.ID,
			OnFailure: func(_ context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
				if err != nil {
					log.Printf("[es] bulk index error for %s: %v", item.DocumentID, err)
				} else {
					log.Printf("[es] bulk index failure for %s: %s", item.DocumentID, res.Error.Reason)
				}
			},
		})
		if err != nil {
			log.Printf("[es] bulk index add error for %s: %v", p.ID, err)
		}
	}

	if err := bi.Close(ctx); err != nil {
		return fmt.Errorf("closing bulk indexer: %w", err)
	}

	return nil
}

func (es *ElasticsearchStorage) GetFilterCounts(category, subcategory string) (map[string]map[string]int, error) {
	if !es.clientAvailable() {
		return nil, fmt.Errorf("elasticsearch client not initialized")
	}

	filters := []map[string]interface{}{
		{"term": map[string]interface{}{"subcategory": subcategory}},
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": filters,
			},
		},
		"size": 0,
	}

	if aggs := buildSpecAggregations(category); aggs != nil {
		body["aggs"] = aggs
	} else {
		return nil, fmt.Errorf("no spec fields defined for category %s", category)
	}

	queryJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling ES filter counts query: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex("products"),
		es.client.Search.WithBody(strings.NewReader(string(queryJSON))),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch filter counts error: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch filter counts response error: %s", res.String())
	}

	var sr esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding ES filter counts response: %w", err)
	}

	result := make(map[string]map[string]int)
	for aggName, raw := range sr.Aggregations {
		if !strings.HasPrefix(aggName, "spec_") {
			continue
		}
		var aggResult esAggregationResult
		if err := json.Unmarshal(raw, &aggResult); err != nil {
			continue
		}
		key := strings.TrimPrefix(aggName, "spec_")
		counts := make(map[string]int, len(aggResult.Buckets))
		for _, bucket := range aggResult.Buckets {
			if bucket.Key != "" {
				counts[bucket.Key] = bucket.DocCount
			}
		}
		if len(counts) > 0 {
			result[key] = counts
		}
	}

	return result, nil
}

func buildPriceRange(minPrice, maxPrice float64) map[string]interface{} {
	priceRange := map[string]interface{}{}
	if minPrice > 0 {
		priceRange["gte"] = minPrice
	}
	if maxPrice > 0 {
		priceRange["lte"] = maxPrice
	}
	if len(priceRange) == 0 {
		return nil
	}
	return map[string]interface{}{
		"range": map[string]interface{}{
			"price": priceRange,
		},
	}
}
