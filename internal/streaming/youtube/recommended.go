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
	"strconv"
	"strings"
	"time"
)

var leadingNumbersRegex = regexp.MustCompile(`^[\d\s\x{00A0}]+`)
var whitespaceRegex = regexp.MustCompile(`[\s\x{00A0}]`)

type RecommendClient struct {
	client *http.Client
}

func NewRecommendClient() *RecommendClient {
	return &RecommendClient{
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

func Recommend(ctx context.Context, query string) ([]RecommendResult, error) {
	client := NewRecommendClient()
	return client.Recommend(ctx, query)
}

type RecommendResult struct {
	ID       string
	Title    string
	Views    int
	Duration time.Duration
}

func (c *RecommendClient) Recommend(ctx context.Context, id string) ([]RecommendResult, error) {
	// Build request URL
	u := url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
		Path:   "/watch",
	}

	query := make(url.Values)
	query.Set("v", id)
	u.RawQuery = query.Encode()

	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)

	slog.Debug("Performing recommend request", slog.String("id", id))
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, ErrTooManyRequests
	} else if res.StatusCode != http.StatusOK {
		slog.Error("Failed to perform recommendation", slog.String("status", res.Status))
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
			TwoColumnWatchNextResults struct {
				SecondaryResults struct {
					SecondaryResults struct {
						Results []struct {
							// NOTE: There are other types of results, such as ads and
							// playlists, but we just care about videos for now
							CompactVideoRenderer struct {
								VideoID string `json:"videoId"`
								Title   struct {
									SimpleText string `json:"simpleText"`
								} `json:"title"`
								ViewCountText struct {
									SimpleText string `json:"simpleText"`
								} `json:"viewCountText"`
								LengthText struct {
									SimpleText string `json:"simpleText"`
								} `json:"lengthText"`
							} `json:"compactVideoRenderer"`
						} `json:"results"`
					} `json:"secondaryResults"`
				} `json:"secondaryResults"`
			} `json:"twoColumnWatchNextResults"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(initialDataBytes, &initialData); err != nil {
		return nil, err
	}

	results := make([]RecommendResult, 0)
	for _, content := range initialData.Contents.TwoColumnWatchNextResults.SecondaryResults.SecondaryResults.Results {
		title := content.CompactVideoRenderer.Title.SimpleText
		id := content.CompactVideoRenderer.VideoID

		if title != "" && id != "" {
			duration, err := parseDuration(content.CompactVideoRenderer.LengthText.SimpleText)
			if err != nil {
				return nil, fmt.Errorf("failed to parse duration of video: %w", err)
			}

			views, err := parseViewCount(content.CompactVideoRenderer.ViewCountText.SimpleText)
			if err != nil {
				return nil, fmt.Errorf("failed to parse view count of video: %w", err)
			}

			results = append(results, RecommendResult{
				ID:       id,
				Title:    title,
				Duration: duration,
				Views:    views,
			})
		}
	}

	slog.Debug("Successfully performed recommendation", slog.Int("results", len(results)))
	return results, nil
}

func parseDuration(text string) (time.Duration, error) {
	result := time.Duration(0)
	parts := strings.Split(text, ":")
	multipliers := []time.Duration{time.Second, time.Minute, time.Hour}
	for i := 0; i < len(parts); i++ {
		part, err := strconv.ParseInt(parts[len(parts)-i-1], 10, 32)
		if err != nil {
			return 0, err
		}

		result += time.Duration(part) * multipliers[i]
	}
	return result, nil
}

func parseViewCount(text string) (int, error) {
	countValue := leadingNumbersRegex.FindString(strings.TrimSpace(text))
	if countValue == "" {
		return 0, fmt.Errorf("invalid view count - expected leading numbers")
	}

	// Count may contain non-breaking spaces and other types of whitespace
	count, err := strconv.ParseInt(whitespaceRegex.ReplaceAllString(countValue, ""), 10, 32)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}
