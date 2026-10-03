package esc

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func (p *Parser) parsePositionRecord(
	r io.Reader,
) (record.PositionRecord, error) {
	transponderType, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.PositionRecord{}, err
	}

	isNormalMode := transponderType[0]&1 == 1

	// Text IDs are encoded as uvarints in position records.
	textID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf(
			"reading aircraft text ID: %w",
			err,
		)
	}

	callsign, ok := p.textMap[uint16(textID)]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"unknown aircraft text ID: %d",
			textID,
		)
	}

	squawk, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	squawkValue := binary.LittleEndian.Uint16(squawk)

	lat, err := binio.ReadBytes(r, 4)
	if err != nil {
		return record.PositionRecord{}, err
	}
	latRaw := int32(binary.LittleEndian.Uint32(lat))
	latValue := float64(latRaw) / 100000.0

	lon, err := binio.ReadBytes(r, 4)
	if err != nil {
		return record.PositionRecord{}, err
	}
	lonRaw := int32(binary.LittleEndian.Uint32(lon))
	lonValue := float64(lonRaw) / 100000.0

	alt, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	altValue := binary.LittleEndian.Uint16(alt)

	hdg, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	hdgValue := binary.LittleEndian.Uint16(hdg)

	p.aircraftStates[callsign] = aircraftState{
		IsNormalMode: isNormalMode,
		Squawk:       squawkValue,
		Latitude:     latRaw,
		Longitude:    lonRaw,
		Altitude:     altValue,
		Heading:      hdgValue,
	}

	return record.PositionRecord{
		IsNormalMode: isNormalMode,
		Callsign:     callsign,
		Squawk:       squawkValue,
		Latitude:     latValue,
		Longitude:    lonValue,
		Altitude:     altValue,
		Heading:      hdgValue,
	}, nil
}

func (p *Parser) parsePositionRecordDelta(
	r io.Reader,
	changeMap byte,
) (record.PositionRecord, error) {
	textID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf(
			"reading aircraft text ID: %w",
			err,
		)
	}

	callsign, ok := p.textMap[uint16(textID)]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"unknown aircraft text ID: %d",
			textID,
		)
	}

	acState, ok := p.aircraftStates[callsign]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"no previous state for aircraft text ID: %d",
			textID,
		)
	}

	if changeMap&changeTransponderType != 0 {
		transponderType, err := binio.ReadBytes(r, 1)
		if err != nil {
			return record.PositionRecord{}, err
		}

		acState.IsNormalMode = transponderType[0]&1 == 1
	}

	if changeMap&changeSquawk != 0 {
		squawk, err := binio.ReadBytes(r, 2)
		if err != nil {
			return record.PositionRecord{}, err
		}

		acState.Squawk = binary.LittleEndian.Uint16(squawk)
	}

	if changeMap&changeLat != 0 {
		delta, err := binio.ReadVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading latitude delta: %w",
				err,
			)
		}

		acState.Latitude += delta
	}

	if changeMap&changeLon != 0 {
		delta, err := binio.ReadVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading longitude delta: %w",
				err,
			)
		}

		acState.Longitude += delta
	}

	if changeMap&changeAlt != 0 {
		delta, err := binio.ReadVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading altitude delta: %w",
				err,
			)
		}

		acState.Altitude = uint16(
			int32(acState.Altitude) + delta,
		)
	}

	if changeMap&changeHdg != 0 {
		delta, err := binio.ReadVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading heading delta: %w",
				err,
			)
		}

		acState.Heading = uint16(
			int32(acState.Heading) + delta,
		)
	}

	p.aircraftStates[callsign] = acState

	return record.PositionRecord{
		IsNormalMode: acState.IsNormalMode,
		Callsign:     callsign,
		Squawk:       acState.Squawk,
		Latitude:     float64(acState.Latitude) / 100000.0,
		Longitude:    float64(acState.Longitude) / 100000.0,
		Altitude:     acState.Altitude,
		Heading:      acState.Heading,
	}, nil
}
