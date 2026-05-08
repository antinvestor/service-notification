import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'test_harness.dart';

void main() {
  testWidgets('TestHarness exposes overridden tenancyContextProvider',
      (tester) async {
    String? observedPartition;
    await tester.pumpWidget(
      TestHarness(
        child: Consumer(
          builder: (context, ref, _) {
            observedPartition = ref.watch(tenancyContextProvider).partitionId;
            return const SizedBox();
          },
        ),
      ),
    );
    expect(observedPartition, 'part-test');
  });
}
