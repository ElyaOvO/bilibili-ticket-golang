package terminal

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

func eraseRune(output io.Writer, value rune) error {
	columns := runeDisplayWidth(value)
	if columns == 0 {
		return nil
	}
	backspaces := strings.Repeat("\b", columns)
	_, err := fmt.Fprint(output, backspaces, strings.Repeat(" ", columns), backspaces)
	return err
}

func runeDisplayWidth(value rune) int {
	if unicode.IsControl(value) || unicode.Is(unicode.Mn, value) || unicode.Is(unicode.Me, value) || unicode.Is(unicode.Cf, value) {
		return 0
	}
	switch width.LookupRune(value).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}
