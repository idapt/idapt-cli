package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfirmAction prompts the user for confirmation.
// Returns true if --confirm flag is set or user types "y"/"yes".
// In non-TTY stdin, returns false unless --confirm is set.
func ConfirmAction(f *Factory, prompt string) bool {
	if f == nil {
		return false
	}

	// Check if we can read from stdin
	if f.In == nil {
		return false
	}

	fmt.Fprintf(f.ErrOut, "%s [y/N]: ", prompt)

	// Check if stdin is a terminal
	if file, ok := f.In.(*os.File); ok {
		fi, err := file.Stat()
		if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
			// Non-TTY stdin
			return false
		}
	}

	scanner := bufio.NewScanner(f.In)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}
