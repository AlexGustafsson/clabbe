package youtube

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchClient(t *testing.T) {
	client := NewSearchClient()

	results, err := client.Search(context.Background(), "BaW_jenozKc")
	require.NoError(t, err)

	fmt.Printf("%+v\n", results)
}
