package importer

import (
	"testing"
	"time"
)

func TestParseGenericRecord(t *testing.T) {
	in := `[10:22:44 >>>> EGPF_APP]
@N:RYR6VW:6202:1:55.51142:-4.60886:36:0:4294966636:34`

	got, err := ParseGenericRecord(in)
	if err != nil {
		t.Fatal(err)
	}

	wantTime := 10*time.Hour + 22*time.Minute + 44*time.Second
	if got.Time != wantTime {
		t.Errorf("Time = %v, want %v", got.Time, wantTime)
	}

	if got.Callsign != "EGPF_APP" {
		t.Errorf("Callsign = %q, want EGPF_APP", got.Callsign)
	}

	if !got.Record.Direction() {
		t.Errorf("Direction = false, want true")
	}

	p, ok := got.Record.(PositionRecord)
	if !ok {
		t.Fatalf("Record type = %T, want PositionRecord", got.Record)
	}

	if !p.IsNormalMode {
		t.Errorf("IsNormalMode = false, want true")
	}

	if p.Callsign != "RYR6VW" {
		t.Errorf("Callsign = %q, want RYR6VW", p.Callsign)
	}

	if p.Squawk != 6202 {
		t.Errorf("Squawk = %d, want 6202", p.Squawk)
	}

	if p.Latitude != 55.51142 {
		t.Errorf("Latitude = %v, want 55.51142", p.Latitude)
	}

	if p.Longitude != -4.60886 {
		t.Errorf("Longitude = %v, want -4.60886", p.Longitude)
	}

	if p.Altitude != 36 {
		t.Errorf("Altitude = %d, want 36", p.Altitude)
	}

	if p.Heading != 298 {
		t.Errorf("Heading = %d, want 298", p.Heading)
	}
}
