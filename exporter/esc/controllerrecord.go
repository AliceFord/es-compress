package esc

import (
	"bytes"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func writeControllerPositionPacket(
	buf *bytes.Buffer,
	r record.ControllerPositionRecord,
	textIDs map[string]uint16,
	controllerPositions *map[uint16]record.ControllerPositionRecord,
) {
	controllerId := textIDs[r.Callsign]

	if prev, ok := (*controllerPositions)[controllerId]; ok {
		if prev == r {
			buf.WriteByte(recordTypeControllerPositionUnchanged)
			binio.WriteUvarint(buf, uint32(controllerId))
			return
		}
	}

	buf.WriteByte(recordTypeControllerPosition)
	binio.WriteUvarint(buf, uint32(controllerId))
	binio.WriteUint24(buf, r.Frequency)
	binio.WriteUint16(buf, r.Altitude)
	binio.WriteUint16(buf, r.ProtocolVer)
	buf.WriteByte(r.Rating)
	binio.WriteInt32(buf, int32(r.Lat*100000))
	binio.WriteInt32(buf, int32(r.Lon*100000))

	(*controllerPositions)[controllerId] = r
}
