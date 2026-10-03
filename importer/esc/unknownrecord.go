package esc

import (
	"io"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

var byteToArrowType = map[byte]string{
	0: ">>>>",
	1: "<<<2",
	2: "2>>1",
}

func (p *Parser) parseUnknownRecord(r io.Reader) (record.UnknownRecord, error) {
	arrowType, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.UnknownRecord{}, err
	}

	arrowTypeStr, ok := byteToArrowType[arrowType[0]]
	if !ok {
		return record.UnknownRecord{}, err
	}

	rawPacket, err := binio.ReadCString(r)
	if err != nil {
		return record.UnknownRecord{}, err
	}

	return record.UnknownRecord{
		ArrowType: arrowTypeStr,
		Raw:       rawPacket,
	}, nil
}
