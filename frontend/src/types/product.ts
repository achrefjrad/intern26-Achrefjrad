export interface Product {
  id: string;
  title?: string;
  name: string;
  price: number;
  category?: string;
  subcategory?: string;
  description?: string;
  image?: string;
  url?: string;
  source?: string;
  reference?: string;
  score?: number;
  in_stock?: boolean;
  specs?: Record<string, string>;
}

export interface PaginatedResponse {
  data: Product[];
  page: number;
  limit: number;
  total: number;
  totalPages: number;
  facets?: Record<string, string[]>;
}

export interface ProductDetailResponse {
  product: Product;
  offers: Product[];
}

export interface PriceHistoryPoint {
  date: string;
  price: number;
}

export interface PriceHistoryResponse {
  current_price: number;
  change_percentage: number;
  history: PriceHistoryPoint[];
}
