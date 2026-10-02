package esc

import (
	"bytes"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

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

	binio.WriteUvarint(buf, uint32(aircraftID))

	binio.WriteUint16(buf, state.Squawk)
	binio.WriteInt32(buf, state.Latitude)
	binio.WriteInt32(buf, state.Longitude)
	binio.WriteUint16(buf, state.Altitude)
	binio.WriteUint16(buf, state.Heading)
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

	binio.WriteUvarint(buf, uint32(aircraftID))

	if d.changeMap&changeTransponderType != 0 {
		if d.isNormalMode {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	}

	if d.changeMap&changeSquawk != 0 {
		binio.WriteUint16(buf, state.Squawk)
	}

	if d.changeMap&changeLat != 0 {
		binio.WriteVarint(buf, d.lat)
	}

	if d.changeMap&changeLon != 0 {
		binio.WriteVarint(buf, d.lon)
	}

	if d.changeMap&changeAlt != 0 {
		binio.WriteVarint(buf, d.alt)
	}

	if d.changeMap&changeHdg != 0 {
		binio.WriteVarint(buf, d.hdg)
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
