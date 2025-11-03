package parsers

import (
	"time"
)

type Event interface {
    GetTimestamp() time.Time
    GetSourceName() string
}

type LogParser interface {
    Name() string
    Parse(line string) (Event, error)
    CanParse(line string) bool
}