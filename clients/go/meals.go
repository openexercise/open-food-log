package openfoodlog

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ListMeals is the user's meals on the days from..to (inclusive; both default to today), earliest
// first, with each day's totals unless the person asked not to see numbers.
func (c *Client) ListMeals(ctx context.Context, from, to Date) (*MealList, error) {
	q := url.Values{}
	setIf(q, "from", from)
	setIf(q, "to", to)
	var out MealList
	return &out, c.Do(ctx, http.MethodGet, "/v1/meals", q, nil, &out)
}

// ChangesOptions select the change feed: every meal whose stored record changed at or after
// UpdatedSince, oldest change first, and with IncludeDeleted the meals deleted since then as
// tombstones.
type ChangesOptions struct {
	UpdatedSince   time.Time
	IncludeDeleted bool
	// Limit is meals per page, 1 to 100; default 20.
	Limit  int
	Cursor string
}

func (o ChangesOptions) query() url.Values {
	q := url.Values{"updated_since": {o.UpdatedSince.UTC().Format(time.RFC3339Nano)}}
	if o.IncludeDeleted {
		q.Set("include_deleted", "true")
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	setIf(q, "cursor", o.Cursor)
	return q
}

// ListChanges is one page of the change feed.
func (c *Client) ListChanges(ctx context.Context, opts ChangesOptions) (*MealList, error) {
	var out MealList
	return &out, c.Do(ctx, http.MethodGet, "/v1/meals", opts.query(), nil, &out)
}

// Change is one entry of the change feed: a meal as it is now, or a tombstone for one deleted.
type Change struct {
	Meal    *Meal
	Deleted *Tombstone
}

// Changes is every change since opts.UpdatedSince, following NextCursor page by page. Keep the
// latest UpdatedAt seen and pass it back as the next UpdatedSince. A failed page ends the
// sequence with its error.
func (c *Client) Changes(ctx context.Context, opts ChangesOptions) iter.Seq2[Change, error] {
	return func(yield func(Change, error) bool) {
		for {
			page, err := c.ListChanges(ctx, opts)
			if err != nil {
				yield(Change{}, err)
				return
			}
			for i := range page.Meals {
				if !yield(Change{Meal: &page.Meals[i]}, nil) {
					return
				}
			}
			for i := range page.Deleted {
				if !yield(Change{Deleted: &page.Deleted[i]}, nil) {
					return
				}
			}
			if !page.Truncated || page.NextCursor == "" {
				return
			}
			opts.Cursor = page.NextCursor
		}
	}
}

// LogMeal stores a meal. Items whose food isn't sure come back in NeedsConfirmation: ask the user,
// then confirm with UpdateItem. A repeated Idempotency-Key or external_id returns the stored meal
// with Replayed set.
func (c *Client) LogMeal(ctx context.Context, in MealInput) (*Written, error) {
	var out Written
	h, err := c.do(ctx, http.MethodPost, "/v1/meals", nil, in, &out)
	if err != nil {
		return nil, err
	}
	out.Replayed = h.Get("Idempotent-Replayed") == "true"
	return &out, nil
}

// GetMeal reads one meal.
func (c *Client) GetMeal(ctx context.Context, id string) (*Written, error) {
	var out Written
	return &out, c.Do(ctx, http.MethodGet, "/v1/meals/"+pathID(id), nil, nil, &out)
}

// UpdateMeal changes a meal's time, day, type, title or notes.
func (c *Client) UpdateMeal(ctx context.Context, id string, patch MealPatch) (*Written, error) {
	var out Written
	return &out, c.Do(ctx, http.MethodPatch, "/v1/meals/"+pathID(id), nil, patch, &out)
}

// DeleteMeal deletes a meal, which RestoreMeal can bring back for a while.
func (c *Client) DeleteMeal(ctx context.Context, id string) (*Deleted, error) {
	var out Deleted
	return &out, c.Do(ctx, http.MethodDelete, "/v1/meals/"+pathID(id), nil, nil, &out)
}

// RestoreMeal undoes a recent delete; after the server's window it's CodeUndoUnavailable.
func (c *Client) RestoreMeal(ctx context.Context, id string) (*Written, error) {
	var out Written
	return &out, c.Do(ctx, http.MethodPost, "/v1/meals/"+pathID(id)+"/restore", nil, nil, &out)
}

// AddItems adds items to a meal.
func (c *Client) AddItems(ctx context.Context, mealID string, items []ItemInput) (*Written, error) {
	var out Written
	return &out, c.Do(ctx, http.MethodPost, "/v1/meals/"+pathID(mealID)+"/items", nil, map[string]any{"items": items}, &out)
}

// UpdateItem confirms an item's food, weighs it again, or changes its notes; the result is the
// item's meal.
func (c *Client) UpdateItem(ctx context.Context, itemID string, patch ItemPatch) (*Written, error) {
	var out Written
	return &out, c.Do(ctx, http.MethodPatch, "/v1/meal-items/"+pathID(itemID), nil, patch, &out)
}

// DeleteItem removes an item from its meal; the result is the meal.
func (c *Client) DeleteItem(ctx context.Context, itemID string) (*Written, error) {
	var out Written
	return &out, c.Do(ctx, http.MethodDelete, "/v1/meal-items/"+pathID(itemID), nil, nil, &out)
}

// Export downloads the whole food log as an Open Food Log log.
func (c *Client) Export(ctx context.Context) (*Log, error) {
	var out Log
	return &out, c.Do(ctx, http.MethodGet, "/v1/export", url.Values{"format": {"json"}}, nil, &out)
}
