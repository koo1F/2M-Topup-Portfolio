package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/2m-topup/backend/pkg/cache"
	"github.com/2m-topup/backend/pkg/config"
	"github.com/2m-topup/backend/pkg/database"
	apimw "github.com/2m-topup/backend/pkg/middleware"
	"github.com/2m-topup/backend/pkg/payment"
	"github.com/2m-topup/backend/pkg/queue"
	"github.com/2m-topup/backend/pkg/user"
	"github.com/2m-topup/backend/pkg/wallet"
	"github.com/stripe/stripe-go/v78"
)

func main() {
	cfg := config.Load()

	// Required startup logs
	log.Println("SERVER STARTING")
	log.Printf("PORT=%s", cfg.Port)
	log.Println("HEALTH ROUTE REGISTERED")

	// Initialize Stripe
	stripe.Key = cfg.StripeSecretKey
	if cfg.StripeSecretKey == "" {
		log.Println("[WARNING] STRIPE_SECRET_KEY is not set! Stripe payments will fail.")
	}

	// Readiness flags
	var (
		dbReady    int32
		redisReady int32
	)

	// ── Postgres (Open Pool Instantly) ────────────────────────────────────────
	log.Printf("[PostgreSQL] Initializing connection pool (URL target host resolution)")
	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[PostgreSQL] Failed to initialize connection pool: %v", err)
	}
	defer db.Close()

	// Asynchronously Ping Postgres and Run Migrations
	go func() {
		for {
			log.Println("[PostgreSQL] Attempting to connect...")
			if err := db.Ping(); err == nil {
				log.Println("[PostgreSQL] Connection successful")
				log.Println("[PostgreSQL] Running database migrations...")
				if err := database.RunMigrations(db); err != nil {
					log.Printf("[PostgreSQL] Migrations failed: %v, retrying in 3s...", err)
					time.Sleep(3 * time.Second)
					continue
				}
				log.Println("[PostgreSQL] Database migrations completed successfully")
				atomic.StoreInt32(&dbReady, 1)
				log.Println("[Readiness] PostgreSQL status changed to READY")
				break
			} else {
				log.Printf("[PostgreSQL] Connection failed (retrying in 3s): %v", err)
				time.Sleep(3 * time.Second)
			}
		}
	}()

	// ── Redis Client (Instantly) ──────────────────────────────────────────────
	redisClient := database.NewRedisClient(cfg.RedisURL)
	redisCache := cache.NewRedisCache(redisClient)

	// Asynchronously Ping Redis
	go func() {
		ctx := context.Background()
		for {
			log.Println("[Redis] Attempting to connect...")
			if err := redisClient.Ping(ctx).Err(); err == nil {
				atomic.StoreInt32(&redisReady, 1)
				log.Println("[Redis] Connection successful")
				log.Println("[Readiness] Redis status changed to READY")
				break
			} else {
				log.Printf("[Redis] Connection failed (retrying in 3s): %v", err)
				time.Sleep(3 * time.Second)
			}
		}
	}()

	// ── Wire dependencies ─────────────────────────────────────────────────────
	userRepo := user.NewRepository(db)
	userSvc := user.NewService(userRepo, cfg.JWTSecret)
	userHandler := user.NewHandler(userSvc)

	walletRepo := wallet.NewRepository(db)
	walletSvc := wallet.NewService(walletRepo, redisCache)
	walletHandler := wallet.NewHandler(walletSvc)

	paymentQueue := queue.NewRedisQueue(redisClient)
	limiter := apimw.NewLimiter(redisClient)

	paymentRepo := payment.NewRepository(db)
	stripeWebhookSecret := cfg.StripeWebhookSecret
	if stripeWebhookSecret == "" {
		stripeWebhookSecret = cfg.WebhookSecret
	}
	paymentSvc := payment.NewService(paymentRepo, redisCache, paymentQueue, stripeWebhookSecret, cfg.StripeSuccessURL, cfg.StripeCancelURL)
	paymentHandler := payment.NewHandler(paymentSvc)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	// Apply a default request timeout middleware to prevent handlers from hanging indefinitely
	r.Use(middleware.Timeout(10 * time.Second))

	// Top-level root and health check routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("HIT / ROOT ROUTE")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	})

	r.Route("/api", func(r chi.Router) {
		// Public endpoints
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"ok"}`)
		})

		r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
			isDBReady := atomic.LoadInt32(&dbReady) == 1
			isRedisReady := atomic.LoadInt32(&redisReady) == 1

			w.Header().Set("Content-Type", "application/json")
			if !isDBReady || !isRedisReady {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(w, `{"status":"not_ready","postgres":%t,"redis":%t}`, isDBReady, isRedisReady)
				return
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ready","postgres":%t,"redis":%t}`, isDBReady, isRedisReady)
		})

		r.Route("/auth", func(r chi.Router) {
			r.Use(checkDBReady(&dbReady)) // Requires Postgres
			r.Post("/register", userHandler.Register)
			r.With(limiter.RateLimit("login", 500, time.Minute, apimw.IPIdentifier)).
				Post("/login", userHandler.Login)
		})

		// Webhook — no JWT, but HMAC-verified
		r.With(checkDBReady(&dbReady), checkRedisReady(&redisReady)). // Requires both
			Post("/webhooks/stripe", paymentHandler.Webhook)

		// Protected endpoints — require valid JWT
		r.Group(func(r chi.Router) {
			r.Use(apimw.Auth(cfg.JWTSecret))
			r.Use(checkDBReady(&dbReady), checkRedisReady(&redisReady)) // Requires both

			// Wallet
			r.Get("/wallet", walletHandler.GetBalance)
			r.Get("/wallet/transactions", walletHandler.GetTransactions)

			// Payment
			r.With(limiter.RateLimit("payment:create", cfg.PaymentRateLimit, time.Minute, apimw.UserIdentifier)).
				Post("/payment/create", paymentHandler.CreatePayment)
			r.Get("/payment/{id}/status", paymentHandler.GetStatus)
		})
	})

	// Log all routes at startup
	log.Println("=== ROUTES REGISTERED ===")
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("%s %s", method, route)
		return nil
	}
	if err := chi.Walk(r, walkFunc); err != nil {
		log.Printf("Logging routes failed: %s", err.Error())
	}

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	log.Printf("API Server started successfully. Listening on http://%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// checkDBReady is a middleware that returns 503 Service Unavailable if PostgreSQL database is not ready.
func checkDBReady(dbReady *int32) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.LoadInt32(dbReady) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"error":"database not ready"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// checkRedisReady is a middleware that returns 503 Service Unavailable if Redis cache is not ready.
func checkRedisReady(redisReady *int32) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.LoadInt32(redisReady) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"error":"cache not ready"}`)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
