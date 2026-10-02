package record

import (
	"time"
)

// GenericRecord is one timestamp line plus one message line from an ES replay.
type GenericRecord struct {
	Time     time.Duration
	Callsign string
	Record   Record
}

// Record is any message that can appear as the second line of a GenericRecord.
// Concrete types implement this interface.
type Record any

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

type MessageRecord struct {
	Sender   string
	Receiver string
	Message  string
}

type UnknownRecord struct {
	ArrowType string
	Raw       string
}

func (g GenericRecord) ArrowType() string {
	switch r := g.Record.(type) {
	case PositionRecord:
		return ">>>>"
	case MessageRecord:
		if r.Sender == g.Callsign {
			return ">>>>"
		}

		return "<<<2"
	case UnknownRecord:
		return r.ArrowType
	default:
		panic("unknown record type: " + r.(any).(string))
	}
}
