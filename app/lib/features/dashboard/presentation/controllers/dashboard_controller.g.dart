// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dashboard_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(dashboardRemoteDataSource)
final dashboardRemoteDataSourceProvider = DashboardRemoteDataSourceProvider._();

final class DashboardRemoteDataSourceProvider extends $FunctionalProvider<
    DashboardRemoteDataSource,
    DashboardRemoteDataSource,
    DashboardRemoteDataSource> with $Provider<DashboardRemoteDataSource> {
  DashboardRemoteDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'dashboardRemoteDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$dashboardRemoteDataSourceHash();

  @$internal
  @override
  $ProviderElement<DashboardRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DashboardRemoteDataSource create(Ref ref) {
    return dashboardRemoteDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DashboardRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DashboardRemoteDataSource>(value),
    );
  }
}

String _$dashboardRemoteDataSourceHash() =>
    r'fc1b4e57a67dd42c32123ef3da6fac44f19f5208';

/// Active ORDER BY for the conversation list. Values mirror the API:
///   "" / "last_message" — most recent customer activity first (default).
///   "first_in_conv"     — oldest first-inbound-at first (prevents
///                         forgotten customers bubbling to the bottom
///                         when new traffic arrives).
///   "oldest"            — conversation created_at ASC.

@ProviderFor(ConversationSort)
final conversationSortProvider = ConversationSortProvider._();

/// Active ORDER BY for the conversation list. Values mirror the API:
///   "" / "last_message" — most recent customer activity first (default).
///   "first_in_conv"     — oldest first-inbound-at first (prevents
///                         forgotten customers bubbling to the bottom
///                         when new traffic arrives).
///   "oldest"            — conversation created_at ASC.
final class ConversationSortProvider
    extends $NotifierProvider<ConversationSort, String> {
  /// Active ORDER BY for the conversation list. Values mirror the API:
  ///   "" / "last_message" — most recent customer activity first (default).
  ///   "first_in_conv"     — oldest first-inbound-at first (prevents
  ///                         forgotten customers bubbling to the bottom
  ///                         when new traffic arrives).
  ///   "oldest"            — conversation created_at ASC.
  ConversationSortProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationSortProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationSortHash();

  @$internal
  @override
  ConversationSort create() => ConversationSort();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String>(value),
    );
  }
}

String _$conversationSortHash() => r'67ca239c23064d71c6d8c2db744994090add7e31';

/// Active ORDER BY for the conversation list. Values mirror the API:
///   "" / "last_message" — most recent customer activity first (default).
///   "first_in_conv"     — oldest first-inbound-at first (prevents
///                         forgotten customers bubbling to the bottom
///                         when new traffic arrives).
///   "oldest"            — conversation created_at ASC.

abstract class _$ConversationSort extends $Notifier<String> {
  String build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String, String>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String, String>, String, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// Which channel the inbox is showing. Null means all of them.
///
/// One inbox with a filter, not two tabs: the salesperson thinks about the
/// clinic, not about the pipe the last message came down. Splitting email and
/// WhatsApp into tabs would cut one customer's story in half.

@ProviderFor(ConversationChannelFilter)
final conversationChannelFilterProvider = ConversationChannelFilterProvider._();

/// Which channel the inbox is showing. Null means all of them.
///
/// One inbox with a filter, not two tabs: the salesperson thinks about the
/// clinic, not about the pipe the last message came down. Splitting email and
/// WhatsApp into tabs would cut one customer's story in half.
final class ConversationChannelFilterProvider
    extends $NotifierProvider<ConversationChannelFilter, String?> {
  /// Which channel the inbox is showing. Null means all of them.
  ///
  /// One inbox with a filter, not two tabs: the salesperson thinks about the
  /// clinic, not about the pipe the last message came down. Splitting email and
  /// WhatsApp into tabs would cut one customer's story in half.
  ConversationChannelFilterProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationChannelFilterProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationChannelFilterHash();

  @$internal
  @override
  ConversationChannelFilter create() => ConversationChannelFilter();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$conversationChannelFilterHash() =>
    r'9c6cd2cc2e5f6c1d5d172e60fdc91be95dadbb89';

/// Which channel the inbox is showing. Null means all of them.
///
/// One inbox with a filter, not two tabs: the salesperson thinks about the
/// clinic, not about the pipe the last message came down. Splitting email and
/// WhatsApp into tabs would cut one customer's story in half.

abstract class _$ConversationChannelFilter extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String?, String?>, String?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// Inbox triage bucket (§6.10 camada 3).
///
/// Null means every category. Sent to the server rather than applied here
/// for the same reason as the channel filter: a page filtered client-side
/// would report "nenhum lead quente" for an inbox that has several, just
/// past the page limit.

@ProviderFor(ConversationCategoryFilter)
final conversationCategoryFilterProvider =
    ConversationCategoryFilterProvider._();

/// Inbox triage bucket (§6.10 camada 3).
///
/// Null means every category. Sent to the server rather than applied here
/// for the same reason as the channel filter: a page filtered client-side
/// would report "nenhum lead quente" for an inbox that has several, just
/// past the page limit.
final class ConversationCategoryFilterProvider
    extends $NotifierProvider<ConversationCategoryFilter, String?> {
  /// Inbox triage bucket (§6.10 camada 3).
  ///
  /// Null means every category. Sent to the server rather than applied here
  /// for the same reason as the channel filter: a page filtered client-side
  /// would report "nenhum lead quente" for an inbox that has several, just
  /// past the page limit.
  ConversationCategoryFilterProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationCategoryFilterProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationCategoryFilterHash();

  @$internal
  @override
  ConversationCategoryFilter create() => ConversationCategoryFilter();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$conversationCategoryFilterHash() =>
    r'acd7b36fac6548823e574a7eff3067bc39ef6a9b';

/// Inbox triage bucket (§6.10 camada 3).
///
/// Null means every category. Sent to the server rather than applied here
/// for the same reason as the channel filter: a page filtered client-side
/// would report "nenhum lead quente" for an inbox that has several, just
/// past the page limit.

abstract class _$ConversationCategoryFilter extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String?, String?>, String?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(ConversationStatusFilter)
final conversationStatusFilterProvider = ConversationStatusFilterProvider._();

final class ConversationStatusFilterProvider
    extends $NotifierProvider<ConversationStatusFilter, String?> {
  ConversationStatusFilterProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationStatusFilterProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationStatusFilterHash();

  @$internal
  @override
  ConversationStatusFilter create() => ConversationStatusFilter();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$conversationStatusFilterHash() =>
    r'f0aa6302655e16892d079fa85fe7419368ca58a3';

abstract class _$ConversationStatusFilter extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String?, String?>, String?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// Tracks which conversations currently have the AI bot "typing" (processing).

@ProviderFor(TypingState)
final typingStateProvider = TypingStateProvider._();

/// Tracks which conversations currently have the AI bot "typing" (processing).
final class TypingStateProvider
    extends $NotifierProvider<TypingState, Map<String, bool>> {
  /// Tracks which conversations currently have the AI bot "typing" (processing).
  TypingStateProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'typingStateProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$typingStateHash();

  @$internal
  @override
  TypingState create() => TypingState();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Map<String, bool> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Map<String, bool>>(value),
    );
  }
}

String _$typingStateHash() => r'2b008a648a4c5335ecec2274e30e8de01280b81e';

/// Tracks which conversations currently have the AI bot "typing" (processing).

abstract class _$TypingState extends $Notifier<Map<String, bool>> {
  Map<String, bool> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<Map<String, bool>, Map<String, bool>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<Map<String, bool>, Map<String, bool>>,
        Map<String, bool>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

/// Holds the latest usage alert percentage received via WebSocket.
/// Null means no alert. Consumed by the UI to show a snackbar or banner.

@ProviderFor(UsageAlert)
final usageAlertProvider = UsageAlertProvider._();

/// Holds the latest usage alert percentage received via WebSocket.
/// Null means no alert. Consumed by the UI to show a snackbar or banner.
final class UsageAlertProvider extends $NotifierProvider<UsageAlert, double?> {
  /// Holds the latest usage alert percentage received via WebSocket.
  /// Null means no alert. Consumed by the UI to show a snackbar or banner.
  UsageAlertProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'usageAlertProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$usageAlertHash();

  @$internal
  @override
  UsageAlert create() => UsageAlert();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(double? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<double?>(value),
    );
  }
}

String _$usageAlertHash() => r'433d31970d4e6f68ab582200bb1a550c686ac5a7';

/// Holds the latest usage alert percentage received via WebSocket.
/// Null means no alert. Consumed by the UI to show a snackbar or banner.

abstract class _$UsageAlert extends $Notifier<double?> {
  double? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<double?, double?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<double?, double?>, double?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// Watches WebSocket events and triggers conversation/message refreshes in real-time.

@ProviderFor(RealtimeListener)
final realtimeListenerProvider = RealtimeListenerProvider._();

/// Watches WebSocket events and triggers conversation/message refreshes in real-time.
final class RealtimeListenerProvider
    extends $NotifierProvider<RealtimeListener, void> {
  /// Watches WebSocket events and triggers conversation/message refreshes in real-time.
  RealtimeListenerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'realtimeListenerProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$realtimeListenerHash();

  @$internal
  @override
  RealtimeListener create() => RealtimeListener();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$realtimeListenerHash() => r'85c5e0ca149738469d302f837a15d7a96dfe1b40';

/// Watches WebSocket events and triggers conversation/message refreshes in real-time.

abstract class _$RealtimeListener extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<void, void>, void, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// The navigation badge: open threads holding a customer message nobody
/// opened (`GET /conversations/unread-count`). keepAlive because the SHELL
/// wears it; refreshed by the same WebSocket events that refresh the inbox,
/// debounced so a burst costs one request. Also bumped after POST /read so
/// the badge drops the moment the thread is opened.

@ProviderFor(UnreadCount)
final unreadCountProvider = UnreadCountProvider._();

/// The navigation badge: open threads holding a customer message nobody
/// opened (`GET /conversations/unread-count`). keepAlive because the SHELL
/// wears it; refreshed by the same WebSocket events that refresh the inbox,
/// debounced so a burst costs one request. Also bumped after POST /read so
/// the badge drops the moment the thread is opened.
final class UnreadCountProvider
    extends $AsyncNotifierProvider<UnreadCount, int> {
  /// The navigation badge: open threads holding a customer message nobody
  /// opened (`GET /conversations/unread-count`). keepAlive because the SHELL
  /// wears it; refreshed by the same WebSocket events that refresh the inbox,
  /// debounced so a burst costs one request. Also bumped after POST /read so
  /// the badge drops the moment the thread is opened.
  UnreadCountProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'unreadCountProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$unreadCountHash();

  @$internal
  @override
  UnreadCount create() => UnreadCount();
}

String _$unreadCountHash() => r'6442809de2b0cf2b2604466772f77e798f546869';

/// The navigation badge: open threads holding a customer message nobody
/// opened (`GET /conversations/unread-count`). keepAlive because the SHELL
/// wears it; refreshed by the same WebSocket events that refresh the inbox,
/// debounced so a burst costs one request. Also bumped after POST /read so
/// the badge drops the moment the thread is opened.

abstract class _$UnreadCount extends $AsyncNotifier<int> {
  FutureOr<int> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<int>, int>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<int>, int>, AsyncValue<int>, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// How many rows the inbox asks for. "Carregar mais" grows it by a page;
/// the list is re-read at the larger size so it stays ONE server-sorted
/// list rather than pages stitched client-side (a new message would
/// otherwise repeat or skip a row across the seam).

@ProviderFor(ConversationListLimit)
final conversationListLimitProvider = ConversationListLimitProvider._();

/// How many rows the inbox asks for. "Carregar mais" grows it by a page;
/// the list is re-read at the larger size so it stays ONE server-sorted
/// list rather than pages stitched client-side (a new message would
/// otherwise repeat or skip a row across the seam).
final class ConversationListLimitProvider
    extends $NotifierProvider<ConversationListLimit, int> {
  /// How many rows the inbox asks for. "Carregar mais" grows it by a page;
  /// the list is re-read at the larger size so it stays ONE server-sorted
  /// list rather than pages stitched client-side (a new message would
  /// otherwise repeat or skip a row across the seam).
  ConversationListLimitProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationListLimitProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationListLimitHash();

  @$internal
  @override
  ConversationListLimit create() => ConversationListLimit();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$conversationListLimitHash() =>
    r'481b78ac84cb64064a43b4864160f916aa3b26b8';

/// How many rows the inbox asks for. "Carregar mais" grows it by a page;
/// the list is re-read at the larger size so it stays ONE server-sorted
/// list rather than pages stitched client-side (a new message would
/// otherwise repeat or skip a row across the seam).

abstract class _$ConversationListLimit extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element = ref.element
        as $ClassProviderElement<AnyNotifier<int, int>, int, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(conversations)
final conversationsProvider = ConversationsProvider._();

final class ConversationsProvider extends $FunctionalProvider<
        AsyncValue<List<ConversationModel>>,
        List<ConversationModel>,
        FutureOr<List<ConversationModel>>>
    with
        $FutureModifier<List<ConversationModel>>,
        $FutureProvider<List<ConversationModel>> {
  ConversationsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationsHash();

  @$internal
  @override
  $FutureProviderElement<List<ConversationModel>> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<List<ConversationModel>> create(Ref ref) {
    return conversations(ref);
  }
}

String _$conversationsHash() => r'2962f84f24184ba43deb6802ca81a6c74d9a676b';

@ProviderFor(SelectedConversation)
final selectedConversationProvider = SelectedConversationProvider._();

final class SelectedConversationProvider
    extends $NotifierProvider<SelectedConversation, String?> {
  SelectedConversationProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'selectedConversationProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$selectedConversationHash();

  @$internal
  @override
  SelectedConversation create() => SelectedConversation();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$selectedConversationHash() =>
    r'5d80e77628a85e8613fda48c8564e4dd216c65dd';

abstract class _$SelectedConversation extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String?, String?>, String?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// Which email the reader has open (message id), or null for the inbox
/// list. Lifted out of EmailInboxView so the detail panel can swap its
/// chrome while a mail is being read — the contact header and notes bar
/// hide, since the reader's own header already names the sender.

@ProviderFor(EmailReaderOpen)
final emailReaderOpenProvider = EmailReaderOpenProvider._();

/// Which email the reader has open (message id), or null for the inbox
/// list. Lifted out of EmailInboxView so the detail panel can swap its
/// chrome while a mail is being read — the contact header and notes bar
/// hide, since the reader's own header already names the sender.
final class EmailReaderOpenProvider
    extends $NotifierProvider<EmailReaderOpen, String?> {
  /// Which email the reader has open (message id), or null for the inbox
  /// list. Lifted out of EmailInboxView so the detail panel can swap its
  /// chrome while a mail is being read — the contact header and notes bar
  /// hide, since the reader's own header already names the sender.
  EmailReaderOpenProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'emailReaderOpenProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$emailReaderOpenHash();

  @$internal
  @override
  EmailReaderOpen create() => EmailReaderOpen();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$emailReaderOpenHash() => r'3f60adb7e94cc0de39d48632663015300f7997af';

/// Which email the reader has open (message id), or null for the inbox
/// list. Lifted out of EmailInboxView so the detail panel can swap its
/// chrome while a mail is being read — the contact header and notes bar
/// hide, since the reader's own header already names the sender.

abstract class _$EmailReaderOpen extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String?, String?>, String?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// One-shot composer prefill (US-015 co-pilot templates): a caller sets
/// the text BEFORE selecting a conversation + navigating to /home; the
/// conversation detail panel consumes it into the message input and
/// clears it. Never auto-sends — the seller always reviews first.

@ProviderFor(ComposerPrefill)
final composerPrefillProvider = ComposerPrefillProvider._();

/// One-shot composer prefill (US-015 co-pilot templates): a caller sets
/// the text BEFORE selecting a conversation + navigating to /home; the
/// conversation detail panel consumes it into the message input and
/// clears it. Never auto-sends — the seller always reviews first.
final class ComposerPrefillProvider
    extends $NotifierProvider<ComposerPrefill, String?> {
  /// One-shot composer prefill (US-015 co-pilot templates): a caller sets
  /// the text BEFORE selecting a conversation + navigating to /home; the
  /// conversation detail panel consumes it into the message input and
  /// clears it. Never auto-sends — the seller always reviews first.
  ComposerPrefillProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'composerPrefillProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$composerPrefillHash();

  @$internal
  @override
  ComposerPrefill create() => ComposerPrefill();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$composerPrefillHash() => r'55ccce1ccbeebc854ab2fa3e9a6ce33b77e72576';

/// One-shot composer prefill (US-015 co-pilot templates): a caller sets
/// the text BEFORE selecting a conversation + navigating to /home; the
/// conversation detail panel consumes it into the message input and
/// clears it. Never auto-sends — the seller always reviews first.

abstract class _$ComposerPrefill extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String?, String?>, String?, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

/// Real messages from the API. Invalidated by RealtimeListener on new_message events.

@ProviderFor(messages)
final messagesProvider = MessagesFamily._();

/// Real messages from the API. Invalidated by RealtimeListener on new_message events.

final class MessagesProvider extends $FunctionalProvider<
        AsyncValue<List<MessageModel>>,
        List<MessageModel>,
        FutureOr<List<MessageModel>>>
    with
        $FutureModifier<List<MessageModel>>,
        $FutureProvider<List<MessageModel>> {
  /// Real messages from the API. Invalidated by RealtimeListener on new_message events.
  MessagesProvider._(
      {required MessagesFamily super.from, required String super.argument})
      : super(
          retry: null,
          name: r'messagesProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$messagesHash();

  @override
  String toString() {
    return r'messagesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<MessageModel>> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<List<MessageModel>> create(Ref ref) {
    final argument = this.argument as String;
    return messages(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is MessagesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$messagesHash() => r'5df05f499cb9198594a43bc9b4a40ac56d015c08';

/// Real messages from the API. Invalidated by RealtimeListener on new_message events.

final class MessagesFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<MessageModel>>, String> {
  MessagesFamily._()
      : super(
          retry: null,
          name: r'messagesProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Real messages from the API. Invalidated by RealtimeListener on new_message events.

  MessagesProvider call(
    String conversationId,
  ) =>
      MessagesProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'messagesProvider';
}

@ProviderFor(MessageHistory)
final messageHistoryProvider = MessageHistoryFamily._();

final class MessageHistoryProvider
    extends $NotifierProvider<MessageHistory, MessageHistoryState> {
  MessageHistoryProvider._(
      {required MessageHistoryFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'messageHistoryProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$messageHistoryHash();

  @override
  String toString() {
    return r'messageHistoryProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  MessageHistory create() => MessageHistory();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MessageHistoryState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MessageHistoryState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is MessageHistoryProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$messageHistoryHash() => r'333ad8c663e582f7378ffd0dc7d41c7f86761e17';

final class MessageHistoryFamily extends $Family
    with
        $ClassFamilyOverride<MessageHistory, MessageHistoryState,
            MessageHistoryState, MessageHistoryState, String> {
  MessageHistoryFamily._()
      : super(
          retry: null,
          name: r'messageHistoryProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  MessageHistoryProvider call(
    String conversationId,
  ) =>
      MessageHistoryProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'messageHistoryProvider';
}

abstract class _$MessageHistory extends $Notifier<MessageHistoryState> {
  late final _$args = ref.$arg as String;
  String get conversationId => _$args;

  MessageHistoryState build(
    String conversationId,
  );
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<MessageHistoryState, MessageHistoryState>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<MessageHistoryState, MessageHistoryState>,
        MessageHistoryState,
        Object?,
        Object?>;
    element.handleCreate(
        ref,
        () => build(
              _$args,
            ));
  }
}

/// Media-only message list for the "Arquivos" tab on the contact
/// detail sheet. Backed by GET /conversations/:id/media which already
/// filters out text messages and resolves GCS object paths to signed
/// URLs server-side, so the UI can render the URLs directly.

@ProviderFor(conversationMedia)
final conversationMediaProvider = ConversationMediaFamily._();

/// Media-only message list for the "Arquivos" tab on the contact
/// detail sheet. Backed by GET /conversations/:id/media which already
/// filters out text messages and resolves GCS object paths to signed
/// URLs server-side, so the UI can render the URLs directly.

final class ConversationMediaProvider extends $FunctionalProvider<
        AsyncValue<List<MessageModel>>,
        List<MessageModel>,
        FutureOr<List<MessageModel>>>
    with
        $FutureModifier<List<MessageModel>>,
        $FutureProvider<List<MessageModel>> {
  /// Media-only message list for the "Arquivos" tab on the contact
  /// detail sheet. Backed by GET /conversations/:id/media which already
  /// filters out text messages and resolves GCS object paths to signed
  /// URLs server-side, so the UI can render the URLs directly.
  ConversationMediaProvider._(
      {required ConversationMediaFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'conversationMediaProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationMediaHash();

  @override
  String toString() {
    return r'conversationMediaProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<MessageModel>> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<List<MessageModel>> create(Ref ref) {
    final argument = this.argument as String;
    return conversationMedia(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ConversationMediaProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$conversationMediaHash() => r'7b505b62269ffa07463cab0992eb1d3c6a2e6db3';

/// Media-only message list for the "Arquivos" tab on the contact
/// detail sheet. Backed by GET /conversations/:id/media which already
/// filters out text messages and resolves GCS object paths to signed
/// URLs server-side, so the UI can render the URLs directly.

final class ConversationMediaFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<MessageModel>>, String> {
  ConversationMediaFamily._()
      : super(
          retry: null,
          name: r'conversationMediaProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Media-only message list for the "Arquivos" tab on the contact
  /// detail sheet. Backed by GET /conversations/:id/media which already
  /// filters out text messages and resolves GCS object paths to signed
  /// URLs server-side, so the UI can render the URLs directly.

  ConversationMediaProvider call(
    String conversationId,
  ) =>
      ConversationMediaProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'conversationMediaProvider';
}

@ProviderFor(contactStats)
final contactStatsProvider = ContactStatsFamily._();

final class ContactStatsProvider extends $FunctionalProvider<
        AsyncValue<ContactStatsModel>,
        ContactStatsModel,
        FutureOr<ContactStatsModel>>
    with
        $FutureModifier<ContactStatsModel>,
        $FutureProvider<ContactStatsModel> {
  ContactStatsProvider._(
      {required ContactStatsFamily super.from, required String super.argument})
      : super(
          retry: null,
          name: r'contactStatsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$contactStatsHash();

  @override
  String toString() {
    return r'contactStatsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<ContactStatsModel> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<ContactStatsModel> create(Ref ref) {
    final argument = this.argument as String;
    return contactStats(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ContactStatsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$contactStatsHash() => r'fb6e44141a5efc1b7c213598e9625642d9101274';

final class ContactStatsFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<ContactStatsModel>, String> {
  ContactStatsFamily._()
      : super(
          retry: null,
          name: r'contactStatsProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  ContactStatsProvider call(
    String conversationId,
  ) =>
      ContactStatsProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'contactStatsProvider';
}

/// Sales-funnel memory for the contact behind a conversation. A 404 means
/// the conversation is not bound to any funnel (or no lead yet) — surfaced
/// as the empty state, not an error.

@ProviderFor(salesProfile)
final salesProfileProvider = SalesProfileFamily._();

/// Sales-funnel memory for the contact behind a conversation. A 404 means
/// the conversation is not bound to any funnel (or no lead yet) — surfaced
/// as the empty state, not an error.

final class SalesProfileProvider extends $FunctionalProvider<
        AsyncValue<LeadProfileModel>,
        LeadProfileModel,
        FutureOr<LeadProfileModel>>
    with $FutureModifier<LeadProfileModel>, $FutureProvider<LeadProfileModel> {
  /// Sales-funnel memory for the contact behind a conversation. A 404 means
  /// the conversation is not bound to any funnel (or no lead yet) — surfaced
  /// as the empty state, not an error.
  SalesProfileProvider._(
      {required SalesProfileFamily super.from, required String super.argument})
      : super(
          retry: null,
          name: r'salesProfileProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$salesProfileHash();

  @override
  String toString() {
    return r'salesProfileProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<LeadProfileModel> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<LeadProfileModel> create(Ref ref) {
    final argument = this.argument as String;
    return salesProfile(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is SalesProfileProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$salesProfileHash() => r'f2e77225383ece0f8a405c49fe8ed7ea5398a6b2';

/// Sales-funnel memory for the contact behind a conversation. A 404 means
/// the conversation is not bound to any funnel (or no lead yet) — surfaced
/// as the empty state, not an error.

final class SalesProfileFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<LeadProfileModel>, String> {
  SalesProfileFamily._()
      : super(
          retry: null,
          name: r'salesProfileProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// Sales-funnel memory for the contact behind a conversation. A 404 means
  /// the conversation is not bound to any funnel (or no lead yet) — surfaced
  /// as the empty state, not an error.

  SalesProfileProvider call(
    String conversationId,
  ) =>
      SalesProfileProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'salesProfileProvider';
}

@ProviderFor(ConversationSearch)
final conversationSearchProvider = ConversationSearchProvider._();

final class ConversationSearchProvider
    extends $NotifierProvider<ConversationSearch, String> {
  ConversationSearchProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationSearchProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationSearchHash();

  @$internal
  @override
  ConversationSearch create() => ConversationSearch();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String>(value),
    );
  }
}

String _$conversationSearchHash() =>
    r'14d81b31d4bb4b70b241f9fbcc80e1bdfe2c03f4';

abstract class _$ConversationSearch extends $Notifier<String> {
  String build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String, String>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String, String>, String, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(MessageSearch)
final messageSearchProvider = MessageSearchProvider._();

final class MessageSearchProvider
    extends $NotifierProvider<MessageSearch, String> {
  MessageSearchProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'messageSearchProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$messageSearchHash();

  @$internal
  @override
  MessageSearch create() => MessageSearch();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String>(value),
    );
  }
}

String _$messageSearchHash() => r'2c7f40ea22529a859011eed4c35d3656a93ad58d';

abstract class _$MessageSearch extends $Notifier<String> {
  String build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String, String>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<String, String>, String, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(ConversationActions)
final conversationActionsProvider = ConversationActionsProvider._();

final class ConversationActionsProvider
    extends $AsyncNotifierProvider<ConversationActions, void> {
  ConversationActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationActionsHash();

  @$internal
  @override
  ConversationActions create() => ConversationActions();
}

String _$conversationActionsHash() =>
    r'ea509d10b9dc526e891e14dbc283513740b1f0d6';

abstract class _$ConversationActions extends $AsyncNotifier<void> {
  FutureOr<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<void>, void>,
        AsyncValue<void>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(ConversationBrief)
final conversationBriefProvider = ConversationBriefFamily._();

final class ConversationBriefProvider
    extends $AsyncNotifierProvider<ConversationBrief, ConversationBriefData?> {
  ConversationBriefProvider._(
      {required ConversationBriefFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'conversationBriefProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationBriefHash();

  @override
  String toString() {
    return r'conversationBriefProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  ConversationBrief create() => ConversationBrief();

  @override
  bool operator ==(Object other) {
    return other is ConversationBriefProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$conversationBriefHash() => r'd42e92459f6a9804e0a9fb09a168c15459ac27ae';

final class ConversationBriefFamily extends $Family
    with
        $ClassFamilyOverride<
            ConversationBrief,
            AsyncValue<ConversationBriefData?>,
            ConversationBriefData?,
            FutureOr<ConversationBriefData?>,
            String> {
  ConversationBriefFamily._()
      : super(
          retry: null,
          name: r'conversationBriefProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  ConversationBriefProvider call(
    String conversationId,
  ) =>
      ConversationBriefProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'conversationBriefProvider';
}

abstract class _$ConversationBrief
    extends $AsyncNotifier<ConversationBriefData?> {
  late final _$args = ref.$arg as String;
  String get conversationId => _$args;

  FutureOr<ConversationBriefData?> build(
    String conversationId,
  );
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<ConversationBriefData?>, ConversationBriefData?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<ConversationBriefData?>, ConversationBriefData?>,
        AsyncValue<ConversationBriefData?>,
        Object?,
        Object?>;
    element.handleCreate(
        ref,
        () => build(
              _$args,
            ));
  }
}

/// LLM-backed reply suggestions for a single conversation. autoDispose
/// so we don't pre-fetch suggestions for conversations the user isn't
/// looking at — wasted ¢. Family keyed by conversation_id keeps the
/// cache clean across navigations.

@ProviderFor(ReplySuggestions)
final replySuggestionsProvider = ReplySuggestionsFamily._();

/// LLM-backed reply suggestions for a single conversation. autoDispose
/// so we don't pre-fetch suggestions for conversations the user isn't
/// looking at — wasted ¢. Family keyed by conversation_id keeps the
/// cache clean across navigations.
final class ReplySuggestionsProvider
    extends $NotifierProvider<ReplySuggestions, AsyncValue<List<String>>> {
  /// LLM-backed reply suggestions for a single conversation. autoDispose
  /// so we don't pre-fetch suggestions for conversations the user isn't
  /// looking at — wasted ¢. Family keyed by conversation_id keeps the
  /// cache clean across navigations.
  ReplySuggestionsProvider._(
      {required ReplySuggestionsFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'replySuggestionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$replySuggestionsHash();

  @override
  String toString() {
    return r'replySuggestionsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  ReplySuggestions create() => ReplySuggestions();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<List<String>> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<List<String>>>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ReplySuggestionsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$replySuggestionsHash() => r'a969226cf0b6c08fb8810b8a201104970dc3b2a1';

/// LLM-backed reply suggestions for a single conversation. autoDispose
/// so we don't pre-fetch suggestions for conversations the user isn't
/// looking at — wasted ¢. Family keyed by conversation_id keeps the
/// cache clean across navigations.

final class ReplySuggestionsFamily extends $Family
    with
        $ClassFamilyOverride<ReplySuggestions, AsyncValue<List<String>>,
            AsyncValue<List<String>>, AsyncValue<List<String>>, String> {
  ReplySuggestionsFamily._()
      : super(
          retry: null,
          name: r'replySuggestionsProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  /// LLM-backed reply suggestions for a single conversation. autoDispose
  /// so we don't pre-fetch suggestions for conversations the user isn't
  /// looking at — wasted ¢. Family keyed by conversation_id keeps the
  /// cache clean across navigations.

  ReplySuggestionsProvider call(
    String conversationId,
  ) =>
      ReplySuggestionsProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'replySuggestionsProvider';
}

/// LLM-backed reply suggestions for a single conversation. autoDispose
/// so we don't pre-fetch suggestions for conversations the user isn't
/// looking at — wasted ¢. Family keyed by conversation_id keeps the
/// cache clean across navigations.

abstract class _$ReplySuggestions extends $Notifier<AsyncValue<List<String>>> {
  late final _$args = ref.$arg as String;
  String get conversationId => _$args;

  AsyncValue<List<String>> build(
    String conversationId,
  );
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<List<String>>, AsyncValue<List<String>>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<List<String>>, AsyncValue<List<String>>>,
        AsyncValue<List<String>>,
        Object?,
        Object?>;
    element.handleCreate(
        ref,
        () => build(
              _$args,
            ));
  }
}

/// Multiselect state for the conversation list. Keeps the set of
/// selected ids + a flag for "selection mode" so the list switches
/// rendering (checkbox affordance, action toolbar) only when needed.

@ProviderFor(ConversationSelection)
final conversationSelectionProvider = ConversationSelectionProvider._();

/// Multiselect state for the conversation list. Keeps the set of
/// selected ids + a flag for "selection mode" so the list switches
/// rendering (checkbox affordance, action toolbar) only when needed.
final class ConversationSelectionProvider
    extends $NotifierProvider<ConversationSelection, Set<String>> {
  /// Multiselect state for the conversation list. Keeps the set of
  /// selected ids + a flag for "selection mode" so the list switches
  /// rendering (checkbox affordance, action toolbar) only when needed.
  ConversationSelectionProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'conversationSelectionProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationSelectionHash();

  @$internal
  @override
  ConversationSelection create() => ConversationSelection();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Set<String> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Set<String>>(value),
    );
  }
}

String _$conversationSelectionHash() =>
    r'e39c07fd496e5d75695a81c5592e81ce9d126709';

/// Multiselect state for the conversation list. Keeps the set of
/// selected ids + a flag for "selection mode" so the list switches
/// rendering (checkbox affordance, action toolbar) only when needed.

abstract class _$ConversationSelection extends $Notifier<Set<String>> {
  Set<String> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<Set<String>, Set<String>>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<Set<String>, Set<String>>, Set<String>, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}
