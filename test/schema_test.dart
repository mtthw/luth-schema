import 'dart:convert';
import 'dart:io';
import 'package:luth_schema/luth_schema.dart';
import 'package:test/test.dart';

JsonObject fixture(String name) =>
    object(jsonDecode(File('fixtures/$name').readAsStringSync()));
JsonObject hold() => fixture('valid/hold.json');

void main() {
  for (final validity in ['valid', 'invalid']) {
    for (final file in Directory(
      'fixtures/$validity',
    ).listSync().whereType<File>()) {
      test(file.path, () {
        final input = jsonDecode(file.readAsStringSync());
        if (validity == 'valid') {
          expect(() => parseExercise(input), returnsNormally);
        } else {
          expect(() => parseExercise(input), throwsFormatException);
        }
      });
    }
  }
  test('normalization and explicit legacy migration', () {
    expect(encodeExercise(hold()), encodeExercise(parseExercise(hold())));
    expect(canonicalJson({'b': 1, 'a': 2}), '{"a":2,"b":1}');
    final legacy = fixture('invalid/legacy.json');
    expect(migrateExercise(legacy)['schemaVersion'], '0.0.1');
    expect(
      () => migrateExercise({...legacy, 'schemaVersion': '0.0.1'}),
      throwsFormatException,
    );
    expect(
      () => migrateExercise({...legacy, 'formatVersion': 2}),
      throwsFormatException,
    );
  });
  test('requires a supported exercise semver string', () {
    for (final version in [
      1,
      0.001,
      null,
      '',
      '0.0',
      '00.0.1',
      'v0.0.1',
      '0.0.2',
      '0.1.0',
      '1.0.0',
      '0.0.1-beta.1',
      '0.0.1+build.1',
    ]) {
      expect(
        () => parseExercise({...hold(), 'schemaVersion': version}),
        throwsFormatException,
      );
    }
    expect(parseExercise(hold())['schemaVersion'], '0.0.1');
  });
  test('execution semantics', () {
    expect(
      countSteps(
        parseExercise(fixture('valid/group.json'))['sets'] as List<dynamic>,
      ),
      5,
    );
    final manual = fixture('valid/integral-numbers.json');
    object((manual['sets'] as List<dynamic>)[0])['reps'] = 100000;
    expect(countSteps(parseExercise(manual)['sets'] as List<dynamic>), 1);
    final timed = hold();
    object((timed['sets'] as List<dynamic>)[0])['reps'] = 100000;
    expect(countSteps(parseExercise(timed)['sets'] as List<dynamic>), 100000);
    (timed['sets'] as List<dynamic>).add({
      ...object((timed['sets'] as List<dynamic>)[0]),
    });
    expect(() => parseExercise(timed), throwsFormatException);
  });
  test('bounds depth, nodes and source size', () {
    final input = hold();
    var node = object((input['sets'] as List<dynamic>)[0]);
    for (var depth = 1; depth < 32; depth++) {
      final child = <String, dynamic>{
        'title': 'Nested',
        'reps': 1,
        'subsets': <dynamic>[],
        'skipLast': false,
      };
      node['durationMicroseconds'] = null;
      node['subsets'] = [child];
      node = child;
    }
    expect(() => parseExercise(input), returnsNormally);
    node['subsets'] = [
      {
        'title': 'Too deep',
        'reps': 1,
        'subsets': <dynamic>[],
        'skipLast': false,
      },
    ];
    expect(() => parseExercise(input), throwsFormatException);
    final wide = hold();
    final leaf = object((wide['sets'] as List<dynamic>)[0]);
    wide['sets'] = List.generate(10000, (_) => {...leaf});
    expect(() => parseExercise(wide), returnsNormally);
    (wide['sets'] as List<dynamic>).add({...leaf});
    expect(() => parseExercise(wide), throwsFormatException);
    expect(
      () => decodeExercise(' ' * (maxSourceBytes + 1)),
      throwsFormatException,
    );
  });
  test(
    'serializes only validated exercise JSON and round trips valid fixtures',
    () {
      for (final file in Directory(
        'fixtures/valid',
      ).listSync().whereType<File>()) {
        final input = jsonDecode(file.readAsStringSync());
        final source = encodeExercise(input);
        expect(decodeExercise(source), parseExercise(input));
        final keys = object(jsonDecode(source)).keys.toList()..sort();
        expect(keys, ['countdownTones', 'schemaVersion', 'sets', 'title']);
      }
      expect(
        () => encodeExercise({...hold(), 'kind': 'exercise'}),
        throwsFormatException,
      );
      expect(
        () => encodeExercise(fixture('invalid/empty.json')),
        throwsFormatException,
      );
      expect(() => decodeExercise('{'), throwsFormatException);
      final huge = {...hold(), 'title': 'é' * (maxSourceBytes ~/ 2)};
      expect(() => encodeExercise(huge), throwsFormatException);
      expect(() => decodeExercise(jsonEncode(huge)), throwsFormatException);
    },
  );
}
