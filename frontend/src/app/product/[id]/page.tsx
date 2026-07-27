"use client";

import { useState, useEffect, Suspense } from "react";
import { useParams, useSearchParams } from "next/navigation";
import Link from "next/link";
import { ProductDetailResponse } from "@/types/product";
import { fetchProductById } from "@/services/api";

function formatPrice(price: number): string {
  return price.toLocaleString(undefined, { minimumFractionDigits: 3, maximumFractionDigits: 3 }) + " TND";
}

function retailerName(source?: string): string {
  const s = (source ?? "").toLowerCase();
  if (s.includes("tunisianet")) return "Tunisianet";
  if (s.includes("mytech")) return "MyTek";
  if (s.includes("wiki")) return "Wiki.tn";
  if (s.includes("sbs")) return "SBS Informatique";
  if (s.includes("beautystore")) return "BeautyStore";
  if (s.includes("fatales")) return "Fatales";
  return source ?? "Inconnu";
}

function ProductDetailContent() {
  const { id } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const returnTo = searchParams.get("returnTo") || "/";
  const [data, setData] = useState<ProductDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchProductById(id)
      .then((res) => {
        if (!cancelled) {
          setData(res);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load product");
          setData(null);
          setLoading(false);
        }
      });
    return () => { cancelled = true; };
  }, [id]);

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <svg className="h-8 w-8 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
          </svg>
          <p className="text-sm text-gray-500">Loading product...</p>
        </div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="min-h-screen bg-gray-50 flex flex-col items-center justify-center gap-4">
        <svg className="h-16 w-16 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <h2 className="text-xl font-semibold text-gray-900">Product Not Found</h2>
        <p className="text-sm text-gray-500">{error ?? "The product you are looking for does not exist."}</p>
        <Link
          href={returnTo}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors mb-6 cursor-pointer"
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Retour aux produits
        </Link>
      </div>
    );
  }

  const { product, offers } = data;
  const bestOffer = offers.length > 0 ? offers[0] : null;

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="max-w-6xl mx-auto py-8 px-4">
        <Link
          href={returnTo}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors mb-6 cursor-pointer"
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Retour aux produits
        </Link>

        <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 mb-8 flex flex-col md:flex-row gap-8">
          <div className="w-full md:w-1/3 flex justify-center items-center p-4">
            {product.image ? (
              <img src={product.image} alt={product.title || product.name} className="w-full h-full object-contain max-h-64" />
            ) : (
              <div className="text-center">
                <svg className="w-20 h-20 mx-auto text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
                <span className="block mt-3 text-sm font-medium text-gray-400">Product Image</span>
              </div>
            )}
          </div>

          <div className="w-full md:w-1/3 flex flex-col justify-center">
            {product.subcategory && (
              <span className="text-xs font-semibold text-blue-600 uppercase tracking-wider mb-1">{product.subcategory}</span>
            )}
            <h1 className="text-2xl font-bold text-gray-900 mb-2">{product.title || product.name}</h1>
            {product.category && (
              <span className="text-xs text-gray-400 uppercase tracking-wider mb-3">{product.category}</span>
            )}
            {product.description && (
              <p className="text-sm text-gray-500 line-clamp-4">{product.description}</p>
            )}
            {product.reference && (
              <div className="flex items-center gap-2 mt-3">
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">Ref:</span>
                <span className="text-sm font-mono text-gray-800 bg-gray-100 px-2 py-0.5 rounded">{product.reference}</span>
              </div>
            )}
          </div>

          <div className="w-full md:w-1/3">
            {bestOffer ? (
              <div className="border border-gray-200 rounded-lg p-6 flex flex-col items-center justify-center text-center shadow-sm">
                <span className="text-xs font-bold text-[#358765] tracking-wide mb-2">MEILLEURE OFFRE ACTUELLE</span>
                <span className="text-3xl font-extrabold text-gray-900 mb-2">{formatPrice(bestOffer.price)}</span>
                <span className="text-xs text-gray-500 mb-4">Comparer {offers.length} offre{offers.length > 1 ? "s" : ""}</span>
                {bestOffer.url && (
                  <a
                    href={bestOffer.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="bg-[#428b69] hover:bg-[#357356] text-white font-medium py-3 px-4 rounded-md w-full transition-colors text-center"
                  >
                    Visiter la boutique
                  </a>
                )}
              </div>
            ) : (
              <div className="border border-gray-200 rounded-lg p-6 flex flex-col items-center justify-center text-center shadow-sm">
                <span className="text-xs font-bold text-gray-400 tracking-wide mb-2">AUCUNE OFFRE</span>
                <span className="text-lg font-semibold text-gray-400">Produit unique</span>
              </div>
            )}
          </div>
        </div>

        {offers.length > 0 && (
          <div>
            {(() => {
              const sortedOffers = [...offers].sort((a, b) => a.price - b.price);
              return (
                <>
                  <h2 className="text-lg font-bold text-gray-900 mb-4">
                    {sortedOffers.length} offre{sortedOffers.length > 1 ? "s" : ""} disponible{sortedOffers.length > 1 ? "s" : ""}
                  </h2>
                  <div>
                    {sortedOffers.map((offer) => (
                <div
                  key={offer.id}
                  className="bg-white border border-gray-200 rounded-lg p-4 mb-3 flex flex-col md:flex-row items-center justify-between shadow-sm hover:shadow-md transition-shadow"
                >
                  <div className="flex flex-col items-center md:items-start md:w-1/6">
                    <span className="text-sm font-extrabold text-gray-800 uppercase border-b-2 border-red-600 pb-1">
                      {retailerName(offer.source)}
                    </span>
                  </div>

                  <p className="text-sm font-medium text-blue-900 flex-1 px-6 text-center md:text-left my-3 md:my-0">
                    {offer.title || offer.name}
                  </p>

                  <div className="flex items-center gap-4 shrink-0">
                    <span className="text-xl font-bold text-[#c92a2a]">{formatPrice(offer.price)}</span>
                    {offer.url ? (
                      <a
                        href={offer.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="bg-[#428b69] hover:bg-[#357356] text-white px-5 py-2 rounded-md text-sm font-medium flex items-center gap-2 transition-colors"
                      >
                        Visiter la boutique
                        <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
                        </svg>
                      </a>
                    ) : (
                      <span className="bg-gray-200 text-gray-400 px-5 py-2 rounded-md text-sm font-medium cursor-not-allowed">
                        Indisponible
                      </span>
                    )}
                  </div>
                  </div>
                ))}
              </div>
            </>);
          })()}
        </div>
      )}
      </div>
    </div>
  );
}

export default function ProductDetailPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <svg className="h-8 w-8 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
          </svg>
          <p className="text-sm text-gray-500">Loading...</p>
        </div>
      </div>
    }>
      <ProductDetailContent />
    </Suspense>
  );
}
