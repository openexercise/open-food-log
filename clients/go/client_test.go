package openfoodlog

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// fake answers with handler and records every request.
func fake(t *testing.T, handler http.HandlerFunc) (*Client, *[]*http.Request) {
	t.Helper()
	var requests []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL+"/", "lca_test")
	c.HTTPClient = srv.Client()
	c.NewIdempotencyKey = func() string { return "key-1" }
	return c, &requests
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestLogMealSendsTokenAndKey(t *testing.T) {
	c, reqs := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") == "again" {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusCreated, map[string]any{"meal": map[string]any{"id": "m1", "source": map[string]any{"kind": "api"}, "items": []any{}}, "needs_confirmation": []any{}})
	})
	out, err := c.LogMeal(context.Background(), MealInput{Items: []ItemInput{{Text: "a bar", Nutrients: map[string]float64{"energy_kcal": 200}}}})
	if err != nil || out.Meal.ID != "m1" || out.Replayed {
		t.Fatalf("log: %v %+v", err, out)
	}
	r := (*reqs)[0]
	if r.URL.Path != "/v1/meals" || r.Header.Get("Authorization") != "Bearer lca_test" || r.Header.Get("Idempotency-Key") != "key-1" {
		t.Errorf("request: %s %v", r.URL, r.Header)
	}
	again, err := c.LogMeal(WithIdempotencyKey(context.Background(), "again"), MealInput{})
	if err != nil || !again.Replayed {
		t.Errorf("a replay: %v %+v", err, again)
	}
	if _, err := c.GetMeal(context.Background(), "m1"); err != nil || (*reqs)[2].Header.Get("Idempotency-Key") != "" {
		t.Errorf("a GET carries a key: %v", err)
	}
}

func TestErrors(t *testing.T) {
	c, _ := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"title": "Too many", "status": 429, "code": "rate_limited", "detail": "slow down"})
	})
	_, err := c.ListMeals(context.Background(), "", "")
	var e *Error
	if !HasCode(err, CodeRateLimited) || !asError(err, &e) || e.RetryAfter != time.Minute || e.Detail != "slow down" {
		t.Errorf("err = %v", err)
	}
}

func asError(err error, target **Error) bool {
	e, ok := err.(*Error)
	*target = e
	return ok
}

func TestChangesFollowsCursors(t *testing.T) {
	c, reqs := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "" {
			writeJSON(w, http.StatusOK, map[string]any{"meals": []any{map[string]any{"id": "m1", "source": map[string]any{"kind": "api"}, "items": []any{}}}, "truncated": true, "next_cursor": "c2"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"meals": []any{}, "deleted": []any{map[string]any{"id": "m2", "deleted_at": time.Now(), "updated_at": time.Now()}}})
	})
	var meals, deleted []string
	for ch, err := range c.Changes(context.Background(), ChangesOptions{UpdatedSince: time.Unix(1_800_000_000, 0), IncludeDeleted: true}) {
		if err != nil {
			t.Fatal(err)
		}
		if ch.Meal != nil {
			meals = append(meals, ch.Meal.ID)
		} else {
			deleted = append(deleted, ch.Deleted.ID)
		}
	}
	if len(meals) != 1 || len(deleted) != 1 || len(*reqs) != 2 {
		t.Errorf("meals %v deleted %v in %d requests", meals, deleted, len(*reqs))
	}
	if q := (*reqs)[0].URL.Query(); q.Get("include_deleted") != "true" || q.Get("updated_since") == "" {
		t.Errorf("query %v", q)
	}
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"event":"meal.created"}`)
	now := time.Unix(1_800_000_000, 0)
	mac := hmac.New(sha256.New, []byte("whsec"))
	mac.Write([]byte(strconv.FormatInt(now.Unix(), 10) + "."))
	mac.Write(body)
	header := "t=" + strconv.FormatInt(now.Unix(), 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifySignature("whsec", header, body, now, 5*time.Minute) {
		t.Error("a good signature doesn't verify")
	}
	if VerifySignature("other", header, body, now, 5*time.Minute) || VerifySignature("whsec", header, body, now.Add(time.Hour), 5*time.Minute) {
		t.Error("a wrong secret or an old delivery verifies")
	}
}
