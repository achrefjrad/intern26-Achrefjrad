package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

var pool *pgxpool.Pool
var esClient *elasticsearch.Client

func loadConfig() (*Config, error) {
	cfg := &Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("DB_HOST is required")
	}
	if cfg.Port == "" {
		return nil, fmt.Errorf("DB_PORT is required")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}
	if cfg.DBName == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}

	return cfg, nil
}

// 🛠️ REWRITTEN FUNCTION: Safely encodes credentials to handle special characters
func (c *Config) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	// Escape user and password to prevent symbols like ?, #, @ from splitting the URL incorrectly
	escapedUser := url.QueryEscape(c.User)
	escapedPassword := url.QueryEscape(c.Password)

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		escapedUser, escapedPassword, c.Host, c.Port, c.DBName, sslMode)
}

func Init() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}

	if err := p.Ping(ctx); err != nil {
		p.Close()
		return fmt.Errorf("pinging database at %s:%s: %w", cfg.Host, cfg.Port, err)
	}

	pool = p

	// Initialise Elasticsearch (non-fatal)
	if err := initElasticsearch(); err != nil {
		log.Printf("Elasticsearch init skipped: %v", err)
	}

	return nil
}

func GetPool() *pgxpool.Pool {
	return pool
}

func GetESClient() *elasticsearch.Client {
	return esClient
}

func Close() {
	if pool != nil {
		pool.Close()
	}
}

func getESURL() string {
	url := os.Getenv("ES_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	return url
}

func initElasticsearch() error {
	url := getESURL()
	apiKey := os.Getenv("ES_API_KEY")

	cfg := elasticsearch.Config{
		Addresses: []string{url},
		APIKey:    apiKey,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("creating elasticsearch client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.Ping(client.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("pinging elasticsearch at %s: %w", url, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch ping error at %s: %s", url, res.String())
	}

	esClient = client
	log.Printf("[es] connected to %s", url)

	return nil
}
