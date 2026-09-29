package esc

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
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

func readVarint(r io.Reader) (int32, error) {
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

func readUvarint(r io.Reader) (uint16, error) {
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

func (p *Parser) parseTimestampRecord(r io.Reader) error {
	timestamp, err := readBytes(r, 3)
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
		if err := p.parseTimestampRecord(r); err != nil {
			return record.GenericRecord{}, err
		}

		return record.GenericRecord{}, nil
	case 4:
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
	transponderType, err := readBytes(r, 1)
	if err != nil {
		return record.PositionRecord{}, err
	}

	isNormalMode := transponderType[0]&1 == 1

	aircraftId, err := readBytes(r, 2)
	if err != nil {
		return record.PositionRecord{}, err
	}

	aircraftIDValue := binary.LittleEndian.Uint16(aircraftId)

	callsign, ok := p.aircraftMap[aircraftIDValue]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"unknown aircraft: %d",
			aircraftIDValue,
		)
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
	latRaw := int32(binary.LittleEndian.Uint32(lat))
	latValue := float64(latRaw) / 100000.0

	lon, err := readBytes(r, 4)
	if err != nil {
		return record.PositionRecord{}, err
	}
	lonRaw := int32(binary.LittleEndian.Uint32(lon))
	lonValue := float64(lonRaw) / 100000.0

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

func (p *Parser) parsePositionRecordDelta(r io.Reader) (record.PositionRecord, error) {
	changeMap, err := readBytes(r, 1)
	if err != nil {
		return record.PositionRecord{}, err
	}
	changeMapValue := changeMap[0]

	aircraftId, err := readUvarint(r)
	if err != nil {
		return record.PositionRecord{}, err
	}

	callsign, ok := p.aircraftMap[aircraftId]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"unknown aircraft: %d",
			aircraftId,
		)
	}

	acState, ok := p.aircraftStates[callsign]
	if !ok {
		return record.PositionRecord{}, fmt.Errorf(
			"no previous state for aircraft: %d",
			aircraftId,
		)
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
		delta, err := readVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading latitude delta: %w",
				err,
			)
		}

		acState.Latitude += delta
	}

	if changeMapValue&changeLon != 0 {
		delta, err := readVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading longitude delta: %w",
				err,
			)
		}

		acState.Longitude += delta
	}

	if changeMapValue&changeAlt != 0 {
		delta, err := readVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading altitude delta: %w",
				err,
			)
		}

		acState.Altitude = uint16(int32(acState.Altitude) + delta)
	}

	if changeMapValue&changeHdg != 0 {
		delta, err := readVarint(r)
		if err != nil {
			return record.PositionRecord{}, fmt.Errorf(
				"reading heading delta: %w",
				err,
			)
		}

		acState.Heading = uint16(int32(acState.Heading) + delta)
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
