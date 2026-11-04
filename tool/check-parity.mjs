import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { decodeExercise, encodeExercise, validateExercise } from '../dist/src/lib/index.js';

function dart(requests) {
  const result = spawnSync('fvm', ['dart', 'run', 'tool/check_parity.dart'], {
    input: JSON.stringify(requests), encoding: 'utf8', maxBuffer: 4 * 1024 * 1024,
  });
  assert.equal(result.status, 0, result.stderr || result.stdout);
  return JSON.parse(result.stdout);
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
for (const title of ['\ud800', '\udfff', 'é', 'e\u0301']) {
  valid.push({ ...structuredClone(base), title });
}
// Exercise both color forms at root and subset level.
valid.push({ ...structuredClone(base), color: 4280311021 });
valid.push({ ...structuredClone(base), color: { light: 4280311021, dark: 4280243200 } });
{
  const paired = structuredClone(base);
  paired.sets[0].color = { light: 4278190080, dark: 4293980160 };
  valid.push(paired);
}
const encoded = dart(valid.map(value => ({ action: 'exercise', value })));
const reverse = [];
for (let i = 0; i < valid.length; i++) {
  const input = valid[i];
  assert.equal(encoded[i].ok, true);
  const source = encodeExercise(input);
  assert.equal(encoded[i].source, source);
  assert.equal(encodeExercise(decodeExercise(encoded[i].source)), source);
  reverse.push({ action: 'decode', value: source });
}
const decoded = dart(reverse);
decoded.forEach((result, i) => {
  assert.equal(result.ok, true);
  assert.equal(result.source, encodeExercise(valid[i]));
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
  title: ['', ' ', '\u0085', '\ufeff', '\u2007', '\u202f', '\u200b', 3],
  reps: [0, -1, 1.5, 100001, '1', null],
  color: [-1, 4294967296, 1.5, '0', true, { light: 1 }, { dark: 1 }, { light: 1, dark: -1 }, { light: 1, dark: 2, extra: 3 }, [1, 2]],
  durationMicroseconds: [0, -1, 0.5, 9007199254740992, '1'],
  skipLast: [0, null, 'false'],
  countdownTones: [false, [{}], [{ pitch: 'Cs6', length: 'short' }], [{ pitch: 'E2\n', length: 'short' }]],
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
dart(candidates.map(value => ({ action: 'validate', value }))).forEach((result, i) => {
  assert.equal(result.ok, validateExercise(candidates[i]).ok, `Validation mismatch: ${JSON.stringify(candidates[i])}`);
});

const malformed = ['', '{', 'null', '[]', ' '.repeat(262145),
  JSON.stringify({ ...base, schemaVersion: '0.0.2' }),
  JSON.stringify({ ...base, kind: 'exercise', transferId: 'not-exercise-content' }),
];
dart(malformed.map(value => ({ action: 'decode', value }))).forEach((result, i) => {
  assert.equal(result.ok, false, `Dart accepted malformed exercise ${i}`);
  assert.throws(() => decodeExercise(malformed[i]), `TypeScript accepted malformed exercise ${i}`);
});
console.log(`Parity passed: ${valid.length} bidirectional JSON vectors, ${candidates.length} validation cases, ${malformed.length} malformed exercises.`);
