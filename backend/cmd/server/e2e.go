//go:build e2e

// This fixture is compiled only with -tags=e2e. It is absent from production
// builds and uses the production API, repository, queue and worker unchanged.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v78"
)

type testSession struct {
	ID, PaymentID, SuccessURL string
	Amount                    int64
}

var testSessions = struct {
	sync.Mutex
	entries map[string]testSession
}{entries: map[string]testSession{}}

func init() {
	if os.Getenv("E2E_FIXTURE") != "1" {
		log.Fatal("e2e build requires E2E_FIXTURE=1")
	}
	databaseURL, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil || (databaseURL.Hostname() != "127.0.0.1" && databaseURL.Hostname() != "localhost") || databaseURL.Path != "/payment_test" {
		log.Fatal("e2e fixture only supports a local test database")
	}
	// Redirect the SDK itself; production payment creation still calls Stripe's SDK.
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String("http://127.0.0.1:19090")}))
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/checkout/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "form", 400)
			return
		}
		id := r.Form.Get("metadata[payment_id]")
		amount, _ := strconv.ParseInt(r.Form.Get("line_items[0][price_data][unit_amount]"), 10, 64)
		s := testSession{ID: "cs_test_" + id, PaymentID: id, Amount: amount, SuccessURL: r.Form.Get("success_url")}
		testSessions.Lock()
		testSessions.entries[s.ID] = s
		testSessions.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.ID, "object": "checkout.session", "url": "http://127.0.0.1:19090/checkout/" + s.ID, "metadata": map[string]string{"payment_id": id}})
	})
	mux.HandleFunc("/v1/checkout/sessions/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/checkout/sessions/")
		s, ok := lookupTestSession(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": s.ID, "object": "checkout.session", "url": "http://127.0.0.1:19090/checkout/" + s.ID})
	})
	page := template.Must(template.New("checkout").Parse(`<!doctype html><html lang="en"><head><title>Local test checkout</title></head><body><h1>Local test checkout</h1><p>Provider fixture only. No real payment is made.</p><p>Payment {{.PaymentID}}</p><form method="post"><button>Complete test payment</button></form></body></html>`))
	mux.HandleFunc("/checkout/", func(w http.ResponseWriter, r *http.Request) {
		s, ok := lookupTestSession(strings.TrimPrefix(r.URL.Path, "/checkout/"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if err := sendTestEvent(s, "evt_e2e_"+s.PaymentID); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			http.Redirect(w, r, s.SuccessURL, http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Execute(w, s)
	})
	mux.HandleFunc("/replay/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		s, ok := lookupTestSession(strings.TrimPrefix(r.URL.Path, "/replay/"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		eventID := "evt_e2e_" + s.PaymentID
		if r.URL.Query().Get("new_event") == "1" {
			eventID += "_retry"
		}
		if err := sendTestEvent(s, eventID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	go func() {
		log.Println("Local provider fixture on 127.0.0.1:19090")
		log.Fatal(http.ListenAndServe("127.0.0.1:19090", mux))
	}()
}
func lookupTestSession(id string) (testSession, bool) {
	testSessions.Lock()
	defer testSessions.Unlock()
	s, ok := testSessions.entries[id]
	return s, ok
}
func sendTestEvent(s testSession, eventID string) error {
	payload, err := json.Marshal(map[string]any{"id": eventID, "object": "event", "api_version": stripe.APIVersion, "type": "checkout.session.completed", "created": time.Now().Unix(), "data": map[string]any{"object": map[string]any{"id": s.ID, "object": "checkout.session", "payment_status": "paid", "amount_total": s.Amount, "currency": "thb", "metadata": map[string]string{"payment_id": s.PaymentID}}}})
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(os.Getenv("STRIPE_WEBHOOK_SECRET")))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(payload)
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+os.Getenv("PORT")+"/api/webhooks/stripe", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t="+timestamp+",v1="+hex.EncodeToString(mac.Sum(nil)))
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned %d", res.StatusCode)
	}
	return nil
}
