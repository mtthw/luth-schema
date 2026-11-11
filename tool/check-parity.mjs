import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { decodeExercise, encodeExercise, maxDescriptionLength, maxSourceBytes, migrateExercise, validateExercise } from '../dist/src/lib/index.js';

// Each SDK answers a JSON list of {action, value} requests with {ok, source?} results.
const sdks = {
  Dart: ['fvm', ['dart', 'run', 'tool/check_parity.dart']],
  Go: ['go', ['run', './tool/check_parity.go']],
};

function run(name, requests) {
  const [command, args] = sdks[name];
  const result = spawnSync(command, args, {
    input: JSON.stringify(requests), encoding: 'utf8', maxBuffer: 16 * 1024 * 1024,
  });
  assert.equal(result.status, 0, `${name}: ${result.stderr || result.stdout}`);
  return JSON.parse(result.stdout);
}

/** Send requests to every other SDK and check each result against TypeScript. */
function compare(requests, check) {
  for (const name of Object.keys(sdks)) {
    run(name, requests).forEach((result, i) => check(result, i, name));
  }
}

function typeScript(action, value) {
  try {
    if (action === 'validate') return { ok: validateExercise(value).ok };
    const exercise = action === 'decode' ? decodeExercise(value)
      : action === 'migrate' ? migrateExercise(value) : value;
    return { ok: true, source: encodeExercise(exercise) };
  } catch {
    return { ok: false };
  }
}

const fixtures = kind => readdirSync(`fixtures/${kind}`).sort().map(file => JSON.parse(readFileSync(`fixtures/${kind}/${file}`, 'utf8')));
const valid = fixtures('valid');
const base = valid.find(value => value.title === 'Simple hold');
// Exercise every permitted pitch, Unicode/escaping and numeric boundaries.
for (const pitch of ['E2', 'Fs2', 'As3', 'Cs4', 'B5', 'C6']) {
  const input = structuredClone(base);
  input.title = `Lúth ${pitch} \" \\ \n \u2028 \u2029 😀`;
  input.countdownTones = [{ pitch, length: 'automatic' }];
  input.sets[0].durationMicroseconds = 9007199254740991;
  valid.push(input);
}
for (const title of ['\ud800', '\udfff', '\ude00\ud83d', '\ud83d\ud83d', 'é', 'e\u0301', '\u200b', '\u180e', '\u0000',
  Array.from({ length: 32 }, (_, code) => String.fromCharCode(code)).join('') + '\u007f\u0080\u009f']) {
  valid.push({ ...structuredClone(base), title });
}
// Exercise both color forms at root and subset level, including the ARGB bounds.
valid.push({ ...structuredClone(base), color: 4280311021 });
valid.push({ ...structuredClone(base), color: 0 });
valid.push({ ...structuredClone(base), color: 4294967295 });
valid.push({ ...structuredClone(base), color: { light: 4280311021, dark: 4280243200 } });
{
  const paired = structuredClone(base);
  paired.sets[0].color = { light: 4278190080, dark: 4293980160 };
  valid.push(paired);
}
{
  const bounds = structuredClone(base);
  bounds.sets[0].reps = 100000;
  bounds.sets[0].durationMicroseconds = 1;
  valid.push(bounds);
}
// Descriptions at both levels, including the code point limit (emoji are two UTF-16 units).
valid.push({ ...structuredClone(base), description: '# Heading\n\n**Bold** and *italic* — Lúth 😀\n\n- one\n- two' });
valid.push({ ...structuredClone(base), description: '😀'.repeat(maxDescriptionLength) });
valid.push({ ...structuredClone(base), description: '\ud800'.repeat(maxDescriptionLength) });
{
  const described = structuredClone(base);
  described.sets[0].description = '1. First\r\n2. Second';
  valid.push(described);
}
const deep = structuredClone(base);
{
  // The deepest permitted tree. Node-count limits cannot be reached within
  // the source size limit, so each SDK's unit tests cover them instead.
  let node = deep.sets[0];
  for (let depth = 1; depth < 32; depth++) {
    node.durationMicroseconds = null;
    node.subsets = [{ title: 'Nested', reps: 1, subsets: [], skipLast: false }];
    node = node.subsets[0];
  }
  valid.push(deep);
}
compare(valid.map(value => ({ action: 'exercise', value })), (result, i, name) => {
  const source = encodeExercise(valid[i]);
  assert.equal(result.ok, true, `${name} rejected valid vector ${i}`);
  assert.equal(result.source, source, `${name} encoded vector ${i} differently`);
  assert.equal(encodeExercise(decodeExercise(result.source)), source);
});
const reverse = valid.map(value => ({ action: 'decode', value: encodeExercise(value) }));
compare(reverse, (result, i, name) => {
  assert.deepEqual(result, { ok: true, source: reverse[i].value }, `${name} decoded vector ${i} differently`);
});

const candidates = [...valid, ...fixtures('invalid')];
for (const key of ['schemaVersion', 'title', 'countdownTones', 'sets']) {
  const input = structuredClone(valid[0]);
  delete input[key];
  candidates.push(input);
}
for (const schemaVersion of [1, 0.001, null, '', '0.0', '00.0.1', 'v0.0.1', '0.0.2', '0.1.0', '1.0.0', '0.0.1-beta.1', '0.0.1+build.1']) {
  candidates.push({ ...structuredClone(base), schemaVersion });
}
for (const [key, values] of Object.entries({
  title: ['', ' ', '\u0085', '\ufeff', '\u00a0', '\u000b', '\u1680', '\u2000', '\u200a', '\u2007', '\u2028', '\u202f', '\u205f', '\u3000', '\u200b', '\u180e', 3, null],
  reps: [0, -1, 1.5, 100000, 100001, '1', null],
  color: [0, 4294967295, -1, 4294967296, 1.5, '0', true, { light: 1 }, { dark: 1 }, { light: 1, dark: -1 }, { light: 1, dark: 2, extra: 3 }, { light: 1, dark: null }, [1, 2]],
  durationMicroseconds: [0, -1, 0.5, 1, 9007199254740991, 9007199254740992, '1', null],
  skipLast: [0, null, 'false', true],
  subsets: [null, {}, [1]],
  countdownTones: [false, [], null, [{}], [{ pitch: 'Cs6', length: 'short' }], [{ pitch: 'E2\n', length: 'short' }], [{ pitch: 'C4', length: 'medium' }], [{ pitch: 'C4', length: 'short', extra: 1 }]],
})) {
  for (const value of values) {
    const input = structuredClone(base);
    input.sets[0][key] = value;
    candidates.push(input);
  }
}
for (const color of ['red', -1, { light: 1 }, true]) {
  candidates.push({ ...structuredClone(base), color });
}
for (const countdownTones of [null, [{ pitch: 'D2', length: 'short' }]]) {
  candidates.push({ ...structuredClone(base), countdownTones });
}
const tooLong = '😀'.repeat(maxDescriptionLength) + 'a';
for (const description of [42, true, [], {}, tooLong, 'a'.repeat(maxDescriptionLength + 1), '\ud800'.repeat(maxDescriptionLength + 1)]) {
  candidates.push({ ...structuredClone(base), description });
  const withSet = structuredClone(base);
  withSet.sets[0].description = description;
  candidates.push(withSet);
}
{
  const withInfo = structuredClone(base);
  withInfo.sets[0].info = 'Renamed to description';
  candidates.push(withInfo);
  const tooDeep = structuredClone(deep);
  let node = tooDeep.sets[0];
  while (node.subsets.length) node = node.subsets[0];
  node.subsets = [{ title: 'Too deep', reps: 1, subsets: [], skipLast: false }];
  candidates.push(tooDeep);
  // A skipped step in an ancestor does not exempt a subtree from the step budget.
  const leaf = { title: 'Hold', reps: 100000, durationMicroseconds: 1, subsets: [], skipLast: false };
  candidates.push({ ...structuredClone(base), sets: [{ title: 'Group', reps: 1, skipLast: true, subsets: [leaf, { ...leaf, reps: 1 }] }] });
  candidates.push({ ...structuredClone(base), sets: [{ ...leaf, skipLast: true }, { ...leaf, reps: 1 }] });
  candidates.push({ ...structuredClone(base), sets: [leaf, { ...leaf, reps: 1 }] });
}
compare(candidates.map(value => ({ action: 'validate', value })), (result, i, name) => {
  assert.equal(result.ok, validateExercise(candidates[i]).ok, `${name} validation mismatch: ${JSON.stringify(candidates[i]).slice(0, 300)}`);
});

// Raw sources: JSON syntax, number notation, escapes, duplicate keys and size limits.
const json = JSON.stringify(base);
const withSet = (field, literal) => json.replace('"skipLast":false', `"skipLast":false,"${field}":${literal}`);
const withTitle = literal => json.replace('"title":"Simple hold"', `"title":${literal}`);
const sources = ['', ' ', '{', 'null', '[]', '"exercise"', '1', ' '.repeat(262145),
  JSON.stringify({ ...base, schemaVersion: '0.0.2' }),
  JSON.stringify({ ...base, kind: 'exercise', transferId: 'not-exercise-content' }),
  ` \t\r\n${json} \t\r\n`, `\ufeff${json}`, `${json},`, `${json}{}`, `${json} null`, `[${json}]`,
  json.replace('}]', '},]'), json.replace('"title"', "'title'"), json.replace('"title"', 'title'), `/* note */${json}`,
  json.replace('"reps":1', '"reps":01'), json.replace('"reps":1', '"reps":+1'), json.replace('"reps":1', '"reps":1.'),
  json.replace('"reps":1', '"reps":.1e1'), json.replace('"reps":1', '"reps":NaN'), json.replace('"reps":1', '"reps":Infinity'),
  json.replace('"reps":1', '"reps":0x1'), json.replace('"reps":1', '"reps":- 1'), json.replace('"reps":1', '"reps":1e'),
  json.replace('"reps":1', '"reps":1.0'), json.replace('"reps":1', '"reps":1e0'), json.replace('"reps":1', '"reps":1E+0'),
  json.replace('"reps":1', '"reps":10e-1'), json.replace('"reps":1', '"reps":0.1e1'), json.replace('"reps":1', '"reps":1.000'),
  json.replace('"reps":1', '"reps":1e5'), json.replace('"reps":1', '"reps":100000.0'), json.replace('"reps":1', '"reps":1.00001e5'),
  json.replace('"reps":1', '"reps":-0'), json.replace('"reps":1', '"reps":0.5e1'), json.replace('"reps":1', '"reps":1e999999999999'),
  json.replace('"reps":1', '"reps":true'), json.replace('"reps":1', '"reps":tru'), json.replace('"reps":1', '"reps":nul'),
  withSet('color', '-0'), withSet('color', '0e5'), withSet('color', '4294967295.0'), withSet('color', '42949672.95e2'),
  withSet('color', '4294967296'), withSet('color', '1e400'), withSet('color', '-1e400'),
  json.replace('30000000', '9007199254740991'), json.replace('30000000', '9007199254740991.0'),
  json.replace('30000000', '9007199254740992'), json.replace('30000000', '9007199254740993'), json.replace('30000000', '9.007199254740991e15'),
  json.replace('30000000', '90071992547409910e-1'), json.replace('30000000', '1e16'),
  withTitle('"\\ud83d\\ude00"'), withTitle('"\\uD83D\\uDE00"'), withTitle('"\\ud800"'), withTitle('"\\udc00\\ud800"'),
  withTitle('"\\ud800\\u0041"'), withTitle('"\\ud800\\n"'), withTitle('"\\u0000"'), withTitle('"\\/\\"\\\\\\b\\f\\n\\r\\t"'),
  withTitle('"\\u00e9"'), withTitle('"\\x41"'), withTitle('"\\u12"'), withTitle('"\\u12G4"'), withTitle('"\\ud800\\u12"'),
  withTitle('"a\tb"'), withTitle('"a\nb"'), withTitle('"a\u007fb"'), withTitle('"\\u0020"'), withTitle('"\\u00a0"'), withTitle('"unterminated'),
  // Duplicate keys keep the last value, as JSON.parse and Dart's jsonDecode do.
  json.replace('"title":"Simple hold"', '"title":"First","title":"Last"'),
  json.replace('"title":"Simple hold"', '"title":"First","title":" "'),
  json.replace('"title":"Simple hold"', '"title":" ","title":"Last"'),
  json.replace('"skipLast":false', '"skipLast":true,"skipLast":false'),
  // Nesting far beyond any valid definition.
  json.replace('"title":"Simple hold"', `"title":${'['.repeat(100000)}${']'.repeat(100000)}`),
  json.replace('"title":"Simple hold"', `"title":${'{"a":'.repeat(10000)}1${'}'.repeat(10000)}`),
];
// The source limit: exactly maxSourceBytes is accepted, one byte more is not.
sources.push(json + ' '.repeat(maxSourceBytes - json.length), json + ' '.repeat(maxSourceBytes - json.length + 1));
const multibyte = withTitle(JSON.stringify('é'.repeat(1000)));
const multibyteLength = new TextEncoder().encode(multibyte).length;
sources.push(multibyte + ' '.repeat(maxSourceBytes - multibyteLength), multibyte + ' '.repeat(maxSourceBytes - multibyteLength + 1));
// The encoded limit: decoded input under the limit whose normalized encoding is exactly at, or over, it.
{
  const many = { ...structuredClone(base), sets: Array.from({ length: 1000 }, () => structuredClone(base.sets[0])) };
  const encodedLength = encodeExercise(many).length;
  const atLimit = { ...many, title: 'x'.repeat(maxSourceBytes - encodedLength + 'Simple hold'.length) };
  assert.equal(encodeExercise(atLimit).length, maxSourceBytes);
  sources.push(JSON.stringify(atLimit), JSON.stringify({ ...atLimit, title: `${atLimit.title}x` }));
}
compare(sources.map(value => ({ action: 'decode', value })), (result, i, name) => {
  assert.deepEqual(result, typeScript('decode', sources[i]), `${name} decode mismatch for source ${i}: ${sources[i].slice(0, 200)}`);
});

// TypeScript and Dart parse numbers as doubles, so these literals round to an
// accepted integer there. Go checks the exact literal and rejects them.
const rounded = [
  withSet('color', '4294967295.0000001'), withSet('color', '1e-400'),
  json.replace('"reps":1', '"reps":1.0000000000000001'),
  json.replace('30000000', '9007199254740990.9999'),
];
const goRounded = run('Go', rounded.map(value => ({ action: 'decode', value })));
const dartRounded = run('Dart', rounded.map(value => ({ action: 'decode', value })));
rounded.forEach((source, i) => {
  assert.equal(typeScript('decode', source).ok, true, `TypeScript no longer rounds ${source}`);
  assert.deepEqual(dartRounded[i], typeScript('decode', source), `Dart differs from TypeScript for ${source}`);
  assert.deepEqual(goRounded[i], { ok: false }, `Go accepted rounded number ${source}`);
});

// Legacy migration renames subset info to description in every SDK.
const legacy = [
  { formatVersion: 1, title: 'Legacy', countdownTones: [], sets: [{ title: 'Hold', info: 'Notes', reps: 1, subsets: [{ title: 'Inner', info: 'Nested', reps: 1, subsets: [], skipLast: false }], skipLast: false }] },
  { formatVersion: 1, title: 'Both', countdownTones: [], sets: [{ title: 'Hold', info: 'a', description: 'b', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 1, title: 'Null info', countdownTones: [], sets: [{ title: 'Hold', info: null, reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 1, title: 'No info', countdownTones: [], color: 1, sets: [{ title: 'Hold', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 1, schemaVersion: '0.0.1', title: 'Mixed', countdownTones: [], sets: [{ title: 'Hold', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 1, schemaVersion: null, title: 'Mixed', countdownTones: [], sets: [{ title: 'Hold', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 2, title: 'Future', countdownTones: [], sets: [{ title: 'Hold', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: '1', title: 'String', countdownTones: [], sets: [{ title: 'Hold', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 1, title: 'Root info', info: 'x', countdownTones: [], sets: [{ title: 'Hold', reps: 1, subsets: [], skipLast: false }] },
  { formatVersion: 1, title: 'Bad sets', countdownTones: [], sets: {} },
  structuredClone(base),
  null,
  [],
];
compare(legacy.map(value => ({ action: 'migrate', value })), (result, i, name) => {
  assert.deepEqual(result, typeScript('migrate', legacy[i]), `${name} migration mismatch: ${JSON.stringify(legacy[i])}`);
});
console.log(`Parity passed for TypeScript, Dart and Go: ${valid.length} bidirectional JSON vectors, ${candidates.length} validation cases, ${sources.length} raw sources, ${rounded.length} documented rounding differences, ${legacy.length} migrations.`);
