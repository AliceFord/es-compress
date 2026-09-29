package esc

import (
	"bytes"
	"encoding/binary"
	"time"

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
			writeUint24(buf, uint32(t/time.Second))
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

			pos, ok := rec.Record.(record.PositionRecord)
			if !ok {
				continue
			}

			writePositionRecord(
				buf,
				pos,
				aircraftIDs,
				aircraftStates,
			)
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

func writePositionRecord(
	buf *bytes.Buffer,
	pos record.PositionRecord,
	aircraftIDs map[string]uint16,
	last map[uint16]aircraftState,
) {
	aircraftID := aircraftIDs[pos.Callsign]
	next := stateFrom(pos)

	prev, seen := last[aircraftID]
	if seen {
		delta := makeDelta(prev, next)

		writeDeltaRecord(
			buf,
			aircraftID,
			next,
			delta,
		)

		last[aircraftID] = applyDelta(prev, delta)
		return
	}

	writeFullPositionRecord(buf, aircraftID, next)
	last[aircraftID] = next
}

func writeFullPositionRecord(
	buf *bytes.Buffer,
	aircraftID uint16,
	state aircraftState,
) {
	buf.WriteByte(recordTypePosition)

	transponder := byte(0)
	if state.IsNormalMode {
		transponder = 1
	}

	buf.WriteByte(transponder)

	writeUvarint(buf, uint32(aircraftID))

	writeUint16(buf, state.Squawk)
	writeInt32(buf, state.Latitude)
	writeInt32(buf, state.Longitude)
	writeUint16(buf, state.Altitude)
	writeUint16(buf, state.Heading)
}

type positionDelta struct {
	changeMap    byte
	isNormalMode bool
	squawk       uint16
	lat          int32
	lon          int32
	alt          int32
	hdg          int32
}

func makeDelta(
	prev aircraftState,
	next aircraftState,
) positionDelta {
	var d positionDelta

	if next.IsNormalMode != prev.IsNormalMode {
		d.changeMap |= changeTransponderType
		d.isNormalMode = next.IsNormalMode
	}

	if next.Squawk != prev.Squawk {
		d.changeMap |= changeSquawk
		d.squawk = next.Squawk
	}

	d.lat = next.Latitude - prev.Latitude
	if d.lat != 0 {
		d.changeMap |= changeLat
	}

	d.lon = next.Longitude - prev.Longitude
	if d.lon != 0 {
		d.changeMap |= changeLon
	}

	d.alt = int32(next.Altitude) - int32(prev.Altitude)
	if d.alt != 0 {
		d.changeMap |= changeAlt
	}

	d.hdg = int32(next.Heading) - int32(prev.Heading)
	if d.hdg != 0 {
		d.changeMap |= changeHdg
	}

	return d
}

func deltaRecordType(changeMap byte) (recordType byte, specialized bool) {
	switch changeMap {
	case 0:
		return recordTypeDeltaNone, true

	case changeLat | changeLon:
		return recordTypeDeltaLatLon, true

	case changeLat | changeLon | changeAlt:
		return recordTypeDeltaLatLonAlt, true

	case changeLat | changeLon | changeAlt | changeHdg:
		return recordTypeDeltaLatLonAltHdg, true

	case changeLat | changeLon | changeHdg:
		return recordTypeDeltaLatLonHdg, true

	default:
		return recordTypeDelta, false
	}
}

func writeDeltaRecord(
	buf *bytes.Buffer,
	aircraftID uint16,
	state aircraftState,
	d positionDelta,
) {
	recordType, specialized := deltaRecordType(d.changeMap)

	buf.WriteByte(recordType)

	if !specialized {
		buf.WriteByte(d.changeMap)
	}

	writeUvarint(buf, uint32(aircraftID))

	if d.changeMap&changeTransponderType != 0 {
		if d.isNormalMode {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	}

	if d.changeMap&changeSquawk != 0 {
		writeUint16(buf, state.Squawk)
	}

	if d.changeMap&changeLat != 0 {
		writeVarint(buf, d.lat)
	}

	if d.changeMap&changeLon != 0 {
		writeVarint(buf, d.lon)
	}

	if d.changeMap&changeAlt != 0 {
		writeVarint(buf, d.alt)
	}

	if d.changeMap&changeHdg != 0 {
		writeVarint(buf, d.hdg)
	}
}

func applyDelta(
	prev aircraftState,
	d positionDelta,
) aircraftState {
	out := prev

	if d.changeMap&changeTransponderType != 0 {
		out.IsNormalMode = d.isNormalMode
	}

	if d.changeMap&changeSquawk != 0 {
		out.Squawk = d.squawk
	}

	if d.changeMap&changeLat != 0 {
		out.Latitude += d.lat
	}

	if d.changeMap&changeLon != 0 {
		out.Longitude += d.lon
	}

	if d.changeMap&changeAlt != 0 {
		out.Altitude = uint16(
			int32(out.Altitude) + d.alt,
		)
	}

	if d.changeMap&changeHdg != 0 {
		out.Heading = uint16(
			int32(out.Heading) + d.hdg,
		)
	}

	return out
}

func stateFrom(pos record.PositionRecord) aircraftState {
	return aircraftState{
		IsNormalMode: pos.IsNormalMode,
		Squawk:       uint16(pos.Squawk),
		Latitude:     int32(pos.Latitude * 100000),
		Longitude:    int32(pos.Longitude * 100000),
		Altitude:     uint16(pos.Altitude),
		Heading:      uint16(pos.Heading),
	}
}

func writeInt32(buf *bytes.Buffer, v int32) {
	binary.Write(buf, binary.LittleEndian, v)
}

func writeUint16(buf *bytes.Buffer, v uint16) {
	var b [2]byte

	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func writeUint24(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
}

func writeVarint(buf *bytes.Buffer, v int32) {
	var b [binary.MaxVarintLen32]byte
	n := binary.PutVarint(b[:], int64(v))
	buf.Write(b[:n])
}

func writeUvarint(buf *bytes.Buffer, v uint32) {
	var b [binary.MaxVarintLen32]byte
	n := binary.PutUvarint(b[:], uint64(v))
	buf.Write(b[:n])
}
