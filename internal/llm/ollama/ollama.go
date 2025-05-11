package ollama

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AlexGustafsson/clabbe/internal/llm"
	ollama "github.com/ollama/ollama/api"
)

var _ llm.Client = (*Client)(nil)

type Client struct {
	client    *ollama.Client
	model     string
	keepAlive time.Duration
}

type Options struct {
	KeepAlive time.Duration
}

func NewClient(base *url.URL, model string, options *Options) *Client {
	keepAlive := time.Duration(0)
	if options != nil {
		keepAlive = options.KeepAlive
	}

	return &Client{
		client:    ollama.NewClient(base, &http.Client{}),
		model:     model,
		keepAlive: keepAlive,
	}
}

// Chat implements llm.Client.
func (c *Client) Chat(ctx context.Context, r *llm.ChatRequest) (*llm.ChatResponse, error) {
	messages := make([]ollama.Message, len(r.Messages))
	for i, m := range r.Messages {
		messages[i] = ollama.Message{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	stream := false
	keepAlive := ollama.Duration{Duration: 0}
	if c.keepAlive > 0 {
		keepAlive = ollama.Duration{Duration: c.keepAlive}
	}

	req := &ollama.ChatRequest{
		Model:     c.model,
		Messages:  messages,
		Stream:    &stream,
		KeepAlive: &keepAlive,
	}

	responses := make([]ollama.ChatResponse, 0)
	err := c.client.Chat(ctx, req, func(res ollama.ChatResponse) error {
		responses = append(responses, res)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var builder strings.Builder
	for _, res := range responses {
		builder.WriteString(res.Message.Content)
	}

	return &llm.ChatResponse{
		Message: llm.Message{

			// Assume assistant role
			Role:    llm.RoleAssistant,
			Content: builder.String(),
		},
	}, nil
}
