package conformance

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"slices"
)

// A person's own food with a barcode (v0.3; spec sections 3 and 5): made through createFood with a
// gtin, it answers that gtin: for its person, under its own ID.

// ownFoodGTIN is a 13-digit code with a valid check digit whose prefix 2 GS1 keeps for restricted
// circulation (in-store codes), so it never names a product a database knows; random per run.
func ownFoodGTIN() string {
	digits := make([]byte, 12)
	digits[0] = '2'
	for i := 1; i < 12; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		digits[i] = byte('0' + n.Int64())
	}
	return string(digits) + string('0'+checkDigit(string(digits)))
}

// checkDigit is the GS1 check digit of a code without it: weights 3 and 1 from the right.
func checkDigit(code string) byte {
	sum := 0
	for i := range len(code) {
		d := int(code[len(code)-1-i] - '0')
		if i%2 == 0 {
			d *= 3
		}
		sum += d
	}
	return byte((10 - sum%10) % 10)
}

// ownFood: POST /v1/foods with a gtin makes one of the person's own foods (201) that carries the
// code as 14 digits; GET /v1/foods/gtin:… then answers with that food, by its own ID, as a valid
// OFL food. The core has no way to delete a food, so the food stays; cleanUpOwnFood asks a server
// that offers DELETE /v1/foods/{id} to remove it.
func (r *runner) ownFood(ctx context.Context) error {
	gtin := ownFoodGTIN()
	full := "0" + gtin
	in := map[string]any{
		"name": "ofl conformance food " + r.run, "brand": "ofl conformance", "gtin": gtin,
		"nutrients_per_100g": map[string]any{"energy_kcal": 250, "protein_g": 10},
	}
	res, err := r.send(ctx, http.MethodPost, "/v1/foods", in, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST /v1/foods with a gtin", http.StatusCreated)
	}
	if err := res.isJSON(); err != nil {
		return err
	}
	var made struct {
		ID   string   `json:"id"`
		Kind string   `json:"kind"`
		GTIN []string `json:"gtin"`
	}
	if err := json.Unmarshal(res.body, &made); err != nil || made.ID == "" {
		return fmt.Errorf("POST /v1/foods: no food id in %s", clip(res.body))
	}
	r.foodID = made.ID
	if made.Kind != "custom" {
		return fmt.Errorf("the food made is kind %q, want custom", made.Kind)
	}
	if !slices.Contains(made.GTIN, full) {
		return fmt.Errorf("the food made has gtin %v, want it to hold %s (14 digits)", made.GTIN, full)
	}

	path := "/v1/foods/" + url.PathEscape("gtin:"+gtin)
	res, err = r.send(ctx, http.MethodGet, path, nil, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK {
		return res.unexpected("GET "+path+" for the person's own food", http.StatusOK)
	}
	if err := res.isJSON(); err != nil {
		return err
	}
	if p := r.v.ValidateAs("food", res.body); len(p) > 0 {
		return fmt.Errorf("GET %s isn't a valid OFL food: %s", path, p[0])
	}
	var found struct {
		ID   string   `json:"id"`
		GTIN []string `json:"gtin"`
	}
	_ = json.Unmarshal(res.body, &found)
	if found.ID != made.ID {
		return fmt.Errorf("GET %s answered %q, want the person's own food %s", path, found.ID, made.ID)
	}
	if !slices.Contains(found.GTIN, full) {
		return fmt.Errorf("GET %s: gtin %v, want it to hold %s", path, found.GTIN, full)
	}
	return nil
}

// cleanUpOwnFood removes the food where the server lets it; a core server may not.
func (r *runner) cleanUpOwnFood(ctx context.Context) {
	if r.foodID != "" {
		_, _ = r.send(context.WithoutCancel(ctx), http.MethodDelete, "/v1/foods/"+url.PathEscape(r.foodID), nil, nil, true)
	}
}
