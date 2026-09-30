package esc

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

const (
	recordTypePosition       byte = 0
	recordTypeDelta          byte = 1 // Generic delta with explicit change map
	recordTypeController     byte = 2
	recordTypeTimestamp      byte = 3
	recordTypeTimestampPlus1 byte = 4

	recordTypeDeltaNone         byte = 5
	recordTypeDeltaLatLon       byte = 6
	recordTypeDeltaLatLonAlt    byte = 7
	recordTypeDeltaLatLonAltHdg byte = 8
	recordTypeDeltaLatLonHdg    byte = 9

	changeTransponderType byte = 1 << 0
	changeSquawk          byte = 1 << 1
	changeLat             byte = 1 << 2
	changeLon             byte = 1 << 3
	changeAlt             byte = 1 << 4
	changeHdg             byte = 1 << 5
)

type aircraftState struct {
	IsNormalMode bool
	Squawk       uint16
	Latitude     int32
	Longitude    int32
	Altitude     uint16
	Heading      uint16
}

type Parser struct {
	Logger *slog.Logger

	currentTimestamp time.Duration
	currentCallsign  string
	aircraftStates   map[string]aircraftState
	callsignMap      map[uint8]string
	aircraftMap      map[uint16]string
}

func NewParser(logger *slog.Logger) *Parser {
	return &Parser{Logger: logger}
}

func (p *Parser) Parse(r io.Reader) ([]record.GenericRecord, error) {
	if err := p.parseHeader(r); err != nil {
		return nil, err
	}

	if err := p.parseCallsignRegistry(r); err != nil {
		return nil, err
	}

	if err := p.parseAircraftRegistry(r); err != nil {
		return nil, err
	}

	return p.parseRecordStream(r)
}

func (p *Parser) parseHeader(r io.Reader) error {
	magic, err := binio.ReadBytes(r, 4)
	if err != nil {
		return err
	}

	if string(magic) != "skog" {
		return fmt.Errorf("incorrect magic number: %s", magic)
	}

	version, err := binio.ReadBytes(r, 1)
	if err != nil {
		return err
	}

	p.Logger.Info("file version", "version", version[0])

	return nil
}

func (p *Parser) parseCallsignRegistry(r io.Reader) error {
	var callsignID uint8
	p.callsignMap = make(map[uint8]string)

	for {
		callsign, err := binio.ReadCString(r)
		if err != nil {
			return err
		}

		// Empty string == [stop] byte.
		if callsign == "" {
			return nil
		}

		p.callsignMap[callsignID] = callsign
		callsignID++
	}
}

func (p *Parser) parseAircraftRegistry(r io.Reader) error {
	var aircraftID uint16
	p.aircraftMap = make(map[uint16]string)

	for {
		callsign, err := binio.ReadCString(r)
		if err != nil {
			return err
		}

		// Empty string == [stop] byte.
		if callsign == "" {
			return nil
		}

		p.aircraftMap[aircraftID] = callsign
		aircraftID++
	}
}

func (p *Parser) parseRecordStream(r io.Reader) ([]record.GenericRecord, error) {
	var records []record.GenericRecord

	p.aircraftStates = make(map[string]aircraftState)

	for {
		genericRecord, err := p.parseGenericRecord(r)
		if err != nil {
			if err == io.EOF {
				return records, nil
			}

			return nil, err
		}

		// No callsign means no error, but no record either.
		if genericRecord.Callsign == "" {
			continue
		}

		records = append(records, genericRecord)
	}
}

func (p *Parser) parseTimestampRecord(r io.Reader) error {
	timestamp, err := binio.ReadBytes(r, 3)
	if err != nil {
		return err
	}

	p.currentTimestamp =
		time.Duration(timestamp[0]) |
			time.Duration(timestamp[1])<<8 |
			time.Duration(timestamp[2])<<16

	p.currentTimestamp *= time.Second

	return nil
}

func (p *Parser) parseTimestampPlus1Record() {
	p.currentTimestamp += time.Second
}

func (p *Parser) parseGenericRecord(r io.Reader) (record.GenericRecord, error) {
	recordType, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.GenericRecord{}, err
	}

	genericRecord := record.GenericRecord{
		Callsign: p.currentCallsign,
		Time:     p.currentTimestamp,
	}

	switch recordType[0] {
	case recordTypePosition:
		posRecord, err := p.parsePositionRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeDelta:
		changeMap, err := binio.ReadBytes(r, 1)
		if err != nil {
			return record.GenericRecord{}, err
		}

		posRecord, err := p.parsePositionRecordDelta(r, changeMap[0])
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeDeltaNone:
		posRecord, err := p.parsePositionRecordDelta(r, 0)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeDeltaLatLon:
		posRecord, err := p.parsePositionRecordDelta(
			r,
			changeLat|changeLon,
		)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeDeltaLatLonAlt:
		posRecord, err := p.parsePositionRecordDelta(
			r,
			changeLat|changeLon|changeAlt,
		)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeDeltaLatLonAltHdg:
		posRecord, err := p.parsePositionRecordDelta(
			r,
			changeLat|changeLon|changeAlt|changeHdg,
		)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeDeltaLatLonHdg:
		posRecord, err := p.parsePositionRecordDelta(
			r,
			changeLat|changeLon|changeHdg,
		)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil

	case recordTypeController:
		if err := p.parseControllerPositionChange(r); err != nil {
			return record.GenericRecord{}, err
		}

		return record.GenericRecord{}, nil

	case recordTypeTimestamp:
		if err := p.parseTimestampRecord(r); err != nil {
			return record.GenericRecord{}, err
		}

		return record.GenericRecord{}, nil

	case recordTypeTimestampPlus1:
		p.parseTimestampPlus1Record()

		return record.GenericRecord{}, nil

	case 0xFF:
		return record.GenericRecord{}, io.EOF

	default:
		return record.GenericRecord{}, fmt.Errorf(
			"unknown record type: %d",
			recordType[0],
		)
	}
}

func (p *Parser) parsePositionRecord(r io.Reader) (record.PositionRecord, error) {
	transponderType, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.PositionRecord{}, err
	}

	isNormalMode := transponderType[0]&1 == 1

	// Aircraft IDs are encoded as uvarints.
	aircraftID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf(
			"reading aircraft ID: %w",
			err,
		)
	}

	callsign, ok := p.aircraftMap[aircraftID]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"unknown aircraft: %d",
			aircraftID,
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
	aircraftID, err := binio.ReadUvarint(r)
	if err != nil {
		return record.PositionRecord{}, fmt.Errorf(
			"reading aircraft ID: %w",
			err,
		)
	}

	callsign, ok := p.aircraftMap[aircraftID]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"unknown aircraft: %d",
			aircraftID,
		)
	}

	acState, ok := p.aircraftStates[callsign]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"no previous state for aircraft: %d",
			aircraftID,
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

func (p *Parser) parseControllerPositionChange(r io.Reader) error {
	controllerID, err := binio.ReadBytes(r, 1)
	if err != nil {
		return err
	}

	p.currentCallsign = p.callsignMap[controllerID[0]]
	return nil
}
