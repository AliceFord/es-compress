package esc

import (
	"bytes"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func writeAddPilotRecord(buf *bytes.Buffer, r record.AddPilotRecord, textIDs map[string]uint16) {
	pilotId := textIDs[r.Callsign]

	buf.WriteByte(recordTypeAddPilot)
	binio.WriteUvarint(buf, uint32(pilotId))
	binio.WriteUint24(buf, r.CID)
	buf.WriteByte(r.Rating)
	buf.WriteString(r.Name)
	buf.WriteByte(stop)
}
