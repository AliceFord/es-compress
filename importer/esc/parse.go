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

	recordTypeMessage  byte = 10
	recordTypeUnknown  byte = 11
	recordTypeAddPilot byte = 12

	recordTypeControllerPosition          byte = 16
	recordTypeControllerPositionUnchanged byte = 17
	recordTypeFlightplan                  byte = 18

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
	controllerStates map[uint16]record.ControllerPositionRecord

	textMap map[uint16]string
}

func NewParser(logger *slog.Logger) *Parser {
	return &Parser{Logger: logger}
}

func (p *Parser) Parse(r io.Reader) ([]record.GenericRecord, error) {
	if err := p.parseHeader(r); err != nil {
		return nil, err
	}

	if err := p.parseTextRegistry(r); err != nil {
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

func (p *Parser) parseTextRegistry(r io.Reader) error {
	var textID uint16

	p.textMap = make(map[uint16]string)

	for {
		text, err := binio.ReadCString(r)
		if err != nil {
			return err
		}

		// Empty string == [stop] byte.
		if text == "" {
			return nil
		}

		p.textMap[textID] = text
		textID++
	}
}

func (p *Parser) parseRecordStream(
	r io.Reader,
) ([]record.GenericRecord, error) {
	var records []record.GenericRecord

	p.aircraftStates = make(map[string]aircraftState)
	p.controllerStates = make(map[uint16]record.ControllerPositionRecord)

	fmt.Printf("%+v\n", p.textMap)

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

func (p *Parser) parseGenericRecord(
	r io.Reader,
) (record.GenericRecord, error) {
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

		posRecord, err := p.parsePositionRecordDelta(
			r,
			changeMap[0],
		)
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

	case recordTypeMessage:
		msgRecord, err := p.parseMessageRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = msgRecord
		return genericRecord, nil

	case recordTypeUnknown:
		unknownRecord, err := p.parseUnknownRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = unknownRecord
		return genericRecord, nil

	case recordTypeAddPilot:
		addPilotRecord, err := p.parseAddPilotRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = addPilotRecord
		return genericRecord, nil

	case recordTypeControllerPosition:
		controllerPositionRecord, err := p.parseControllerPositionRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = controllerPositionRecord
		return genericRecord, nil

	case recordTypeControllerPositionUnchanged:
		controllerPositionRecord, err := p.parseControllerPositionUnchangedRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = controllerPositionRecord
		return genericRecord, nil

	case recordTypeFlightplan:
		flightplanRecord, err := p.parseFlightplanRecord(r)
		if err != nil {
			return record.GenericRecord{}, err
		}

		genericRecord.Record = flightplanRecord
		return genericRecord, nil

	case 0xFF:
		return record.GenericRecord{}, io.EOF

	default:
		return record.GenericRecord{}, fmt.Errorf(
			"unknown record type: %d",
			recordType[0],
		)
	}
}

func (p *Parser) parseControllerPositionChange(
	r io.Reader,
) error {
	controllerIDBytes, err := binio.ReadBytes(r, 2)
	if err != nil {
		return err
	}

	textID := binary.LittleEndian.Uint16(controllerIDBytes)

	callsign, ok := p.textMap[textID]
	if !ok {
		return fmt.Errorf(
			"unknown controller text ID: %d",
			textID,
		)
	}

	p.currentCallsign = callsign

	return nil
}
