package openfoodlog

import (
	"encoding/json"
	"time"
)

// Date is a day, YYYY-MM-DD, in the user's timezone.
type Date = string

// Meal types.
const (
	Breakfast = "breakfast"
	Lunch     = "lunch"
	Dinner    = "dinner"
	Snack     = "snack"
)

// Meal is an Open Food Log v0 meal (https://openfoodlog.org/schema/v0/meal.json).
type Meal struct {
	Schema    string     `json:"$schema,omitempty"`
	ID        string     `json:"id"`
	EatenAt   *time.Time `json:"eaten_at,omitempty"`
	Date      Date       `json:"date,omitempty"`
	MealType  string     `json:"meal_type,omitempty"`
	Title     string     `json:"title,omitempty"`
	Notes     string     `json:"notes,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Source    Source     `json:"source"`
	Items     []Item     `json:"items"`
	// Estimates are JSON pointers to the values the server assumed rather than was given.
	Estimates []string `json:"estimates,omitempty"`
}

// Source is where a meal came from.
type Source struct {
	Kind       string  `json:"kind"`
	Client     string  `json:"client,omitempty"`
	ExternalID *string `json:"external_id,omitempty"`
}

// Item is one thing eaten.
type Item struct {
	ID     string   `json:"id,omitempty"`
	Text   string   `json:"text"`
	Food   FoodRef  `json:"food"`
	Amount *Amount  `json:"amount,omitempty"`
	Grams  *float64 `json:"grams,omitempty"`
	// Nutrients are what the item held, as counted when logged; a key left out is unknown.
	Nutrients map[string]float64 `json:"nutrients,omitempty"`
	Notes     string             `json:"notes,omitempty"`
}

// FoodRef is the food an item was matched to. Match is catalogue, custom, close, unmatched or
// none (an item that names no food).
type FoodRef struct {
	ID    *string `json:"id"`
	Name  string  `json:"name"`
	Match string  `json:"match"`
}

// Amount is how much, in the measure the person used: a weight (g, kg, oz, lb), a volume (ml, l,
// fl_oz), or a portion of the food (Unit "portion" and the portion's ID).
type Amount struct {
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Portion string  `json:"portion,omitempty"`
}

// MealInput is a meal to log. Without EatenAt or Date the meal is now.
type MealInput struct {
	EatenAt  *time.Time `json:"eaten_at,omitempty"`
	Date     Date       `json:"date,omitempty"`
	MealType string     `json:"meal_type,omitempty"`
	Title    string     `json:"title,omitempty"`
	Notes    string     `json:"notes,omitempty"`
	// ExternalID is the meal's ID in the app it came from: sending it again replays the stored
	// meal (Written.Replayed), and one the person deleted is CodeNotFound and stays deleted.
	ExternalID string      `json:"external_id,omitempty"`
	Items      []ItemInput `json:"items"`
}

// ItemInput is one thing eaten: the person's words, and the food when known (else the words are
// matched), how much (else the food's default portion, listed in estimates), or, for an item
// that names no food, the nutrients the person gave.
type ItemInput struct {
	Text      string             `json:"text"`
	FoodID    string             `json:"food_id,omitempty"`
	Amount    *Amount            `json:"amount,omitempty"`
	Nutrients map[string]float64 `json:"nutrients,omitempty"`
	Notes     string             `json:"notes,omitempty"`
}

// MealPatch changes a meal; fields left nil stay. To clear one, send null through Do.
type MealPatch struct {
	EatenAt  *time.Time `json:"eaten_at,omitempty"`
	Date     *Date      `json:"date,omitempty"`
	MealType *string    `json:"meal_type,omitempty"`
	Title    *string    `json:"title,omitempty"`
	Notes    *string    `json:"notes,omitempty"`
}

// ItemPatch confirms an item's food (FoodID, which a server may learn from), weighs it again
// (Amount), or changes its notes or, for an item that names no food, its nutrients.
type ItemPatch struct {
	FoodID    *string            `json:"food_id,omitempty"`
	Amount    *Amount            `json:"amount,omitempty"`
	Nutrients map[string]float64 `json:"nutrients,omitempty"`
	Notes     *string            `json:"notes,omitempty"`
}

// Written is a meal write's or read's result: the meal, the items whose food to confirm with the
// user, and the meal's totals, which a server leaves out when the person asked not to see numbers.
type Written struct {
	Meal              Meal      `json:"meal"`
	NeedsConfirmation []Confirm `json:"needs_confirmation"`
	Totals            *Totals   `json:"totals,omitempty"`
	// Replayed is true when the response repeats an earlier request with the same
	// Idempotency-Key or external_id; nothing new was stored.
	Replayed bool `json:"-"`
}

// Confirm is an item whose food isn't sure, with foods it could be.
type Confirm struct {
	ItemID     string      `json:"item_id"`
	Text       string      `json:"text"`
	Candidates []FoodMatch `json:"candidates"`
}

// Totals add up the items' nutrients. IsComplete is false when an item has none yet (its food
// isn't confirmed), so they're a lower bound.
type Totals struct {
	Nutrients  map[string]float64 `json:"nutrients"`
	IsComplete bool               `json:"is_complete"`
}

// Day is one day's totals.
type Day struct {
	Date   Date   `json:"date"`
	Totals Totals `json:"totals"`
}

// MealList is the meals on some days, or one page of the change feed.
type MealList struct {
	Meals []Meal `json:"meals"`
	Days  []Day  `json:"days,omitempty"`
	// The change feed's: the tombstones, and whether more pages follow.
	Deleted    []Tombstone `json:"deleted,omitempty"`
	Truncated  bool        `json:"truncated,omitempty"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

// Tombstone is a deleted meal in the change feed: enough to remove a mirror's copy.
type Tombstone struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Deleted is a delete's result: when, and until when the server can restore it.
type Deleted struct {
	DeletedAt    time.Time `json:"deleted_at"`
	RestoreUntil time.Time `json:"restore_until"`
}

// Food is an Open Food Log v0 food (https://openfoodlog.org/schema/v0/food.json). Raw keeps the
// whole document, for fields this type doesn't name.
type Food struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Kind             string             `json:"kind"`
	NutrientsPer100g map[string]float64 `json:"nutrients_per_100g"`
	DensityGPerML    *float64           `json:"density_g_per_ml,omitempty"`
	Portions         []Portion          `json:"portions,omitempty"`
	Aliases          []Alias            `json:"aliases,omitempty"`
	Raw              json.RawMessage    `json:"-"`
}

// UnmarshalJSON keeps the whole document in Raw.
func (f *Food) UnmarshalJSON(b []byte) error {
	type plain Food
	if err := json.Unmarshal(b, (*plain)(f)); err != nil {
		return err
	}
	f.Raw = append(json.RawMessage(nil), b...)
	return nil
}

// Portion is a named measure of a food and what one weighs.
type Portion struct {
	ID        string  `json:"id,omitempty"`
	Label     string  `json:"label"`
	Grams     float64 `json:"grams"`
	IsDefault bool    `json:"is_default,omitempty"`
}

// Alias is another name people call a food.
type Alias struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

// FoodMatch is a search result: a food, and how well the query matched it from 0 to 1.
type FoodMatch struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Kind             string             `json:"kind"`
	NutrientsPer100g map[string]float64 `json:"nutrients_per_100g"`
	Score            float64            `json:"score"`
}

// FoodInput makes one of the user's own foods: NutrientsPer100g, or a label's
// NutrientsPerServing with the Serving it describes (which becomes the default portion).
// energy_kcal is required.
type FoodInput struct {
	Name                string             `json:"name"`
	NutrientsPer100g    map[string]float64 `json:"nutrients_per_100g,omitempty"`
	NutrientsPerServing map[string]float64 `json:"nutrients_per_serving,omitempty"`
	Serving             *Portion           `json:"serving,omitempty"`
	DensityGPerML       *float64           `json:"density_g_per_ml,omitempty"`
	Portions            []Portion          `json:"portions,omitempty"`
}

// Webhook events of the core.
const (
	MealCreated = "meal.created"
	MealUpdated = "meal.updated"
	MealDeleted = "meal.deleted"
)

// WebhookInput subscribes to events at an https URL on a public host.
type WebhookInput struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// Webhook is a subscription. Secret is only in the creation response.
type Webhook struct {
	ID                  string     `json:"id"`
	URL                 string     `json:"url"`
	Events              []string   `json:"events"`
	CreatedAt           time.Time  `json:"created_at"`
	PausedAt            *time.Time `json:"paused_at,omitempty"`
	LastDeliveredAt     *time.Time `json:"last_delivered_at,omitempty"`
	LastStatus          int        `json:"last_status,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	Secret              string     `json:"secret,omitempty"`
}

// Delivery is a webhook delivery's body: what changed, never the meal itself; read it after.
type Delivery struct {
	Event          string    `json:"event"`
	MealID         string    `json:"meal_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	SubscriptionID string    `json:"subscription_id"`
	DeliveryID     string    `json:"delivery_id"`
}

// Log is an Open Food Log v0 log (https://openfoodlog.org/schema/v0/log.json): the export.
type Log struct {
	ExportedAt   time.Time     `json:"exported_at"`
	Meals        []Meal        `json:"meals"`
	Foods        []Food        `json:"foods,omitempty"`
	DeletedMeals []DeletedMeal `json:"deleted_meals,omitempty"`
}

// DeletedMeal is a meal the person deleted, in the export.
type DeletedMeal struct {
	DeletedAt time.Time `json:"deleted_at"`
	Meal      Meal      `json:"meal"`
}
