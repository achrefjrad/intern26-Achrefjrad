package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

func (es *ElasticsearchStorage) InitializeProductIndex(ctx context.Context) error {
	if !es.clientAvailable() {
		return fmt.Errorf("elasticsearch client not initialized")
	}

	exists, err := es.client.Indices.Exists(
		[]string{"products"},
		es.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("checking products index: %w", err)
	}
	defer exists.Body.Close()

	if exists.StatusCode == 200 {
		log.Println("[es] products index already exists")
		return nil
	}

	body := map[string]interface{}{
		"settings": map[string]interface{}{
			"analysis": map[string]interface{}{
				"tokenizer": map[string]interface{}{
					"autocomplete_tokenizer": map[string]interface{}{
						"type":     "edge_ngram",
						"min_gram": 2,
						"max_gram": 15,
					},
				},
				"filter": map[string]interface{}{
					"french_stop": map[string]interface{}{
						"type":      "stop",
						"stopwords": "_french_",
					},
					"french_stemmer": map[string]interface{}{
						"type":     "stemmer",
						"language": "light_french",
					},
					"ecommerce_synonyms": map[string]interface{}{
						"type": "synonym",
						"synonyms": []string{
							"ordinateur, pc, laptop",
							"smartphone, telephone",
							"ecouteur, casque",
						},
					},
				},
				"analyzer": map[string]interface{}{
					"french_nlp_index_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "standard",
						"filter": []string{
							"lowercase",
							"french_stop",
							"french_stemmer",
						},
					},
					"french_nlp_search_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "standard",
						"filter": []string{
							"lowercase",
							"french_stop",
							"ecommerce_synonyms",
							"french_stemmer",
						},
					},
					"autocomplete_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "autocomplete_tokenizer",
						"filter": []string{
							"lowercase",
						},
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type": "keyword",
				},
				"title": map[string]interface{}{
					"type":            "text",
					"analyzer":        "french_nlp_index_analyzer",
					"search_analyzer": "french_nlp_search_analyzer",
					"fields": map[string]interface{}{
						"raw": map[string]interface{}{
							"type":     "text",
							"analyzer": "standard",
						},
						"suggest": map[string]interface{}{
							"type":            "text",
							"analyzer":        "autocomplete_analyzer",
							"search_analyzer": "standard",
						},
					},
				},
				"price": map[string]interface{}{
					"type": "float",
				},
				"price_text": map[string]interface{}{
					"type":  "keyword",
					"index": false,
				},
				"category": map[string]interface{}{
					"type": "keyword",
				},
				"subcategory": map[string]interface{}{
					"type": "keyword",
				},
				"description": map[string]interface{}{
					"type":            "text",
					"analyzer":        "french_nlp_index_analyzer",
					"search_analyzer": "french_nlp_search_analyzer",
				},
				"image": map[string]interface{}{
					"type":  "keyword",
					"index": false,
				},
				"url": map[string]interface{}{
					"type":  "keyword",
					"index": false,
				},
				"reference": map[string]interface{}{
					"type": "keyword",
				},
			"base_reference": map[string]interface{}{
				"type": "keyword",
			},
			"ean": map[string]interface{}{
				"type": "keyword",
			},
			"source": map[string]interface{}{
					"type": "keyword",
				},
				"specs": map[string]interface{}{
					"type":     "object",
					"dynamic":  true,
				},
				"scraped_at": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}

	mappingJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling NLP index mapping: %w", err)
	}

	res, err := es.client.Indices.Create(
		"products",
		es.client.Indices.Create.WithBody(strings.NewReader(string(mappingJSON))),
		es.client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating products index with NLP settings: %s", res.String())
	}

	log.Println("[es] products index created with NLP settings")
	return nil
}