package output

import (
	"fmt"
	"io"
	"strings"
)

type TableFormatter struct {
	w       io.Writer
	noColor bool
}

func (f *TableFormatter) WriteList(items []map[string]interface{}, columns []Column) error {
	if len(columns) == 0 {
		return nil
	}

	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col.Header)
		if col.Width > 0 {
			widths[i] = col.Width
		}
	}
	for _, item := range items {
		for i, col := range columns {
			val := formatValue(extractField(item, col.Field))
			if len(val) > widths[i] && (col.Width == 0 || col.Width > widths[i]) {
				if col.Width > 0 {
					widths[i] = col.Width
				} else {
					widths[i] = len(val)
				}
			}
		}
	}

	for i, col := range columns {
		if i > 0 {
			fmt.Fprint(f.w, "  ")
		}
		fmt.Fprintf(f.w, "%-*s", widths[i], col.Header)
	}
	fmt.Fprintln(f.w)

	totalWidth := 0
	for i, w := range widths {
		if i > 0 {
			totalWidth += 2
		}
		totalWidth += w
	}
	fmt.Fprintln(f.w, strings.Repeat("-", totalWidth))

	for _, item := range items {
		for i, col := range columns {
			if i > 0 {
				fmt.Fprint(f.w, "  ")
			}
			val := formatValue(extractField(item, col.Field))
			if col.Width > 0 && len(val) > col.Width {
				val = val[:col.Width-3] + "..."
			}
			fmt.Fprintf(f.w, "%-*s", widths[i], val)
		}
		fmt.Fprintln(f.w)
	}

	return nil
}

func (f *TableFormatter) WriteItem(item map[string]interface{}, columns []Column) error {
	maxKeyLen := 0
	for _, col := range columns {
		if len(col.Header) > maxKeyLen {
			maxKeyLen = len(col.Header)
		}
	}

	for _, col := range columns {
		val := formatValue(extractField(item, col.Field))
		fmt.Fprintf(f.w, "%-*s  %s\n", maxKeyLen, col.Header+":", val)
	}
	return nil
}

func (f *TableFormatter) WriteRaw(data []byte) error {
	_, err := f.w.Write(data)
	return err
}

func (f *TableFormatter) Flush() error {
	return nil
}

func formatValue(v interface{}) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%v", v)
}
