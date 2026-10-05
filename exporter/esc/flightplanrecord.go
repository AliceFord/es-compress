package esc

import (
	"bytes"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func writeFlightplanRecord(buf *bytes.Buffer, r record.FlightplanRecord, textIDs map[string]uint16) {
	pilotId := textIDs[r.Callsign]
	aircraftTypeId := textIDs[r.AircraftType]
	detailsId := textIDs[r.Details]
	routeId := textIDs[r.Route]

	buf.WriteByte(recordTypeFlightplan)
	binio.WriteUvarint(buf, uint32(pilotId))
	buf.WriteByte(r.FlightRules)
	binio.WriteUvarint(buf, uint32(aircraftTypeId))
	binio.WriteUint16(buf, r.Speed)
	buf.WriteString(r.Departure)
	buf.WriteByte(stop)
	binio.WriteUint16(buf, uint16(r.OffblocksTime.Hours()*60))
	binio.WriteUint16(buf, r.CruiseAlt)
	buf.WriteString(r.Arrival)
	buf.WriteByte(stop)
	binio.WriteUint16(buf, uint16(r.EnrouteTime.Hours()*60))
	binio.WriteUint16(buf, uint16(r.EnrouteFuel.Hours()*60))
	buf.WriteString(r.Alternate)
	buf.WriteByte(stop)
	binio.WriteUvarint(buf, uint32(detailsId))
	binio.WriteUvarint(buf, uint32(routeId))
}
