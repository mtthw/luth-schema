# Lúth schema

The portable exercise contract for Lúth, with small TypeScript,
Dart and Go SDKs. No Flutter, React, database, account or server dependency.

Definition schema 0.0.1 is an initial **draft**. Consumers have not yet been migrated. Nothing
here publishes packages or manages releases; there is no release-please setup.

## Contents

- `schemas/`: the JSON Schema draft 2020-12 exercise definition.
- `src/lib/`: browser-safe TypeScript validation, serialization and migration helpers.
- `lib/`: Flutter-free Dart validation, serialization and migration helpers.
- `exercise/`: standard-library-only Go decoding, validation, encoding and migration.
- `fixtures/`: shared valid/invalid exercise examples.
- `test/`, `exercise/*_test.go` and `tool/`: SDK tests and cross-language
  TypeScript/Dart/Go compatibility checks.

## Versioning

An exercise carries `"schemaVersion": "0.0.1"`. Its stable schema ID is
`urn:luth:schema:exercise:0.0.1`; it does not depend on owning a domain.

Two kinds of version exist and are independent:

- The **definition schema version** is the `schemaVersion` field inside each
  exercise document, currently `"0.0.1"`. It describes the document format.
- The **SDK version** is the release of this repository (the `v0.0.2` Git tag,
  `package.json`/`pubspec.yaml` version, and Go module version). A new SDK
  release can fix bugs or add APIs without changing the supported schema version.

Document versions are semver strings.
After 1.0, major versions signal breaking contract changes, minor versions
backward-compatible additions, and patches compatible fixes or clarifications.
During 0.x the contract is experimental and compatibility is not guaranteed.
The SDKs currently accept **only `"0.0.1"`**, not arbitrary semver ranges:
support for another version requires explicit validation and migration rules.
Incompatible document changes require an explicit migration. Package releases
also use semver independently; pin consumers to a reviewed commit during 0.x.

Validation never silently upgrades a document. `migrateExercise` explicitly
converts the prototype's `formatVersion: 1` to `schemaVersion: "0.0.1"`, renaming
subset `info` to `description`, then validates the result. Current documents are
accepted too. Unknown versions and mixed legacy/new version fields are rejected. This is a document migration, not a
SQLite migration or an exercise's saved revision.

## Exercise contract

See [`fixtures/valid/hold.json`](fixtures/valid/hold.json) for a complete example.
An exercise has a title, default countdown tones and a nonempty `sets` array.
Each subset has a title, `reps`, `subsets` and `skipLast`.

- A leaf has `subsets: []`. This is not an empty exercise: a timed leaf runs for
  `durationMicroseconds`; a manual leaf waits for the user.
- Timed leaves repeat `reps` times. Groups repeat their child sequence `reps`
  times. Manual leaf `reps` is a prescription, not repeated execution.
- `skipLast` removes the last expanded leaf of that node. An exercise must
  still contain at least one executable step.
- A timed node cannot have children. Nesting is at most 32 subset levels,
  with at most 10,000 nodes and 100,000 executable steps. Every subtree must
  obey the step budget, even if an ancestor would skip a step.
- Titles cannot be blank. Tones accept pitches E2–C6 (`s` for sharps) and
  lengths `automatic`, `short` or `long`.
- A colour is an unsigned 32-bit ARGB integer, or an object with `light` and
  `dark` ARGB integers for mode-dependent colours. The exercise root and each
  subset accept an optional `color` of either form.
- The exercise root and each subset accept an optional `description`: Markdown
  text of at most 10,000 Unicode code points, displayed as described in
  [Descriptions](#descriptions).
- `description`, `color`, `durationMicroseconds` and subset `countdownTones`
  can be omitted or null. SDKs normalize omissions to null without changing input.
- Root tones are defaults. Subset null/omission inherits those defaults; `[]`
  means silence. A group's tone override does not cascade to descendants.
- Duration is a positive integer no greater than 9,007,199,254,740,991, for
  exact Dart/JavaScript interchange. Mathematically integral JSON numbers
  such as `1.0` are accepted and normalized to integers in Dart. See
  [Number precision](#number-precision) for a difference in the Go SDK.
- Unknown fields are rejected, not silently discarded. Inputs are JSON trees;
  runtime objects, aliases, cycles and non-JSON values are not part of the API.

### Descriptions

A `description` is [CommonMark](https://spec.commonmark.org/) source. Apps
parse it with a CommonMark parser and display only this subset, so the same
text looks the same everywhere:

- Headings: levels 1–3. Deeper headings display as level 3.
- Paragraphs, with **bold** (`**`), *italic* (`*`) and both combined.
- Bulleted (`-`, `*`) and numbered (`1.`) lists, one level deep. Nested lists
  continue as items of the outer list; an item with several paragraphs joins
  them with a line break.
- A soft line break displays as a space; a hard break (two trailing spaces or
  a backslash) as a line break.

Everything else falls back to plain text and is never interpreted: a link
shows its text, an image its alt text, code its contents (a code block as a
paragraph), and raw HTML its literal source. Block quotes show their contents;
thematic breaks are dropped. Adjacent text with the same formatting merges, and
empty blocks are dropped.

[`fixtures/markdown`](fixtures/markdown) pairs each `.md` source with the
expected display as JSON: a list of `heading` (`level`, `spans`), `paragraph`
(`spans`) and `list` (`ordered`, `items` of spans) blocks, where a span is
`{text, bold, italic}`. App renderers should test against these fixtures.

JSON Schema checks structure, **not** expanded execution or nesting budgets.
Use SDK validation for untrusted definitions. `encodeExercise` and `decodeExercise`
cap UTF-8 JSON source at 262,144 bytes. Already-parsed `parseExercise` validates
structure and execution, not serialized source size.

## TypeScript use

For local development, build this repo, then add a local dependency:

```sh
# In luth-schema
npm ci
# In the consuming project, when deliberately integrating it
npm install ../luth-schema
```

Once the `v0.0.2` tag is available on GitHub, use it in the consumer's `package.json`
(or replace it with a full reviewed commit hash):

```json
{"dependencies": {"luth-schema": "git+https://github.com/mtthw/luth-schema.git#v0.0.2"}}
```

The Git `prepare` hook builds the TypeScript package; Node >=22.12 and npm are
required during installation. Keep lifecycle scripts enabled for Git installs.
`private: true` prevents accidental registry publishing, not Git dependency use.

```ts
import { decodeExercise, encodeExercise } from 'luth-schema';

const exercise = decodeExercise(source);
const json = encodeExercise(exercise);
```

Other exports include `parseExercise`, `validateExercise` (a result rather than
an exception), `migrateExercise`, `countSteps` and `canonicalJson`. The schema is
available at `luth-schema/schemas/exercise.schema.json`.

## Dart use

Use a local path while integrating, or pin the public repository at a release tag
or full commit hash once it is available on GitHub:

```yaml
dependencies:
  luth_schema:
    path: ../luth-schema
# Or:
#  luth_schema:
#    git:
#      url: https://github.com/mtthw/luth-schema.git
#      ref: v0.0.2
```

```dart
import 'package:luth_schema/luth_schema.dart';

final exercise = decodeExercise(source);
final json = encodeExercise(exercise);
```

The Dart API returns copied JSON maps/lists; consumers adapt them to their own
models. Validation throws `FormatException`. The package has no `dart:io` or
Flutter dependency and can also be used on the web.

## Go use

Module path `github.com/mtthw/luth-schema`, package
`github.com/mtthw/luth-schema/exercise`. It requires Go 1.22 or later and has
no dependencies outside the standard library. During integration, use a local
replacement; once the `v0.0.2` tag is on GitHub, require that version (the Go
module version is the repository's SDK tag, not the schema version):

```sh
go get github.com/mtthw/luth-schema@v0.0.2
# Or, while integrating locally:
go mod edit -replace github.com/mtthw/luth-schema=../luth-schema
```

```go
import "github.com/mtthw/luth-schema/exercise"

definition, err := exercise.Decode(source) // []byte, at most 262,144 bytes
if errors.Is(err, exercise.ErrInvalid) {
	// Reject the import; errors.Is(err, exercise.ErrTooLarge) for oversized source.
}
json, err := exercise.Encode(definition) // validated, canonical, bounded
```

`Decode` parses strict JSON, validates every rule (including nesting, node and
step budgets) and returns a normalized `Definition`: omitted or null optional
fields are nil, and a subset's `CountdownTones` is nil to inherit or a non-nil
empty slice for silence. `Definition.Validate` checks a value built in Go, and
`Encode` validates before writing the same canonical bytes as the other SDKs.
`Migrate` is the explicit legacy conversion; `Decode` never migrates.
`CountSteps` counts executable steps. Errors wrap `ErrInvalid`.

The Go API takes JSON bytes rather than already-parsed values, so `Decode` and
`Migrate` always apply the source-size limit. Use `json.RawMessage` to pass an
embedded exercise through `encoding/json` unchanged. A lone UTF-16 surrogate
escape such as `"\ud800"` is kept in its WTF-8 form and encoded back as the same
escape, so Go output stays byte-identical to TypeScript and Dart.

### Number precision

Go reads JSON numbers from their exact text and accepts only literals that are
mathematically integers within range: `2`, `2.0` and `2e0` are accepted,
`4294967295.0000001` and `1e-400` are not. TypeScript and Dart parse numbers as
64-bit floats, which round those literals to an accepted integer (`4294967295`
and `0`). The parity check records these cases; they only arise from literals
no SDK writes.

## Serialization and application boundaries

`encodeExercise` validates and normalizes the exercise, then returns the exercise
JSON directly: there is no wrapper, transfer ID or separate transfer schema.
Output is compact JSON with object keys sorted by UTF-16 code units, preserving
strings and array order without Unicode normalization. Strings are escaped as
`JSON.stringify` does: quotes, backslashes, control characters and lone surrogates. All numbers are safe integers.
`canonicalJson` is a lower-level sorted-key serializer and does not validate input.

The web and app own QR encoding, compression, links, trusted-origin policies and
bounded decompression. They also decide how to interpret an import, ask for
confirmation, handle duplicates and assign storage identities. This package
only validates the resulting exercise content; it does not prescribe a QR format.

### Differences from the private prototypes

- Exercise `formatVersion` becomes `schemaVersion` through explicit migration.
- Unknown fields now fail, and duration is limited to JavaScript safe integers.
- Integral floating-point JSON notation is accepted consistently in Dart and TS.
- JSON serialization and decoding enforce the source-size limit.

Consumers that support legacy QR links must decode those links themselves and
explicitly migrate the extracted exercise. No consumer repository is modified here.

## Development

Use Node >=22.12, npm, `just`, FVM and Go >=1.22. `.fvmrc` pins the Dart SDK via Flutter;
the package itself does not depend on Flutter. No global packages are installed.

```sh
just install
just check       # TS build/types/tests, Dart analyze/format/tests, Go fmt/vet/tests, parity
just test        # the three SDK test suites only
just fuzz 5m     # fuzz the Go decoder
just pack-check  # inspect npm package contents without publishing
go test ./exercise         # Go tests alone
npm run build && npm run parity  # cross-language parity alone
```

All SDKs run the same fixtures. The parity runner (`tool/check-parity.mjs`)
sends the same definitions and raw JSON sources to Dart and Go and compares
their acceptance and encoded bytes with TypeScript, in both directions:
validation, normalization, serialization, numeric and Unicode boundaries,
JSON syntax, duplicate keys, size limits and migration.

## Git releases

No npm, pub.dev or Go proxy publication step is required: consumers can install from the
public Git repository. `private: true` and `publish_to: none` intentionally prevent
accidental registry publication.

To release, bump the SDK version in `package.json`, `package-lock.json`,
`pubspec.yaml` and the dependency examples above (not the schema version), run
`just check` and `just pack-check`, review and commit the files, then create an
annotated `vX.Y.Z` tag on that commit. The commit and tag must then be pushed by
the maintainer before the dependency examples above will work.
The same `vX.Y.Z` tag is the Go module version, because `go.mod` is at the
repository root.
Never move a released tag. Commit each consumer's lockfile after installing it.

Adding the dependency does not replace a consumer's existing codec; integration
and any required legacy-import support are separate changes.
