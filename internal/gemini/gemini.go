package gemini

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/dawreen/audio-to-go/internal/config"
)

const prompt = `You received an audio note from a meeting or personal capture.
Analyze it and respond ONLY with a markdown block in this exact format:

## Meeting Note — %s

**Tone:** [one of: focused / reflective / urgent / casual / brainstorming]

**Summary:**
[2-3 sentence summary of what was discussed or decided]

**Key Points:**
- [point 1]
- [point 2]

**Tasks:**
- [ ] [task 1]
- [ ] [task 2]

---`

// Client wraps the Gemini API.
type Client struct {
	inner *genai.Client
	model string
}

func New(cfg *config.Config) (*Client, error) {
	ctx := context.Background()
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating gemini client: %w", err)
	}
	return &Client{inner: c, model: cfg.GeminiModel}, nil
}

// Analyze sends audio bytes to Gemini and returns a formatted markdown block.
// It retries up to 3 times with exponential backoff on transient errors.
func (c *Client) Analyze(ctx context.Context, audioBytes []byte, mimeType string) (string, error) {
	timestamp := time.Now().Format("15:04")
	filledPrompt := fmt.Sprintf(prompt, timestamp)

	parts := []*genai.Part{
		{Text: filledPrompt},
		{InlineData: &genai.Blob{Data: audioBytes, MIMEType: mimeType}},
	}
	contents := []*genai.Content{{Parts: parts}}

	var lastErr error
	backoff := 2 * time.Second

	for attempt := 1; attempt <= 3; attempt++ {
		result, err := c.inner.Models.GenerateContent(ctx, c.model, contents, nil)
		if err != nil {
			lastErr = err
			log.Printf("gemini attempt %d/3 failed: %v", attempt, err)
			if attempt < 3 {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}

		raw := result.Text()
		if raw == "" {
			lastErr = fmt.Errorf("gemini returned empty response")
			log.Printf("gemini attempt %d/3: empty response", attempt)
			if attempt < 3 {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}

		return extractBlock(raw), nil
	}

	return "", fmt.Errorf("gemini failed after 3 attempts: %w", lastErr)
}

// extractBlock pulls the markdown note block from the raw Gemini response.
// Gemini sometimes adds preamble text before the ## heading.
func extractBlock(raw string) string {
	start := strings.Index(raw, "## Meeting Note")
	if start == -1 {
		// No heading found — use the full response and log a warning.
		log.Printf("warning: gemini response missing '## Meeting Note' heading, using full response")
		return strings.TrimSpace(raw)
	}

	block := raw[start:]

	// Include the trailing --- separator if present.
	if end := strings.LastIndex(block, "---"); end != -1 {
		block = block[:end+3]
	}

	return strings.TrimSpace(block)
}
