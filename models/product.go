package models

type Product struct {
	ID            string            `json:"id"`
	Name          string            `json:"title"`
	Price         float64           `json:"price"`
	PriceText     string            `json:"price_text,omitempty"`
	Category      string            `json:"category"`
	Subcategory   string            `json:"subcategory"`
	Description   string            `json:"description"`
	Image         string            `json:"image"`
	ProductURL    string            `json:"url,omitempty"`
	Source        string            `json:"source,omitempty"`
	Reference     string            `json:"reference"`
	BaseReference string            `json:"base_reference,omitempty"`
	EAN           string            `json:"ean,omitempty"`
	InStock       bool              `json:"in_stock"`
	Specs         map[string]string `json:"specs,omitempty"`
	SearchRank    int               `json:"-"`
	Score         float64           `json:"score,omitempty"`
}

type PaginatedResponse struct {
	Data       []Product            `json:"data"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	Total      int                  `json:"total"`
	TotalPages int                  `json:"totalPages"`
	Facets     map[string][]string  `json:"facets,omitempty"`
}

type ProductDetailResponse struct {
	Product Product   `json:"product"`
	Offers  []Product `json:"offers"`
}
