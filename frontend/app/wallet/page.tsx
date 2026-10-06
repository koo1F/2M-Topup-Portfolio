"use client";

import React, { useState } from "react";
import { useRemoteData } from "@/lib/use-remote-data";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LogOut, LayoutDashboard, Wallet, CreditCard, ChevronLeft, ChevronRight, AlertCircle } from "lucide-react";
import TransactionList, { Transaction } from "@/components/TransactionList";

export default function WalletPage() {
  const router = useRouter();
  const [page, setPage] = useState(1);
  const limit = 10;
  const history = useRemoteData<{ data: Transaction[]; pagination: { total: number } }>(`/api/wallet/transactions?page=${page}&limit=${limit}`);
  const transactions = history.data?.data ?? [];
  const total = history.data?.pagination?.total ?? 0;
  const loading = history.loading;
  const error = history.error;

  const handleLogout = async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
      router.push("/login");
      router.refresh();
    } catch (err) {
      console.error("Logout failed", err);
    }
  };

  const totalPages = Math.ceil(total / limit);

  return (
    <div className="min-h-screen bg-canvas-parchment text-ink flex flex-col select-none">
      {/* Global Nav: Row 1 */}
      <nav className="component-global-nav text-white text-nav-link relative z-50">
        <div className="max-w-7xl mx-auto px-6 w-full flex items-center justify-between">
          <div className="flex items-center gap-8">
            <span className="font-semibold tracking-tight text-white mr-4">2M-Topup</span>
            <Link href="/dashboard" className="text-white/60 hover:text-white flex items-center gap-1.5 py-3 transition duration-150">
              <LayoutDashboard className="w-3.5 h-3.5" />
              Dashboard
            </Link>
            <Link href="/wallet" className="text-white font-semibold flex items-center gap-1.5 py-3 border-b-2 border-primary">
              <Wallet className="w-3.5 h-3.5" />
              Wallet
            </Link>
            <Link href="/payment" className="text-white/60 hover:text-white flex items-center gap-1.5 py-3 transition duration-150">
              <CreditCard className="w-3.5 h-3.5" />
              Top Up
            </Link>
          </div>
          <div className="flex items-center gap-4">
            <button
              onClick={handleLogout}
              className="text-white/60 hover:text-white flex items-center gap-1.5 cursor-pointer transition duration-150"
            >
              <LogOut className="w-3.5 h-3.5" />
              Logout
            </button>
          </div>
        </div>
      </nav>

      {/* Sub Nav Frosted: Row 2 */}
      <nav className="component-sub-nav-frosted">
        <div className="max-w-7xl mx-auto px-6 w-full flex items-center justify-between">
          <span className="text-tagline font-semibold text-ink">Transaction History</span>
          <div className="flex items-center gap-3">
            <Link href="/payment" className="button-primary">
              Top Up Wallet
            </Link>
            <button
              onClick={handleLogout}
              className="button-dark-utility"
            >
              Logout
            </button>
          </div>
        </div>
      </nav>

      {/* Main Layout */}
      <main className="flex-1 max-w-7xl mx-auto px-6 py-10 w-full space-y-6">
        <div>
          <h1 className="text-display-lg text-ink font-semibold tracking-[-0.28px]">Transaction History</h1>
          <p className="text-caption-apple text-ink-muted-48 mt-1">Review all your deposit, withdrawal and wallet activity details.</p>
        </div>

        {error && (
          <div className="bg-rose-50 border border-rose-200 text-rose-700 p-4 rounded-sm text-caption-apple flex items-center gap-3">
            <AlertCircle className="w-5 h-5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="space-y-4">
          <TransactionList transactions={transactions} loading={loading} />

          {/* Pagination Controls */}
          {totalPages > 1 && (
            <div className="flex justify-between items-center bg-white p-4 rounded-lg border border-hairline">
              <p className="text-caption-apple text-ink-muted-80 font-medium">
                Showing <span className="font-semibold text-ink">{(page - 1) * limit + 1}</span> to{" "}
                <span className="font-semibold text-ink">{Math.min(page * limit, total)}</span> of{" "}
                <span className="font-semibold text-ink">{total}</span> records
              </p>

              <div className="flex gap-2">
                <button
                  onClick={() => setPage((p) => Math.max(p - 1, 1))}
                  disabled={page === 1 || loading}
                  className="w-10 h-10 rounded-full border border-hairline bg-white text-ink flex items-center justify-center cursor-pointer transition-all duration-200 hover:bg-canvas-parchment active:scale-95 disabled:opacity-30 disabled:cursor-not-allowed"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setPage((p) => Math.min(p + 1, totalPages))}
                  disabled={page === totalPages || loading}
                  className="w-10 h-10 rounded-full border border-hairline bg-white text-ink flex items-center justify-center cursor-pointer transition-all duration-200 hover:bg-canvas-parchment active:scale-95 disabled:opacity-30 disabled:cursor-not-allowed"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
