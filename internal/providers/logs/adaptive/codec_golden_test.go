package logs //nolint:testpackage // Golden fixtures exercise the package's table codecs.

import (
	"bytes"
	"testing"

	"github.com/grafana/gcx/internal/format"
	"github.com/grafana/gcx/internal/testutils"
	"github.com/stretchr/testify/require"
)

func TestAdaptiveTablesGolden(t *testing.T) {
	exemptions := []Exemption{
		{ID: "exemption-2", StreamSelector: `{service_name="checkout"}`, Reason: "Keep audit logs", CreatedAt: "2026-09-01T12:00:00Z", ManagedBy: "operator", ExpiresAt: "2026-12-31T00:00:00Z", ActiveInterval: "24h", CreatedBy: "user@example.com", UpdatedAt: "2026-09-02T12:00:00Z"},
		{ID: "exemption-1", StreamSelector: `{level="error"}`},
	}
	segments := []LogSegment{
		{ID: "segment-2", Name: "Checkout", Selector: `{service_name="checkout"}`, FallbackToDefault: true, IsEarly: true, CreatedAt: "2026-09-01T12:00:00Z", UpdatedAt: "2026-09-02T12:00:00Z"},
		{ID: "segment-1", Name: "Default"},
	}
	for _, tc := range []struct {
		name  string
		codec func(string) format.Codec
		rows  any
		empty any
	}{
		{"exemptions", exemptionsTable().Codec, exemptions, []Exemption{}},
		{"segments", segmentsTable().Codec, segments, []LogSegment{}},
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
