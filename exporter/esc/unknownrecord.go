package esc

import (
	"bytes"

	"github.com/AliceFord/es-compress/record"
)

var arrowTypeToByte = map[string]byte{
	">>>>": 0,
	"<<<2": 1,
	"2>>1": 2,
}

func writeUnknownRecord(buf *bytes.Buffer, r record.UnknownRecord) {
	buf.WriteByte(recordTypeUnknown)
	buf.WriteByte(arrowTypeToByte[r.ArrowType])
	buf.WriteString(r.Raw)
	buf.WriteByte(stop)
}
