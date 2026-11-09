import 'dart:convert';
import 'dart:io';
import 'package:luth_schema/luth_schema.dart';

Future<void> main() async {
  final input =
      jsonDecode(await stdin.transform(utf8.decoder).join()) as List<dynamic>;
  final results = input.map((item) {
    final request = object(item);
    try {
      switch (request['action']) {
        case 'exercise':
          final source = encodeExercise(request['value']);
          return {'ok': true, 'source': source};
        case 'decode':
          final exercise = decodeExercise(request['value'] as String);
          return {'ok': true, 'source': encodeExercise(exercise)};
        case 'migrate':
          final exercise = migrateExercise(request['value']);
          return {'ok': true, 'source': encodeExercise(exercise)};
        case 'validate':
          parseExercise(request['value']);
          return {'ok': true};
        default:
          throw const FormatException('Unknown parity action');
      }
    } on Exception {
      return {'ok': false};
    }
  }).toList();
  stdout.write(jsonEncode(results));
}
