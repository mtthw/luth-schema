package exercise

// Decode parses and validates JSON source of at most MaxSourceBytes bytes,
// normalizing omitted optional fields to nil. It never migrates legacy
// definitions; use Migrate for that.
func Decode(source []byte) (Definition, error) {
	value, err := parseSource(source)
	if err != nil {
		return Definition{}, err
	}
	return definitionFromJSON(value)
}

// Migrate explicitly converts the prototype's {"formatVersion": 1} JSON to
// schema version 0.0.1, renaming each subset's info to description unless it
// already has one, then validates the result. Current definitions are decoded
// unchanged; any other version is rejected.
func Migrate(source []byte) (Definition, error) {
	value, err := parseSource(source)
	if err != nil {
		return Definition{}, err
	}
	if object, ok := value.(map[string]any); ok && isLegacy(object) {
		delete(object, "formatVersion")
		object["schemaVersion"] = SchemaVersion
		if sets, ok := object["sets"].([]any); ok {
			migrateLegacySets(sets)
		}
	}
	return definitionFromJSON(value)
}

func parseSource(source []byte) (any, error) {
	if len(source) > MaxSourceBytes {
		return nil, errTooLarge
	}
	return parseJSON(source)
}

func isLegacy(object map[string]any) bool {
	version, ok := object["formatVersion"].(number)
	if !ok {
		return false
	}
	_, hasSchemaVersion := object["schemaVersion"]
	_, isOne := exactInteger(version, 1, 1)
	return isOne && !hasSchemaVersion
}

// migrateLegacySets edits freshly parsed sets in place. Parser nesting limits
// bound the recursion; too-deep input is rejected by validation afterwards.
func migrateLegacySets(sets []any) {
	for _, value := range sets {
		set, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if info, hasInfo := set["info"]; hasInfo {
			if _, hasDescription := set["description"]; !hasDescription {
				set["description"] = info
				delete(set, "info")
			}
		}
		if children, ok := set["subsets"].([]any); ok {
			migrateLegacySets(children)
		}
	}
}

func definitionFromJSON(value any) (Definition, error) {
	object, err := objectWithKeys(value, "exercise",
		[]string{"schemaVersion", "title", "countdownTones", "sets"},
		[]string{"color", "description"})
	if err != nil {
		return Definition{}, err
	}
	if version, ok := object["schemaVersion"].(string); !ok || version != SchemaVersion {
		return Definition{}, invalidf("unsupported exercise schema version")
	}
	d := Definition{SchemaVersion: SchemaVersion}
	if d.Title, err = stringFromJSON(object["title"], "title"); err != nil {
		return Definition{}, err
	}
	if d.Description, err = optionalString(object["description"], "description"); err != nil {
		return Definition{}, err
	}
	if d.CountdownTones, err = tonesFromJSON(object["countdownTones"]); err != nil {
		return Definition{}, err
	}
	if d.Color, err = colorFromJSON(object["color"]); err != nil {
		return Definition{}, err
	}
	if d.Sets, err = setsFromJSON(object["sets"], 0); err != nil {
		return Definition{}, err
	}
	if err := d.Validate(); err != nil {
		return Definition{}, err
	}
	return d, nil
}

func setsFromJSON(value any, depth int) ([]Set, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, invalidf("sets and subsets must be arrays")
	}
	if len(items) > 0 && depth >= MaxDepth {
		return nil, errTooComplex
	}
	sets := make([]Set, len(items))
	for i, item := range items {
		object, err := objectWithKeys(item, "subset",
			[]string{"title", "reps", "subsets", "skipLast"},
			[]string{"description", "durationMicroseconds", "color", "countdownTones"})
		if err != nil {
			return nil, err
		}
		set := &sets[i]
		if set.Title, err = stringFromJSON(object["title"], "title"); err != nil {
			return nil, err
		}
		if set.Description, err = optionalString(object["description"], "description"); err != nil {
			return nil, err
		}
		reps, err := integerFromJSON(object["reps"], "reps", 1, MaxReps)
		if err != nil {
			return nil, err
		}
		set.Reps = int(reps)
		if object["durationMicroseconds"] != nil {
			duration, err := integerFromJSON(object["durationMicroseconds"], "durationMicroseconds", 1, MaxSafeInteger)
			if err != nil {
				return nil, err
			}
			set.DurationMicroseconds = &duration
		}
		if set.SkipLast, ok = object["skipLast"].(bool); !ok {
			return nil, invalidf("skipLast must be a boolean")
		}
		if set.Color, err = colorFromJSON(object["color"]); err != nil {
			return nil, err
		}
		if object["countdownTones"] != nil {
			if set.CountdownTones, err = tonesFromJSON(object["countdownTones"]); err != nil {
				return nil, err
			}
		}
		if set.Subsets, err = setsFromJSON(object["subsets"], depth+1); err != nil {
			return nil, err
		}
	}
	return sets, nil
}

func objectWithKeys(value any, name string, required, optional []string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, invalidf("%s must be a JSON object", name)
	}
	for _, key := range required {
		if _, ok := object[key]; !ok {
			return nil, invalidf("%s is missing %q", name, key)
		}
	}
	for key := range object {
		if !contains(required, key) && !contains(optional, key) {
			return nil, invalidf("%s has unknown field %q", name, key)
		}
	}
	return object, nil
}

func contains(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

func stringFromJSON(value any, name string) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", invalidf("%s must be a string", name)
	}
	return text, nil
}

func optionalString(value any, name string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	text, err := stringFromJSON(value, name)
	return &text, err
}

func integerFromJSON(value any, name string, minimum, maximum int64) (int64, error) {
	literal, ok := value.(number)
	if !ok {
		return 0, invalidf("%s must be a number", name)
	}
	integer, ok := exactInteger(literal, minimum, maximum)
	if !ok {
		return 0, invalidf("%s must be an integer from %d to %d", name, minimum, maximum)
	}
	return integer, nil
}

func colorFromJSON(value any) (*Color, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		object, err := objectWithKeys(value, "color", []string{"light", "dark"}, nil)
		if err != nil {
			return nil, err
		}
		light, err := integerFromJSON(object["light"], "light color", 0, MaxARGB)
		if err != nil {
			return nil, err
		}
		dark, err := integerFromJSON(object["dark"], "dark color", 0, MaxARGB)
		if err != nil {
			return nil, err
		}
		return &Color{Modes: &ModeColor{Light: uint32(light), Dark: uint32(dark)}}, nil
	default:
		argb, err := integerFromJSON(value, "color", 0, MaxARGB)
		if err != nil {
			return nil, err
		}
		return &Color{ARGB: uint32(argb)}, nil
	}
}

func tonesFromJSON(value any) ([]Tone, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, invalidf("countdownTones must be an array")
	}
	tones := make([]Tone, len(items))
	for i, item := range items {
		object, err := objectWithKeys(item, "tone", []string{"pitch", "length"}, nil)
		if err != nil {
			return nil, err
		}
		pitch, pitchOK := object["pitch"].(string)
		length, lengthOK := object["length"].(string)
		if !pitchOK || !lengthOK {
			return nil, invalidf("tone pitch and length must be strings")
		}
		tones[i] = Tone{Pitch: pitch, Length: length}
	}
	return tones, nil
}
