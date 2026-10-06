"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LogIn, Mail, Lock, Loader2, AlertCircle } from "lucide-react";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    if (!email || !password) {
      setError("Email and password are required.");
      setLoading(false);
      return;
    }

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Login failed");
      }

      router.push("/dashboard");
      router.refresh();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="min-h-screen flex items-center justify-center bg-canvas-parchment px-4 select-none relative">
      {/* Sleek, quiet background header linking back home */}
      <div className="absolute top-8 left-8">
        <Link href="/" className="text-tagline font-semibold text-ink tracking-tight select-none">
          2M-Topup
        </Link>
      </div>

      <div className="w-full max-w-[440px] z-10">
        <div className="store-utility-card space-y-8 p-8 md:p-10 bg-white">
          <div className="text-center space-y-2">
            <div className="inline-flex p-3 bg-canvas-parchment rounded-sm text-ink mb-2 border border-hairline">
              <LogIn className="w-6 h-6 text-ink" />
            </div>
            <h1 className="text-display-md text-ink font-semibold tracking-[-0.374px]">Welcome Back</h1>
            <p className="text-caption-apple text-ink-muted-48">Sign in to manage your premium wallet</p>
          </div>

          {error && (
            <div className="flex items-center gap-3 bg-rose-50 border border-rose-200 text-rose-700 p-4 rounded-sm text-caption-apple">
              <AlertCircle className="w-5 h-5 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-2">
              <label className="text-caption-strong text-ink font-semibold">Email Address</label>
              <div className="relative">
                <Mail className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-muted-48" />
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="name@example.com"
                  required
                  className="w-full pl-11 pr-4 py-3 bg-white border border-hairline focus:border-primary focus:ring-1 focus:ring-primary rounded-pill text-body-apple text-ink placeholder-ink-muted-48 outline-none transition duration-150"
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className="text-caption-strong text-ink font-semibold">Password</label>
              <div className="relative">
                <Lock className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-muted-48" />
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  required
                  className="w-full pl-11 pr-4 py-3 bg-white border border-hairline focus:border-primary focus:ring-1 focus:ring-primary rounded-pill text-body-apple text-ink placeholder-ink-muted-48 outline-none transition duration-150"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="button-primary w-full py-3.5 text-body-apple font-semibold flex items-center justify-center gap-2"
            >
              {loading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Signing In...
                </>
              ) : (
                <>
                  Sign In
                  <LogIn className="w-4 h-4" />
                </>
              )}
            </button>
          </form>

          <p className="text-center text-caption-apple text-ink-muted-48 pt-2">
            Don&apos;t have an account?{" "}
            <Link href="/register" className="text-primary hover:underline font-semibold transition duration-150">
              Create Account
            </Link>
          </p>
        </div>
      </div>
    </main>
  );
}
