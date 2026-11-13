package openfoodlog

import (
	"context"
	"net/http"
	"time"
)

// Recipe is an Open Food Log recipe: a food made of other foods (section 8). Its ID is a food ID
// too: a meal item logs it with FoodID and an Amount in servings or grams.
type Recipe struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name"`
	Notes               string             `json:"notes,omitempty"`
	Instructions        string             `json:"instructions,omitempty"`
	Lines               []Line             `json:"lines"`
	Servings            float64            `json:"servings"`
	YieldG              *float64           `json:"yield_g,omitempty"`
	CookingYieldFactor  *float64           `json:"cooking_yield_factor,omitempty"`
	NutrientsPer100g    map[string]float64 `json:"nutrients_per_100g"`
	NutrientsPerServing map[string]float64 `json:"nutrients_per_serving"`
	Source              Source             `json:"source"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

// Line is a recipe's ingredient or a saved meal's item: a meal item without an ID.
type Line struct {
	Text      string             `json:"text"`
	Food      FoodRef            `json:"food"`
	Amount    *Amount            `json:"amount,omitempty"`
	Share     *float64           `json:"share,omitempty"`
	Grams     *float64           `json:"grams,omitempty"`
	Nutrients map[string]float64 `json:"nutrients,omitempty"`
	Notes     string             `json:"notes,omitempty"`
}

// RecipeInput makes or replaces a recipe. Every line must name its food surely (send FoodID) or
// give its own nutrients, and have an amount that can be weighed.
type RecipeInput struct {
	Name               string      `json:"name"`
	Lines              []ItemInput `json:"lines"`
	Servings           float64     `json:"servings"`
	YieldG             *float64    `json:"yield_g,omitempty"`
	CookingYieldFactor *float64    `json:"cooking_yield_factor,omitempty"`
	Instructions       string      `json:"instructions,omitempty"`
	Notes              string      `json:"notes,omitempty"`
}

// SavedMeal is an Open Food Log v0.2 saved meal: items the person logs together again.
type SavedMeal struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	MealType  string    `json:"meal_type,omitempty"`
	Label     string    `json:"label,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Items     []Line    `json:"items"`
	Source    Source    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
	Estimates []string  `json:"estimates,omitempty"`
}

// SavedMealInput makes or replaces a saved meal: from Items, or from a logged meal (FromMealID).
type SavedMealInput struct {
	Name       string      `json:"name"`
	MealType   string      `json:"meal_type,omitempty"`
	Label      string      `json:"label,omitempty"`
	Notes      string      `json:"notes,omitempty"`
	Items      []ItemInput `json:"items,omitempty"`
	Estimates  []string    `json:"estimates,omitempty"`
	FromMealID string      `json:"from_meal_id,omitempty"`
}

// LogAgain says when a meal logged again was eaten; all optional, today by default, and the meal
// type it had unless MealType is set.
type LogAgain struct {
	Date     *Date      `json:"date,omitempty"`
	EatenAt  *time.Time `json:"eaten_at,omitempty"`
	Timezone string     `json:"timezone,omitempty"`
	MealType string     `json:"meal_type,omitempty"`
}

// ListRecipes returns the person's recipes by name.
func (c *Client) ListRecipes(ctx context.Context) ([]Recipe, error) {
	var out struct {
		Recipes []Recipe `json:"recipes"`
	}
	return out.Recipes, c.Do(ctx, http.MethodGet, "/v1/recipes", nil, nil, &out)
}

// GetRecipe reads one recipe.
func (c *Client) GetRecipe(ctx context.Context, id string) (*Recipe, error) {
	var out Recipe
	return &out, c.Do(ctx, http.MethodGet, "/v1/recipes/"+pathID(id), nil, nil, &out)
}

// CreateRecipe makes a recipe. A line whose food isn't sure is CodeInvalidInput naming the line.
func (c *Client) CreateRecipe(ctx context.Context, in RecipeInput) (*Recipe, error) {
	var out Recipe
	return &out, c.Do(ctx, http.MethodPost, "/v1/recipes", nil, in, &out)
}

// ReplaceRecipe replaces a recipe, keeping its ID; meals already logged keep what they counted.
func (c *Client) ReplaceRecipe(ctx context.Context, id string, in RecipeInput) (*Recipe, error) {
	var out Recipe
	return &out, c.Do(ctx, http.MethodPut, "/v1/recipes/"+pathID(id), nil, in, &out)
}

// DeleteRecipe deletes a recipe.
func (c *Client) DeleteRecipe(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/recipes/"+pathID(id), nil, nil, nil)
}

// ListSavedMeals returns the person's saved meals by name.
func (c *Client) ListSavedMeals(ctx context.Context) ([]SavedMeal, error) {
	var out struct {
		SavedMeals []SavedMeal `json:"saved_meals"`
	}
	return out.SavedMeals, c.Do(ctx, http.MethodGet, "/v1/saved-meals", nil, nil, &out)
}

// GetSavedMeal reads one saved meal.
func (c *Client) GetSavedMeal(ctx context.Context, id string) (*SavedMeal, error) {
	var out SavedMeal
	return &out, c.Do(ctx, http.MethodGet, "/v1/saved-meals/"+pathID(id), nil, nil, &out)
}

// CreateSavedMeal saves a meal to log again.
func (c *Client) CreateSavedMeal(ctx context.Context, in SavedMealInput) (*SavedMeal, error) {
	var out SavedMeal
	return &out, c.Do(ctx, http.MethodPost, "/v1/saved-meals", nil, in, &out)
}

// ReplaceSavedMeal replaces a saved meal, keeping its ID.
func (c *Client) ReplaceSavedMeal(ctx context.Context, id string, in SavedMealInput) (*SavedMeal, error) {
	var out SavedMeal
	return &out, c.Do(ctx, http.MethodPut, "/v1/saved-meals/"+pathID(id), nil, in, &out)
}

// DeleteSavedMeal deletes a saved meal; meals logged from it stay.
func (c *Client) DeleteSavedMeal(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/v1/saved-meals/"+pathID(id), nil, nil, nil)
}

// LogSavedMeal logs a saved meal as a new meal, each item counted again.
func (c *Client) LogSavedMeal(ctx context.Context, id string, when LogAgain) (*Written, error) {
	return c.written(ctx, "/v1/saved-meals/"+pathID(id)+"/log", when)
}

// CopyMeal logs a meal again, each item counted again.
func (c *Client) CopyMeal(ctx context.Context, mealID string, when LogAgain) (*Written, error) {
	return c.written(ctx, "/v1/meals/"+pathID(mealID)+"/copy", when)
}

func (c *Client) written(ctx context.Context, path string, body any) (*Written, error) {
	var out Written
	h, err := c.do(ctx, http.MethodPost, path, nil, body, &out)
	if err != nil {
		return nil, err
	}
	out.Replayed = h.Get("Idempotent-Replayed") == "true"
	return &out, nil
}
