package replay

import (
	"fmt"
	"strings"

	"github.com/AliceFord/es-compress/record"
)

func Export(records []record.GenericRecord) string {
	var out strings.Builder

	for _, rec := range records {
		positionRecord, ok := rec.Record.(record.PositionRecord)
		if !ok {
			panic("unexpected record type: " + fmt.Sprintf("%T", rec.Record))
		}

		transponderLetter := "N"
		if !positionRecord.IsNormalMode {
			transponderLetter = "S"
		}

		encodedHdg := int((float64(positionRecord.Heading)*2.88 + 0.5) * 4)

		fmt.Fprintf(&out, "[%02d:%02d:%02d >>>> %s]\n", int64(rec.Time.Hours()), int64(rec.Time.Minutes())%60, int64(rec.Time.Seconds())%60, rec.Callsign)
		fmt.Fprintf(&out, "@%s:%s:%d:1:%f:%f:%d:0:%d:0\n", transponderLetter, positionRecord.Callsign, positionRecord.Squawk, positionRecord.Latitude, positionRecord.Longitude, positionRecord.Altitude, encodedHdg)
	}

	return out.String()
}
