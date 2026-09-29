package record

import "time"

// GenericRecord is one timestamp line plus one message line from an ES replay.
type GenericRecord struct {
	Time     time.Duration
	Callsign string
	Record   Record
}

// Record is any message that can appear as the second line of a GenericRecord.
// Concrete types implement this interface.
type Record interface {
	Direction() bool // true if direction is >>, false if <<
	isPosition()
}

// PositionRecord is the @N / @S position update record.
type PositionRecord struct {
	IsNormalMode bool
	Callsign     string
	Squawk       uint16
	Latitude     float64
	Longitude    float64
	Altitude     uint16
	Heading      uint16
}

func (PositionRecord) isPosition() {}

func (PositionRecord) Direction() bool {
	return true
}
