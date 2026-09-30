package e2e

import (
	"bytes"
	"io"
	"log/slog"
	"math"
	"os"
	"testing"

	exportesc "github.com/AliceFord/es-compress/exporter/esc"
	importesc "github.com/AliceFord/es-compress/importer/esc"
	importreplay "github.com/AliceFord/es-compress/importer/replay"
	"github.com/AliceFord/es-compress/record"
	"github.com/google/go-cmp/cmp"
)

func TestReplayToESCRoundTripPreservesRecords(t *testing.T) {
	data, err := os.ReadFile("testdata/painefulglasgow.golden.txt")
	if err != nil {
		t.Fatalf("failed to read test replay: %v", err)
	}

	original, err := importreplay.Parse(string(data))
	if err != nil {
		t.Fatalf("failed to parse replay: %v", err)
	}

	esc := exportesc.Export(original)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	roundTripped, err := importesc.NewParser(logger).Parse(
		bytes.NewReader(esc),
	)
	if err != nil {
		t.Fatalf("failed to parse exported ESC: %v", err)
	}

	positionComparer := cmp.Comparer(func(a, b record.PositionRecord) bool {
		return a.IsNormalMode == b.IsNormalMode &&
			a.Callsign == b.Callsign &&
			a.Squawk == b.Squawk &&
			math.Abs(a.Latitude-b.Latitude) <= 0.00001+1e-12 &&
			math.Abs(a.Longitude-b.Longitude) <= 0.00001+1e-12 &&
			a.Altitude == b.Altitude &&
			a.Heading == b.Heading
	})

	if diff := cmp.Diff(original, roundTripped, positionComparer); diff != "" {
		t.Errorf("records differ after round trip (-want +got):\n%s", diff)
	}
}
