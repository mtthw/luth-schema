import { readFileSync, readdirSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { canonicalJson, countSteps, decodeExercise, encodeExercise, maxDescriptionLength, maxSourceBytes, migrateExercise, parseExercise, validateExercise } from '../src/lib/index.js';

const fixture = (name: string) => JSON.parse(readFileSync(`fixtures/${name}`, 'utf8'));
const hold = () => fixture('valid/hold.json');

describe('shared fixtures', () => {
  for (const validity of ['valid', 'invalid']) {
    for (const file of readdirSync(`fixtures/${validity}`)) {
      it(`${validity}/${file}`, () => {
        expect(validateExercise(fixture(`${validity}/${file}`)).ok).toBe(validity === 'valid');
      });
    }
  }
});

it('normalizes null and omitted fields and ignores object key ordering', () => {
  const exercise = hold();
  const normalized = parseExercise(exercise);
  expect(normalized.description).toBeNull();
  expect(normalized.sets[0]!.description).toBeNull();
  expect(encodeExercise(exercise)).toBe(encodeExercise(normalized));
  expect(canonicalJson({ b: 1, a: 2 })).toBe('{"a":2,"b":1}');
});

it('migrates legacy documents only on explicit request', () => {
  const legacy = fixture('invalid/legacy.json');
  expect(() => parseExercise(legacy)).toThrow();
  const migrated = migrateExercise(legacy);
  expect(migrated.schemaVersion).toBe('0.0.1');
  expect(migrated.sets[0]!.description).toBe('Prototype notes');
  expect(migrated.sets[0]).not.toHaveProperty('info');
  expect(legacy.sets[0].info).toBe('Prototype notes');
  expect(() => migrateExercise({ ...legacy, schemaVersion: '0.0.1' })).toThrow();
  expect(() => migrateExercise({ ...legacy, formatVersion: 2 })).toThrow();
});

it('accepts single ARGB colors and light/dark pairs at both levels', () => {
  const input = hold();
  input.color = { light: 4280311021, dark: 4280243200 };
  input.sets[0].color = 4294967295;
  const parsed = parseExercise(input);
  expect(parsed.color).toEqual({ light: 4280311021, dark: 4280243200 });
  expect(parsed.sets[0]!.color).toBe(4294967295);
  expect(encodeExercise(parsed)).toBe(encodeExercise(parseExercise(parsed)));
  expect(parseExercise(hold()).color).toBeNull();
  const bad = ['0xFF2F80ED', -1, 4294967296, 1.5, { light: 1 }, { dark: 1 }, { light: 1, dark: 2, extra: 3 }, [1, 2], true];
  for (const color of bad) {
    expect(() => parseExercise({ ...hold(), color })).toThrow();
    const withSet = hold();
    withSet.sets[0].color = color as number;
    expect(() => parseExercise(withSet)).toThrow();
  }
});

it('accepts Markdown descriptions up to the code point limit at both levels', () => {
  const described = parseExercise(fixture('valid/description.json'));
  expect(described.description).toContain('# Warm up first');
  expect(described.sets[0]!.description).toBe('Stop if anything **hurts**.');
  // Emoji are single code points but two UTF-16 units, so the limit counts code points.
  const longest = '😀'.repeat(maxDescriptionLength);
  expect(parseExercise({ ...hold(), description: longest }).description).toBe(longest);
  expect(() => parseExercise({ ...hold(), description: `${longest}a` })).toThrow();
  for (const description of [42, true, ['a'], { text: 'a' }]) {
    expect(() => parseExercise({ ...hold(), description })).toThrow();
    const withSet = hold();
    withSet.sets[0].description = description;
    expect(() => parseExercise(withSet)).toThrow();
  }
  expect(() => parseExercise(fixture('invalid/info-field.json'))).toThrow();
});

it('requires an explicitly supported exercise semver string', () => {
  for (const schemaVersion of [1, 0.001, null, '', '0.0', '00.0.1', 'v0.0.1', '0.0.2', '0.1.0', '1.0.0', '0.0.1-beta.1', '0.0.1+build.1']) {
    expect(() => parseExercise({ ...hold(), schemaVersion })).toThrow();
  }
  expect(parseExercise(hold()).schemaVersion).toBe('0.0.1');
});

it('applies manual, timed, group repetition and skipLast semantics', () => {
  expect(countSteps(parseExercise(fixture('valid/group.json')).sets)).toBe(5);
  const manual = fixture('valid/integral-numbers.json');
  manual.sets[0].reps = 100000;
  expect(countSteps(parseExercise(manual).sets)).toBe(1);
  const timed = hold();
  timed.sets[0].reps = 100000;
  expect(countSteps(parseExercise(timed).sets)).toBe(100000);
  timed.sets.push(timed.sets[0]);
  expect(() => parseExercise(timed)).toThrow();
});

it('bounds depth, nodes, source size, and cyclic objects', () => {
  const input = hold();
  let node = input.sets[0];
  for (let depth = 1; depth < 32; depth++) {
    node.durationMicroseconds = null;
    node.subsets = [{ title: 'Nested', reps: 1, subsets: [], skipLast: false }];
    node = node.subsets[0];
  }
  expect(() => parseExercise(input)).not.toThrow();
  node.subsets = [{ title: 'Too deep', reps: 1, subsets: [], skipLast: false }];
  expect(() => parseExercise(input)).toThrow();
  const wide = hold();
  wide.sets = Array.from({ length: 10000 }, () => ({ ...wide.sets[0] }));
  expect(() => parseExercise(wide)).not.toThrow();
  wide.sets.push({ ...wide.sets[0] });
  expect(() => parseExercise(wide)).toThrow();
  const cyclic = hold();
  cyclic.sets[0].subsets = cyclic.sets;
  expect(() => parseExercise(cyclic)).toThrow();
  expect(() => decodeExercise(' '.repeat(maxSourceBytes + 1))).toThrow();
});

it('serializes only validated exercise JSON and round trips every valid fixture', () => {
  for (const file of readdirSync('fixtures/valid')) {
    const input = fixture(`valid/${file}`);
    const source = encodeExercise(input);
    expect(decodeExercise(source)).toEqual(parseExercise(input));
    expect(Object.keys(JSON.parse(source)).sort()).toEqual(['color', 'countdownTones', 'description', 'schemaVersion', 'sets', 'title']);
  }
  expect(() => encodeExercise({ ...hold(), kind: 'exercise' })).toThrow();
  expect(() => encodeExercise(fixture('invalid/empty.json'))).toThrow();
  expect(() => decodeExercise('{')).toThrow();
  const huge = hold();
  huge.title = 'é'.repeat(maxSourceBytes / 2);
  expect(() => encodeExercise(huge)).toThrow();
  expect(() => decodeExercise(JSON.stringify(huge))).toThrow();
});
