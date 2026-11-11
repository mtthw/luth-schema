package exercise_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mtthw/luth-schema/exercise"
)

const holdSource = `{"schemaVersion":"0.0.1","title":"Simple hold","countdownTones":[{"pitch":"C5","length":"short"}],` +
	`"sets":[{"title":"Hold","reps":1,"durationMicroseconds":30000000,"subsets":[],"skipLast":false}]}`

const holdEncoded = `{"color":null,"countdownTones":[{"length":"short","pitch":"C5"}],"description":null,` +
	`"schemaVersion":"0.0.1","sets":[{"color":null,"countdownTones":null,"description":null,` +
	`"durationMicroseconds":30000000,"reps":1,"skipLast":false,"subsets":[],"title":"Hold"}],"title":"Simple hold"}`

func fixtures(t testing.TB, kind string) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "fixtures", kind, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no %s fixtures: %v", kind, err)
	}
	sources := map[string][]byte{}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[filepath.Base(path)] = source
	}
	return sources
}

func decode(t *testing.T, source string) exercise.Definition {
	t.Helper()
	d, err := exercise.Decode([]byte(source))
	if err != nil {
		t.Fatalf("Decode(%.200s): %v", source, err)
	}
	return d
}

func encode(t *testing.T, d exercise.Definition) string {
	t.Helper()
	source, err := exercise.Encode(d)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return string(source)
}

func rejects(t *testing.T, source string) error {
	t.Helper()
	_, err := exercise.Decode([]byte(source))
	if !errors.Is(err, exercise.ErrInvalid) {
		t.Errorf("Decode(%.200s) = %v, want ErrInvalid", source, err)
	}
	return err
}

func withHoldSet(field, literal string) string {
	return strings.Replace(holdSource, `"skipLast":false`, `"skipLast":false,"`+field+`":`+literal, 1)
}

func withTitle(literal string) string {
	return strings.Replace(holdSource, `"title":"Simple hold"`, `"title":`+literal, 1)
}

func leaf(title string) exercise.Set {
	return exercise.Set{Title: title, Reps: 1, Subsets: []exercise.Set{}}
}

func hold() exercise.Definition {
	return exercise.Definition{SchemaVersion: exercise.SchemaVersion, Title: "Simple hold", Sets: []exercise.Set{leaf("Hold")}}
}

func TestSharedFixtures(t *testing.T) {
	for name, source := range fixtures(t, "valid") {
		t.Run("valid/"+name, func(t *testing.T) { decode(t, string(source)) })
	}
	for name, source := range fixtures(t, "invalid") {
		t.Run("invalid/"+name, func(t *testing.T) { rejects(t, string(source)) })
	}
}

func TestEncodeRoundTripsValidFixtures(t *testing.T) {
	for name, source := range fixtures(t, "valid") {
		encoded := encode(t, decode(t, string(source)))
		if again := encode(t, decode(t, encoded)); again != encoded {
			t.Errorf("%s: unstable encoding\n%s\n%s", name, encoded, again)
		}
	}
	if got := encode(t, decode(t, holdSource)); got != holdEncoded {
		t.Errorf("Encode(hold) =\n%s\nwant\n%s", got, holdEncoded)
	}
}

func TestNormalizesOptionalFields(t *testing.T) {
	d := decode(t, holdSource)
	set := d.Sets[0]
	if d.Description != nil || d.Color != nil || set.Description != nil || set.Color != nil || set.CountdownTones != nil {
		t.Errorf("omitted fields not nil: %+v", d)
	}
	explicit := strings.Replace(holdSource, `"skipLast":false`,
		`"skipLast":false,"description":null,"color":null,"countdownTones":null`, 1)
	if got := encode(t, decode(t, explicit)); got != holdEncoded {
		t.Errorf("explicit nulls encode as %s", got)
	}
	silent := decode(t, withHoldSet("countdownTones", "[]")).Sets[0].CountdownTones
	if silent == nil || len(silent) != 0 {
		t.Errorf("[] tones = %#v, want empty non-nil", silent)
	}
	integral := decode(t, string(fixtures(t, "valid")["integral-numbers.json"])).Sets[0]
	if integral.Reps != 2 || integral.Color == nil || integral.Color.ARGB != 0 || integral.Color.Modes != nil {
		t.Errorf("integral numbers = %+v", integral)
	}
	d.CountdownTones = nil // a nil root slice still encodes as []
	if got := encode(t, d); !strings.Contains(got, `"countdownTones":[],"description"`) {
		t.Errorf("nil root tones encode as %s", got)
	}
}

func TestColors(t *testing.T) {
	d := hold()
	d.Color = &exercise.Color{Modes: &exercise.ModeColor{Light: 4280311021, Dark: 4280243200}}
	d.Sets[0].Color = &exercise.Color{ARGB: exercise.MaxARGB}
	got := encode(t, d)
	if !strings.HasPrefix(got, `{"color":{"dark":4280243200,"light":4280311021},`) || !strings.Contains(got, `[{"color":4294967295,`) {
		t.Errorf("colors encode as %s", got)
	}
	if again := encode(t, decode(t, got)); again != got {
		t.Errorf("colors do not round trip: %s", again)
	}
	for _, bad := range []string{`"0xFF2F80ED"`, `-1`, `4294967296`, `1.5`, `{"light":1}`, `{"dark":1}`,
		`{"light":1,"dark":2,"extra":3}`, `{"light":1,"dark":null}`, `[1,2]`, `true`} {
		rejects(t, withHoldSet("color", bad))
		rejects(t, strings.Replace(holdSource, `"countdownTones"`, `"color":`+bad+`,"countdownTones"`, 1))
	}
}

func TestNumbersAreExact(t *testing.T) {
	cases := []struct {
		source string
		ok     bool
	}{
		{strings.Replace(holdSource, `"reps":1`, `"reps":100000`, 1), true},
		{strings.Replace(holdSource, `"reps":1`, `"reps":100000.000`, 1), true},
		{strings.Replace(holdSource, `"reps":1`, `"reps":1E+5`, 1), true},
		{strings.Replace(holdSource, `"reps":1`, `"reps":10e-1`, 1), true},
		{strings.Replace(holdSource, `"reps":1`, `"reps":0.1e1`, 1), true},
		{strings.Replace(holdSource, `"reps":1`, `"reps":100001`, 1), false},
		{strings.Replace(holdSource, `"reps":1`, `"reps":0`, 1), false},
		{strings.Replace(holdSource, `"reps":1`, `"reps":-0`, 1), false},
		{strings.Replace(holdSource, `"reps":1`, `"reps":1.0000000000000001`, 1), false},
		{strings.Replace(holdSource, `"reps":1`, `"reps":1e-999999999999`, 1), false},
		{strings.Replace(holdSource, `"reps":1`, `"reps":1e999999999999`, 1), false},
		{strings.Replace(holdSource, `"reps":1`, `"reps":"1"`, 1), false},
		{strings.Replace(holdSource, "30000000", "9007199254740991", 1), true},
		{strings.Replace(holdSource, "30000000", "9.007199254740991e15", 1), true},
		{strings.Replace(holdSource, "30000000", "9007199254740992", 1), false},
		{strings.Replace(holdSource, "30000000", "9007199254740993", 1), false},
		{strings.Replace(holdSource, "30000000", "9007199254740990.9999", 1), false},
		{strings.Replace(holdSource, "30000000", "99999999999999999999999", 1), false},
		{strings.Replace(holdSource, "30000000", "1", 1), true},
		{strings.Replace(holdSource, "30000000", "0", 1), false},
		{withHoldSet("color", "0"), true},
		{withHoldSet("color", "-0"), true},
		{withHoldSet("color", "0.000e-5"), true},
		{withHoldSet("color", "4294967295"), true},
		{withHoldSet("color", "4294967295.0000001"), false},
		{withHoldSet("color", "1e-400"), false},
	}
	for _, c := range cases {
		_, err := exercise.Decode([]byte(c.source))
		if (err == nil) != c.ok {
			t.Errorf("Decode(%s) error = %v, want ok %v", c.source, err, c.ok)
		}
	}
	d := decode(t, strings.Replace(holdSource, "30000000", "9007199254740991.0", 1))
	if *d.Sets[0].DurationMicroseconds != exercise.MaxSafeInteger {
		t.Errorf("duration = %d", *d.Sets[0].DurationMicroseconds)
	}
	if got := encode(t, decode(t, withHoldSet("color", "-0"))); !strings.Contains(got, `[{"color":0,`) {
		t.Errorf("-0 encodes as %s", got)
	}
}

func TestStrings(t *testing.T) {
	cases := map[string]string{
		`"\ud83d\ude00"`:                   `"😀"`,
		`"\uD83D\uDE00"`:                   `"😀"`,
		`"\ud800"`:                         `"\ud800"`,
		`"\udc00\ud800"`:                   `"\udc00\ud800"`,
		`"\ud800A"`:                        `"\ud800A"`,
		`"\u0000\u001f\u007f\u2028"`:       "\"\\u0000\\u001f\x7f\u2028\"",
		`"\/\"\\\b\f\n\r\t"`:               `"/\"\\\b\f\n\r\t"`,
		`"Lúth <&> \u200b \u180e e\u0301"`: "\"Lúth <&> \u200b \u180e e\u0301\"",
	}
	for title, want := range cases {
		got := encode(t, decode(t, withTitle(title)))
		if !strings.HasSuffix(got, `"title":`+want+`}`) {
			t.Errorf("title %s encodes as %s, want %s", title, got[strings.LastIndex(got, `"title"`):], want)
		}
	}
	for _, blank := range []string{`""`, `" \t\n\r"`, `"\u000b\u000c"`, `"\u0085\ufeff\u00a0"`, `"\u1680\u2000\u200a"`,
		`"\u2028\u2029\u202f\u205f\u3000"`, `3`, `null`} {
		rejects(t, withTitle(blank))
	}
	longest := strings.Repeat("😀", exercise.MaxDescriptionLength)
	lone := strings.Repeat(`\ud800`, exercise.MaxDescriptionLength)
	for _, description := range []string{`"` + longest + `"`, `"` + lone + `"`} {
		decode(t, withHoldSet("description", description))
	}
	for _, description := range []string{`"` + longest + `a"`, `"` + lone + `a"`, `42`, `true`, `[]`, `{}`} {
		rejects(t, withHoldSet("description", description))
	}
	rejects(t, withHoldSet("info", `"Renamed to description"`))
}

func TestTypedStringsMustBeWellFormed(t *testing.T) {
	for _, title := range []string{"\xff", "\xc0\x80", "a\xed\xa0", "\xed\xa0\xbd\xed\xb8\x80"} {
		d := hold()
		d.Title = title
		if _, err := exercise.Encode(d); !errors.Is(err, exercise.ErrInvalid) {
			t.Errorf("Encode with title %q = %v, want ErrInvalid", title, err)
		}
	}
	d := hold()
	d.Title = "\xed\xa0\x80" // WTF-8 for a lone U+D800
	if got := encode(t, d); !strings.HasSuffix(got, `"title":"\ud800"}`) {
		t.Errorf("lone surrogate encodes as %s", got)
	}
}

func TestMalformedJSON(t *testing.T) {
	sources := []string{"", " ", "{", "null", "[]", `"exercise"`, "1", "\ufeff" + holdSource,
		holdSource + ",", holdSource + "{}", "[" + holdSource + "]", "/**/" + holdSource,
		strings.Replace(holdSource, "}]", "},]", 1),
		strings.Replace(holdSource, `"title"`, `'title'`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":01`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":+1`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":1.`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":.1e1`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":1e`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":NaN`, 1),
		strings.Replace(holdSource, `"reps":1`, `"reps":tru`, 1),
		withTitle(`"\x41"`), withTitle(`"\u12"`), withTitle(`"\u12G4"`), withTitle(`"\ud800\u12"`),
		withTitle("\"a\tb\""), withTitle(`"unterminated`),
		withTitle("\"\xff\""), withTitle("\"\xc0\x80\""), withTitle("\"\xed\xa0\x80\""),
		withTitle(strings.Repeat("[", 100000) + strings.Repeat("]", 100000)),
		strings.Replace(holdSource, `"schemaVersion":"0.0.1"`, `"schemaVersion":"0.0.2"`, 1),
		strings.Replace(holdSource, `{"schemaVersion"`, `{"kind":"exercise","schemaVersion"`, 1),
	}
	for _, source := range sources {
		rejects(t, source)
	}
}

func TestDuplicateKeysKeepTheLastValue(t *testing.T) {
	d := decode(t, withTitle(`"First","title":"Last"`))
	if d.Title != "Last" {
		t.Errorf("title = %q", d.Title)
	}
	rejects(t, withTitle(`"First","title":" "`))
}

func TestSchemaVersions(t *testing.T) {
	for _, version := range []string{`1`, `0.001`, `null`, `""`, `"0.0"`, `"00.0.1"`, `"v0.0.1"`, `"0.0.2"`,
		`"0.1.0"`, `"1.0.0"`, `"0.0.1-beta.1"`, `"0.0.1+build.1"`} {
		rejects(t, strings.Replace(holdSource, `"0.0.1"`, version, 1))
	}
	for _, version := range []string{"", "0.0.2"} {
		d := hold()
		d.SchemaVersion = version
		if _, err := exercise.Encode(d); !errors.Is(err, exercise.ErrInvalid) {
			t.Errorf("Encode with schema version %q = %v", version, err)
		}
	}
}

func TestExecutionSemantics(t *testing.T) {
	steps := func(source string) int {
		t.Helper()
		count, err := exercise.CountSteps(decode(t, source).Sets)
		if err != nil {
			t.Fatal(err)
		}
		return count
	}
	if got := steps(string(fixtures(t, "valid")["group.json"])); got != 5 {
		t.Errorf("group steps = %d, want 5", got)
	}
	manual := `{"schemaVersion":"0.0.1","title":"M","countdownTones":[],"sets":[{"title":"Manual","reps":100000,"subsets":[],"skipLast":false}]}`
	if got := steps(manual); got != 1 {
		t.Errorf("manual steps = %d, want 1", got)
	}
	timed := strings.Replace(holdSource, `"reps":1`, `"reps":100000`, 1)
	if got := steps(timed); got != exercise.MaxSteps {
		t.Errorf("timed steps = %d", got)
	}
	extra := `,{"title":"More","reps":1,"durationMicroseconds":1,"subsets":[],"skipLast":false}]}`
	rejects(t, strings.TrimSuffix(timed, "]}")+extra)
	skipped := strings.Replace(timed, `"skipLast":false`, `"skipLast":true`, 1)
	if got := steps(strings.TrimSuffix(skipped, "]}") + extra); got != exercise.MaxSteps {
		t.Errorf("skipped steps = %d", got)
	}
	// An ancestor's skipLast does not exempt its subtree from the budget.
	group := `{"schemaVersion":"0.0.1","title":"G","countdownTones":[],"sets":[{"title":"Group","reps":1,"skipLast":true,"subsets":[` +
		`{"title":"A","reps":100000,"durationMicroseconds":1,"subsets":[],"skipLast":false},` +
		`{"title":"B","reps":1,"durationMicroseconds":1,"subsets":[],"skipLast":false}]}]}`
	rejects(t, group)
	// Manual reps are a prescription, so this group runs one step 100,000 times.
	repeated := `{"schemaVersion":"0.0.1","title":"G","countdownTones":[],"sets":[{"title":"Group","reps":100000,"skipLast":false,"subsets":[` +
		`{"title":"A","reps":2,"subsets":[],"skipLast":false}]}]}`
	if got := steps(repeated); got != exercise.MaxSteps {
		t.Errorf("repeated manual steps = %d", got)
	}
	rejects(t, strings.Replace(repeated, `"reps":2,`, `"reps":2,"durationMicroseconds":1,`, 1))
}

func TestDepthAndNodeLimits(t *testing.T) {
	d := hold()
	node := &d.Sets[0]
	for depth := 1; depth < exercise.MaxDepth; depth++ {
		node.Subsets = []exercise.Set{leaf("Nested")}
		node = &node.Subsets[0]
	}
	deepest := encode(t, d)
	decode(t, deepest)
	node.Subsets = []exercise.Set{leaf("Too deep")}
	if err := d.Validate(); !errors.Is(err, exercise.ErrInvalid) {
		t.Errorf("33 levels: %v", err)
	}
	rejects(t, strings.Replace(deepest, `"subsets":[]`, `"subsets":[{"title":"Too deep","reps":1,"subsets":[],"skipLast":false}]`, 1))

	wide := hold()
	wide.Sets = make([]exercise.Set, exercise.MaxNodes)
	for i := range wide.Sets {
		wide.Sets[i] = leaf("Manual")
	}
	if err := wide.Validate(); err != nil {
		t.Errorf("%d sets: %v", exercise.MaxNodes, err)
	}
	wide.Sets = append(wide.Sets, leaf("Manual"))
	if err := wide.Validate(); !errors.Is(err, exercise.ErrInvalid) {
		t.Errorf("%d sets: %v", exercise.MaxNodes+1, err)
	}
	// 10,001 nodes in total, though no single list is too long.
	group := exercise.Set{Title: "Group", Reps: 1, Subsets: make([]exercise.Set, exercise.MaxNodes/2)}
	for i := range group.Subsets {
		group.Subsets[i] = leaf("Manual")
	}
	nested := hold()
	nested.Sets = []exercise.Set{group, group}
	if err := nested.Validate(); !errors.Is(err, exercise.ErrInvalid) {
		t.Errorf("%d nodes: %v", exercise.MaxNodes+2, err)
	}
	// Aliased slices can form a cycle; validation must stop rather than recurse forever.
	cyclic := make([]exercise.Set, 1)
	cyclic[0] = leaf("Cycle")
	cyclic[0].Subsets = cyclic
	looped := hold()
	looped.Sets = cyclic
	if err := looped.Validate(); !errors.Is(err, exercise.ErrInvalid) {
		t.Errorf("cycle: %v", err)
	}
	if _, err := exercise.CountSteps(cyclic); !errors.Is(err, exercise.ErrInvalid) {
		t.Errorf("CountSteps(cycle): %v", err)
	}
}

func TestTypedValidation(t *testing.T) {
	duration := int64(1)
	tests := map[string]func(*exercise.Definition){
		"no sets":       func(d *exercise.Definition) { d.Sets = nil },
		"zero reps":     func(d *exercise.Definition) { d.Sets[0].Reps = 0 },
		"too many reps": func(d *exercise.Definition) { d.Sets[0].Reps = exercise.MaxReps + 1 },
		"zero duration": func(d *exercise.Definition) { zero := int64(0); d.Sets[0].DurationMicroseconds = &zero },
		"unsafe duration": func(d *exercise.Definition) {
			big := int64(exercise.MaxSafeInteger + 1)
			d.Sets[0].DurationMicroseconds = &big
		},
		"timed group": func(d *exercise.Definition) {
			d.Sets[0].DurationMicroseconds = &duration
			d.Sets[0].Subsets = []exercise.Set{leaf("Child")}
		},
		"blank title": func(d *exercise.Definition) { d.Sets[0].Title = "\u3000" },
		"bad pitch":   func(d *exercise.Definition) { d.CountdownTones = []exercise.Tone{{Pitch: "Cs6", Length: "short"}} },
		"bad length": func(d *exercise.Definition) {
			d.Sets[0].CountdownTones = []exercise.Tone{{Pitch: "C4", Length: "medium"}}
		},
		"skipped only": func(d *exercise.Definition) { d.Sets[0].SkipLast = true },
	}
	for name, change := range tests {
		d := hold()
		change(&d)
		if _, err := exercise.Encode(d); !errors.Is(err, exercise.ErrInvalid) {
			t.Errorf("%s: Encode = %v, want ErrInvalid", name, err)
		}
	}
}

func TestSourceSizeLimits(t *testing.T) {
	padded := holdSource + strings.Repeat(" ", exercise.MaxSourceBytes-len(holdSource))
	decode(t, padded)
	if err := rejects(t, padded+" "); !errors.Is(err, exercise.ErrTooLarge) {
		t.Errorf("oversized source: %v, want ErrTooLarge", err)
	}
	if _, err := exercise.Migrate([]byte(padded + " ")); !errors.Is(err, exercise.ErrTooLarge) {
		t.Errorf("oversized legacy source: %v, want ErrTooLarge", err)
	}
	// Normalization adds null fields, so valid input can encode past the limit.
	d := hold()
	d.Title = "x"
	d.Title = strings.Repeat("x", exercise.MaxSourceBytes-len(encode(t, d))+1)
	if got := encode(t, d); len(got) != exercise.MaxSourceBytes {
		t.Fatalf("encoded %d bytes", len(got))
	}
	d.Title += "é"
	if _, err := exercise.Encode(d); !errors.Is(err, exercise.ErrTooLarge) {
		t.Errorf("oversized encoding: %v, want ErrTooLarge", err)
	}
}

func TestMigrationIsExplicit(t *testing.T) {
	legacy := string(fixtures(t, "invalid")["legacy.json"])
	rejects(t, legacy)
	d, err := exercise.Migrate([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if d.SchemaVersion != exercise.SchemaVersion || d.Sets[0].Description == nil || *d.Sets[0].Description != "Prototype notes" {
		t.Errorf("migrated = %+v", d)
	}
	nested := `{"formatVersion":1.0,"title":"L","countdownTones":[],"sets":[{"title":"Hold","info":"Notes","reps":1,"skipLast":false,` +
		`"subsets":[{"title":"Inner","info":null,"reps":1,"subsets":[],"skipLast":false}]}]}`
	d, err = exercise.Migrate([]byte(nested))
	if err != nil || *d.Sets[0].Description != "Notes" || d.Sets[0].Subsets[0].Description != nil {
		t.Errorf("nested migration = %+v, %v", d, err)
	}
	if got, err := exercise.Migrate([]byte(holdSource)); err != nil || encode(t, got) != holdEncoded {
		t.Errorf("current definition migrates to %+v, %v", got, err)
	}
	for _, source := range []string{
		strings.Replace(legacy, `"formatVersion": 1`, `"formatVersion": 1, "schemaVersion": "0.0.1"`, 1),
		strings.Replace(legacy, `"formatVersion": 1`, `"formatVersion": 1, "schemaVersion": null`, 1),
		strings.Replace(legacy, `"formatVersion": 1`, `"formatVersion": 2`, 1),
		strings.Replace(legacy, `"formatVersion": 1`, `"formatVersion": "1"`, 1),
		strings.Replace(legacy, `"formatVersion": 1`, `"formatVersion": 1.0000000000000001`, 1),
		strings.Replace(legacy, `"info"`, `"description": "b", "info"`, 1),
		"null", "[]",
	} {
		if _, err := exercise.Migrate([]byte(source)); !errors.Is(err, exercise.ErrInvalid) {
			t.Errorf("Migrate(%s) = %v, want ErrInvalid", source, err)
		}
	}
}

// FuzzDecode checks that arbitrary input never panics and that accepted
// definitions encode to a stable canonical form.
func FuzzDecode(f *testing.F) {
	for _, source := range fixtures(f, "valid") {
		f.Add(source)
	}
	f.Add([]byte(withTitle(`"𐀀\udc00"`)))
	f.Fuzz(func(t *testing.T, source []byte) {
		d, err := exercise.Decode(source)
		if err != nil {
			return
		}
		encoded, err := exercise.Encode(d)
		if err != nil {
			if !errors.Is(err, exercise.ErrTooLarge) {
				t.Fatalf("decoded definition fails to encode: %v", err)
			}
			return
		}
		again, err := exercise.Decode(encoded)
		if err != nil {
			t.Fatalf("encoded definition fails to decode: %v\n%s", err, encoded)
		}
		if reencoded, _ := exercise.Encode(again); string(reencoded) != string(encoded) {
			t.Fatalf("unstable encoding:\n%s\n%s", encoded, reencoded)
		}
	})
}
