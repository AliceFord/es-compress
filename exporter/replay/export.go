package replay

import (
	"fmt"
	"strings"

	"github.com/AliceFord/es-compress/record"
)

func Export(records []record.GenericRecord) string {
	var out strings.Builder

	for _, rec := range records {
		fmt.Fprintf(&out, "[%02d:%02d:%02d %s %s]\n", int64(rec.Time.Hours()), int64(rec.Time.Minutes())%60, int64(rec.Time.Seconds())%60, rec.ArrowType(), rec.Callsign)

		switch r := rec.Record.(type) {
		case record.PositionRecord:
			exportPositionRecord(&out, r)
		case record.MessageRecord:
			exportMessageRecord(&out, r)
		case record.UnknownRecord:
			exportUnknownRecord(&out, r)
		case record.AddPilotRecord:
			exportAddPilotRecord(&out, r)
		default:
			panic(fmt.Sprintf("unknown record type: %+v", r))
		}
	}

	return out.String()
}

func exportPositionRecord(out *strings.Builder, rec record.PositionRecord) {
	transponderLetter := "N"
	if !rec.IsNormalMode {
		transponderLetter = "S"
	}

	encodedHdg := int((float64(rec.Heading)*2.88 + 0.5) * 4)

	fmt.Fprintf(out, "@%s:%s:%d:1:%f:%f:%d:0:%d:0\n", transponderLetter, rec.Callsign, rec.Squawk, rec.Latitude, rec.Longitude, rec.Altitude, encodedHdg)
}

func exportMessageRecord(out *strings.Builder, rec record.MessageRecord) {
	fmt.Fprintf(out, "#TM%s:%s:%s\n", rec.Sender, rec.Receiver, rec.Message)
}

func exportUnknownRecord(out *strings.Builder, rec record.UnknownRecord) {
	fmt.Fprintf(out, "%s\n", rec.Raw)
}

func exportAddPilotRecord(out *strings.Builder, rec record.AddPilotRecord) {
	fmt.Fprintf(out, "#AP%s:SERVER:%d::1:101:%d:%s\n", rec.Callsign, rec.CID, rec.Rating, rec.Name)
}
