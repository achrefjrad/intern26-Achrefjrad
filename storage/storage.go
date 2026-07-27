package storage

import (
	"context"

	"project/models"
)

type Storage interface {
	SaveProduct(ctx context.Context, product *models.Product) error
	SaveProducts(ctx context.Context, products []models.Product) error
	AppendProducts(ctx context.Context, products []models.Product) error
}

var defaultStorage Storage
var defaultES *ElasticsearchStorage

func Init(s Storage) {
	defaultStorage = s
}

func InitES(es *ElasticsearchStorage) {
	defaultES = es
}

func GetStorage() Storage {
	return defaultStorage
}

func GetES() *ElasticsearchStorage {
	return defaultES
}