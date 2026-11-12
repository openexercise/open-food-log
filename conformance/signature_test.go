package conformance

import (
	"strings"
	"testing"
	"time"
)

func TestSignature(t *testing.T) {
	at := time.Unix(1_800_000_000, 0)
	body := []byte(`{"event":"meal.created","meal_id":"m1"}`)
	header := Sign("whsec", at, body)
	if !strings.HasPrefix(header, "t=1800000000,v1=") {
		t.Fatalf("header %q", header)
	}
	if err := VerifySignature("whsec", header, body, at.Add(time.Minute), 5*time.Minute); err != nil {
		t.Errorf("a fresh delivery: %v", err)
	}
	for name, err := range map[string]error{
		"another secret": VerifySignature("other", header, body, at, 5*time.Minute),
		"another body":   VerifySignature("whsec", header, []byte(`{}`), at, 5*time.Minute),
		"too old":        VerifySignature("whsec", header, body, at.Add(10*time.Minute), 5*time.Minute),
		"no header":      VerifySignature("whsec", "", body, at, 5*time.Minute),
	} {
		if err == nil {
			t.Errorf("%s: verified", name)
		}
	}
	// A rotated secret's second v1 still verifies.
	if err := VerifySignature("whsec", "t=1800000000,v1=00,"+strings.Split(header, ",")[1], body, at, time.Minute); err != nil {
		t.Errorf("a second v1: %v", err)
	}
}
