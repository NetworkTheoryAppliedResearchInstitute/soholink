package printer

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GCodeValidator checks G-code files for safety before printing.
// It enforces temperature limits, feedrate limits, and blocks prohibited commands.
type GCodeValidator struct {
	MaxTemp            int      // Max hotend temperature (e.g. 280 for PLA)
	MaxBedTemp         int      // Max bed temperature (e.g. 110)
	MaxFeedRate        int      // Max feedrate in mm/min
	MaxAccel           int      // Max acceleration in mm/s²
	ProhibitedCommands []string // e.g. M0, M1, M112
}

// NewGCodeValidator creates a validator with safe defaults.
func NewGCodeValidator() *GCodeValidator {
	return &GCodeValidator{
		MaxTemp:     280,
		MaxBedTemp:  110,
		MaxFeedRate: 15000,
		MaxAccel:    5000,
		ProhibitedCommands: []string{
			"M0",   // Unconditional stop
			"M1",   // Sleep
			"M112", // Emergency stop
			"M997", // Firmware update
			"M999", // Reset
		},
	}
}

// Validate reads a G-code file and checks all commands against safety limits.
func (v *GCodeValidator) Validate(gcodeFile string) error {
	f, err := os.Open(gcodeFile)
	if err != nil {
		return fmt.Errorf("failed to open G-code file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Strip inline comments
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		// Check prohibited commands
		for _, cmd := range v.ProhibitedCommands {
			if strings.HasPrefix(line, cmd+" ") || line == cmd {
				return fmt.Errorf("line %d: prohibited command %s", lineNum, cmd)
			}
		}

		// Check hotend temperature commands (M104 set, M109 wait)
		if strings.HasPrefix(line, "M104") || strings.HasPrefix(line, "M109") {
			temp := parseParameter(line, "S")
			if temp > 0 && temp > v.MaxTemp {
				return fmt.Errorf("line %d: hotend temperature %d exceeds limit %d", lineNum, temp, v.MaxTemp)
			}
		}

		// Check bed temperature commands (M140 set, M190 wait)
		if strings.HasPrefix(line, "M140") || strings.HasPrefix(line, "M190") {
			temp := parseParameter(line, "S")
			if temp > 0 && temp > v.MaxBedTemp {
				return fmt.Errorf("line %d: bed temperature %d exceeds limit %d", lineNum, temp, v.MaxBedTemp)
			}
		}

		// Check feedrate
		if strings.Contains(line, "F") {
			feedrate := parseParameter(line, "F")
			if feedrate > 0 && feedrate > v.MaxFeedRate {
				return fmt.Errorf("line %d: feedrate %d exceeds limit %d", lineNum, feedrate, v.MaxFeedRate)
			}
		}
	}

	return scanner.Err()
}

// parseParameter extracts an integer parameter value from a G-code line.
// e.g. parseParameter("M104 S220", "S") returns 220.
func parseParameter(line string, param string) int {
	fields := strings.Fields(line)
	for _, field := range fields {
		if strings.HasPrefix(field, param) {
			valStr := field[len(param):]
			val, err := strconv.Atoi(valStr)
			if err != nil {
				// Try parsing as float and truncating
				fval, ferr := strconv.ParseFloat(valStr, 64)
				if ferr != nil {
					return 0
				}
				return int(fval)
			}
			return val
		}
	}
	return 0
}
