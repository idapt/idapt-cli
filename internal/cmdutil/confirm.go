package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ConfirmAction(f *Factory, prompt string) bool {
	if f == nil {
		return false
	}

	if f.In == nil {
		return false
	}

	fmt.Fprintf(f.ErrOut, "%s [y/N]: ", prompt)

	if file, ok := f.In.(*os.File); ok {
		fi, err := file.Stat()
		if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
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
