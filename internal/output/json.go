package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type JSONFormatter struct {
	w io.Writer
}

func (f *JSONFormatter) WriteList(items []map[string]interface{}, _ []Column) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f.w, string(data))
	return err
}

func (f *JSONFormatter) WriteItem(item map[string]interface{}, _ []Column) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f.w, string(data))
	return err
}

func (f *JSONFormatter) WriteRaw(data []byte) error {
	_, err := f.w.Write(data)
	return err
}

func (f *JSONFormatter) Flush() error {
	return nil
}

type JSONLFormatter struct {
	w io.Writer
}

func (f *JSONLFormatter) WriteList(items []map[string]interface{}, _ []Column) error {
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(f.w, string(data)); err != nil {
			return err
		}
	}
	return nil
}

func (f *JSONLFormatter) WriteItem(item map[string]interface{}, _ []Column) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f.w, string(data))
	return err
}

func (f *JSONLFormatter) WriteRaw(data []byte) error {
	_, err := f.w.Write(data)
	return err
}

func (f *JSONLFormatter) Flush() error {
	return nil
}

func writeJSONError(w io.Writer, err error) {
	obj := map[string]interface{}{
		"error": map[string]interface{}{
			"message": err.Error(),
		},
	}
	data, _ := json.Marshal(obj)
	fmt.Fprintln(w, string(data))
}

func writeTextError(w io.Writer, err error) {
	fmt.Fprintf(w, "Error: %s\n", err.Error())
}
