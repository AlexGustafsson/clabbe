package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	client *http.Client
	apiKey string
}

func NewClient(apiKey string) *Client {
	return &Client{
		client: &http.Client{},
		apiKey: apiKey,
	}
}

type CompletionRequest struct {
	// Messages contains messages / conversation history to use for completion.
	Messages []Message `json:"messages"`
	// Temperature controls randomness.
	// Lowering results in less random completions.
	// As the temperature approaches zero, the model will become deterministic and
	// repetitive.
	Temperature float64 `json:"temperature"`
	// MaxTokens to generate.
	MaxTokens int `json:"max_tokens"`
	// TopP controls diversity via nucleus sampling: 0.5 means half of all
	// likelihood-weighted options are considered.
	TopP float64 `json:"top_p"`
	// FrequencyPenalty controls how much the penalize new tokens based on their
	// existing frequency in the text so far. Decreases the model's likelihood to
	// repeat the same line verbatim.
	FrequencyPenalty float64 `json:"frequency_penalty"`
	// Controls how much to penalize new tokens based on whether they appear in
	// the text so far. Increases the model's likelihood to talk about new topics.
	PresencePenalty float64 `json:"presence_penalty"`
	// Model controls the model to use.
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

const DefaultModel string = "gpt-3.5-turbo-0125"

type Role string

const (
	RoleSystem    Role = "system"
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type CompletionResponse struct {
	ID                string             `json:"id"`
	Object            string             `json:"object"`
	Created           int                `json:"created"`
	Model             string             `json:"model"`
	Choices           []CompletionChoice `json:"choices"`
	Usage             CompletionUsage    `json:"usage"`
	SystemFingerprint string             `json:"system_fingerprint"`
}

type CompletionChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *Client) FetchCompletion(request *CompletionRequest) (*CompletionResponse, error) {
	if request.Stream {
		return nil, fmt.Errorf("stream mode is unsupported")
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got unexpected status: %s", res.Status)
	}

	var response CompletionResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}
