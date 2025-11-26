package youtube

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecommendClient(t *testing.T) {
	client := NewRecommendClient()

	// Youtube dl test video
	results, err := client.Recommend(context.Background(), "AaBw37-nWaY")
	require.NoError(t, err)

	fmt.Printf("%+v\n", results)
}

func TestParseViewCount(t *testing.T) {
	count, err := parseViewCount("7 867 visningar")
	assert.NoError(t, err)
	assert.Equal(t, 7867, count)
}
