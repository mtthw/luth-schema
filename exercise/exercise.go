// Package exercise decodes, validates, normalizes and encodes Lúth exercise
// definitions in the portable JSON contract described by
// schemas/exercise.schema.json (schema version 0.0.1).
//
// It matches the TypeScript and Dart SDKs in this repository, with one
// deliberate difference: JSON numbers are checked exactly, so a literal such
// as 1.0000000000000001 is not rounded into an accepted integer.
//
// Strings follow JSON's UTF-16 model: a lone surrogate written as an escape
// such as "\ud800" is kept, stored in its three-byte WTF-8 form, and encoded
// back as the same escape.
package exercise

import (
	"errors"
	"fmt"
)

// SchemaVersion is the only definition schema version this package accepts.
// It is the version of the exercise document format, not of this package.
const SchemaVersion = "0.0.1"

// Limits of the contract, shared with the TypeScript and Dart SDKs.
const (
	MaxDepth             = 32
	MaxNodes             = 10_000
	MaxSteps             = 100_000
	MaxReps              = 100_000
	MaxSourceBytes       = 262_144
	MaxDescriptionLength = 10_000 // Unicode code points
	MaxSafeInteger       = 9_007_199_254_740_991
	MaxARGB              = 0xFFFFFFFF
)

var (
	// ErrInvalid is wrapped by every error that rejects a definition.
	ErrInvalid = errors.New("invalid exercise")
	// ErrTooLarge is also wrapped when JSON source exceeds MaxSourceBytes.
	ErrTooLarge = errors.New("exercise source is too large")
)

var errTooLarge = fmt.Errorf("%w: %w", ErrInvalid, ErrTooLarge)

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...)
}

// Definition is a normalized exercise definition.
type Definition struct {
	SchemaVersion string
	Title         string
	// Description is CommonMark text, or nil when omitted or null.
	Description *string
	// CountdownTones are the default tones; nil encodes as [].
	CountdownTones []Tone
	Color          *Color
	Sets           []Set
}

// Set is a subset of an exercise. A set without subsets is a leaf: timed when
// DurationMicroseconds is set, otherwise manual.
type Set struct {
	Title                string
	Reps                 int
	Subsets              []Set
	SkipLast             bool
	Description          *string
	DurationMicroseconds *int64
	Color                *Color
	// CountdownTones nil inherits the definition's defaults; a non-nil empty
	// slice means silence.
	CountdownTones []Tone
}

// Tone is a countdown tone, such as {Pitch: "Cs4", Length: "short"}.
type Tone struct {
	Pitch  string
	Length string // "automatic", "short" or "long"
}

// Color is a single ARGB colour, or a light/dark pair when Modes is set.
type Color struct {
	ARGB uint32
	// Modes, when non-nil, holds mode-dependent colours and ARGB is ignored.
	Modes *ModeColor
}

// ModeColor holds separate ARGB colours for light and dark mode.
type ModeColor struct {
	Light uint32
	Dark  uint32
}
