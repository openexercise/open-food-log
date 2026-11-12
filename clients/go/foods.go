package openfoodlog

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SearchFoods finds foods by name or alias, best first, from the server's catalogue and the
// user's own. limit 0 is the server's default.
func (c *Client) SearchFoods(ctx context.Context, query string, limit int) ([]FoodMatch, error) {
	q := url.Values{"query": {query}}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out struct {
		Foods []FoodMatch `json:"foods"`
	}
	return out.Foods, c.Do(ctx, http.MethodGet, "/v1/foods", q, nil, &out)
}

// GetFood reads one food, with its portions; a retired catalogue food is still returned.
func (c *Client) GetFood(ctx context.Context, id string) (*Food, error) {
	var out Food
	return &out, c.Do(ctx, http.MethodGet, "/v1/foods/"+pathID(id), nil, nil, &out)
}

// CreateFood makes one of the user's own foods, or returns theirs by that name unchanged.
func (c *Client) CreateFood(ctx context.Context, in FoodInput) (*Food, error) {
	var out Food
	return &out, c.Do(ctx, http.MethodPost, "/v1/foods", nil, in, &out)
}
