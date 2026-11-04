import 'dart:convert';

typedef JsonObject = Map<String, dynamic>;

const maxDepth = 32;
const maxNodes = 10000;
const maxSteps = 100000;
const maxSourceBytes = 262144;
const maxSafeInteger = 9007199254740991;

Never _fail(String message) => throw FormatException(message);

JsonObject object(Object? value) {
  if (value is! Map<String, dynamic>) _fail('Expected a JSON object');
  return value;
}

void _keys(JsonObject value, List<String> required, List<String> optional) {
  if (required.any((key) => !value.containsKey(key)) ||
      value.keys.any(
        (key) => !required.contains(key) && !optional.contains(key),
      )) {
    _fail('Missing or unknown fields');
  }
}

int _integer(Object? value, int minimum, int maximum) {
  if (value is! num ||
      !value.isFinite ||
      value != value.truncateToDouble() ||
      value < minimum ||
      value > maximum) {
    _fail('Expected an integer from $minimum to $maximum');
  }
  return value.toInt();
}

String _title(Object? value) {
  // Match ECMAScript whitespace plus NEL, consistently with the JSON Schema.
  final blank = RegExp(r'^[\s\u0085\uFEFF]*$', unicode: true);
  if (value is! String || blank.hasMatch(value)) {
    _fail('Expected a nonblank title');
  }
  return value;
}

List<dynamic> _list(Object? value) {
  if (value is! List) _fail('Expected a list');
  return value;
}

List<JsonObject> _tones(Object? value) => _list(value).map((item) {
  final tone = object(item);
  _keys(tone, ['pitch', 'length'], []);
  const classes = [
    'C',
    'Cs',
    'D',
    'Ds',
    'E',
    'F',
    'Fs',
    'G',
    'Gs',
    'A',
    'As',
    'B',
  ];
  final pitch = tone['pitch'];
  final match = pitch is String
      ? RegExp(r'^([A-G]s?)([2-6])$').firstMatch(pitch)
      : null;
  if (match == null ||
      match.end != (pitch as String).length ||
      !classes.contains(match[1])) {
    _fail('Invalid tone pitch');
  }
  final midi = (int.parse(match[2]!) + 1) * 12 + classes.indexOf(match[1]!);
  if (midi < 40 || midi > 84) _fail('Pitch must be E2 through C6');
  if (!['automatic', 'short', 'long'].contains(tone['length'])) {
    _fail('Invalid tone length');
  }
  return <String, dynamic>{'pitch': pitch, 'length': tone['length']};
}).toList();

class _Budget {
  int nodes = 0;
}

// A color is a single ARGB integer or an explicit light/dark mode pair.
Object? _color(Object? value) {
  if (value == null) return null;
  if (value is Map) {
    if (value.length != 2 ||
        !value.containsKey('light') ||
        !value.containsKey('dark')) {
      _fail('Expected light and dark ARGB integers');
    }
    return {
      'light': _integer(value['light'], 0, 0xffffffff),
      'dark': _integer(value['dark'], 0, 0xffffffff),
    };
  }
  return _integer(value, 0, 0xffffffff);
}

JsonObject _subset(Object? value, int depth, _Budget budget) {
  if (depth >= maxDepth || ++budget.nodes > maxNodes) {
    _fail('Exercise definition is too complex');
  }
  final set = object(value);
  _keys(
    set,
    ['title', 'reps', 'subsets', 'skipLast'],
    ['info', 'durationMicroseconds', 'color', 'countdownTones'],
  );
  final color = _color(set['color']);
  final children = _list(set['subsets']);
  if (children.length > maxNodes) _fail('Too many subsets');
  final duration = set['durationMicroseconds'] == null
      ? null
      : _integer(set['durationMicroseconds'], 1, maxSafeInteger);
  if (duration != null && children.isNotEmpty) {
    _fail('A timed subset cannot contain subsets');
  }
  if (set['info'] != null && set['info'] is! String) _fail('Invalid info');
  if (set['skipLast'] is! bool) _fail('Invalid skipLast');
  return {
    'title': _title(set['title']),
    'reps': _integer(set['reps'], 1, maxSteps),
    'skipLast': set['skipLast'],
    'info': set['info'],
    'durationMicroseconds': duration,
    'color': color,
    'countdownTones': set['countdownTones'] == null
        ? null
        : _tones(set['countdownTones']),
    'subsets': children
        .map((child) => _subset(child, depth + 1, budget))
        .toList(),
  };
}

/// Count executable leaves without expanding repeated groups.
int countSteps(List<dynamic> sets) {
  var count = 0;
  for (final value in sets) {
    final set = object(value);
    final children = set['subsets'] as List<dynamic>;
    final reps = set['reps'] as int;
    final leaves = children.isNotEmpty
        ? countSteps(children) * reps
        : set['durationMicroseconds'] != null
        ? reps
        : 1;
    count += leaves - (set['skipLast'] == true && leaves > 0 ? 1 : 0);
    if (count > maxSteps) {
      _fail('Exercise expands to too many executable steps');
    }
  }
  return count;
}

/// Validate and copy a definition, normalizing optional fields to null.
JsonObject parseExercise(Object? input) {
  final value = object(input);
  _keys(value, ['schemaVersion', 'title', 'countdownTones', 'sets'], ['color']);
  if (value['schemaVersion'] != '0.0.1') {
    _fail('Unsupported exercise schema version');
  }
  final sets = _list(value['sets']);
  if (sets.isEmpty || sets.length > maxNodes) _fail('Invalid sets count');
  final budget = _Budget();
  final exercise = <String, dynamic>{
    'schemaVersion': '0.0.1',
    'title': _title(value['title']),
    'countdownTones': _tones(value['countdownTones']),
    'color': _color(value['color']),
    'sets': sets.map((set) => _subset(set, 0, budget)).toList(),
  };
  if (countSteps(exercise['sets'] as List<dynamic>) == 0) {
    _fail('An exercise needs an executable step');
  }
  return exercise;
}

/// Decode bounded JSON. Throws FormatException on invalid definitions.
JsonObject decodeExercise(String source) {
  if (utf8.encode(source).length > maxSourceBytes) {
    _fail('Exercise source is too large');
  }
  return parseExercise(jsonDecode(source));
}

/// Explicit migration from the prototype's formatVersion: 1 document.
JsonObject migrateExercise(Object? input) {
  final value = object(input);
  if (value['formatVersion'] == 1 && !value.containsKey('schemaVersion')) {
    final migrated = Map<String, dynamic>.of(value)..remove('formatVersion');
    migrated['schemaVersion'] = '0.0.1';
    return parseExercise(migrated);
  }
  return parseExercise(value);
}
