// Package output provides formatters for CLI output (table, JSON, JSONL, quiet).
package output

import (
	"io"
	"os"
)

// Format represents an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatQuiet Format = "quiet"
)

// Column defines a table column.
type Column struct {
	Header string
	Field  string
	Width  int // max width (0 = auto)
}

// Formatter writes structured data to an output stream.
type Formatter interface {
	WriteList(items []map[string]interface{}, columns []Column) error
	WriteItem(item map[string]interface{}, columns []Column) error
	WriteRaw(data []byte) error
	Flush() error
}

// New creates a Formatter for the given format.
func New(format Format, w io.Writer, noColor bool) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{w: w}
	case FormatJSONL:
		return &JSONLFormatter{w: w}
	case FormatQuiet:
		return &QuietFormatter{w: w}
	default:
		return &TableFormatter{w: w, noColor: noColor}
	}
}

// Detect auto-detects the output format based on whether stdout is a TTY.
func Detect() Format {
	if IsTerminal(os.Stdout) {
		return FormatTable
	}
	return FormatJSON
}

// IsTerminal returns true if the given file is a terminal.
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// WriteError writes an error to stderr in the appropriate format.
func WriteError(format Format, w io.Writer, err error) {
	switch format {
	case FormatJSON, FormatJSONL:
		writeJSONError(w, err)
	default:
		writeTextError(w, err)
	}
}

func extractField(item map[string]interface{}, field string) interface{} {
	return item[field]
}
