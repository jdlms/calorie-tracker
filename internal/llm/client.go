package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers"
	"github.com/mozilla-ai/any-llm-go/providers/openai"

	"github.com/joshua-lamorey/calorie-counter/internal/config"
	"github.com/joshua-lamorey/calorie-counter/internal/model"
)

// Client estimates nutritional information from free text.
type Client interface {
	EstimateEntry(ctx context.Context, message string) (model.Entry, error)
}

// NutritionClient estimates nutrition data using an LLM provider.
type NutritionClient struct {
	model    string
	provider providers.Provider
}

type llmEntryResponse struct {
	Description string `json:"description"`
	Kcal        int    `json:"kcal"`
	Protein     int    `json:"protein"`
	Fat         int    `json:"fat"`
	Carbs       int    `json:"carbs"`
}

// New creates an LLM client from config.
func New(cfg config.Config) (Client, error) {
	if cfg.LLMModel == "" {
		return NewStubClient(), nil
	}

	provider, err := newProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating llm provider: %w", err)
	}

	return &NutritionClient{
		model:    cfg.LLMModel,
		provider: provider,
	}, nil
}

func newProvider(cfg config.Config) (providers.Provider, error) {
	if cfg.LLMBaseURL != "" {
		provider, err := openai.NewCompatible(openai.CompatibleConfig{
			Name:           "openai-compatible",
			DefaultBaseURL: cfg.LLMBaseURL,
			RequireAPIKey:  false,
			Capabilities: providers.Capabilities{
				Completion: true,
			},
		}, anyllm.WithAPIKey(cfg.LLMAPIKey))
		if err != nil {
			return nil, fmt.Errorf("creating openai-compatible provider: %w", err)
		}

		return provider, nil
	}

	provider, err := openai.New(anyllm.WithAPIKey(cfg.LLMAPIKey))
	if err != nil {
		return nil, fmt.Errorf("creating openai provider: %w", err)
	}

	return provider, nil
}

// EstimateEntry estimates nutrition data for a free-text meal description.
func (c *NutritionClient) EstimateEntry(ctx context.Context, message string) (model.Entry, error) {
	if err := ctx.Err(); err != nil {
		return model.Entry{}, fmt.Errorf("checking context: %w", err)
	}

	strict := true
	response, err := c.provider.Completion(ctx, anyllm.CompletionParams{
		Model: c.model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleSystem, Content: nutritionSystemPrompt},
			{Role: anyllm.RoleUser, Content: message},
		},
		ResponseFormat: &anyllm.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &anyllm.JSONSchema{
				Name:        "nutrition_entry",
				Description: "Estimated nutritional information for a single food message",
				Strict:      &strict,
				Schema:      nutritionSchema(),
			},
		},
	})
	if err != nil {
		return model.Entry{}, fmt.Errorf("requesting llm completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return model.Entry{}, fmt.Errorf("llm returned no choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.ContentString())
	if content == "" {
		return model.Entry{}, fmt.Errorf("llm returned empty content")
	}

	var parsed llmEntryResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return model.Entry{}, fmt.Errorf("parsing llm json response: %w", err)
	}

	if parsed.Kcal < 0 || parsed.Protein < 0 || parsed.Fat < 0 || parsed.Carbs < 0 {
		return model.Entry{}, fmt.Errorf("llm returned negative nutrition values")
	}

	return model.Entry{
		Description: strings.TrimSpace(parsed.Description),
		Kcal:        parsed.Kcal,
		Protein:     parsed.Protein,
		Fat:         parsed.Fat,
		Carbs:       parsed.Carbs,
	}, nil
}

// StubClient is a temporary implementation used when no LLM is configured.
type StubClient struct{}

// NewStubClient creates a stub LLM client.
func NewStubClient() *StubClient {
	return &StubClient{}
}

// EstimateEntry returns a not-yet-configured error.
func (c *StubClient) EstimateEntry(ctx context.Context, message string) (model.Entry, error) {
	if err := ctx.Err(); err != nil {
		return model.Entry{}, fmt.Errorf("checking context: %w", err)
	}

	return model.Entry{}, fmt.Errorf("llm is not configured")
}

const nutritionSystemPrompt = "You estimate nutritional information for foods. Return only valid JSON matching the schema. Use best-effort estimates for restaurant and homemade foods. If multiple foods are mentioned, estimate the total meal. Use integer grams for macros and integer calories."

func nutritionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type": "string",
			},
			"kcal": map[string]any{
				"type":    "integer",
				"minimum": 0,
			},
			"protein": map[string]any{
				"type":    "integer",
				"minimum": 0,
			},
			"fat": map[string]any{
				"type":    "integer",
				"minimum": 0,
			},
			"carbs": map[string]any{
				"type":    "integer",
				"minimum": 0,
			},
		},
		"required":             []string{"description", "kcal", "protein", "fat", "carbs"},
		"additionalProperties": false,
	}
}
