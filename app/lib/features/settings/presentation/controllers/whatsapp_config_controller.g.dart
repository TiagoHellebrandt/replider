// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'whatsapp_config_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(whatsAppConfigRemoteDataSource)
final whatsAppConfigRemoteDataSourceProvider =
    WhatsAppConfigRemoteDataSourceProvider._();

final class WhatsAppConfigRemoteDataSourceProvider extends $FunctionalProvider<
        WhatsAppConfigRemoteDataSource,
        WhatsAppConfigRemoteDataSource,
        WhatsAppConfigRemoteDataSource>
    with $Provider<WhatsAppConfigRemoteDataSource> {
  WhatsAppConfigRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'whatsAppConfigRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$whatsAppConfigRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<WhatsAppConfigRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  WhatsAppConfigRemoteDataSource create(Ref ref) {
    return whatsAppConfigRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(WhatsAppConfigRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride:
          $SyncValueProvider<WhatsAppConfigRemoteDataSource>(value),
    );
  }
}

String _$whatsAppConfigRemoteDataSourceHash() =>
    r'3d5ed3b31459950cf94fac7c0862e6887df0384c';

@ProviderFor(WhatsAppTemplates)
final whatsAppTemplatesProvider = WhatsAppTemplatesFamily._();

final class WhatsAppTemplatesProvider extends $AsyncNotifierProvider<
    WhatsAppTemplates, List<WhatsAppTemplateModel>> {
  WhatsAppTemplatesProvider._(
      {required WhatsAppTemplatesFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'whatsAppTemplatesProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$whatsAppTemplatesHash();

  @override
  String toString() {
    return r'whatsAppTemplatesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  WhatsAppTemplates create() => WhatsAppTemplates();

  @override
  bool operator ==(Object other) {
    return other is WhatsAppTemplatesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$whatsAppTemplatesHash() => r'1a918d2f53e6f4223c881e1b214b6b18893cb24a';

final class WhatsAppTemplatesFamily extends $Family
    with
        $ClassFamilyOverride<
            WhatsAppTemplates,
            AsyncValue<List<WhatsAppTemplateModel>>,
            List<WhatsAppTemplateModel>,
            FutureOr<List<WhatsAppTemplateModel>>,
            String> {
  WhatsAppTemplatesFamily._()
      : super(
          retry: null,
          name: r'whatsAppTemplatesProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  WhatsAppTemplatesProvider call(
    String channelId,
  ) =>
      WhatsAppTemplatesProvider._(argument: channelId, from: this);

  @override
  String toString() => r'whatsAppTemplatesProvider';
}

abstract class _$WhatsAppTemplates
    extends $AsyncNotifier<List<WhatsAppTemplateModel>> {
  late final _$args = ref.$arg as String;
  String get channelId => _$args;

  FutureOr<List<WhatsAppTemplateModel>> build(
    String channelId,
  );
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<WhatsAppTemplateModel>>,
        List<WhatsAppTemplateModel>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<WhatsAppTemplateModel>>,
            List<WhatsAppTemplateModel>>,
        AsyncValue<List<WhatsAppTemplateModel>>,
        Object?,
        Object?>;
    element.handleCreate(
        ref,
        () => build(
              _$args,
            ));
  }
}
