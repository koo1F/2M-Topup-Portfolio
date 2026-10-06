"use client";

import React, { useEffect, useState, Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, AlertCircle, Loader2, LayoutDashboard, Wallet } from "lucide-react";

function PaymentSuccessContent() {
  const searchParams = useSearchParams();
  const paymentId = searchParams.get("payment_id");

  const [result, setResult] = useState<{ paymentId: string; status?: string; amount?: number; error?: string }>();
  const current = result?.paymentId === paymentId ? result : undefined;
  const loading = Boolean(paymentId) && !current;
  const error = paymentId ? current?.error ?? "" : "No payment ID was found in the URL.";
  const status = current?.status ?? null;
  const amount = current?.amount ?? null;

  useEffect(() => {
    if (!paymentId) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const checkStatus = async () => {
      try {
        const res = await fetch(`/api/payment/${paymentId}/status`, { signal: controller.signal });
        if (!res.ok) {
          const errData = await res.json().catch(() => ({}));
          throw new Error(errData.error || "Failed to fetch payment status");
        }
        const data = await res.json();
        if (controller.signal.aborted) return;
        setResult({ paymentId, status: data.status, amount: data.amount });
        if (data.status === "PENDING") timer = setTimeout(checkStatus, 1500);
      } catch (err: unknown) {
        if (!controller.signal.aborted) setResult({ paymentId, error: err instanceof Error ? err.message : "Failed to verify payment status." });
      }
    };
    void checkStatus();
    return () => { controller.abort(); if (timer) clearTimeout(timer); };
  }, [paymentId]);

  return (
    <div className="max-w-md w-full bg-slate-900/60 rounded-3xl p-8 border border-slate-800/80 shadow-2xl text-center space-y-6 flex flex-col items-center">
      {loading && (
        <div className="py-8 space-y-4 flex flex-col items-center">
          <Loader2 className="w-12 h-12 text-indigo-500 animate-spin" />
          <h2 className="text-xl font-bold text-white">Verifying payment status...</h2>
          <p className="text-sm text-slate-400">Please wait while we confirm your transaction with Stripe.</p>
        </div>
      )}

      {!loading && error && (
        <div className="py-6 space-y-4 flex flex-col items-center">
          <div className="w-16 h-16 bg-rose-500/10 rounded-full flex items-center justify-center text-rose-400 border border-rose-500/20">
            <AlertCircle className="w-10 h-10" />
          </div>
          <div className="space-y-1">
            <h2 className="text-2xl font-bold text-white">Verification Failed</h2>
            <p className="text-sm text-slate-400">{error}</p>
          </div>
          <div className="pt-4 flex gap-4">
            <Link href="/payment" className="px-6 py-3 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold rounded-2xl transition duration-150 cursor-pointer">
              Back to Top Up
            </Link>
          </div>
        </div>
      )}

      {!loading && !error && status === "SUCCESS" && (
        <div className="py-6 space-y-4 flex flex-col items-center">
          <div className="w-16 h-16 bg-emerald-500/10 rounded-full flex items-center justify-center text-emerald-400 border border-emerald-500/20">
            <CheckCircle2 className="w-10 h-10 animate-bounce" />
          </div>
          <div className="space-y-1">
            <h2 className="text-2xl font-bold text-white">Payment Successful ✓</h2>
            {amount !== null && (
              <p className="text-3xl font-extrabold text-indigo-400 font-mono my-2">
                ฿{amount.toLocaleString("en-US", { minimumFractionDigits: 2 })}
              </p>
            )}
            <p className="text-sm text-slate-400">
              Your wallet has been topped up successfully.
            </p>
          </div>
          <div className="pt-4 flex gap-4 w-full justify-center">
            <Link href="/dashboard" className="px-6 py-3 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold rounded-2xl transition duration-150 flex items-center gap-2 shadow-lg shadow-indigo-600/20">
              <LayoutDashboard className="w-4 h-4" />
              Dashboard
            </Link>
            <Link href="/wallet" className="px-6 py-3 bg-slate-950 hover:bg-slate-900 border border-slate-800 text-slate-300 font-semibold rounded-2xl transition duration-150 flex items-center gap-2">
              <Wallet className="w-4 h-4" />
              Wallet
            </Link>
          </div>
        </div>
      )}

      {!loading && !error && status !== "SUCCESS" && (
        <div className="py-6 space-y-4 flex flex-col items-center">
          <div className="w-16 h-16 bg-amber-500/10 rounded-full flex items-center justify-center text-amber-400 border border-amber-500/20">
            <AlertCircle className="w-10 h-10" />
          </div>
          <div className="space-y-1">
            <h2 className="text-2xl font-bold text-white">Payment Status: {status}</h2>
            <p className="text-sm text-slate-400">
              Your payment session is in status &quot;{status}&quot;. It has not been successfully settled yet.
            </p>
          </div>
          <div className="pt-4 flex gap-4">
            <Link href="/payment" className="px-6 py-3 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold rounded-2xl transition duration-150 cursor-pointer">
              Try Again
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}

export default function PaymentSuccessPage() {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col items-center justify-center p-4">
      <Suspense fallback={
        <div className="max-w-md w-full bg-slate-900/60 rounded-3xl p-8 border border-slate-800/80 shadow-2xl text-center flex flex-col items-center justify-center py-12">
          <Loader2 className="w-12 h-12 text-indigo-500 animate-spin" />
          <h2 className="text-xl font-bold text-white mt-4">Loading verification...</h2>
        </div>
      }>
        <PaymentSuccessContent />
      </Suspense>
    </div>
  );
}
