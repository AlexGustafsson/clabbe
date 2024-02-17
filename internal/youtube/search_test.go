package youtube

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchClient(t *testing.T) {
	client, err := NewSearchClient(context.Background())
	require.NoError(t, err)

	ids, err := client.Search(context.Background(), "test")
	require.NoError(t, err)

	fmt.Println(ids)
}
