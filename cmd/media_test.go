package cmd

import (
	"testing"
)

func TestClassifyInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected inputType
	}{
		{
			name:     "HTTP URL",
			input:    "http://example.com/photo.png",
			expected: inputTypeURL,
		},
		{
			name:     "HTTPS URL",
			input:    "https://example.com/images/photo.png",
			expected: inputTypeURL,
		},
		{
			name:     "remote idapt path with 3+ slashes",
			input:    "/alice/personal/files/Photos/photo.png",
			expected: inputTypeRemotePath,
		},
		{
			name:     "remote idapt path minimal",
			input:    "/owner/project/files",
			expected: inputTypeRemotePath,
		},
		{
			name:     "local file relative",
			input:    "./photo.png",
			expected: inputTypeLocalFile,
		},
		{
			name:     "local file bare name",
			input:    "photo.png",
			expected: inputTypeLocalFile,
		},
		{
			name:     "local file with directory",
			input:    "images/photo.png",
			expected: inputTypeLocalFile,
		},
		{
			name:     "short absolute path is local (only 2 slashes)",
			input:    "/tmp/photo.png",
			expected: inputTypeLocalFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyInput(tt.input)
			if result != tt.expected {
				t.Errorf("classifyInput(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}
