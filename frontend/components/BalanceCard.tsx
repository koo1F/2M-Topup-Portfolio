"use client";

import React from "react";
import { Wallet, RefreshCw } from "lucide-react";

interface BalanceCardProps {
  balance: number | null;
  currency: string;
  loading: boolean;
  onRefresh?: () => void;
}

export default function BalanceCard({ balance, currency, loading, onRefresh }: BalanceCardProps) {
  return (
    <div className="bg-surface-tile-1 text-white rounded-lg p-6 min-h-[180px] flex flex-col justify-between relative select-none">
      <div className="flex justify-between items-start">
        <div className="flex items-center gap-3">
          <div className="p-2.5 bg-white/10 rounded-sm">
            <Wallet className="w-5 h-5 text-white" />
          </div>
          <div>
            <p className="text-caption-apple font-semibold text-body-muted">Wallet Balance</p>
            <p className="text-fine-print text-white/40">Primary Account</p>
          </div>
        </div>

        {onRefresh && (
          <button
            onClick={onRefresh}
            disabled={loading}
            className="w-10 h-10 rounded-full bg-white/10 text-white flex items-center justify-center cursor-pointer transition-all duration-200 hover:bg-white/20 active:scale-95 disabled:opacity-50"
            aria-label="Refresh balance"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />
          </button>
        )}
      </div>

      <div className="mt-6">
        {loading ? (
          <div className="h-10 w-48 bg-white/10 rounded-sm animate-pulse" />
        ) : (
          <div className="flex items-baseline gap-1.5">
            <span className="text-caption-strong text-primary-on-dark uppercase tracking-wider">{currency}</span>
            <span className="text-display-md text-white font-semibold">
              {balance !== null ? balance.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "0.00"}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
