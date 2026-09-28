package esc

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"time"

	"github.com/AliceFord/es-compress/record"
)

const (
	version              byte = 0
	recordTypePosition   byte = 0
	recordTypeDelta      byte = 1
	recordTypeController byte = 2
	stop                      = 0

	changeTransponderType byte = 1 << 0
	changeSquawk          byte = 1 << 1
	changeLat             byte = 1 << 2
	changeLon             byte = 1 << 3
	changeAlt             byte = 1 << 4
	changeHdg             byte = 1 << 5

	coordDeltaUnit = 1e-4 // degrees per signed lat/lon delta unit
)

type universalState struct {
	currentController string
}

type aircraftState struct {
	IsNormalMode bool
	Squawk       uint16
	Latitude     float32
	Longitude    float32
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

func writeRegistries(buf *bytes.Buffer, records []record.GenericRecord) (map[string]uint8, map[string]uint16) {
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

func writeRecordStream(buf *bytes.Buffer, records []record.GenericRecord, callsignIDs map[string]uint8, aircraftIDs map[string]uint16) {
	// Aircraft states
	aircraftStates := make(map[uint16]aircraftState)

	// Universal state
	currentState := new(universalState)

	for i := 0; i < len(records); {
		t := records[i].Time
		j := i
		// Handle all records with same timestamp in one
		for j < len(records) && records[j].Time == t {
			j++
		}

		// Write time header
		writeUint24(buf, uint32(t/time.Second))

		for _, rec := range records[i:j] {
			// Write controller change record if necessary
			if rec.Callsign != currentState.currentController {
				writeControllerChangeRecord(buf, rec.Callsign, callsignIDs, currentState)
			}

			pos, ok := rec.Record.(record.PositionRecord)
			if !ok {
				continue
			}

			writePositionRecord(buf, pos, aircraftIDs, aircraftStates)
		}

		// Write timestamp [stop] byte
		buf.WriteByte(stop)

		i = j
	}

	buf.WriteByte(stop)
}

func writeControllerChangeRecord(buf *bytes.Buffer, callsign string, callsignIDs map[string]uint8, currentState *universalState) {
	callsignId := callsignIDs[callsign]

	currentState.currentController = callsign

	buf.WriteByte(recordTypeController)
	buf.WriteByte(callsignId)
}

func writePositionRecord(buf *bytes.Buffer, pos record.PositionRecord, aircraftIDs map[string]uint16, last map[uint16]aircraftState) {
	aircraftID := aircraftIDs[pos.Callsign]
	next := stateFrom(pos)

	prev, seen := last[aircraftID]
	if seen {
		delta, ok := makeDelta(prev, next)
		if ok {
			writeDeltaRecord(buf, aircraftID, next, delta)
			last[aircraftID] = applyDelta(prev, next, delta)
			return
		}

		log.Printf("aircraft %s (id %d): delta exceeds range, writing type 0", pos.Callsign, aircraftID)
	}

	writeFullPositionRecord(buf, aircraftID, next)
	last[aircraftID] = next
}

func writeFullPositionRecord(buf *bytes.Buffer, aircraftID uint16, state aircraftState) {
	buf.WriteByte(recordTypePosition)

	transponder := byte(0)
	if state.IsNormalMode {
		transponder = 1
	}

	buf.WriteByte(transponder)
	writeUint16(buf, aircraftID)
	writeUint16(buf, state.Squawk)
	writeFloat32(buf, state.Latitude)
	writeFloat32(buf, state.Longitude)
	writeUint16(buf, state.Altitude)
	writeUint16(buf, state.Heading)
}

type positionDelta struct {
	changeMap    byte
	isNormalMode bool
	squawk       uint16
	lat          int16
	lon          int16
	alt          int16
	hdg          int8
}

func makeDelta(prev aircraftState, next aircraftState) (positionDelta, bool) {
	var d positionDelta

	if next.IsNormalMode != prev.IsNormalMode {
		d.changeMap |= changeTransponderType
		d.isNormalMode = next.IsNormalMode
	}

	if next.Squawk != prev.Squawk {
		d.changeMap |= changeSquawk
		d.squawk = next.Squawk
	}

	lat, ok := coordDelta(next.Latitude - prev.Latitude)
	if !ok {
		log.Printf("lat")
		return positionDelta{}, false
	}

	if lat != 0 {
		d.changeMap |= changeLat
		d.lat = lat
	}

	lon, ok := coordDelta(next.Longitude - prev.Longitude)
	if !ok {
		log.Printf("lon")
		return positionDelta{}, false
	}

	if lon != 0 {
		d.changeMap |= changeLon
		d.lon = lon
	}

	alt, ok := int16Delta(int(next.Altitude) - int(prev.Altitude))
	if !ok {
		log.Printf("alt")
		return positionDelta{}, false
	}

	if alt != 0 {
		d.changeMap |= changeAlt
		d.alt = alt
	}

	hdg, ok := int8Delta(int(next.Heading) - int(prev.Heading))
	if !ok {
		return positionDelta{}, false
	}

	if hdg != 0 {
		d.changeMap |= changeHdg
		d.hdg = hdg
	}

	return d, true
}

func writeDeltaRecord(buf *bytes.Buffer, aircraftID uint16, state aircraftState, d positionDelta) {
	buf.WriteByte(recordTypeDelta)
	buf.WriteByte(d.changeMap)
	writeUint16(buf, aircraftID)

	if d.changeMap&changeTransponderType != 0 {
		if d.isNormalMode {
			buf.WriteByte(0b1)
		} else {
			buf.WriteByte(0b0)
		}
	}

	if d.changeMap&changeSquawk != 0 {
		writeUint16(buf, state.Squawk)
	}

	if d.changeMap&changeLat != 0 {
		writeInt16(buf, d.lat)
	}

	if d.changeMap&changeLon != 0 {
		writeInt16(buf, d.lon)
	}

	if d.changeMap&changeAlt != 0 {
		writeInt16(buf, d.alt)
	}

	if d.changeMap&changeHdg != 0 {
		buf.WriteByte(byte(d.hdg))
	}
}

func applyDelta(prev aircraftState, next aircraftState, d positionDelta) aircraftState {
	out := prev
	out.IsNormalMode = next.IsNormalMode

	if d.changeMap&changeSquawk != 0 {
		out.Squawk = d.squawk
	}

	if d.changeMap&changeTransponderType != 0 {
		out.IsNormalMode = d.isNormalMode
	}

	if d.changeMap&changeLat != 0 {
		out.Latitude += float32(d.lat) * coordDeltaUnit
	}

	if d.changeMap&changeLon != 0 {
		out.Longitude += float32(d.lon) * coordDeltaUnit
	}

	if d.changeMap&changeAlt != 0 {
		out.Altitude = uint16(int(out.Altitude) + int(d.alt))
	}

	if d.changeMap&changeHdg != 0 {
		out.Heading = uint16(int(out.Heading) + int(d.hdg))
	}

	return out
}

func stateFrom(pos record.PositionRecord) aircraftState {
	return aircraftState{
		IsNormalMode: pos.IsNormalMode,
		Squawk:       uint16(pos.Squawk),
		Latitude:     float32(pos.Latitude),
		Longitude:    float32(pos.Longitude),
		Altitude:     uint16(pos.Altitude),
		Heading:      uint16(pos.Heading),
	}
}

func coordDelta(diff float32) (int16, bool) {
	return int16Delta(int(math.Round(float64(diff) / coordDeltaUnit)))
}

func int8Delta(diff int) (int8, bool) {
	if diff < math.MinInt8 || diff > math.MaxInt8 {
		return 0, false
	}

	return int8(diff), true
}

func int16Delta(diff int) (int16, bool) {
	if diff < math.MinInt16 || diff > math.MaxInt16 {
		return 0, false
	}

	return int16(diff), true
}

func writeUint16(buf *bytes.Buffer, v uint16) {
	var b [2]byte

	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func writeInt16(buf *bytes.Buffer, v int16) {
	binary.Write(buf, binary.LittleEndian, v)
}

func writeUint24(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
}

func writeFloat32(buf *bytes.Buffer, v float32) {
	var b [4]byte

	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	buf.Write(b[:])
}
