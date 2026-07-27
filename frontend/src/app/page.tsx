"use client";

import { useState, useEffect, useRef, Suspense, startTransition } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { Product } from "@/types/product";
import { CATEGORIES, Category, Subcategory } from "@/types/constants";
import { fetchProducts, fetchSuggestions } from "@/services/api";
import DynamicFilters from "@/components/DynamicFilters";

function SkeletonCard() {
  return (
    <div className="rounded-xl border border-gray-200 bg-white shadow-sm flex flex-col overflow-hidden animate-pulse">
      <div className="aspect-[4/3] bg-gray-200" />
      <div className="p-4 space-y-3">
        <div className="h-3 w-16 bg-gray-200 rounded" />
        <div className="h-4 w-full bg-gray-200 rounded" />
        <div className="h-3 w-3/4 bg-gray-200 rounded" />
        <div className="flex items-center justify-between pt-2">
          <div className="h-5 w-20 bg-gray-200 rounded" />
          <div className="h-8 w-24 bg-gray-200 rounded-lg" />
        </div>
      </div>
    </div>
  );
}

function generatePageNumbers(current: number, total: number): number[] {
  if (total <= 5) return Array.from({ length: total }, (_, i) => i + 1);

  const pages: number[] = [];
  if (current <= 3) {
    pages.push(1, 2, 3, 4, 5, -1, total);
  } else if (current >= total - 2) {
    pages.push(1, -1, total - 4, total - 3, total - 2, total - 1, total);
  } else {
    pages.push(1, -1, current - 1, current, current + 1, -1, total);
  }
  return pages;
}

function getSourceBadge(source?: string): { label: string; className: string; title: string } {
  const s = (source ?? "").toLowerCase();
  if (s.includes("tunisianet")) return { label: "TN", className: "bg-red-600", title: "Source: Tunisianet" };
  if (s.includes("mytech")) return { label: "MY", className: "bg-blue-600", title: "Source: MyTech" };
  if (s.includes("wiki")) return { label: "WT", className: "bg-emerald-600", title: "Source: Wiki TN" };
  if (s.includes("sbs")) return { label: "SB", className: "bg-purple-600", title: "Source: SBS Informatique" };
  if (s.includes("beautystore")) return { label: "BS", className: "bg-pink-600", title: "Source: BeautyStore" };
  if (s.includes("fatales")) return { label: "FA", className: "bg-orange-600", title: "Source: Fatales" };
  if (s.includes("bricodirect") || s.includes("brico-direct")) return { label: "BD", className: "bg-teal-600", title: "Source: BricoDirect" };
  if (s.includes("bigdepot")) return { label: "BP", className: "bg-amber-600", title: "Source: BigDepot" };
  return { label: "", className: "", title: "" };
}

function ProductCard({ product, returnTo }: { product: Product; returnTo: string }) {
  const badge = getSourceBadge(product.source);

  return (
    <Link
      href={`/product/${product.id}?returnTo=${encodeURIComponent(returnTo)}`}
      className="block rounded-xl border border-gray-200 bg-white shadow-sm transition-all duration-300 hover:scale-105 hover:shadow-lg flex flex-col overflow-hidden cursor-pointer"
    >
      <div className="relative aspect-[4/3] bg-gradient-to-br from-gray-50 to-gray-100 flex items-center justify-center overflow-hidden">
        {badge.label && (
          <span
            title={badge.title}
            className={`absolute top-2 left-2 z-10 px-1.5 py-0.5 text-[10px] font-bold text-white rounded ${badge.className}`}
          >
            {badge.label}
          </span>
        )}
        {product.image ? (
          <img src={product.image} alt={product.title || product.name} className="w-full h-full object-contain p-2" />
        ) : (
          <div className="text-center">
            <svg className="w-12 h-12 mx-auto text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
            <span className="block mt-2 text-xs text-gray-400 font-medium">Image</span>
          </div>
        )}
      </div>
      <div className="p-4 flex flex-col gap-1.5 flex-1">
        {product.subcategory && (
          <span className="text-xs font-medium text-blue-600 uppercase tracking-wider">{product.subcategory}</span>
        )}
        <h3 className="text-sm font-semibold text-gray-900 leading-snug line-clamp-2 mt-1">{product.title || product.name}</h3>
        
        <div className="mt-auto flex items-center justify-between pt-2">
          <span className="text-lg font-bold text-gray-900">{product.price.toLocaleString()} TND</span>
          
          {product.in_stock !== false ? (
            <span className="inline-flex items-center px-2 py-0.5 text-[10px] font-semibold text-green-700 bg-green-100 rounded-full">
              En Stock
            </span>
          ) : (
            <span className="inline-flex items-center px-2 py-0.5 text-[10px] font-semibold text-red-700 bg-red-100 rounded-full">
              Rupture de stock
            </span>
          )}
        </div>
      </div>
    </Link>
  );
}

const ITEMS_PER_PAGE = 24;

function HomeContent() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const [searchQuery, setSearchQuery] = useState(searchParams.get("q") ?? "");
  const [inputValue, setInputValue] = useState(searchParams.get("q") ?? "");
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isCategoryMenuOpen, setIsCategoryMenuOpen] = useState(false);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(searchParams.get("category") ?? null);
  const [selectedSubcategory, setSelectedSubcategory] = useState<string | null>(searchParams.get("subcategory") ?? null);
  const [sortOption, setSortOption] = useState<"default" | "price-asc" | "price-desc">(
    (searchParams.get("sort") as "default" | "price-asc" | "price-desc") ?? "default"
  );
  const [hoveredMenuCategory, setHoveredMenuCategory] = useState<string>(Object.keys(CATEGORIES)[0] ?? "");
  const [currentPage, setCurrentPage] = useState(parseInt(searchParams.get("page") ?? "1", 10));
  const [totalPages, setTotalPages] = useState(0);
  const [total, setTotal] = useState(0);
  const [facets, setFacets] = useState<Record<string, string[]> | undefined>(undefined);
  const [pageLoading, setPageLoading] = useState(false);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState(-1);

  // Ref to track latest products for SSE diffing (avoids stale closures in onmessage)
  const productsRef = useRef<Product[]>(products);
  productsRef.current = products;

  // Refs to track latest values for URL builder
  const searchQueryRef = useRef(searchQuery);
  searchQueryRef.current = searchQuery;
  const selectedCategoryRef = useRef(selectedCategory);
  selectedCategoryRef.current = selectedCategory;
  const selectedSubcategoryRef = useRef(selectedSubcategory);
  selectedSubcategoryRef.current = selectedSubcategory;
  const sortOptionRef = useRef(sortOption);
  sortOptionRef.current = sortOption;
  const currentPageRef = useRef(currentPage);
  currentPageRef.current = currentPage;

  // Build and push URL from live refs, preserving existing spec_*/price params
  function pushURL(extra?: (p: URLSearchParams) => void) {
    const params = new URLSearchParams(searchParams.toString());
    if (searchQueryRef.current) params.set("q", searchQueryRef.current);
    else params.delete("q");
    if (selectedCategoryRef.current) params.set("category", selectedCategoryRef.current);
    else params.delete("category");
    if (selectedSubcategoryRef.current) params.set("subcategory", selectedSubcategoryRef.current);
    else params.delete("subcategory");
    if (sortOptionRef.current !== "default") params.set("sort", sortOptionRef.current);
    else params.delete("sort");
    if (currentPageRef.current > 1) params.set("page", String(currentPageRef.current));
    else params.delete("page");
    if (extra) extra(params);
    const qs = params.toString();
    startTransition(() => {
      router.replace(qs ? `?${qs}` : "/", { scroll: false });
    });
  }

  // Sync committed state from URL when it changes (router.back(), etc.)
  useEffect(() => {
    startTransition(() => {
      setSearchQuery(searchParams.get("q") ?? "");
      setInputValue(searchParams.get("q") ?? "");
      setSelectedCategory(searchParams.get("category") ?? null);
      setSelectedSubcategory(searchParams.get("subcategory") ?? null);
      setSortOption((searchParams.get("sort") as "default" | "price-asc" | "price-desc") ?? "default");
      setCurrentPage(parseInt(searchParams.get("page") ?? "1", 10));
    });
  }, [searchParams]);

  // Hook 1: Browse mode — standard paginated fetch
  useEffect(() => {
    if (searchQuery) return;

    let cancelled = false;
    setPageLoading(true);

    const sortParam = sortOption !== "default" ? sortOption.replace("-", "_") : undefined;

    const specs: Record<string, string> = {};
    for (const [key, val] of searchParams.entries()) {
      if (key.startsWith("spec_")) {
        specs[key.slice(5)] = val;
      }
    }
    const minPrice = searchParams.get("min_price") ?? undefined;
    const maxPrice = searchParams.get("max_price") ?? undefined;

    fetchProducts(
      currentPage,
      selectedCategory || undefined,
      selectedSubcategory || undefined,
      sortParam,
      Object.keys(specs).length > 0 ? specs : undefined,
      minPrice,
      maxPrice,
    )
      .then((res) => {
        if (!cancelled) {
          setProducts(res.data ?? []);
          setCurrentPage(res.page);
          setTotalPages(res.totalPages);
          setTotal(res.total);
          setFacets(res.facets ?? undefined);
          setLoading(false);
          setSearching(false);
          setPageLoading(false);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load products");
          setLoading(false);
          setSearching(false);
          setPageLoading(false);
        }
      });

    return () => { cancelled = true; };
  }, [searchQuery, selectedCategory, selectedSubcategory, sortOption, currentPage, searchParams.toString()]);

  // Hook 2: Search mode — SSE streaming with optimistic cache
  useEffect(() => {
    if (!searchQuery) return;

    setError(null);

    // Optimistically load cache for instant UI, but still connect SSE for sync
    const cached = sessionStorage.getItem(`search:${searchQuery}`);
    if (cached) {
      try {
        const parsed = JSON.parse(cached) as Product[];
        setProducts(parsed);
        setTotal(parsed.length);
        setTotalPages(Math.ceil(parsed.length / ITEMS_PER_PAGE) || 1);
        setLoading(false);
      } catch {
        setProducts([]);
        setTotalPages(1);
        setTotal(0);
        setLoading(true);
      }
    } else {
      setProducts([]);
      setTotalPages(1);
      setTotal(0);
      setLoading(true);
    }

    setSearching(true);

    const sortParam = sortOption !== "default" ? sortOption.replace("-", "_") : undefined;
    let url = `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/search/stream?q=${encodeURIComponent(searchQuery)}`;
    if (sortParam) url += `&sort=${encodeURIComponent(sortParam)}`;

    const es = new EventSource(url);

    es.onmessage = (event) => {
      try {
        const parsedProducts = JSON.parse(event.data) as Product[];

        // Skip full state replacement when the product list is unchanged
        // (same IDs in the same order). This prevents unnecessary React
        // re-renders and UI reshuffling, especially when returning from a
        // product page where cached data matches the first SSE batch.
        const prev = productsRef.current;
        const changed = prev.length !== parsedProducts.length ||
          prev.some((p, i) => p.id !== parsedProducts[i].id);

        if (changed) {
          setProducts(parsedProducts);
        }
        setTotal(parsedProducts.length);
        setTotalPages(Math.ceil(parsedProducts.length / ITEMS_PER_PAGE) || 1);
        sessionStorage.setItem(`search:${searchQuery}`, JSON.stringify(parsedProducts));
      } catch (e) {
        console.error("[stream] parse error:", e);
      }
    };

    es.addEventListener("close", () => {
      es.close();
      setLoading(false);
      setSearching(false);
      setPageLoading(false);
    });

    es.onerror = () => {
      es.close();
    };

    return () => {
      es.close();
    };
  }, [searchQuery, sortOption]);

  // Hook 3: Autocomplete suggestions with debounce
  useEffect(() => {
    if (!inputValue.trim() || searchQuery === inputValue) {
      setSuggestions([]);
      setShowSuggestions(false);
      return;
    }

    const timer = setTimeout(async () => {
      const results = await fetchSuggestions(inputValue);
      setSuggestions(results);
      setShowSuggestions(results.length > 0);
      setSelectedSuggestionIndex(-1);
    }, 300);

    return () => clearTimeout(timer);
  }, [inputValue, searchQuery]);

  // Build return URL for product cards (preserves search state on navigation)
  const currentQueryString = searchParams.toString();
  const returnUrl = currentQueryString ? `/?${currentQueryString}` : "/";

  // Client-side slice while streaming search, server-side paginated for browse
  const displayProducts = searchQuery
    ? products.slice((currentPage - 1) * ITEMS_PER_PAGE, currentPage * ITEMS_PER_PAGE)
    : products;
  const displayTotal = total;
  const displayTotalPages = totalPages;

  const handleCategoryClick = (name: string) => {
    const newCat = selectedCategory === name ? null : name;
    setSelectedCategory(newCat);
    setSelectedSubcategory(null);
    setCurrentPage(1);
    setIsCategoryMenuOpen(false);
    setTimeout(() => pushURL((p) => {
      for (const key of [...p.keys()]) {
        if (key.startsWith("spec_") || key === "min_price" || key === "max_price") p.delete(key);
      }
    }), 0);
  };

  const handleSubcategoryClick = (categoryName: string, subName: string) => {
    const next = selectedCategory === categoryName && selectedSubcategory === subName;
    setSelectedCategory(next ? null : categoryName);
    setSelectedSubcategory(next ? null : subName);
    setCurrentPage(1);
    setIsCategoryMenuOpen(false);
    setTimeout(() => pushURL((p) => {
      for (const key of [...p.keys()]) {
        if (key.startsWith("spec_") || key === "min_price" || key === "max_price") p.delete(key);
      }
    }), 0);
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSearchQuery(inputValue);
    setCurrentPage(1);
    setShowSuggestions(false);
    // Search clears category/subcategory/specs from URL
    const params = new URLSearchParams();
    if (inputValue) params.set("q", inputValue);
    router.replace(params.toString() ? `?${params.toString()}` : "/", { scroll: false });
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputValue(e.target.value);
    setSelectedSuggestionIndex(-1);
  };

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || suggestions.length === 0) return;

    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedSuggestionIndex((prev) =>
        prev < suggestions.length - 1 ? prev + 1 : 0
      );
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedSuggestionIndex((prev) =>
        prev > 0 ? prev - 1 : suggestions.length - 1
      );
    } else if (e.key === "Enter" && selectedSuggestionIndex >= 0) {
      e.preventDefault();
      const selected = suggestions[selectedSuggestionIndex];
      setInputValue(selected);
      setSearchQuery(selected);
      setCurrentPage(1);
      setShowSuggestions(false);
      const params = new URLSearchParams();
      params.set("q", selected);
      router.replace(`?${params.toString()}`, { scroll: false });
    } else if (e.key === "Escape") {
      setShowSuggestions(false);
    }
  };

  const handleSuggestionClick = (suggestion: string) => {
    setInputValue(suggestion);
    setSearchQuery(suggestion);
    setCurrentPage(1);
    setShowSuggestions(false);
    const params = new URLSearchParams();
    params.set("q", suggestion);
    router.replace(`?${params.toString()}`, { scroll: false });
  };

  const handleClearFilters = () => {
    setSelectedCategory(null);
    setSelectedSubcategory(null);
    setCurrentPage(1);
    setTimeout(() => pushURL((p) => {
      for (const key of [...p.keys()]) {
        if (key.startsWith("spec_") || key === "min_price" || key === "max_price") p.delete(key);
      }
    }), 0);
  };

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-4">
              <Link
                href="/"
                onClick={handleClearFilters}
                className="text-2xl font-extrabold bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent tracking-tight shrink-0"
              >
                comparateur
              </Link>
              <div className="relative">
                <button
                  onClick={() => setIsCategoryMenuOpen((prev) => !prev)}
                  className="flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 cursor-pointer"
                >
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                  </svg>
                  Categories
                </button>
                {isCategoryMenuOpen && (
                  <>
                    <div className="fixed inset-0 z-30" onClick={() => setIsCategoryMenuOpen(false)} />
                    <div className="absolute left-0 top-full mt-2 z-50 w-[700px] rounded-xl border border-gray-200 bg-white shadow-lg p-0 flex">
                      {/* Left column: main categories */}
                      <div className="w-1/3 border-r border-gray-100 p-3 space-y-0.5">
                        <button
                          onClick={() => { handleClearFilters(); setIsCategoryMenuOpen(false); }}
                          className={`w-full rounded-lg px-3 py-2 text-sm font-medium text-left transition-colors cursor-pointer ${
                            !selectedCategory && !selectedSubcategory
                              ? "bg-blue-50 text-blue-700 font-semibold"
                              : "text-gray-700 hover:bg-gray-100"
                          }`}
                        >
                          All Products
                        </button>
                        {(Object.entries(CATEGORIES) as [string, Subcategory[]][]).map(([catName, subs]) => {
                          const isActive = hoveredMenuCategory === catName;
                          return (
                            <button
                              key={catName}
                              onMouseEnter={() => setHoveredMenuCategory(catName)}
                              onClick={() => { handleCategoryClick(catName); }}
                              className={`w-full rounded-lg px-3 py-2 text-sm font-medium text-left transition-colors cursor-pointer ${
                                isActive
                                  ? "bg-blue-50 text-blue-700 font-semibold"
                                  : "text-gray-700 hover:bg-gray-100"
                              }`}
                            >
                              {catName}
                            </button>
                          );
                        })}
                      </div>
                      {/* Right column: subcategories of hovered category */}
                      <div className="w-2/3 p-4">
                        {(() => {
                          const subs = CATEGORIES[hoveredMenuCategory as Category];
                          if (!subs) return null;
                          return (
                            <div className="grid grid-cols-2 gap-3">
                              {subs.map((sub) => {
                                const isSubSelected = selectedCategory === hoveredMenuCategory && selectedSubcategory === sub;
                                return (
                                  <button
                                    key={sub}
                                    onClick={() => handleSubcategoryClick(hoveredMenuCategory, sub)}
                                    className={`rounded-lg px-3 py-2 text-left text-sm transition-colors cursor-pointer ${
                                      isSubSelected
                                        ? "bg-blue-50 text-blue-700 font-semibold"
                                        : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
                                    }`}
                                  >
                                    {sub}
                                  </button>
                                );
                              })}
                            </div>
                          );
                        })()}
                      </div>
                    </div>
                  </>
                )}
              </div>
            </div>
            <form onSubmit={handleSearchSubmit} className="relative flex-1 max-w-md">
              <svg className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                type="text"
                placeholder="Search products..."
                value={inputValue}
                onChange={handleInputChange}
                onKeyDown={handleInputKeyDown}
                onFocus={() => { if (suggestions.length > 0) setShowSuggestions(true); }}
                onBlur={() => setTimeout(() => setShowSuggestions(false), 150)}
                className="w-full rounded-lg border border-gray-300 bg-gray-50 py-2 pl-10 pr-4 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:bg-white focus:outline-none focus:ring-1 focus:ring-blue-500 transition-colors"
              />
              {showSuggestions && suggestions.length > 0 && (
                <ul className="absolute top-full left-0 right-0 z-50 mt-1 max-h-60 overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg">
                  {suggestions.map((s, i) => (
                    <li
                      key={i}
                      onMouseDown={() => handleSuggestionClick(s)}
                      className={`px-4 py-2 text-sm cursor-pointer transition-colors ${
                        i === selectedSuggestionIndex
                          ? "bg-blue-50 text-blue-700"
                          : "text-gray-700 hover:bg-gray-50"
                      }`}
                    >
                      {s}
                    </li>
                  ))}
                </ul>
              )}
            </form>
          </div>
        </div>
      </header>

      <div className="mx-auto flex w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:px-8">
        {selectedSubcategory && (
          <aside className="hidden w-56 shrink-0 md:block">
            <Suspense fallback={<div className="text-sm text-gray-400 p-4">Loading filters…</div>}>
              <div className="sticky top-24 rounded-xl border border-gray-200 bg-white p-4 shadow-sm">
                <DynamicFilters facets={facets} />
              </div>
            </Suspense>
          </aside>
        )}
        <main className="flex-1 min-w-0">
          {loading && !products.length ? (
            <div className="flex flex-col items-center justify-center py-20">
              <svg className="h-8 w-8 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
              </svg>
              <p className="mt-4 text-sm font-medium text-gray-700">
                {searching ? "Searching for products..." : "Loading products..."}
              </p>
            </div>
          ) : error ? (
            <div className="flex flex-col items-center justify-center py-20">
              <svg className="h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
              </svg>
              <p className="mt-4 text-sm font-medium text-red-600">{error}</p>
              <button
                onClick={() => {
                  setLoading(true);
                  setError(null);
                  setCurrentPage(1);
                }}
                className="mt-4 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 transition-colors cursor-pointer"
              >
                Retry
              </button>
            </div>
          ) : (
            <>
              <div className="mb-4 flex items-center justify-between">
                <p className="text-sm text-gray-500">
                  {displayTotal} product{displayTotal !== 1 ? "s" : ""} found
                </p>
                <select
                  value={sortOption}
                  onChange={(e) => { setSortOption(e.target.value as "default" | "price-asc" | "price-desc"); setCurrentPage(1); setTimeout(() => pushURL(), 0); }}
                  className="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs text-gray-700 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 transition-colors cursor-pointer"
                >
                  <option value="default">Default</option>
                  <option value="price-asc">Price: Low → High</option>
                  <option value="price-desc">Price: High → Low</option>
                </select>
              </div>
              {pageLoading ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                  {Array.from({ length: 24 }).map((_, i) => (
                    <SkeletonCard key={i} />
                  ))}
                </div>
              ) : displayProducts.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-20">
                  <svg className="h-16 w-16 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                  </svg>
                  {searchQuery ? (
                    <>
                      <p className="mt-4 text-sm font-medium text-gray-700">No results found</p>
                      <p className="mt-1 text-xs text-gray-400">Try a different search term</p>
                    </>
                  ) : (
                    <>
                      <p className="mt-4 text-sm font-medium text-gray-700">No products yet</p>
                      <p className="mt-1 text-xs text-gray-400">Use the search bar above to find products</p>
                    </>
                  )}
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                  {displayProducts.map((product) => (
                    <ProductCard key={product.id} product={product} returnTo={returnUrl} />
                  ))}
                </div>
              )}
              {displayTotalPages > 1 && (
                <div className="mt-8 flex items-center justify-center gap-2">
                  <button
                    onClick={() => { setCurrentPage((p) => Math.max(1, p - 1)); setTimeout(() => pushURL(), 0); }}
                    disabled={currentPage <= 1}
                    className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40 cursor-pointer"
                  >
                    Previous
                  </button>
                  {generatePageNumbers(currentPage, displayTotalPages).map((p, i) =>
                    p === -1 ? (
                      <span key={`ellipsis-${i}`} className="px-2 text-sm text-gray-400">...</span>
                    ) : (
                      <button
                        key={p}
                        onClick={() => { setCurrentPage(p); setTimeout(() => pushURL(), 0); }}
                        className={`rounded-lg px-3 py-2 text-sm font-medium transition-colors cursor-pointer ${
                          p === currentPage
                            ? "bg-blue-600 text-white"
                            : "border border-gray-300 bg-white text-gray-700 hover:bg-gray-50"
                        }`}
                      >
                        {p}
                      </button>
                    ),
                  )}
                  <button
                    onClick={() => { setCurrentPage((p) => Math.min(displayTotalPages, p + 1)); setTimeout(() => pushURL(), 0); }}
                    disabled={currentPage >= displayTotalPages}
                    className="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40 cursor-pointer"
                  >
                    Next
                  </button>
                </div>
              )}
            </>
          )}
        </main>
      </div>
    </div>
  );
}

export default function Home() {
  return (
    <Suspense fallback={
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <svg className="h-8 w-8 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
          </svg>
          <p className="text-sm font-medium text-gray-700">Loading...</p>
        </div>
      </div>
    }>
      <HomeContent />
    </Suspense>
  );
}
