package esc

import (
	"bytes"
	"fmt"
	"time"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

const (
	version byte = 0

	recordTypePosition       byte = 0
	recordTypeDelta          byte = 1 // Generic delta with explicit change map
	recordTypeController     byte = 2
	recordTypeTimestamp      byte = 3
	recordTypeTimestampPlus1 byte = 4

	recordTypeDeltaNone         byte = 5
	recordTypeDeltaLatLon       byte = 6
	recordTypeDeltaLatLonAlt    byte = 7
	recordTypeDeltaLatLonAltHdg byte = 8
	recordTypeDeltaLatLonHdg    byte = 9

	recordTypeMessage byte = 10
	recordTypeUnknown byte = 11

	stop = 0

	changeTransponderType byte = 1 << 0
	changeSquawk          byte = 1 << 1
	changeLat             byte = 1 << 2
	changeLon             byte = 1 << 3
	changeAlt             byte = 1 << 4
	changeHdg             byte = 1 << 5
)

type universalState struct {
	currentController string
}

type aircraftState struct {
	IsNormalMode bool
	Squawk       uint16
	Latitude     int32
	Longitude    int32
	Altitude     uint16
	Heading      uint16
}

func Export(records []record.GenericRecord) []byte {
	var buf bytes.Buffer

	buf.WriteString("skog")
	buf.WriteByte(version)

	callsignIDs, aircraftIDs := writeRegistries(&buf, records)
	writeRecordStream(&buf, records, callsignIDs, aircraftIDs)

	return buf.Bytes()
}

func writeRegistries(
	buf *bytes.Buffer,
	records []record.GenericRecord,
) (map[string]uint8, map[string]uint16) {
	callsigns := make([]string, 0)
	callsignIDs := make(map[string]uint8)

	aircraft := make([]string, 0)
	aircraftIDs := make(map[string]uint16)

	for _, rec := range records {
		if _, ok := callsignIDs[rec.Callsign]; !ok {
			callsignIDs[rec.Callsign] = uint8(len(callsigns))
			callsigns = append(callsigns, rec.Callsign)
		}

		pos, ok := rec.Record.(record.PositionRecord)
		if !ok {
			continue
		}

		if _, ok := aircraftIDs[pos.Callsign]; !ok {
			aircraftIDs[pos.Callsign] = uint16(len(aircraft))
			aircraft = append(aircraft, pos.Callsign)
		}
	}

	writeCallsignList(buf, callsigns)
	writeCallsignList(buf, aircraft)

	return callsignIDs, aircraftIDs
}

func writeCallsignList(buf *bytes.Buffer, callsigns []string) {
	for _, cs := range callsigns {
		buf.WriteString(cs)
		buf.WriteByte(stop)
	}

	buf.WriteByte(stop)
}

func writeRecordStream(
	buf *bytes.Buffer,
	records []record.GenericRecord,
	callsignIDs map[string]uint8,
	aircraftIDs map[string]uint16,
) {
	aircraftStates := make(map[uint16]aircraftState)
	currentState := new(universalState)

	var prevTime time.Duration

	for i := 0; i < len(records); {
		t := records[i].Time
		j := i

		// Handle all records with the same timestamp together.
		for j < len(records) && records[j].Time == t {
			j++
		}

		if t-prevTime == time.Second {
			buf.WriteByte(recordTypeTimestampPlus1)
		} else {
			buf.WriteByte(recordTypeTimestamp)
			binio.WriteUint24(buf, uint32(t/time.Second))
		}

		for _, rec := range records[i:j] {
			if rec.Callsign != currentState.currentController {
				writeControllerChangeRecord(
					buf,
					rec.Callsign,
					callsignIDs,
					currentState,
				)
			}

			writeGenericRecord(buf, rec, aircraftIDs, aircraftStates)
		}

		i = j
		prevTime = t
	}

	buf.WriteByte(stop)
}

func writeControllerChangeRecord(
	buf *bytes.Buffer,
	callsign string,
	callsignIDs map[string]uint8,
	currentState *universalState,
) {
	callsignID := callsignIDs[callsign]

	currentState.currentController = callsign

	buf.WriteByte(recordTypeController)
	buf.WriteByte(callsignID)
}

func writeGenericRecord(
	buf *bytes.Buffer,
	rec record.GenericRecord,
	aircraftIDs map[string]uint16,
	last map[uint16]aircraftState,
) {
	switch r := rec.Record.(type) {
	case record.PositionRecord:
		writePositionRecord(buf, r, aircraftIDs, last)
	// case record.MessageRecord:
	// 	writeMessageRecord(buf, r, aircraftIDs)
	case record.UnknownRecord:
		writeUnknownRecord(buf, r)
	default:
		panic(fmt.Sprintf("unknown record type: %+v", r))
	}
}
