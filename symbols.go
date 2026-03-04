package harness

import (
	"os"
)

var Symbols = func() statusSymbols {
	if os.Getenv("HARNESS_FALLBACK_SYMBOLS") != "" {
		return fallbackSymbols()
	}
	return defaultSymbols()
}()

// statusSymbols holds the display symbols used for status output.
type statusSymbols struct {
	Success string // ✔ or [OK]
	Error   string // ✘ or [ERR]
	Command string // ⌘ or >
	Dot     string // • or o
	Detail  string // ╰ or --
}

// defaultSymbols returns Unicode symbols for modern terminals.
func defaultSymbols() statusSymbols {
	return statusSymbols{
		Success: "✔",
		Error:   "✘",
		Command: "⌘",
		Dot:     "•",
		Detail:  "╰",
	}
}

// fallbackSymbols returns ASCII symbols for legacy terminals.
func fallbackSymbols() statusSymbols {
	return statusSymbols{
		Success: "[OK]",
		Error:   "[ERR]",
		Command: ">",
		Dot:     "o",
		Detail:  "--",
	}
}
