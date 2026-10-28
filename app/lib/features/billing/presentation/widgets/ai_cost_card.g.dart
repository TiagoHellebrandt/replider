// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'ai_cost_card.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(aiCostReport)
final aiCostReportProvider = AiCostReportProvider._();

final class AiCostReportProvider extends $FunctionalProvider<
        AsyncValue<AICostReport>, AICostReport, FutureOr<AICostReport>>
    with $FutureModifier<AICostReport>, $FutureProvider<AICostReport> {
  AiCostReportProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'aiCostReportProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$aiCostReportHash();

  @$internal
  @override
  $FutureProviderElement<AICostReport> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<AICostReport> create(Ref ref) {
    return aiCostReport(ref);
  }
}

String _$aiCostReportHash() => r'fc09f3cdfd82fdf7972c41a4d3671b9852e1e983';
