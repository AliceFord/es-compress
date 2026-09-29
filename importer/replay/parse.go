package replay

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AliceFord/es-compress/record"
)

var (
	genericRecordPattern = regexp.MustCompile(`^\[(\d{2}):(\d{2}):(\d{2}) (?:2>>1|>>>>|<<<2) ([A-Za-z0-9_]+)]$`)
	// XXX: The "-?" before the altitude is memes. Turns out altitude
	// can be negative. Thanks amsterdam.
	positionRecordPattern = regexp.MustCompile(`^@([NS]):([A-Z0-9_]+):(\d{1,4}):1:(-?\d{1,2}\.\d+):(-?\d{1,2}\.\d+):-?(\d+):\d+:(\d+):-?\d+$`)
)

func Parse(s string) ([]record.GenericRecord, error) {
	text := strings.ReplaceAll(s, "\r\n", "\n")

	var outData []record.GenericRecord

	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "[") {
			combined := line + "\n" + lines[i+1]
			genericRecord, err := ParseGenericRecord(combined)
			if err != nil {
				if strings.Contains(err.Error(), "unknown record type") {
					continue
				}

				return nil, fmt.Errorf("line %d: %w", i, err)
			}

			outData = append(outData, genericRecord)

			i++
		}
	}

	return outData, nil
}

// ParseGenericRecord parses a generic record, e.g.
//
//	[10:22:44 >>>> EGPF_APP]
//	@N:RYR6VW:6202:1:55.51142:-4.60886:36:0:4294966636:34
func ParseGenericRecord(s string) (record.GenericRecord, error) {
	lines := strings.Split(s, "\n")
	if len(lines) != 2 {
		return record.GenericRecord{}, fmt.Errorf("expected 2 lines, got %d", len(lines))
	}

	t, callsign, err := parseTimeLine(lines[0])
	if err != nil {
		return record.GenericRecord{}, fmt.Errorf("parse time line: %w", err)
	}

	theRecord, err := parseRecordLine(lines[1])
	if err != nil {
		return record.GenericRecord{}, fmt.Errorf("parse record line: %w", err)
	}

	return record.GenericRecord{
		Time:     t,
		Callsign: callsign,
		Record:   theRecord,
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

func parseRecordLine(line string) (record.Record, error) {
	if strings.HasPrefix(line, "@N:") || strings.HasPrefix(line, "@S:") {
		return parsePositionRecord(line)
	}

	return nil, fmt.Errorf("unknown record type: %q", line)
}

func parsePositionRecord(line string) (record.PositionRecord, error) {
	matches := positionRecordPattern.FindAllStringSubmatch(line, -1)
	if len(matches) != 1 {
		return record.PositionRecord{}, fmt.Errorf("expected 1 match, got %d for line %s", len(matches), line)
	}

	match := matches[0]
	if len(match) != 8 {
		return record.PositionRecord{}, fmt.Errorf("expected 8 matches, got %d", len(match))
	}

	isNormalMode := match[1] == "N"
	callsign := match[2]

	squawk, err := strconv.ParseUint(match[3], 10, 16)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("squawk: %w", err)
	}

	latitude, err := strconv.ParseFloat(match[4], 64)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("latitude: %w", err)
	}

	longitude, err := strconv.ParseFloat(match[5], 64)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("longitude: %w", err)
	}

	altitude, err := strconv.ParseUint(match[6], 10, 16)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("altitude: %w", err)
	}

	headingEncoded, err := strconv.ParseUint(match[7], 10, 32)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("heading: %w", err)
	}

	heading := uint16(float64((headingEncoded&0xFFF)>>2) / 2.88)

	return record.PositionRecord{
		IsNormalMode: isNormalMode,
		Callsign:     callsign,
		Squawk:       uint16(squawk),
		Latitude:     latitude,
		Longitude:    longitude,
		Altitude:     uint16(altitude),
		Heading:      heading,
	}, nil
}
