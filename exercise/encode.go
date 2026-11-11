package exercise

import "strconv"

// Encode validates d and returns compact JSON with object keys sorted by
// UTF-16 code units and every optional field written, null when absent. The
// output is byte-identical to the TypeScript and Dart encodeExercise and at
// most MaxSourceBytes long.
func Encode(d Definition) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	source := appendDefinition(nil, d)
	if len(source) > MaxSourceBytes {
		return nil, errTooLarge
	}
	return source, nil
}

// Keys below are written in sorted order.

func appendDefinition(b []byte, d Definition) []byte {
	b = append(b, `{"color":`...)
	b = appendColor(b, d.Color)
	b = append(b, `,"countdownTones":`...)
	b = appendTones(b, d.CountdownTones)
	b = append(b, `,"description":`...)
	b = appendOptionalString(b, d.Description)
	b = append(b, `,"schemaVersion":`...)
	b = appendString(b, d.SchemaVersion)
	b = append(b, `,"sets":`...)
	b = appendSets(b, d.Sets)
	b = append(b, `,"title":`...)
	b = appendString(b, d.Title)
	return append(b, '}')
}

func appendSets(b []byte, sets []Set) []byte {
	b = append(b, '[')
	for i, set := range sets {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, `{"color":`...)
		b = appendColor(b, set.Color)
		b = append(b, `,"countdownTones":`...)
		if set.CountdownTones == nil {
			b = append(b, "null"...)
		} else {
			b = appendTones(b, set.CountdownTones)
		}
		b = append(b, `,"description":`...)
		b = appendOptionalString(b, set.Description)
		b = append(b, `,"durationMicroseconds":`...)
		if set.DurationMicroseconds == nil {
			b = append(b, "null"...)
		} else {
			b = strconv.AppendInt(b, *set.DurationMicroseconds, 10)
		}
		b = append(b, `,"reps":`...)
		b = strconv.AppendInt(b, int64(set.Reps), 10)
		b = append(b, `,"skipLast":`...)
		b = strconv.AppendBool(b, set.SkipLast)
		b = append(b, `,"subsets":`...)
		b = appendSets(b, set.Subsets)
		b = append(b, `,"title":`...)
		b = appendString(b, set.Title)
		b = append(b, '}')
	}
	return append(b, ']')
}

func appendTones(b []byte, tones []Tone) []byte {
	b = append(b, '[')
	for i, tone := range tones {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, `{"length":`...)
		b = appendString(b, tone.Length)
		b = append(b, `,"pitch":`...)
		b = appendString(b, tone.Pitch)
		b = append(b, '}')
	}
	return append(b, ']')
}

func appendColor(b []byte, color *Color) []byte {
	switch {
	case color == nil:
		return append(b, "null"...)
	case color.Modes != nil:
		b = append(b, `{"dark":`...)
		b = strconv.AppendUint(b, uint64(color.Modes.Dark), 10)
		b = append(b, `,"light":`...)
		b = strconv.AppendUint(b, uint64(color.Modes.Light), 10)
		return append(b, '}')
	default:
		return strconv.AppendUint(b, uint64(color.ARGB), 10)
	}
}

func appendOptionalString(b []byte, s *string) []byte {
	if s == nil {
		return append(b, "null"...)
	}
	return appendString(b, *s)
}
