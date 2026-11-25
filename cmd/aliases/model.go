package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"google.golang.org/genai"
)

// The models. Claude Haiku answers, as it does for the /food-aliases skill; Gemini on Vertex AI is
// the backup, asked only when the Claude API is unavailable (see unavailable).
const (
	claudeModel = "claude-haiku-5-5"
	geminiModel = "gemini-3.8-flash"
	location    = "global"
)

// models asks Claude, and Gemini when Claude is unavailable. gemini is nil without OFL_GCP_PROJECT.
type models struct {
	claude anthropic.Client
	gemini *genai.Client
}

// newModels builds the Claude client from ANTHROPIC_API_KEY (or an `ant auth login` profile) and,
// when OFL_GCP_PROJECT is set, the Vertex AI backup from Application Default Credentials.
func newModels(ctx context.Context) (*models, error) {
	m := &models{claude: anthropic.NewClient()}
	project := os.Getenv("OFL_GCP_PROJECT")
	if project == "" {
		return m, nil
	}
	gemini, err := genai.NewClient(ctx, &genai.ClientConfig{Backend: genai.BackendVertexAI, Project: project, Location: location})
	if err != nil {
		return nil, fmt.Errorf("vertex ai backup: %w", err)
	}
	m.gemini = gemini
	return m, nil
}

// answer is one attempt: Claude, then Gemini if Claude is unavailable and a backup is configured.
// It returns the answer and the model that gave it.
func (m *models) answer(ctx context.Context, prompt string) (string, string, error) {
	text, err := m.askClaude(ctx, prompt)
	if err == nil {
		return text, claudeModel, nil
	}
	if m.gemini == nil || ctx.Err() != nil || !unavailable(err) {
		return "", "", err
	}
	fmt.Fprintf(os.Stderr, "claude unavailable (%v); asking %s\n", err, geminiModel)
	text, err = m.askGemini(ctx, prompt)
	return text, geminiModel, err
}

func (m *models) askClaude(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	resp, err := m.claude.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     claudeModel,
		MaxTokens: 16000,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	})
	if err != nil {
		return "", fmt.Errorf("claude: %w", err)
	}
	if resp.StopReason != anthropic.StopReasonEndTurn {
		return "", fmt.Errorf("claude: stopped with %s", resp.StopReason)
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	return text.String(), nil
}

func (m *models) askGemini(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	resp, err := m.gemini.Models.GenerateContent(ctx, geminiModel, genai.Text(prompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
		ResponseMIMEType:  "application/json",
		Temperature:       genai.Ptr[float32](0),
	})
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	return resp.Text(), nil
}

// unavailable reports whether a Claude error is an outage the backup should cover: a rate limit, a
// server error, a timeout or a transport failure. Any other API status (a bad request, a missing
// credential, a permission) is a fault the backup would only hide.
func unavailable(err error) bool {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}
	return true
}

// jsonBody is the answer without the Markdown code fence a model may wrap JSON in.
func jsonBody(text string) []byte {
	text = strings.TrimSpace(text)
	if rest, ok := strings.CutPrefix(text, "```"); ok {
		rest = strings.TrimPrefix(rest, "json")
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "```"))
	}
	return []byte(text)
}
