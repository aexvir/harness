package internal

import "os"

// StatusSymbols holds the display symbols used for status output.
type StatusSymbols struct {
	Success string // ✔ or [OK]
	Error   string // ✘ or [ERR]
	Command string // ⌘ or >
	Dot     string // • or o
	Detail  string // └ or --
}

var Symbols = func() StatusSymbols {
	if os.Getenv("HARNESS_FALLBACK_SYMBOLS") != "" {
		return fallbackSymbols()
	}
	return defaultSymbols()
}()

func defaultSymbols() StatusSymbols {
	return StatusSymbols{
		Success: "✔",
		Error:   "✘",
		Command: "⌘",
		Dot:     "•",
		Detail:  "└",
	}
}

func fallbackSymbols() StatusSymbols {
	return StatusSymbols{
		Success: "[OK]",
		Error:   "[ERR]",
		Command: ">",
		Dot:     "o",
		Detail:  "--",
	}
}
