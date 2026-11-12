package conformance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader is the header a webhook delivery is signed in. The scheme is Open Workout's
// (OWS-Signature), so a server of both signs both the same way.
const SignatureHeader = "OFL-Signature"

// Sign computes a delivery's OFL-Signature: "t=<unix seconds>,v1=<hex HMAC-SHA256>", the HMAC
// keyed with the subscription's secret over "<t>.<body>".
func Sign(secret string, t time.Time, body []byte) string {
	ts := strconv.FormatInt(t.Unix(), 10)
	return "t=" + ts + ",v1=" + mac(secret, ts, body)
}

func mac(secret, ts string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(ts))
	m.Write([]byte("."))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// VerifySignature checks a webhook delivery as a receiver should: the OFL-Signature header's v1
// is the HMAC of the body sent at its t, compared in constant time, and t is within tolerance of
// now, so a captured delivery can't be replayed later.
func VerifySignature(secret, header string, body []byte, now time.Time, tolerance time.Duration) error {
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || len(sigs) == 0 {
		return errors.New("the signature header needs t=<unix seconds> and v1=<hex HMAC>")
	}
	if d := now.Sub(time.Unix(t, 0)); d > tolerance || d < -tolerance {
		return fmt.Errorf("signed at %s, more than %s from now", time.Unix(t, 0).UTC().Format(time.RFC3339), tolerance)
	}
	want := mac(secret, ts, body)
	for _, s := range sigs {
		if hmac.Equal([]byte(s), []byte(want)) {
			return nil
		}
	}
	return errors.New("no v1 signature matches the body")
}
