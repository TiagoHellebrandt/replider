import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class CustomTextField extends StatefulWidget {
  final String hint;
  final bool isPassword;
  final TextEditingController? controller;
  final String? Function(String?)? validator;
  final TextInputType keyboardType;
  final void Function(String)? onChanged;
  final IconData? prefixIcon;

  /// Shows the cursor and opens the keyboard as soon as the form is
  /// displayed. Use on the first field of a screen so the user can
  /// start typing without tapping first.
  final bool autofocus;

  /// Locks the field against edits while keeping it visibly filled — used by
  /// the invite signup, where the e-mail is bound server-side and typing a
  /// different one would only earn a refusal.
  final bool readOnly;

  /// Autofill semantics for browsers and password managers. Without them,
  /// extensions guess — and a wrong guess overlays/blocks the field (the
  /// register screen's confirm-password was unusable under one).
  final List<String>? autofillHints;

  /// Controls the return-key label + behavior. Use `next` on
  /// intermediate fields and `done` on the last one so the keyboard
  /// can drive the whole form (Tab on web, enter on mobile).
  final TextInputAction? textInputAction;

  /// Optional explicit focus node — attach one when you need to
  /// programmatically jump focus (e.g. after tapping an inline action).
  final FocusNode? focusNode;

  /// Invoked when the user presses the done/next action on the
  /// keyboard. Defaults to advancing focus when null, so there's no
  /// breakage by simply wiring this up.
  final void Function(String)? onSubmitted;

  const CustomTextField({
    super.key,
    required this.hint,
    this.isPassword = false,
    this.controller,
    this.validator,
    this.keyboardType = TextInputType.text,
    this.onChanged,
    this.prefixIcon,
    this.autofocus = false,
    this.textInputAction,
    this.focusNode,
    this.onSubmitted,
    this.readOnly = false,
    this.autofillHints,
  });

  @override
  State<CustomTextField> createState() => _CustomTextFieldState();
}

class _CustomTextFieldState extends State<CustomTextField> {
  bool _obscureText = true;

  @override
  void initState() {
    super.initState();
    _obscureText = widget.isPassword;
  }

  void _toggleVisibility() {
    setState(() {
      _obscureText = !_obscureText;
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return TextFormField(
      controller: widget.controller,
      readOnly: widget.readOnly,
      autofillHints: widget.autofillHints,
      focusNode: widget.focusNode,
      autofocus: widget.autofocus,
      obscureText: _obscureText,
      validator: widget.validator,
      keyboardType: widget.keyboardType,
      textInputAction: widget.textInputAction ??
          (widget.isPassword ? TextInputAction.done : TextInputAction.next),
      onChanged: widget.onChanged,
      onFieldSubmitted: widget.onSubmitted,
      style: typography.body.copyWith(color: colors.textPrimary, fontSize: 14),
      decoration: InputDecoration(
        hintText: widget.hint,
        hintStyle:
            typography.body.copyWith(color: colors.textSecondary, fontSize: 14),
        // The shape comes from the theme, as it does for every field in the app.
        // This component drew a pill (radius 100) just for itself, and was the
        // third field shape in circulation — the product's front door did not look
        // like the same product.
        contentPadding:
            const EdgeInsets.symmetric(horizontal: 16, vertical: 18),
        prefixIcon: widget.prefixIcon != null
            ? Padding(
                padding: const EdgeInsets.only(left: 20, right: 12),
                child: Icon(widget.prefixIcon,
                    color: colors.textSecondary, size: 20),
              )
            : null,
        suffixIcon: widget.isPassword
            ? Padding(
                padding: const EdgeInsets.only(right: 12),
                child: IconButton(
                  icon: Icon(
                    _obscureText
                        ? Icons.visibility_off_outlined
                        : Icons.visibility_outlined,
                    color: colors.textSecondary,
                    size: 20,
                  ),
                  onPressed: _toggleVisibility,
                ),
              )
            : null,
      ),
    );
  }
}
