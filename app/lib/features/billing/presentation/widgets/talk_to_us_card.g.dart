// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'talk_to_us_card.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The sales team's channels: a booking page (when configured), WhatsApp
/// and e-mail. One read per session — the values change on deploy, not in
/// conversation.

@ProviderFor(contactSales)
final contactSalesProvider = ContactSalesProvider._();

/// The sales team's channels: a booking page (when configured), WhatsApp
/// and e-mail. One read per session — the values change on deploy, not in
/// conversation.

final class ContactSalesProvider extends $FunctionalProvider<
        AsyncValue<ContactSalesChannels>,
        ContactSalesChannels,
        FutureOr<ContactSalesChannels>>
    with
        $FutureModifier<ContactSalesChannels>,
        $FutureProvider<ContactSalesChannels> {
  /// The sales team's channels: a booking page (when configured), WhatsApp
  /// and e-mail. One read per session — the values change on deploy, not in
  /// conversation.
  ContactSalesProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'contactSalesProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$contactSalesHash();

  @$internal
  @override
  $FutureProviderElement<ContactSalesChannels> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<ContactSalesChannels> create(Ref ref) {
    return contactSales(ref);
  }
}

String _$contactSalesHash() => r'83bcc5c9ed4808280c02a109e240bc49f551af63';
