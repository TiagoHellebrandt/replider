// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notes_panel.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(conversationNotes)
final conversationNotesProvider = ConversationNotesFamily._();

final class ConversationNotesProvider extends $FunctionalProvider<
        AsyncValue<List<NoteModel>>, List<NoteModel>, FutureOr<List<NoteModel>>>
    with $FutureModifier<List<NoteModel>>, $FutureProvider<List<NoteModel>> {
  ConversationNotesProvider._(
      {required ConversationNotesFamily super.from,
      required String super.argument})
      : super(
          retry: null,
          name: r'conversationNotesProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$conversationNotesHash();

  @override
  String toString() {
    return r'conversationNotesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<List<NoteModel>> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<List<NoteModel>> create(Ref ref) {
    final argument = this.argument as String;
    return conversationNotes(
      ref,
      argument,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ConversationNotesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$conversationNotesHash() => r'd08f1d447e9543bb277500e76e02b0e3d1a07be2';

final class ConversationNotesFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<List<NoteModel>>, String> {
  ConversationNotesFamily._()
      : super(
          retry: null,
          name: r'conversationNotesProvider',
          dependencies: null,
          $allTransitiveDependencies: null,
          isAutoDispose: true,
        );

  ConversationNotesProvider call(
    String conversationId,
  ) =>
      ConversationNotesProvider._(argument: conversationId, from: this);

  @override
  String toString() => r'conversationNotesProvider';
}
