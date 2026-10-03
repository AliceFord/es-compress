package esc

import (
	"bytes"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func writeMessageRecord(buf *bytes.Buffer, r record.MessageRecord, textIDs map[string]uint16) {
	buf.WriteByte(recordTypeMessage)
	binio.WriteUvarint(buf, uint32(textIDs[r.Sender]))
	binio.WriteUvarint(buf, uint32(textIDs[r.Receiver]))
	buf.WriteString(r.Message)
	buf.WriteByte(stop)
}
