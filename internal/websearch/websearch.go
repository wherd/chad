package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type SearchResult struct {
	Web *WebResults `json:"web"`
}

type WebResults struct {
	Results []*WebResult `json:"results"`
}

type WebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	PageAge     string `json:"page_age"`
}

func Search(apikey string, query string) ([]*WebResult, error) {
	if apikey == "" {
		return nil, fmt.Errorf("brave search key not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.search.brave.com/res/v1/web/search?count=7&safesearch=strict&text_decorations=false&result_filter=web&extra_snippets=true&q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Subscription-Token", apikey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var results SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return results.Web.Results, nil
}
