"use client";

import React from "react";
import { useRemoteData } from "@/lib/use-remote-data";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LogOut, LayoutDashboard, Wallet, CreditCard, AlertCircle } from "lucide-react";
import BalanceCard from "@/components/BalanceCard";
import TransactionList, { Transaction } from "@/components/TransactionList";

export default function DashboardPage() {
  const router = useRouter();
  const wallet = useRemoteData<{ balance: number; currency: string }>("/api/wallet");
  const history = useRemoteData<{ data: Transaction[] }>("/api/wallet/transactions?page=1&limit=5");
  const balance = wallet.data?.balance ?? null;
  const currency = wallet.data?.currency ?? "THB";
  const transactions = history.data?.data ?? [];
  const loadingBalance = wallet.loading;
  const loadingTx = history.loading;
  const error = wallet.error || history.error;
  const fetchData = () => { wallet.refresh(); history.refresh(); };

  const handleLogout = async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
      router.push("/login");
      router.refresh();
    } catch (err) {
      console.error("Logout failed", err);
    }
  };

  return (
    <div className="min-h-screen bg-canvas-parchment text-ink flex flex-col select-none">
      {/* Global Nav: Row 1 */}
      <nav className="component-global-nav text-white text-nav-link relative z-50">
        <div className="max-w-7xl mx-auto px-6 w-full flex items-center justify-between">
          <div className="flex items-center gap-8">
            <span className="font-semibold tracking-tight text-white mr-4">2M-Topup</span>
            <Link href="/dashboard" className="text-white font-semibold flex items-center gap-1.5 py-3 border-b-2 border-primary">
              <LayoutDashboard className="w-3.5 h-3.5" />
              Dashboard
            </Link>
            <Link href="/wallet" className="text-white/60 hover:text-white flex items-center gap-1.5 py-3 transition duration-150">
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
          <span className="text-tagline font-semibold text-ink">Dashboard</span>
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
      <main className="flex-1 max-w-7xl mx-auto px-6 py-10 w-full space-y-8">
        {error && (
          <div className="bg-rose-50 border border-rose-200 text-rose-700 p-4 rounded-sm text-caption-apple flex items-center gap-3">
            <AlertCircle className="w-5 h-5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          {/* Balance card (Dark surface-tile-1 card) */}
          <div className="md:col-span-1">
            <BalanceCard
              balance={balance}
              currency={currency}
              loading={loadingBalance}
              onRefresh={fetchData}
            />
          </div>

          {/* Operations shortcuts (White store-utility-card) */}
          <div className="md:col-span-2 store-utility-card flex flex-col justify-between">
            <div>
              <h2 className="text-tagline font-semibold text-ink">Quick Actions</h2>
              <p className="text-caption-apple text-ink-muted-48 mt-1">Manage, deposit, and inspect your financial assets.</p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mt-6">
              <Link href="/payment" className="button-primary py-3.5 text-body-strong flex items-center gap-2">
                <CreditCard className="w-4 h-4" />
                Make Top Up
              </Link>
              <Link href="/wallet" className="button-secondary-pill py-3.5 text-body-strong flex items-center gap-2">
                <Wallet className="w-4 h-4" />
                Transaction History
              </Link>
            </div>
          </div>
        </div>

        {/* Recent Transactions section */}
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <h2 className="text-tagline font-semibold text-ink">Recent Activity</h2>
            <Link href="/wallet" className="text-caption-strong text-primary hover:underline transition duration-150">
              View All
            </Link>
          </div>
          <TransactionList transactions={transactions} loading={loadingTx} />
        </div>
      </main>
    </div>
  );
}
