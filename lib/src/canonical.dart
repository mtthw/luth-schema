import 'dart:convert';
import 'exercise.dart';

Object? _canonicalize(Object? value) {
  if (value is List) return value.map(_canonicalize).toList();
  if (value is Map<String, dynamic>) {
    final keys = value.keys.toList()..sort();
    return {for (final key in keys) key: _canonicalize(value[key])};
  }
  return value;
}

/// Sorted-key compact JSON; use validated exercises for numeric parity.
String canonicalJson(Object? value) => jsonEncode(_canonicalize(value));

/// Validate and serialize an exercise as bounded, sorted-key JSON.
String encodeExercise(Object? input) {
  final source = canonicalJson(parseExercise(input));
  if (utf8.encode(source).length > maxSourceBytes) {
    throw const FormatException('Exercise source is too large');
  }
  return source;
}
