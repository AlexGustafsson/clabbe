package openai

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchCompletion(t *testing.T) {
	apiKey, ok := os.LookupEnv("OPENAI_API_KEY")
	if !ok {
		t.Skip("OPENAI_API_KEY not set")
	}

	client := NewClient(apiKey)

	res, err := client.FetchCompletion(context.Background(), &CompletionRequest{
		Messages: []Message{
			{
				Role:    RoleSystem,
				Content: "Add the numbers provided by the user. Respond only with the sum, nothing else.",
			},
			{
				Role:    RoleUser,
				Content: "1 2",
			},
		},
		Temperature:      1,
		MaxTokens:        256,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Model:            DefaultModel,
		Stream:           false,
	})
	require.NoError(t, err)

	fmt.Printf("%+v\n", res)

	assert.Equal(t, "3", res.Choices[0].Message.Content)
}
