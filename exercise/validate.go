package exercise

var pitches = func() map[string]bool {
	set := map[string]bool{}
	for _, pitch := range []string{
		"E2", "F2", "Fs2", "G2", "Gs2", "A2", "As2", "B2",
		"C3", "Cs3", "D3", "Ds3", "E3", "F3", "Fs3", "G3", "Gs3", "A3", "As3", "B3",
		"C4", "Cs4", "D4", "Ds4", "E4", "F4", "Fs4", "G4", "Gs4", "A4", "As4", "B4",
		"C5", "Cs5", "D5", "Ds5", "E5", "F5", "Fs5", "G5", "Gs5", "A5", "As5", "B5",
		"C6",
	} {
		set[pitch] = true
	}
	return set
}()

var errTooComplex = invalidf("exercise definition is too complex")

// Validate checks every rule of the contract, including the nesting, node and
// expanded-step budgets that JSON Schema cannot express. It does not check
// the encoded size; Encode does.
func (d Definition) Validate() error {
	if d.SchemaVersion != SchemaVersion {
		return invalidf("unsupported exercise schema version")
	}
	if err := validateTitle(d.Title); err != nil {
		return err
	}
	if err := validateDescription(d.Description); err != nil {
		return err
	}
	if err := validateTones(d.CountdownTones); err != nil {
		return err
	}
	if len(d.Sets) == 0 || len(d.Sets) > MaxNodes {
		return invalidf("an exercise needs 1 to %d sets", MaxNodes)
	}
	nodes := 0
	if err := validateSets(d.Sets, 0, &nodes); err != nil {
		return err
	}
	steps, err := CountSteps(d.Sets)
	if err != nil {
		return err
	}
	if steps == 0 {
		return invalidf("an exercise needs an executable step")
	}
	return nil
}

func validateSets(sets []Set, depth int, nodes *int) error {
	if len(sets) > MaxNodes {
		return invalidf("too many subsets")
	}
	for i := range sets {
		set := &sets[i]
		*nodes++
		if depth >= MaxDepth || *nodes > MaxNodes {
			return errTooComplex
		}
		if err := validateTitle(set.Title); err != nil {
			return err
		}
		if err := validateDescription(set.Description); err != nil {
			return err
		}
		if set.Reps < 1 || set.Reps > MaxReps {
			return invalidf("reps must be an integer from 1 to %d", MaxReps)
		}
		if duration := set.DurationMicroseconds; duration != nil {
			if *duration < 1 || *duration > MaxSafeInteger {
				return invalidf("durationMicroseconds must be an integer from 1 to %d", int64(MaxSafeInteger))
			}
			if len(set.Subsets) > 0 {
				return invalidf("a timed subset cannot contain subsets")
			}
		}
		if err := validateTones(set.CountdownTones); err != nil {
			return err
		}
		if err := validateSets(set.Subsets, depth+1, nodes); err != nil {
			return err
		}
	}
	return nil
}

func validateTitle(title string) error {
	if _, ok := codePoints(title); !ok {
		return invalidf("title is not valid UTF-8")
	}
	if isBlank(title) {
		return invalidf("title must not be blank")
	}
	return nil
}

func validateDescription(description *string) error {
	if description == nil {
		return nil
	}
	if count, ok := codePoints(*description); !ok || count > MaxDescriptionLength {
		return invalidf("description must be UTF-8 of at most %d characters", MaxDescriptionLength)
	}
	return nil
}

func validateTones(tones []Tone) error {
	for _, tone := range tones {
		if !pitches[tone.Pitch] {
			return invalidf("tone pitch must be E2 through C6")
		}
		switch tone.Length {
		case "automatic", "short", "long":
		default:
			return invalidf("invalid tone length")
		}
	}
	return nil
}

// CountSteps counts the executable steps of validated sets without expanding
// repeated groups. A timed leaf runs reps times, a manual leaf once, a group
// repeats its children reps times, and skipLast drops a node's last step.
// Every subtree must stay within MaxSteps.
func CountSteps(sets []Set) (int, error) {
	count, err := countSteps(sets, 0)
	return int(count), err
}

func countSteps(sets []Set, depth int) (int64, error) {
	if len(sets) > 0 && depth >= MaxDepth {
		return 0, errTooComplex
	}
	var count int64
	for _, set := range sets {
		var leaves int64
		switch {
		case len(set.Subsets) > 0:
			children, err := countSteps(set.Subsets, depth+1)
			if err != nil {
				return 0, err
			}
			leaves = children * int64(set.Reps)
		case set.DurationMicroseconds != nil:
			leaves = int64(set.Reps)
		default:
			leaves = 1
		}
		if set.SkipLast && leaves > 0 {
			leaves--
		}
		count += leaves
		if count > MaxSteps {
			return 0, invalidf("exercise expands to too many executable steps")
		}
	}
	return count, nil
}
