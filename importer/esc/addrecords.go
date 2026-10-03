package esc

import (
	"io"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func (p *Parser) parseAddPilotRecord(r io.Reader) (record.AddPilotRecord, error) {
	textID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.AddPilotRecord{}, err
	}

	callsign, ok := p.textMap[uint16(textID)]
	if !ok {
		return record.AddPilotRecord{}, err
	}

	cid, err := binio.ReadBytes(r, 3)
	if err != nil {
		return record.AddPilotRecord{}, err
	}

	cidValue := uint32(cid[0]) | uint32(cid[1])<<8 | uint32(cid[2])<<16

	ratingBytes, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.AddPilotRecord{}, err
	}

	rating := ratingBytes[0]

	name, err := binio.ReadCString(r)
	if err != nil {
		return record.AddPilotRecord{}, err
	}

	return record.AddPilotRecord{
		Callsign: callsign,
		CID:      cidValue,
		Rating:   rating,
		Name:     name,
	}, nil
}
