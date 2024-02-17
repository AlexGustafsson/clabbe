package youtube

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchClient(t *testing.T) {
	client := NewSearchClient()

	ids, err := client.Search(context.Background(), "test")
	require.NoError(t, err)

	fmt.Println(ids)
}
