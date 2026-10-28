import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/onboarding/domain/onboarding_variant.dart';

// The wizard enumerates its steps in THREE places: the flow itself
// (`stepIndices`), the rule that wakes the primary button (`_isNextEnabled`),
// and the action behind it (`_handleNext`). Nothing forces a new step to visit
// all three, and a step missing from either switch falls into `default` — Next
// is dead, or it accepts the click and goes nowhere. Both happened on the same
// feature, a deploy apart, and neither is visible in a diff.
//
// A source scan rather than a widget pump on purpose: a step legitimately
// disabled (the company step, until its form is complete) is indistinguishable
// at runtime from a step nobody taught the switch about. What we can state
// exactly is that every step in the flow is NAMED in both switches.

const _namedIndices = <String, int>{'aiKeyStepIndex': aiKeyStepIndex};

/// The body of [method], matched by braces from its signature.
String _methodBody(String source, String signature) {
  final start = source.indexOf(signature);
  expect(start, isNot(-1), reason: 'method $signature not found — renamed?');
  final open = source.indexOf('{', start);
  var depth = 0;
  for (var i = open; i < source.length; i++) {
    if (source[i] == '{') depth++;
    if (source[i] == '}') {
      depth--;
      if (depth == 0) return source.substring(open, i + 1);
    }
  }
  fail('unbalanced braces after $signature');
}

/// Step indices named by `case N:` / `case someConst:` inside [body].
Set<int> _casesIn(String body) {
  final out = <int>{};
  for (final m
      in RegExp(r'case\s+([A-Za-z_][A-Za-z0-9_]*|\d+)\s*:').allMatches(body)) {
    final label = m.group(1)!;
    final n = int.tryParse(label) ?? _namedIndices[label];
    // A label this test cannot resolve is a hole in the guard, not a pass.
    expect(n, isNotNull,
        reason: 'unknown case label "$label" — add it to _namedIndices');
    out.add(n!);
  }
  return out;
}

void main() {
  final source = File(
    'lib/features/onboarding/presentation/screens/onboarding_screen.dart',
  ).readAsStringSync();

  final flow = OnboardingVariant.reduced.stepIndices.toSet();

  test('every step in the flow is named in _isNextEnabled', () {
    final covered = _casesIn(_methodBody(source, 'bool _isNextEnabled('));
    final missing = flow.difference(covered);
    expect(missing, isEmpty,
        reason: 'these steps fall into `default: return false`, so their Next '
            'button is dead on arrival: $missing');
  });

  test('every step in the flow is named in _handleNext', () {
    final covered = _casesIn(_methodBody(source, 'Future<void> _handleNext('));
    final missing = flow.difference(covered);
    expect(missing, isEmpty,
        reason: 'these steps accept the click and do nothing — no submit, no '
            'advance: $missing');
  });

  test('the switches do not answer for steps outside the flow', () {
    // The other direction: a case left behind by a deleted step is a rule
    // nobody reaches, and it reads as if that step were still shipped.
    final next = _casesIn(_methodBody(source, 'bool _isNextEnabled('));
    final handle = _casesIn(_methodBody(source, 'Future<void> _handleNext('));
    expect(next.difference(flow), isEmpty,
        reason: 'dead cases in _isNextEnabled');
    expect(handle.difference(flow), isEmpty,
        reason: 'dead cases in _handleNext');
  });
}
