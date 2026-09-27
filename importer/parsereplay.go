package importer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GenericRecord is one timestamp line plus one message line from an ES replay.
type GenericRecord struct {
	Time     time.Duration
	Callsign string
	Record   Record
}

// Record is any message that can appear as the second line of a GenericRecord.
// Concrete types implement this interface.
type Record interface {
	Direction() bool // true if direction is >>, false if <<
	isPosition()
}

// PositionRecord is the @N / @S position update record.
type PositionRecord struct {
	IsNormalMode bool
	Callsign     string
	Squawk       uint64
	Latitude     float64
	Longitude    float64
	Altitude     uint64
	Heading      uint64
}

func (PositionRecord) isPosition() {}

func (r PositionRecord) Direction() bool {
	return true
}

var (
	genericRecordPattern  = regexp.MustCompile(`^\[(\d{2}):(\d{2}):(\d{2}) (?:2>>1|>>>>|<<<2) ([A-Za-z0-9_]+)\]$`)
	positionRecordPattern = regexp.MustCompile(`^@(N|S):([A-Z0-9_]+):(\d{1,4}):1:(-?\d{1,2}\.\d+):(-?\d{1,2}\.\d+):(\d+):\d+:(\d+):-?\d+$`)
)

func Parse(s string) ([]GenericRecord, error) {
	text := strings.ReplaceAll(s, "\r\n", "\n")

	outData := []GenericRecord{}

	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "[") {
			combined := line + "\n" + lines[i+1]
			record, err := ParseGenericRecord(combined)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i, err)
			}

			outData = append(outData, record)

			i++
		}
	}

	return outData, nil
}

// ParseGenericRecord parses a generic record, e.g.
//
//	[10:22:44 >>>> EGPF_APP]
//	@N:RYR6VW:6202:1:55.51142:-4.60886:36:0:4294966636:34
func ParseGenericRecord(s string) (GenericRecord, error) {
	lines := strings.Split(s, "\n")
	if len(lines) != 2 {
		return GenericRecord{}, fmt.Errorf("expected 2 lines, got %d", len(lines))
	}

	t, callsign, err := parseTimeLine(lines[0])
	if err != nil {
		return GenericRecord{}, fmt.Errorf("parse time line: %w", err)
	}

	record, err := parseRecordLine(lines[1])
	if err != nil {
		return GenericRecord{}, fmt.Errorf("parse record line: %w", err)
	}

	return GenericRecord{
		Time:     t,
		Callsign: callsign,
		Record:   record,
	}, nil
}

func parseTimeLine(line string) (time.Duration, string, error) {
	matches := genericRecordPattern.FindAllStringSubmatch(line, -1)
	if len(matches) != 1 {
		return 0, "", fmt.Errorf("expected 1 match, got %d", len(matches))
	}

	match := matches[0]
	if len(match) != 5 {
		return 0, "", fmt.Errorf("expected 6 matches, got %d", len(match))
	}

	hour, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, "", fmt.Errorf("hour: %w", err)
	}

	minute, err := strconv.Atoi(match[2])
	if err != nil {
		return 0, "", fmt.Errorf("minute: %w", err)
	}

	second, err := strconv.Atoi(match[3])
	if err != nil {
		return 0, "", fmt.Errorf("second: %w", err)
	}

	t := time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second

	callsign := match[4]

	return t, callsign, nil
}

func parseRecordLine(line string) (Record, error) {
	if strings.HasPrefix(line, "@N:") || strings.HasPrefix(line, "@S:") {
		return parsePositionRecord(line)
	}

	// XXX: should be nil, fmt.Errorf("unknown record type: %q", line)
	return nil, nil
}

func parsePositionRecord(line string) (PositionRecord, error) {
	matches := positionRecordPattern.FindAllStringSubmatch(line, -1)
	if len(matches) != 1 {
		return PositionRecord{}, fmt.Errorf("expected 1 match, got %d for line %s", len(matches), line)
	}

	match := matches[0]
	if len(match) != 8 {
		return PositionRecord{}, fmt.Errorf("expected 8 matches, got %d", len(match))
	}

	isNormalMode := match[1] == "N"
	callsign := match[2]

	squawk, err := strconv.ParseUint(match[3], 10, 16)
	if err != nil {
		return PositionRecord{}, fmt.Errorf("squawk: %w", err)
	}

	latitude, err := strconv.ParseFloat(match[4], 64)
	if err != nil {
		return PositionRecord{}, fmt.Errorf("latitude: %w", err)
	}

	longitude, err := strconv.ParseFloat(match[5], 64)
	if err != nil {
		return PositionRecord{}, fmt.Errorf("longitude: %w", err)
	}

	altitude, err := strconv.ParseUint(match[6], 10, 16)
	if err != nil {
		return PositionRecord{}, fmt.Errorf("altitude: %w", err)
	}

	headingEncoded, err := strconv.ParseUint(match[7], 10, 32)
	if err != nil {
		return PositionRecord{}, fmt.Errorf("heading: %w", err)
	}

	heading := uint64(float64((headingEncoded&0xFFF)>>2) / 2.88)

	return PositionRecord{
		IsNormalMode: isNormalMode,
		Callsign:     callsign,
		Squawk:       squawk,
		Latitude:     latitude,
		Longitude:    longitude,
		Altitude:     altitude,
		Heading:      heading,
	}, nil
}
