import type { PaginatedResponse, Product, ProductDetailResponse } from "@/types/product";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function fetchProducts(
  page?: number,
  category?: string,
  subcategory?: string,
  sort?: string,
  specs?: Record<string, string>,
  minPrice?: string,
  maxPrice?: string,
): Promise<PaginatedResponse> {
  const url = new URL(`${API_BASE}/search`);
  if (page && page > 1) url.searchParams.set("page", String(page));
  if (category) url.searchParams.set("category", category);
  if (subcategory) url.searchParams.set("subcategory", subcategory);
  if (sort) url.searchParams.set("sort", sort);
  if (minPrice) url.searchParams.set("min_price", minPrice);
  if (maxPrice) url.searchParams.set("max_price", maxPrice);
  for (const [key, val] of Object.entries(specs ?? {})) {
    if (val) url.searchParams.set(`spec_${key}`, val);
  }
  url.searchParams.set("limit", "24");
  const res = await fetch(url.toString());
  if (!res.ok) throw new Error(`Server error: ${res.status} ${res.statusText}`);
  return res.json();
}

export async function fetchSuggestions(query: string): Promise<string[]> {
  if (!query.trim()) return [];
  const url = `${API_BASE}/suggest?q=${encodeURIComponent(query)}`;
  const res = await fetch(url);
  if (!res.ok) return [];
  return res.json();
}

export async function fetchProductById(id: string): Promise<ProductDetailResponse> {
  const res = await fetch(`${API_BASE}/products/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error(`Server error: ${res.status} ${res.statusText}`);
  return res.json();
}

export interface FilterValue {
  value: string;
  count: number;
}

export interface FilterGroup {
  name: string;
  key: string;
  values: FilterValue[];
}

export interface FilterSchemaResponse {
  subcategory: string;
  category?: string;
  filters: FilterGroup[];
  updatedAt: string;
}

export async function fetchFilterSchema(subcategory: string): Promise<FilterSchemaResponse> {
  const res = await fetch(`${API_BASE}/api/filters/schema/${encodeURIComponent(subcategory)}`, {
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to fetch filter schema: ${res.status}`);
  return res.json();
}


