import { maxSourceBytes, parseExercise, record } from './exercise.js';

function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalize);
  if (record(value)) return Object.fromEntries(Object.keys(value).sort().map(key => [key, canonicalize(value[key])]));
  return value;
}

/** Sorted-key, compact JSON. Exercise numbers are safe integers after validation. */
export function canonicalJson(value: unknown): string {
  return JSON.stringify(canonicalize(value));
}

/** Validate and serialize an exercise as bounded, sorted-key JSON. */
export function encodeExercise(input: unknown): string {
  const source = canonicalJson(parseExercise(input));
  if (new TextEncoder().encode(source).length > maxSourceBytes) {
    throw new Error('Exercise source is too large');
  }
  return source;
}
