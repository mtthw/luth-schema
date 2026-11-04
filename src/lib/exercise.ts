import { Ajv2020 } from 'ajv/dist/2020.js';
import schema from '../../schemas/exercise.schema.json' with { type: 'json' };

export interface CountdownTone {
  pitch: string;
  length: 'automatic' | 'short' | 'long';
}

/** A single ARGB integer, or an explicit light/dark mode pair. */
export type ExerciseColor = number | { light: number; dark: number };

export interface ExerciseSet {
  title: string;
  reps: number;
  subsets: ExerciseSet[];
  skipLast: boolean;
  info: string | null;
  durationMicroseconds: number | null;
  color: ExerciseColor | null;
  countdownTones: CountdownTone[] | null;
}

export interface ExerciseDefinition {
  schemaVersion: '0.0.1';
  title: string;
  countdownTones: CountdownTone[];
  color: ExerciseColor | null;
  sets: ExerciseSet[];
}

export const maxDepth = 32;
export const maxNodes = 10_000;
export const maxSteps = 100_000;
export const maxSourceBytes = 262_144;

const checkSchema = new Ajv2020({ allErrors: false }).compile(schema);

export function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

// Bound recursive work before asking the schema engine to walk untrusted input.
function preflight(input: unknown): void {
  const pending: { value: unknown; depth: number }[] = [{ value: input, depth: -1 }];
  const seen = new Set<object>();
  let nodes = 0;
  while (pending.length) {
    const { value, depth } = pending.pop()!;
    if (!record(value)) continue;
    if (seen.has(value)) throw new Error('Exercise contains a cycle or shared node');
    seen.add(value);
    if (depth >= 0 && (depth >= maxDepth || ++nodes > maxNodes)) {
      throw new Error('Exercise definition is too complex');
    }
    const children = depth === -1 ? value.sets : value.subsets;
    if (Array.isArray(children)) {
      if (children.length > maxNodes) throw new Error('Too many subsets');
      for (const child of children) pending.push({ value: child, depth: depth + 1 });
    }
  }
}

function tones(value: unknown): CountdownTone[] {
  return (value as CountdownTone[]).map(({ pitch, length }) => ({ pitch, length }));
}

function normalizeSet(value: unknown): ExerciseSet {
  const set = value as ExerciseSet;
  return {
    title: set.title, reps: set.reps, skipLast: set.skipLast,
    info: set.info ?? null,
    durationMicroseconds: set.durationMicroseconds ?? null,
    color: set.color ?? null,
    countdownTones: set.countdownTones == null ? null : tones(set.countdownTones),
    subsets: set.subsets.map(normalizeSet),
  };
}

export function countSteps(sets: ExerciseSet[]): number {
  let count = 0;
  for (const set of sets) {
    const leaves = set.subsets.length ? countSteps(set.subsets) * set.reps
      : set.durationMicroseconds !== null ? set.reps : 1;
    count += leaves - (set.skipLast && leaves > 0 ? 1 : 0);
    if (count > maxSteps) throw new Error('Exercise expands to too many executable steps');
  }
  return count;
}

/** Validate and copy a definition, normalizing omitted optional fields to null. */
export function parseExercise(input: unknown): ExerciseDefinition {
  preflight(input);
  if (!checkSchema(input)) {
    const error = checkSchema.errors?.[0];
    throw new Error(`${error?.instancePath || '/'} ${error?.message || 'Invalid exercise'}`);
  }
  const value = input as unknown as ExerciseDefinition;
  const exercise: ExerciseDefinition = {
    schemaVersion: '0.0.1', title: value.title,
    countdownTones: tones(value.countdownTones), color: value.color ?? null,
    sets: value.sets.map(normalizeSet),
  };
  if (countSteps(exercise.sets) === 0) throw new Error('An exercise needs an executable step');
  return exercise;
}

export type ValidationResult =
  | { ok: true; exercise: ExerciseDefinition }
  | { ok: false; errors: string[] };

export function validateExercise(input: unknown): ValidationResult {
  try { return { ok: true, exercise: parseExercise(input) }; }
  catch (error) { return { ok: false, errors: [error instanceof Error ? error.message : String(error)] }; }
}

export function decodeExercise(source: string): ExerciseDefinition {
  if (new TextEncoder().encode(source).length > maxSourceBytes) throw new Error('Exercise source is too large');
  return parseExercise(JSON.parse(source));
}

/** Explicit opt-in migration; ordinary validation never accepts legacy fields. */
export function migrateExercise(input: unknown): ExerciseDefinition {
  if (record(input) && input.formatVersion === 1 && !('schemaVersion' in input)) {
    const { formatVersion: _, ...rest } = input;
    return parseExercise({ ...rest, schemaVersion: '0.0.1' });
  }
  return parseExercise(input);
}
