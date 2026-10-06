package payment_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/2m-topup/backend/pkg/payment"
	"github.com/stripe/stripe-go/v78"
)

func TestPaymentStatusChecksOwnerBeforeCache(t *testing.T) {
	for _, userID := range []int64{7, 8} {
		t.Run(fmt.Sprint(userID), func(t *testing.T) {
			cacheRead := false
			repo := &mockRepository{getPaymentByIDFn: func(context.Context, int64) (*payment.Payment, error) {
				return &payment.Payment{ID: 1, UserID: 7, Amount: 100, Status: "SUCCESS"}, nil
			}}
			c := &mockCache{getFn: func(context.Context, string) (string, error) {
				cacheRead = true
				return `{"payment_id":"PAY000001","amount":100,"status":"SUCCESS"}`, nil
			}}
			svc := payment.NewService(repo, c, &mockQueue{}, "test", "", " ")
			result, err := svc.GetPaymentStatus(context.Background(), userID, "PAY000001")
			if userID == 8 {
				if !errors.Is(err, payment.ErrPaymentNotFound) || result != nil || cacheRead {
					t.Fatalf("non-owner accessed payment: result=%v err=%v cacheRead=%v", result, err, cacheRead)
				}
			} else if err != nil || result.Status != "SUCCESS" || !cacheRead {
				t.Fatalf("owner lookup failed: %v %v", result, err)
			}
		})
	}
}

func TestIdempotencyIsScopedAndRejectsChangedRequest(t *testing.T) {
	old := stripe.GetBackend(stripe.APIBackend)
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, old) })
	stripe.SetBackend(stripe.APIBackend, &mockStripeBackend{callFn: func(_, path, _ string, _ stripe.ParamsContainer, v stripe.LastResponseSetter) error {
		sess := v.(*stripe.CheckoutSession)
		sess.ID = "cs_test"
		sess.URL = "http://localhost/test-checkout"
		return nil
	}})
	entries := map[string]string{}
	creates := 0
	repo := &mockRepository{createPaymentFn: func(_ context.Context, userID int64, amount float64, _, key, method string, expires time.Time) (*payment.Payment, error) {
		creates++
		return &payment.Payment{ID: int64(creates), UserID: userID, Amount: amount, Method: method, IdempotencyKey: key, ExpiresAt: expires}, nil
	}}
	c := &mockCache{getFn: func(_ context.Context, key string) (string, error) {
		v, ok := entries[key]
		if !ok {
			return "", errors.New("miss")
		}
		return v, nil
	}, setFn: func(_ context.Context, key, value string, _ time.Duration) error { entries[key] = value; return nil }}
	svc := payment.NewService(repo, c, &mockQueue{}, "test", "", "")
	req := payment.CreatePaymentRequest{Amount: 100, Method: "card"}
	a, _, err := svc.CreatePayment(context.Background(), 1, req, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := svc.CreatePayment(context.Background(), 2, req, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if a.PaymentID == b.PaymentID || creates != 2 {
		t.Fatal("users shared a payment")
	}
	replay, created, err := svc.CreatePayment(context.Background(), 1, req, "same-key")
	if err != nil || created || replay.PaymentID != a.PaymentID || creates != 2 {
		t.Fatal("retry created another payment")
	}
	for _, changed := range []payment.CreatePaymentRequest{{Amount: 200, Method: "card"}, {Amount: 100, Method: "promptpay"}} {
		if _, _, err = svc.CreatePayment(context.Background(), 1, changed, "same-key"); !errors.Is(err, payment.ErrIdempotencyConflict) {
			t.Fatalf("expected conflict: %v", err)
		}
	}
	if _, ok := entries["idempotency:payment:1:same-key"]; !ok {
		t.Fatal("cache key not scoped")
	}
}

func TestIdempotencyConflictWithoutCache(t *testing.T) {
	for _, change := range []struct {
		amount float64
		method string
	}{{200, "card"}, {100, "promptpay"}} {
		repo := &mockRepository{
			createPaymentFn: func(context.Context, int64, float64, string, string, string, time.Time) (*payment.Payment, error) {
				return nil, payment.ErrDuplicateIdempotencyKey
			},
			getPaymentByIdempKeyFn: func(_ context.Context, userID int64, key string) (*payment.Payment, error) {
				if userID != 7 || key != "same-key" {
					t.Fatal("lookup not scoped")
				}
				return &payment.Payment{ID: 1, UserID: 7, Amount: 100, Method: "card"}, nil
			},
		}
		svc := payment.NewService(repo, &mockCache{}, &mockQueue{}, "test", "", "")
		if _, _, err := svc.CreatePayment(context.Background(), 7, payment.CreatePaymentRequest{Amount: change.amount, Method: change.method}, "same-key"); !errors.Is(err, payment.ErrIdempotencyConflict) {
			t.Fatalf("expected conflict: %v", err)
		}
	}
}

func TestMockWebhookDisabledByDefault(t *testing.T) {
	svc := payment.NewService(&mockRepository{}, &mockCache{}, &mockQueue{}, "known-secret", "", "")
	if err := svc.EnqueueWebhook(context.Background(), []byte(`{}`), "", "anything"); !errors.Is(err, payment.ErrMockWebhookDisabled) {
		t.Fatalf("mock webhook accepted: %v", err)
	}
}

func TestInvalidPaymentInputDoesNotInsert(t *testing.T) {
	svc := payment.NewService(&mockRepository{createPaymentFn: func(context.Context, int64, float64, string, string, string, time.Time) (*payment.Payment, error) {
		t.Fatal("invalid request inserted")
		return nil, nil
	}}, &mockCache{}, &mockQueue{}, "test", "", "")
	for _, amount := range []float64{math.NaN(), math.Inf(1), 9, 10.001, 10.000001} {
		if _, _, err := svc.CreatePayment(context.Background(), 1, payment.CreatePaymentRequest{Amount: amount}, "key"); err == nil {
			t.Fatal("invalid amount accepted")
		}
	}
	if _, _, err := svc.CreatePayment(context.Background(), 1, payment.CreatePaymentRequest{Amount: 100, Method: "other"}, "key"); !errors.Is(err, payment.ErrInvalidMethod) {
		t.Fatal(err)
	}
}

func TestCardAmountRoundsToSatang(t *testing.T) {
	old := stripe.GetBackend(stripe.APIBackend)
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, old) })
	stripe.SetBackend(stripe.APIBackend, &mockStripeBackend{callFn: func(_, _, _ string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
		checkout := params.(*stripe.CheckoutSessionParams)
		if *checkout.LineItems[0].PriceData.UnitAmount != 1029 {
			t.Fatalf("10.29 THB converted to %d satang", *checkout.LineItems[0].PriceData.UnitAmount)
		}
		session := v.(*stripe.CheckoutSession)
		session.ID = "cs_test_rounding"
		session.URL = "http://localhost/test"
		return nil
	}})
	repo := &mockRepository{createPaymentFn: func(_ context.Context, userID int64, amount float64, _, key, method string, expires time.Time) (*payment.Payment, error) {
		return &payment.Payment{ID: 1, UserID: userID, Amount: amount, IdempotencyKey: key, Method: method, ExpiresAt: expires}, nil
	}}
	svc := payment.NewService(repo, &mockCache{}, &mockQueue{}, "test", "", "")
	if _, _, err := svc.CreatePayment(context.Background(), 1, payment.CreatePaymentRequest{Amount: 10.29, Method: "card"}, "rounding"); err != nil {
		t.Fatal(err)
	}
}
