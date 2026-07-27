"use client";

import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { useState, useEffect, useRef } from "react";
import { fetchFilterSchema } from "@/services/api";

const FILTER_LABELS: Record<string, string> = {
  marque: "Marque",
  type_de_peau: "Type de peau",
  type_de_cheveux: "Type de cheveux",
  besoin: "Besoin",
  besoin_cheveux: "Besoin cheveux",
  texture_de_soin: "Texture",
  zone: "Zone",
  finition: "Finition",
  indice_spf: "Indice SPF",
  contenance: "Contenance",
  ingredients_actifs: "Ingrédients actifs",
  parfum: "Parfum",
  gamme: "Gamme",
  type_de_produit: "Type de produit",
  probleme_specifique: "Problème spécifique",
  processeur: "Processeur",
  ram: "RAM",
  stockage: "Stockage",
  ecran: "Écran",
  carte_graphique: "Carte graphique",
  os: "Système d'exploitation",
  resolution: "Résolution",
  connectivite: "Connectivité",
  format: "Format",
  couleur: "Couleur",
  batterie: "Batterie",
  resolution_appareil_photo: "Résolution appareil photo",
  double_sim: "Double SIM",
  capacite: "Capacité",
  classe_energetique: "Classe énergétique",
  dimensions: "Dimensions",
  puissance: "Puissance",
  nombre_de_programmes: "Nombre de programmes",
  niveau_sonore: "Niveau sonore",
  tension: "Tension",
  pression: "Pression",
  largeur_coupe: "Largeur de coupe",
};

const filterKeyMapping: Record<string, string> = {
  "mémoire": "ram",
  "mémoire vive (ram)": "ram",
  "taille ecran": "écran",
  "taille de l'écran": "écran",
  "capacité de stockage": "stockage",
  "disque dur": "stockage",
  "processeur": "processeur",
  "système d'exploitation": "os",
  "carte graphique": "carte_graphique",
};

function normalizeFilterValue(value: string): string {
  return value.replace(/\bGo\b/g, "GB").replace(/\bTo\b/g, "TB");
}

function translateFilterName(name: string): string {
  const lower = name.toLowerCase().trim();
  if (filterKeyMapping[lower]) return filterKeyMapping[lower];
  const noAccents = lower.normalize("NFD").replace(/[\u0300-\u036f]/g, "");
  return noAccents.replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "");
}

// ---- Sub-components ----

function CollapsibleSection({
  title,
  defaultOpen = true,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-gray-200 pb-3 mb-3 last:border-b-0 last:pb-0 last:mb-0">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between text-xs font-bold uppercase tracking-wider text-gray-700 cursor-pointer hover:text-gray-900"
      >
        <span>{title}</span>
        <svg
          className={`h-3 w-3 transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && <div className="mt-2">{children}</div>}
    </div>
  );
}

function PriceRange({
  minPrice,
  maxPrice,
  onMinChange,
  onMaxChange,
  onMinBlur,
  onMaxBlur,
  onSliderCommit,
}: {
  minPrice: string;
  maxPrice: string;
  onMinChange: (v: string) => void;
  onMaxChange: (v: string) => void;
  onMinBlur: () => void;
  onMaxBlur: () => void;
  onSliderCommit: () => void;
}) {
  const minVal = parseInt(minPrice || "0", 10);
  const maxVal = parseInt(maxPrice || "10000", 10);

  return (
    <div className="space-y-3">
      <div className="relative h-2 mt-2">
        <div className="absolute inset-0 rounded-full bg-gray-200" />
        <div
          className="absolute top-0 bottom-0 rounded-full bg-blue-600"
          style={{ left: `${(Math.min(minVal, 10000) / 10000) * 100}%`, right: `${100 - (Math.min(maxVal, 10000) / 10000) * 100}%` }}
        />
        <input
          type="range"
          min={0}
          max={10000}
          step={10}
          value={minVal}
          onChange={(e) => onMinChange(e.target.value)}
          onMouseUp={onSliderCommit}
          onTouchEnd={onSliderCommit}
          className="absolute inset-0 w-full appearance-none bg-transparent pointer-events-none [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-blue-600 [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-white [&::-webkit-slider-thumb]:shadow [&::-webkit-slider-thumb]:cursor-pointer [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-blue-600 [&::-moz-range-thumb]:border-2 [&::-moz-range-thumb]:border-white [&::-moz-range-thumb]:shadow [&::-moz-range-thumb]:cursor-pointer"
        />
        <input
          type="range"
          min={0}
          max={10000}
          step={10}
          value={maxVal}
          onChange={(e) => onMaxChange(e.target.value)}
          onMouseUp={onSliderCommit}
          onTouchEnd={onSliderCommit}
          className="absolute inset-0 w-full appearance-none bg-transparent pointer-events-none [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-blue-600 [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-white [&::-webkit-slider-thumb]:shadow [&::-webkit-slider-thumb]:cursor-pointer [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-blue-600 [&::-moz-range-thumb]:border-2 [&::-moz-range-thumb]:border-white [&::-moz-range-thumb]:shadow [&::-moz-range-thumb]:cursor-pointer"
        />
      </div>
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <input
            type="number"
            min={0}
            placeholder="Min"
            value={minPrice}
            onChange={(e) => onMinChange(e.target.value)}
            onBlur={onMinBlur}
            className="w-full rounded-md border border-gray-300 py-1.5 pl-2 pr-7 text-xs text-gray-700 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <span className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-gray-400">DT</span>
        </div>
        <span className="text-gray-400 text-xs">-</span>
        <div className="relative flex-1">
          <input
            type="number"
            min={0}
            placeholder="Max"
            value={maxPrice}
            onChange={(e) => onMaxChange(e.target.value)}
            onBlur={onMaxBlur}
            className="w-full rounded-md border border-gray-300 py-1.5 pl-2 pr-7 text-xs text-gray-700 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <span className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-gray-400">DT</span>
        </div>
      </div>
    </div>
  );
}

function CheckboxList({
  paramKey,
  options,
  searchParams,
  onToggle,
}: {
  paramKey: string;
  options: { value: string; label: string }[];
  searchParams: URLSearchParams;
  onToggle: (key: string, value: string, checked: boolean) => void;
}) {
  const list = (
    <div className="space-y-1.5">
      {options.map((opt) => {
        const current = searchParams.get(paramKey);
        const selected = current ? current.split(",").includes(opt.value) : false;
        return (
          <label
            key={opt.value}
            className="flex items-center gap-2 text-xs text-gray-600 cursor-pointer hover:text-gray-900 hover:bg-gray-50 rounded px-1 py-0.5 transition-colors"
          >
            <input
              type="checkbox"
              checked={selected}
              onChange={(e) => onToggle(paramKey, opt.value, e.target.checked)}
              className="accent-blue-600 rounded"
            />
            {opt.label}
          </label>
        );
      })}
    </div>
  );

  if (options.length > 8) {
    return <div className="overflow-y-auto max-h-48 -mr-1 pr-1">{list}</div>;
  }
  return list;
}

// ---- Main component ----

export default function DynamicFilters({ facets }: { facets?: Record<string, string[]> }) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const subcategory = searchParams.get("subcategory");

  const [schemaFilters, setSchemaFilters] = useState<Record<string, string[]> | null>(null);
  const [schemaLoading, setSchemaLoading] = useState(false);
  const [schemaError, setSchemaError] = useState(false);

  // Fetch filter schema from API when subcategory changes
  useEffect(() => {
    if (!subcategory) {
      setSchemaFilters(null);
      return;
    }

    let cancelled = false;
    setSchemaLoading(true);
    setSchemaError(false);

    fetchFilterSchema(subcategory)
      .then((res) => {
        if (!cancelled) {
          setSchemaFilters(res.filters || {});
          setSchemaLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSchemaError(true);
          setSchemaLoading(false);
        }
      });

    return () => { cancelled = true; };
  }, [subcategory]);

  // Price state — only used for input responsiveness, never pushes to router
  const [minPrice, setMinPrice] = useState(searchParams.get("min_price") ?? "");
  const [maxPrice, setMaxPrice] = useState(searchParams.get("max_price") ?? "");

  // Keep a ref with the committed price values for commitPrice
  const minPriceRef = useRef(minPrice);
  const maxPriceRef = useRef(maxPrice);

  // Sync from URL when browser navigates back/forward
  useEffect(() => {
    setMinPrice(searchParams.get("min_price") ?? "");
    setMaxPrice(searchParams.get("max_price") ?? "");
    minPriceRef.current = searchParams.get("min_price") ?? "";
    maxPriceRef.current = searchParams.get("max_price") ?? "";
  }, [searchParams]);

  // Build and push URL with current price refs + current search params
  function commitPrice() {
    const params = new URLSearchParams(searchParams.toString());
    if (minPriceRef.current) params.set("min_price", minPriceRef.current);
    else params.delete("min_price");
    if (maxPriceRef.current) params.set("max_price", maxPriceRef.current);
    else params.delete("max_price");
    params.delete("page");
    const qs = params.toString();
    router.push(qs ? `?${qs}` : pathname, { scroll: false });
  }

  function handlePriceChange(type: "min" | "max", value: string) {
    if (type === "min") {
      setMinPrice(value);
      minPriceRef.current = value;
    } else {
      setMaxPrice(value);
      maxPriceRef.current = value;
    }
  }

  function handleSpecToggle(esKey: string, value: string, checked: boolean) {
    const params = new URLSearchParams(searchParams.toString());
    const paramKey = `spec_${esKey}`;
    if (checked) params.set(paramKey, value);
    else params.delete(paramKey);
    params.delete("page");
    const qs = params.toString();
    router.push(qs ? `?${qs}` : pathname, { scroll: false });
  }

  return (
    <div>
      {/* Price */}
      <CollapsibleSection title="Prix">
        <PriceRange
          minPrice={minPrice}
          maxPrice={maxPrice}
          onMinChange={(v) => handlePriceChange("min", v)}
          onMaxChange={(v) => handlePriceChange("max", v)}
          onMinBlur={commitPrice}
          onMaxBlur={commitPrice}
          onSliderCommit={commitPrice}
        />
      </CollapsibleSection>

      {/* Dynamic filters from API */}
      {schemaLoading && (
        <div className="text-xs text-gray-400 py-2 animate-pulse">Loading filters...</div>
      )}
      {schemaError && (
        <div className="text-xs text-red-400 py-2">Failed to load filters</div>
      )}
      {(() => {
        const schemaRendered = new Set<string>();

        const schemaEls = !schemaLoading && !schemaError
          ? Object.entries(schemaFilters || {}).map(([frenchTitle, values]) => {
              const esKey = translateFilterName(frenchTitle);
              schemaRendered.add(esKey);
              const paramKey = `spec_${esKey}`;
              return (
                <CollapsibleSection key={esKey} title={frenchTitle}>
                  <CheckboxList
                    paramKey={paramKey}
                    options={(values || []).map((v) => ({ value: normalizeFilterValue(v), label: v }))}
                    searchParams={searchParams}
                    onToggle={(_key, value, checked) => handleSpecToggle(esKey, value, checked)}
                  />
                </CollapsibleSection>
              );
            })
          : [];

        const facetEls = facets
          ? Object.entries(facets)
              .filter(([esKey]) => !schemaRendered.has(esKey))
              .map(([esKey, values]) => {
                if (!values || values.length < 2) return null;
                const label = FILTER_LABELS[esKey] || esKey.replace(/_/g, " ");
                return (
                  <CollapsibleSection key={esKey} title={label}>
                    <CheckboxList
                      paramKey={`spec_${esKey}`}
                      options={values.map((v) => ({ value: v, label: v }))}
                      searchParams={searchParams}
                      onToggle={(_key, value, checked) => handleSpecToggle(esKey, value, checked)}
                    />
                  </CollapsibleSection>
                );
              })
          : [];

        return [...schemaEls, ...facetEls];
      })()}
    </div>
  );
}
