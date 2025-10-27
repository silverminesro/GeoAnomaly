// This is a basic Flutter widget test.
//
// To perform an interaction with a widget in your test, use the WidgetTester
// utility in the flutter_test package. For example, you can send tap and scroll
// gestures. You can also use WidgetTester to find child widgets in the widget
// tree, read text, and verify that the values of widget properties are correct.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:geoanomaly/main.dart';

void main() {
  testWidgets('Inventory screen smoke test', (WidgetTester tester) async {
    // Build our app and trigger a frame.
    await tester.pumpWidget(const GeoAnomalyApp());

    // Verify that inventory screen loads
    expect(find.text('Inventory'), findsOneWidget);
    expect(find.text('Stats'), findsOneWidget);
    expect(find.text('Artifacts'), findsOneWidget);
    expect(find.text('Gear'), findsOneWidget);
  });
}