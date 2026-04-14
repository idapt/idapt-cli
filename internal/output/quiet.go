package output

import (
	"fmt"
	"io"
)

type QuietFormatter struct {
	w io.Writer
}

func (f *QuietFormatter) WriteList(items []map[string]interface{}, _ []Column) error {
	for _, item := range items {
		if id, ok := item["id"]; ok {
			fmt.Fprintln(f.w, formatValue(id))
		}
	}
	return nil
}

func (f *QuietFormatter) WriteItem(item map[string]interface{}, _ []Column) error {
	if id, ok := item["id"]; ok {
		fmt.Fprintln(f.w, formatValue(id))
	}
	return nil
}

func (f *QuietFormatter) WriteRaw(data []byte) error {
	_, err := f.w.Write(data)
	return err
}

func (f *QuietFormatter) Flush() error {
	return nil
}
