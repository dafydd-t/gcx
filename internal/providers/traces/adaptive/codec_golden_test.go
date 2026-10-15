package traces //nolint:testpackage // Golden fixtures exercise the package's table codecs.

import (
	"bytes"
	"testing"

	"github.com/grafana/gcx/internal/format"
	"github.com/grafana/gcx/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestAdaptiveTablesGolden(t *testing.T) {
	recommendations := []Recommendation{
		{ID: "rec-2", Message: "Reduce noisy traces", Tags: []string{"volume", "cost"}, Applied: true, CreatedAt: "2026-09-01T12:00:00Z", Actions: []RecommendationAction{{}, {}}},
		{ID: "rec-1", Message: "Keep errors", Dismissed: true, Stale: true},
	}
	policies := []Policy{
		{ID: "policy-2", Name: "Sample 10%", Type: "probabilistic", ExpiresAt: "2026-12-31T00:00:00Z", VersionCreatedBy: "user@example.com", VersionCreatedAt: "2026-09-01T12:00:00Z"},
		{ID: "policy-1", Name: "Rate limit", Type: "rate_limiting"},
	}
	for _, tc := range []struct {
		name  string
		codec func(string) format.Codec
		rows  any
		empty any
	}{
		{"recommendations", recommendationTable().Codec, recommendations, []Recommendation{}},
		{"policies", PolicyTable().Codec, policies, []Policy{}},
	} {
		for _, name := range []string{"table", "wide"} {
			for _, empty := range []bool{false, true} {
				golden := tc.name + "_" + name
				rows := tc.rows
				if empty {
					golden += "_empty"
					rows = tc.empty
				}
				t.Run(golden, func(t *testing.T) {
					var buf bytes.Buffer
					require.NoError(t, tc.codec(name).Encode(&buf, rows))
					testutils.Golden(t, golden, buf.String())
				})
			}
		}
	}
}
