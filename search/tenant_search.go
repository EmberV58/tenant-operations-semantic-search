package search

import (
	"context"
	"fmt"
	"strings"
)

type Embedder interface {
	Embed(context.Context, string) ([]float64, error)
}

type VectorQuerier interface {
	Query(context.Context, VectorQuery) ([]Match, error)
}

type VectorQuery struct {
	Collection      string
	Embedding       []float64
	TopK            int
	Filter          map[string]any
	IncludeMetadata bool
}

type Match struct {
	ID       string         `json:"id"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}

type Result struct {
	ContentID string  `json:"content_id"`
	Title     string  `json:"title"`
	Stage     string  `json:"stage"`
	Score     float64 `json:"score"`
}

type TenantSearch struct {
	embedder   Embedder
	vectors    VectorQuerier
	collection string
}

func NewTenantSearch(embedder Embedder, vectors VectorQuerier, collection string) *TenantSearch {
	return &TenantSearch{embedder: embedder, vectors: vectors, collection: collection}
}

func (s *TenantSearch) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit < 1 || limit > 20 {
		return nil, fmt.Errorf("limit must be between 1 and 20")
	}

	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed search query: %w", err)
	}
	matches, err := s.vectors.Query(ctx, VectorQuery{
		Collection:      s.collection,
		Embedding:       embedding,
		TopK:            limit,
		Filter:          map[string]any{"account_status": "active"},
		IncludeMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("query tenant content: %w", err)
	}

	results := make([]Result, 0, len(matches))
	for _, match := range matches {
		title, titleOK := match.Metadata["title"].(string)
		stage, stageOK := match.Metadata["lifecycle_stage"].(string)
		status, statusOK := match.Metadata["account_status"].(string)
		if !titleOK || !stageOK || !statusOK || status != "active" {
			continue
		}
		results = append(results, Result{ContentID: match.ID, Title: title, Stage: stage, Score: match.Score})
	}
	return results, nil
}
