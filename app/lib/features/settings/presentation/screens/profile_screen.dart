import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/settings/presentation/controllers/profile_controller.dart';
import 'package:app/features/settings/presentation/widgets/two_factor_settings_card.dart';
import 'package:app/core/forms/dirty_form_mixin.dart';
import 'package:app/design_system/components/settings_section_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/i18n/strings.g.dart';

class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen>
    with DirtyFormMixin<ProfileScreen> {
  final _nameController = TextEditingController();
  final _emailController = TextEditingController();
  final _signatureController = TextEditingController();
  final _currentPasswordController = TextEditingController();
  final _newPasswordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();

  bool _initialized = false;
  bool _savingProfile = false;
  bool _savingPassword = false;
  bool _showCurrentPassword = false;
  bool _showNewPassword = false;
  bool _showConfirmPassword = false;

  // Only the profile fields participate in dirty tracking. The password
  // form is its own subform with its own save button — losing the
  // typed current/new password on back-navigation is acceptable
  // (security + low cost to retype) and would create false positives.
  @override
  Map<String, Object?> snapshotForm() => {
        'name': _nameController.text.trim(),
        'email': _emailController.text.trim(),
        'signature': _signatureController.text.trim(),
      };

  @override
  void dispose() {
    _nameController.removeListener(recomputeDirty);
    _emailController.removeListener(recomputeDirty);
    _signatureController.removeListener(recomputeDirty);
    _nameController.dispose();
    _emailController.dispose();
    _signatureController.dispose();
    _currentPasswordController.dispose();
    _newPasswordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  void _initFields(String name, String email, String signature) {
    if (!_initialized) {
      _nameController.text = name;
      _emailController.text = email;
      _signatureController.text = signature;
      _nameController.addListener(recomputeDirty);
      _emailController.addListener(recomputeDirty);
      _signatureController.addListener(recomputeDirty);
      resetSnapshot();
      _initialized = true;
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final authAsync = ref.watch(authControllerProvider);

    return buildDirtyGuard(
      onDiscard: () => context.go('/settings'),
      child: Scaffold(
        backgroundColor: colors.backgroundPrimary,
        appBar: AppBar(
          backgroundColor: colors.backgroundSecondary,
          elevation: 0,
          leading: IconButton(
            tooltip: t.common.back,
            icon: Icon(Icons.arrow_back, color: colors.textSecondary),
            onPressed: () async {
              if (!isDirty) {
                context.go('/settings');
                return;
              }
              if (await confirmDiscard() && context.mounted) {
                context.go('/settings');
              }
            },
          ),
          title: Text(
            t.profile.title,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary,
            ),
          ),
        ),
        body: authAsync.when(
          data: (user) {
            if (user == null) {
              return Center(
                child: Text(t.profile.user_not_found,
                    style: TextStyle(color: colors.textMuted)),
              );
            }
            _initFields(user.name, user.email, user.signature ?? '');

            return SettingsPageBody(
              children: [
                // Avatar
                Center(
                  child: Column(
                    children: [
                      _buildAvatar(colors, user),
                      const SizedBox(height: 12),
                      Text(
                        user.name,
                        style: TextStyle(
                          color: colors.textPrimary,
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        user.email,
                        style: TextStyle(color: colors.textMuted, fontSize: 13),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 32),

                // Info section
                SettingsSectionCard(
                  title: t.profile.personal_info,
                  icon: Icons.person_outline,
                  child: Column(
                    children: [
                      _buildTextField(
                        colors,
                        controller: _nameController,
                        label: t.profile.name,
                        icon: Icons.person_outline,
                        textCapitalization: TextCapitalization.words,
                      ),
                      const SizedBox(height: 14),
                      _buildTextField(
                        colors,
                        controller: _emailController,
                        label: t.profile.email,
                        icon: Icons.email_outlined,
                        keyboardType: TextInputType.emailAddress,
                      ),
                      const SizedBox(height: 14),
                      _buildTextField(
                        colors,
                        controller: _signatureController,
                        label: t.profile.signature_label,
                        icon: Icons.history_edu_outlined,
                        maxLines: 2,
                        maxLength: 280,
                        helperText: t.profile.signature_help,
                      ),
                      const SizedBox(height: 16),
                      SizedBox(
                        width: double.infinity,
                        child: ElevatedButton(
                          onPressed: _savingProfile ? null : _saveProfile,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: colors.bluePrimary,
                            foregroundColor: Colors.white,
                            padding: const EdgeInsets.symmetric(vertical: 14),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(10),
                            ),
                          ),
                          child: _savingProfile
                              ? const SizedBox(
                                  width: 20,
                                  height: 20,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                    color: Colors.white,
                                  ),
                                )
                              : Text(
                                  t.profile.save_info,
                                  style: const TextStyle(
                                      fontWeight: FontWeight.w600),
                                ),
                        ),
                      ),
                    ],
                  ),
                ),

                const SizedBox(height: 24),

                // Password section
                SettingsSectionCard(
                  title: t.profile.change_password,
                  icon: Icons.lock_outline,
                  child: Column(
                    children: [
                      _buildPasswordField(
                        colors,
                        controller: _currentPasswordController,
                        label: t.profile.current_password,
                        visible: _showCurrentPassword,
                        onToggle: () => setState(
                          () => _showCurrentPassword = !_showCurrentPassword,
                        ),
                      ),
                      const SizedBox(height: 14),
                      _buildPasswordField(
                        colors,
                        controller: _newPasswordController,
                        label: t.profile.new_password,
                        visible: _showNewPassword,
                        onToggle: () => setState(
                          () => _showNewPassword = !_showNewPassword,
                        ),
                      ),
                      const SizedBox(height: 14),
                      _buildPasswordField(
                        colors,
                        controller: _confirmPasswordController,
                        label: t.profile.confirm_new_password,
                        visible: _showConfirmPassword,
                        onToggle: () => setState(
                          () => _showConfirmPassword = !_showConfirmPassword,
                        ),
                      ),
                      const SizedBox(height: 16),
                      SizedBox(
                        width: double.infinity,
                        child: ElevatedButton(
                          onPressed: _savingPassword ? null : _savePassword,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: colors.cyanGlow,
                            foregroundColor: Colors.black87,
                            padding: const EdgeInsets.symmetric(vertical: 14),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(10),
                            ),
                          ),
                          child: _savingPassword
                              ? const SizedBox(
                                  width: 20,
                                  height: 20,
                                  child:
                                      CircularProgressIndicator(strokeWidth: 2),
                                )
                              : Text(
                                  t.profile.change_password_button,
                                  style: const TextStyle(
                                      fontWeight: FontWeight.w600),
                                ),
                        ),
                      ),
                    ],
                  ),
                ),

                const SizedBox(height: 24),
                _buildSectionLabel(colors, t.profile.two_factor_section_label),
                const TwoFactorSettingsCard(),

                const SizedBox(height: 32),
              ],
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(
            child: Text('${t.profile.error_prefix}: $e',
                style: TextStyle(color: colors.redAlert)),
          ),
        ),
      ),
    );
  }

  Future<void> _saveProfile() async {
    final name = _nameController.text.trim();
    final email = _emailController.text.trim();
    if (name.isEmpty || email.isEmpty) {
      _showSnack(t.profile.fill_all_fields, isError: true);
      return;
    }
    setState(() => _savingProfile = true);
    final error =
        await ref.read(profileControllerProvider.notifier).updateProfile(
              name: name,
              email: email,
              // Always sent: '' clears the signature server-side.
              signature: _signatureController.text.trim(),
              // Same tri-state contract: '' clears the number.
            );
    if (mounted) {
      setState(() => _savingProfile = false);
      if (error == null) resetSnapshot();
      _showSnack(error ?? t.profile.profile_updated, isError: error != null);
    }
  }

  Future<void> _savePassword() async {
    final current = _currentPasswordController.text;
    final newPass = _newPasswordController.text;
    final confirm = _confirmPasswordController.text;

    if (current.isEmpty || newPass.isEmpty || confirm.isEmpty) {
      _showSnack(t.profile.fill_all_password_fields, isError: true);
      return;
    }
    if (newPass != confirm) {
      _showSnack(t.profile.passwords_dont_match, isError: true);
      return;
    }
    if (newPass.length < 6) {
      _showSnack(t.profile.min_password_length, isError: true);
      return;
    }

    setState(() => _savingPassword = true);
    final error =
        await ref.read(profileControllerProvider.notifier).changePassword(
              currentPassword: current,
              newPassword: newPass,
            );
    if (mounted) {
      setState(() => _savingPassword = false);
      if (error == null) {
        _currentPasswordController.clear();
        _newPasswordController.clear();
        _confirmPasswordController.clear();
      }
      _showSnack(error ?? t.profile.password_changed, isError: error != null);
    }
  }

  void _showSnack(String msg, {bool isError = false}) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: isError ? Colors.red.shade700 : null,
      behavior: SnackBarBehavior.floating,
    ));
  }

  /// Avatar with the initial letter as fallback. When the user has a profile
  /// picture (Google sign-in) it renders through [AppNetworkImage] so the
  /// decoded bitmap is served from Flutter's ImageCache instead of being
  /// refetched/redecoded on every rebuild.
  Widget _buildAvatar(AppColorsExtension colors, UserModel user) {
    final initialAvatar = CircleAvatar(
      radius: 40,
      backgroundColor:
          user.isGoogleUser ? colors.backgroundSecondary : colors.bluePrimary,
      child: Text(
        user.name.isNotEmpty ? user.name[0].toUpperCase() : '?',
        style: const TextStyle(
          color: Colors.white,
          fontSize: 32,
          fontWeight: FontWeight.w700,
        ),
      ),
    );

    if (!user.isGoogleUser) return initialAvatar;

    return AppNetworkImage(
      url: user.picture,
      width: 80,
      height: 80,
      renderWidth: 80,
      borderRadius: BorderRadius.circular(40),
      fallback: initialAvatar,
    );
  }

  Widget _buildSectionLabel(AppColorsExtension colors, String label) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w700,
          color: colors.textMuted,
          letterSpacing: 0.8,
        ),
      ),
    );
  }

  Widget _buildTextField(
    AppColorsExtension colors, {
    required TextEditingController controller,
    required String label,
    required IconData icon,
    TextInputType? keyboardType,
    TextCapitalization textCapitalization = TextCapitalization.none,
    int maxLines = 1,
    int? maxLength,
    String? helperText,
  }) {
    return TextField(
      controller: controller,
      keyboardType: keyboardType,
      textCapitalization: textCapitalization,
      maxLines: maxLines,
      maxLength: maxLength,
      style: TextStyle(color: colors.textPrimary),
      decoration: InputDecoration(
        labelText: label,
        labelStyle: TextStyle(color: colors.textMuted),
        helperText: helperText,
        helperMaxLines: 2,
        helperStyle: TextStyle(color: colors.textMuted, fontSize: 11),
        counterStyle: TextStyle(color: colors.textMuted, fontSize: 11),
        prefixIcon: Icon(icon, size: 20, color: colors.textMuted),
      ),
    );
  }

  Widget _buildPasswordField(
    AppColorsExtension colors, {
    required TextEditingController controller,
    required String label,
    required bool visible,
    required VoidCallback onToggle,
  }) {
    return TextField(
      controller: controller,
      obscureText: !visible,
      style: TextStyle(color: colors.textPrimary),
      decoration: InputDecoration(
        labelText: label,
        labelStyle: TextStyle(color: colors.textMuted),
        prefixIcon: Icon(Icons.lock_outline, size: 20, color: colors.textMuted),
        suffixIcon: IconButton(
          icon: Icon(
            visible ? Icons.visibility_off : Icons.visibility,
            size: 20,
            color: colors.textMuted,
          ),
          onPressed: onToggle,
        ),
      ),
    );
  }
}
