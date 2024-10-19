package youtube

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var initialDataRegex = regexp.MustCompile(`var ytInitialData = (.*?)};`)

var (
	ErrTooManyRequests = errors.New("too many requests")
)

type SearchClient struct {
	client *http.Client
}

func NewSearchClient() *SearchClient {
	return &SearchClient{
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// YouTube has started to redirect users to a /sorry page when some rate
				// is reached. The URL causes the golang HTTP client to get stuck in a
				// redirect loop. Catch this edge case
				if req.URL.Hostname() == "www.google.com" && strings.HasPrefix(req.URL.Path, "/sorry") {
					return ErrTooManyRequests
				}

				return nil
			},
		},
	}
}

func Search(ctx context.Context, query string) ([]SearchResult, error) {
	client := NewSearchClient()
	return client.Search(ctx, query)
}

type SearchResult struct {
	ID    string
	Title string
}

func (c *SearchClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
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

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, ErrTooManyRequests
	} else if res.StatusCode != http.StatusOK {
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
										Title   struct {
											Runs []struct {
												Text string `json:"text"`
											} `json:"runs"`
										} `json:"title"`
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

	results := make([]SearchResult, 0)
	for _, content := range initialData.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents {
		for _, content := range content.ItemSectionRenderer.Contents {
			title := ""
			if len(content.VideoRenderer.Title.Runs) > 0 {
				title = content.VideoRenderer.Title.Runs[0].Text
			}
			results = append(results, SearchResult{
				ID:    content.VideoRenderer.VideoID,
				Title: title,
			})
		}
	}

	// Some ids might be empty every now and then - filter these out
	results = slices.DeleteFunc(results, func(result SearchResult) bool {
		return result.ID == ""
	})

	slog.Debug("Successfully performed search", slog.Int("results", len(results)))
	return results, nil
}
