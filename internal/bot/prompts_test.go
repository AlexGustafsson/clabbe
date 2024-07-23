package bot

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderPrompt(t *testing.T) {
	output, err := RenderPrompt(defaultThemeSuggestionTemplate, nil)
	require.NoError(t, err)
	fmt.Println(output)

	output, err = RenderPrompt(defaultSongSuggestionTemplate, map[string]any{
		"history": []map[string]any{
			{"name": "Foo"},
			{"name": "Bar"},
			{"name": "Foo"},
			{"name": "Bar"},
			{"name": "Foo"},
			{"name": "Bar"},
			{"name": "Foo"},
			{"name": "Bar"},
		},
		"similar": []map[string]any{
			{"name": "Foo"},
			{"name": "Bar"},
			{"name": "Foo"},
			{"name": "Bar"},
			{"name": "Foo"},
			{"name": "Bar"},
			{"name": "Foo"},
			{"name": "Bar"},
		},
	})
	require.NoError(t, err)
	fmt.Println(output)
}
