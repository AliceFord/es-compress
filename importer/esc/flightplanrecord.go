package esc

import (
	"fmt"
	"io"
	"time"

	"github.com/AliceFord/es-compress/binio"
	"github.com/AliceFord/es-compress/record"
)

func (p *Parser) parseFlightplanRecord(r io.Reader) (record.FlightplanRecord, error) {
	callsignId, err := binio.ReadUvarint(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	callsign, ok := p.textMap[uint16(callsignId)]
	if !ok {
		return record.FlightplanRecord{}, fmt.Errorf("No textMap entry found for id %d\n", callsignId)
	}

	flightRulesBytes, err := binio.ReadBytes(r, 1)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	aircraftTypeId, err := binio.ReadUvarint(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	aircraftType, ok := p.textMap[uint16(aircraftTypeId)]
	if !ok {
		return record.FlightplanRecord{}, fmt.Errorf("No textMap entry found for id %d\n", aircraftTypeId)
	}

	speedBytes, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.FlightplanRecord{}, err
	}
	speed := uint16(speedBytes[0]) | uint16(speedBytes[1])<<8

	departure, err := binio.ReadCString(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	offblocksTimeBytes, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.FlightplanRecord{}, err
	}
	offBlocksTime := time.Duration(uint16(offblocksTimeBytes[0])|uint16(offblocksTimeBytes[1])<<8) * time.Minute

	cruiseAltBytes, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.FlightplanRecord{}, err
	}
	cruiseAlt := uint16(cruiseAltBytes[0]) | uint16(cruiseAltBytes[1])<<8

	arrival, err := binio.ReadCString(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	enrouteTimeBytes, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.FlightplanRecord{}, err
	}
	enrouteTime := time.Duration(uint32(enrouteTimeBytes[0])|uint32(enrouteTimeBytes[1])<<8) * time.Minute

	enrouteFuelBytes, err := binio.ReadBytes(r, 2)
	if err != nil {
		return record.FlightplanRecord{}, err
	}
	enrouteFuel := time.Duration(uint32(enrouteFuelBytes[0])|uint32(enrouteFuelBytes[1])<<8) * time.Minute

	alternate, err := binio.ReadCString(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	detailsId, err := binio.ReadUvarint(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	details, ok := p.textMap[uint16(detailsId)]
	if !ok {
		return record.FlightplanRecord{}, fmt.Errorf("No textMap entry found for id %d\n", detailsId)
	}

	routeId, err := binio.ReadUvarint(r)
	if err != nil {
		return record.FlightplanRecord{}, err
	}

	route, ok := p.textMap[uint16(routeId)]
	if !ok {
		return record.FlightplanRecord{}, fmt.Errorf("No textMap entry found for id %d\n", routeId)
	}

	return record.FlightplanRecord{
		Callsign:      callsign,
		FlightRules:   flightRulesBytes[0],
		AircraftType:  aircraftType,
		Speed:         speed,
		Departure:     departure,
		OffblocksTime: offBlocksTime,
		CruiseAlt:     cruiseAlt,
		Arrival:       arrival,
		EnrouteTime:   enrouteTime,
		EnrouteFuel:   enrouteFuel,
		Alternate:     alternate,
		Details:       details,
		Route:         route,
	}, nil
}
