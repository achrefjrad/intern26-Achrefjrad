# intern26-Achrefjrad-back

A Tunisian e-commerce price aggregation platform that scrapes product listings from 8 local merchants (Tunisianet, MyTek, Wiki.tn, SBS Informatique, BeautyStore, Fatales, BigDepot, EGM), normalizes them into a unified catalog, and serves them through a searchable, filterable web interface with price trend tracking.

---

## 📊 Tech Stack

![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=for-the-badge&logo=postgresql&logoColor=white)
![Elasticsearch](https://img.shields.io/badge/Elasticsearch-005571?style=for-the-badge&logo=elasticsearch&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-000000?style=for-the-badge&logo=nextdotjs&logoColor=white)
![React](https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB)
![Recharts](https://img.shields.io/badge/Recharts-22b5bf?style=for-the-badge&logo=recharts&logoColor=white)
![TailwindCSS](https://img.shields.io/badge/TailwindCSS-38B2AC?style=for-the-badge&logo=tailwind-css&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)

| Layer    | Technology                                                    |
| -------- | ------------------------------------------------------------- |
| Backend  | Go (Echo), Colly, goquery, pgx, go-elasticsearch/v8           |
| Storage  | PostgreSQL (canonical catalog + schemas + price history)       |
| Search   | Elasticsearch (facets, aggregations, fast full-text search)    |
| Frontend | Next.js (App Router), React 19, Recharts, Tailwind CSS v4      |
| Infra    | Docker Compose (Postgres, Elasticsearch, Kibana, backend, FE)  |

---

## ✨ Key Features

### 1. Smart Aggregation & Normalization

- **8 merchant scrapers** register into a common `ScraperFunc` registry and stream `models.Product` results over a channel (`scrapers/registry.go`, `scrapers/manager_stream.go`).
- Every scraped product is passed through `services.EnrichProduct()` (`services/specextractor.go`), which:
  - Assigns a **category & subcategory** via keyword/regex rules (`services/categorizer.go`).
  - Extracts structured **specs** from the title/description using compiled regexes (RAM, storage, screen size, processor, volume, SPF, wattage, voltage, battery, pressure, capacity…).
  - Infers a normalized **brand** from a keyword map or uppercase-name heuristics.
  - Renames messy spec keys (`memoire` → `ram`, `disque_dur` → `stockage`, `volume` → `contenance`, …) and strips category-irrelevant keys (e.g. Jardinage keeps only `marque`).
- The **Dynamic Filter Engine** (`services/schema_scraper.go`) scrapes each merchant's *own* sidebar filter facets across 4 e-commerce platforms (custom PrestaShop module, PrestaShop 1.7, Magento 2, WooCommerce), then:
  - Deduplicates group titles via `normalizeGroupTitle()` (e.g. "Marques" / "brand" → "Marque").
  - Normalizes values via `normalizeFilterValue()` ("8go" → "8 Go", "1 tb" → "1 To", warranties → "1 an/2 ans", "10000mah" → "10000 mAh", screen sizes → `"`).
  - Merges all merchants for a subcategory into **one unified schema** (`MergeSubcategorySchemas`), dropping duplicates and keeping first-seen casing.
- Schemas are persisted to the `filter_schemas` PostgreSQL table and re-scraped weekly (plus on startup).

### 2. Dynamic Filtering

- Search and browse are powered by **Elasticsearch** aggregations (`storage/elasticsearch.go`).
- The sidebar (`frontend/src/components/DynamicFilters.tsx`) renders groups from the unified schema and enriches each value with a **live product count** pulled from ES aggregations (`handlers/filters.go` → `GetFilterCounts`).
- **Zero-count fallback:** a value the merchant advertises but that has no current product match shows `(0)` instead of disappearing, keeping the merchant schema intact and honest.
- **Strict whitelisting:** `categorySpecFields` and `subcategorySpecFields` (`resolveSpecFields`) define exactly which spec keys may appear as ES aggregations per category/subcategory. Any bucket outside the whitelist is discarded, so:
  - IT/smartphone specs (RAM, OS, écran, processeur…) never leak into **Accessoires**.
  - Santé & Beauté only exposes its curated beauty fields.
  - Jardinage & Bricolage is strictly **Marque-only**.
- Spec filters are passed back as `spec_<key>` query params and applied by the ES query builder; `Marque` is always pinned to index 0 in the sidebar.

### 3. Price Trend Tracking

- Every product upsert records a price point in the `price_history` PostgreSQL table using a lightweight **"insert on change"** strategy (`storage/postgres.go`):
  - Only a new row if the product has **no** history yet or the scraped price **differs** from the last recorded one → no bloat from identical rescrapes.
- At startup, `BackfillPriceHistory()` seeds a baseline for products with no history, then ensures every single-point product gets a **yesterday-dated** point at its current price, so every product renders a 2-point trend line.
- `GET /products/:id/price-history` (`handlers/price_history.go`) returns:
  ```json
  {
    "current_price": 87.000,
    "change_percentage": -2.15,
    "history": [
      {"date": "2026-07-01", "price": 89.000},
      {"date": "2026-07-15", "price": 87.000}
    ]
  }
  ```
- The product detail page embeds `frontend/src/components/PriceTrendChart.tsx` — an interactive **Recharts AreaChart** (smooth `monotone` curve, blue gradient fill, DD/MM axis labels, custom TND tooltip) with a "TENDANCE PRIX" header, a % change badge (green down / red up / gray at 0%), and a flat-line-safe Y-axis domain.

### 4. Search Pipeline

- `GET /search` implements a staged search:
  1. **Reference VIP lane** — exact reference/EAN matches score 100.
  2. **Fresh scrape check** — if the query was scraped within the last 24h, serve straight from ES (`scrape_history` index).
  3. **On-demand scrape** — otherwise trigger all 8 scrapers, persist to PG + ES in batches, then re-query.
- `GET /search/stream` provides a **Server-Sent Events** live-updating search that flushes results incrementally as scrapers stream them in.

---

## 🚀 Setup & Installation

### Prerequisites

- Docker + Docker Compose (recommended path)
- Go 1.26+ and Node.js 20+ (for running locally without Docker)

### Environment Variables

| Variable            | Required | Default               | Description                                   |
| ------------------- | -------- | --------------------- | --------------------------------------------- |
| `DB_HOST`           | ✅        | —                     | PostgreSQL host                               |
| `DB_PORT`           | ✅        | —                     | PostgreSQL port                               |
| `DB_USER`           | ✅        | —                     | PostgreSQL user                               |
| `DB_PASSWORD`       | ✅        | —                     | PostgreSQL password                           |
| `DB_NAME`           | ✅        | —                     | PostgreSQL database name                      |
| `DB_SSLMODE`        | ❌        | `disable`             | PostgreSQL SSL mode                           |
| `ES_URL`            | ✅        | `http://localhost:9200` | Elasticsearch URL                            |
| `ES_API_KEY`        | ❌        | —                     | Optional ES API key                           |
| `FRONTEND_URL`      | ❌        | —                     | Extra allowed CORS origin                     |
| `PORT`              | ❌        | `8080`                | Backend listen port                           |
| `NEXT_PUBLIC_API_URL` | ❌      | `http://localhost:8080` | Frontend's backend base URL (build arg) |

### Running with Docker Compose

```bash
# 1. Start the full stack (Postgres, Elasticsearch, Kibana, backend, frontend)
docker compose up -d

# 2. Rebuild backend/frontend after code changes
docker compose build backend frontend
docker compose up -d backend frontend
```

Services are exposed on:

| Service        | Port  |
| -------------- | ----- |
| Frontend       | 3000  |
| Backend API    | 8080  |
| PostgreSQL     | 5433  |
| Elasticsearch  | 9200  |
| Kibana         | 5601  |

### Running Locally (no Docker)

```bash
# Backend
go mod download
DB_HOST=localhost DB_PORT=5433 DB_USER=postgres DB_PASSWORD=postgres DB_NAME=project \
DB_SSLMODE=disable ES_URL=http://localhost:9200 \
go run .
```

```bash
# Frontend
cd frontend
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
```

---

## 🛠️ Important Scripts & Commands

### Test a search scrape (CLI)

Runs all 8 registered scrapers against a query and prints the streamed results with prices, categories, and specs:

```bash
go run cmd/scrape_test/main.go "tondeuse"
```

### Run data migration + Elasticsearch re-indexing

Re-runs `EnrichProduct()` over the entire catalog (re-categorizing, re-extracting specs, re-normalizing keys), writes the normalized specs back to PostgreSQL, then bulk-re-indexes everything into Elasticsearch:

```bash
# Requires DB_* and ES_URL env vars (see above)
go run cmd/migrate_specs/main.go
```

> Note: this migration **only skips** products that never had specs and still have none extracted — so whitelist stripping (e.g. Jardinage → Marque-only) also clears stale data.

### Filter schema scraping

The backend automatically scrapes and merges merchant filter schemas **on startup** and then **weekly** (see `startFilterSchemaSync()` in `main.go`). No manual command is needed.

### Price history seeding & backfill

- Runs automatically on **backend startup** (`storage.BackfillPriceHistory()`).
- Inserts a baseline point for every product without history.
- Ensures every single-point product gets a second point dated **yesterday** at its current price, so all price-trend charts render.
- Idempotent — safe to run repeatedly.
- Subsequent price points are recorded automatically on every product upsert via the "insert on change" logic.

---

## 📁 Project Layout

```
├── cmd/
│   ├── migrate_specs/        # Specs migration + ES re-index tool
│   └── scrape_test/          # CLI to test the scraper pipeline
├── database/                 # pgx pool init, schema/migrations
├── handlers/                 # Echo HTTP handlers (search, product, filters, price history)
├── models/                   # Go domain models
├── routes/                   # Echo route registration
├── scrapers/                 # 8 merchant scrapers + registry + stream manager
├── services/                 # Enrichment, categorization, spec extraction, filter schema scraper
├── storage/                  # PostgresStorage + ElasticsearchStorage
├── types/                    # Category/subcategory constants
├── workers/                  # Scheduled product re-sync
└── frontend/                 # Next.js app (App Router) + Recharts + Tailwind
```