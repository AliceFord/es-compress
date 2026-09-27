package exporter

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"

	"github.com/AliceFord/es-compress/importer"
)

const (
	version            byte = 0
	recordTypePosition byte = 0
	stop                    = 0
)

func Export(records []importer.GenericRecord) []byte {
	var buf bytes.Buffer

	buf.WriteString("esco")
	buf.WriteByte(version)

	callsignIDs, aircraftIDs := writeRegistries(&buf, records)
	writeRecordStream(&buf, records, callsignIDs, aircraftIDs)

	return buf.Bytes()
}

func writeRegistries(buf *bytes.Buffer, records []importer.GenericRecord) (map[string]uint8, map[string]uint16) {
	callsigns := make([]string, 0)
	callsignIDs := make(map[string]uint8)
	aircraft := make([]string, 0)
	aircraftIDs := make(map[string]uint16)

	for _, rec := range records {
		if _, ok := callsignIDs[rec.Callsign]; !ok {
			callsignIDs[rec.Callsign] = uint8(len(callsigns))
			callsigns = append(callsigns, rec.Callsign)
		}

		pos, ok := rec.Record.(importer.PositionRecord)
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

func writeRecordStream(buf *bytes.Buffer, records []importer.GenericRecord, callsignIDs map[string]uint8, aircraftIDs map[string]uint16) {
	for i := 0; i < len(records); {
		t := records[i].Time
		j := i
		for j < len(records) && records[j].Time == t {
			j++
		}

		wroteHeader := false
		for _, rec := range records[i:j] {
			pos, ok := rec.Record.(importer.PositionRecord)
			if !ok {
				continue
			}

			if !wroteHeader {
				writeUint24(buf, uint32(t/time.Second))
				wroteHeader = true
			}

			writePositionRecord(buf, rec.Callsign, pos, callsignIDs, aircraftIDs)
		}

		if wroteHeader {
			buf.WriteByte(stop)
		}

		i = j
	}
	buf.WriteByte(stop)
}

func writePositionRecord(buf *bytes.Buffer, positionCallsign string, pos importer.PositionRecord, callsignIDs map[string]uint8, aircraftIDs map[string]uint16) {
	buf.WriteByte(recordTypePosition)

	transponder := byte(0)
	if pos.IsNormalMode {
		transponder = 1
	}

	buf.WriteByte(transponder)
	buf.WriteByte(callsignIDs[positionCallsign])
	writeUint16(buf, aircraftIDs[pos.Callsign])
	writeUint16(buf, uint16(pos.Squawk))
	writeFloat32(buf, float32(pos.Latitude))
	writeFloat32(buf, float32(pos.Longitude))
	writeUint16(buf, uint16(pos.Altitude))
	writeUint16(buf, uint16(pos.Heading))
	buf.WriteByte(stop)
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

func writeFloat32(buf *bytes.Buffer, v float32) {
	var b [4]byte

	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	buf.Write(b[:])
}
