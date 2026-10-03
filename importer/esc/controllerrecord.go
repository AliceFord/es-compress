package esc

import (
	"encoding/binary"
	"io"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func (p *Parser) parseControllerPositionRecord(r io.Reader) (record.ControllerPositionRecord, error) {
	controllerId, err := binio.ReadUvarint(r)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}
	controllerCallsign := p.textMap[uint16(controllerId)]

	frequencyBytes, err := binio.ReadBytes(r, 3)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}
	frequency := uint32(frequencyBytes[0]) | uint32(frequencyBytes[1])<<8 | uint32(frequencyBytes[2])<<16

	altitude, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}
	altitudeValue := uint16(altitude[0]) | uint16(altitude[1])<<8

	protocolVer, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}
	protocolVerValue := uint16(protocolVer[0]) | uint16(protocolVer[1])<<8

	ratingBytes, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}

	lat, err := binio.ReadBytes(r, 4)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}
	latRaw := int32(binary.LittleEndian.Uint32(lat))
	latValue := float64(latRaw) / 100000.0

	lon, err := binio.ReadBytes(r, 4)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}
	lonRaw := int32(binary.LittleEndian.Uint32(lon))
	lonValue := float64(lonRaw) / 100000.0

	rec := record.ControllerPositionRecord{
		Callsign:    controllerCallsign,
		Frequency:   frequency,
		Altitude:    altitudeValue,
		ProtocolVer: protocolVerValue,
		Rating:      ratingBytes[0],
		Lat:         latValue,
		Lon:         lonValue,
	}

	p.controllerStates[uint16(controllerId)] = rec

	return rec, nil
}

func (p *Parser) parseControllerPositionUnchangedRecord(r io.Reader) (record.ControllerPositionRecord, error) {
	controllerId, err := binio.ReadUvarint(r)
	if err != nil {
		return record.ControllerPositionRecord{}, err
	}

	rec, ok := p.controllerStates[uint16(controllerId)]
	if !ok {
		return record.ControllerPositionRecord{}, err
	}

	return rec, nil
}
