package gridlayout

import (
	"fmt"
	"strconv"
	"strings"
)

// Mode is a column's base width-sizing strategy.
type Mode int

const (
	SizeAuto  Mode = iota // size to content (max-content); the default
	SizeFixed             // exactly Min(==Max) cells, content-blind
	SizeGrow              // take a Weight-proportional share of residual space
)

// Sizing is a parsed column width spec: a base Mode, optional Min/Max bounds
// (in cells), and a grow Weight (meaningful only for SizeGrow).
//
// Zero value is SizeAuto with no bounds — the default for a bare field name.
// Min is only meaningful when MinSet is true; an explicit "0.." floor (MinSet
// with Min==0) lets a SizeGrow column shrink to nothing, distinct from the
// implicit min-content floor of a plain :fr.
type Sizing struct {
	Mode   Mode
	Min    int
	Max    int // 0 = unbounded
	MinSet bool
	Weight int // SizeGrow only; default 1
}

// ParseSizing parses the text following a cell's ":" (already stripped). The
// empty string yields the auto default. Grammar: [mode][min..max] where mode is
// "auto" | "<int>" (fixed) | "[<weight>]fr", and min/max are optional integers
// around "..". Bounds may attach to any mode.
func ParseSizing(s string) (Sizing, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Sizing{Mode: SizeAuto}, nil
	}

	// split bounds off the end: forms are "mode", "mode..max", "min..mode",
	// "min..max", "min..", "..max". We isolate the ".." once.
	modePart, minStr, maxStr, hasBounds := splitBounds(s)

	sz, err := parseMode(modePart)
	if err != nil {
		return Sizing{}, err
	}
	if hasBounds {
		if minStr != "" {
			n, err := strconv.Atoi(minStr)
			if err != nil || n < 0 {
				return Sizing{}, fmt.Errorf("invalid min bound %q", minStr)
			}
			sz.Min, sz.MinSet = n, true
		}
		if maxStr != "" {
			n, err := strconv.Atoi(maxStr)
			if err != nil || n < 1 {
				return Sizing{}, fmt.Errorf("invalid max bound %q", maxStr)
			}
			sz.Max = n
		}
	}
	return sz, nil
}

// splitBounds isolates an optional "min..max" suffix where either side, or the
// mode token, may sit on either side of "..". Accepted shapes: "X", "X..",
// "..X", "X..Y" where X/Y are each either the mode token or an integer bound.
func splitBounds(s string) (mode, minBound, maxBound string, hasBounds bool) {
	if !strings.Contains(s, "..") {
		return s, "", "", false
	}
	parts := strings.SplitN(s, "..", 2)
	left, right := parts[0], parts[1]
	// the mode token is the non-integer side. Determine which side is the mode.
	if isModeToken(left) {
		return left, "", right, true // mode..max
	}
	if isModeToken(right) {
		return right, left, "", true // min..mode
	}
	// both sides integers -> auto mode with min..max
	return "", left, right, true
}

func isModeToken(s string) bool {
	if s == "auto" {
		return true
	}
	return strings.HasSuffix(s, "fr")
}

func parseMode(s string) (Sizing, error) {
	switch {
	case s == "" || s == "auto":
		return Sizing{Mode: SizeAuto}, nil
	case strings.HasSuffix(s, "fr"):
		w := 1
		if num := strings.TrimSuffix(s, "fr"); num != "" {
			n, err := strconv.Atoi(num)
			if err != nil || n < 1 {
				return Sizing{}, fmt.Errorf("invalid fr weight %q", num)
			}
			w = n
		}
		return Sizing{Mode: SizeGrow, Weight: w}, nil
	default:
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return Sizing{}, fmt.Errorf("invalid width %q (want positive integer, auto, or fr)", s)
		}
		return Sizing{Mode: SizeFixed, Min: n, Max: n}, nil
	}
}
