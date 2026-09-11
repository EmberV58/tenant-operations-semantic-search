package search

import (
	"context"
	"reflect"
	"testing"
)

type fixedEmbedder struct{ vector []float64 }

func (e fixedEmbedder) Embed(context.Context, string) ([]float64, error) { return e.vector, nil }

type recordingVectors struct {
	matches []Match
	query   VectorQuery
}

func (v *recordingVectors) Query(_ context.Context, query VectorQuery) ([]Match, error) {
	v.query = query
	return v.matches, nil
}

func TestTenantSearchAppliesLifecycleBoundary(t *testing.T) {
	tests := []struct {
		name    string
		matches []Match
		want    []Result
	}{
		{
			name: "keeps active account guidance in semantic order",
			matches: []Match{
				{ID: "admin-sso", Score: 0.94, Metadata: map[string]any{"title": "Rotate an SSO certificate", "lifecycle_stage": "admin", "account_status": "active"}},
				{ID: "closed-export", Score: 0.91, Metadata: map[string]any{"title": "Export a closed account", "lifecycle_stage": "offboarding", "account_status": "closed"}},
				{ID: "invite-team", Score: 0.86, Metadata: map[string]any{"title": "Invite the first workspace members", "lifecycle_stage": "onboarding", "account_status": "active"}},
			},
			want: []Result{
				{ContentID: "admin-sso", Title: "Rotate an SSO certificate", Stage: "admin", Score: 0.94},
				{ContentID: "invite-team", Title: "Invite the first workspace members", Stage: "onboarding", Score: 0.86},
			},
		},
		{
			name:    "drops incomplete metadata",
			matches: []Match{{ID: "missing-stage", Score: 0.8, Metadata: map[string]any{"title": "Account settings", "account_status": "active"}}},
			want:    []Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vectors := &recordingVectors{matches: tt.matches}
			service := NewTenantSearch(fixedEmbedder{vector: []float64{0.1, 0.2}}, vectors, "saas-operations")
			got, err := service.Search(context.Background(), "how do I rotate SSO?", 5)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("results = %#v, want %#v", got, tt.want)
			}
			if vectors.query.Filter["account_status"] != "active" || !vectors.query.IncludeMetadata {
				t.Fatalf("query boundary = %#v", vectors.query)
			}
		})
	}
}
