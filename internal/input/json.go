package input

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func ParseJSONFlag(value string, stdin io.Reader) (map[string]interface{}, error) {
	var data []byte
	var err error

	if value == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("no input on stdin")
		}
		data, err = io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("no input on stdin")
		}
	} else {
		data = []byte(value)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return result, nil
}

func ReadFileFlag(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func MergeFlags(base map[string]interface{}, overrides map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	if overrides == nil {
		return base
	}
	for k, v := range overrides {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		base[k] = v
	}
	return base
}
