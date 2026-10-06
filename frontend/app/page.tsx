import React from "react";
import Link from "next/link";
import { CreditCard, Shield, Zap } from "lucide-react";

export default function Home() {
  return (
    <div className="flex flex-col min-h-screen bg-canvas">
      {/* Global Nav: Row 1 */}
      <nav className="component-global-nav text-white text-nav-link relative z-50">
        <div className="max-w-7xl mx-auto px-6 w-full flex items-center justify-between">
          <div className="flex items-center gap-6">
            <span className="font-semibold tracking-tight text-white mr-4">2M-Topup</span>
            <a href="#features" className="text-white/80 hover:text-white transition-colors duration-150">Features</a>
            <a href="#security" className="text-white/80 hover:text-white transition-colors duration-150">Security</a>
            <a href="#about" className="text-white/80 hover:text-white transition-colors duration-150">Developer API</a>
          </div>
          <div className="flex items-center gap-4">
            <Link href="/login" className="text-white/80 hover:text-white transition-colors duration-150">
              Sign In
            </Link>
          </div>
        </div>
      </nav>

      {/* Sub Nav Frosted: Row 2 */}
      <nav className="component-sub-nav-frosted select-none">
        <div className="max-w-7xl mx-auto px-6 w-full flex items-center justify-between">
          <span className="text-tagline font-semibold text-ink">2M-Topup</span>
          <div className="flex items-center gap-4">
            <Link href="/login" className="button-primary">
              Get Started
            </Link>
          </div>
        </div>
      </nav>

      {/* Stack of edge-to-edge product tiles */}
      <main className="flex-1">
        
        {/* Tile 1: Hero (Light Parchment Canvas) */}
        <section className="bg-canvas-parchment py-20 px-6 text-center flex flex-col items-center overflow-hidden border-b border-hairline">
          <div className="max-w-3xl mx-auto space-y-6">
            <h1 className="text-hero-display text-ink font-semibold tracking-[-0.28px]">
              2M-Topup. Money, elevated.
            </h1>
            <p className="text-lead text-ink-muted-80 max-w-xl mx-auto">
              A fullstack wallet top-up demo with secure sign-in, Stripe test payments, and asynchronous wallet settlement.
            </p>
            <div className="flex justify-center gap-4 pt-4">
              <Link href="/register" className="button-primary px-6 py-3">
                Create Account
              </Link>
              <Link href="/login" className="button-secondary-pill px-6 py-3">
                Sign In &rarr;
              </Link>
            </div>
          </div>

          {/* Interactive Mock Product Image/Render with shadow-product */}
          <div className="mt-16 max-w-4xl w-full bg-white rounded-lg border border-hairline p-8 shadow-product relative z-10 translate-y-4">
            <div className="flex justify-between items-center pb-6 border-b border-hairline">
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-sm bg-primary/10 flex items-center justify-center text-primary font-bold">2M</div>
                <div className="text-left">
                  <p className="text-caption-strong text-ink">Wallet Dashboard</p>
                  <p className="text-fine-print text-ink-muted-48">Illustrative demo data</p>
                </div>
              </div>
              <span className="px-3 py-1 bg-emerald-50 text-emerald-700 border border-emerald-200 rounded-full text-xs font-semibold">฿1,420.50</span>
            </div>
            <div className="grid grid-cols-3 gap-6 pt-6 text-left">
              <div>
                <p className="text-fine-print text-ink-muted-48">RECENT INFLOW</p>
                <p className="text-body-strong text-emerald-600 font-mono mt-1">+฿500.00</p>
                <p className="text-micro-legal text-ink-muted-48">Example test deposit</p>
              </div>
              <div>
                <p className="text-fine-print text-ink-muted-48">PAYMENT STATUS</p>
                <p className="text-body-strong text-ink font-mono mt-1">SUCCESS</p>
                <p className="text-micro-legal text-ink-muted-48">Worker settlement</p>
              </div>
              <div>
                <p className="text-fine-print text-ink-muted-48">IDEMPOTENCY SYSTEM</p>
                <p className="text-body-strong text-primary font-mono mt-1">Verified</p>
                <p className="text-micro-legal text-ink-muted-48">Double-deposit protection</p>
              </div>
            </div>
          </div>
        </section>

        {/* Tile 2: Alternative Dark Canvas */}
        <section id="security" className="bg-surface-tile-1 py-20 px-6 text-center text-white flex flex-col items-center overflow-hidden">
          <div className="max-w-2xl mx-auto space-y-6">
            <h2 className="text-display-lg text-white font-semibold tracking-[0px]">
              Scan & Go. PromptPay support.
            </h2>
            <p className="text-lead text-body-muted max-w-lg mx-auto">
              Create a PromptPay QR through Stripe. Signed webhooks queue the result, and the worker updates your wallet while the page polls for status.
            </p>
            <div className="pt-2">
              <Link href="/login" className="text-primary-on-dark text-body-apple font-semibold hover:underline">
                Try the test payment flow &rarr;
              </Link>
            </div>
          </div>

          {/* PromptPay Mock Render with shadow-product */}
          <div className="mt-16 max-w-md w-full bg-white rounded-lg p-8 shadow-product text-ink">
            <div className="space-y-4">
              <div className="bg-[#002f6c] p-4 rounded-sm text-white font-bold flex justify-between items-center">
                <span>PromptPay QR</span>
                <span className="text-xs tracking-wider opacity-80">TEST PREVIEW</span>
              </div>
              <div className="py-8 bg-canvas-parchment rounded-sm border border-hairline inline-flex p-4 justify-center items-center w-full">
                <div className="w-48 h-48 bg-ink rounded-sm flex items-center justify-center text-white font-mono text-xs text-center p-4 select-none">
                  [ QR preview — sign in to create a test payment ]
                </div>
              </div>
              <div className="text-center space-y-1">
                <p className="text-fine-print text-ink-muted-48">TRANSACTION AMOUNT</p>
                <p className="text-display-md text-ink font-semibold">฿1,000.00</p>
              </div>
            </div>
          </div>
        </section>

        {/* Tile 3: White Canvas (Store Grid Layout) */}
        <section id="features" className="bg-canvas py-20 px-6">
          <div className="max-w-7xl mx-auto">
            <div className="text-center max-w-2xl mx-auto mb-16 space-y-4">
              <h2 className="text-display-lg text-ink font-semibold">
                Designed for high fidelity.
              </h2>
              <p className="text-lead text-ink-muted-80">
                A minimal core wrapped with powerful features to manage your online payments.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
              {/* Store Utility Card 1 */}
              <div className="store-utility-card text-left space-y-4 flex flex-col justify-between">
                <div className="w-12 h-12 bg-canvas-parchment rounded-sm flex items-center justify-center text-primary">
                  <Zap className="w-6 h-6" />
                </div>
                <div className="space-y-2">
                  <h3 className="text-body-strong text-ink">Asynchronous Settlement</h3>
                  <p className="text-caption-apple text-ink-muted-48">
                    A Redis queue connects signed payment events to the Go worker. The payment page polls until settlement completes.
                  </p>
                </div>
              </div>

              {/* Store Utility Card 2 */}
              <div className="store-utility-card text-left space-y-4 flex flex-col justify-between">
                <div className="w-12 h-12 bg-canvas-parchment rounded-sm flex items-center justify-center text-primary">
                  <CreditCard className="w-6 h-6" />
                </div>
                <div className="space-y-2">
                  <h3 className="text-body-strong text-ink">Stripe Checkout</h3>
                  <p className="text-caption-apple text-ink-muted-48">
                    Explore card checkout with Stripe test payments, signed webhooks, and transactional wallet updates.
                  </p>
                </div>
              </div>

              {/* Store Utility Card 3 */}
              <div className="store-utility-card text-left space-y-4 flex flex-col justify-between">
                <div className="w-12 h-12 bg-canvas-parchment rounded-sm flex items-center justify-center text-primary">
                  <Shield className="w-6 h-6" />
                </div>
                <div className="space-y-2">
                  <h3 className="text-body-strong text-ink">Double-Deposit Protection</h3>
                  <p className="text-caption-apple text-ink-muted-48">
                    Per-user idempotency keys and database transactions prevent duplicate credits in the tested payment flow.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>

      {/* Apple Styled Footer */}
      <footer id="about" className="bg-canvas-parchment text-ink-muted-80 text-fine-print border-t border-hairline py-16 px-6">
        <div className="max-w-7xl mx-auto space-y-12">
          {/* Dense link grid */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
            <div className="space-y-4">
              <h4 className="text-caption-strong text-ink">Test payments</h4>
              <ul className="space-y-2 text-dense-link leading-[1.8]">
                <li><Link href="/payment" className="hover:text-ink">Wallet Top Up</Link></li>
                <li><a href="#security" className="hover:text-ink">PromptPay integration</a></li>
                <li><a href="#features" className="hover:text-ink">Stripe Checkout</a></li>
              </ul>
            </div>
            <div className="space-y-4">
              <h4 className="text-caption-strong text-ink">Account</h4>
              <ul className="space-y-2 text-dense-link leading-[1.8]">
                <li><Link href="/login" className="hover:text-ink">Sign In</Link></li>
                <li><Link href="/register" className="hover:text-ink">Create Account</Link></li>
                <li><Link href="/dashboard" className="hover:text-ink">Dashboard</Link></li>
                <li><Link href="/wallet" className="hover:text-ink">Wallet Details</Link></li>
              </ul>
            </div>
            <div className="space-y-4">
              <h4 className="text-caption-strong text-ink">Project</h4>
              <ul className="space-y-2 text-dense-link leading-[1.8]">
                <li><a href="https://github.com/koo1F/2M-Topup-Portfolio#readme" className="hover:text-ink">Setup and API documentation</a></li>
                <li><a href="https://github.com/koo1F/2M-Topup-Portfolio" className="hover:text-ink">Source code</a></li>
              </ul>
            </div>
            <div className="space-y-4">
              <h4 className="text-caption-strong text-ink">Built with</h4>
              <ul className="space-y-2 text-dense-link leading-[1.8]">
                <li>Next.js and TypeScript</li>
                <li>Go and PostgreSQL</li>
                <li>Redis and Stripe</li>
              </ul>
            </div>
          </div>

          <div className="border-t border-hairline pt-8 space-y-4 text-ink-muted-48 leading-relaxed">
            <p className="text-micro-legal">
              A portfolio learning project. Use Stripe test mode for demos. The preview above is illustrative; this application is not ready to handle real money.
            </p>
            <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 text-micro-legal">
              <p>&copy; {new Date().getFullYear()} 2M-Topup. All rights reserved.</p>

            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
