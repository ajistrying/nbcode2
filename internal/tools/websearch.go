package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type WebSearchTool struct{}

type webSearchArgs struct {
	Query string `json:"query"`
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) RiskTier() RiskTier   { return RiskHigh }
func (t *WebSearchTool) Description() string {
	return "Search the web for information. Returns search results with titles, URLs, and snippets."
}

func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query"
			}
		},
		"required": ["query"]
	}`)
}

func (t *WebSearchTool) Execute(args json.RawMessage) (string, error) {
	var a webSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Use DuckDuckGo HTML search (no API key required)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(a.Query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "nbcode/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web search failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	// Parse the HTML response for result snippets
	results := extractDDGResults(string(body))
	if len(results) == 0 {
		return "No results found.", nil
	}

	return strings.Join(results, "\n\n"), nil
}

// extractDDGResults does basic extraction from DuckDuckGo HTML results.
func extractDDGResults(html string) []string {
	var results []string

	// Split on result divs
	parts := strings.Split(html, "class=\"result__a\"")
	for i := 1; i < len(parts) && i <= 10; i++ {
		part := parts[i]

		// Extract title
		title := extractBetween(part, ">", "</a>")
		title = stripHTMLTags(title)

		// Extract URL
		href := extractBetween(parts[i-1]+part, "href=\"", "\"")
		if strings.HasPrefix(href, "//duckduckgo.com/l/") {
			// Extract actual URL from redirect
			if u := extractBetween(href, "uddg=", "&"); u != "" {
				decoded, err := url.QueryUnescape(u)
				if err == nil {
					href = decoded
				}
			}
		}

		// Extract snippet
		snippet := ""
		if snipIdx := strings.Index(part, "result__snippet"); snipIdx >= 0 {
			snippet = extractBetween(part[snipIdx:], ">", "</")
			snippet = stripHTMLTags(snippet)
		}

		if title != "" {
			result := fmt.Sprintf("**%s**\n%s", strings.TrimSpace(title), strings.TrimSpace(href))
			if snippet != "" {
				result += fmt.Sprintf("\n%s", strings.TrimSpace(snippet))
			}
			results = append(results, result)
		}
	}

	return results
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i+len(start):]
	j := strings.Index(s, end)
	if j < 0 {
		return s
	}
	return s[:j]
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}
