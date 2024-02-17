package youtube

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
)

var initialDataRegex = regexp.MustCompile(`var ytInitialData = (.*?)};`)

type SearchClient struct {
	client *http.Client
}

func NewSearchClient() *SearchClient {
	return &SearchClient{
		client: &http.Client{},
	}
}

func Search(ctx context.Context, query string) ([]string, error) {
	client := NewSearchClient()
	return client.Search(ctx, query)
}

func (c *SearchClient) Search(ctx context.Context, query string) ([]string, error) {
	// Build request URL
	searchURL := url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
		Path:   "/results",
	}

	searchQuery := make(url.Values)
	searchQuery.Set("search_query", query)
	searchURL.RawQuery = searchQuery.Encode()

	req, _ := http.NewRequest(http.MethodGet, searchURL.String(), nil)

	slog.Debug("Performing search request", slog.String("query", query))
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		slog.Error("Failed to perform search", slog.String("status", res.Status))
		return nil, fmt.Errorf("unexpected status: %s", res.Status)
	}

	// Read just as little as is requried to match the initial data
	var buffer bytes.Buffer
	reader := bufio.NewReader(io.TeeReader(res.Body, &buffer))
	match := initialDataRegex.FindReaderIndex(reader)
	if match == nil {
		return nil, fmt.Errorf("unable to find initial data in response")
	}

	// Extract the match
	initialDataBytes := buffer.Bytes()[match[0]+20 : match[1]-1]
	var initialData struct {
		Contents struct {
			TwoColumnSearchResultsRenderer struct {
				PrimaryContents struct {
					SectionListRenderer struct {
						Contents []struct {
							ItemSectionRenderer struct {
								Contents []struct {
									VideoRenderer struct {
										VideoID string `json:"videoId"`
									} `json:"videoRenderer"`
								} `json:"contents"`
							} `json:"itemSectionRenderer"`
						} `json:"contents"`
					} `json:"sectionListRenderer"`
				} `json:"primaryContents"`
			} `json:"twoColumnSearchResultsRenderer"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(initialDataBytes, &initialData); err != nil {
		return nil, err
	}

	ids := make([]string, 0)
	for _, content := range initialData.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents {
		for _, content := range content.ItemSectionRenderer.Contents {
			ids = append(ids, content.VideoRenderer.VideoID)
		}
	}

	slog.Debug("Successfully performed search", slog.Int("results", len(ids)))
	return ids, nil
}
