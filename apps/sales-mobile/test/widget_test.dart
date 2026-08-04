import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sales_mobile/app.dart';

void main() {
  testWidgets('starts in English and opens Cash + General Customer sale', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(430, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(const ItembaSalesApp());
    await tester.pumpAndSettle();

    expect(find.text('ITEMBA-Z Sales'), findsOneWidget);
    await tester.tap(find.byKey(const Key('home-new-sale')));
    await tester.pumpAndSettle();

    expect(find.text('Cash'), findsWidgets);
    expect(find.text('General Customer'), findsOneWidget);
    expect(find.byKey(const Key('customer-selector')), findsOneWidget);
  });

  testWidgets('credit selector excludes General Customer', (tester) async {
    tester.view.physicalSize = const Size(430, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(const ItembaSalesApp());
    await tester.tap(find.byKey(const Key('home-new-sale')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('sale-type-credit')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('customer-customer-general')), findsNothing);
    expect(find.byKey(const Key('customer-customer-kijiji')), findsOneWidget);
  });

  testWidgets('language control switches navigation to Swahili', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(430, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(const ItembaSalesApp());
    await tester.pumpAndSettle();
    await tester.tap(find.text('SW'));
    await tester.pumpAndSettle();

    expect(find.text('Mwanzo wa Mauzo'), findsOneWidget);
    expect(find.text('Mauzo Yangu'), findsWidgets);
  });
}
