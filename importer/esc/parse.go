package esc

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/AliceFord/es-compress/record"
)

const (
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

func readBytes(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)

	return buf, err
}

func readCString(r io.Reader) (string, error) {
	var buf []byte

	for {
		b, err := readBytes(r, 1)
		if err != nil {
			return "", err
		}

		if b[0] == 0 {
			return string(buf), nil
		}

		buf = append(buf, b[0])
	}
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
	magic, err := readBytes(r, 4)
	if err != nil {
		return err
	}

	if string(magic) != "skog" {
		return fmt.Errorf("incorrect magic number: %s", magic)
	}

	version, err := readBytes(r, 1)
	if err != nil {
		return err
	}

	p.Logger.Info("file version", "version", version[0])

	return nil
}

func (p *Parser) parseCallsignRegistry(r io.Reader) error {
	var callsignId uint8
	p.callsignMap = make(map[uint8]string)

	for {
		callsign, err := readCString(r)
		if err != nil {
			return err
		}

		// Empty string == [stop] byte
		if callsign == "" {
			return nil
		}

		p.callsignMap[callsignId] = callsign
		callsignId++
	}
}

func (p *Parser) parseAircraftRegistry(r io.Reader) error {
	var aircraftId uint16
	p.aircraftMap = make(map[uint16]string)

	for {
		callsign, err := readCString(r)
		if err != nil {
			return err
		}

		// Empty string == [stop] byte
		if callsign == "" {
			return nil
		}

		p.aircraftMap[aircraftId] = callsign
		aircraftId++
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

		// For now we use no callsign <--> no error but no record either.
		if genericRecord.Callsign == "" {
			continue
		}

		records = append(records, genericRecord)
	}
}

func (p *Parser) parseTimestampRecord(r io.Reader) (time.Duration, error) {
	timestamp, err := readBytes(r, 3)
	if err != nil {
		return 0, err
	}

	return time.Duration(timestamp[0])<<16 | time.Duration(timestamp[1])<<8 | time.Duration(timestamp[2]), nil
}

func (p *Parser) parseGenericRecord(r io.Reader) (record.GenericRecord, error) {
	recordType, err := readBytes(r, 1)
	if err != nil {
		return record.GenericRecord{}, err
	}

	genericRecord := record.GenericRecord{
		Callsign: p.currentCallsign,
		Time:     p.currentTimestamp,
	}

	switch recordType[0] {
	case 0:
		posRecord, err := p.parsePositionRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil
	case 1:
		posRecord, err := p.parsePositionRecordDelta(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = posRecord
		return genericRecord, nil
	case 2:
		err := p.parseControllerPositionChange(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		return record.GenericRecord{}, err
	case 3:
		newTimestamp, err := p.parseTimestampRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		p.currentTimestamp = newTimestamp
		return record.GenericRecord{}, nil
	case 0xFF:
		return record.GenericRecord{}, io.EOF
	default:
		return record.GenericRecord{}, fmt.Errorf("unknown record type: %d", recordType[0])
	}
}

func (p *Parser) parsePositionRecord(r io.Reader) (record.PositionRecord, error) {
	transponderType, err := readBytes(r, 1)
	if err != nil {
		return record.PositionRecord{}, err
	}
	isNormalMode := transponderType[0]&1 == 1

	aircraftId, err := readBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}

	callsign, ok := p.aircraftMap[binary.LittleEndian.Uint16(aircraftId)]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf("unknown aircraft: %d", binary.LittleEndian.Uint16(aircraftId))
	}

	squawk, err := readBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	squawkValue := binary.LittleEndian.Uint16(squawk)

	lat, err := readBytes(r, 4)
	if err != nil {
		return record.PositionRecord{}, err
	}
	latValue := float64(int32(binary.LittleEndian.Uint32(lat))) / 100000.0

	lon, err := readBytes(r, 4)
	if err != nil {
		return record.PositionRecord{}, err
	}
	lonValue := float64(int32(binary.LittleEndian.Uint32(lon))) / 100000.0

	alt, err := readBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	altValue := binary.LittleEndian.Uint16(alt)

	hdg, err := readBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	hdgValue := binary.LittleEndian.Uint16(hdg)

	p.aircraftStates[callsign] = aircraftState{
		IsNormalMode: isNormalMode,
		Squawk:       squawkValue,
		Latitude:     int32(binary.LittleEndian.Uint32(lat)),
		Longitude:    int32(binary.LittleEndian.Uint32(lon)),
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

func (p *Parser) parsePositionRecordDelta(r io.Reader) (record.PositionRecord, error) {
	changeMap, err := readBytes(r, 1)
	if err != nil {
		return record.PositionRecord{}, err
	}
	changeMapValue := changeMap[0]

	aircraftId, err := readBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}
	callsign := p.aircraftMap[binary.LittleEndian.Uint16(aircraftId)]

	acState, ok := p.aircraftStates[callsign]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf("unknown aircraft: %d", binary.LittleEndian.Uint16(aircraftId))
	}

	if changeMapValue&changeTransponderType != 0 {
		transponderType, err := readBytes(r, 1)
		if err != nil {
			return record.PositionRecord{}, err
		}
		acState.IsNormalMode = transponderType[0]&1 == 1
	}

	if changeMapValue&changeSquawk != 0 {
		squawk, err := readBytes(r, 2)
		if err != nil {
			return record.PositionRecord{}, err
		}
		acState.Squawk = binary.LittleEndian.Uint16(squawk)
	}

	if changeMapValue&changeLat != 0 {
		latDelta, err := readBytes(r, 2)
		if err != nil {
			return record.PositionRecord{}, err
		}
		latDeltaValue := int16(binary.LittleEndian.Uint16(latDelta))

		acState.Latitude += int32(latDeltaValue)
	}

	if changeMapValue&changeLon != 0 {
		lonDelta, err := readBytes(r, 2)
		if err != nil {
			return record.PositionRecord{}, err
		}
		lonDeltaValue := int16(binary.LittleEndian.Uint16(lonDelta))

		acState.Longitude += int32(lonDeltaValue)
	}

	if changeMapValue&changeAlt != 0 {
		altDelta, err := readBytes(r, 2)
		if err != nil {
			return record.PositionRecord{}, err
		}
		altDeltaValue := int16(binary.LittleEndian.Uint16(altDelta))

		acState.Altitude = uint16(int(acState.Altitude) + int(altDeltaValue))
	}

	if changeMapValue&changeHdg != 0 {
		hdgDelta, err := readBytes(r, 1)
		if err != nil {
			return record.PositionRecord{}, err
		}
		hdgDeltaValue := int8(hdgDelta[0])

		acState.Heading = uint16(int(acState.Heading) + int(hdgDeltaValue))
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
	controllerId, err := readBytes(r, 1)
	if err != nil {
		return err
	}

	p.currentCallsign = p.callsignMap[controllerId[0]]
	return nil
}
