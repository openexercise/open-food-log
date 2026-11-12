# openfoodlog (Go)

A thin Go client for any [Open Food Log](https://openfoodlog.org) server, on the standard library
alone: `net/http`, the access token on every request, an `Idempotency-Key` on every POST, RFC 9457
problems as errors, and keyset paging over the meals change feed. Meals and foods are Open Food Log
v0 documents. It covers the API core (`api/core.yaml`), which every server serves; it's the sister
of Open Workout's client (`github.com/openexercise/open-workout/clients/go`).

```go
import "github.com/openexercise/open-food-log/clients/go"

c := openfoodlog.New(openfoodlog.LifterclubURL, tokens.Access) // or any Open Food Log server

written, err := c.LogMeal(ctx, openfoodlog.MealInput{
	ExternalID: "my-7", // sending it again replays the stored meal
	Items:      []openfoodlog.ItemInput{{Text: "banana", FoodID: "ofc:bananas-raw"}},
})
for _, confirm := range written.NeedsConfirmation {
	// Ask the user which food they meant, then c.UpdateItem(ctx, confirm.ItemID, ...).
}

// Mirror a food log: every change since last time, oldest first, deleted ones as tombstones.
for ch, err := range c.Changes(ctx, openfoodlog.ChangesOptions{UpdatedSince: lastSeen, IncludeDeleted: true}) {
	if err != nil {
		return err
	}
	if ch.Deleted != nil {
		remove(ch.Deleted.ID)
	} else {
		store(*ch.Meal)
	}
}

// A webhook delivery.
ok := openfoodlog.VerifySignature(secret, r.Header.Get(openfoodlog.SignatureHeader), body, time.Now(), 5*time.Minute)
```

Getting the token is the app's job: OAuth 2.1 with PKCE, with `meals:read` and `meals:write`,
found from the server's 401. Set `TokenSource` to hand the client the current token as you refresh
it. Pass `openfoodlog.WithIdempotencyKey(ctx, key)` to retry a write across restarts with the same
key. Routes without a method here go through `Do`.

Published from [Lifterclub](https://lifterclub.com/developers), the reference implementation, whose
end-to-end tests run it against the real API.
