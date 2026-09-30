package binio

import (
	"bytes"
	"encoding/binary"
)

func WriteInt32(buf *bytes.Buffer, v int32) {
	binary.Write(buf, binary.LittleEndian, v)
}

func WriteUint16(buf *bytes.Buffer, v uint16) {
	var b [2]byte

	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func WriteUint24(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
}

func WriteVarint(buf *bytes.Buffer, v int32) {
	var b [binary.MaxVarintLen32]byte
	n := binary.PutVarint(b[:], int64(v))
	buf.Write(b[:n])
}

func WriteUvarint(buf *bytes.Buffer, v uint32) {
	var b [binary.MaxVarintLen32]byte
	n := binary.PutUvarint(b[:], uint64(v))
	buf.Write(b[:n])
}
