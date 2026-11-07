package hub

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"
)

// Global monotonic sequence counter for event ordering.
var globalSeq uint64

// NextEventID generates a monotonically increasing event ID based on current millisecond timestamp and sequence counter.
func NextEventID() (string, uint64) {
	seq := atomic.AddUint64(&globalSeq, 1)
	now := time.Now().UnixMilli()
	return fmt.Sprintf("%d-%06d", now, seq%1000000), seq
}

// Event represents an event payload broadcast across channels.
type Event struct {
	ID        string    `json:"id"`
	Seq       uint64    `json:"seq"`
	Channel   string    `json:"channel"`
	Event     string    `json:"event"`
	Data      []byte    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	RawSSE    []byte    `json:"-"` // Pre-rendered SSE formatted frame: id, event, data, \n\n
}

// FormatSSE formats an event into standard Server-Sent Events framing.
// id: <id>\n
// event: <event>\n
// data: <data>\n\n
func (e *Event) FormatSSE() []byte {
	if len(e.RawSSE) > 0 {
		return e.RawSSE
	}

	var buf bytes.Buffer
	if e.ID != "" {
		buf.WriteString("id: ")
		buf.WriteString(e.ID)
		buf.WriteByte('\n')
	}
	if e.Event != "" {
		buf.WriteString("event: ")
		buf.WriteString(e.Event)
		buf.WriteByte('\n')
	}

	// In SSE, each line of data is prefixed with "data: "
	buf.WriteString("data: ")
	buf.Write(e.Data)
	buf.WriteString("\n\n")

	e.RawSSE = buf.Bytes()
	return e.RawSSE
}
