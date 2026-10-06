"use client";

import React, { useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { QRCodeSVG } from "qrcode.react";
import { LogOut, LayoutDashboard, Wallet, CreditCard, Loader2, AlertCircle, CheckCircle2, Clock } from "lucide-react";

export default function PaymentPage() {
  const router = useRouter();
  const [amount, setAmount] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<"promptpay" | "card">("promptpay");

  // Active Payment Details
  const [paymentId, setPaymentId] = useState<string | null>(null);
  const [qrPayload, setQrPayload] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null); // PENDING, SUCCESS, EXPIRED, FAILED

  // Countdown and Polling Timers
  const [timeLeft, setTimeLeft] = useState<number>(0); // in seconds
  const timerRef = useRef<NodeJS.Timeout | null>(null);
  const pollRef = useRef<NodeJS.Timeout | null>(null);

  const handleCreatePayment = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setError("");
    setLoading(true);
    setPaymentId(null);
    setQrPayload(null);
    setExpiresAt(null);
    setStatus(null);
    setTimeLeft(0);

    // Clean up any existing timers
    if (timerRef.current) clearInterval(timerRef.current);
    if (pollRef.current) clearInterval(pollRef.current);

    const amountNum = parseFloat(amount);
    if (isNaN(amountNum) || amountNum < 10) {
      setError("Please enter a valid amount of at least 10 THB.");
      setLoading(false);
      return;
    }

    try {
      console.log("selected payment method:", paymentMethod);
      const payload = {
        amount: amountNum,
        method: paymentMethod,
      };

      const idempotencyKey = crypto.randomUUID();
      const res = await fetch("/api/payment/create", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": idempotencyKey,
        },
        body: JSON.stringify(payload),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "Failed to create payment");
      }

      if (data.redirect_url) {
        window.location.href = data.redirect_url;
        return;
      }

      setPaymentId(data.payment_id);
      if (data.qr_payload) {
        setQrPayload(data.qr_payload);
      }
      setExpiresAt(data.expires_at);
      setStatus(data.status); // PENDING
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred.");
      }
      setLoading(false);
    }
  };

  // 1. Handle countdown timer
  useEffect(() => {
    if (!expiresAt || status !== "PENDING") return;

    const expiryTime = new Date(expiresAt).getTime();
    const updateTimer = () => {
      const now = Date.now();
      const diff = Math.max(0, Math.floor((expiryTime - now) / 1000));
      setTimeLeft(diff);

      if (diff <= 0) {
        setStatus("EXPIRED");
        if (timerRef.current) clearInterval(timerRef.current);
        if (pollRef.current) clearInterval(pollRef.current);
      }
    };

    updateTimer(); // run once immediately
    timerRef.current = setInterval(updateTimer, 1000);

    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [expiresAt, status]);

  // 2. Handle status polling every 5 seconds
  useEffect(() => {
    if (!paymentId || status !== "PENDING") return;

    const pollStatus = async () => {
      try {
        const res = await fetch(`/api/payment/${paymentId}/status`);
        if (!res.ok) return;
        const data = await res.json();

        if (data.status && data.status !== "PENDING") {
          setStatus(data.status); // transitions to SUCCESS, FAILED, EXPIRED
        }
      } catch (err) {
        console.error("Polling error:", err);
      }
    };

    pollRef.current = setInterval(pollStatus, 5000);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [paymentId, status]);

  // Clean up all timers on component unmount
  useEffect(() => {
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  const handleLogout = async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
      router.push("/login");
      router.refresh();
    } catch (err) {
      console.error("Logout failed", err);
    }
  };

  // Formatter for MM:SS
  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${String(mins).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
  };

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
            <Link href="/wallet" className="text-white/60 hover:text-white flex items-center gap-1.5 py-3 transition duration-150">
              <Wallet className="w-3.5 h-3.5" />
              Wallet
            </Link>
            <Link href="/payment" className="text-white font-semibold flex items-center gap-1.5 py-3 border-b-2 border-primary">
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
          <span className="text-tagline font-semibold text-ink">Top Up Wallet</span>
          <div className="flex items-center gap-3">
            <Link href="/dashboard" className="button-secondary-pill">
              Dashboard
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
      <main className="flex-1 max-w-[480px] mx-auto px-6 py-12 w-full space-y-8 flex flex-col justify-center">
        <div className="text-center space-y-2">
          <h1 className="text-display-lg text-ink font-semibold tracking-[-0.28px]">Top Up Wallet</h1>
          <p className="text-caption-apple text-ink-muted-48">Generate a dynamic PromptPay QR code to top up instantly</p>
        </div>

        {error && (
          <div className="bg-rose-50 border border-rose-200 text-rose-700 p-4 rounded-sm text-caption-apple flex items-center gap-3 animate-pulse">
            <AlertCircle className="w-5 h-5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {!qrPayload && (
          <div className="store-utility-card space-y-6 bg-white">
            <form onSubmit={handleCreatePayment} className="space-y-6">
              <div className="space-y-2">
                <label className="text-caption-strong text-ink font-semibold">Amount (THB)</label>
                <input
                  type="number"
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                  placeholder="100.00"
                  step="0.01"
                  min="10"
                  required
                  disabled={loading}
                  className="w-full px-4 py-4 bg-canvas-parchment border border-hairline focus:border-primary focus:ring-1 focus:ring-primary rounded-pill text-display-md font-semibold font-mono text-ink placeholder-ink-muted-48 outline-none transition duration-150 text-center"
                />
              </div>

              <div className="space-y-2">
                <label className="text-caption-strong text-ink font-semibold">Payment Method</label>
                <div className="grid grid-cols-2 gap-4">
                  <button
                    type="button"
                    disabled={loading}
                    onClick={() => setPaymentMethod("promptpay")}
                    className={`configurator-option-chip ${
                      paymentMethod === "promptpay" ? "configurator-option-chip-selected" : ""
                    }`}
                  >
                    <div className="flex flex-col items-center">
                      <span className="font-semibold">PromptPay</span>
                      <span className="text-[10px] opacity-60">Instant Scan</span>
                    </div>
                  </button>
                  <button
                    type="button"
                    disabled={loading}
                    onClick={() => setPaymentMethod("card")}
                    className={`configurator-option-chip ${
                      paymentMethod === "card" ? "configurator-option-chip-selected" : ""
                    }`}
                  >
                    <div className="flex flex-col items-center">
                      <span className="font-semibold">Card</span>
                      <span className="text-[10px] opacity-60">Stripe Checkout</span>
                    </div>
                  </button>
                </div>
              </div>

              <button
                type="submit"
                disabled={loading}
                className="button-primary w-full py-4 text-body-strong font-semibold flex items-center justify-center gap-2"
              >
                {loading ? (
                  <>
                    <Loader2 className="w-5 h-5 animate-spin" />
                    Generating Session...
                  </>
                ) : (
                  <>
                    {paymentMethod === "promptpay" ? "Generate PromptPay QR" : "Pay with Card"}
                    <CreditCard className="w-5 h-5" />
                  </>
                )}
              </button>
            </form>
          </div>
        )}

        {qrPayload && (
          <div className="store-utility-card p-8 bg-white text-center space-y-6 flex flex-col items-center">
            
            {/* State PENDING */}
            {status === "PENDING" && (
              <>
                <div className="space-y-1">
                  <h2 className="text-caption-strong text-ink font-semibold">Scan to Pay</h2>
                  <p className="text-display-md font-semibold text-ink font-mono">
                    ฿{parseFloat(amount).toLocaleString("en-US", { minimumFractionDigits: 2 })}
                  </p>
                </div>

                {/* QR Container styled cleanly */}
                <div className="p-4 bg-white border border-hairline rounded-lg inline-flex">
                  <QRCodeSVG value={qrPayload} size={220} level="M" />
                </div>

                <div className="flex items-center gap-2 bg-canvas-parchment px-4 py-2 rounded-pill border border-hairline inline-flex">
                  <Clock className="w-4 h-4 text-primary" />
                  <span className="text-caption-apple font-semibold text-ink font-mono">
                    Expires in: {formatTime(timeLeft)}
                  </span>
                </div>

                <p className="text-caption-apple text-ink-muted-48 max-w-xs mx-auto">
                  Please do not close this window. Real-time bank synchronization in progress.
                </p>
              </>
            )}

            {/* State SUCCESS */}
            {status === "SUCCESS" && (
              <div className="py-6 space-y-4 flex flex-col items-center w-full">
                <div className="w-14 h-14 bg-emerald-50 rounded-full flex items-center justify-center text-emerald-600 border border-emerald-200">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <div className="space-y-1">
                  <h2 className="text-tagline font-semibold text-ink">Payment Successful ✓</h2>
                  <p className="text-caption-apple text-ink-muted-48 max-w-xs mx-auto">
                    Your deposit of ฿{parseFloat(amount).toLocaleString("en-US", { minimumFractionDigits: 2 })} has been settled.
                  </p>
                </div>
                <div className="pt-4 flex flex-col sm:flex-row gap-3 w-full justify-center">
                  <Link href="/dashboard" className="button-primary py-3 px-6 text-body-strong font-semibold">
                    Go to Dashboard
                  </Link>
                  <button
                    onClick={() => {
                      setQrPayload(null);
                      setPaymentId(null);
                      setExpiresAt(null);
                      setStatus(null);
                    }}
                    className="button-secondary-pill py-3 px-6 text-body-strong font-semibold"
                  >
                    Top Up More
                  </button>
                </div>
              </div>
            )}

            {/* State EXPIRED */}
            {status === "EXPIRED" && (
              <div className="py-6 space-y-4 flex flex-col items-center w-full">
                <div className="w-14 h-14 bg-amber-50 rounded-full flex items-center justify-center text-amber-600 border border-amber-200">
                  <Clock className="w-8 h-8" />
                </div>
                <div className="space-y-1">
                  <h2 className="text-tagline font-semibold text-ink">QR Code Expired</h2>
                  <p className="text-caption-apple text-ink-muted-48 max-w-xs mx-auto">The PromptPay session has timed out. Please request a new QR code.</p>
                </div>
                <div className="pt-4 flex flex-col sm:flex-row gap-3 w-full justify-center">
                  <button
                    onClick={handleCreatePayment}
                    className="button-primary py-3 px-6 text-body-strong font-semibold"
                  >
                    Generate New QR
                  </button>
                  <button
                    onClick={() => setQrPayload(null)}
                    className="button-secondary-pill py-3 px-6 text-body-strong font-semibold"
                  >
                    Change Amount
                  </button>
                </div>
              </div>
            )}

            {/* State FAILED */}
            {status === "FAILED" && (
              <div className="py-6 space-y-4 flex flex-col items-center w-full">
                <div className="w-14 h-14 bg-rose-50 rounded-full flex items-center justify-center text-rose-600 border border-rose-200">
                  <AlertCircle className="w-8 h-8" />
                </div>
                <div className="space-y-1">
                  <h2 className="text-tagline font-semibold text-ink">Payment Failed</h2>
                  <p className="text-caption-apple text-ink-muted-48 max-w-xs mx-auto">The gateway reported a settlement failure. Please try again.</p>
                </div>
                <div className="pt-4 flex flex-col sm:flex-row gap-3 w-full justify-center">
                  <button
                    onClick={handleCreatePayment}
                    className="button-primary py-3 px-6 text-body-strong font-semibold"
                  >
                    Try Again
                  </button>
                  <button
                    onClick={() => setQrPayload(null)}
                    className="button-secondary-pill py-3 px-6 text-body-strong font-semibold"
                  >
                    Change Amount
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
