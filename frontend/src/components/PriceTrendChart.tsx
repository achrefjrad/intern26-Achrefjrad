"use client";

import { useState, useEffect, useCallback } from "react";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { fetchPriceHistory } from "@/services/api";
import type { PriceHistoryResponse } from "@/types/product";

function formatPrice(price: number): string {
  return price.toLocaleString(undefined, {
    minimumFractionDigits: 3,
    maximumFractionDigits: 3,
  }) + " TND";
}

function formatShortDate(dateStr: string): string {
  const d = new Date(dateStr + "T00:00:00");
  return `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function CustomTooltip({ active, payload }: { active?: boolean; payload?: { value: number }[] }) {
  if (!active || !payload || payload.length === 0) return null;
  return (
    <div className="bg-white border border-gray-200 rounded-lg shadow-lg px-3 py-2">
      <p className="text-sm font-bold text-gray-900">{formatPrice(payload[0].value)}</p>
    </div>
  );
}

export default function PriceTrendChart({ productId }: { productId: string }) {
  const [data, setData] = useState<PriceHistoryResponse | null>(null);

  const load = useCallback(() => {
    fetchPriceHistory(productId).then(setData).catch(() => setData(null));
  }, [productId]);

  useEffect(() => { load(); }, [load]);

  if (!data || data.history.length < 2) return null;

  const noChange = data.change_percentage === 0;
  const changeColor = noChange ? "text-gray-500" : data.change_percentage < 0 ? "text-emerald-500" : "text-red-500";

  const allSame = data.history.every((p) => p.price === data.history[0].price);

  return (
    <div className="bg-white rounded-lg border border-gray-200 shadow-sm p-5">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-xs font-bold text-gray-800 uppercase tracking-wider">
          TENDANCE PRIX
        </h3>
        <span className={`text-sm font-bold ${changeColor}`}>
          {data.change_percentage > 0 ? "+" : ""}
          {data.change_percentage.toFixed(2)}%
        </span>
      </div>
      <ResponsiveContainer width="100%" height={200}>
        <AreaChart data={data.history} margin={{ top: 5, right: 5, left: -20, bottom: 0 }}>
          <defs>
            <linearGradient id="priceGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2} />
              <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="date"
            tickFormatter={formatShortDate}
            tick={{ fontSize: 11, fill: "#9ca3af" }}
            axisLine={false}
            tickLine={false}
            interval="preserveStartEnd"
          />
          <YAxis
            domain={
              allSame
                ? [(dataMin: number) => dataMin - 10, (dataMax: number) => dataMax + 10]
                : ["auto", "auto"]
            }
            tick={{ fontSize: 11, fill: "#9ca3af" }}
            axisLine={false}
            tickLine={false}
            tickFormatter={(v: number) => `${v.toFixed(0)}`}
          />
          <Tooltip content={<CustomTooltip />} cursor={{ stroke: "#d1d5db", strokeDasharray: "3 3" }} />
          <Area
            type="monotone"
            dataKey="price"
            stroke="#3b82f6"
            strokeWidth={2}
            fill="url(#priceGradient)"
            dot={false}
            activeDot={{ r: 4, fill: "#3b82f6", stroke: "#fff", strokeWidth: 2 }}
          />
        </AreaChart>
      </ResponsiveContainer>
      <button
        className="mt-4 w-full text-center text-xs font-semibold text-gray-500 border border-gray-200 rounded-md py-2 hover:bg-gray-50 hover:text-gray-700 transition-colors uppercase tracking-wider"
        onClick={() => alert("Fonctionnalité à venir")}
      >
        Activer une alerte prix
      </button>
    </div>
  );
}