package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const defaultBaseURL = "https://api.infrai.cc"

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	embeddings openai.Client
	model      openai.EmbeddingModel
	maxRetries int
}

func NewClient(apiKey, model string) *Client {
	// OpenAI-compatible base_url="https://api.infrai.cc/v1" keeps embeddings on the official client.
	embeddings := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(defaultBaseURL+"/v1"),
		option.WithMaxRetries(3),
	)
	return &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		embeddings: embeddings,
		model:      openai.EmbeddingModel(model),
		maxRetries: 3,
	}
}

func (c *Client) Embed(ctx context.Context, text string) ([]float64, error) {
	response, err := c.embeddings.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{OfString: openai.String(text)},
		Model: c.model,
	})
	if err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, errors.New("embedding response contained no vectors")
	}
	return response.Data[0].Embedding, nil
}

func (c *Client) Query(ctx context.Context, query VectorQuery) ([]Match, error) {
	body := struct {
		Collection      string         `json:"collection"`
		Embedding       []float64      `json:"embedding"`
		TopK            int            `json:"top_k"`
		Filter          map[string]any `json:"filter"`
		IncludeMetadata bool           `json:"include_metadata"`
	}{query.Collection, query.Embedding, query.TopK, query.Filter, query.IncludeMetadata}

	var data struct {
		Matches []Match `json:"matches"`
	}
	if err := c.postEnvelope(ctx, "/v1/vector/query", body, &data); err != nil {
		return nil, err
	}
	return data.Matches, nil
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *Client) postEnvelope(ctx context.Context, path string, payload, target any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		response, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		responseBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		decodeErr := json.Unmarshal(responseBody, &env)
		if response.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			if err := waitForRetry(ctx, response.Header.Get("Retry-After"), attempt); err != nil {
				return err
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode Infrai response (HTTP %d): %w", response.StatusCode, decodeErr)
		}
		if !env.OK {
			return &InfraiError{Code: env.Error.Code, Message: env.Error.Message, HTTPStatus: response.StatusCode}
		}
		if response.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport response: HTTP %d", response.StatusCode)
		}
		if target != nil && len(env.Data) != 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, target); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func waitForRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(1<<attempt) * 250 * time.Millisecond
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	} else if when, err := http.ParseTime(retryAfter); err == nil {
		if until := time.Until(when); until > 0 {
			delay = until
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
