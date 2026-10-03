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

	recordTypeMessage  byte = 10
	recordTypeUnknown  byte = 11
	recordTypeAddPilot byte = 12

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

	textIDs := writeTextRegistry(&buf, records)
	writeRecordStream(&buf, records, textIDs)

	return buf.Bytes()
}

func writeTextRegistry(
	buf *bytes.Buffer,
	records []record.GenericRecord,
) map[string]uint16 {
	texts := make([]string, 0)
	textIDs := make(map[string]uint16)

	addText := func(text string) {
		if _, ok := textIDs[text]; ok {
			return
		}

		textIDs[text] = uint16(len(texts))
		texts = append(texts, text)
	}

	for _, rec := range records {
		// Controller/source callsign.
		addText(rec.Callsign)

		// Aircraft callsign.
		if pos, ok := rec.Record.(record.PositionRecord); ok {
			addText(pos.Callsign)
		}

		// Sender and receiver callsigns for messages.
		if pos, ok := rec.Record.(record.MessageRecord); ok {
			addText(pos.Sender)
			addText(pos.Receiver)
		}

		// Pilot callsign
		if pos, ok := rec.Record.(record.AddPilotRecord); ok {
			addText(pos.Callsign)
		}
	}

	writeTextList(buf, texts)

	return textIDs
}

func writeTextList(buf *bytes.Buffer, texts []string) {
	for _, text := range texts {
		buf.WriteString(text)
		buf.WriteByte(stop)
	}

	buf.WriteByte(stop)
}

func writeRecordStream(
	buf *bytes.Buffer,
	records []record.GenericRecord,
	textIDs map[string]uint16,
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
					textIDs,
					currentState,
				)
			}

			writeGenericRecord(buf, rec, textIDs, aircraftStates)
		}

		i = j
		prevTime = t
	}

	buf.WriteByte(stop)
}

func writeControllerChangeRecord(
	buf *bytes.Buffer,
	callsign string,
	textIDs map[string]uint16,
	currentState *universalState,
) {
	textID := textIDs[callsign]

	currentState.currentController = callsign

	buf.WriteByte(recordTypeController)
	binio.WriteUint16(buf, textID)
}

func writeGenericRecord(
	buf *bytes.Buffer,
	rec record.GenericRecord,
	textIDs map[string]uint16,
	last map[uint16]aircraftState,
) {
	switch r := rec.Record.(type) {
	case record.PositionRecord:
		writePositionRecord(buf, r, textIDs, last)
	case record.MessageRecord:
		writeMessageRecord(buf, r, textIDs)
	case record.UnknownRecord:
		writeUnknownRecord(buf, r)
	case record.AddPilotRecord:
		writeAddPilotRecord(buf, r, textIDs)
	default:
		panic(fmt.Sprintf("unknown record type: %+v", r))
	}
}
