package binio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

func ReadBytes(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)

	return buf, err
}

func ReadVarint(r io.Reader) (int32, error) {
	var br io.ByteReader

	if b, ok := r.(io.ByteReader); ok {
		br = b
	} else {
		br = &byteReader{r: r}
	}

	v, err := binary.ReadVarint(br)
	if err != nil {
		return 0, err
	}

	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("varint out of int32 range: %d", v)
	}

	return int32(v), nil
}

func ReadUvarint(r io.Reader) (uint16, error) {
	var br io.ByteReader

	if b, ok := r.(io.ByteReader); ok {
		br = b
	} else {
		br = &byteReader{r: r}
	}

	v, err := binary.ReadUvarint(br)
	if err != nil {
		return 0, err
	}

	if v > math.MaxUint16 {
		return 0, fmt.Errorf("varint out of uint16 range: %d", v)
	}

	return uint16(v), nil
}

type byteReader struct {
	r io.Reader
}

func (r *byteReader) ReadByte() (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(r.r, b[:])
	return b[0], err
}

func ReadCString(r io.Reader) (string, error) {
	var buf []byte

	for {
		b, err := ReadBytes(r, 1)
		if err != nil {
			return "", err
		}

		if b[0] == 0 {
			return string(buf), nil
		}

		buf = append(buf, b[0])
	}
}
