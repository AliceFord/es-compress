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
	ignoredMessages = []string{
		"$CQ",
		"$SB",
		"$ZC",
		"$ZR",
		"#ST",
		"#AX",
	}

	genericRecordPattern = regexp.MustCompile(`^\[(\d{2}):(\d{2}):(\d{2}) (2>>1|>>>>|<<<2) ([A-Za-z0-9_]+)]$`)
	// XXX: The "-?" before the altitude is memes. Turns out altitude
	// can be negative. Thanks amsterdam.
	positionRecordPattern           = regexp.MustCompile(`^@([NS]):([A-Z0-9_]+):(\d{1,4}):1:(-?\d{1,2}\.\d+):(-?\d{1,2}\.\d+):-?(\d+):\d+:(\d+):-?\d+$`)
	messageRecordPattern            = regexp.MustCompile(`^#TM(.+?):(.+?):(.*)$`)
	addPilotRecordPattern           = regexp.MustCompile(`^#AP([A-Z0-9_]+):SERVER:(\d+)::1:\d+:(\d+):(.*)$`)
	controllerPositionRecordPattern = regexp.MustCompile(`^%([A-Z0-9_]+):(\d+):(\d+):(\d+):(\d+):(-?\d{1,2}\.\d+):(-?\d{1,2}\.\d+):\d+$`)
	flightplanRecordPattern         = regexp.MustCompile(`^\$FP([A-Z0-9_]+):.*?:(.):(.*?):(\d+):(.*?):(\d+):\d+:((?:FL)?\d*):(.*?):(\d+):(\d+):(\d+):(\d+):(.*?):(.*?):(.*)$`)
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
				if strings.Contains(err.Error(), "ignoring") {
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

	t, arrowType, callsign, err := parseTimeLine(lines[0])
	if err != nil {
		return record.GenericRecord{}, fmt.Errorf("parse time line: %w", err)
	}

	theRecord, err := parseRecordLine(lines[1], arrowType)
	if err != nil {
		return record.GenericRecord{}, fmt.Errorf("parse record line: %w", err)
	}

	return record.GenericRecord{
		Time:     t,
		Callsign: callsign,
		Record:   theRecord,
	}, nil
}

func parseTimeLine(line string) (time.Duration, string, string, error) {
	matches := genericRecordPattern.FindStringSubmatch(line)
	if len(matches) != 6 {
		return 0, "", "", fmt.Errorf("expected 6 matches, got %d", len(matches))
	}

	hour, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, "", "", fmt.Errorf("hour: %w", err)
	}

	minute, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, "", "", fmt.Errorf("minute: %w", err)
	}

	second, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, "", "", fmt.Errorf("second: %w", err)
	}

	t := time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second

	arrowType := matches[4]
	callsign := matches[5]

	return t, arrowType, callsign, nil
}

func parseRecordLine(line string, arrowType string) (record.Record, error) {
	if strings.HasPrefix(line, "@N:") || strings.HasPrefix(line, "@S:") {
		return parsePositionRecord(line)
	}
	if strings.HasPrefix(line, "#TM") {
		return parseMessageRecord(line)
	}
	if strings.HasPrefix(line, "#AP") {
		return parseAddPilotRecord(line)
	}
	if strings.HasPrefix(line, "%") {
		return parseControllerPositionRecord(line)
	}
	if strings.HasPrefix(line, "$FP") {
		return parseFlightplanRecord(line)
	}

	for _, ignored := range ignoredMessages {
		if strings.HasPrefix(line, ignored) {
			return record.UnknownRecord{}, fmt.Errorf("ignoring intentionally unplanned messages: %q", line)
		}
	}
	return parseUnknownRecord(line, arrowType)
}

func parsePositionRecord(line string) (record.PositionRecord, error) {
	matches := positionRecordPattern.FindStringSubmatch(line)
	if len(matches) != 8 {
		return record.PositionRecord{}, fmt.Errorf("expected 8 matches, got %d", len(matches))
	}

	isNormalMode := matches[1] == "N"
	callsign := matches[2]

	squawk, err := strconv.ParseUint(matches[3], 10, 16)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("squawk: %w", err)
	}

	latitude, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("latitude: %w", err)
	}

	longitude, err := strconv.ParseFloat(matches[5], 64)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("longitude: %w", err)
	}

	altitude, err := strconv.ParseUint(matches[6], 10, 16)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf("altitude: %w", err)
	}

	headingEncoded, err := strconv.ParseUint(matches[7], 10, 32)
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

func parseMessageRecord(line string) (record.MessageRecord, error) {
	matches := messageRecordPattern.FindStringSubmatch(line)
	if len(matches) != 4 {
		return record.MessageRecord{}, fmt.Errorf("expected 4 matches, got %d", len(matches))
	}

	sender := matches[1]
	receiver := matches[2]
	message := matches[3]

	if receiver == "FP" {
		return record.MessageRecord{}, fmt.Errorf("ignoring FP message: %q", line)
	}

	return record.MessageRecord{
		Sender:   sender,
		Receiver: receiver,
		Message:  message,
	}, nil
}

func parseUnknownRecord(line string, arrowType string) (record.UnknownRecord, error) {
	return record.UnknownRecord{
		Raw:       line,
		ArrowType: arrowType,
	}, nil
}

func parseAddPilotRecord(line string) (record.AddPilotRecord, error) {
	matches := addPilotRecordPattern.FindStringSubmatch(line)
	if len(matches) != 5 {
		return record.AddPilotRecord{}, fmt.Errorf("expected 5 matches, got %d", len(matches))
	}

	callsign := matches[1]
	cid, err := strconv.ParseUint(matches[2], 10, 32)
	if err != nil {
		return record.AddPilotRecord{}, fmt.Errorf("cid: %w", err)
	}

	rating, err := strconv.ParseUint(matches[3], 10, 8)
	if err != nil {
		return record.AddPilotRecord{}, fmt.Errorf("rating: %w", err)
	}

	name := matches[4]

	return record.AddPilotRecord{
		Callsign: callsign,
		CID:      uint32(cid),
		Rating:   uint8(rating),
		Name:     name,
	}, nil
}

func parseControllerPositionRecord(line string) (record.ControllerPositionRecord, error) {
	matches := controllerPositionRecordPattern.FindStringSubmatch(line)
	if len(matches) != 8 {
		return record.ControllerPositionRecord{}, fmt.Errorf("expected 8 matches, got %d", len(matches))
	}

	callsign := matches[1]
	frequency, err := strconv.ParseUint(matches[2], 10, 24)
	if err != nil {
		return record.ControllerPositionRecord{}, fmt.Errorf("frequency: %w", err)
	}

	altitude, err := strconv.ParseUint(matches[3], 10, 16)
	if err != nil {
		return record.ControllerPositionRecord{}, fmt.Errorf("altitude: %w", err)
	}

	protocolVer, err := strconv.ParseUint(matches[4], 10, 16)
	if err != nil {
		return record.ControllerPositionRecord{}, fmt.Errorf("protocolVer: %w", err)
	}

	rating, err := strconv.ParseUint(matches[5], 10, 8)
	if err != nil {
		return record.ControllerPositionRecord{}, fmt.Errorf("rating: %w", err)
	}

	lat, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return record.ControllerPositionRecord{}, fmt.Errorf("lat: %w", err)
	}

	lon, err := strconv.ParseFloat(matches[7], 64)
	if err != nil {
		return record.ControllerPositionRecord{}, fmt.Errorf("lon: %w", err)
	}

	return record.ControllerPositionRecord{
		Callsign:    callsign,
		Frequency:   uint32(frequency),
		Altitude:    uint16(altitude),
		ProtocolVer: uint16(protocolVer),
		Rating:      uint8(rating),
		Lat:         lat,
		Lon:         lon,
	}, nil
}

func parseFlightplanRecord(line string) (record.FlightplanRecord, error) {
	matches := flightplanRecordPattern.FindStringSubmatch(line)
	if len(matches) != 16 {
		return record.FlightplanRecord{}, fmt.Errorf("expected 16 matches, got %d", len(matches))
	}

	callsign := matches[1]
	flightRules := matches[2][0]
	aircraftType := matches[3]
	speed, err := strconv.ParseUint(matches[4], 10, 16)
	if err != nil {
		return record.FlightplanRecord{}, fmt.Errorf("speed: %w", err)
	}

	departure := matches[5]
	offBlocksRaw := matches[6]
	var offblocksTime time.Duration
	if len(offBlocksRaw) >= 3 {
		offblocksTime, err = time.ParseDuration(offBlocksRaw[:len(offBlocksRaw)-2] + "h" + offBlocksRaw[len(offBlocksRaw)-2:] + "m")
		if err != nil {
			return record.FlightplanRecord{}, fmt.Errorf("offblocksTime: %w", err)
		}
	} else {
		offblocksTime, err = time.ParseDuration(offBlocksRaw + "m")
		if err != nil {
			return record.FlightplanRecord{}, fmt.Errorf("offblocksTime: %w", err)
		}
	}

	cruiseRaw := matches[7]
	var cruiseAlt uint64

	if cruiseRaw != "" {
		if strings.HasPrefix(cruiseRaw, "FL") {
			cruiseRaw = cruiseRaw[2:] + "00"
		}

		cruiseAlt, err = strconv.ParseUint(cruiseRaw, 10, 16)
		if err != nil {
			return record.FlightplanRecord{}, fmt.Errorf("cruiseAlt: %w", err)
		}
	}

	arrival := matches[8]
	enrouteTime, err := time.ParseDuration(matches[9] + "h" + matches[10] + "m")
	if err != nil {
		return record.FlightplanRecord{}, fmt.Errorf("enrouteTime: %w", err)
	}

	enrouteFuel, err := time.ParseDuration(matches[11] + "h" + matches[12] + "m")
	if err != nil {
		return record.FlightplanRecord{}, fmt.Errorf("enrouteFuel: %w", err)
	}

	alternate := matches[13]
	details := matches[14]
	route := matches[15]

	return record.FlightplanRecord{
		Callsign:      callsign,
		FlightRules:   flightRules,
		AircraftType:  aircraftType,
		Speed:         uint16(speed),
		Departure:     departure,
		OffblocksTime: offblocksTime,
		CruiseAlt:     uint16(cruiseAlt),
		Arrival:       arrival,
		EnrouteTime:   enrouteTime,
		EnrouteFuel:   enrouteFuel,
		Alternate:     alternate,
		Details:       details,
		Route:         route,
	}, nil
}
