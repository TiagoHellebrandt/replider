///
/// Generated file. Do not edit.
///
// coverage:ignore-file
// ignore_for_file: type=lint, unused_import
// dart format off

part of 'strings.g.dart';

// Path: <root>
typedef TranslationsEn = Translations; // ignore: unused_element
class Translations with BaseTranslations<AppLocale, Translations> {
	/// Returns the current translations of the given [context].
	///
	/// Usage:
	/// final t = Translations.of(context);
	static Translations of(BuildContext context) => InheritedLocaleData.of<AppLocale, Translations>(context).translations;

	/// You can call this constructor and build your own translation instance of this locale.
	/// Constructing via the enum [AppLocale.build] is preferred.
	Translations({Map<String, Node>? overrides, PluralResolver? cardinalResolver, PluralResolver? ordinalResolver, TranslationMetadata<AppLocale, Translations>? meta})
		: assert(overrides == null, 'Set "translation_overrides: true" in order to enable this feature.'),
		  $meta = meta ?? TranslationMetadata(
		    locale: AppLocale.en,
		    overrides: overrides ?? {},
		    cardinalResolver: cardinalResolver,
		    ordinalResolver: ordinalResolver,
		  ) {
		$meta.setFlatMapFunction(_flatMapFunction);
	}

	/// Metadata for the translations of <en>.
	@override final TranslationMetadata<AppLocale, Translations> $meta;

	/// Access flat map
	dynamic operator[](String key) => $meta.getTranslation(key);

	late final Translations _root = this; // ignore: unused_field

	Translations $copyWith({TranslationMetadata<AppLocale, Translations>? meta}) => Translations(meta: meta ?? this.$meta);

	// Translations
	late final Translations$home$en home = Translations$home$en.internal(_root);
	late final Translations$auth$en auth = Translations$auth$en.internal(_root);
	late final Translations$profile$en profile = Translations$profile$en.internal(_root);
	late final Translations$onboarding$en onboarding = Translations$onboarding$en.internal(_root);
	late final Translations$email_verification$en email_verification = Translations$email_verification$en.internal(_root);
	late final Translations$dashboard$en dashboard = Translations$dashboard$en.internal(_root);
	late final Translations$settings$en settings = Translations$settings$en.internal(_root);
	late final Translations$common$en common = Translations$common$en.internal(_root);
	late final Translations$billing$en billing = Translations$billing$en.internal(_root);
	late final Translations$team$en team = Translations$team$en.internal(_root);
	late final Translations$channels$en channels = Translations$channels$en.internal(_root);
	late final Translations$whatsapp$en whatsapp = Translations$whatsapp$en.internal(_root);
	late final Translations$notifications$en notifications = Translations$notifications$en.internal(_root);
	late final Translations$api_keys$en api_keys = Translations$api_keys$en.internal(_root);
	late final Translations$notes$en notes = Translations$notes$en.internal(_root);
	late final Translations$message_actions$en message_actions = Translations$message_actions$en.internal(_root);
	late final Translations$contact_sheet$en contact_sheet = Translations$contact_sheet$en.internal(_root);
	late final Translations$conversation$en conversation = Translations$conversation$en.internal(_root);
	late final Translations$lgpd$en lgpd = Translations$lgpd$en.internal(_root);
	late final Translations$feedback$en feedback = Translations$feedback$en.internal(_root);
	late final Translations$product_tour$en product_tour = Translations$product_tour$en.internal(_root);
	late final Translations$feature_announcement$en feature_announcement = Translations$feature_announcement$en.internal(_root);
	late final Translations$errors$en errors = Translations$errors$en.internal(_root);
	late final Translations$audio_recorder$en audio_recorder = Translations$audio_recorder$en.internal(_root);
	late final Translations$bulk_actions$en bulk_actions = Translations$bulk_actions$en.internal(_root);
	late final Translations$image_viewer$en image_viewer = Translations$image_viewer$en.internal(_root);
	late final Translations$media_send$en media_send = Translations$media_send$en.internal(_root);
	late final Translations$reply_suggestions$en reply_suggestions = Translations$reply_suggestions$en.internal(_root);
	late final Translations$pwa$en pwa = Translations$pwa$en.internal(_root);
	late final Translations$audio_player$en audio_player = Translations$audio_player$en.internal(_root);
	late final Translations$media_preview$en media_preview = Translations$media_preview$en.internal(_root);
	late final Translations$sentiment$en sentiment = Translations$sentiment$en.internal(_root);
	late final Translations$owner$en owner = Translations$owner$en.internal(_root);
	late final Translations$sales$en sales = Translations$sales$en.internal(_root);
	late final Translations$support$en support = Translations$support$en.internal(_root);
	late final Translations$claude_integration$en claude_integration = Translations$claude_integration$en.internal(_root);
	late final Translations$shell$en shell = Translations$shell$en.internal(_root);
	late final Translations$board$en board = Translations$board$en.internal(_root);
	late final Translations$lead_detail$en lead_detail = Translations$lead_detail$en.internal(_root);
	late final Translations$account_menu$en account_menu = Translations$account_menu$en.internal(_root);
	late final Translations$copilot$en copilot = Translations$copilot$en.internal(_root);
	late final Translations$attachment$en attachment = Translations$attachment$en.internal(_root);
	late final Translations$sender_auth$en sender_auth = Translations$sender_auth$en.internal(_root);
	late final Translations$email_inbox$en email_inbox = Translations$email_inbox$en.internal(_root);
	late final Translations$email_risk$en email_risk = Translations$email_risk$en.internal(_root);
	late final Translations$brand$en brand = Translations$brand$en.internal(_root);
	late final Translations$suppressions$en suppressions = Translations$suppressions$en.internal(_root);
	late final Translations$signals$en signals = Translations$signals$en.internal(_root);
	late final Translations$sales_record$en sales_record = Translations$sales_record$en.internal(_root);
	late final Translations$portfolio$en portfolio = Translations$portfolio$en.internal(_root);
	late final Translations$cross_sell$en cross_sell = Translations$cross_sell$en.internal(_root);
	late final Translations$relationship$en relationship = Translations$relationship$en.internal(_root);
	late final Translations$command_center$en command_center = Translations$command_center$en.internal(_root);
	late final Translations$ai_debounce$en ai_debounce = Translations$ai_debounce$en.internal(_root);
	late final Translations$overnight$en overnight = Translations$overnight$en.internal(_root);
	late final Translations$export$en export = Translations$export$en.internal(_root);
	late final Translations$my_customers$en my_customers = Translations$my_customers$en.internal(_root);
	late final Translations$proposal$en proposal = Translations$proposal$en.internal(_root);
	late final Translations$product_import$en product_import = Translations$product_import$en.internal(_root);
	late final Translations$customer_import$en customer_import = Translations$customer_import$en.internal(_root);
	late final Translations$sales_import$en sales_import = Translations$sales_import$en.internal(_root);
	late final Translations$lead_timeline$en lead_timeline = Translations$lead_timeline$en.internal(_root);
	late final Translations$email_composer$en email_composer = Translations$email_composer$en.internal(_root);
	late final Translations$radar$en radar = Translations$radar$en.internal(_root);
	late final Translations$start$en start = Translations$start$en.internal(_root);
	late final Translations$calendar$en calendar = Translations$calendar$en.internal(_root);
}

// Path: home
class Translations$home$en {
	Translations$home$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Getting started — $current of $total'
	String onboarding_checklist_title({required Object current, required Object total}) => 'Getting started — ${current} of ${total}';

	/// en: 'Hide for now'
	String get onboarding_checklist_dismiss => 'Hide for now';

	/// en: 'Create your first agent'
	String get onboarding_checklist_agent => 'Create your first agent';

	/// en: 'Try your agent in the preview chat'
	String get onboarding_checklist_preview => 'Try your agent in the preview chat';

	/// en: 'Try now'
	String get onboarding_checklist_preview_cta => 'Try now';

	/// en: 'Connect a messaging channel'
	String get onboarding_checklist_channel => 'Connect a messaging channel';

	/// en: 'Connect'
	String get onboarding_checklist_channel_cta => 'Connect';

	/// en: 'Teach your agent about your business'
	String get onboarding_checklist_knowledge => 'Teach your agent about your business';

	/// en: 'Add context'
	String get onboarding_checklist_knowledge_cta => 'Add context';

	/// en: 'Receive your first real message'
	String get onboarding_checklist_inbound => 'Receive your first real message';

	/// en: 'Welcome, $name'
	String welcome_user({required Object name}) => 'Welcome, ${name}';

	/// en: 'Bring in what you sell'
	String get onboarding_checklist_products => 'Bring in what you sell';

	/// en: 'Import price list'
	String get onboarding_checklist_products_cta => 'Import price list';

	/// en: 'Bring in your customers'
	String get onboarding_checklist_customers => 'Bring in your customers';

	/// en: 'Import spreadsheet'
	String get onboarding_checklist_customers_cta => 'Import spreadsheet';

	/// en: 'Make your first proposal'
	String get onboarding_checklist_proposal => 'Make your first proposal';

	/// en: 'Open the funnel'
	String get onboarding_checklist_proposal_cta => 'Open the funnel';

	/// en: 'Invite your seller'
	String get onboarding_checklist_team => 'Invite your seller';

	/// en: 'Invite'
	String get onboarding_checklist_team_cta => 'Invite';
}

// Path: auth
class Translations$auth$en {
	Translations$auth$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Offers ready to approve, every day.'
	String get login_hero_title => 'Offers ready to approve, every day.';

	/// en: 'YOUR AUTONOMOUS SELLER'
	String get login_eyebrow => 'YOUR AUTONOMOUS SELLER';

	/// en: 'Sign in to your account'
	String get login_title => 'Sign in to your account';

	/// en: 'Enter your email'
	String get email_placeholder => 'Enter your email';

	/// en: 'Enter your password'
	String get password_placeholder => 'Enter your password';

	/// en: 'Remember me'
	String get remember_me => 'Remember me';

	/// en: 'Forgot password'
	String get forgot_password => 'Forgot password';

	/// en: 'Login'
	String get login_button => 'Login';

	/// en: 'Or'
	String get or_login_with => 'Or';

	/// en: 'Google'
	String get google => 'Google';

	/// en: 'Facebook'
	String get meta => 'Facebook';

	/// en: 'Don't have an account?'
	String get dont_have_account => 'Don\'t have an account?';

	/// en: 'Create an account'
	String get create_account => 'Create an account';

	/// en: 'Create Account'
	String get register_title => 'Create Account';

	/// en: 'Start using Replider today'
	String get register_subtitle => 'Start using Replider today';

	/// en: 'Sign-ups opening soon'
	String get register_paused_title => 'Sign-ups opening soon';

	/// en: 'We're building the new Replider and sign-ups are temporarily closed. Already have an account? Just log in.'
	String get register_paused_body => 'We\'re building the new Replider and sign-ups are temporarily closed. Already have an account? Just log in.';

	/// en: 'Sign-ups are temporarily closed.'
	String get register_paused_error => 'Sign-ups are temporarily closed.';

	/// en: 'Company name'
	String get account_name => 'Company name';

	/// en: 'Full name'
	String get full_name => 'Full name';

	/// en: 'Confirm password'
	String get confirm_password => 'Confirm password';

	/// en: 'Already have an account?'
	String get already_have_account => 'Already have an account?';

	/// en: 'Sign in'
	String get sign_in => 'Sign in';

	/// en: 'Required field'
	String get field_required => 'Required field';

	/// en: 'Invalid email'
	String get invalid_email => 'Invalid email';

	/// en: 'Password too short'
	String get password_too_short => 'Password too short';

	/// en: 'Minimum $n characters'
	String min_characters({required Object n}) => 'Minimum ${n} characters';

	/// en: 'Passwords don't match'
	String get passwords_dont_match => 'Passwords don\'t match';

	/// en: 'Reset password'
	String get forgot_title => 'Reset password';

	/// en: 'Enter your email and we'll send you reset instructions.'
	String get forgot_subtitle => 'Enter your email and we\'ll send you reset instructions.';

	/// en: 'Send reset link'
	String get forgot_send_link => 'Send reset link';

	/// en: 'Resend in ${seconds}s'
	String forgot_resend_in({required Object seconds}) => 'Resend in ${seconds}s';

	/// en: 'If an account exists, you'll receive an email shortly.'
	String get forgot_success => 'If an account exists, you\'ll receive an email shortly.';

	/// en: 'Set a new password'
	String get reset_title => 'Set a new password';

	/// en: 'Choose a new password for your account.'
	String get reset_subtitle => 'Choose a new password for your account.';

	/// en: 'New password'
	String get reset_new_password => 'New password';

	/// en: 'Confirm new password'
	String get reset_confirm_password => 'Confirm new password';

	/// en: 'Password must be at least 8 characters.'
	String get reset_min_length => 'Password must be at least 8 characters.';

	/// en: 'Passwords don't match.'
	String get reset_mismatch => 'Passwords don\'t match.';

	/// en: 'Reset password'
	String get reset_submit => 'Reset password';

	/// en: 'Password reset! You can now log in with your new password.'
	String get reset_success => 'Password reset! You can now log in with your new password.';

	/// en: 'This reset link is invalid or incomplete. Request a new one.'
	String get reset_invalid_link => 'This reset link is invalid or incomplete. Request a new one.';

	/// en: 'Back to login'
	String get back_to_login => 'Back to login';

	/// en: 'You must accept the Terms of Service and Privacy Policy.'
	String get terms_required => 'You must accept the Terms of Service and Privacy Policy.';

	/// en: 'Try again'
	String get retry => 'Try again';

	/// en: 'Two-factor verification'
	String get mfa_title => 'Two-factor verification';

	/// en: 'Enter the 6-digit code from your authenticator app, or one of your recovery codes.'
	String get mfa_subtitle => 'Enter the 6-digit code from your authenticator app, or one of your recovery codes.';

	/// en: 'Code'
	String get mfa_code_hint => 'Code';

	/// en: 'Verify and continue'
	String get mfa_verify_button => 'Verify and continue';

	/// en: 'Back to sign in'
	String get mfa_cancel => 'Back to sign in';

	/// en: 'Coupon $code applied — $percent% off at checkout.'
	String coupon_banner_valid({required Object code, required Object percent}) => 'Coupon ${code} applied — ${percent}% off at checkout.';

	/// en: 'Coupon $code is no longer valid.'
	String coupon_banner_invalid({required Object code}) => 'Coupon ${code} is no longer valid.';

	/// en: 'Your session expired for security. Sign in again — you'll return to the screen you were on.'
	String get session_expired_notice => 'Your session expired for security. Sign in again — you\'ll return to the screen you were on.';

	/// en: 'Your session ends in $minutes min'
	String session_ending_title({required Object minutes}) => 'Your session ends in ${minutes} min';

	/// en: 'Sign in again now, on your own terms, instead of losing whatever you are in the middle of.'
	String get session_ending_body => 'Sign in again now, on your own terms, instead of losing whatever you are in the middle of.';

	/// en: 'Sign in again'
	String get session_ending_cta => 'Sign in again';

	/// en: 'Invite for $email — your account starts on the contracted plan.'
	String invite_banner({required Object email}) => 'Invite for ${email} — your account starts on the contracted plan.';

	/// en: 'This invite link is invalid, expired or already used. Ask for a new link.'
	String get invite_invalid => 'This invite link is invalid, expired or already used. Ask for a new link.';

	/// en: 'Invite link expired'
	String get invite_invalid_title => 'Invite link expired';

	/// en: 'Activate the account on the plan we agreed with you.'
	String get register_subtitle_invite => 'Activate the account on the plan we agreed with you.';

	/// en: 'We couldn't verify your invite right now — a connection problem, not the link. Try again.'
	String get invite_probe_failed => 'We couldn\'t verify your invite right now — a connection problem, not the link. Try again.';

	/// en: 'your account's e-mail'
	String get verify_email_unknown => 'your account\'s e-mail';

	/// en: 'I have read and accept the '
	String get consent_prefix => 'I have read and accept the ';

	/// en: 'Terms of Use'
	String get consent_terms => 'Terms of Use';

	/// en: ' and the '
	String get consent_and => ' and the ';

	/// en: 'Privacy Policy'
	String get consent_privacy => 'Privacy Policy';
}

// Path: profile
class Translations$profile$en {
	Translations$profile$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'My Profile'
	String get title => 'My Profile';

	/// en: 'User not found'
	String get user_not_found => 'User not found';

	/// en: 'PERSONAL INFORMATION'
	String get personal_info => 'PERSONAL INFORMATION';

	/// en: 'Name'
	String get name => 'Name';

	/// en: 'Email'
	String get email => 'Email';

	/// en: 'Save Information'
	String get save_info => 'Save Information';

	/// en: 'CHANGE PASSWORD'
	String get change_password => 'CHANGE PASSWORD';

	/// en: 'Current password'
	String get current_password => 'Current password';

	/// en: 'New password'
	String get new_password => 'New password';

	/// en: 'Confirm new password'
	String get confirm_new_password => 'Confirm new password';

	/// en: 'Change Password'
	String get change_password_button => 'Change Password';

	/// en: 'Fill in all fields'
	String get fill_all_fields => 'Fill in all fields';

	/// en: 'Fill in all password fields'
	String get fill_all_password_fields => 'Fill in all password fields';

	/// en: 'Passwords don't match'
	String get passwords_dont_match => 'Passwords don\'t match';

	/// en: 'New password must be at least 6 characters'
	String get min_password_length => 'New password must be at least 6 characters';

	/// en: 'Profile updated successfully!'
	String get profile_updated => 'Profile updated successfully!';

	/// en: 'Password changed successfully!'
	String get password_changed => 'Password changed successfully!';

	/// en: 'Profile update not yet available in the API'
	String get profile_update_unavailable => 'Profile update not yet available in the API';

	/// en: 'Password change not yet available in the API'
	String get password_change_unavailable => 'Password change not yet available in the API';

	/// en: 'Incorrect current password'
	String get incorrect_current_password => 'Incorrect current password';

	/// en: 'Unexpected error'
	String get unexpected_error => 'Unexpected error';

	/// en: 'Network error'
	String get network_error => 'Network error';

	/// en: 'Error'
	String get error_prefix => 'Error';

	/// en: 'TWO-FACTOR AUTHENTICATION'
	String get two_factor_section_label => 'TWO-FACTOR AUTHENTICATION';

	/// en: 'Two-factor authentication'
	String get two_factor_label => 'Two-factor authentication';

	/// en: 'Adds a second step at login using a 6-digit code from an authenticator app (Google Authenticator, 1Password, Authy).'
	String get two_factor_desc => 'Adds a second step at login using a 6-digit code from an authenticator app (Google Authenticator, 1Password, Authy).';

	/// en: 'OFF'
	String get two_factor_status_off => 'OFF';

	/// en: 'ON'
	String get two_factor_status_on => 'ON';

	/// en: 'Set up 2FA'
	String get two_factor_setup_cta => 'Set up 2FA';

	/// en: '1. Scan this QR with your authenticator app'
	String get two_factor_step1 => '1. Scan this QR with your authenticator app';

	/// en: '2. Enter the 6-digit code shown in the app'
	String get two_factor_step2 => '2. Enter the 6-digit code shown in the app';

	/// en: 'Or enter this secret manually:'
	String get two_factor_manual_secret_label => 'Or enter this secret manually:';

	/// en: 'Verify and enable'
	String get two_factor_verify_cta => 'Verify and enable';

	/// en: 'Enter a current 2FA code (or recovery code) to disable.'
	String get two_factor_disable_prompt => 'Enter a current 2FA code (or recovery code) to disable.';

	/// en: '6-digit code or recovery code'
	String get two_factor_code_hint => '6-digit code or recovery code';

	/// en: 'Disable 2FA'
	String get two_factor_disable_cta => 'Disable 2FA';

	/// en: 'Save your recovery codes'
	String get two_factor_recovery_title => 'Save your recovery codes';

	/// en: 'Each code works once. Store them somewhere safe — without them, losing your phone locks you out.'
	String get two_factor_recovery_warning => 'Each code works once. Store them somewhere safe — without them, losing your phone locks you out.';

	/// en: 'Copy all'
	String get two_factor_recovery_copy_cta => 'Copy all';

	/// en: 'I've saved them'
	String get two_factor_recovery_done_cta => 'I\'ve saved them';

	/// en: 'Signature'
	String get signature_label => 'Signature';

	/// en: 'Optional sign-off offered in the composer on manual replies. Never added to AI responses.'
	String get signature_help => 'Optional sign-off offered in the composer on manual replies. Never added to AI responses.';
}

// Path: onboarding
class Translations$onboarding$en {
	Translations$onboarding$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Welcome to Replider!'
	String get welcome_title => 'Welcome to Replider!';

	/// en: 'Let's set up your workspace in a few quick steps.'
	String get welcome_subtitle => 'Let\'s set up your workspace in a few quick steps.';

	/// en: 'Your company name'
	String get company_name_hint => 'Your company name';

	/// en: 'About your company'
	String get step_company_title => 'About your company';

	/// en: 'The AI writes its outreach from this. Without it, all it knows is the name.'
	String get step_company_subtitle => 'The AI writes its outreach from this. Without it, all it knows is the name.';

	/// en: 'e.g. Your Company Hospital Equipment'
	String get step_company_placeholder => 'e.g. Your Company Hospital Equipment';

	/// en: 'What you do'
	String get company_about_label => 'What you do';

	/// en: 'e.g. Sales, rental and technical service of hospital equipment in Mato Grosso.'
	String get company_about_placeholder => 'e.g. Sales, rental and technical service of hospital equipment in Mato Grosso.';

	/// en: 'One sentence. Who you serve and what you solve.'
	String get company_about_help => 'One sentence. Who you serve and what you solve.';

	/// en: 'How you speak (optional)'
	String get company_tone_label => 'How you speak (optional)';

	/// en: 'How does your company earn?'
	String get company_lines_label => 'How does your company earn?';

	/// en: 'I sell equipment'
	String get company_line_sell => 'I sell equipment';

	/// en: 'I rent equipment'
	String get company_line_rent => 'I rent equipment';

	/// en: 'I do technical service'
	String get company_line_service => 'I do technical service';

	/// en: 'Where do you sell?'
	String get company_territory_label => 'Where do you sell?';

	/// en: 'State'
	String get company_territory_uf => 'State';

	/// en: 'Cities (optional)'
	String get company_territory_cities => 'Cities (optional)';

	/// en: 'Cuiabá, Várzea Grande'
	String get company_territory_cities_hint => 'Cuiabá, Várzea Grande';

	/// en: 'Direct'
	String get company_tone_direct => 'Direct';

	/// en: 'Write directly and to the point, no filler and no sales adjectives.'
	String get company_tone_direct_prose => 'Write directly and to the point, no filler and no sales adjectives.';

	/// en: 'Consultative'
	String get company_tone_consultative => 'Consultative';

	/// en: 'Write like a specialist who understands the customer's problem, explaining the why before the what.'
	String get company_tone_consultative_prose => 'Write like a specialist who understands the customer\'s problem, explaining the why before the what.';

	/// en: 'Warm'
	String get company_tone_warm => 'Warm';

	/// en: 'Write warmly and personally, like someone who already knows them, without being over-familiar.'
	String get company_tone_warm_prose => 'Write warmly and personally, like someone who already knows them, without being over-familiar.';

	/// en: 'Formal'
	String get company_tone_formal => 'Formal';

	/// en: 'Write formally and respectfully, avoiding slang.'
	String get company_tone_formal_prose => 'Write formally and respectfully, avoiding slang.';

	/// en: 'Give your agent a name and choose its AI brain.'
	String get step_agent_subtitle => 'Give your agent a name and choose its AI brain.';

	/// en: 'AI Provider'
	String get provider_label => 'AI Provider';

	/// en: 'Where do you send from?'
	String get step_channel_title => 'Where do you send from?';

	/// en: 'The first offer goes out by email: connect the account you already use. WhatsApp comes later, for the relationship.'
	String get step_channel_subtitle => 'The first offer goes out by email: connect the account you already use. WhatsApp comes later, for the relationship.';

	/// en: 'Fastest'
	String get step_channel_badge_telegram => 'Fastest';

	/// en: 'Popular · ~10 min'
	String get step_channel_badge_whatsapp => 'Popular · ~10 min';

	/// en: 'Coming soon'
	String get step_channel_badge_coming_soon => 'Coming soon';

	/// en: 'Not sure where to start?'
	String get step_channel_whatsapp_help => 'Not sure where to start?';

	/// en: 'Couldn't open the help video. Please check your browser settings.'
	String get step_channel_whatsapp_help_failed => 'Couldn\'t open the help video. Please check your browser settings.';

	/// en: 'Telegram Bot Token'
	String get telegram_bot_token => 'Telegram Bot Token';

	/// en: 'WhatsApp Phone Number ID'
	String get whatsapp_phone_number_id => 'WhatsApp Phone Number ID';

	/// en: 'WhatsApp Access Token'
	String get whatsapp_access_token => 'WhatsApp Access Token';

	/// en: 'Instagram Page ID'
	String get instagram_page_id => 'Instagram Page ID';

	/// en: 'Instagram Access Token'
	String get instagram_access_token => 'Instagram Access Token';

	/// en: 'Requires an Instagram professional account (Business or Creator) and an access token with the instagram_business_manage_messages permission.'
	String get step_channel_instagram_hint => 'Requires an Instagram professional account (Business or Creator) and an access token with the instagram_business_manage_messages permission.';

	/// en: 'Twilio Account SID'
	String get sms_account_sid => 'Twilio Account SID';

	/// en: 'Twilio Auth Token'
	String get sms_auth_token => 'Twilio Auth Token';

	/// en: 'Twilio Phone Number (E.164)'
	String get sms_from_number => 'Twilio Phone Number (E.164)';

	/// en: 'Bring your own Twilio number: paste the Account SID, Auth Token, and phone number from the Twilio console. After finishing, copy the channel's webhook URL in Settings → Channels and set it on your number in Twilio.'
	String get step_channel_sms_hint => 'Bring your own Twilio number: paste the Account SID, Auth Token, and phone number from the Twilio console. After finishing, copy the channel\'s webhook URL in Settings → Channels and set it on your number in Twilio.';

	/// en: 'The Account SID must start with AC followed by 32 characters. Copy it from the Twilio console.'
	String get invalid_sms_account_sid => 'The Account SID must start with AC followed by 32 characters. Copy it from the Twilio console.';

	/// en: 'Paste your Twilio Auth Token to connect the SMS channel.'
	String get invalid_sms_auth_token => 'Paste your Twilio Auth Token to connect the SMS channel.';

	/// en: 'The phone number must be in international E.164 format, e.g. +15551234567.'
	String get invalid_sms_from_number => 'The phone number must be in international E.164 format, e.g. +15551234567.';

	/// en: 'Create the agent before uploading the file.'
	String get create_agent_first_for_file => 'Create the agent before uploading the file.';

	/// en: 'Create the agent before importing the URL.'
	String get create_agent_first_for_url => 'Create the agent before importing the URL.';

	/// en: '$fileName · $count snippets indexed'
	String knowledge_summary_file({required Object fileName, required Object count}) => '${fileName} · ${count} snippets indexed';

	/// en: '$pages pages · $count snippets indexed'
	String knowledge_summary_url({required Object pages, required Object count}) => '${pages} pages · ${count} snippets indexed';

	/// en: 'You're all set!'
	String get step_done_title => 'You\'re all set!';

	/// en: 'Your workspace is ready. Messages will start flowing as soon as customers reach out.'
	String get step_done_subtitle => 'Your workspace is ready. Messages will start flowing as soon as customers reach out.';

	/// en: 'Test your agent in Settings → Agent → Preview'
	String get step_done_tip_1 => 'Test your agent in Settings → Agent → Preview';

	/// en: 'Monitor conversations in the Dashboard'
	String get step_done_tip_2 => 'Monitor conversations in the Dashboard';

	/// en: 'Add more knowledge anytime in Settings'
	String get step_done_tip_3 => 'Add more knowledge anytime in Settings';

	/// en: 'Go to Dashboard'
	String get go_to_dashboard => 'Go to Dashboard';

	/// en: 'Next'
	String get next => 'Next';

	/// en: 'Back'
	String get back => 'Back';

	/// en: 'Skip for now'
	String get skip => 'Skip for now';

	/// en: 'Step $current of $total'
	String step_of({required Object current, required Object total}) => 'Step ${current} of ${total}';

	/// en: 'Required field'
	String get field_required => 'Required field';

	/// en: 'It works! Next'
	String get step_preview_chat_next => 'It works! Next';

	/// en: 'Test later'
	String get step_preview_chat_skip => 'Test later';

	/// en: 'Enter the Phone Number ID from Meta for Developers. The access token is managed automatically.'
	String get step_channel_whatsapp_manual_hint => 'Enter the Phone Number ID from Meta for Developers. The access token is managed automatically.';

	/// en: 'WhatsApp connected! Click "Next" to continue.'
	String get step_channel_whatsapp_connected => 'WhatsApp connected! Click "Next" to continue.';

	/// en: 'or connect manually'
	String get step_channel_whatsapp_or_manual => 'or connect manually';

	/// en: 'Main'
	String get step_channel_badge_email => 'Main';

	/// en: 'Sending address'
	String get email_from_address => 'Sending address';

	/// en: 'SMTP server'
	String get email_smtp_host => 'SMTP server';

	/// en: 'Port'
	String get email_smtp_port => 'Port';

	/// en: 'SMTP password'
	String get email_smtp_password => 'SMTP password';

	/// en: 'Next: authorise your domain to send'
	String get email_dns_title => 'Next: authorise your domain to send';

	/// en: 'Your domain must publish three records (SPF, DKIM and DMARC) so the mail reaches the inbox instead of spam. When you finish, Replider shows the exact records to copy. Propagation takes minutes to a few hours.'
	String get email_dns_body => 'Your domain must publish three records (SPF, DKIM and DMARC) so the mail reaches the inbox instead of spam. When you finish, Replider shows the exact records to copy. Propagation takes minutes to a few hours.';

	/// en: 'What do you sell?'
	String get products_title => 'What do you sell?';

	/// en: 'Bring your price list as a file. Every proposal is made of these, and you can add or fix them later.'
	String get products_subtitle => 'Bring your price list as a file. Every proposal is made of these, and you can add or fix them later.';

	/// en: 'Your AI key'
	String get aikey_title => 'Your AI key';

	/// en: 'On your contract the AI runs on YOUR key — the usage is yours, with nobody in between. Paste the Google AI Studio (Gemini) key so it can read catalogues and write offers.'
	String get aikey_subtitle => 'On your contract the AI runs on YOUR key — the usage is yours, with nobody in between. Paste the Google AI Studio (Gemini) key so it can read catalogues and write offers.';

	/// en: 'API key (Gemini)'
	String get aikey_field => 'API key (Gemini)';

	/// en: 'Stored encrypted and never shown back — only the last digits.'
	String get aikey_hint => 'Stored encrypted and never shown back — only the last digits.';

	/// en: 'Key configured ✓'
	String get aikey_saved => 'Key configured ✓';

	/// en: 'This account already has a key. You can move on.'
	String get aikey_saved_hint => 'This account already has a key. You can move on.';

	/// en: 'Save key'
	String get aikey_save => 'Save key';

	/// en: 'You can move on without one: the price list comes in as CSV, which uses no AI. Catalogue PDFs and AI offers wait until you paste a key.'
	String get aikey_skip_note => 'You can move on without one: the price list comes in as CSV, which uses no AI. Catalogue PDFs and AI offers wait until you paste a key.';

	/// en: 'Reading a PDF needs the account's AI key. For now bring the list as CSV — it uses no AI.'
	String get products_needs_key => 'Reading a PDF needs the account\'s AI key. For now bring the list as CSV — it uses no AI.';

	/// en: 'I have my own mail server (SMTP)'
	String get email_smtp_toggle => 'I have my own mail server (SMTP)';

	/// en: 'Other channels'
	String get step_channel_other_channels => 'Other channels';

	/// en: 'Hide other channels'
	String get step_channel_other_channels_hide => 'Hide other channels';

	/// en: 'Who already buys from you?'
	String get customers_title => 'Who already buys from you?';

	/// en: 'Bring your customer list as a file. Repurchases, cross-sell and the first offer all start from it — you can add more later.'
	String get customers_subtitle => 'Bring your customer list as a file. Repurchases, cross-sell and the first offer all start from it — you can add more later.';

	/// en: 'Your first offer'
	String get first_offer_title => 'Your first offer';

	/// en: 'Reading your products and your customers and writing the first message…'
	String get first_offer_writing => 'Reading your products and your customers and writing the first message…';

	/// en: 'For $name, one of your customers:'
	String first_offer_for_customer({required Object name}) => 'For ${name}, one of your customers:';

	/// en: 'For $name, found in your territory:'
	String first_offer_for_prospect({required Object name}) => 'For ${name}, found in your territory:';

	/// en: 'This is how opportunities will arrive every day. This one is already in your Opportunities list — approve it, edit it, or ask for another approach there. Nothing leaves without you.'
	String get first_offer_in_queue => 'This is how opportunities will arrive every day. This one is already in your Opportunities list — approve it, edit it, or ask for another approach there. Nothing leaves without you.';

	/// en: 'Nothing to write yet: bring a customer list, or say which state you sell in, and the first opportunity will be waiting in Opportunities.'
	String get first_offer_nothing => 'Nothing to write yet: bring a customer list, or say which state you sell in, and the first opportunity will be waiting in Opportunities.';

	/// en: 'We could not find a company in your territory right now. Bring a customer list, or try the search in Opportunities later.'
	String get first_offer_no_business => 'We could not find a company in your territory right now. Bring a customer list, or try the search in Opportunities later.';

	/// en: 'See my offer queue'
	String get finish_to_queue => 'See my offer queue';

	/// en: 'File not imported'
	String get unconfirmed_import_title => 'File not imported';

	/// en: 'You reviewed the file but haven't confirmed the import. If you continue now, nothing will be created.'
	String get unconfirmed_import_body => 'You reviewed the file but haven\'t confirmed the import. If you continue now, nothing will be created.';

	/// en: 'Continue without importing'
	String get unconfirmed_import_confirm => 'Continue without importing';

	/// en: 'Fields marked * are required — Next unlocks once all four are filled.'
	String get company_required_hint => 'Fields marked * are required — Next unlocks once all four are filled.';

	/// en: 'Go back and import customers'
	String get first_offer_import_cta => 'Go back and import customers';

	/// en: 'Company search isn't available right now — import a customer list, or skip: prospecting works normally later.'
	String get first_offer_unavailable => 'Company search isn\'t available right now — import a customer list, or skip: prospecting works normally later.';

	/// en: 'Company search isn't set up on the platform yet — import a customer list, or skip: prospecting is enabled later.'
	String get first_offer_prospecting_not_configured => 'Company search isn\'t set up on the platform yet — import a customer list, or skip: prospecting is enabled later.';

	/// en: 'Import products'
	String get first_offer_cta_products => 'Import products';

	/// en: 'Fix the customer list'
	String get first_offer_cta_customers => 'Fix the customer list';

	/// en: 'Open Settings'
	String get first_offer_cta_settings => 'Open Settings';
}

// Path: email_verification
class Translations$email_verification$en {
	Translations$email_verification$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Check your email'
	String get title => 'Check your email';

	/// en: 'We've sent a verification link to'
	String get subtitle => 'We\'ve sent a verification link to';

	/// en: 'Click the link in the email to activate your account. The link expires in 24 hours.'
	String get instruction => 'Click the link in the email to activate your account. The link expires in 24 hours.';

	/// en: 'Back to Login'
	String get back_to_login => 'Back to Login';

	/// en: 'Didn't receive the email?'
	String get didnt_receive => 'Didn\'t receive the email?';

	/// en: 'Resend email'
	String get resend => 'Resend email';

	/// en: 'Email sent. Check your inbox in a minute.'
	String get resent => 'Email sent. Check your inbox in a minute.';

	/// en: 'Could not resend. Try again in a moment.'
	String get resend_failed => 'Could not resend. Try again in a moment.';

	/// en: 'Also check your spam folder.'
	String get check_spam => 'Also check your spam folder.';

	/// en: 'Confirm your email to secure your account.'
	String get banner_message => 'Confirm your email to secure your account.';

	/// en: 'Verifying your email…'
	String get verifying => 'Verifying your email…';

	/// en: 'Email verified'
	String get verified_title => 'Email verified';

	/// en: 'Your email is confirmed. You can log in now.'
	String get verified_subtitle => 'Your email is confirmed. You can log in now.';

	/// en: 'Verification failed'
	String get verify_failed_title => 'Verification failed';

	/// en: 'This link is invalid or has expired. Request a new one from the app.'
	String get verify_failed_subtitle => 'This link is invalid or has expired. Request a new one from the app.';
}

// Path: dashboard
class Translations$dashboard$en {
	Translations$dashboard$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Conversations'
	String get conversations_title => 'Conversations';

	/// en: 'Customer'
	String get unknown_contact => 'Customer';

	/// en: '$contact has been waiting $minutes min — reply now'
	String reply_reminder_toast({required Object contact, required Object minutes}) => '${contact} has been waiting ${minutes} min — reply now';

	/// en: 'Open'
	String get reply_reminder_open => 'Open';

	/// en: 'Search conversations...'
	String get search_conversations => 'Search conversations...';

	/// en: 'Search in conversation...'
	String get search_in_conversation => 'Search in conversation...';

	/// en: 'Select a conversation'
	String get no_conversation_selected => 'Select a conversation';

	/// en: 'Pick a conversation from the list to view its messages.'
	String get no_conversation_selected_subtitle => 'Pick a conversation from the list to view its messages.';

	/// en: 'Settings'
	String get settings => 'Settings';

	/// en: 'Type a message...'
	String get message_placeholder => 'Type a message...';

	/// en: 'No conversations found'
	String get no_conversations_found => 'No conversations found';

	/// en: 'Your inbox is empty. Connect an email to receive and reply to customer messages here.'
	String get empty_inbox_hint_setup => 'Your inbox is empty. Connect an email to receive and reply to customer messages here.';

	/// en: 'Connect email'
	String get empty_inbox_cta_setup => 'Connect email';

	/// en: 'Your inbox is empty. Customer replies to your offers show up here.'
	String get empty_inbox_hint_share => 'Your inbox is empty. Customer replies to your offers show up here.';

	/// en: 'View offers'
	String get empty_inbox_cta_share => 'View offers';

	/// en: 'No messages'
	String get no_messages => 'No messages';

	/// en: 'Today'
	String get today => 'Today';

	/// en: 'Yesterday'
	String get yesterday => 'Yesterday';

	/// en: 'Now'
	String get now => 'Now';

	/// en: 'AI active'
	String get bot_active => 'AI active';

	/// en: 'Waiting for agent'
	String get waiting_agent => 'Waiting for agent';

	/// en: 'Closed'
	String get closed => 'Closed';

	/// en: 'Agent'
	String get agent_label => 'Agent';

	/// en: 'Search...'
	String get search_hint => 'Search...';

	/// en: 'Close search'
	String get close_search => 'Close search';

	/// en: 'Search'
	String get search => 'Search';

	/// en: 'Analytics'
	String get analytics => 'Analytics';

	/// en: 'Settings'
	String get settings_tooltip => 'Settings';

	/// en: 'Search...'
	String get search_in_messages => 'Search...';

	/// en: 'No conversations matching "$query"'
	String no_conversations_for_query({required Object query}) => 'No conversations matching "${query}"';

	/// en: '$count result(s) for "$query"'
	String results_for_query({required Object count, required Object query}) => '${count} result(s) for "${query}"';

	/// en: 'Failed to load conversations'
	String get error_loading_conversations => 'Failed to load conversations';

	/// en: 'Check your connection and try again.'
	String get check_connection => 'Check your connection and try again.';

	/// en: 'All'
	String get filter_all => 'All';

	/// en: 'Open'
	String get filter_open => 'Open';

	/// en: 'Pending'
	String get filter_pending => 'Pending';

	/// en: 'Closed'
	String get filter_closed => 'Closed';

	/// en: 'Mine'
	String get assignment_mine => 'Mine';

	/// en: 'Unassigned'
	String get assignment_unassigned => 'Unassigned';

	/// en: 'All'
	String get assignment_all => 'All';

	/// en: 'Recent'
	String get sort_recent => 'Recent';

	/// en: 'Order received'
	String get sort_first_in_conv => 'Order received';

	/// en: 'Oldest'
	String get sort_oldest => 'Oldest';

	/// en: 'Select a conversation'
	String get select_conversation_title => 'Select a conversation';

	/// en: 'Pick a conversation from the list to view the messages.'
	String get select_conversation_subtitle => 'Pick a conversation from the list to\nview the messages.';

	/// en: 'Drop to send'
	String get drop_to_send => 'Drop to send';

	/// en: 'Search...'
	String get search_messages_hint => 'Search...';

	/// en: 'Conversation summary (AI)'
	String get summary_tooltip => 'Conversation summary (AI)';

	/// en: 'Send file'
	String get send_file_tooltip => 'Send file';

	/// en: 'Record audio'
	String get record_audio_tooltip => 'Record audio';

	/// en: 'Send Flow'
	String get send_flow_tooltip => 'Send Flow';

	/// en: 'Type a message...'
	String get type_a_message => 'Type a message...';

	/// en: 'AI paused'
	String get bot_paused_pill => 'AI paused';

	/// en: 'Under human handling'
	String get human_handling => 'Under human handling';

	/// en: 'AI active'
	String get status_open_bot => 'AI active';

	/// en: 'Waiting for agent'
	String get status_pending_human => 'Waiting for agent';

	/// en: 'Closed'
	String get status_closed => 'Closed';

	/// en: 'Today'
	String get day_today => 'Today';

	/// en: 'Yesterday'
	String get day_yesterday => 'Yesterday';

	/// en: 'Conversation summary'
	String get summary_title => 'Conversation summary';

	/// en: 'Failed to generate summary: $error'
	String summary_failed({required Object error}) => 'Failed to generate summary: ${error}';

	/// en: 'Not enough messages to summarize.'
	String get summary_empty => 'Not enough messages to summarize.';

	/// en: 'Refresh'
	String get summary_refresh => 'Refresh';

	/// en: '24h window expired. Send an approved template to reopen the conversation.'
	String get wa_24h_window_expired => '24h window expired. Send an approved template to reopen the conversation.';

	/// en: 'Send template'
	String get wa_send_template => 'Send template';

	/// en: 'Retry'
	String get retry_send => 'Retry';

	/// en: 'Flow "$name" sent.'
	String flow_sent({required Object name}) => 'Flow "${name}" sent.';

	/// en: 'Token usage at $percent% of limit.'
	String token_usage_warning_snackbar({required Object percent}) => 'Token usage at ${percent}% of limit.';

	/// en: 'Reconnecting...'
	String get reconnecting => 'Reconnecting...';

	/// en: 'AI credits exhausted. Buy an extra pack or upgrade.'
	String get tokens_exhausted_banner => 'AI credits exhausted. Buy an extra pack or upgrade.';

	/// en: 'You've used $percent% of your AI credits this month.'
	String tokens_high_usage_banner({required Object percent}) => 'You\'ve used ${percent}% of your AI credits this month.';

	/// en: 'Talk to us'
	String get upgrade_btn => 'Talk to us';

	/// en: 'Your trial period has ended. Upgrade to keep using Replider.'
	String get trial_ended => 'Your trial period has ended. Upgrade to keep using Replider.';

	/// en: 'Last day of free trial. Upgrade to keep service running.'
	String get trial_last_day => 'Last day of free trial. Upgrade to keep service running.';

	/// en: '$days days left in your free trial ($used/$max conversations). Upgrade to unlock more.'
	String trial_days_remaining({required Object days, required Object used, required Object max}) => '${days} days left in your free trial (${used}/${max} conversations). Upgrade to unlock more.';

	/// en: 'You're on a trial ($used of $max conversations used). Want the full product working for your company? Let's talk.'
	String trial_free_plan({required Object used, required Object max}) => 'You\'re on a trial (${used} of ${max} conversations used). Want the full product working for your company? Let\'s talk.';

	/// en: 'Insert signature'
	String get insert_signature_tooltip => 'Insert signature';

	/// en: 'All channels'
	String get channel_all => 'All channels';

	/// en: 'Email'
	String get channel_email => 'Email';

	/// en: 'WhatsApp'
	String get channel_whatsapp => 'WhatsApp';

	/// en: 'Telegram'
	String get channel_telegram => 'Telegram';

	/// en: 'Instagram'
	String get channel_instagram => 'Instagram';

	/// en: 'SMS'
	String get channel_sms => 'SMS';

	/// en: 'Most important'
	String get sort_importance => 'Most important';

	/// en: 'All categories'
	String get category_all => 'All categories';

	/// en: 'Hot lead'
	String get category_hot_lead => 'Hot lead';

	/// en: 'Customer'
	String get category_customer => 'Customer';

	/// en: 'Financial'
	String get category_financial => 'Financial';

	/// en: 'Supplier'
	String get category_supplier => 'Supplier';

	/// en: 'Newsletter'
	String get category_newsletter => 'Newsletter';

	/// en: 'Quarantine'
	String get category_quarantine => 'Quarantine';

	/// en: 'Other'
	String get category_other => 'Other';

	/// en: 'Filters'
	String get filters_button => 'Filters';

	/// en: 'Filters · $count'
	String filters_button_n({required Object count}) => 'Filters · ${count}';

	/// en: 'Clear filters'
	String get filters_clear => 'Clear filters';

	/// en: 'Filter conversations'
	String get filters_title => 'Filter conversations';

	/// en: 'Status'
	String get filter_label_status => 'Status';

	/// en: 'Channel'
	String get filter_label_channel => 'Channel';

	/// en: 'Order'
	String get filter_label_sort => 'Order';

	/// en: 'Category'
	String get filter_label_category => 'Category';

	/// en: 'No conversations match these filters.'
	String get no_conversations_for_filters => 'No conversations match these filters.';

	/// en: 'Clear filters'
	String get clear_filters => 'Clear filters';

	/// en: 'File'
	String get media_file_label => 'File';

	/// en: 'Location'
	String get media_location_label => 'Location';

	/// en: 'Content not delivered by WhatsApp'
	String get wa_unsupported_title => 'Content not delivered by WhatsApp';

	/// en: 'Polls, live locations and view-once media never reach the service side. Ask the customer to resend it another way.'
	String get wa_unsupported_body => 'Polls, live locations and view-once media never reach the service side. Ask the customer to resend it another way.';

	/// en: 'The message field comes back when the customer replies.'
	String get wa_window_composer_back => 'The message field comes back when the customer replies.';

	/// en: 'AGREED'
	String get brief_agreed => 'AGREED';

	/// en: 'PENDING'
	String get brief_pending => 'PENDING';

	/// en: 'NEXT STEP'
	String get brief_next_step => 'NEXT STEP';

	/// en: 'generated just now'
	String get brief_generated_now => 'generated just now';

	/// en: 'generated $when ago'
	String brief_generated_ago({required Object when}) => 'generated ${when} ago';

	/// en: 'Summary copied.'
	String get brief_copied => 'Summary copied.';

	/// en: 'This conversation is still short — open the messages, the summary would be longer than the thread.'
	String get summary_too_short => 'This conversation is still short — open the messages, the summary would be longer than the thread.';
}

// Path: settings
class Translations$settings$en {
	Translations$settings$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Settings'
	String get title => 'Settings';

	/// en: 'Account'
	String get account => 'Account';

	/// en: 'Subscription'
	String get subscription => 'Subscription';

	/// en: 'Team'
	String get team => 'Team';

	/// en: 'Manage Team'
	String get manage_team => 'Manage Team';

	/// en: 'Invite members and manage roles'
	String get manage_team_subtitle => 'Invite members and manage roles';

	/// en: 'AI Agents'
	String get ai_agents => 'AI Agents';

	/// en: 'New'
	String get kNew => 'New';

	/// en: 'No agents configured'
	String get no_agents => 'No agents configured';

	/// en: 'No agent selected'
	String get no_agent_selected => 'No agent selected';

	/// en: 'Failed to load team'
	String get error_loading_team => 'Failed to load team';

	/// en: 'Failed to load tags'
	String get error_loading_tags => 'Failed to load tags';

	/// en: 'Failed to load canned responses'
	String get error_loading_responses => 'Failed to load canned responses';

	/// en: 'Failed to load calendars'
	String get error_loading_calendars => 'Failed to load calendars';

	/// en: 'Failed to load templates'
	String get error_loading_templates => 'Failed to load templates';

	/// en: 'Failed to load'
	String get error_loading_webhooks => 'Failed to load';

	/// en: 'Agent not found'
	String get agent_not_found => 'Agent not found';

	/// en: 'Create Agent'
	String get create_agent => 'Create Agent';

	/// en: 'Active'
	String get active => 'Active';

	/// en: 'Inactive'
	String get inactive => 'Inactive';

	/// en: 'Error loading agents'
	String get error_loading_agents => 'Error loading agents';

	/// en: 'Try again'
	String get retry => 'Try again';

	/// en: 'Log out'
	String get logout => 'Log out';

	/// en: 'Plan & Usage'
	String get plan_and_usage => 'Plan & Usage';

	/// en: 'Manage your plan and track usage'
	String get manage_plan => 'Manage your plan and track usage';

	/// en: 'Delete Agent'
	String get delete_agent_title => 'Delete Agent';

	/// en: 'Are you sure you want to delete "$name"? This action cannot be undone.'
	String delete_agent_confirm({required Object name}) => 'Are you sure you want to delete "${name}"? This action cannot be undone.';

	/// en: 'Cancel'
	String get cancel => 'Cancel';

	/// en: 'Delete'
	String get delete => 'Delete';

	/// en: 'Agent deleted successfully'
	String get agent_deleted => 'Agent deleted successfully';

	/// en: 'New Agent'
	String get new_agent => 'New Agent';

	/// en: 'Agent name'
	String get agent_name => 'Agent name';

	/// en: 'AI Provider'
	String get ai_provider => 'AI Provider';

	/// en: 'Create'
	String get create => 'Create';

	/// en: 'Agent created successfully!'
	String get agent_created => 'Agent created successfully!';

	/// en: 'Delete agent'
	String get delete_agent_tooltip => 'Delete agent';

	/// en: 'No channels connected'
	String get no_channels => 'No channels connected';

	/// en: 'No messages'
	String get no_messages => 'No messages';

	/// en: 'typing...'
	String get typing => 'typing...';

	/// en: 'Company name, workspace and danger zone'
	String get account_subtitle => 'Company name, workspace and danger zone';

	/// en: 'Company'
	String get company_section_title => 'Company';

	/// en: 'Company name'
	String get company_name_label => 'Company name';

	/// en: 'How the AI agent introduces itself and how your workspace appears to your team.'
	String get company_name_helper => 'How the AI agent introduces itself and how your workspace appears to your team.';

	/// en: 'Company name updated'
	String get company_save_success => 'Company name updated';

	/// en: 'Save'
	String get company_save_cta => 'Save';

	/// en: 'Branding'
	String get branding_section_title => 'Branding';

	/// en: 'Logo and timezone used by the agent.'
	String get branding_section_subtitle => 'Logo and timezone used by the agent.';

	/// en: 'Workspace logo'
	String get branding_logo_label => 'Workspace logo';

	/// en: 'PNG or JPG, up to 2 MB. Shown in the header and future customer channels.'
	String get branding_logo_helper => 'PNG or JPG, up to 2 MB. Shown in the header and future customer channels.';

	/// en: 'Upload logo'
	String get branding_logo_upload_cta => 'Upload logo';

	/// en: 'Replace'
	String get branding_logo_change_cta => 'Replace';

	/// en: 'Remove'
	String get branding_logo_remove_cta => 'Remove';

	/// en: 'Uploading logo...'
	String get branding_logo_uploading => 'Uploading logo...';

	/// en: 'Logo larger than 2 MB'
	String get branding_logo_too_large => 'Logo larger than 2 MB';

	/// en: 'Failed to upload logo'
	String get branding_logo_upload_failed => 'Failed to upload logo';

	/// en: 'Default timezone'
	String get branding_timezone_label => 'Default timezone';

	/// en: 'Default when creating new agents. Existing agents keep their current timezone.'
	String get branding_timezone_helper => 'Default when creating new agents. Existing agents keep their current timezone.';

	/// en: 'Save branding'
	String get branding_save_cta => 'Save branding';

	/// en: 'Branding updated'
	String get branding_save_success => 'Branding updated';

	/// en: 'Danger zone'
	String get danger_zone_title => 'Danger zone';

	/// en: 'Irreversible actions on the workspace.'
	String get danger_zone_subtitle => 'Irreversible actions on the workspace.';

	/// en: 'Delete account permanently'
	String get delete_account_cta => 'Delete account permanently';

	/// en: 'Delete $company?'
	String delete_account_dialog_title({required Object company}) => 'Delete ${company}?';

	/// en: 'This permanently removes every agent, channel, conversation and message. It cannot be undone. Type the company name below to confirm.'
	String get delete_account_dialog_body => 'This permanently removes every agent, channel, conversation and message. It cannot be undone. Type the company name below to confirm.';

	/// en: 'Type "$company" to unlock'
	String delete_account_type_hint({required Object company}) => 'Type "${company}" to unlock';

	/// en: 'Delete workspace'
	String get delete_account_confirm_button => 'Delete workspace';

	/// en: 'Account deleted.'
	String get delete_account_success => 'Account deleted.';

	/// en: 'Manage the AI agents that reply to customers'
	String get agents_subtitle => 'Manage the AI agents that reply to customers';

	/// en: 'E-mail for offers · WhatsApp for the relationship'
	String get channels_subtitle => 'E-mail for offers · WhatsApp for the relationship';

	/// en: 'Organize conversations by theme or priority'
	String get tags_subtitle => 'Organize conversations by theme or priority';

	/// en: 'Quick replies attendants can reuse'
	String get canned_responses_subtitle => 'Quick replies attendants can reuse';

	/// en: 'Outbound templates (WhatsApp)'
	String get campaigns_subtitle => 'Outbound templates (WhatsApp)';

	/// en: 'LGPD and audit log'
	String get compliance_subtitle => 'LGPD and audit log';

	/// en: 'SMTP test'
	String get smtp_test_title => 'SMTP test';

	/// en: 'Sends an email to you to confirm the server can deliver messages.'
	String get smtp_test_subtitle => 'Sends an email to you to confirm the server can deliver messages.';

	/// en: 'Send test email'
	String get smtp_test_cta => 'Send test email';

	/// en: 'Test email sent. Check your inbox.'
	String get smtp_test_success => 'Test email sent. Check your inbox.';

	/// en: 'SMTP is not configured on the server.'
	String get smtp_test_not_configured => 'SMTP is not configured on the server.';

	/// en: 'Failed to send test email'
	String get smtp_test_failed => 'Failed to send test email';

	/// en: '1 template under Meta review'
	String get wa_templates_pending_banner_one => '1 template under Meta review';

	/// en: '$count templates under Meta review'
	String wa_templates_pending_banner_other({required Object count}) => '${count} templates under Meta review';

	/// en: 'Refreshing every 30 seconds.'
	String get wa_templates_polling_hint => 'Refreshing every 30 seconds.';

	/// en: 'Numbers under WABA'
	String get wa_phone_numbers_title => 'Numbers under WABA';

	/// en: 'All phone numbers registered under your WhatsApp Business Account. Each number can become its own Replider channel.'
	String get wa_phone_numbers_subtitle => 'All phone numbers registered under your WhatsApp Business Account. Each number can become its own Replider channel.';

	/// en: 'No other numbers found in this WABA.'
	String get wa_phone_numbers_empty => 'No other numbers found in this WABA.';

	/// en: 'Failed to list numbers'
	String get wa_phone_numbers_failed => 'Failed to list numbers';

	/// en: 'Currently connected'
	String get wa_phone_numbers_current => 'Currently connected';

	/// en: 'Action (exact)'
	String get audit_filter_action => 'Action (exact)';

	/// en: 'Entity type'
	String get audit_filter_entity => 'Entity type';

	/// en: 'Actor ID'
	String get audit_filter_actor => 'Actor ID';

	/// en: 'Apply'
	String get audit_filter_apply => 'Apply';

	/// en: 'Clear'
	String get audit_filter_clear => 'Clear';

	/// en: 'Export CSV'
	String get audit_export_cta => 'Export CSV';

	/// en: 'CSV copied. Paste into a .csv file to open in Excel.'
	String get audit_export_copied => 'CSV copied. Paste into a .csv file to open in Excel.';

	/// en: 'Failed to export CSV'
	String get audit_export_failed => 'Failed to export CSV';

	/// en: 'Test connection'
	String get calendar_test_connection_cta => 'Test connection';

	/// en: 'Connected successfully'
	String get calendar_test_connection_ok => 'Connected successfully';

	/// en: 'Connection failed'
	String get calendar_test_connection_fail => 'Connection failed';

	/// en: 'Disconnect Google'
	String get calendar_disconnect_cta => 'Disconnect Google';

	/// en: 'Disconnect $name from Google?'
	String calendar_disconnect_confirm_title({required Object name}) => 'Disconnect ${name} from Google?';

	/// en: 'The calendar stays in Replider but the agent will no longer pull Google events. You can reconnect later by editing the calendar.'
	String get calendar_disconnect_confirm_body => 'The calendar stays in Replider but the agent will no longer pull Google events. You can reconnect later by editing the calendar.';

	/// en: 'Disconnect'
	String get calendar_disconnect_confirm_cta => 'Disconnect';

	/// en: 'Disconnected from Google.'
	String get calendar_disconnect_success => 'Disconnected from Google.';

	/// en: 'Google Calendar'
	String get gcal_title => 'Google Calendar';

	/// en: 'Connect your Google account so the agent can check free slots and create events directly on your calendar.'
	String get gcal_connect_desc => 'Connect your Google account so the agent can check free slots and create events directly on your calendar.';

	/// en: 'Connect Google Calendar'
	String get gcal_connect_cta => 'Connect Google Calendar';

	/// en: 'Connected as $email'
	String gcal_connected_as({required Object email}) => 'Connected as ${email}';

	/// en: 'Disconnect'
	String get gcal_disconnect_cta => 'Disconnect';

	/// en: 'Disconnect Google Calendar?'
	String get gcal_disconnect_confirm_title => 'Disconnect Google Calendar?';

	/// en: 'The agent loses access to your calendar. Bookings fall back to the calendar shared with the platform.'
	String get gcal_disconnect_confirm_body => 'The agent loses access to your calendar. Bookings fall back to the calendar shared with the platform.';

	/// en: 'Finish connecting in the browser tab we opened. Come back here when you're done.'
	String get gcal_open_browser => 'Finish connecting in the browser tab we opened. Come back here when you\'re done.';

	/// en: 'Google Calendar connected!'
	String get gcal_connected_snackbar => 'Google Calendar connected!';

	/// en: 'Failed to connect Google Calendar. Try again.'
	String get gcal_error_snackbar => 'Failed to connect Google Calendar. Try again.';

	/// en: 'Google connection cancelled.'
	String get gcal_denied_snackbar => 'Google connection cancelled.';

	/// en: 'Google calendar'
	String get gcal_pick_calendar_label => 'Google calendar';

	/// en: 'Choose which calendar appointments are booked on'
	String get gcal_pick_calendar_hint => 'Choose which calendar appointments are booked on';

	/// en: 'Enter ID manually'
	String get gcal_manual_id_toggle => 'Enter ID manually';

	/// en: 'Today'
	String get team_metrics_period_today => 'Today';

	/// en: '7d'
	String get team_metrics_period_7d => '7d';

	/// en: '30d'
	String get team_metrics_period_30d => '30d';

	/// en: 'MTD'
	String get team_metrics_period_mtd => 'MTD';

	/// en: 'Convs'
	String get team_metrics_col_convs => 'Convs';

	/// en: 'TTFR'
	String get team_metrics_col_tmpr => 'TTFR';

	/// en: 'CSAT'
	String get team_metrics_col_csat => 'CSAT';

	/// en: 'Online'
	String get team_metrics_status_online => 'Online';

	/// en: 'Offline'
	String get team_metrics_status_offline => 'Offline';

	/// en: 'Export CSV'
	String get team_metrics_export_cta => 'Export CSV';

	/// en: 'CSV copied. Paste into a .csv file to open in Excel.'
	String get team_metrics_export_copied => 'CSV copied. Paste into a .csv file to open in Excel.';

	/// en: 'Failed to export'
	String get team_metrics_export_failed => 'Failed to export';

	/// en: '$name's metrics'
	String team_metrics_drill_title({required Object name}) => '${name}\'s metrics';

	/// en: 'Volume'
	String get team_metrics_card_volume => 'Volume';

	/// en: 'Speed'
	String get team_metrics_card_speed => 'Speed';

	/// en: 'Quality'
	String get team_metrics_card_quality => 'Quality';

	/// en: 'Activity'
	String get team_metrics_card_activity => 'Activity';

	/// en: 'Conversations assigned'
	String get team_metrics_conversations_assigned => 'Conversations assigned';

	/// en: 'Resolved'
	String get team_metrics_conversations_resolved => 'Resolved';

	/// en: 'Messages sent'
	String get team_metrics_messages_sent => 'Messages sent';

	/// en: 'Avg first response time'
	String get team_metrics_avg_first_response => 'Avg first response time';

	/// en: 'Avg resolution time'
	String get team_metrics_avg_resolution => 'Avg resolution time';

	/// en: 'CSAT avg'
	String get team_metrics_csat_avg => 'CSAT avg';

	/// en: 'Responses'
	String get team_metrics_csat_count => 'Responses';

	/// en: 'Last activity'
	String get team_metrics_last_seen => 'Last activity';

	/// en: 'No data for the selected period.'
	String get team_metrics_empty => 'No data for the selected period.';

	/// en: '—'
	String get team_metrics_no_data => '—';

	/// en: 'Online'
	String get live_supervision_online => 'Online';

	/// en: 'Offline'
	String get live_supervision_offline => 'Offline';

	/// en: 'E-mail & channels'
	String get channels_title => 'E-mail & channels';

	/// en: 'Business hours'
	String get business_hours_title => 'Business hours';

	/// en: 'Only reply during business hours'
	String get business_hours_enabled => 'Only reply during business hours';

	/// en: 'Outside the window the AI stays quiet — attendants can still reply manually.'
	String get business_hours_enabled_subtitle => 'Outside the window the AI stays quiet — attendants can still reply manually.';

	/// en: 'Start'
	String get business_hours_start => 'Start';

	/// en: 'End'
	String get business_hours_end => 'End';

	/// en: 'Days of the week'
	String get business_hours_days => 'Days of the week';

	/// en: 'Business hours saved'
	String get business_hours_save_success => 'Business hours saved';

	/// en: 'Back'
	String get back_tooltip => 'Back';

	/// en: 'Your contract is inactive — sending and edits are paused. Talk to us to reactivate.'
	String get plan_expired_banner => 'Your contract is inactive — sending and edits are paused. Talk to us to reactivate.';

	/// en: 'View plans'
	String get view_plans => 'View plans';

	/// en: '$count offline'
	String channels_count_offline({required Object count}) => '${count} offline';

	/// en: 'Channels'
	String get section_channels => 'Channels';

	/// en: 'Support'
	String get section_attendance => 'Support';

	/// en: 'Compliance'
	String get section_compliance => 'Compliance';

	/// en: 'Campaigns'
	String get campaigns_title => 'Campaigns';

	/// en: 'WhatsApp Flows'
	String get flows_title => 'WhatsApp Flows';

	/// en: 'Native forms for lead capture and bookings.'
	String get flows_subtitle => 'Native forms for lead capture and bookings.';

	/// en: 'Webhooks'
	String get webhooks_title => 'Webhooks';

	/// en: 'Connect Zapier, n8n, your CRM or your own backend.'
	String get webhooks_subtitle => 'Connect Zapier, n8n, your CRM or your own backend.';

	/// en: 'LGPD & Audit'
	String get compliance_title => 'LGPD & Audit';

	/// en: 'Log out'
	String get logout_title => 'Log out';

	/// en: 'End this session'
	String get logout_subtitle => 'End this session';

	/// en: 'Log out of account'
	String get logout_confirm_title => 'Log out of account';

	/// en: 'Are you sure? You'll need to sign in again to access the dashboard.'
	String get logout_confirm_body => 'Are you sure? You\'ll need to sign in again to access the dashboard.';

	/// en: 'Trial expired'
	String get trial_expired_title => 'Trial expired';

	/// en: 'Plan inactive'
	String get plan_inactive_title => 'Plan inactive';

	/// en: 'View plans'
	String get plan_view_plans => 'View plans';

	/// en: 'Plan: $name'
	String plan_label({required Object name}) => 'Plan: ${name}';

	/// en: 'Tokens: $tokens% | Conversations: $conv%'
	String tokens_conversations_usage({required Object tokens, required Object conv}) => 'Tokens: ${tokens}% | Conversations: ${conv}%';

	/// en: 'e.g. Sales Assistant'
	String get agent_name_placeholder => 'e.g. Sales Assistant';

	/// en: 'Provider'
	String get ai_provider_label => 'Provider';

	/// en: 'Agent Active'
	String get agent_active_title => 'Agent Active';

	/// en: 'The agent is replying to messages'
	String get agent_active_subtitle => 'The agent is replying to messages';

	/// en: 'The agent is paused'
	String get agent_paused_subtitle => 'The agent is paused';

	/// en: 'Test Agent'
	String get agent_test_title => 'Test Agent';

	/// en: 'Start a test conversation with this agent'
	String get agent_test_subtitle => 'Start a test conversation with this agent';

	/// en: 'Calendars'
	String get agent_calendars_title => 'Calendars';

	/// en: 'Google Calendar for bookings'
	String get agent_calendars_subtitle => 'Google Calendar for bookings';

	/// en: 'Knowledge Base'
	String get agent_kb_title => 'Knowledge Base';

	/// en: 'Texts and PDFs for the AI'
	String get agent_kb_subtitle => 'Texts and PDFs for the AI';

	/// en: 'AGENT NAME'
	String get agent_section_name => 'AGENT NAME';

	/// en: 'ARTIFICIAL INTELLIGENCE'
	String get agent_section_ai => 'ARTIFICIAL INTELLIGENCE';

	/// en: 'SYSTEM PROMPT'
	String get agent_section_prompt => 'SYSTEM PROMPT';

	/// en: 'LEARNED PERSONALITY'
	String get agent_section_personality => 'LEARNED PERSONALITY';

	/// en: 'BUSINESS HOURS'
	String get agent_section_business_hours => 'BUSINESS HOURS';

	/// en: 'BEHAVIOR'
	String get agent_section_behavior => 'BEHAVIOR';

	/// en: 'ADDITIONAL INFO'
	String get agent_section_info => 'ADDITIONAL INFO';

	/// en: 'Define the personality and instructions of the AI agent.'
	String get agent_prompt_helper_text => 'Define the personality and instructions of the AI agent.';

	/// en: 'Using the saved prompt: $name'
	String agent_prompt_library_match({required Object name}) => 'Using the saved prompt: ${name}';

	/// en: 'Open library'
	String get agent_prompt_library_open => 'Open library';

	/// en: 'You are a customer service assistant for company X. Your goal is to help customers with questions about products and orders...'
	String get agent_prompt_hint => 'You are a customer service assistant for company X. Your goal is to help customers with questions about products and orders...';

	/// en: 'Save'
	String get agent_save => 'Save';

	/// en: 'Saved'
	String get agent_saved => 'Saved';

	/// en: 'Upgrade plan'
	String get upgrade_plan_action => 'Upgrade plan';

	/// en: 'Agent not found'
	String get agent_not_found_message => 'Agent not found';

	/// en: 'Agent updated successfully!'
	String get agent_updated_success => 'Agent updated successfully!';

	/// en: 'No personality learned yet. Reply to a few conversations manually and the bot will start mimicking your team's style.'
	String get personality_empty_hint => 'No personality learned yet. Reply to a few conversations manually and the bot will start mimicking your team\'s style.';

	/// en: 'Personality learned from the most recent human replies. Applied automatically to the bot.'
	String get personality_active_hint => 'Personality learned from the most recent human replies. Applied automatically to the bot.';

	/// en: 'Personality updated.'
	String get personality_updated => 'Personality updated.';

	/// en: 'Limit auto-replies to a schedule'
	String get bh_limit_label => 'Limit auto-replies to a schedule';

	/// en: 'The bot only replies inside this schedule. Outside the window the conversation goes to a human.'
	String get bh_explainer => 'The bot only replies inside this schedule. Outside the window the conversation goes to a human.';

	/// en: 'Start'
	String get bh_start => 'Start';

	/// en: 'End'
	String get bh_end => 'End';

	/// en: '--:--'
	String get bh_time_placeholder => '--:--';

	/// en: 'S'
	String get bh_dom_d => 'S';

	/// en: 'S'
	String get bh_d_sun => 'S';

	/// en: 'M'
	String get bh_d_mon => 'M';

	/// en: 'T'
	String get bh_d_tue => 'T';

	/// en: 'W'
	String get bh_d_wed => 'W';

	/// en: 'T'
	String get bh_d_thu => 'T';

	/// en: 'F'
	String get bh_d_fri => 'F';

	/// en: 'S'
	String get bh_d_sat => 'S';

	/// en: 'Human style (doesn't announce it's AI)'
	String get behavior_hide_ai_title => 'Human style (doesn\'t announce it\'s AI)';

	/// en: 'Bot writes like a human attendant and never volunteers that it's an AI. If the customer asks directly, it answers honestly — required by WhatsApp (Meta) policy.'
	String get behavior_hide_ai_subtitle => 'Bot writes like a human attendant and never volunteers that it\'s an AI. If the customer asks directly, it answers honestly — required by WhatsApp (Meta) policy.';

	/// en: 'Wait for human when unsure'
	String get behavior_silent_title => 'Wait for human when unsure';

	/// en: 'When the bot doesn't know the answer, it stays silent and hands the conversation to a human without telling the customer. Turn off so the bot says "I'll check" in the customer's language.'
	String get behavior_silent_subtitle => 'When the bot doesn\'t know the answer, it stays silent and hands the conversation to a human without telling the customer. Turn off so the bot says "I\'ll check" in the customer\'s language.';

	/// en: 'ID'
	String get agent_id_label => 'ID';

	/// en: 'Timezone'
	String get agent_timezone_label => 'Timezone';

	/// en: 'Created on'
	String get agent_created_label => 'Created on';

	/// en: 'Back'
	String get channels_back_tooltip => 'Back';

	/// en: 'Channels'
	String get channels_screen_title => 'Channels';

	/// en: 'Add channel'
	String get channels_add_tooltip => 'Add channel';

	/// en: 'Connect the email you already use so offers go out, and WhatsApp for the relationship.'
	String get channels_empty_subtitle => 'Connect the email you already use so offers go out, and WhatsApp for the relationship.';

	/// en: 'Edit channel'
	String get channel_dialog_edit_title => 'Edit channel';

	/// en: 'Add channel'
	String get channel_dialog_add_title => 'Add channel';

	/// en: 'Update name, credentials or assigned agent.'
	String get channel_dialog_edit_subtitle => 'Update name, credentials or assigned agent.';

	/// en: 'Where your offers go out and the replies come in.'
	String get channel_dialog_add_subtitle => 'Where your offers go out and the replies come in.';

	/// en: 'Channel name'
	String get channel_name_label => 'Channel name';

	/// en: 'e.g. Sales email'
	String get channel_name_hint => 'e.g. Sales email';

	/// en: 'or connect manually'
	String get channel_or_manual => 'or connect manually';

	/// en: 'Responsible agent'
	String get channel_responsible_agent => 'Responsible agent';

	/// en: 'None (human only)'
	String get channel_no_agent_human => 'None (human only)';

	/// en: 'Save'
	String get channel_save_btn => 'Save';

	/// en: 'Connect'
	String get channel_connect_btn => 'Connect';

	/// en: 'Fill in every field to rotate credentials, or leave them all blank to keep the current ones.'
	String get channel_partial_credentials => 'Fill in every field to rotate credentials, or leave them all blank to keep the current ones.';

	/// en: 'Invalid Phone Number ID (must be 12-16 numeric digits).'
	String get channel_invalid_phone_id => 'Invalid Phone Number ID (must be 12-16 numeric digits).';

	/// en: 'Invalid WABA ID (must be 12-18 numeric digits).'
	String get channel_invalid_waba_id => 'Invalid WABA ID (must be 12-18 numeric digits).';

	/// en: 'Access Token too short. Paste the full Meta token.'
	String get channel_invalid_token => 'Access Token too short. Paste the full Meta token.';

	/// en: 'Invalid Account SID (must start with AC followed by 32 hex characters).'
	String get channel_invalid_account_sid => 'Invalid Account SID (must start with AC followed by 32 hex characters).';

	/// en: 'Invalid phone number. Use E.164 format, e.g. +15551234567.'
	String get channel_invalid_from_number => 'Invalid phone number. Use E.164 format, e.g. +15551234567.';

	/// en: 'Channel updated successfully!'
	String get channel_updated_success => 'Channel updated successfully!';

	/// en: 'Channel created successfully!'
	String get channel_created_success => 'Channel created successfully!';

	/// en: 'Channel deleted'
	String get channel_deleted_success => 'Channel deleted';

	/// en: 'Delete "$name"? Messages received on this channel will stop.'
	String channel_delete_confirm({required Object name}) => 'Delete "${name}"? Messages received on this channel will stop.';

	/// en: 'Connected'
	String get channel_status_connected => 'Connected';

	/// en: 'Disabled'
	String get channel_status_disabled => 'Disabled';

	/// en: 'Incomplete'
	String get channel_status_incomplete => 'Incomplete';

	/// en: 'Edit'
	String get channel_edit_tooltip => 'Edit';

	/// en: 'Test Connection'
	String get channel_test_btn => 'Test Connection';

	/// en: 'Testing...'
	String get channel_testing => 'Testing...';

	/// en: 'Configure'
	String get channel_configure_btn => 'Configure';

	/// en: 'Snippet'
	String get channel_snippet_btn => 'Snippet';

	/// en: 'Connected! $label'
	String channel_test_success_named({required Object label}) => 'Connected! ${label}';

	/// en: 'Connection OK!'
	String get channel_test_success => 'Connection OK!';

	/// en: 'Failed: $error'
	String channel_test_failed({required Object error}) => 'Failed: ${error}';

	/// en: 'Unknown error'
	String get channel_test_unknown_error => 'Unknown error';

	/// en: 'Reconnect'
	String get channel_reconnect_action => 'Reconnect';

	/// en: 'Connected through Google: $address'
	String channel_google_connected({required Object address}) => 'Connected through Google: ${address}';

	/// en: 'This mailbox was connected by consent — there is no server or password to edit. If sending or reading stops, connect it again.'
	String get channel_google_edit_hint => 'This mailbox was connected by consent — there is no server or password to edit. If sending or reading stops, connect it again.';

	/// en: 'Reconnect with Google'
	String get channel_google_reconnect => 'Reconnect with Google';

	/// en: 'Reading replies has failed $count time(s): $error'
	String channel_imap_failing({required Object count, required Object error}) => 'Reading replies has failed ${count} time(s): ${error}';

	/// en: 'Webhook re-registered.'
	String get channel_webhook_re_registered => 'Webhook re-registered.';

	/// en: 'Failed to re-register webhook: $error'
	String channel_webhook_register_failed({required Object error}) => 'Failed to re-register webhook: ${error}';

	/// en: '2 min'
	String get channels_select_telegram_badge => '2 min';

	/// en: 'Popular'
	String get channels_select_popular_badge => 'Popular';

	/// en: 'Coming soon'
	String get channels_select_coming_soon => 'Coming soon';

	/// en: 'How to get your Page ID and access token'
	String get channel_instagram_help_link => 'How to get your Page ID and access token';

	/// en: 'How to get a compliant texting number (Twilio guide)'
	String get channel_sms_help_link => 'How to get a compliant texting number (Twilio guide)';

	/// en: 'Webhook URL (Twilio)'
	String get channel_webhook_url_label => 'Webhook URL (Twilio)';

	/// en: 'Paste this URL as the "A message comes in" webhook for your number in the Twilio console.'
	String get channel_webhook_url_help => 'Paste this URL as the "A message comes in" webhook for your number in the Twilio console.';

	/// en: 'Webhook URL copied'
	String get channel_webhook_url_copied => 'Webhook URL copied';

	/// en: 'Webhook URL'
	String get channel_webhook_url_btn => 'Webhook URL';

	/// en: 'Back'
	String get wa_settings_back_tooltip => 'Back';

	/// en: 'WhatsApp Settings'
	String get wa_settings_title => 'WhatsApp Settings';

	/// en: 'Profile'
	String get wa_settings_tab_profile => 'Profile';

	/// en: 'Templates'
	String get wa_settings_tab_templates => 'Templates';

	/// en: 'DIAGNOSTICS'
	String get wa_section_diagnostics => 'DIAGNOSTICS';

	/// en: 'NUMBERS IN WABA'
	String get wa_section_phones => 'NUMBERS IN WABA';

	/// en: 'ABOUT THE BUSINESS'
	String get wa_section_business => 'ABOUT THE BUSINESS';

	/// en: 'About'
	String get wa_field_about => 'About';

	/// en: 'Short description (shown on your profile)'
	String get wa_field_about_hint => 'Short description (shown on your profile)';

	/// en: 'Full description'
	String get wa_field_description => 'Full description';

	/// en: 'Detailed description of your business'
	String get wa_field_description_hint => 'Detailed description of your business';

	/// en: 'Email'
	String get wa_field_email => 'Email';

	/// en: 'contact@company.com'
	String get wa_field_email_hint => 'contact@company.com';

	/// en: 'Address'
	String get wa_field_address => 'Address';

	/// en: '123 Main St — City/State'
	String get wa_field_address_hint => '123 Main St — City/State';

	/// en: 'Site 1'
	String get wa_field_site_one => 'Site 1';

	/// en: 'https://...'
	String get wa_field_site_one_hint => 'https://...';

	/// en: 'Site 2'
	String get wa_field_site_two => 'Site 2';

	/// en: 'https://... (optional)'
	String get wa_field_site_two_hint => 'https://... (optional)';

	/// en: 'These details show publicly on your WhatsApp Business profile.'
	String get wa_profile_helper => 'These details show publicly on your WhatsApp Business profile.';

	/// en: 'Save changes'
	String get wa_profile_save_btn => 'Save changes';

	/// en: 'Saving...'
	String get wa_profile_saving => 'Saving...';

	/// en: 'Profile updated successfully!'
	String get wa_profile_saved_success => 'Profile updated successfully!';

	/// en: 'Replace photo'
	String get wa_picture_replace => 'Replace photo';

	/// en: 'Change photo'
	String get wa_picture_set => 'Change photo';

	/// en: 'The photo will be applied when you click "Save changes".'
	String get wa_picture_apply_hint => 'The photo will be applied when you click "Save changes".';

	/// en: 'Remove cached photo'
	String get wa_picture_remove_button => 'Remove cached photo';

	/// en: 'Remove cached photo?'
	String get wa_picture_remove_dialog_title => 'Remove cached photo?';

	/// en: 'Drops the local copy of the profile picture. The photo on the WhatsApp number itself stays — to remove it there, use Meta Business Manager. Use this once Meta is no longer returning the picture so the dashboard stops serving the old one.'
	String get wa_picture_remove_dialog_body => 'Drops the local copy of the profile picture. The photo on the WhatsApp number itself stays — to remove it there, use Meta Business Manager. Use this once Meta is no longer returning the picture so the dashboard stops serving the old one.';

	/// en: 'Remove'
	String get wa_picture_remove_dialog_confirm => 'Remove';

	/// en: 'Cached photo removed.'
	String get wa_picture_removed_success => 'Cached photo removed.';

	/// en: 'Templates are pre-approved messages from Meta. You need them to start conversations with customers outside the 24h window.'
	String get wa_templates_no_templates_subtitle => 'Templates are pre-approved messages from Meta. You need them to start conversations with customers outside the 24h window.';

	/// en: 'Connection with Meta needs to be renewed'
	String get wa_templates_create_loading_error_title => 'Connection with Meta needs to be renewed';

	/// en: 'Preview'
	String get wa_templates_visualize_tooltip => 'Preview';

	/// en: 'Delete'
	String get wa_templates_delete_tooltip => 'Delete';

	/// en: 'Delete "$name"? This also removes the template on Meta.'
	String wa_templates_delete_confirm({required Object name}) => 'Delete "${name}"? This also removes the template on Meta.';

	/// en: 'Template deleted'
	String get wa_templates_delete_success => 'Template deleted';

	/// en: 'Reason: $reason'
	String wa_templates_rejected_reason({required Object reason}) => 'Reason: ${reason}';

	/// en: 'No examples saved — variables show as placeholders.'
	String get wa_templates_no_examples_warning => 'No examples saved — variables show as placeholders.';

	/// en: 'New template'
	String get wa_create_dialog_title => 'New template';

	/// en: '1. TECHNICAL NAME'
	String get wa_create_section_name => '1. TECHNICAL NAME';

	/// en: 'e.g. welcome_message'
	String get wa_create_name_hint => 'e.g. welcome_message';

	/// en: 'Lowercase letters, numbers and underscore only. Customer never sees it.'
	String get wa_create_name_helper => 'Lowercase letters, numbers and underscore only. Customer never sees it.';

	/// en: '2. CATEGORY'
	String get wa_create_section_category => '2. CATEGORY';

	/// en: '3. LANGUAGE'
	String get wa_create_section_language => '3. LANGUAGE';

	/// en: 'HEADER (optional)'
	String get wa_create_section_header => 'HEADER (optional)';

	/// en: '4. MESSAGE'
	String get wa_create_section_message => '4. MESSAGE';

	/// en: 'Hello {{1}}, your order #{{2}} has been confirmed! 🎉'
	String get wa_create_message_hint => 'Hello {{1}}, your order #{{2}} has been confirmed! 🎉';

	/// en: 'Use {{1}}, {{2}}... for variables. Formatting: *bold*, _italic_, ~strike~, `mono` (WhatsApp uses 1 asterisk, not 2).'
	String get wa_create_message_helper => 'Use {{1}}, {{2}}... for variables. Formatting: *bold*, _italic_, ~strike~, `mono` (WhatsApp uses 1 asterisk, not 2).';

	/// en: 'WhatsApp uses *text* (one asterisk), not **text**. Fix before sending.'
	String get wa_create_double_asterisk_warn => 'WhatsApp uses *text* (one asterisk), not **text**. Fix before sending.';

	/// en: '5. EXAMPLES FOR VARIABLES'
	String get wa_create_section_examples => '5. EXAMPLES FOR VARIABLES';

	/// en: 'Example for {{$n}}'
	String wa_create_example_for({required Object n}) => 'Example for {{${n}}}';

	/// en: 'John Smith'
	String get wa_create_example_first => 'John Smith';

	/// en: 'Value'
	String get wa_create_example_other => 'Value';

	/// en: 'Meta uses these examples to review the template.'
	String get wa_create_examples_helper => 'Meta uses these examples to review the template.';

	/// en: 'FOOTER (optional)'
	String get wa_create_section_footer => 'FOOTER (optional)';

	/// en: 'e.g. Reply STOP to opt out'
	String get wa_create_footer_hint => 'e.g. Reply STOP to opt out';

	/// en: 'BUTTONS (optional, max 3)'
	String get wa_create_section_buttons => 'BUTTONS (optional, max 3)';

	/// en: 'PREVIEW'
	String get wa_create_section_preview => 'PREVIEW';

	/// en: 'Submit for review'
	String get wa_create_submit => 'Submit for review';

	/// en: 'Submitting...'
	String get wa_create_submitting => 'Submitting...';

	/// en: 'Name must contain only lowercase letters, numbers and _.'
	String get wa_create_validation_name => 'Name must contain only lowercase letters, numbers and _.';

	/// en: 'Message is required.'
	String get wa_create_validation_body_required => 'Message is required.';

	/// en: 'Remove the ** from the text — WhatsApp uses *text* (one asterisk) for bold.'
	String get wa_create_validation_double_asterisk => 'Remove the ** from the text — WhatsApp uses *text* (one asterisk) for bold.';

	/// en: 'Text cannot start with a variable. Add a word before {{1}} (e.g. "Hello {{1}}").'
	String get wa_create_validation_no_leading_var => 'Text cannot start with a variable. Add a word before {{1}} (e.g. "Hello {{1}}").';

	/// en: 'Text cannot end with a variable. Add a word or punctuation after the last {{N}}.'
	String get wa_create_validation_no_trailing_var => 'Text cannot end with a variable. Add a word or punctuation after the last {{N}}.';

	/// en: 'Fill in the examples for the variables.'
	String get wa_create_validation_examples_required => 'Fill in the examples for the variables.';

	/// en: 'Text header cannot be empty.'
	String get wa_create_validation_header_empty => 'Text header cannot be empty.';

	/// en: 'Text header exceeds 60 characters.'
	String get wa_create_validation_header_too_long => 'Text header exceeds 60 characters.';

	/// en: 'Header accepts at most one variable.'
	String get wa_create_validation_header_one_var => 'Header accepts at most one variable.';

	/// en: 'Fill in the header example.'
	String get wa_create_validation_header_example => 'Fill in the header example.';

	/// en: 'Wait for the header upload to finish.'
	String get wa_create_validation_media_uploading => 'Wait for the header upload to finish.';

	/// en: 'Pick and upload a file for the header.'
	String get wa_create_validation_media_required => 'Pick and upload a file for the header.';

	/// en: 'Every button needs text.'
	String get wa_create_validation_button_text_required => 'Every button needs text.';

	/// en: 'URL buttons need a link.'
	String get wa_create_validation_button_url => 'URL buttons need a link.';

	/// en: 'Phone buttons need an E.164 number (e.g. +14155551212).'
	String get wa_create_validation_button_phone => 'Phone buttons need an E.164 number (e.g. +14155551212).';

	/// en: 'Text looks promotional ("news", "offer"...). Meta usually rejects UTILITY with this content — switch to MARKETING.'
	String get wa_create_warning_promo_for_utility => 'Text looks promotional ("news", "offer"...). Meta usually rejects UTILITY with this content — switch to MARKETING.';

	/// en: 'Text looks transactional ("confirmation", "order"...). Consider UTILITY to reduce the chance of rejection.'
	String get wa_create_warning_tx_for_marketing => 'Text looks transactional ("confirmation", "order"...). Consider UTILITY to reduce the chance of rejection.';

	/// en: 'Promotions and offers'
	String get wa_category_marketing => 'Promotions and offers';

	/// en: 'Confirmations and updates'
	String get wa_category_utility => 'Confirmations and updates';

	/// en: 'Portuguese (Brazil)'
	String get wa_lang_pt_br => 'Portuguese (Brazil)';

	/// en: 'English (US)'
	String get wa_lang_en_us => 'English (US)';

	/// en: 'Spanish'
	String get wa_lang_es => 'Spanish';

	/// en: 'No header'
	String get wa_header_none => 'No header';

	/// en: 'Text'
	String get wa_header_text => 'Text';

	/// en: 'Image'
	String get wa_header_image => 'Image';

	/// en: 'Video'
	String get wa_header_video => 'Video';

	/// en: 'Document'
	String get wa_header_document => 'Document';

	/// en: 'e.g. Order confirmation {{1}}'
	String get wa_header_text_hint => 'e.g. Order confirmation {{1}}';

	/// en: 'Max 60 characters. Allows up to one variable {{1}}.'
	String get wa_header_text_helper => 'Max 60 characters. Allows up to one variable {{1}}.';

	/// en: 'Example for {{1}} in the header'
	String get wa_header_example_label => 'Example for {{1}} in the header';

	/// en: '#12345'
	String get wa_header_example_hint => '#12345';

	/// en: 'Pick file'
	String get wa_header_pick_file => 'Pick file';

	/// en: 'Uploading...'
	String get wa_header_uploading => 'Uploading...';

	/// en: 'Remove'
	String get wa_header_remove_tooltip => 'Remove';

	/// en: 'Failed to process the file on Meta. Try another.'
	String get wa_header_upload_failed_processing => 'Failed to process the file on Meta. Try another.';

	/// en: 'Upload failed: $error'
	String wa_header_upload_failed_generic({required Object error}) => 'Upload failed: ${error}';

	/// en: 'Button text (max 25)'
	String get wa_button_text_label => 'Button text (max 25)';

	/// en: 'URL'
	String get wa_button_url_label => 'URL';

	/// en: 'https://...'
	String get wa_button_url_hint => 'https://...';

	/// en: 'Number (E.164)'
	String get wa_button_phone_label => 'Number (E.164)';

	/// en: '+14155551212'
	String get wa_button_phone_hint => '+14155551212';

	/// en: 'Call number'
	String get wa_button_call => 'Call number';

	/// en: 'Your message appears here...'
	String get wa_preview_message_placeholder => 'Your message appears here...';

	/// en: 'Button text'
	String get wa_preview_button_placeholder => 'Button text';

	/// en: 'Template submitted! Meta review usually takes up to 24 hours.'
	String get wa_template_submit_success => 'Template submitted! Meta review usually takes up to 24 hours.';

	/// en: 'Sends a real message to validate the connection end-to-end. Useful for the Meta App Review screencast.'
	String get wa_test_template_explain => 'Sends a real message to validate the connection end-to-end. Useful for the Meta App Review screencast.';

	/// en: 'Meta has not confirmed the number registration. Inbound messages won't reach Replider.'
	String get wa_diag_phone_not_registered_no_platform => 'Meta has not confirmed the number registration. Inbound messages won\'t reach Replider.';

	/// en: 'platform_type = $platform. Register the number to receive messages.'
	String wa_diag_phone_not_registered_with_platform({required Object platform}) => 'platform_type = ${platform}. Register the number to receive messages.';

	/// en: 'Locked'
	String get wa_diag_cooldown_title => 'Locked';

	/// en: 'Wait $time before the next attempt. Each click resets the Meta lock.'
	String wa_diag_cooldown_snackbar({required Object time}) => 'Wait ${time} before the next attempt. Each click resets the Meta lock.';

	/// en: 'Refresh'
	String get wa_diag_atualizar_tooltip => 'Refresh';

	/// en: 'Automatic CSAT'
	String get csat_auto_title => 'Automatic CSAT';

	/// en: 'Ask 1 to 5 after the conversation goes idle'
	String get csat_auto_subtitle => 'Ask 1 to 5 after the conversation goes idle';

	/// en: '$count min'
	String team_metrics_relative_min({required Object count}) => '${count} min';

	/// en: '$count h'
	String team_metrics_relative_hours({required Object count}) => '${count} h';

	/// en: '$count d'
	String team_metrics_relative_days({required Object count}) => '${count} d';

	/// en: '$count w'
	String team_metrics_relative_weeks({required Object count}) => '${count} w';

	/// en: 'Sent in the conversation's detected language (pt/en/es). Turn off in B2B flows where the agent already debriefs manually.'
	String get csat_auto_helper => 'Sent in the conversation\'s detected language (pt/en/es). Turn off in B2B flows where the agent already debriefs manually.';

	/// en: 'Reply reminder'
	String get reply_reminder_title => 'Reply reminder';

	/// en: 'Never forget a waiting customer'
	String get reply_reminder_subtitle => 'Never forget a waiting customer';

	/// en: 'When the AI is paused and a customer is waiting longer than the threshold, every team member with a registered device gets a push + dashboard alert. Escalates at 1x, 2x, and 4x the threshold, then stops.'
	String get reply_reminder_helper => 'When the AI is paused and a customer is waiting longer than the threshold, every team member with a registered device gets a push + dashboard alert. Escalates at 1x, 2x, and 4x the threshold, then stops.';

	/// en: 'Notify after'
	String get reply_reminder_threshold_label => 'Notify after';

	/// en: 'Claude (AI)'
	String get claude_title => 'Claude (AI)';

	/// en: 'Control your account by chatting with Claude (MCP).'
	String get claude_subtitle => 'Control your account by chatting with Claude (MCP).';

	/// en: 'Company identity'
	String get brand_tile => 'Company identity';

	/// en: 'Voice, what may and may never be said'
	String get brand_tile_sub => 'Voice, what may and may never be said';

	/// en: 'Email templates'
	String get templates_tile => 'Email templates';

	/// en: 'Ready-made email texts, with how well each one replies'
	String get templates_tile_sub => 'Ready-made email texts, with how well each one replies';

	/// en: 'Blocked addresses'
	String get suppressions_tile => 'Blocked addresses';

	/// en: 'Who stopped receiving your email, and why'
	String get suppressions_tile_sub => 'Who stopped receiving your email, and why';

	/// en: 'When approaches may go out without your approval'
	String get autopilot_tile_sub => 'When approaches may go out without your approval';

	/// en: 'Tokens: $tokens | Conversations: $conv'
	String tokens_conversations_used({required Object tokens, required Object conv}) => 'Tokens: ${tokens} | Conversations: ${conv}';

	/// en: 'Everyone who has talked to you, in one place'
	String get contacts_subtitle => 'Everyone who has talked to you, in one place';

	/// en: 'Volume, response time and AI performance'
	String get analytics_subtitle => 'Volume, response time and AI performance';

	/// en: 'The agent would hand this conversation to a person. In a real conversation the bot is paused and someone takes over — in the test there is nobody to hand off to.'
	String get preview_handoff_note => 'The agent would hand this conversation to a person. In a real conversation the bot is paused and someone takes over — in the test there is nobody to hand off to.';

	/// en: 'Test sent to $to. Check that inbox — no customer was contacted. You can now approve up to $cap approaches a day.'
	String channel_test_email_success({required Object to, required Object cap}) => 'Test sent to ${to}. Check that inbox — no customer was contacted. You can now approve up to ${cap} approaches a day.';

	/// en: 'Test sent to $to, but we could not record the release. Run the test again.'
	String channel_test_email_not_unlocked({required Object to}) => 'Test sent to ${to}, but we could not record the release. Run the test again.';

	/// en: 'Company and brand'
	String get section_company => 'Company and brand';

	/// en: 'Email and sending'
	String get section_email => 'Email and sending';

	/// en: 'Catalogue and funnels'
	String get section_catalog => 'Catalogue and funnels';

	/// en: 'Documents'
	String get section_documents => 'Documents';

	/// en: 'Integrations'
	String get section_integrations => 'Integrations';

	/// en: 'Subscribe or grow'
	String get section_contract => 'Subscribe or grow';

	/// en: 'Automated support'
	String get section_support_bot => 'Automated support';

	/// en: 'Stages, the data to collect and the webhook'
	String get funnels_tile_sub => 'Stages, the data to collect and the webhook';

	/// en: 'Advanced'
	String get section_advanced => 'Advanced';

	/// en: 'Funnels and relationship dates. Already set up for you; open only if you want to change them.'
	String get section_advanced_sub => 'Funnels and relationship dates. Already set up for you; open only if you want to change them.';

	/// en: 'Birthdays, anniversaries and sector days that turn into a draft on the day'
	String get relationship_tile_sub => 'Birthdays, anniversaries and sector days that turn into a draft on the day';

	/// en: 'Agents, prompts, flows, tags and the bot's own metrics'
	String get support_bot_tile_sub => 'Agents, prompts, flows, tags and the bot\'s own metrics';

	/// en: 'The chatbot side of Replider. If you are here to sell, none of this is needed — the selling tools are on the previous screen.'
	String get support_bot_intro => 'The chatbot side of Replider. If you are here to sell, none of this is needed — the selling tools are on the previous screen.';

	/// en: 'Warm-up not started — verify the mailbox (DNS or reconnect) to enable cold outreach'
	String get email_warmup_not_started => 'Warm-up not started — verify the mailbox (DNS or reconnect) to enable cold outreach';

	/// en: 'Warm-up: day $day of $total · $sent of $cap sends today'
	String email_warmup_progress({required Object day, required Object total, required Object sent, required Object cap}) => 'Warm-up: day ${day} of ${total} · ${sent} of ${cap} sends today';

	/// en: 'Warmed up — up to $cap sends per day'
	String email_warmup_done({required Object cap}) => 'Warmed up — up to ${cap} sends per day';

	/// en: 'Give the channel a name'
	String get channel_name_required => 'Give the channel a name';

	/// en: 'Fill in: $fields'
	String channel_required_missing({required Object fields}) => 'Fill in: ${fields}';

	/// en: 'How to set up your domain's e-mail'
	String get channel_email_help_link => 'How to set up your domain\'s e-mail';

	/// en: 'Usage & AI costs'
	String get usage_costs_title => 'Usage & AI costs';

	/// en: 'Monthly usage, AI spend and your budget ceiling'
	String get usage_costs_subtitle => 'Monthly usage, AI spend and your budget ceiling';
}

// Path: common
class Translations$common$en {
	Translations$common$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Cancel'
	String get cancel => 'Cancel';

	/// en: 'Save'
	String get save => 'Save';

	/// en: 'Create'
	String get create => 'Create';

	/// en: 'Delete'
	String get delete => 'Delete';

	/// en: 'Edit'
	String get edit => 'Edit';

	/// en: 'Try again'
	String get retry => 'Try again';

	/// en: 'Subscribe'
	String get subscribe => 'Subscribe';

	/// en: '/mo'
	String get per_month => '/mo';

	/// en: 'Free'
	String get free => 'Free';

	/// en: 'Success'
	String get success => 'Success';

	/// en: 'Are you sure you want to delete this?'
	String get confirm_delete => 'Are you sure you want to delete this?';

	/// en: 'No results found.'
	String get no_results => 'No results found.';

	/// en: 'Confirm'
	String get confirm => 'Confirm';

	/// en: 'Remove'
	String get remove => 'Remove';

	/// en: 'Close'
	String get close => 'Close';

	/// en: 'Add'
	String get add => 'Add';

	/// en: 'Search'
	String get search => 'Search';

	/// en: 'Copy'
	String get copy => 'Copy';

	/// en: 'Change'
	String get change => 'Change';

	/// en: 'Load more'
	String get load_more => 'Load more';

	/// en: 'Error: $error'
	String error_generic({required Object error}) => 'Error: ${error}';

	/// en: 'Back'
	String get back => 'Back';

	/// en: 'Saving...'
	String get saving => 'Saving...';

	/// en: 'Sending...'
	String get sending => 'Sending...';

	/// en: 'Sent'
	String get sent => 'Sent';

	/// en: 'Loading...'
	String get loading => 'Loading...';

	/// en: 'OK'
	String get ok => 'OK';

	/// en: 'Open'
	String get open => 'Open';

	/// en: 'Send'
	String get send => 'Send';

	/// en: 'Discard'
	String get discard => 'Discard';

	/// en: 'Keep editing'
	String get keep_editing => 'Keep editing';

	/// en: 'Discard changes?'
	String get discard_changes_title => 'Discard changes?';

	/// en: 'You have unsaved changes. Leaving now will discard them.'
	String get discard_changes_body => 'You have unsaved changes. Leaving now will discard them.';

	/// en: 'Yes'
	String get yes => 'Yes';

	/// en: 'No'
	String get no => 'No';

	/// en: 'Log out'
	String get logout => 'Log out';

	/// en: 'Refresh'
	String get refresh => 'Refresh';

	/// en: 'Select'
	String get select => 'Select';

	/// en: 'Name'
	String get name_label => 'Name';

	/// en: 'Color'
	String get color => 'Color';

	/// en: 'New'
	String get new_short => 'New';

	/// en: 'Unreadable content'
	String get unreadable_content => 'Unreadable content';

	/// en: 'Couldn't load. Tap to try again.'
	String get load_failed_tap => 'Couldn\'t load. Tap to try again.';
}

// Path: billing
class Translations$billing$en {
	Translations$billing$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Usage & AI costs'
	String get title => 'Usage & AI costs';

	/// en: 'Failed to start checkout.'
	String get checkout_error => 'Failed to start checkout.';

	/// en: 'Manage payment'
	String get manage_payment_title => 'Manage payment';

	/// en: 'Open the Stripe portal to update your card, download invoices, or cancel.'
	String get manage_payment_subtitle => 'Open the Stripe portal to update your card, download invoices, or cancel.';

	/// en: 'Open portal'
	String get manage_payment_cta => 'Open portal';

	/// en: 'Opening portal...'
	String get portal_opening => 'Opening portal...';

	/// en: 'Pick your plan'
	String get upgrade_modal_title => 'Pick your plan';

	/// en: 'Subscribe directly from here. Secure payment via Stripe.'
	String get upgrade_modal_subtitle => 'Subscribe directly from here. Secure payment via Stripe.';

	/// en: 'Current plan'
	String get upgrade_current_plan_badge => 'Current plan';

	/// en: 'Most popular'
	String get upgrade_popular_badge => 'Most popular';

	/// en: 'Subscribe'
	String get upgrade_cta => 'Subscribe';

	/// en: 'Contact sales'
	String get upgrade_cta_contact_sales => 'Contact sales';

	/// en: 'Sold under contract'
	String get contract_badge => 'Sold under contract';

	/// en: 'Custom limits + bring your own LLM API key. Talk to a specialist for pricing and setup.'
	String get contract_subtitle => 'Custom limits + bring your own LLM API key. Talk to a specialist for pricing and setup.';

	/// en: 'Email'
	String get contract_email_cta => 'Email';

	/// en: 'WhatsApp'
	String get contract_whatsapp_cta => 'WhatsApp';

	/// en: 'Replider $plan plan — interest'
	String contract_email_subject({required Object plan}) => 'Replider ${plan} plan — interest';

	/// en: 'Couldn't open the link. Email us at support@replider.com.'
	String get contract_link_failed => 'Couldn\'t open the link. Email us at support@replider.com.';

	/// en: '/mo'
	String get upgrade_per_month => '/mo';

	/// en: 'Monthly'
	String get upgrade_interval_monthly => 'Monthly';

	/// en: 'Annual'
	String get upgrade_interval_annual => 'Annual';

	/// en: 'Save 20%'
	String get upgrade_annual_save => 'Save 20%';

	/// en: 'billed annually'
	String get upgrade_billed_annually => 'billed annually';

	/// en: 'Current'
	String get upgrade_compare_current => 'Current';

	/// en: 'New'
	String get upgrade_compare_new => 'New';

	/// en: 'Conversations/mo'
	String get upgrade_compare_conversations => 'Conversations/mo';

	/// en: 'AI agents'
	String get upgrade_compare_agents => 'AI agents';

	/// en: 'Channels'
	String get upgrade_compare_channels => 'Channels';

	/// en: 'AI tokens/mo'
	String get upgrade_compare_tokens => 'AI tokens/mo';

	/// en: 'Opening checkout...'
	String get upgrade_checkout_opening => 'Opening checkout...';

	/// en: '$count AI agent(s)'
	String upgrade_feature_agents({required Object count}) => '${count} AI agent(s)';

	/// en: '$count channel(s)'
	String upgrade_feature_channels({required Object count}) => '${count} channel(s)';

	/// en: '$count conversations/mo'
	String upgrade_feature_conversations({required Object count}) => '${count} conversations/mo';

	/// en: '$tokens AI tokens/mo'
	String upgrade_feature_tokens({required Object tokens}) => '${tokens} AI tokens/mo';

	/// en: 'Trial: $days days'
	String billing_pill_trial_days({required Object days}) => 'Trial: ${days} days';

	/// en: 'Trial ends today'
	String get billing_pill_trial_last_day => 'Trial ends today';

	/// en: 'Trial expired'
	String get billing_pill_trial_expired => 'Trial expired';

	/// en: 'Payment pending'
	String get billing_pill_past_due => 'Payment pending';

	/// en: 'Manage'
	String get billing_pill_manage => 'Manage';

	/// en: 'Complete your first subscription before opening the billing portal.'
	String get no_customer_yet => 'Complete your first subscription before opening the billing portal.';

	/// en: 'Failed to start checkout.'
	String get checkout_open_failed => 'Failed to start checkout.';

	/// en: 'Failed to purchase the pack.'
	String get addon_buy_failed => 'Failed to purchase the pack.';

	/// en: 'Failed to open the billing portal.'
	String get portal_open_failed => 'Failed to open the billing portal.';

	/// en: 'Conversations'
	String get quota_conversations_label => 'Conversations';

	/// en: 'AI credits'
	String get quota_tokens_label => 'AI credits';

	/// en: 'AI agents'
	String get quota_agents_label => 'AI agents';

	/// en: 'AI limit reached'
	String get quota_wall_title => 'AI limit reached';

	/// en: 'You've used all your AI credits for this cycle. The agent stopped replying automatically until you add more credits or the cycle resets.'
	String get quota_wall_body => 'You\'ve used all your AI credits for this cycle. The agent stopped replying automatically until you add more credits or the cycle resets.';

	/// en: 'Wait for monthly reset'
	String get quota_wall_cta_later => 'Wait for monthly reset';

	/// en: 'Cycle resets on day $day of next month.'
	String quota_wall_reset_hint({required Object day}) => 'Cycle resets on day ${day} of next month.';

	/// en: 'Back'
	String get back_tooltip => 'Back';

	/// en: 'CURRENT USAGE'
	String get section_usage => 'CURRENT USAGE';

	/// en: 'ADDON PACKS'
	String get section_addon_packs => 'ADDON PACKS';

	/// en: 'AVAILABLE PLANS'
	String get section_plans => 'AVAILABLE PLANS';

	/// en: 'Addon packs unavailable.'
	String get addon_packs_unavailable => 'Addon packs unavailable.';

	/// en: 'Failed to load usage'
	String get load_usage_failed => 'Failed to load usage';

	/// en: 'Failed to load plans'
	String get load_plans_failed => 'Failed to load plans';

	/// en: 'View plans'
	String get trial_view_plans => 'View plans';

	/// en: 'AI tokens'
	String get usage_ai_tokens => 'AI tokens';

	/// en: 'Includes $count extra tokens'
	String usage_includes_addon_tokens({required Object count}) => 'Includes ${count} extra tokens';

	/// en: 'Input'
	String get usage_input => 'Input';

	/// en: 'Output'
	String get usage_output => 'Output';

	/// en: 'Conversations'
	String get usage_conversations => 'Conversations';

	/// en: 'Paying for extra conversations'
	String get overage_active_title => 'Paying for extra conversations';

	/// en: '$count conversations beyond your plan · $amount billed this cycle'
	String overage_active_body({required Object count, required Object amount}) => '${count} conversations beyond your plan · ${amount} billed this cycle';

	/// en: '$rate per extra conversation'
	String overage_rate_note({required Object rate}) => '${rate} per extra conversation';

	/// en: 'Overage cap reached — extra conversations are paused until your cycle resets.'
	String get overage_cap_reached => 'Overage cap reached — extra conversations are paused until your cycle resets.';

	/// en: 'Free'
	String get plan_free => 'Free';

	/// en: '/mo'
	String get plan_per_month_short => '/mo';

	/// en: '$count AI agent(s)'
	String plan_feature_agents({required Object count}) => '${count} AI agent(s)';

	/// en: '$count support channel(s)'
	String plan_feature_channels({required Object count}) => '${count} support channel(s)';

	/// en: '$count conversations/mo'
	String plan_feature_conversations({required Object count}) => '${count} conversations/mo';

	/// en: '$tokens AI tokens/mo'
	String plan_feature_tokens({required Object tokens}) => '${tokens} AI tokens/mo';

	/// en: '$mb MB storage'
	String plan_feature_storage({required Object mb}) => '${mb} MB storage';

	/// en: '$gb GB of storage'
	String plan_feature_storage_gb({required Object gb}) => '${gb} GB of storage';

	/// en: '$count attendant seat(s)'
	String plan_feature_seats({required Object count}) => '${count} attendant seat(s)';

	/// en: '$count broadcast messages/mo'
	String plan_feature_broadcast({required Object count}) => '${count} broadcast messages/mo';

	/// en: '$count recipients per campaign'
	String plan_feature_recipients({required Object count}) => '${count} recipients per campaign';

	/// en: 'Extra conversations: $cents¢ each (capped at $cap× plan price)'
	String plan_feature_overage({required Object cents, required Object cap}) => 'Extra conversations: ${cents}¢ each (capped at ${cap}× plan price)';

	/// en: 'Your $plan contract limits'
	String effective_limits_title({required Object plan}) => 'Your ${plan} contract limits';

	/// en: 'Resolved from your contract overrides — billed off-platform.'
	String get effective_limits_subtitle => 'Resolved from your contract overrides — billed off-platform.';

	/// en: 'Bring your own key'
	String get byok_label => 'Bring your own key';

	/// en: 'Configured: $providers'
	String byok_configured({required Object providers}) => 'Configured: ${providers}';

	/// en: 'Not configured — the agent won't reply until you set a key.'
	String get byok_pending => 'Not configured — the agent won\'t reply until you set a key.';

	/// en: 'Manage'
	String get byok_manage_cta => 'Manage';

	/// en: 'Cancel subscription'
	String get cancel_subscription => 'Cancel subscription';

	/// en: 'Custom'
	String get plan_contract => 'Custom';

	/// en: '$count leads/month'
	String plan_feature_leads({required Object count}) => '${count} leads/month';

	/// en: '$count company lookups/month'
	String plan_feature_enrichments({required Object count}) => '${count} company lookups/month';

	/// en: '$count AI drafts/month'
	String plan_feature_drafts({required Object count}) => '${count} AI drafts/month';

	/// en: '$count funnel(s)'
	String plan_feature_funnels({required Object count}) => '${count} funnel(s)';

	/// en: '$count mailbox(es)'
	String plan_feature_mailboxes({required Object count}) => '${count} mailbox(es)';

	/// en: 'YOUR MONTH'
	String get usage_sdr_title => 'YOUR MONTH';

	/// en: 'Leads in the pipeline'
	String get usage_sdr_leads => 'Leads in the pipeline';

	/// en: 'Company lookups'
	String get usage_sdr_enrichments => 'Company lookups';

	/// en: 'Approaches sent'
	String get usage_sdr_approaches => 'Approaches sent';

	/// en: 'Meetings booked'
	String get usage_sdr_meetings => 'Meetings booked';

	/// en: '$used of $limit'
	String usage_sdr_of_limit({required Object used, required Object limit}) => '${used} of ${limit}';

	/// en: '$used this month'
	String usage_sdr_no_limit({required Object used}) => '${used} this month';

	/// en: 'Want to subscribe or grow?'
	String get talk_title => 'Want to subscribe or grow?';

	/// en: 'We build the proposal in the conversation, sized to your operation. No price list: every company sells differently.'
	String get talk_body => 'We build the proposal in the conversation, sized to your operation. No price list: every company sells differently.';

	/// en: 'Book a call'
	String get talk_book_cta => 'Book a call';

	/// en: 'I want to know Replider'
	String get talk_email_subject => 'I want to know Replider';

	/// en: 'AI cost (your own key)'
	String get ai_cost_title => 'AI cost (your own key)';

	/// en: 'This cycle'
	String get ai_cost_cycle => 'This cycle';

	/// en: 'Monthly pace'
	String get ai_cost_projection => 'Monthly pace';

	/// en: 'Your ceiling'
	String get ai_cost_budget => 'Your ceiling';

	/// en: 'none set'
	String get ai_cost_no_budget => 'none set';

	/// en: 'Set monthly ceiling'
	String get ai_cost_set_budget => 'Set monthly ceiling';

	/// en: 'Monthly AI ceiling'
	String get ai_cost_budget_dialog_title => 'Monthly AI ceiling';

	/// en: 'Hard stop: once spend reaches this, AI generation pauses until the cycle resets. 0 removes the ceiling.'
	String get ai_cost_budget_help => 'Hard stop: once spend reaches this, AI generation pauses until the cycle resets. 0 removes the ceiling.';
}

// Path: team
class Translations$team$en {
	Translations$team$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Team'
	String get title => 'Team';

	/// en: 'Invite Member'
	String get invite_member => 'Invite Member';

	/// en: 'Member invited successfully.'
	String get member_invited => 'Member invited successfully.';

	/// en: 'Member removed.'
	String get member_removed => 'Member removed.';

	/// en: 'Role updated.'
	String get role_updated => 'Role updated.';

	/// en: 'Change role'
	String get change_role => 'Change role';

	/// en: 'Remove member'
	String get remove_member => 'Remove member';

	/// en: 'Inactive'
	String get inactive => 'Inactive';

	/// en: 'Pending'
	String get pending_badge => 'Pending';

	/// en: 'Resend invite'
	String get resend_invite => 'Resend invite';

	/// en: 'Invite resent.'
	String get invite_resent => 'Invite resent.';

	/// en: 'Active now'
	String get active_now => 'Active now';

	/// en: 'Last seen $when ago'
	String last_seen_ago({required Object when}) => 'Last seen ${when} ago';

	/// en: 'Invite'
	String get invite_short => 'Invite';

	/// en: 'No team members yet'
	String get empty_subtitle => 'No team members yet';

	/// en: 'Role'
	String get role_label => 'Role';

	/// en: 'Admin'
	String get role_admin => 'Admin';

	/// en: 'Attendant'
	String get role_attendant => 'Attendant';

	/// en: 'Change Role'
	String get change_role_title => 'Change Role';

	/// en: 'Change "$name"'s role to $role?'
	String change_role_body({required Object name, required Object role}) => 'Change "${name}"\'s role to ${role}?';

	/// en: 'Remove Member'
	String get remove_title => 'Remove Member';

	/// en: 'Really remove "$name" from the team? This person will lose access to the system.'
	String remove_body({required Object name}) => 'Really remove "${name}" from the team? This person will lose access to the system.';

	/// en: '$count min'
	String ago_minutes({required Object count}) => '${count} min';

	/// en: '${count}h'
	String ago_hours({required Object count}) => '${count}h';

	/// en: '$count days'
	String ago_days({required Object count}) => '${count} days';

	/// en: '$count months'
	String ago_months({required Object count}) => '${count} months';

	/// en: 'Owner'
	String get role_owner => 'Owner';

	/// en: 'Manager'
	String get role_gestor => 'Manager';

	/// en: 'Sales rep'
	String get role_vendedor => 'Sales rep';

	/// en: 'Works their own leads and offers; no general inbox'
	String get role_vendedor_hint => 'Works their own leads and offers; no general inbox';

	/// en: 'Oversees the team and the pipeline; no billing or integrations'
	String get role_gestor_hint => 'Oversees the team and the pipeline; no billing or integrations';

	/// en: 'Answers inbox conversations; no pipeline access'
	String get role_attendant_hint => 'Answers inbox conversations; no pipeline access';

	/// en: 'Everything: settings, team, channels and billing'
	String get role_admin_hint => 'Everything: settings, team, channels and billing';

	/// en: 'Fill in name and e-mail'
	String get invite_fields_required => 'Fill in name and e-mail';

	/// en: 'Invalid e-mail — check the address'
	String get invite_email_invalid => 'Invalid e-mail — check the address';
}

// Path: channels
class Translations$channels$en {
	Translations$channels$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Channels'
	String get title => 'Channels';

	/// en: 'Connect WhatsApp, Instagram, or Telegram to receive customer messages.'
	String get connect_description => 'Connect WhatsApp, Instagram, or Telegram to receive customer messages.';

	/// en: 'Channel created successfully.'
	String get channel_created => 'Channel created successfully.';

	/// en: 'Channel deleted.'
	String get channel_deleted => 'Channel deleted.';

	/// en: 'Webhook down'
	String get webhook_down => 'Webhook down';

	/// en: 'Test connection'
	String get test_connection => 'Test connection';

	/// en: 'Delete channel'
	String get delete_channel => 'Delete channel';

	/// en: 'Site (Webchat)'
	String get type_webchat_label => 'Site (Webchat)';

	/// en: 'Enter the Phone Number ID of your number on the Meta for Developers panel. The access token is managed automatically by Replider.'
	String get setup_whatsapp => 'Enter the Phone Number ID of your number on the Meta for Developers panel. The access token is managed automatically by Replider.';

	/// en: 'Create a bot with @BotFather on Telegram and paste the token here. The webhook is configured automatically.'
	String get setup_telegram => 'Create a bot with @BotFather on Telegram and paste the token here. The webhook is configured automatically.';

	/// en: 'Connect your Instagram professional account (Business or Creator). Enter the account ID and an access token with the instagram_business_manage_messages permission. Direct messages only.'
	String get setup_instagram => 'Connect your Instagram professional account (Business or Creator). Enter the account ID and an access token with the instagram_business_manage_messages permission. Direct messages only.';

	/// en: 'Live chat for your website. After creating, copy the snippet and paste it in your site's HTML.'
	String get setup_webchat => 'Live chat for your website. After creating, copy the snippet and paste it in your site\'s HTML.';

	/// en: 'Connect your own Twilio number for two-way SMS conversations. Paste the Account SID, Auth Token, and phone number (E.164) from the Twilio console. After connecting, copy the channel's webhook URL and set it as the "A message comes in" webhook for your number in Twilio.'
	String get setup_sms => 'Connect your own Twilio number for two-way SMS conversations. Paste the Account SID, Auth Token, and phone number (E.164) from the Twilio console. After connecting, copy the channel\'s webhook URL and set it as the "A message comes in" webhook for your number in Twilio.';

	/// en: 'Build configuration incomplete. Set WHATSAPP_APP_ID and WHATSAPP_EMBEDDED_CONFIG_ID via --dart-define.'
	String get embedded_signup_missing_config => 'Build configuration incomplete. Set WHATSAPP_APP_ID and WHATSAPP_EMBEDDED_CONFIG_ID via --dart-define.';

	/// en: 'Unknown signup failure.'
	String get embedded_signup_unknown_failure => 'Unknown signup failure.';

	/// en: 'Failed to finish connection: $error'
	String embedded_signup_complete_failed({required Object error}) => 'Failed to finish connection: ${error}';

	/// en: 'WhatsApp connected successfully!'
	String get embedded_signup_success => 'WhatsApp connected successfully!';

	/// en: 'Connecting...'
	String get embedded_signup_connecting => 'Connecting...';

	/// en: 'Connect WhatsApp via Meta'
	String get embedded_signup_cta => 'Connect WhatsApp via Meta';

	/// en: 'WHATSAPP_APP_ID or WHATSAPP_EMBEDDED_CONFIG_ID not set in build.'
	String get embedded_signup_missing_env => 'WHATSAPP_APP_ID or WHATSAPP_EMBEDDED_CONFIG_ID not set in build.';

	/// en: 'No response from the Facebook SDK.'
	String get embedded_signup_no_response => 'No response from the Facebook SDK.';

	/// en: 'Login cancelled by the user.'
	String get embedded_signup_user_cancelled => 'Login cancelled by the user.';

	/// en: 'Facebook didn't return an authorisation code (status=$status).'
	String embedded_signup_no_code({required Object status}) => 'Facebook didn\'t return an authorisation code (status=${status}).';

	/// en: 'Embedded Signup failed: $error'
	String embedded_signup_failure({required Object error}) => 'Embedded Signup failed: ${error}';

	/// en: 'Send and receive email from your own domain. You need your provider’s SMTP details and SPF, DKIM and DMARC published in DNS — without them Gmail and Outlook drop the message silently.'
	String get setup_email => 'Send and receive email from your own domain. You need your provider’s SMTP details and SPF, DKIM and DMARC published in DNS — without them Gmail and Outlook drop the message silently.';

	/// en: 'Domain authenticated — ready to send.'
	String get email_dns_verified => 'Domain authenticated — ready to send.';

	/// en: 'The domain still needs authentication. Without it, your customers' providers (Gmail, Outlook and others) discard your messages.'
	String get email_dns_pending => 'The domain still needs authentication. Without it, your customers\' providers (Gmail, Outlook and others) discard your messages.';

	/// en: 'Check again'
	String get email_dns_recheck => 'Check again';

	/// en: 'optional'
	String get email_dns_optional => 'optional';

	/// en: 'Name'
	String get email_dns_host => 'Name';

	/// en: 'Value'
	String get email_dns_value => 'Value';

	/// en: 'Currently published'
	String get email_dns_found => 'Currently published';

	/// en: 'Copied.'
	String get email_dns_copied => 'Copied.';

	/// en: 'Google authenticates this address's sending — nothing to publish.'
	String get email_dns_provider_managed => 'Google authenticates this address\'s sending — nothing to publish.';

	/// en: 'Already published — nothing to do here.'
	String get email_dns_already_published => 'Already published — nothing to do here.';

	/// en: 'DNS at $provider'
	String email_dns_detected({required Object provider}) => 'DNS at ${provider}';

	/// en: 'Open guide'
	String get email_dns_open_guide => 'Open guide';

	/// en: 'Set up automatically'
	String get email_dns_auto_configure => 'Set up automatically';

	/// en: 'In $provider, create an API token with the Zone → DNS → Edit permission for this domain, and paste it here. We use it once to publish the records and never store it.'
	String email_dns_token_help({required Object provider}) => 'In ${provider}, create an API token with the Zone → DNS → Edit permission for this domain, and paste it here. We use it once to publish the records and never store it.';

	/// en: 'Paste the token'
	String get email_dns_token_hint => 'Paste the token';

	/// en: 'Records published'
	String get email_dns_auto_done => 'Records published';

	/// en: 'Partly published'
	String get email_dns_auto_partial => 'Partly published';

	/// en: 'created'
	String get dns_action_created => 'created';

	/// en: 'updated'
	String get dns_action_updated => 'updated';

	/// en: 'merged into your existing record'
	String get dns_action_merged => 'merged into your existing record';

	/// en: 'yours kept, untouched'
	String get dns_action_kept => 'yours kept, untouched';

	/// en: 'failed'
	String get dns_action_failed => 'failed';

	/// en: 'Need help?'
	String get email_dns_need_help => 'Need help?';

	/// en: 'not detected'
	String get email_dns_provider_unknown => 'not detected';

	/// en: 'I need help setting up my email domain. Domain: $domain DNS provider: $provider Missing records: $records'
	String email_dns_help_draft({required Object domain, required Object provider, required Object records}) => 'I need help setting up my email domain.\n\nDomain: ${domain}\nDNS provider: ${provider}\nMissing records: ${records}';

	/// en: 'Health not verified'
	String get webhook_health_unknown => 'Health not verified';

	/// en: 'Connect the account you already use'
	String get gmail_connect_title => 'Connect the account you already use';

	/// en: 'One click, no server or password to type. Works with Gmail and Google Workspace, and replies keep landing in the inbox you already read.'
	String get gmail_connect_help => 'One click, no server or password to type. Works with Gmail and Google Workspace, and replies keep landing in the inbox you already read.';

	/// en: 'Connect with Google'
	String get gmail_connect_cta => 'Connect with Google';

	/// en: 'Email connected.'
	String get gmail_connected => 'Email connected.';

	/// en: 'Email connection renewed.'
	String get gmail_reconnected => 'Email connection renewed.';

	/// en: 'Connection cancelled. Nothing changed.'
	String get gmail_denied => 'Connection cancelled. Nothing changed.';

	/// en: 'Could not finish the connection. Try again.'
	String get gmail_failed => 'Could not finish the connection. Try again.';

	/// en: 'or set up SMTP manually'
	String get gmail_or_smtp => 'or set up SMTP manually';

	/// en: 'Mailbox connected: $name'
	String gmail_already_connected({required Object name}) => 'Mailbox connected: ${name}';

	/// en: 'I authorized it — check'
	String get gmail_connect_check => 'I authorized it — check';

	/// en: '$provider API token'
	String email_dns_token_title_named({required Object provider}) => '${provider} API token';

	/// en: 'Sending e-mail'
	String get email_field_from => 'Sending e-mail';

	/// en: 'SMTP server'
	String get email_field_smtp_host => 'SMTP server';

	/// en: 'Port'
	String get email_field_smtp_port => 'Port';

	/// en: 'SMTP username'
	String get email_field_smtp_user => 'SMTP username';

	/// en: 'SMTP password'
	String get email_field_smtp_password => 'SMTP password';

	/// en: 'IMAP server (optional)'
	String get email_field_imap_host => 'IMAP server (optional)';

	/// en: 'same as SMTP if empty'
	String get email_field_imap_hint => 'same as SMTP if empty';

	/// en: 'IMAP port (optional)'
	String get email_field_imap_port => 'IMAP port (optional)';

	/// en: 'We could not auto-detect your DNS provider. No problem: open the panel of the company that manages the domain (whoever sold it or hosts the site), look for "DNS", "DNS zone" or "Manage records", and create the records below.'
	String get email_dns_generic_steps => 'We could not auto-detect your DNS provider. No problem: open the panel of the company that manages the domain (whoever sold it or hosts the site), look for "DNS", "DNS zone" or "Manage records", and create the records below.';

	/// en: 'Create each record below in the domain's DNS zone — the buttons copy name and value. After saving, propagation takes minutes to a few hours; this screen re-checks by itself.'
	String get email_dns_how_to => 'Create each record below in the domain\'s DNS zone — the buttons copy name and value. After saving, propagation takes minutes to a few hours; this screen re-checks by itself.';

	/// en: 'The Embedded Signup flow only works in the browser. On mobile, use the manual form under "Add channel".'
	String get fb_signup_mobile_only => 'The Embedded Signup flow only works in the browser. On mobile, use the manual form under "Add channel".';

	/// en: 'We stopped auto-checking — after ~10 minutes the usual cause is a wrong record, not propagation. Fix and tap refresh to check again.'
	String get email_dns_poll_stopped => 'We stopped auto-checking — after ~10 minutes the usual cause is a wrong record, not propagation. Fix and tap refresh to check again.';
}

// Path: whatsapp
class Translations$whatsapp$en {
	Translations$whatsapp$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Error loading profile'
	String get profile_load_error => 'Error loading profile';

	/// en: 'Segment'
	String get segment => 'Segment';

	/// en: 'New template'
	String get new_template => 'New template';

	/// en: 'Reconnect via Meta'
	String get reconnect_meta => 'Reconnect via Meta';

	/// en: 'Re-authorise the Meta connection to refresh the API token. Takes about a minute — you'll stay on this screen.'
	String get reconnect_meta_help => 'Re-authorise the Meta connection to refresh the API token. Takes about a minute — you\'ll stay on this screen.';

	/// en: 'Connection refreshed.'
	String get reconnect_success => 'Connection refreshed.';

	/// en: 'No templates yet'
	String get no_templates => 'No templates yet';

	/// en: 'Create first template'
	String get create_first_template => 'Create first template';

	/// en: 'Delete template'
	String get delete_template => 'Delete template';

	/// en: 'Add button'
	String get add_button => 'Add button';

	/// en: 'Quick reply'
	String get quick_reply => 'Quick reply';

	/// en: 'Open URL'
	String get open_url => 'Open URL';

	/// en: 'Template sent to $phone'
	String template_sent_success({required Object phone}) => 'Template sent to ${phone}';

	/// en: 'Send template'
	String get send_template_title => 'Send template';

	/// en: 'To $name (+$phone)'
	String send_template_to_with_name({required Object name, required Object phone}) => 'To ${name} (+${phone})';

	/// en: 'To +$phone'
	String send_template_to({required Object phone}) => 'To +${phone}';

	/// en: 'Approved template'
	String get approved_template_label => 'Approved template';

	/// en: 'Choose an approved template.'
	String get choose_approved_template => 'Choose an approved template.';

	/// en: 'No approved templates on this channel. Create one in the Templates tab and wait for Meta approval.'
	String get no_approved_templates_hint => 'No approved templates on this channel. Create one in the Templates tab and wait for Meta approval.';

	/// en: 'Failed to load templates: $error'
	String loading_templates_error({required Object error}) => 'Failed to load templates: ${error}';

	/// en: 'Sending...'
	String get sending => 'Sending...';

	/// en: 'Send'
	String get send => 'Send';

	/// en: 'Approved'
	String get template_status_approved => 'Approved';

	/// en: 'In review'
	String get template_status_pending => 'In review';

	/// en: 'Rejected'
	String get template_status_rejected => 'Rejected';

	/// en: 'Paused'
	String get template_status_paused => 'Paused';

	/// en: 'Disabled'
	String get template_status_disabled => 'Disabled';

	/// en: 'Authentication'
	String get category_authentication => 'Authentication';

	/// en: 'Portuguese (BR)'
	String get language_pt_br => 'Portuguese (BR)';

	/// en: 'English (US)'
	String get language_en_us => 'English (US)';

	/// en: 'Spanish'
	String get language_es => 'Spanish';

	/// en: 'The WhatsApp token expired. Reconnect via Meta to restore access.'
	String get error_token_expired => 'The WhatsApp token expired. Reconnect via Meta to restore access.';

	/// en: '24h window expired. Use an approved template to reopen the conversation.'
	String get error_outside_window => '24h window expired. Use an approved template to reopen the conversation.';

	/// en: 'Meta denied the permission. Check that the app has whatsapp_business_management enabled.'
	String get error_missing_permission => 'Meta denied the permission. Check that the app has whatsapp_business_management enabled.';

	/// en: 'Invalid or unregistered WhatsApp recipient: $message'
	String error_recipient({required Object message}) => 'Invalid or unregistered WhatsApp recipient: ${message}';

	/// en: 'Template error: $message'
	String error_template({required Object message}) => 'Template error: ${message}';

	/// en: 'This channel has no WABA ID. Reconnect via Meta Embedded Signup.'
	String get error_missing_waba => 'This channel has no WABA ID. Reconnect via Meta Embedded Signup.';

	/// en: 'This channel has no access token. Reconnect via Meta Embedded Signup.'
	String get error_missing_token => 'This channel has no access token. Reconnect via Meta Embedded Signup.';

	/// en: 'Meta blocked register attempts on this number due to too many calls. Wait ~24h before trying again. Meanwhile, check that the WABA has a payment method and that two-step verification is off.'
	String get error_register_rate_limited => 'Meta blocked register attempts on this number due to too many calls. Wait ~24h before trying again. Meanwhile, check that the WABA has a payment method and that two-step verification is off.';

	/// en: 'Wrong PIN. Use the "Forgot PIN" tab to receive a code via SMS.'
	String get error_pin_mismatch => 'Wrong PIN. Use the "Forgot PIN" tab to receive a code via SMS.';

	/// en: 'This number is already verified on another Cloud API app. Use "Release number" to free it before registering here.'
	String get error_already_verified => 'This number is already verified on another Cloud API app. Use "Release number" to free it before registering here.';

	/// en: 'Meta blocked the registration because two-step verification (2FA) is on for the WABA. Turn it off in Meta Business → Account settings → Two-step verification and try again.'
	String get error_cert_2fa_blocked => 'Meta blocked the registration because two-step verification (2FA) is on for the WABA. Turn it off in Meta Business → Account settings → Two-step verification and try again.';

	/// en: 'Meta hasn't verified ownership of this number yet. Switch to the "Forgot PIN" tab to receive a code via SMS or voice and finish registration in one go.'
	String get error_ownership_not_verified => 'Meta hasn\'t verified ownership of this number yet. Switch to the "Forgot PIN" tab to receive a code via SMS or voice and finish registration in one go.';

	/// en: 'Wrong, expired or already-used code. Click "Send code" again to receive a fresh one (Meta SMS codes only last ~10 minutes).'
	String get error_verify_code_invalid => 'Wrong, expired or already-used code. Click "Send code" again to receive a fresh one (Meta SMS codes only last ~10 minutes).';

	/// en: 'Couldn't register the number on Cloud API. $detail'
	String error_register_failed({required Object detail}) => 'Couldn\'t register the number on Cloud API. ${detail}';

	/// en: 'Failed to release the number. $detail'
	String error_deregister_failed({required Object detail}) => 'Failed to release the number. ${detail}';

	/// en: 'Meta couldn't send the verification code. $detail Try again in 1 minute, or switch between SMS and voice.'
	String error_request_code_failed({required Object detail}) => 'Meta couldn\'t send the verification code. ${detail} Try again in 1 minute, or switch between SMS and voice.';

	/// en: 'Invalid or expired code. Request a new one and try again.'
	String get error_verify_code_failed => 'Invalid or expired code. Request a new one and try again.';

	/// en: 'Couldn't subscribe Replider to receive webhooks. $detail'
	String error_subscribe_failed({required Object detail}) => 'Couldn\'t subscribe Replider to receive webhooks. ${detail}';

	/// en: 'Couldn't check the webhook status. $detail'
	String error_webhook_status_failed({required Object detail}) => 'Couldn\'t check the webhook status. ${detail}';

	/// en: 'Meta refused the token exchange during connection. Try Embedded Signup again.'
	String get error_token_exchange_failed => 'Meta refused the token exchange during connection. Try Embedded Signup again.';

	/// en: 'Meta rejected the template creation. $detail'
	String error_template_create_failed({required Object detail}) => 'Meta rejected the template creation. ${detail}';

	/// en: 'Meta rejected the template deletion. $detail'
	String error_template_delete_failed({required Object detail}) => 'Meta rejected the template deletion. ${detail}';

	/// en: 'Couldn't load the WhatsApp Business profile. Try reloading.'
	String get error_profile_fetch_failed => 'Couldn\'t load the WhatsApp Business profile. Try reloading.';

	/// en: 'Couldn't save the WhatsApp Business profile. $detail'
	String error_profile_update_failed({required Object detail}) => 'Couldn\'t save the WhatsApp Business profile. ${detail}';

	/// en: 'Failed to upload the profile picture. Check that it's a JPG/PNG under 5MB.'
	String get error_picture_upload_failed => 'Failed to upload the profile picture. Check that it\'s a JPG/PNG under 5MB.';

	/// en: 'This number is already connected to another Replider account. Contact support to release it.'
	String get error_phone_owned_elsewhere => 'This number is already connected to another Replider account. Contact support to release it.';

	/// en: 'Channel without valid credentials. Reconnect via Embedded Signup.'
	String get error_missing_creds => 'Channel without valid credentials. Reconnect via Embedded Signup.';

	/// en: 'Operation not supported for this channel type.'
	String get error_not_supported => 'Operation not supported for this channel type.';

	/// en: 'Your free trial has ended. Upgrade your plan to keep using WhatsApp.'
	String get error_trial_expired => 'Your free trial has ended. Upgrade your plan to keep using WhatsApp.';

	/// en: 'Your subscription is not active. Update the payment method to reactivate.'
	String get error_subscription_inactive => 'Your subscription is not active. Update the payment method to reactivate.';

	/// en: 'Meta code $code.'
	String error_meta_code({required Object code}) => 'Meta code ${code}.';

	/// en: 'Unknown error ($code).'
	String error_unknown_with_code({required Object code}) => 'Unknown error (${code}).';

	/// en: 'A template with this name and language already exists. Pick another name or delete the existing one before resubmitting.'
	String get error_invalid_param_duplicate_name => 'A template with this name and language already exists. Pick another name or delete the existing one before resubmitting.';

	/// en: 'Template creation is paused for this account. Wait a few hours or contact Meta support.'
	String get error_invalid_param_paused => 'Template creation is paused for this account. Wait a few hours or contact Meta support.';

	/// en: 'This language isn't enabled for templates on this account yet. Try pt_BR or en_US.'
	String get error_invalid_param_language => 'This language isn\'t enabled for templates on this account yet. Try pt_BR or en_US.';

	/// en: 'Meta rejected the template. The text likely doesn't match the chosen category: MARKETING for promotions/news, UTILITY only for confirmations/transactional notices, AUTHENTICATION only for verification codes. Also review variables {{1}} {{2}} and examples.'
	String get error_invalid_param_category_mismatch => 'Meta rejected the template. The text likely doesn\'t match the chosen category: MARKETING for promotions/news, UTILITY only for confirmations/transactional notices, AUTHENTICATION only for verification codes. Also review variables {{1}} {{2}} and examples.';

	/// en: 'Meta rejected the parameters: $message$trace'
	String error_invalid_param_default({required Object message, required Object trace}) => 'Meta rejected the parameters: ${message}${trace}';

	/// en: 'HOW THE CUSTOMER WILL RECEIVE IT'
	String get template_preview_label => 'HOW THE CUSTOMER WILL RECEIVE IT';

	/// en: '+ Create new template'
	String get create_template_link => '+ Create new template';

	/// en: 'Create template'
	String get create_template_title => 'Create template';

	/// en: 'Meta must approve every template — usually minutes to a day. It cannot be used right NOW, but stays available here for every future conversation.'
	String get create_template_approval_notice => 'Meta must approve every template — usually minutes to a day. It cannot be used right NOW, but stays available here for every future conversation.';

	/// en: 'Resume conversation'
	String get suggestion_reopen => 'Resume conversation';

	/// en: 'Proposal follow-up'
	String get suggestion_followup => 'Proposal follow-up';

	/// en: 'From scratch'
	String get suggestion_blank => 'From scratch';

	/// en: 'Name'
	String get create_template_name_label => 'Name';

	/// en: 'lowercase, numbers and _ only'
	String get create_template_name_helper => 'lowercase, numbers and _ only';

	/// en: 'Body'
	String get create_template_body_label => 'Body';

	/// en: '{{1}}, {{2}} become fill-in fields at send time. Cannot start or end with a variable.'
	String get create_template_body_helper => '{{1}}, {{2}} become fill-in fields at send time. Cannot start or end with a variable.';

	/// en: 'Fill in the name and the body.'
	String get create_template_missing_fields => 'Fill in the name and the body.';

	/// en: 'Template submitted — it appears in the list once Meta approves it.'
	String get create_template_submitted => 'Template submitted — it appears in the list once Meta approves it.';

	/// en: 'Submit for approval'
	String get create_template_submit => 'Submit for approval';
}

// Path: notifications
class Translations$notifications$en {
	Translations$notifications$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Notifications'
	String get title => 'Notifications';

	/// en: 'Get alerts even when the tab is in the background'
	String get subtitle_default => 'Get alerts even when the tab is in the background';

	/// en: 'You'll get an alert when a customer writes'
	String get subtitle_subscribed => 'You\'ll get an alert when a customer writes';

	/// en: 'Permission blocked. Allow it in Site settings > Notifications'
	String get subtitle_denied => 'Permission blocked. Allow it in Site settings > Notifications';

	/// en: 'Browser doesn't support push notifications'
	String get subtitle_unsupported => 'Browser doesn\'t support push notifications';

	/// en: 'On iPhone, install the app on the home screen to receive notifications'
	String get subtitle_ios_install => 'On iPhone, install the app on the home screen to receive notifications';

	/// en: 'Couldn't load notification state: $error'
	String error_loading({required Object error}) => 'Couldn\'t load notification state: ${error}';

	/// en: 'Send a test'
	String get test_btn => 'Send a test';

	/// en: 'No active subscription on this device. Toggle notifications on first.'
	String get test_no_subs => 'No active subscription on this device. Toggle notifications on first.';

	/// en: 'Test sent: $ok delivered, $failed failed.'
	String test_success({required Object ok, required Object failed}) => 'Test sent: ${ok} delivered, ${failed} failed.';

	/// en: 'All $count subscriptions failed. Re-enable notifications to refresh keys.'
	String test_all_failed({required Object count}) => 'All ${count} subscriptions failed. Re-enable notifications to refresh keys.';

	/// en: '$count subscription(s) disabled — likely VAPID key mismatch. Re-enable on each device.'
	String test_disabled_some({required Object count}) => '${count} subscription(s) disabled — likely VAPID key mismatch. Re-enable on each device.';

	/// en: 'Test request failed: $error'
	String test_request_failed({required Object error}) => 'Test request failed: ${error}';
}

// Path: api_keys
class Translations$api_keys$en {
	Translations$api_keys$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'AI provider keys'
	String get title => 'AI provider keys';

	/// en: 'Your contract plan uses your own API keys for AI requests. Replider stores them encrypted and never shares them.'
	String get subtitle => 'Your contract plan uses your own API keys for AI requests. Replider stores them encrypted and never shares them.';

	/// en: 'Not configured'
	String get not_configured => 'Not configured';

	/// en: 'Paste the API key'
	String get input_hint => 'Paste the API key';

	/// en: 'Add'
	String get add => 'Add';

	/// en: 'Change'
	String get edit => 'Change';

	/// en: 'Save'
	String get save => 'Save';

	/// en: 'Clear'
	String get clear => 'Clear';

	/// en: 'Key updated.'
	String get saved => 'Key updated.';

	/// en: 'Couldn't save the key: $error'
	String save_failed({required Object error}) => 'Couldn\'t save the key: ${error}';
}

// Path: notes
class Translations$notes$en {
	Translations$notes$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Error adding note'
	String get add_error => 'Error adding note';

	/// en: 'Error deleting note'
	String get delete_error => 'Error deleting note';

	/// en: 'Internal notes'
	String get internal_notes => 'Internal notes';

	/// en: 'No notes yet'
	String get no_notes => 'No notes yet';

	/// en: 'Internal note'
	String get internal_note => 'Internal note';

	/// en: 'Error loading notes'
	String get load_error => 'Error loading notes';

	/// en: 'Add note...'
	String get add_hint => 'Add note...';

	/// en: 'Delete note?'
	String get delete_confirm_title => 'Delete note?';

	/// en: 'This internal note will be removed for the whole team.'
	String get delete_confirm_body => 'This internal note will be removed for the whole team.';
}

// Path: message_actions
class Translations$message_actions$en {
	Translations$message_actions$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Copy text'
	String get copy_text => 'Copy text';

	/// en: 'Message copied'
	String get message_copied => 'Message copied';

	/// en: 'Copy message ID'
	String get copy_message_id => 'Copy message ID';

	/// en: 'ID copied'
	String get id_copied => 'ID copied';
}

// Path: contact_sheet
class Translations$contact_sheet$en {
	Translations$contact_sheet$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Info'
	String get tab_info => 'Info';

	/// en: 'Files'
	String get tab_files => 'Files';

	/// en: 'Conversations'
	String get stat_conversations => 'Conversations';

	/// en: 'Messages'
	String get stat_messages => 'Messages';

	/// en: 'Customer since'
	String get stat_customer_since => 'Customer since';

	/// en: 'Images & videos'
	String get files_section_images => 'Images & videos';

	/// en: 'Documents & audio'
	String get files_section_documents => 'Documents & audio';

	/// en: 'No media exchanged in this conversation yet.'
	String get files_empty => 'No media exchanged in this conversation yet.';

	/// en: 'Could not load files.'
	String get files_error => 'Could not load files.';

	/// en: 'Unnamed file'
	String get files_unnamed => 'Unnamed file';

	/// en: 'Conversation'
	String get conversation_section => 'Conversation';

	/// en: '$convs conversations · $msgs messages · first contact $since'
	String footer_stats({required Object convs, required Object msgs, required Object since}) => '${convs} conversations · ${msgs} messages · first contact ${since}';

	/// en: 'Email attachments'
	String get files_section_attachments => 'Email attachments';

	/// en: '(no subject)'
	String get files_email_no_subject => '(no subject)';
}

// Path: conversation
class Translations$conversation$en {
	Translations$conversation$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Failed to send. Message recovered — please try again.'
	String get send_failed_recovered => 'Failed to send. Message recovered — please try again.';

	/// en: 'Not delivered'
	String get message_send_failed => 'Not delivered';

	/// en: 'Resume Bot'
	String get resume_bot => 'Resume Bot';

	/// en: 'Could not read the file.'
	String get file_read_error => 'Could not read the file.';

	/// en: 'Error sending file: $error'
	String file_send_error({required Object error}) => 'Error sending file: ${error}';

	/// en: 'Nobody'
	String get nobody => 'Nobody';

	/// en: 'Take over'
	String get assume => 'Take over';

	/// en: 'Return to bot'
	String get release => 'Return to bot';

	/// en: 'Being handled by $name'
	String under_takeover_by({required Object name}) => 'Being handled by ${name}';

	/// en: 'Under human handling'
	String get under_takeover_anonymous => 'Under human handling';

	/// en: 'Take over the conversation to reply manually'
	String get input_disabled_tooltip => 'Take over the conversation to reply manually';

	/// en: 'Resolve'
	String get resolve_action => 'Resolve';

	/// en: 'Reopen'
	String get reopen_action => 'Reopen';

	/// en: 'Load earlier messages'
	String get load_older_messages => 'Load earlier messages';

	/// en: 'Resolve conversation?'
	String get resolve_confirm_title => 'Resolve conversation?';

	/// en: 'The conversation will move to "Closed". You can reopen it later from the list filter.'
	String get resolve_confirm_body => 'The conversation will move to "Closed". You can reopen it later from the list filter.';

	/// en: 'Convert lead'
	String get convert_lead_action => 'Convert lead';

	/// en: 'Convert lead into customer'
	String get convert_lead_title => 'Convert lead into customer';

	/// en: 'Links a customer account to this conversation's assigned attendant as seller (inbound lead, 20% track).'
	String get convert_lead_help => 'Links a customer account to this conversation\'s assigned attendant as seller (inbound lead, 20% track).';

	/// en: 'Account name or email'
	String get convert_lead_search_hint => 'Account name or email';

	/// en: 'Type at least 2 characters to search.'
	String get convert_lead_search_min => 'Type at least 2 characters to search.';

	/// en: 'No account matches this search.'
	String get convert_lead_no_results => 'No account matches this search.';

	/// en: 'Already linked to a seller'
	String get convert_lead_already_linked => 'Already linked to a seller';

	/// en: 'Convert'
	String get convert_lead_cta => 'Convert';

	/// en: 'Lead converted: $account is now in the assigned attendant's portfolio.'
	String convert_lead_success({required Object account}) => 'Lead converted: ${account} is now in the assigned attendant\'s portfolio.';

	/// en: 'Could not convert lead: $error'
	String convert_lead_error({required Object error}) => 'Could not convert lead: ${error}';

	/// en: 'Urgent'
	String get priority_urgent => 'Urgent';

	/// en: 'High'
	String get priority_high => 'High';

	/// en: 'Normal'
	String get priority_normal => 'Normal';

	/// en: 'Low'
	String get priority_low => 'Low';

	/// en: 'Assign'
	String get assign_label => 'Assign';

	/// en: 'Tags'
	String get tags_label => 'Tags';

	/// en: 'Select Tags'
	String get tag_picker_title => 'Select Tags';

	/// en: 'New tag'
	String get tag_picker_new => 'New tag';

	/// en: 'Search tag...'
	String get tag_picker_search => 'Search tag...';

	/// en: 'You don't have any tags yet.'
	String get tag_picker_empty_title => 'You don\'t have any tags yet.';

	/// en: 'Create your first tag using the button above.'
	String get tag_picker_empty_body => 'Create your first tag using the button above.';

	/// en: 'No tag matching "$query".'
	String tag_picker_no_match({required Object query}) => 'No tag matching "${query}".';

	/// en: 'Error updating tag: $error'
	String tag_update_error({required Object error}) => 'Error updating tag: ${error}';

	/// en: 'Error creating tag: $error'
	String tag_create_error({required Object error}) => 'Error creating tag: ${error}';

	/// en: 'Color'
	String get tag_color_label => 'Color';

	/// en: 'New tag'
	String get tag_dialog_title => 'New tag';

	/// en: 'Name'
	String get tag_name_label => 'Name';

	/// en: 'e.g. VIP, Support, Question'
	String get tag_name_hint => 'e.g. VIP, Support, Question';

	/// en: 'Create'
	String get tag_dialog_create => 'Create';

	/// en: 'Bio'
	String get contact_bio => 'Bio';

	/// en: 'Status'
	String get contact_status_label => 'Status';

	/// en: 'Bot'
	String get contact_bot_label => 'Bot';

	/// en: 'Paused'
	String get contact_bot_paused => 'Paused';

	/// en: 'Active'
	String get contact_bot_active => 'Active';

	/// en: 'Handled by'
	String get contact_handled_by => 'Handled by';

	/// en: 'Last message'
	String get contact_last_message => 'Last message';

	/// en: 'Close'
	String get contact_close_btn => 'Close';

	/// en: 'Open'
	String get status_open => 'Open';

	/// en: 'Waiting for agent'
	String get status_pending => 'Waiting for agent';

	/// en: 'WhatsApp'
	String get source_whatsapp => 'WhatsApp';

	/// en: 'Telegram'
	String get source_telegram => 'Telegram';

	/// en: 'Instagram'
	String get source_instagram => 'Instagram';

	/// en: 'SMS'
	String get source_sms => 'SMS';

	/// en: 'Internal support'
	String get source_internal => 'Internal support';

	/// en: 'Email'
	String get source_email => 'Email';

	/// en: 'Shared contact'
	String get shared_contact => 'Shared contact';

	/// en: 'Number copied'
	String get shared_contact_copied => 'Number copied';

	/// en: 'Video'
	String get media_video => 'Video';

	/// en: 'This conversation is being handled by $name. Hand it back to the bot anyway?'
	String resume_bot_other_confirm({required Object name}) => 'This conversation is being handled by ${name}. Hand it back to the bot anyway?';

	/// en: 'handled by $name'
	String contact_handled_by_inline({required Object name}) => 'handled by ${name}';

	/// en: 'bot paused'
	String get contact_bot_paused_inline => 'bot paused';

	/// en: 'bot active'
	String get contact_bot_active_inline => 'bot active';
}

// Path: lgpd
class Translations$lgpd$en {
	Translations$lgpd$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Anonymize data?'
	String get anonymize_title => 'Anonymize data?';

	/// en: 'Anonymize'
	String get anonymize => 'Anonymize';

	/// en: 'Copy JSON'
	String get copy_json => 'Copy JSON';

	/// en: 'No actions recorded yet.'
	String get no_actions => 'No actions recorded yet.';

	/// en: 'GDPR export'
	String get audit_action_export => 'GDPR export';

	/// en: 'GDPR anonymisation'
	String get audit_action_anonymize => 'GDPR anonymisation';

	/// en: 'Data privacy & audit'
	String get screen_title => 'Data privacy & audit';

	/// en: 'DATA SUBJECT RIGHTS (Art. 18 of the Brazilian LGPD)'
	String get section_data_subject_rights => 'DATA SUBJECT RIGHTS (Art. 18 of the Brazilian LGPD)';

	/// en: 'INFRASTRUCTURE'
	String get section_infrastructure => 'INFRASTRUCTURE';

	/// en: 'AUDIT TRAIL'
	String get section_audit_trail => 'AUDIT TRAIL';

	/// en: 'This action is IRREVERSIBLE. All personal data for contact $externalId will be erased (name, phone, messages). Aggregate metrics (counts, campaigns) are preserved. The action is recorded in the audit log.'
	String anonymize_confirm_body({required Object externalId}) => 'This action is IRREVERSIBLE. All personal data for contact ${externalId} will be erased (name, phone, messages). Aggregate metrics (counts, campaigns) are preserved. The action is recorded in the audit log.';

	/// en: 'JSON copied. Save it to a file lgpd-export-$id.json'
	String json_copied_hint({required Object id}) => 'JSON copied. Save it to a file lgpd-export-${id}.json';

	/// en: 'Contact data anonymised. The action was recorded in the audit log.'
	String get anonymize_success => 'Contact data anonymised. The action was recorded in the audit log.';

	/// en: 'Search a contact by phone (E.164, e.g. 5511999999999) to export or erase their data.'
	String get titular_search_hint => 'Search a contact by phone (E.164, e.g. 5511999999999) to export or erase their data.';

	/// en: 'Data found for $id'
	String data_found_for({required Object id}) => 'Data found for ${id}';

	/// en: 'Conversations'
	String get stat_conversations => 'Conversations';

	/// en: 'Messages'
	String get stat_messages => 'Messages';

	/// en: 'Campaigns'
	String get stat_campaigns => 'Campaigns';

	/// en: '$shown of $total records'
	String audit_records_summary({required Object shown, required Object total}) => '${shown} of ${total} records';

	/// en: 'What this screen is for'
	String get intro_title => 'What this screen is for';

	/// en: 'Admin panel for privacy and audit. This is where you respond to customer requests under data privacy law, test the server's email delivery, and review everything that account admins have done.'
	String get intro_body => 'Admin panel for privacy and audit. This is where you respond to customer requests under data privacy law, test the server\'s email delivery, and review everything that account admins have done.';

	/// en: 'Data subject rights'
	String get intro_bullet_rights_title => 'Data subject rights';

	/// en: 'Export or anonymise a contact's data (name, phone, messages) — handles Art. 18 of the Brazilian LGPD in one click.'
	String get intro_bullet_rights_body => 'Export or anonymise a contact\'s data (name, phone, messages) — handles Art. 18 of the Brazilian LGPD in one click.';

	/// en: 'Infrastructure'
	String get intro_bullet_infra_title => 'Infrastructure';

	/// en: 'Quick SMTP server test to make sure verification, password recovery and alert emails are going out.'
	String get intro_bullet_infra_body => 'Quick SMTP server test to make sure verification, password recovery and alert emails are going out.';

	/// en: 'Audit trail'
	String get intro_bullet_audit_title => 'Audit trail';

	/// en: 'Timeline of every administrative action (takeover, anonymisation, channel creation, etc.). Use the filters to track an actor or a specific action; export CSV to attach to reports.'
	String get intro_bullet_audit_body => 'Timeline of every administrative action (takeover, anonymisation, channel creation, etc.). Use the filters to track an actor or a specific action; export CSV to attach to reports.';
}

// Path: feedback
class Translations$feedback$en {
	Translations$feedback$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'How likely are you to recommend Replider?'
	String get nps_title => 'How likely are you to recommend Replider?';

	/// en: 'On a scale from 0 to 10, how likely are you to recommend Replider to a colleague or friend?'
	String get nps_subtitle => 'On a scale from 0 to 10, how likely are you to recommend Replider to a colleague or friend?';

	/// en: 'Not likely'
	String get nps_scale_low => 'Not likely';

	/// en: 'Very likely'
	String get nps_scale_high => 'Very likely';

	/// en: 'What's the main reason for your score? (optional)'
	String get nps_comment_hint => 'What\'s the main reason for your score? (optional)';

	/// en: 'Send feedback'
	String get nps_submit => 'Send feedback';

	/// en: 'Maybe later'
	String get nps_later => 'Maybe later';

	/// en: 'Thanks for your feedback!'
	String get nps_thanks => 'Thanks for your feedback!';

	/// en: 'Before you go — why are you cancelling?'
	String get cancel_title => 'Before you go — why are you cancelling?';

	/// en: 'Your feedback helps us improve Replider. This takes just a few seconds.'
	String get cancel_subtitle => 'Your feedback helps us improve Replider. This takes just a few seconds.';

	/// en: 'Anything else you'd like to share? (optional)'
	String get cancel_comment_hint => 'Anything else you\'d like to share? (optional)';

	/// en: 'Send feedback'
	String get cancel_submit => 'Send feedback';

	/// en: 'Thanks for your feedback!'
	String get cancel_thanks => 'Thanks for your feedback!';

	late final Translations$feedback$cancel_reasons$en cancel_reasons = Translations$feedback$cancel_reasons$en.internal(_root);
}

// Path: product_tour
class Translations$product_tour$en {
	Translations$product_tour$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Skip tour'
	String get skip => 'Skip tour';

	/// en: 'Next'
	String get next => 'Next';

	/// en: 'Got it'
	String get done => 'Got it';

	/// en: '$current of $total'
	String step_counter({required Object current, required Object total}) => '${current} of ${total}';

	/// en: 'Welcome to Replider 🎉'
	String get step1_title => 'Welcome to Replider 🎉';

	/// en: 'Your AI agent is ready. Here's a quick 30-second tour of where everything lives so you can get the most out of it.'
	String get step1_body => 'Your AI agent is ready. Here\'s a quick 30-second tour of where everything lives so you can get the most out of it.';

	/// en: 'Your inbox'
	String get step2_title => 'Your inbox';

	/// en: 'Every conversation from your channels lands here in real time. Jump in anytime to take over from the agent and reply yourself.'
	String get step2_body => 'Every conversation from your channels lands here in real time. Jump in anytime to take over from the agent and reply yourself.';

	/// en: 'Train your agent'
	String get step3_title => 'Train your agent';

	/// en: 'Add knowledge — prices, hours, policies — in Settings so the agent answers accurately instead of guessing. The more you add, the smarter it gets.'
	String get step3_body => 'Add knowledge — prices, hours, policies — in Settings so the agent answers accurately instead of guessing. The more you add, the smarter it gets.';

	/// en: 'Watch your usage'
	String get step4_title => 'Watch your usage';

	/// en: 'The bar at the bottom tracks conversations and tokens for your billing cycle. Upgrade anytime to lift your limits as you grow.'
	String get step4_body => 'The bar at the bottom tracks conversations and tokens for your billing cycle. Upgrade anytime to lift your limits as you grow.';
}

// Path: feature_announcement
class Translations$feature_announcement$en {
	Translations$feature_announcement$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'What's new'
	String get title => 'What\'s new';

	/// en: 'Latest updates'
	String get release_label => 'Latest updates';

	/// en: 'Got it'
	String get cta => 'Got it';

	/// en: 'Manage your subscription anytime'
	String get item1_title => 'Manage your subscription anytime';

	/// en: 'Open the secure billing portal right from the app to update your card, switch plans or cancel — no support ticket needed.'
	String get item1_body => 'Open the secure billing portal right from the app to update your card, switch plans or cancel — no support ticket needed.';

	/// en: 'Save 20% with annual billing'
	String get item2_title => 'Save 20% with annual billing';

	/// en: 'Switch any plan to yearly and pay less per month. Toggle the interval on the billing screen to see your new price.'
	String get item2_body => 'Switch any plan to yearly and pay less per month. Toggle the interval on the billing screen to see your new price.';

	/// en: 'Usage alerts before you hit the limit'
	String get item3_title => 'Usage alerts before you hit the limit';

	/// en: 'We now warn you at 80% of your monthly quota, so a busy week never blocks your agent by surprise.'
	String get item3_body => 'We now warn you at 80% of your monthly quota, so a busy week never blocks your agent by surprise.';
}

// Path: errors
class Translations$errors$en {
	Translations$errors$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Invalid email or password.'
	String get AUTH_INVALID_CREDENTIALS => 'Invalid email or password.';

	/// en: 'The webhook URL cannot point at an internal network address.'
	String get WEBHOOK_URL_BLOCKED => 'The webhook URL cannot point at an internal network address.';

	/// en: 'This email is already registered.'
	String get AUTH_EMAIL_ALREADY_EXISTS => 'This email is already registered.';

	/// en: 'Sign-ups are temporarily closed. Already have an account? Just log in.'
	String get REGISTRATION_PAUSED => 'Sign-ups are temporarily closed. Already have an account? Just log in.';

	/// en: 'This email already belongs to an account. Use the "existing account" mode to apply a contract to it.'
	String get OWNER_EMAIL_EXISTS => 'This email already belongs to an account. Use the "existing account" mode to apply a contract to it.';

	/// en: 'No active seller with this email on the platform account.'
	String get OWNER_SELLER_NOT_FOUND => 'No active seller with this email on the platform account.';

	/// en: 'This user was not found on the platform account.'
	String get COMMISSION_SELLER_NOT_FOUND => 'This user was not found on the platform account.';

	/// en: 'This seller is deactivated.'
	String get OWNER_SELLER_INACTIVE => 'This seller is deactivated.';

	/// en: 'Invalid lead source — use prospected (ref) or inbound.'
	String get OWNER_INVALID_LEAD_SOURCE => 'Invalid lead source — use prospected (ref) or inbound.';

	/// en: 'PLATFORM_ACCOUNT_ID is not configured in this environment.'
	String get OWNER_PLATFORM_NOT_CONFIGURED => 'PLATFORM_ACCOUNT_ID is not configured in this environment.';

	/// en: 'This conversation has no assigned attendant. Take it over (or assign it) before converting.'
	String get SELLER_CONV_UNASSIGNED => 'This conversation has no assigned attendant. Take it over (or assign it) before converting.';

	/// en: 'Only the assigned attendant or an admin can convert this lead.'
	String get SELLER_NOT_CONV_ASSIGNEE => 'Only the assigned attendant or an admin can convert this lead.';

	/// en: 'This account is already linked to a seller. Reallocation is done from the owner panel.'
	String get SELLER_ACCOUNT_ALREADY_LINKED => 'This account is already linked to a seller. Reallocation is done from the owner panel.';

	/// en: 'The platform account itself can't be converted.'
	String get SELLER_CANNOT_CONVERT_PLATFORM => 'The platform account itself can\'t be converted.';

	/// en: 'You are not a registered seller. Ask the owner to grant seller access.'
	String get SELLER_NOT_FLAGGED => 'You are not a registered seller. Ask the owner to grant seller access.';

	/// en: 'This account already has payment history and the conversation's contact does not match it. Conversion blocked.'
	String get SELLER_CONV_TARGET_UNRELATED => 'This account already has payment history and the conversation\'s contact does not match it. Conversion blocked.';

	/// en: 'Session expired. Please login again.'
	String get AUTH_UNAUTHORIZED => 'Session expired. Please login again.';

	/// en: 'Social authentication failed. Please try again.'
	String get AUTH_SOCIAL_FAILED => 'Social authentication failed. Please try again.';

	/// en: 'Invalid or expired reset link.'
	String get AUTH_INVALID_RESET_TOKEN => 'Invalid or expired reset link.';

	/// en: 'Please verify your email before logging in. Check your inbox or request a new link.'
	String get AUTH_EMAIL_NOT_VERIFIED => 'Please verify your email before logging in. Check your inbox or request a new link.';

	/// en: 'Agent not found.'
	String get AGENT_NOT_FOUND => 'Agent not found.';

	/// en: 'You've reached the agent limit for your plan. Upgrade to create more agents.'
	String get AGENT_LIMIT_REACHED => 'You\'ve reached the agent limit for your plan. Upgrade to create more agents.';

	/// en: 'You reached your contract's limit. Talk to us to expand it.'
	String get PLAN_LIMIT_EXCEEDED => 'You reached your contract\'s limit. Talk to us to expand it.';

	/// en: 'Your contract covers $limit agents (you have $current). Talk to us to expand it.'
	String PLAN_LIMIT_EXCEEDED_agents({required Object limit, required Object current}) => 'Your contract covers ${limit} agents (you have ${current}). Talk to us to expand it.';

	/// en: 'Your contract covers $limit channels (you have $current). Talk to us to expand it.'
	String PLAN_LIMIT_EXCEEDED_channels({required Object limit, required Object current}) => 'Your contract covers ${limit} channels (you have ${current}). Talk to us to expand it.';

	/// en: 'Your contract covers $limit seats (you have $current). Talk to us to expand it.'
	String PLAN_LIMIT_EXCEEDED_seats({required Object limit, required Object current}) => 'Your contract covers ${limit} seats (you have ${current}). Talk to us to expand it.';

	/// en: 'Conversation not found.'
	String get CONV_NOT_FOUND => 'Conversation not found.';

	/// en: 'Outside the WhatsApp 24h messaging window.'
	String get CONV_OUTSIDE_24H => 'Outside the WhatsApp 24h messaging window.';

	/// en: 'Insufficient AI tokens.'
	String get USAGE_INSUFFICIENT_TOKENS => 'Insufficient AI tokens.';

	/// en: 'AI usage limit reached.'
	String get USAGE_NO_CREDITS => 'AI usage limit reached.';

	/// en: 'This cycle's usage limit was reached. Talk to us to expand it, or wait for the next cycle.'
	String get USAGE_LIMIT_EXCEEDED => 'This cycle\'s usage limit was reached. Talk to us to expand it, or wait for the next cycle.';

	/// en: 'Account suspended due to an unpaid invoice. Update your payment method to continue.'
	String get ACCOUNT_SUSPENDED => 'Account suspended due to an unpaid invoice. Update your payment method to continue.';

	/// en: 'Cannot remove the last admin. Transfer the admin role first.'
	String get TEAM_CANNOT_REMOVE_LAST_ADMIN => 'Cannot remove the last admin. Transfer the admin role first.';

	/// en: 'Selected participant is not an active member of the team.'
	String get TEAM_INVALID_PARTICIPANT => 'Selected participant is not an active member of the team.';

	/// en: 'Too many requests. Please slow down and try again in a minute.'
	String get RATE_LIMITED => 'Too many requests. Please slow down and try again in a minute.';

	/// en: 'Your trial has ended. Talk to us to activate your contract.'
	String get TRIAL_EXPIRED => 'Your trial has ended. Talk to us to activate your contract.';

	/// en: 'Your account is not active. Talk to us to sort it out.'
	String get SUBSCRIPTION_INACTIVE => 'Your account is not active. Talk to us to sort it out.';

	/// en: 'You must accept the Terms of Service and Privacy Policy.'
	String get TERMS_NOT_ACCEPTED => 'You must accept the Terms of Service and Privacy Policy.';

	/// en: 'The server took too long to respond. Try again in a moment.'
	String get CONNECTION_TIMEOUT => 'The server took too long to respond. Try again in a moment.';

	/// en: 'We couldn't reach the server. It may be your connection or a problem on our side — try again in a moment.'
	String get CONNECTION_ERROR => 'We couldn\'t reach the server. It may be your connection or a problem on our side — try again in a moment.';

	/// en: 'Server temporarily unavailable. Please try again in a few minutes.'
	String get SERVER_ERROR => 'Server temporarily unavailable. Please try again in a few minutes.';

	/// en: 'Request canceled.'
	String get REQUEST_CANCELED => 'Request canceled.';

	/// en: 'Invalid input. Please check the fields and try again.'
	String get BAD_REQUEST => 'Invalid input. Please check the fields and try again.';

	/// en: 'An unexpected error occurred. Please try again.'
	String get UNKNOWN => 'An unexpected error occurred. Please try again.';

	/// en: 'Code did not match. Check the time on your device and try again.'
	String get TOTP_INVALID_CODE => 'Code did not match. Check the time on your device and try again.';

	/// en: 'Your two-factor proof expired. Confirm the code to continue.'
	String get OWNER_2FA_STALE => 'Your two-factor proof expired. Confirm the code to continue.';

	/// en: 'Enable 2FA in your profile to use the owner panel.'
	String get OWNER_2FA_REQUIRED => 'Enable 2FA in your profile to use the owner panel.';

	/// en: 'Start the 2FA setup before confirming.'
	String get TOTP_NOT_STAGED => 'Start the 2FA setup before confirming.';

	/// en: 'Two-factor authentication isn't enabled.'
	String get TOTP_NOT_ENABLED => 'Two-factor authentication isn\'t enabled.';

	/// en: 'Two-factor authentication is unavailable right now. Try again later.'
	String get TOTP_CIPHER_UNCONFIGURED => 'Two-factor authentication is unavailable right now. Try again later.';

	/// en: 'The message was saved but could not be delivered to the customer. Check the channel and try again.'
	String get MESSAGE_SEND_FAILED => 'The message was saved but could not be delivered to the customer. Check the channel and try again.';

	/// en: 'Plan not found. Refresh the page and try again.'
	String get PLAN_NOT_FOUND => 'Plan not found. Refresh the page and try again.';

	/// en: 'This plan is sold under contract. Talk to a specialist to subscribe.'
	String get PLAN_NOT_PURCHASABLE => 'This plan is sold under contract. Talk to a specialist to subscribe.';

	/// en: 'This plan is not available in your account's billing currency.'
	String get PLAN_CURRENCY_MISMATCH => 'This plan is not available in your account\'s billing currency.';

	/// en: 'This coupon has reached its redemption limit.'
	String get COUPON_EXHAUSTED => 'This coupon has reached its redemption limit.';

	/// en: 'Extra credit packs are not available in your account's billing currency.'
	String get ADDON_CURRENCY_UNSUPPORTED => 'Extra credit packs are not available in your account\'s billing currency.';

	/// en: 'Your plan doesn't include bulk campaigns. Upgrade to enable broadcasts to contact lists.'
	String get BROADCAST_NOT_ALLOWED => 'Your plan doesn\'t include bulk campaigns. Upgrade to enable broadcasts to contact lists.';

	/// en: 'This campaign has more recipients than your plan allows ($current/$limit). Split it into smaller campaigns or upgrade.'
	String RECIPIENTS_OVER_CAMPAIGN_CAP({required Object current, required Object limit}) => 'This campaign has more recipients than your plan allows (${current}/${limit}). Split it into smaller campaigns or upgrade.';

	/// en: 'You've reached the monthly broadcast cap ($current/$limit). Wait for renewal or upgrade.'
	String MONTHLY_BROADCAST_CAP({required Object current, required Object limit}) => 'You\'ve reached the monthly broadcast cap (${current}/${limit}). Wait for renewal or upgrade.';

	/// en: 'Name is empty.'
	String get EMPTY_NAME => 'Name is empty.';

	/// en: 'Unexpected error.'
	String get UNEXPECTED => 'Unexpected error.';

	/// en: 'flow.json must be a valid JSON object.'
	String get JSON_INVALID_OBJECT => 'flow.json must be a valid JSON object.';

	/// en: 'Invalid JSON: $message'
	String JSON_INVALID_FORMAT({required Object message}) => 'Invalid JSON: ${message}';

	/// en: 'Price row not found.'
	String get PROVIDER_COST_NOT_FOUND => 'Price row not found.';

	/// en: 'A price for this provider/model/date already exists.'
	String get PROVIDER_COST_DUPLICATE => 'A price for this provider/model/date already exists.';

	/// en: 'Rate must be between 1.00 and 100.00 BRL per USD.'
	String get PROVIDER_COST_INVALID_RATE => 'Rate must be between 1.00 and 100.00 BRL per USD.';

	/// en: 'Only admins can manage team-wide quick replies.'
	String get CANNED_RESPONSE_FORBIDDEN => 'Only admins can manage team-wide quick replies.';

	/// en: 'Token not found or already revoked.'
	String get MCP_TOKEN_NOT_FOUND => 'Token not found or already revoked.';

	/// en: 'Active token limit reached (10). Revoke an old token before creating another.'
	String get MCP_TOKEN_LIMIT_REACHED => 'Active token limit reached (10). Revoke an old token before creating another.';

	/// en: 'Invalid phone. Use area code + number, like 51 98888-7766.'
	String get LEAD_INVALID_PHONE => 'Invalid phone. Use area code + number, like 51 98888-7766.';

	/// en: 'That approach is no longer waiting for approval.'
	String get LEAD_NOT_PENDING_APPROVAL => 'That approach is no longer waiting for approval.';

	/// en: 'Couldn't send the approach. Try again in a moment.'
	String get LEAD_SEND_FAILED => 'Couldn\'t send the approach. Try again in a moment.';

	/// en: 'Lead not found.'
	String get LEAD_NOT_FOUND => 'Lead not found.';

	/// en: 'The message still has a field to fill in.'
	String get APPROACH_HAS_PLACEHOLDER => 'The message still has a field to fill in.';

	/// en: 'This address no longer receives email: it bounced permanently or reported us as spam. Check Settings → Suppressed addresses.'
	String get EMAIL_ADDRESS_SUPPRESSED => 'This address no longer receives email: it bounced permanently or reported us as spam. Check Settings → Suppressed addresses.';

	/// en: 'Your plan does not include AI drafts. Upgrade to have the AI write your approaches.'
	String get AI_DRAFTS_NOT_ALLOWED => 'Your plan does not include AI drafts. Upgrade to have the AI write your approaches.';

	/// en: 'Your plan does not include AI contract reading. Upgrade to have contracts read automatically.'
	String get DOC_EXTRACTION_NOT_ALLOWED => 'Your plan does not include AI contract reading. Upgrade to have contracts read automatically.';

	/// en: 'You have reached this feature's limit for the cycle. Wait for the renewal or upgrade.'
	String get SPEND_BUDGET_EXCEEDED => 'You have reached this feature\'s limit for the cycle. Wait for the renewal or upgrade.';

	/// en: 'This screen doesn't exist'
	String get route_not_found_title => 'This screen doesn\'t exist';

	/// en: 'The address may have a typo, or the link is out of date.'
	String get route_not_found_body => 'The address may have a typo, or the link is out of date.';

	/// en: 'Go to start'
	String get route_not_found_cta => 'Go to start';

	/// en: 'I could not open that file as a PDF. Send the manufacturer's original brochure.'
	String get PRODUCT_PDF_UNREADABLE => 'I could not open that file as a PDF. Send the manufacturer\'s original brochure.';

	/// en: 'The antivirus is not available right now — PDF upload is disabled. Try again later.'
	String get PRODUCT_PDF_SCAN_UNAVAILABLE => 'The antivirus is not available right now — PDF upload is disabled. Try again later.';

	/// en: 'The antivirus refused this file.'
	String get PRODUCT_PDF_INFECTED => 'The antivirus refused this file.';

	/// en: 'The automatic reading failed. Try again, or use the CSV import.'
	String get PRODUCT_PDF_EXTRACT_FAILED => 'The automatic reading failed. Try again, or use the CSV import.';

	/// en: 'The reading found no products in this PDF.'
	String get PRODUCT_PDF_NO_PRODUCTS => 'The reading found no products in this PDF.';

	/// en: 'This account's AI provider cannot read files. Use the CSV import instead.'
	String get PRODUCT_PDF_PROVIDER_CANT_READ => 'This account\'s AI provider cannot read files. Use the CSV import instead.';

	/// en: 'PDF import is not configured in this environment.'
	String get PRODUCT_PDF_IMPORT_UNAVAILABLE => 'PDF import is not configured in this environment.';

	/// en: 'This mailbox never started its warm-up — verify the domain (DNS) or reconnect the mailbox before cold outreach.'
	String get EMAIL_WARMUP_NOT_STARTED => 'This mailbox never started its warm-up — verify the domain (DNS) or reconnect the mailbox before cold outreach.';

	/// en: 'Batch approvals need volume sending configured for your domain. Enable it in Settings → Channels (email channel), or approve one by one through your own mailbox.'
	String get BULK_SENDING_NOT_READY => 'Batch approvals need volume sending configured for your domain. Enable it in Settings → Channels (email channel), or approve one by one through your own mailbox.';

	/// en: 'Volume sending is not available on the platform right now — approve one by one, or talk to us.'
	String get BULK_SENDING_UNAVAILABLE => 'Volume sending is not available on the platform right now — approve one by one, or talk to us.';

	/// en: 'The mailbox reached its daily cold-send cap. Sending resumes tomorrow.'
	String get EMAIL_DAILY_CAP_REACHED => 'The mailbox reached its daily cold-send cap. Sending resumes tomorrow.';

	/// en: 'This send needs an approved WhatsApp template — open the lead and pick one.'
	String get LEAD_APPROACH_TEMPLATE_REQUIRED => 'This send needs an approved WhatsApp template — open the lead and pick one.';

	/// en: 'Invalid, expired or already-used invite — ask for a new link from whoever invited you.'
	String get INVITE_INVALID => 'Invalid, expired or already-used invite — ask for a new link from whoever invited you.';

	/// en: 'The default funnel can't be deleted — it's where imports and new leads land.'
	String get SALES_FUNNEL_IS_DEFAULT => 'The default funnel can\'t be deleted — it\'s where imports and new leads land.';

	/// en: 'Company search isn't available on the platform right now — talk to support.'
	String get PROSPECTING_NOT_CONFIGURED => 'Company search isn\'t available on the platform right now — talk to support.';

	/// en: 'Too many searches in a row — wait a moment and try again.'
	String get PROSPECTING_RATE_LIMITED => 'Too many searches in a row — wait a moment and try again.';

	/// en: 'Tell us what to search: a segment and a city.'
	String get PROSPECTING_INPUT_REQUIRED => 'Tell us what to search: a segment and a city.';

	/// en: 'The company search failed — try again in a moment.'
	String get PROSPECTING_SEARCH_FAILED => 'The company search failed — try again in a moment.';

	/// en: 'Register your products first — with an empty catalog the draft comes out generic.'
	String get APPROACH_NEEDS_PRODUCTS => 'Register your products first — with an empty catalog the draft comes out generic.';

	/// en: 'Connect an e-mail in Settings → Channels to send.'
	String get LEAD_FUNNEL_NO_CHANNEL => 'Connect an e-mail in Settings → Channels to send.';

	/// en: 'This customer has no e-mail on file.'
	String get LEAD_CONTACT_NOT_MESSAGEABLE => 'This customer has no e-mail on file.';

	/// en: 'This customer was already approached — reply on the conversation instead.'
	String get LEAD_ALREADY_CONTACTED => 'This customer was already approached — reply on the conversation instead.';

	/// en: 'This customer asked not to be contacted.'
	String get CONTACT_OPTED_OUT => 'This customer asked not to be contacted.';

	/// en: 'Add your AI key in Settings → Contract.'
	String get BYOK_NOT_CONFIGURED => 'Add your AI key in Settings → Contract.';

	/// en: 'Your role has no access to this.'
	String get FORBIDDEN => 'Your role has no access to this.';

	/// en: 'Nobody has written to this customer yet — send the first offer from the queue.'
	String get LEAD_NO_CONVERSATION => 'Nobody has written to this customer yet — send the first offer from the queue.';

	/// en: 'The AI provider is unavailable right now. Try again in a moment.'
	String get AI_PROVIDER_UNAVAILABLE => 'The AI provider is unavailable right now. Try again in a moment.';

	/// en: 'A lead that was already contacted cannot go back to the start of the funnel. Mark it lost or create another lead.'
	String get FUNNEL_STAGE_BACKWARD => 'A lead that was already contacted cannot go back to the start of the funnel. Mark it lost or create another lead.';
}

// Path: audio_recorder
class Translations$audio_recorder$en {
	Translations$audio_recorder$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Microphone blocked'
	String get permission_blocked_title => 'Microphone blocked';

	/// en: 'Allow microphone access in your browser/system settings and try again.'
	String get permission_blocked_body => 'Allow microphone access in your browser/system settings and try again.';

	/// en: 'OK'
	String get ok => 'OK';

	/// en: 'Cancel'
	String get cancel_tooltip => 'Cancel';

	/// en: 'Pause'
	String get pause_tooltip => 'Pause';

	/// en: 'Continue'
	String get resume_tooltip => 'Continue';

	/// en: 'Finish recording'
	String get stop_tooltip => 'Finish recording';

	/// en: 'Discard'
	String get discard_tooltip => 'Discard';

	/// en: 'Re-record'
	String get redo_tooltip => 'Re-record';

	/// en: 'Play'
	String get play_tooltip => 'Play';

	/// en: 'Pause'
	String get pause_play_tooltip => 'Pause';

	/// en: 'Send audio'
	String get send_tooltip => 'Send audio';

	/// en: 'Failed to start recording: $error'
	String start_failed({required Object error}) => 'Failed to start recording: ${error}';

	/// en: 'Failed to pause: $error'
	String pause_failed({required Object error}) => 'Failed to pause: ${error}';

	/// en: 'Failed to stop recording: $error'
	String stop_failed({required Object error}) => 'Failed to stop recording: ${error}';

	/// en: 'Failed to send audio: $error'
	String send_failed({required Object error}) => 'Failed to send audio: ${error}';

	/// en: 'Empty recording.'
	String get empty_recording => 'Empty recording.';

	/// en: 'Empty audio.'
	String get empty_audio => 'Empty audio.';
}

// Path: bulk_actions
class Translations$bulk_actions$en {
	Translations$bulk_actions$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Clear selection'
	String get clear_tooltip => 'Clear selection';

	/// en: '$count selected'
	String selected_count({required Object count}) => '${count} selected';

	/// en: 'Select all'
	String get select_all => 'Select all';

	/// en: 'Close'
	String get close_btn => 'Close';

	/// en: 'Close $count conversations?'
	String close_confirm_title({required Object count}) => 'Close ${count} conversations?';

	/// en: 'The conversations will move to "Closed". New customer messages reopen them automatically.'
	String get close_confirm_body => 'The conversations will move to "Closed". New customer messages reopen them automatically.';

	/// en: '$ok of $total closed.'
	String close_done({required Object ok, required Object total}) => '${ok} of ${total} closed.';

	/// en: ' Some failed — check permissions.'
	String get close_partial_warning => ' Some failed — check permissions.';

	/// en: 'Failed: $error'
	String close_failed({required Object error}) => 'Failed: ${error}';
}

// Path: image_viewer
class Translations$image_viewer$en {
	Translations$image_viewer$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Image'
	String get default_filename => 'Image';

	/// en: 'image.jpg'
	String get default_save_filename => 'image.jpg';

	/// en: 'Download'
	String get download_btn => 'Download';

	/// en: 'Close'
	String get close_tooltip => 'Close';

	/// en: 'Could not download the image.'
	String get download_failed => 'Could not download the image.';
}

// Path: media_send
class Translations$media_send$en {
	Translations$media_send$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'File too large ($actualMb MB). The limit for this type is $maxMb MB.'
	String file_too_large({required Object actualMb, required Object maxMb}) => 'File too large (${actualMb} MB). The limit for this type is ${maxMb} MB.';

	/// en: 'Sending $filename...'
	String sending_file({required Object filename}) => 'Sending ${filename}...';

	/// en: 'Send file'
	String get send_file_dialog_title => 'Send file';

	/// en: 'Caption (optional)'
	String get caption_label => 'Caption (optional)';

	/// en: 'Send'
	String get send_btn => 'Send';

	/// en: 'Cancel'
	String get cancel_btn => 'Cancel';

	/// en: 'Another file is already uploading on this conversation. Wait for it to finish.'
	String get upload_in_progress => 'Another file is already uploading on this conversation. Wait for it to finish.';
}

// Path: reply_suggestions
class Translations$reply_suggestions$en {
	Translations$reply_suggestions$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Generating suggestions...'
	String get loading => 'Generating suggestions...';

	/// en: 'Failed to generate suggestions. Try again.'
	String get load_failed => 'Failed to generate suggestions. Try again.';

	/// en: 'Retry'
	String get retry => 'Retry';

	/// en: 'Suggest replies'
	String get suggest_replies => 'Suggest replies';

	/// en: 'Refresh suggestions'
	String get refresh_tooltip => 'Refresh suggestions';

	/// en: 'Hide'
	String get hide_tooltip => 'Hide';
}

// Path: pwa
class Translations$pwa$en {
	Translations$pwa$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Install the app'
	String get install_title => 'Install the app';

	/// en: 'Get notified when customers message you, even when the app is closed.'
	String get install_body => 'Get notified when customers message you, even when the app is closed.';

	/// en: 'Install'
	String get install_btn => 'Install';

	/// en: 'Not now'
	String get not_now => 'Not now';

	/// en: 'Add to home screen'
	String get ios_title => 'Add to home screen';

	/// en: 'Required to receive iPhone notifications — Safari only allows alerts inside installed apps.'
	String get ios_body => 'Required to receive iPhone notifications — Safari only allows alerts inside installed apps.';

	/// en: 'Dismiss'
	String get dismiss_tooltip => 'Dismiss';

	/// en: 'Tap Share in Safari's bottom bar'
	String get ios_step_1 => 'Tap Share in Safari\'s bottom bar';

	/// en: 'Choose "Add to Home Screen"'
	String get ios_step_2 => 'Choose "Add to Home Screen"';

	/// en: 'Open Replider from the installed icon and turn on notifications'
	String get ios_step_3 => 'Open Replider from the installed icon and turn on notifications';
}

// Path: audio_player
class Translations$audio_player$en {
	Translations$audio_player$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Transcribe'
	String get transcribe_tooltip => 'Transcribe';

	/// en: 'Transcribing...'
	String get transcribing => 'Transcribing...';

	/// en: 'Failed to transcribe, try again.'
	String get transcribe_failed => 'Failed to transcribe, try again.';

	/// en: 'Couldn't load the audio. It may have expired.'
	String get load_failed => 'Couldn\'t load the audio. It may have expired.';
}

// Path: media_preview
class Translations$media_preview$en {
	Translations$media_preview$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Audio'
	String get audio => 'Audio';

	/// en: 'Image'
	String get image => 'Image';

	/// en: 'Video'
	String get video => 'Video';

	/// en: 'File'
	String get document => 'File';

	/// en: 'Location'
	String get location => 'Location';

	/// en: 'Sticker'
	String get sticker => 'Sticker';
}

// Path: sentiment
class Translations$sentiment$en {
	Translations$sentiment$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Detected sentiment: $label'
	String detected_tooltip({required Object label}) => 'Detected sentiment: ${label}';

	/// en: 'Angry'
	String get angry => 'Angry';

	/// en: 'Negative'
	String get negative => 'Negative';

	/// en: 'Positive'
	String get positive => 'Positive';
}

// Path: owner
class Translations$owner$en {
	Translations$owner$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Owner panel'
	String get menu_label => 'Owner panel';

	/// en: 'Accounts'
	String get accounts_title => 'Accounts';

	/// en: 'Search by company name or email'
	String get search_hint => 'Search by company name or email';

	/// en: 'New contract'
	String get create_contract_cta => 'New contract';

	/// en: 'No accounts yet'
	String get empty_title => 'No accounts yet';

	/// en: 'Use "New contract" to provision your first B2B client.'
	String get empty_desc => 'Use "New contract" to provision your first B2B client.';

	/// en: 'suspended'
	String get badge_suspended => 'suspended';

	/// en: 'Account'
	String get detail_title => 'Account';

	/// en: 'ID'
	String get field_id => 'ID';

	/// en: 'Status'
	String get field_status => 'Status';

	/// en: 'Owner'
	String get field_owner => 'Owner';

	/// en: 'Plan'
	String get field_plan => 'Plan';

	/// en: 'Created'
	String get field_created => 'Created';

	/// en: 'Active internal support threads'
	String get field_internal_threads => 'Active internal support threads';

	/// en: 'Suspended reason'
	String get field_suspended_reason => 'Suspended reason';

	/// en: 'Seller'
	String get seller_label => 'Seller';

	/// en: 'No seller linked'
	String get seller_none => 'No seller linked';

	/// en: 'Lead source'
	String get seller_source_label => 'Lead source';

	/// en: 'Prospected (ref) — 50%'
	String get seller_source_seller_ref => 'Prospected (ref) — 50%';

	/// en: 'Inbound — 20%'
	String get seller_source_inbound => 'Inbound — 20%';

	/// en: 'Linked at'
	String get seller_linked_at => 'Linked at';

	/// en: 'Link / reallocate'
	String get seller_edit_cta => 'Link / reallocate';

	/// en: 'Seller matches this account's own admin (self-referral) — review the link'
	String get seller_self_match_warning => 'Seller matches this account\'s own admin (self-referral) — review the link';

	/// en: 'Link seller'
	String get seller_dialog_title => 'Link seller';

	/// en: 'Seller email (platform-account user)'
	String get seller_email_hint => 'Seller email (platform-account user)';

	/// en: 'Unlink'
	String get seller_unlink_cta => 'Unlink';

	/// en: 'Save'
	String get seller_save_cta => 'Save';

	/// en: 'Seller updated.'
	String get seller_saved => 'Seller updated.';

	/// en: 'Caps (overrides)'
	String get overrides_label => 'Caps (overrides)';

	/// en: 'Leave a field empty to clear the override and fall back to the plan default.'
	String get overrides_clear_hint => 'Leave a field empty to clear the override and fall back to the plan default.';

	/// en: 'Edit caps'
	String get edit_overrides_cta => 'Edit caps';

	/// en: 'Max agents'
	String get cap_max_agents => 'Max agents';

	/// en: 'Max channels'
	String get cap_max_channels => 'Max channels';

	/// en: 'Max conversations / month'
	String get cap_max_conversations => 'Max conversations / month';

	/// en: 'Max AI tokens / month'
	String get cap_max_tokens => 'Max AI tokens / month';

	/// en: 'Usage this period'
	String get usage_label => 'Usage this period';

	/// en: 'Tokens used'
	String get usage_tokens => 'Tokens used';

	/// en: 'Addon tokens'
	String get usage_addon => 'Addon tokens';

	/// en: 'Conversations'
	String get usage_conversations => 'Conversations';

	/// en: 'Messages'
	String get usage_messages => 'Messages';

	/// en: 'Suspend account'
	String get suspend_cta => 'Suspend account';

	/// en: 'Suspend account?'
	String get suspend_dialog_title => 'Suspend account?';

	/// en: 'Reason (required)'
	String get suspend_reason_hint => 'Reason (required)';

	/// en: 'Suspend'
	String get suspend_confirm_cta => 'Suspend';

	/// en: 'Reactivate account'
	String get reactivate_cta => 'Reactivate account';

	/// en: 'Reactivate this account?'
	String get reactivate_confirm_title => 'Reactivate this account?';

	/// en: 'Inbound messages and writes will resume immediately.'
	String get reactivate_confirm_msg => 'Inbound messages and writes will resume immediately.';

	/// en: 'New contract'
	String get create_title => 'New contract';

	/// en: 'Company name'
	String get create_company => 'Company name';

	/// en: 'Admin email'
	String get create_admin_email => 'Admin email';

	/// en: 'Admin name'
	String get create_admin_name => 'Admin name';

	/// en: 'Plan'
	String get create_plan => 'Plan';

	/// en: 'Skip trial gate'
	String get create_trial_bypass => 'Skip trial gate';

	/// en: 'The account will be usable immediately without a Stripe subscription.'
	String get create_trial_bypass_hint => 'The account will be usable immediately without a Stripe subscription.';

	/// en: 'Create contract'
	String get create_submit => 'Create contract';

	/// en: 'Fill in company, admin email and plan.'
	String get form_required => 'Fill in company, admin email and plan.';

	/// en: 'Top spenders this period'
	String get top_label => 'Top spenders this period';

	/// en: 'Open'
	String get top_open_cta => 'Open';

	/// en: 'Grant credits'
	String get grant_cta => 'Grant credits';

	/// en: 'Grant addon tokens'
	String get grant_dialog_title => 'Grant addon tokens';

	/// en: 'Number of tokens'
	String get grant_hint => 'Number of tokens';

	/// en: 'Extend trial'
	String get extend_cta => 'Extend trial';

	/// en: 'Extend trial by'
	String get extend_dialog_title => 'Extend trial by';

	/// en: 'Days from now'
	String get extend_hint => 'Days from now';

	/// en: 'Login as account'
	String get impersonate_cta => 'Login as account';

	/// en: 'Login as this account?'
	String get impersonate_confirm_title => 'Login as this account?';

	/// en: 'You'll act as the account's admin user for 15 minutes. Every action is audited.'
	String get impersonate_confirm_msg => 'You\'ll act as the account\'s admin user for 15 minutes. Every action is audited.';

	/// en: 'Acting as $account — every action is logged.'
	String banner_acting_as({required Object account}) => 'Acting as ${account} — every action is logged.';

	/// en: 'End'
	String get banner_end_cta => 'End';

	/// en: 'expires in $time'
	String banner_expires_in({required Object time}) => 'expires in ${time}';

	/// en: 'All'
	String get filter_all => 'All';

	/// en: 'Active'
	String get filter_active => 'Active';

	/// en: 'Trialing'
	String get filter_trialing => 'Trialing';

	/// en: 'Past due'
	String get filter_past_due => 'Past due';

	/// en: 'Suspended'
	String get filter_suspended => 'Suspended';

	/// en: '$pct% of cap'
	String cap_badge({required Object pct}) => '${pct}% of cap';

	/// en: 'End the impersonation session before logging out.'
	String get logout_blocked_during_impersonate => 'End the impersonation session before logging out.';

	/// en: 'Recent conversations'
	String get conversations_label => 'Recent conversations';

	/// en: 'This account has no conversations yet.'
	String get conversations_empty => 'This account has no conversations yet.';

	/// en: 'Accounts'
	String get health_accounts => 'Accounts';

	/// en: 'Active'
	String get health_active => 'Active';

	/// en: 'Suspended'
	String get health_suspended => 'Suspended';

	/// en: 'Conv 24h'
	String get health_conv_24h => 'Conv 24h';

	/// en: 'Msg 24h'
	String get health_msg_24h => 'Msg 24h';

	/// en: 'Usage trend'
	String get usage_trend_label => 'Usage trend';

	/// en: 'AI tokens consumed per day, last 30 days.'
	String get usage_trend_desc => 'AI tokens consumed per day, last 30 days.';

	/// en: 'No usage in the last 30 days.'
	String get usage_trend_empty => 'No usage in the last 30 days.';

	/// en: '$tokens tokens total'
	String usage_trend_total({required Object tokens}) => '${tokens} tokens total';

	/// en: 'Invoices'
	String get invoices_label => 'Invoices';

	/// en: 'Last 10 Stripe invoices for this account.'
	String get invoices_desc => 'Last 10 Stripe invoices for this account.';

	/// en: 'No invoices yet — account has not transacted with Stripe.'
	String get invoices_empty => 'No invoices yet — account has not transacted with Stripe.';

	/// en: 'Paid'
	String get invoice_status_paid => 'Paid';

	/// en: 'Open'
	String get invoice_status_open => 'Open';

	/// en: 'Draft'
	String get invoice_status_draft => 'Draft';

	/// en: 'Uncollectible'
	String get invoice_status_uncollectible => 'Uncollectible';

	/// en: 'Void'
	String get invoice_status_void => 'Void';

	/// en: 'Open invoice page'
	String get invoice_open_hosted => 'Open invoice page';

	/// en: 'Download PDF'
	String get invoice_open_pdf => 'Download PDF';

	/// en: 'Enrol in 2FA to continue'
	String get gate_enroll_title => 'Enrol in 2FA to continue';

	/// en: 'Owner-panel access requires two-factor authentication. Set it up from your profile and come back.'
	String get gate_enroll_body => 'Owner-panel access requires two-factor authentication. Set it up from your profile and come back.';

	/// en: 'Go to profile'
	String get gate_enroll_cta => 'Go to profile';

	/// en: 'Re-confirm your 2FA code'
	String get gate_refresh_title => 'Re-confirm your 2FA code';

	/// en: 'Your two-factor proof expired. Enter the code from your authenticator to continue — no need to sign out.'
	String get gate_refresh_body => 'Your two-factor proof expired. Enter the code from your authenticator to continue — no need to sign out.';

	/// en: 'Confirm code'
	String get gate_refresh_cta => 'Confirm code';

	/// en: 'Confirm your 2FA'
	String get stepup_title => 'Confirm your 2FA';

	/// en: 'Your two-factor proof expired. Enter the code from your authenticator to continue — nothing you typed is lost.'
	String get stepup_body => 'Your two-factor proof expired. Enter the code from your authenticator to continue — nothing you typed is lost.';

	/// en: '6-digit code'
	String get stepup_code => '6-digit code';

	/// en: 'Confirm'
	String get stepup_confirm => 'Confirm';

	/// en: 'You can also use a recovery code.'
	String get stepup_hint => 'You can also use a recovery code.';

	/// en: 'Coupons'
	String get coupons_title => 'Coupons';

	/// en: 'No coupons yet'
	String get coupons_empty_title => 'No coupons yet';

	/// en: 'Mint a code and share the invite link to apply a discount at checkout.'
	String get coupons_empty_desc => 'Mint a code and share the invite link to apply a discount at checkout.';

	/// en: 'New coupon'
	String get coupon_create_cta => 'New coupon';

	/// en: 'Create coupon'
	String get coupon_create_title => 'Create coupon';

	/// en: 'Code'
	String get coupon_code_label => 'Code';

	/// en: 'Discount'
	String get coupon_percent_label => 'Discount';

	/// en: 'Max redemptions'
	String get coupon_max_label => 'Max redemptions';

	/// en: 'Leave blank for unlimited'
	String get coupon_max_hint => 'Leave blank for unlimited';

	/// en: 'Set expiry'
	String get coupon_expires_pick => 'Set expiry';

	/// en: 'No expiry'
	String get coupon_expires_none => 'No expiry';

	/// en: 'Expires $date'
	String coupon_expires({required Object date}) => 'Expires ${date}';

	/// en: 'Create'
	String get coupon_save_cta => 'Create';

	/// en: 'Code and discount are required'
	String get coupon_form_invalid => 'Code and discount are required';

	/// en: 'Coupon created'
	String get coupon_created_toast => 'Coupon created';

	/// en: 'ACTIVE'
	String get coupon_status_active => 'ACTIVE';

	/// en: 'INACTIVE'
	String get coupon_status_inactive => 'INACTIVE';

	/// en: '$percent% off'
	String coupon_percent_off({required Object percent}) => '${percent}% off';

	/// en: '$used redemptions'
	String coupon_redeemed_unlimited({required Object used}) => '${used} redemptions';

	/// en: '$used / $cap redeemed'
	String coupon_redeemed_capped({required Object used, required Object cap}) => '${used} / ${cap} redeemed';

	/// en: 'Deactivate'
	String get coupon_deactivate_cta => 'Deactivate';

	/// en: 'Deactivate coupon?'
	String get coupon_deactivate_title => 'Deactivate coupon?';

	/// en: 'Code $code will stop accepting new redemptions. Existing subscriptions keep their discount.'
	String coupon_deactivate_body({required Object code}) => 'Code ${code} will stop accepting new redemptions. Existing subscriptions keep their discount.';

	/// en: 'Overview'
	String get tab_overview => 'Overview';

	/// en: 'Usage'
	String get tab_usage => 'Usage';

	/// en: 'Invoices'
	String get tab_invoices => 'Invoices';

	/// en: 'Conversations'
	String get tab_conversations => 'Conversations';

	/// en: 'Actions'
	String get actions_menu => 'Actions';

	/// en: 'Tap a conversation to enter this account'
	String get conversations_tap_hint => 'Tap a conversation to enter this account';

	/// en: 'Sort'
	String get sort_label => 'Sort';

	/// en: 'Usage'
	String get sort_consumption => 'Usage';

	/// en: 'Recent'
	String get sort_recent => 'Recent';

	/// en: 'Status'
	String get sort_status => 'Status';

	/// en: 'Overdue on top'
	String get overdue_first => 'Overdue on top';

	/// en: 'No limit'
	String get usage_no_cap => 'No limit';

	/// en: 'New account'
	String get mode_new_account => 'New account';

	/// en: 'Existing account'
	String get mode_existing_account => 'Existing account';

	/// en: 'Search the account to apply the contract'
	String get pick_account_hint => 'Search the account to apply the contract';

	/// en: 'Account'
	String get pick_account_label => 'Account';

	/// en: 'Apply contract'
	String get apply_contract_submit => 'Apply contract';

	/// en: 'Select an account first'
	String get pick_account_required => 'Select an account first';

	/// en: 'Ban risk'
	String get wa_risk_ban => 'Ban risk';

	/// en: 'WhatsApp quality'
	String get wa_quality_warning => 'WhatsApp quality';

	/// en: 'Set plan / contract'
	String get apply_plan_cta => 'Set plan / contract';

	/// en: 'Run on our AI key'
	String get use_our_key_title => 'Run on our AI key';

	/// en: 'Bill on the platform key at the plan rate instead of requiring the client to bring their own.'
	String get use_our_key_hint => 'Bill on the platform key at the plan rate instead of requiring the client to bring their own.';

	/// en: 'Provider costs'
	String get provider_costs_title => 'Provider costs';

	/// en: 'No prices yet'
	String get provider_costs_empty_title => 'No prices yet';

	/// en: 'Add the real API price per model so monthly profit is computed on actual cost.'
	String get provider_costs_empty_desc => 'Add the real API price per model so monthly profit is computed on actual cost.';

	/// en: 'Add price'
	String get provider_cost_add_cta => 'Add price';

	/// en: 'New provider price'
	String get provider_cost_add_title => 'New provider price';

	/// en: 'Fix provider price'
	String get provider_cost_edit_title => 'Fix provider price';

	/// en: 'Provider'
	String get provider_cost_provider_label => 'Provider';

	/// en: 'Model'
	String get provider_cost_model_label => 'Model';

	/// en: 'Input / 1M tokens'
	String get provider_cost_input_label => 'Input / 1M tokens';

	/// en: 'Output / 1M tokens'
	String get provider_cost_output_label => 'Output / 1M tokens';

	/// en: 'in $input¢ / out $output¢ USD per 1M tokens'
	String provider_cost_costs_line({required Object input, required Object output}) => 'in ${input}¢ / out ${output}¢ USD per 1M tokens';

	/// en: 'Effective from $date'
	String provider_cost_effective_from({required Object date}) => 'Effective from ${date}';

	/// en: 'Change date'
	String get provider_cost_pick_date => 'Change date';

	/// en: 'Fill every field with valid values.'
	String get provider_cost_form_invalid => 'Fill every field with valid values.';

	/// en: 'Price saved.'
	String get provider_cost_saved_toast => 'Price saved.';

	/// en: 'Delete price?'
	String get provider_cost_delete_title => 'Delete price?';

	/// en: 'Remove the $model price row? Past profit computations that used it will pick the previous effective price.'
	String provider_cost_delete_body({required Object model}) => 'Remove the ${model} price row? Past profit computations that used it will pick the previous effective price.';

	/// en: 'USD → BRL rate'
	String get exchange_rate_title => 'USD → BRL rate';

	/// en: 'R$ $rate per US$ 1'
	String exchange_rate_value({required Object rate}) => 'R\$ ${rate} per US\$ 1';

	/// en: 'Edit'
	String get exchange_rate_edit_cta => 'Edit';

	/// en: 'BRL per 1 USD'
	String get exchange_rate_label => 'BRL per 1 USD';

	/// en: 'Rate saved.'
	String get exchange_rate_saved => 'Rate saved.';

	/// en: 'Seller payouts'
	String get payouts_title => 'Seller payouts';

	/// en: 'Cycle'
	String get payouts_cycle_label => 'Cycle';

	/// en: 'Total due'
	String get payouts_total_due => 'Total due';

	/// en: 'Total paid'
	String get payouts_total_paid => 'Total paid';

	/// en: 'No payouts in this cycle'
	String get payouts_empty_title => 'No payouts in this cycle';

	/// en: 'No commission entries were found for this cycle.'
	String get payouts_empty_desc => 'No commission entries were found for this cycle.';

	/// en: 'Pending (cycle): ${amount}'
	String payouts_pending_line({required Object amount}) => 'Pending (cycle): ${amount}';

	/// en: 'Adjustments from earlier cycles: ${amount} (${count} entries)'
	String payouts_adjustment_line({required Object amount, required Object count}) => 'Adjustments from earlier cycles: ${amount} (${count} entries)';

	/// en: 'Paid in this cycle: ${amount}'
	String payouts_paid_line({required Object amount}) => 'Paid in this cycle: ${amount}';

	/// en: 'Total due: ${amount}'
	String payouts_total_due_line({required Object amount}) => 'Total due: ${amount}';

	/// en: 'Copy PIX key'
	String get payouts_pix_label => 'Copy PIX key';

	/// en: 'No PIX key registered'
	String get payouts_pix_missing => 'No PIX key registered';

	/// en: 'PIX key copied.'
	String get payouts_pix_copied => 'PIX key copied.';

	/// en: 'Settled'
	String get payouts_settled_chip => 'Settled';

	/// en: 'Mark as paid'
	String get payouts_mark_paid_cta => 'Mark as paid';

	/// en: 'Confirm payout'
	String get payouts_mark_paid_title => 'Confirm payout';

	/// en: 'Settle ${amount} to ${name}? All pending entries through this cycle become paid.'
	String payouts_mark_paid_body({required Object amount, required Object name}) => 'Settle ${amount} to ${name}? All pending entries through this cycle become paid.';

	/// en: 'PIX transaction # (optional)'
	String get payouts_note_label => 'PIX transaction # (optional)';

	/// en: 'Payout marked as paid.'
	String get payouts_marked_toast => 'Payout marked as paid.';

	/// en: 'Cycle still open'
	String get payouts_open_cycle_title => 'Cycle still open';

	/// en: 'This cycle hasn't closed yet — you'd be settling a partial amount, and any later recompute becomes an adjustment. Pay the open cycle anyway?'
	String get payouts_open_cycle_body => 'This cycle hasn\'t closed yet — you\'d be settling a partial amount, and any later recompute becomes an adjustment. Pay the open cycle anyway?';

	/// en: 'Pay open cycle'
	String get payouts_open_cycle_cta => 'Pay open cycle';

	/// en: 'Export CSV'
	String get payouts_export_cta => 'Export CSV';

	/// en: 'CSV copied to clipboard.'
	String get payouts_export_copied => 'CSV copied to clipboard.';

	/// en: 'Sellers'
	String get sellers_title => 'Sellers';

	/// en: 'Platform account users. Turning the seller toggle on flags the user, mints the signup link and joins the lead rotation — all at once.'
	String get sellers_subtitle => 'Platform account users. Turning the seller toggle on flags the user, mints the signup link and joins the lead rotation — all at once.';

	/// en: 'No platform users'
	String get sellers_empty_title => 'No platform users';

	/// en: 'Create attendants in Settings → Team first, then flag them as sellers here.'
	String get sellers_empty_desc => 'Create attendants in Settings → Team first, then flag them as sellers here.';

	/// en: 'Is seller'
	String get sellers_is_seller_label => 'Is seller';

	/// en: 'Active (commission SLA)'
	String get sellers_active_label => 'Active (commission SLA)';

	/// en: 'In rotation'
	String get sellers_in_rotation_chip => 'In rotation';

	/// en: 'Out of rotation'
	String get sellers_out_rotation_chip => 'Out of rotation';

	/// en: 'Copy signup link'
	String get sellers_ref_link_label => 'Copy signup link';

	/// en: 'Signup link copied.'
	String get sellers_link_copied => 'Signup link copied.';

	/// en: 'No PIX key registered'
	String get sellers_pix_missing => 'No PIX key registered';

	/// en: '${count} account(s) in portfolio'
	String sellers_carteira_line({required Object count}) => '${count} account(s) in portfolio';

	/// en: 'Seller onboarded — flag, link and rotation set.'
	String get sellers_onboarded_toast => 'Seller onboarded — flag, link and rotation set.';

	/// en: 'Seller offboarded — removed from rotation; history kept.'
	String get sellers_offboarded_toast => 'Seller offboarded — removed from rotation; history kept.';

	/// en: 'Activity updated.'
	String get sellers_activity_toast => 'Activity updated.';

	/// en: 'Commission an admin?'
	String get sellers_admin_warn_title => 'Commission an admin?';

	/// en: '${name} is an admin — flagging them as a seller generates commission and SLA duties for them. Continue?'
	String sellers_admin_warn_body({required Object name}) => '${name} is an admin — flagging them as a seller generates commission and SLA duties for them. Continue?';

	/// en: 'Flag as seller'
	String get sellers_admin_warn_confirm => 'Flag as seller';

	/// en: 'Invite customer'
	String get invite_title => 'Invite customer';

	/// en: 'Customer e-mail'
	String get invite_email => 'Customer e-mail';

	/// en: 'Contracted plan'
	String get invite_plan => 'Contracted plan';

	/// en: 'Valid for (days)'
	String get invite_days => 'Valid for (days)';

	/// en: 'Generate invite'
	String get invite_create => 'Generate invite';

	/// en: 'The account can only be created with this link, by this e-mail, and is born on the chosen plan.'
	String get invite_hint => 'The account can only be created with this link, by this e-mail, and is born on the chosen plan.';

	/// en: 'Invite link — shown once'
	String get invite_link_title => 'Invite link — shown once';

	/// en: 'Copy it NOW and send it over the channel where the deal happened. The server keeps only a hash: closing this dialog loses the link (revoke and reissue).'
	String get invite_link_hint => 'Copy it NOW and send it over the channel where the deal happened. The server keeps only a hash: closing this dialog loses the link (revoke and reissue).';

	/// en: 'Copy link'
	String get invite_copy => 'Copy link';

	/// en: 'Link copied.'
	String get invite_copied => 'Link copied.';
}

// Path: sales
class Translations$sales$en {
	Translations$sales$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Active Sales'
	String get nav_tooltip => 'Active Sales';

	/// en: 'Back'
	String get back_tooltip => 'Back';

	/// en: 'New funnel'
	String get funnel_new_title => 'New funnel';

	/// en: 'Funnel'
	String get funnel_detail_title => 'Funnel';

	late final Translations$sales$products$en products = Translations$sales$products$en.internal(_root);
	late final Translations$sales$funnels$en funnels = Translations$sales$funnels$en.internal(_root);
	late final Translations$sales$schema$en schema = Translations$sales$schema$en.internal(_root);
	late final Translations$sales$webhook$en webhook = Translations$sales$webhook$en.internal(_root);
	late final Translations$sales$lead_profile$en lead_profile = Translations$sales$lead_profile$en.internal(_root);
	late final Translations$sales$leads$en leads = Translations$sales$leads$en.internal(_root);
	late final Translations$sales$prospecting$en prospecting = Translations$sales$prospecting$en.internal(_root);

	/// en: 'Hunt'
	String get hunt_tab => 'Hunt';
}

// Path: support
class Translations$support$en {
	Translations$support$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Support'
	String get title => 'Support';

	/// en: 'Your Replider account manager'
	String get subtitle => 'Your Replider account manager';

	/// en: 'Support'
	String get nav_tooltip => 'Support';

	/// en: 'Talk to us'
	String get empty_title => 'Talk to us';

	/// en: 'Questions about your account, channels or billing? Send a message — our team replies right here.'
	String get empty_subtitle => 'Questions about your account, channels or billing? Send a message — our team replies right here.';

	/// en: 'Type a message...'
	String get input_hint => 'Type a message...';

	/// en: 'Could not load your support conversation.'
	String get load_error => 'Could not load your support conversation.';

	/// en: 'Try again'
	String get retry => 'Try again';

	/// en: 'Get notified about replies'
	String get push_prompt_title => 'Get notified about replies';

	/// en: 'Enable notifications and we'll ping you when your account manager answers, even with the dashboard closed.'
	String get push_prompt_body => 'Enable notifications and we\'ll ping you when your account manager answers, even with the dashboard closed.';

	/// en: 'Enable notifications'
	String get push_prompt_enable => 'Enable notifications';

	/// en: 'Not now'
	String get push_prompt_later => 'Not now';
}

// Path: claude_integration
class Translations$claude_integration$en {
	Translations$claude_integration$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Claude (AI)'
	String get title => 'Claude (AI)';

	/// en: 'Back'
	String get back_tooltip => 'Back';

	/// en: 'Failed to load: $error'
	String load_failed({required Object error}) => 'Failed to load: ${error}';

	/// en: 'Copy'
	String get copy_tooltip => 'Copy';

	/// en: 'Copied.'
	String get copied => 'Copied.';

	/// en: 'Connect to Claude'
	String get connect_title => 'Connect to Claude';

	/// en: 'Manage your account by chatting with Claude — configure the agent, reply to customers, view metrics. Works on claude.ai and the mobile/desktop apps.'
	String get connect_subtitle => 'Manage your account by chatting with Claude — configure the agent, reply to customers, view metrics. Works on claude.ai and the mobile/desktop apps.';

	/// en: 'Access tokens'
	String get tokens_title => 'Access tokens';

	/// en: 'Personal tokens for Claude Code. Replace SEU_TOKEN in the command above. The full token is shown only once.'
	String get tokens_subtitle => 'Personal tokens for Claude Code. Replace SEU_TOKEN in the command above. The full token is shown only once.';

	/// en: 'Create token'
	String get tokens_create => 'Create token';

	/// en: 'No tokens created.'
	String get tokens_empty => 'No tokens created.';

	/// en: 'never used'
	String get tokens_never_used => 'never used';

	/// en: 'used on $date'
	String tokens_last_used({required Object date}) => 'used on ${date}';

	/// en: 'Revoke token'
	String get tokens_revoke_tooltip => 'Revoke token';

	/// en: 'New access token'
	String get create_dialog_title => 'New access token';

	/// en: 'Name (e.g. Tiago's Claude Code)'
	String get create_dialog_name_label => 'Name (e.g. Tiago\'s Claude Code)';

	/// en: 'Create'
	String get create_dialog_confirm => 'Create';

	/// en: 'Copy the token now'
	String get token_dialog_title => 'Copy the token now';

	/// en: 'This is the only time the token will be shown. Copy it into Claude — it cannot be recovered later.'
	String get token_dialog_body => 'This is the only time the token will be shown. Copy it into Claude — it cannot be recovered later.';

	/// en: 'Copy'
	String get token_dialog_copy => 'Copy';

	/// en: 'Token copied.'
	String get token_copied => 'Token copied.';

	/// en: 'Done'
	String get token_dialog_done => 'Done';

	/// en: 'Revoke token?'
	String get revoke_token_title => 'Revoke token?';

	/// en: 'Claude connected with this token will lose access immediately.'
	String get revoke_token_body => 'Claude connected with this token will lose access immediately.';

	/// en: 'Revoke'
	String get revoke_confirm => 'Revoke';

	/// en: 'Active connections'
	String get connections_title => 'Active connections';

	/// en: 'Apps authorized via OAuth (e.g. the claude.ai connector).'
	String get connections_subtitle => 'Apps authorized via OAuth (e.g. the claude.ai connector).';

	/// en: 'No active connections.'
	String get connections_empty => 'No active connections.';

	/// en: 'Revoke'
	String get connections_revoke => 'Revoke';

	/// en: 'Revoke connection?'
	String get revoke_connection_title => 'Revoke connection?';

	/// en: 'This app will lose access to your Replider account immediately.'
	String get revoke_connection_body => 'This app will lose access to your Replider account immediately.';

	/// en: 'Copy the address below.'
	String get connect_step1 => 'Copy the address below.';

	/// en: 'In Claude, go to Settings → Connectors → Add custom connector and paste the address.'
	String get connect_step2 => 'In Claude, go to Settings → Connectors → Add custom connector and paste the address.';

	/// en: 'Click connect and sign in with the same email and password you use here on Replider.'
	String get connect_step3 => 'Click connect and sign in with the same email and password you use here on Replider.';

	/// en: 'No token or install needed — just sign in with your email and password. (Custom connectors require a Claude Pro, Max, Team or Enterprise plan.)'
	String get connect_no_token_note => 'No token or install needed — just sign in with your email and password. (Custom connectors require a Claude Pro, Max, Team or Enterprise plan.)';

	/// en: 'Advanced — Claude Code'
	String get advanced_title => 'Advanced — Claude Code';

	/// en: 'For people using Claude Code in the terminal. If you use Claude on the web or apps, skip this section.'
	String get advanced_subtitle => 'For people using Claude Code in the terminal. If you use Claude on the web or apps, skip this section.';

	/// en: 'Recommended — browser login (no token)'
	String get cc_oauth_label => 'Recommended — browser login (no token)';

	/// en: 'Run the command in the terminal. Claude Code opens the browser, you sign in with your Replider email and password, and it returns to the terminal automatically.'
	String get cc_oauth_hint => 'Run the command in the terminal. Claude Code opens the browser, you sign in with your Replider email and password, and it returns to the terminal automatically.';

	/// en: 'Alternative — with token (for server/CI, no browser)'
	String get cc_token_label => 'Alternative — with token (for server/CI, no browser)';

	/// en: 'Use when no browser is available. Create a token below and replace SEU_TOKEN in the command.'
	String get cc_token_hint => 'Use when no browser is available. Create a token below and replace SEU_TOKEN in the command.';
}

// Path: shell
class Translations$shell$en {
	Translations$shell$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Home'
	String get start => 'Home';

	/// en: 'Customers'
	String get funnel => 'Customers';

	/// en: 'Conversations'
	String get conversations => 'Conversations';

	/// en: 'Opportunities'
	String get offers => 'Opportunities';

	/// en: 'Copilot'
	String get copilot => 'Copilot';

	/// en: 'Copilot coming soon'
	String get copilot_soon_title => 'Copilot coming soon';

	/// en: 'For now, talk to Replider through the MCP connector in Claude or ChatGPT — the same actions live there.'
	String get copilot_soon_body => 'For now, talk to Replider through the MCP connector in Claude or ChatGPT — the same actions live there.';

	/// en: 'Your day'
	String get start_title => 'Your day';

	/// en: 'Shortcuts to where the work happens.'
	String get start_subtitle => 'Shortcuts to where the work happens.';

	/// en: 'Open the pipeline'
	String get start_go_funnel => 'Open the pipeline';

	/// en: 'Open conversations'
	String get start_go_conversations => 'Open conversations';

	/// en: 'Find companies'
	String get start_go_prospecting => 'Find companies';

	/// en: 'Active leads'
	String get stat_active_leads => 'Active leads';

	/// en: 'Waiting approval'
	String get stat_pending_approval => 'Waiting approval';

	/// en: 'Unassigned'
	String get stat_unowned => 'Unassigned';

	/// en: 'NEEDS YOU'
	String get needs_you_title => 'NEEDS YOU';

	/// en: 'Nothing is waiting on you right now.'
	String get needs_you_clear => 'Nothing is waiting on you right now.';

	/// en: 'Open'
	String get needs_open => 'Open';

	/// en: '$count approach(es) waiting on your approval'
	String needs_approvals({required Object count}) => '${count} approach(es) waiting on your approval';

	/// en: '$count lead(s) with no owner — nobody is working them'
	String needs_unowned({required Object count}) => '${count} lead(s) with no owner — nobody is working them';

	/// en: 'Your pipeline is empty. Start by hunting for clients.'
	String get needs_empty_pipeline => 'Your pipeline is empty. Start by hunting for clients.';
}

// Path: board
class Translations$board$en {
	Translations$board$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Move to'
	String get move_to => 'Move to';

	/// en: 'Score'
	String get score => 'Score';

	/// en: 'Unassigned'
	String get unassigned => 'Unassigned';

	/// en: 'No leads here'
	String get empty_column => 'No leads here';

	/// en: 'This funnel has no stages yet.'
	String get empty_board => 'This funnel has no stages yet.';

	/// en: 'Pick a funnel'
	String get select_funnel => 'Pick a funnel';

	/// en: 'Pipeline'
	String get board_title => 'Pipeline';

	/// en: 'New lead'
	String get new_lead => 'New lead';

	/// en: 'Contact'
	String get new_lead_name => 'Contact';

	/// en: 'Who decides or buys'
	String get new_lead_name_hint => 'Who decides or buys';

	/// en: 'Phone / WhatsApp'
	String get new_lead_phone => 'Phone / WhatsApp';

	/// en: '+55 65 99999-0000'
	String get new_lead_phone_hint => '+55 65 99999-0000';

	/// en: 'Lead added to the funnel.'
	String get new_lead_created => 'Lead added to the funnel.';

	/// en: 'That contact is already in the funnel.'
	String get new_lead_existed => 'That contact is already in the funnel.';

	/// en: 'You don't have a funnel yet.'
	String get no_funnel_yet => 'You don\'t have a funnel yet.';

	/// en: 'Search company, city or contact'
	String get search_hint => 'Search company, city or contact';

	/// en: 'Clear search'
	String get search_clear => 'Clear search';

	/// en: 'All · $count'
	String filter_all({required Object count}) => 'All · ${count}';

	/// en: 'Replied · $count'
	String filter_responded({required Object count}) => 'Replied · ${count}';

	/// en: 'Awaiting approval · $count'
	String filter_awaiting({required Object count}) => 'Awaiting approval · ${count}';

	/// en: 'Idle for over 7 days · $count'
	String filter_stalled({required Object count}) => 'Idle for over 7 days · ${count}';

	/// en: '$showing of $total · longest idle first'
	String column_showing({required Object showing, required Object total}) => '${showing} of ${total} · longest idle first';

	/// en: '$showing of $total · manual order'
	String column_showing_manual({required Object showing, required Object total}) => '${showing} of ${total} · manual order';

	/// en: 'Sort by idle'
	String get order_reset => 'Sort by idle';

	/// en: 'Nothing matching “$query” in this stage.'
	String column_no_match({required Object query}) => 'Nothing matching “${query}” in this stage.';

	/// en: 'It may be in another stage, or not imported yet.'
	String get column_no_match_hint => 'It may be in another stage, or not imported yet.';

	/// en: 'Showing this funnel's $count most recent leads. Search covers only those.'
	String truncated_notice({required Object count}) => 'Showing this funnel\'s ${count} most recent leads. Search covers only those.';

	/// en: '$count d idle'
	String idle_days({required Object count}) => '${count} d idle';

	/// en: 'Moved today'
	String get idle_today => 'Moved today';

	/// en: 'No recorded activity'
	String get idle_unknown => 'No recorded activity';

	/// en: 'Pipeline'
	String get view_pipeline => 'Pipeline';

	/// en: 'Draft in Today →'
	String get tag_draft_ready => 'Draft in Today →';

	/// en: 'Wants to buy'
	String get tag_buy_intent => 'Wants to buy';

	/// en: 'Qualified'
	String get tag_qualified => 'Qualified';

	/// en: 'Going cold'
	String get tag_idle_risk => 'Going cold';

	/// en: 'Required'
	String get field_required => 'Required';

	/// en: 'Funnel fields'
	String get funnel_fields => 'Funnel fields';

	/// en: 'Field saved'
	String get funnel_fields_saved => 'Field saved';

	/// en: 'This funnel asks for no extra fields.'
	String get funnel_fields_empty => 'This funnel asks for no extra fields.';

	/// en: 'Owner'
	String get owner => 'Owner';

	/// en: 'Nobody yet'
	String get owner_none => 'Nobody yet';

	/// en: 'Set owner'
	String get owner_assign => 'Set owner';

	/// en: 'Visibility'
	String get visibility => 'Visibility';

	/// en: 'Whole team'
	String get visibility_team => 'Whole team';

	/// en: 'Owner only'
	String get visibility_private => 'Owner only';

	/// en: 'Disappears from the pipeline of anyone who is not the owner.'
	String get visibility_private_hint => 'Disappears from the pipeline of anyone who is not the owner.';

	/// en: 'Where this came from'
	String get sources => 'Where this came from';

	/// en: 'The AI built this profile from these pages.'
	String get sources_hint => 'The AI built this profile from these pages.';

	/// en: 'Assign the ownerless'
	String get bulk_assign => 'Assign the ownerless';

	/// en: 'Assign leads with no owner'
	String get bulk_assign_title => 'Assign leads with no owner';

	/// en: '$count leads in this funnel have no owner. Hand them all to:'
	String bulk_assign_body({required Object count}) => '${count} leads in this funnel have no owner. Hand them all to:';

	/// en: 'Counts only what the current search and filter show.'
	String get bulk_assign_filtered => 'Counts only what the current search and filter show.';

	/// en: 'Assign $count'
	String bulk_assign_confirm({required Object count}) => 'Assign ${count}';

	/// en: '$count leads assigned'
	String bulk_assign_done({required Object count}) => '${count} leads assigned';

	/// en: '$count assigned · $skipped were no longer there'
	String bulk_assign_partial({required Object count, required Object skipped}) => '${count} assigned · ${skipped} were no longer there';

	/// en: '200 at a time, max. The first 200 will be assigned.'
	String get bulk_assign_cap => '200 at a time, max. The first 200 will be assigned.';

	/// en: 'Assign in this search'
	String get bulk_assign_here => 'Assign in this search';

	/// en: 'The company, and the person you talk to there.'
	String get new_lead_subtitle => 'The company, and the person you talk to there.';

	/// en: 'Company'
	String get new_lead_company => 'Company';

	/// en: 'e.g. Cuiabá Regional Hospital'
	String get new_lead_company_hint => 'e.g. Cuiabá Regional Hospital';

	/// en: 'Enter the company.'
	String get new_lead_company_required => 'Enter the company.';

	/// en: 'Role'
	String get new_lead_role => 'Role';

	/// en: 'e.g. Purchasing manager'
	String get new_lead_role_hint => 'e.g. Purchasing manager';

	/// en: 'Email'
	String get new_lead_email => 'Email';

	/// en: 'purchasing@hospital.com'
	String get new_lead_email_hint => 'purchasing@hospital.com';

	/// en: 'City'
	String get new_lead_city => 'City';

	/// en: 'e.g. Cuiabá, MT'
	String get new_lead_city_hint => 'e.g. Cuiabá, MT';

	/// en: 'Phone or email — at least one, so the offer has a way out.'
	String get new_lead_reach_help => 'Phone or email — at least one, so the offer has a way out.';

	/// en: 'Enter a phone or an email.'
	String get new_lead_reach_required => 'Enter a phone or an email.';

	/// en: 'Add'
	String get new_lead_submit => 'Add';

	/// en: 'Configure stages'
	String get configure_stages => 'Configure stages';

	/// en: 'Replied'
	String get tag_responded => 'Replied';

	/// en: 'Leads with status "$status". They move on send and on reply — or drag them.'
	String column_hint({required Object status}) => 'Leads with status "${status}". They move on send and on reply — or drag them.';

	/// en: 'Final stage — leads with status "$status".'
	String column_hint_terminal({required Object status}) => 'Final stage — leads with status "${status}".';

	/// en: 'Open draft in Today'
	String get panel_open_in_queue => 'Open draft in Today';

	/// en: 'CNES installed base'
	String get panel_cnes_title => 'CNES installed base';

	/// en: 'public data · matched by phone · counts, not age'
	String get panel_cnes_by_phone => 'public data · matched by phone · counts, not age';

	/// en: 'public data · unique name match · counts, not age'
	String get panel_cnes_by_name => 'public data · unique name match · counts, not age';

	/// en: 'CNES data from $month'
	String panel_cnes_month({required Object month}) => 'CNES data from ${month}';

	/// en: 'CNES code $code'
	String panel_cnes_code({required Object code}) => 'CNES code ${code}';

	/// en: 'What you can offer'
	String get panel_offer_title => 'What you can offer';

	/// en: '$product — they have $count $unit of $equipment'
	String panel_offer_line({required Object product, required Object count, required Object unit, required Object equipment}) => '${product} — they have ${count} ${unit} of ${equipment}';

	/// en: 'No product in your catalogue matches the declared park'
	String get panel_offer_none => 'No product in your catalogue matches the declared park';

	/// en: 'your catalogue crossed with the registry — no AI'
	String get panel_offer_free => 'your catalogue crossed with the registry — no AI';

	/// en: ' — they have $count $unit of $equipment'
	String panel_offer_tail({required Object count, required Object unit, required Object equipment}) => ' — they have ${count} ${unit} of ${equipment}';

	/// en: 'Open $product'
	String panel_offer_open({required Object product}) => 'Open ${product}';
}

// Path: lead_detail
class Translations$lead_detail$en {
	Translations$lead_detail$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Profile'
	String get profile => 'Profile';

	/// en: 'Approach'
	String get approach => 'Approach';

	/// en: 'Write or generate the approach message…'
	String get approach_hint => 'Write or generate the approach message…';

	/// en: 'Generate another'
	String get regenerate => 'Generate another';

	/// en: 'Generate approach'
	String get generate => 'Generate approach';

	/// en: 'Approve and send'
	String get approve_send => 'Approve and send';

	/// en: 'Timeline'
	String get timeline => 'Timeline';

	/// en: 'Confirm send'
	String get confirm_title => 'Confirm send';

	/// en: 'To'
	String get confirm_to => 'To';

	/// en: 'Approach sent.'
	String get sent => 'Approach sent.';

	/// en: 'You can view this lead, but it belongs to another rep.'
	String get read_only => 'You can view this lead, but it belongs to another rep.';

	/// en: 'Approval queue'
	String get queue_title => 'Approval queue';

	/// en: 'Nothing to approve right now.'
	String get queue_empty => 'Nothing to approve right now.';

	/// en: 'When the AI drafts approaches, they wait for your approval here.'
	String get queue_empty_hint => 'When the AI drafts approaches, they wait for your approval here.';

	/// en: '$count remaining'
	String queue_remaining({required Object count}) => '${count} remaining';

	/// en: 'Discard'
	String get discard => 'Discard';

	/// en: 'Edit'
	String get edit => 'Edit';

	/// en: 'Draft discarded — the lead goes back to the pipeline.'
	String get discarded => 'Draft discarded — the lead goes back to the pipeline.';

	/// en: 'Undo'
	String get undo => 'Undo';

	/// en: 'Goes out via'
	String get send_channel => 'Goes out via';

	/// en: 'WhatsApp needs an approved template — open the lead'
	String get bulk_needs_template => 'WhatsApp needs an approved template — open the lead';

	/// en: 'Automatic check'
	String get check_title => 'Automatic check';

	/// en: 'No pattern promising a deadline, a price or a result was found in this text.'
	String get check_clean => 'No pattern promising a deadline, a price or a result was found in this text.';

	/// en: '$count passage(s) to look at before sending.'
	String check_found({required Object count}) => '${count} passage(s) to look at before sending.';

	/// en: 'You edited the text. The check below is of the saved draft — this text has not been checked.'
	String get check_edited => 'You edited the text. The check below is of the saved draft — this text has not been checked.';

	/// en: 'A commitment made in your name'
	String get check_kind_promise => 'A commitment made in your name';

	/// en: 'A claim about the customer with nothing behind it'
	String get check_kind_invented_fact => 'A claim about the customer with nothing behind it';

	/// en: 'Outside what the company said it does'
	String get check_kind_out_of_scope => 'Outside what the company said it does';

	/// en: 'A fill-in field left behind'
	String get check_kind_placeholder => 'A fill-in field left behind';

	/// en: 'Tone that does not fit the context'
	String get check_kind_tone => 'Tone that does not fit the context';

	/// en: 'Review with AI'
	String get review_action => 'Review with AI';

	/// en: 'Reading the message…'
	String get review_running => 'Reading the message…';

	/// en: 'Reviewed by a second AI: nothing to flag.'
	String get review_clean => 'Reviewed by a second AI: nothing to flag.';

	/// en: 'The second AI flagged $count point(s).'
	String review_found({required Object count}) => 'The second AI flagged ${count} point(s).';

	/// en: 'The AI review did not run. The automatic check above still stands.'
	String get review_unavailable => 'The AI review did not run. The automatic check above still stands.';

	/// en: 'Written by AI from: $source'
	String written_by_ai({required Object source}) => 'Written by AI from: ${source}';

	/// en: 'Written by AI'
	String get written_by_ai_unknown => 'Written by AI';

	/// en: 'approve'
	String get shortcut_approve => 'approve';

	/// en: 'edit'
	String get shortcut_edit => 'edit';

	/// en: 'discard'
	String get shortcut_discard => 'discard';

	/// en: 'move'
	String get shortcut_move => 'move';

	/// en: 'Contact'
	String get contact => 'Contact';

	/// en: 'Call'
	String get call => 'Call';

	/// en: 'Open conversation'
	String get open_conversation => 'Open conversation';

	/// en: 'No conversation with this contact yet.'
	String get no_conversation => 'No conversation with this contact yet.';

	/// en: 'Research this company'
	String get enrich => 'Research this company';

	/// en: 'Research again'
	String get enrich_again => 'Research again';

	/// en: 'Researching…'
	String get enriching => 'Researching…';

	/// en: 'Researching the company. Check back shortly.'
	String get enrich_started => 'Researching the company. Check back shortly.';

	/// en: 'We know nothing about this company yet.'
	String get profile_empty => 'We know nothing about this company yet.';

	/// en: 'Research looks up the decision maker, the line of business and the address, so the approach can cite something real instead of reading as a template.'
	String get profile_empty_hint => 'Research looks up the decision maker, the line of business and the address, so the approach can cite something real instead of reading as a template.';

	/// en: 'Researching this company now. The result shows up here.'
	String get profile_researching => 'Researching this company now. The result shows up here.';

	/// en: 'The research found no public data about this company.'
	String get profile_found_nothing => 'The research found no public data about this company.';

	/// en: 'That happens with small companies and ones with no online presence. Write the approach with what you already know, or research again later.'
	String get profile_found_nothing_hint => 'That happens with small companies and ones with no online presence. Write the approach with what you already know, or research again later.';

	/// en: 'Nothing to research: this lead has no company name and no website.'
	String get profile_blocked => 'Nothing to research: this lead has no company name and no website.';

	/// en: 'The search starts from the name or the website to find the decision maker. With neither, there is nothing to look for.'
	String get profile_blocked_hint => 'The search starts from the name or the website to find the decision maker. With neither, there is nothing to look for.';

	/// en: 'Conversation summary'
	String get summary => 'Conversation summary';

	/// en: 'Reading the conversation…'
	String get summary_generating => 'Reading the conversation…';

	/// en: 'Too short to summarise.'
	String get summary_short => 'Too short to summarise.';

	/// en: 'Could not summarise right now.'
	String get summary_failed => 'Could not summarise right now.';

	/// en: 'Website'
	String get website => 'Website';

	/// en: 'The text says a file is attached, and none is'
	String get check_kind_missing_attachment => 'The text says a file is attached, and none is';

	/// en: 'Fill in the field before sending.'
	String get blocked_placeholder => 'Fill in the field before sending.';

	/// en: 'Subject'
	String get subject => 'Subject';

	/// en: 'What the recipient sees before opening'
	String get subject_hint => 'What the recipient sees before opening';

	/// en: 'Use a template'
	String get use_template => 'Use a template';

	/// en: 'reply'
	String get reply_rate_short => 'reply';

	/// en: '$count selected'
	String bulk_selected({required Object count}) => '${count} selected';

	/// en: 'Send selected'
	String get bulk_send => 'Send selected';

	/// en: 'Send $count approaches?'
	String bulk_confirm_title({required Object count}) => 'Send ${count} approaches?';

	/// en: 'Review $count approaches'
	String bulk_review_title({required Object count}) => 'Review ${count} approaches';

	/// en: 'Untick whatever should not go. Each text is sent exactly as it reads here.'
	String get bulk_review_hint => 'Untick whatever should not go. Each text is sent exactly as it reads here.';

	/// en: 'Send $count'
	String bulk_review_send({required Object count}) => 'Send ${count}';

	/// en: 'Nothing ticked'
	String get bulk_review_none => 'Nothing ticked';

	/// en: '$count approaches sent'
	String bulk_sent({required Object count}) => '${count} approaches sent';

	/// en: '$count sent. Failed: $failed'
	String bulk_partial({required Object count, required Object failed}) => '${count} sent. Failed: ${failed}';

	/// en: '$count fact(s) about the company'
	String profile_facts_count({required Object count}) => '${count} fact(s) about the company';

	/// en: 'Nothing researched yet'
	String get profile_nothing_yet => 'Nothing researched yet';

	/// en: '$count event(s)'
	String timeline_events({required Object count}) => '${count} event(s)';

	/// en: 'Open'
	String get section_expand => 'Open';

	/// en: 'Close'
	String get section_collapse => 'Close';

	/// en: 'All'
	String get queue_filter_all => 'All';

	/// en: 'No funnel'
	String get queue_no_funnel => 'No funnel';

	/// en: 'Oldest first'
	String get queue_sort_oldest => 'Oldest first';

	/// en: 'Newest first'
	String get queue_sort_newest => 'Newest first';

	/// en: 'Flagged only'
	String get queue_flagged_only => 'Flagged only';

	/// en: 'Showing $shown of $total. Pick a funnel to work through the rest.'
	String queue_partial({required Object shown, required Object total}) => 'Showing ${shown} of ${total}. Pick a funnel to work through the rest.';

	/// en: 'Nothing in this selection.'
	String get queue_filtered_empty => 'Nothing in this selection.';

	/// en: 'The other drafts are still queued — clear the filter to see them.'
	String get queue_filtered_empty_hint => 'The other drafts are still queued — clear the filter to see them.';

	/// en: 'queued $count min'
	String queue_waiting_minutes({required Object count}) => 'queued ${count} min';

	/// en: 'queued ${count}h'
	String queue_waiting_hours({required Object count}) => 'queued ${count}h';

	/// en: 'queued ${count}d'
	String queue_waiting_days({required Object count}) => 'queued ${count}d';

	/// en: 'Find companies'
	String get queue_empty_cta => 'Find companies';

	/// en: 'Mailbox $mailbox warming up (day $day of $total): $remaining cold sends left today. The cap grows daily to protect your domain.'
	String email_warmup_banner_named({required Object mailbox, required Object day, required Object total, required Object remaining}) => 'Mailbox ${mailbox} warming up (day ${day} of ${total}): ${remaining} cold sends left today. The cap grows daily to protect your domain.';

	/// en: 'Mailbox $mailbox used up today's cold sends (day $day of $total of the warm-up). New approvals go out tomorrow.'
	String email_warmup_exhausted({required Object mailbox, required Object day, required Object total}) => 'Mailbox ${mailbox} used up today\'s cold sends (day ${day} of ${total} of the warm-up). New approvals go out tomorrow.';

	/// en: 'Company'
	String get field_company => 'Company';

	/// en: 'Website'
	String get field_website => 'Website';

	/// en: 'Decision maker'
	String get field_decision_maker => 'Decision maker';

	/// en: 'Role'
	String get field_role => 'Role';

	/// en: 'Address'
	String get field_address => 'Address';

	/// en: 'Activity'
	String get field_activity => 'Activity';

	/// en: 'Subject for this batch's emails'
	String get bulk_subject_hint => 'Subject for this batch\'s emails';

	/// en: 'Add a subject to send'
	String get bulk_subject_missing => 'Add a subject to send';

	/// en: 'Configure'
	String get error_action_configure => 'Configure';

	/// en: 'This lead isn't awaiting approval — generate a draft to put it back in the queue.'
	String get approve_needs_queue_status => 'This lead isn\'t awaiting approval — generate a draft to put it back in the queue.';

	/// en: 'This approach was already sent — the text is locked so history stays true.'
	String get approach_locked_after_send => 'This approach was already sent — the text is locked so history stays true.';

	/// en: 'The research is taking a while — it keeps running in the background; check back later.'
	String get research_still_running => 'The research is taking a while — it keeps running in the background; check back later.';

	/// en: 'Created on $date'
	String timeline_created({required Object date}) => 'Created on ${date}';

	/// en: 'Qualified on $date'
	String timeline_qualified({required Object date}) => 'Qualified on ${date}';

	/// en: 'Signalled buying intent on $date'
	String timeline_buy_intent({required Object date}) => 'Signalled buying intent on ${date}';

	/// en: 'Researched on $date'
	String timeline_researched({required Object date}) => 'Researched on ${date}';

	/// en: 'Last updated on $date'
	String timeline_updated({required Object date}) => 'Last updated on ${date}';

	/// en: 'From your book'
	String get queue_group_carteira => 'From your book';

	/// en: 'From a radar signal'
	String get queue_group_sinal => 'From a radar signal';

	/// en: 'Cold, from the territory'
	String get queue_group_frio => 'Cold, from the territory';

	/// en: 'Discard all $count'
	String queue_frio_discard_all({required Object count}) => 'Discard all ${count}';

	/// en: 'Rewrite with the catalog'
	String get queue_frio_redraft_all => 'Rewrite with the catalog';

	/// en: '+ $count more like these · show all'
	String queue_frio_show_all({required Object count}) => '+ ${count} more like these · show all';

	/// en: 'Discard cold drafts'
	String get queue_batch_discard_title => 'Discard cold drafts';

	/// en: 'Discard $count queued drafts? The leads move to lost and nothing is sent.'
	String queue_batch_discard_message({required Object count}) => 'Discard ${count} queued drafts? The leads move to lost and nothing is sent.';

	/// en: 'Rewrite with the catalog'
	String get queue_batch_redraft_title => 'Rewrite with the catalog';

	/// en: 'Rewrite $count drafts using your product catalog? Each rewrite is an AI call billed to your key.'
	String queue_batch_redraft_message({required Object count}) => 'Rewrite ${count} drafts using your product catalog? Each rewrite is an AI call billed to your key.';

	/// en: '$count drafts discarded.'
	String queue_batch_discarded({required Object count}) => '${count} drafts discarded.';

	/// en: '$count drafts rewritten.'
	String queue_batch_redrafted({required Object count}) => '${count} drafts rewritten.';

	/// en: 'This week: $sent sent · $replies replies · $proposals proposals'
	String queue_outcome_line({required Object sent, required Object replies, required Object proposals}) => 'This week: ${sent} sent · ${replies} replies · ${proposals} proposals';

	/// en: 'Today's limit is spent — send scheduled for $when.'
	String approve_scheduled({required Object when}) => 'Today\'s limit is spent — send scheduled for ${when}.';

	/// en: 'Scheduled · goes out $when'
	String queue_scheduled_badge({required Object when}) => 'Scheduled · goes out ${when}';

	/// en: 'Send $n of $cap today'
	String route_strip_title({required Object n, required Object cap}) => 'Send ${n} of ${cap} today';

	/// en: 'Today's limit is spent — approvals now schedule for tomorrow morning'
	String get route_strip_exhausted => 'Today\'s limit is spent — approvals now schedule for tomorrow morning';

	/// en: '$mailbox warming up, day $day of $total — approving past the limit schedules by itself.'
	String route_strip_sub({required Object mailbox, required Object day, required Object total}) => '${mailbox} warming up, day ${day} of ${total} — approving past the limit schedules by itself.';

	/// en: 'Nothing can go out yet'
	String get queue_blocked_title => 'Nothing can go out yet';

	/// en: 'The offers are ready, but this account has no channel connected.'
	String get queue_blocked_body => 'The offers are ready, but this account has no channel connected.';

	/// en: 'Connect a channel'
	String get queue_blocked_cta => 'Connect a channel';

	/// en: 'tender closes in ${count}d'
	String queue_tender_deadline_days({required Object count}) => 'tender closes in ${count}d';

	/// en: 'tender closes today'
	String get queue_tender_deadline_today => 'tender closes today';

	/// en: 'tender closed'
	String get queue_tender_deadline_past => 'tender closed';

	/// en: 'estimated • $value'
	String queue_tender_value({required Object value}) => 'estimated • ${value}';

	/// en: 'No address to send to'
	String get queue_no_address_title => 'No address to send to';

	/// en: 'None of your channels can reach this contact. Find the buyer’s e-mail and fill the customer in — the offer is already written.'
	String get queue_no_address_body => 'None of your channels can reach this contact. Find the buyer’s e-mail and fill the customer in — the offer is already written.';

	/// en: 'Open customer'
	String get queue_no_address_cta => 'Open customer';

	/// en: 'Send from my WhatsApp'
	String get queue_wa_send => 'Send from my WhatsApp';

	/// en: 'opens on your number, text ready'
	String get queue_wa_hint => 'opens on your number, text ready';

	/// en: 'Did you send it?'
	String get queue_wa_confirm_title => 'Did you send it?';

	/// en: 'Nothing left from here — you sent it yourself. If you did, I will take this offer out of the queue and mark the customer as contacted.'
	String get queue_wa_confirm_body => 'Nothing left from here — you sent it yourself. If you did, I will take this offer out of the queue and mark the customer as contacted.';

	/// en: 'I sent it'
	String get queue_wa_confirm_yes => 'I sent it';

	/// en: 'Not yet'
	String get queue_wa_confirm_no => 'Not yet';
}

// Path: account_menu
class Translations$account_menu$en {
	Translations$account_menu$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Profile'
	String get profile => 'Profile';

	/// en: 'Team'
	String get team => 'Team';

	/// en: 'Channels'
	String get channels => 'Channels';

	/// en: 'Settings'
	String get settings => 'Settings';

	/// en: 'Log out'
	String get logout => 'Log out';

	/// en: 'Account'
	String get title => 'Account';
}

// Path: copilot
class Translations$copilot$en {
	Translations$copilot$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'What do you want to do?'
	String get empty_title => 'What do you want to do?';

	/// en: 'Ask in plain words. I do it and hand back a link. I never send anything without your approval.'
	String get empty_body => 'Ask in plain words. I do it and hand back a link. I never send anything without your approval.';

	/// en: 'ask for something…'
	String get hint => 'ask for something…';

	/// en: 'working…'
	String get thinking => 'working…';

	/// en: 'Clear conversation'
	String get clear => 'Clear conversation';

	/// en: 'Find 20 [segment] companies in [city]'
	String get sample_prospect => 'Find 20 [segment] companies in [city]';

	/// en: 'Draft approaches for the new leads in the funnel'
	String get sample_drafts => 'Draft approaches for the new leads in the funnel';

	/// en: 'How did my prospecting go in the last 30 days?'
	String get sample_report => 'How did my prospecting go in the last 30 days?';

	/// en: 'I could not answer. Try again.'
	String get failed => 'I could not answer. Try again.';

	/// en: 'looked up your funnels'
	String get tool_list_funnels => 'looked up your funnels';

	/// en: 'read the pipeline numbers'
	String get tool_pipeline_overview => 'read the pipeline numbers';

	/// en: 'read the approval queue'
	String get tool_list_approval_queue => 'read the approval queue';

	/// en: 'drafted approaches'
	String get tool_draft_approaches => 'drafted approaches';

	/// en: 'read your period report'
	String get tool_case_report => 'read your period report';

	/// en: 'Still working. Searches and drafts can take up to 2 minutes.'
	String get thinking_long => 'Still working. Searches and drafts can take up to 2 minutes.';

	/// en: 'Find 20 $keyword in $location'
	String sample_prospect_audience({required Object keyword, required Object location}) => 'Find 20 ${keyword} in ${location}';
}

// Path: attachment
class Translations$attachment$en {
	Translations$attachment$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: '$count attachment(s)'
	String count({required Object count}) => '${count} attachment(s)';

	/// en: 'Open'
	String get open => 'Open';

	/// en: 'Could not open the file right now.'
	String get download_failed => 'Could not open the file right now.';

	/// en: 'Scanned, nothing found'
	String get status_clean => 'Scanned, nothing found';

	/// en: 'Not scanned yet'
	String get status_pending => 'Not scanned yet';

	/// en: 'Not scanned'
	String get status_skipped => 'Not scanned';

	/// en: 'The scan did not finish'
	String get status_failed => 'The scan did not finish';

	/// en: 'Threat found'
	String get status_infected => 'Threat found';

	/// en: 'This file has not been released'
	String get blocked_title => 'This file has not been released';

	/// en: 'The scanner has not finished. Try again in a few minutes.'
	String get blocked_pending => 'The scanner has not finished. Try again in a few minutes.';

	/// en: 'No scanner is configured on this installation, so nobody checked this file. It came from outside — open at your own risk.'
	String get blocked_skipped => 'No scanner is configured on this installation, so nobody checked this file. It came from outside — open at your own risk.';

	/// en: 'The scan failed. The system retries automatically; if it persists, contact support.'
	String get blocked_failed => 'The scan failed. The system retries automatically; if it persists, contact support.';

	/// en: 'The scanner found a threat ($signature). This file cannot be opened. If you believe this is wrong, ask for the file another way.'
	String blocked_infected({required Object signature}) => 'The scanner found a threat (${signature}). This file cannot be opened. If you believe this is wrong, ask for the file another way.';

	/// en: 'Download anyway'
	String get download_anyway => 'Download anyway';

	/// en: 'Nobody checked this file. It was sent by someone outside your company. Open it?'
	String get download_anyway_confirm => 'Nobody checked this file. It was sent by someone outside your company. Open it?';

	/// en: 'Why?'
	String get why_blocked => 'Why?';

	/// en: 'View original'
	String get view_original => 'View original';

	/// en: 'View original with images'
	String get view_original_images => 'View original with images';

	/// en: 'Loading images tells the sender you opened the message. Continue?'
	String get view_original_images_warning => 'Loading images tells the sender you opened the message. Continue?';
}

// Path: sender_auth
class Translations$sender_auth$en {
	Translations$sender_auth$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Sender not verified'
	String get badge_unknown => 'Sender not verified';

	/// en: 'Suspicious sender'
	String get badge_failed => 'Suspicious sender';

	/// en: 'Nobody verified this sender'
	String get title_unknown => 'Nobody verified this sender';

	/// en: 'This sender failed verification'
	String get title_failed => 'This sender failed verification';

	/// en: 'The server that received this message did not report whether the domain authorised the send. This is common with older providers and does not mean it is fake — it means we cannot say it is genuine.'
	String get body_unknown => 'The server that received this message did not report whether the domain authorised the send. This is common with older providers and does not mean it is fake — it means we cannot say it is genuine.';

	/// en: 'The domain in the sender address did NOT authorise this send. Someone may be impersonating that company. Before sharing data, paying an invoice or changing bank details, confirm by phone using a number you already had.'
	String get body_failed => 'The domain in the sender address did NOT authorise this send. Someone may be impersonating that company. Before sharing data, paying an invoice or changing bank details, confirm by phone using a number you already had.';

	/// en: 'Checks: $checks'
	String checks({required Object checks}) => 'Checks: ${checks}';
}

// Path: email_inbox
class Translations$email_inbox$en {
	Translations$email_inbox$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: '(no subject)'
	String get no_subject => '(no subject)';

	/// en: 'No emails in this thread yet.'
	String get empty => 'No emails in this thread yet.';

	/// en: 'You'
	String get you => 'You';

	/// en: 'Back to emails'
	String get back_to_list => 'Back to emails';

	/// en: 'Loading email…'
	String get loading_email => 'Loading email…';

	/// en: 'Text'
	String get view_text => 'Text';

	/// en: 'Original'
	String get view_original => 'Original';
}

// Path: email_risk
class Translations$email_risk$en {
	Translations$email_risk$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Careful: the payment details changed'
	String get payment_title => 'Careful: the payment details changed';

	/// en: 'Confirm by phone before paying, using a number you already had — not the one in this email.'
	String get payment_action => 'Confirm by phone before paying, using a number you already had — not the one in this email.';

	/// en: 'Message held for review'
	String get quarantine_title => 'Message held for review';

	/// en: 'Worth a look before you act'
	String get attention_title => 'Worth a look before you act';

	/// en: 'This is legitimate'
	String get legit => 'This is legitimate';

	/// en: 'Confirm this sender is trustworthy? We will stop warning about them on this account.'
	String get legit_confirm => 'Confirm this sender is trustworthy? We will stop warning about them on this account.';

	/// en: 'Done. We will not warn about this sender again.'
	String get legit_done => 'Done. We will not warn about this sender again.';

	/// en: 'Could not record that right now.'
	String get legit_failed => 'Could not record that right now.';

	/// en: 'This is a scam'
	String get scam => 'This is a scam';

	/// en: 'Report as a scam'
	String get scam_title => 'Report as a scam';

	/// en: 'Why do you think it is a scam? (optional)'
	String get scam_why => 'Why do you think it is a scam? (optional)';

	/// en: 'e.g. this supplier never sends invoices by email'
	String get scam_why_hint => 'e.g. this supplier never sends invoices by email';

	/// en: 'Your report protects the other companies on the network. When two companies report the same sender, it is blocked for everyone.'
	String get scam_explain => 'Your report protects the other companies on the network. When two companies report the same sender, it is blocked for everyone.';

	/// en: 'Reported. Thank you — this protects the other companies on the network.'
	String get scam_done => 'Reported. Thank you — this protects the other companies on the network.';
}

// Path: brand
class Translations$brand$en {
	Translations$brand$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Company identity'
	String get title => 'Company identity';

	/// en: 'What the AI knows about you, and what it may never say. Applies to every approach, from every seller.'
	String get subtitle => 'What the AI knows about you, and what it may never say. Applies to every approach, from every seller.';

	/// en: 'Company identity'
	String get identity_section => 'Company identity';

	/// en: 'Company name'
	String get company_name => 'Company name';

	/// en: 'What you do'
	String get about => 'What you do';

	/// en: 'e.g. we distribute multiparameter monitors and infusion pumps to clinics in the interior of SP'
	String get about_hint => 'e.g. we distribute multiparameter monitors and infusion pumps to clinics in the interior of SP';

	/// en: 'How you speak'
	String get tone => 'How you speak';

	/// en: 'e.g. direct, no jargon, informal'
	String get tone_hint => 'e.g. direct, no jargon, informal';

	/// en: 'Default signature'
	String get signature => 'Default signature';

	/// en: 'What we may claim'
	String get allowed => 'What we may claim';

	/// en: 'True things that help you sell. e.g. our own service team in Sorocaba'
	String get allowed_hint => 'True things that help you sell. e.g. our own service team in Sorocaba';

	/// en: 'Never say'
	String get forbidden => 'Never say';

	/// en: 'The AI is blocked from writing these, and the text is checked before it reaches the queue. e.g. cures, approved by ANVISA'
	String get forbidden_hint => 'The AI is blocked from writing these, and the text is checked before it reaches the queue. e.g. cures, approved by ANVISA';

	/// en: 'Add'
	String get add => 'Add';

	/// en: 'Remove'
	String get remove => 'Remove';

	/// en: 'Identity saved.'
	String get saved => 'Identity saved.';

	/// en: 'Your own way'
	String get personal => 'Your own way';

	/// en: 'Adds to the company's, never replaces it. You cannot unlock what the company forbade.'
	String get personal_subtitle => 'Adds to the company\'s, never replaces it. You cannot unlock what the company forbade.';

	/// en: 'Your personal tone'
	String get tone_note => 'Your personal tone';

	/// en: 'e.g. I speak slowly, I always ask about the team before price'
	String get tone_note_hint => 'e.g. I speak slowly, I always ask about the team before price';

	/// en: 'Your signature'
	String get personal_signature => 'Your signature';

	/// en: 'e.g. João Silva · (15) 99999-0000'
	String get personal_signature_hint => 'e.g. João Silva · (15) 99999-0000';

	/// en: 'Your own way was saved.'
	String get personal_saved => 'Your own way was saved.';

	/// en: 'Your differentiators'
	String get differentiators => 'Your differentiators';

	/// en: 'Why yours rather than the one the hospital already owns. Written by you — the AI uses what fits and invents none. Empty, the proposal argues from the product alone.'
	String get differentiators_hint => 'Why yours rather than the one the hospital already owns. Written by you — the AI uses what fits and invents none. Empty, the proposal argues from the product alone.';
}

// Path: suppressions
class Translations$suppressions$en {
	Translations$suppressions$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Blocked addresses'
	String get title => 'Blocked addresses';

	/// en: 'Addresses that no longer receive email from you: they bounced permanently or reported you as spam. Writing to them again is what costs a sending domain its reputation.'
	String get subtitle => 'Addresses that no longer receive email from you: they bounced permanently or reported you as spam. Writing to them again is what costs a sending domain its reputation.';

	/// en: 'No blocked addresses'
	String get empty => 'No blocked addresses';

	/// en: 'Anything that bounces permanently will show up here, with the reason the receiving server gave.'
	String get empty_hint => 'Anything that bounces permanently will show up here, with the reason the receiving server gave.';

	/// en: 'Release'
	String get release => 'Release';

	/// en: 'Release this address?'
	String get release_title => 'Release this address?';

	/// en: 'A mail server refused this address, or the recipient reported you as spam. Only release it if you know the problem was fixed.'
	String get release_warning => 'A mail server refused this address, or the recipient reported you as spam. Only release it if you know the problem was fixed.';

	/// en: 'Address released'
	String get released => 'Address released';

	/// en: 'Permanent bounce — the address does not exist'
	String get reason_hard_bounce => 'Permanent bounce — the address does not exist';

	/// en: 'Reported as spam by the recipient'
	String get reason_complaint => 'Reported as spam by the recipient';
}

// Path: signals
class Translations$signals$en {
	Translations$signals$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Buying signals'
	String get title => 'Buying signals';

	/// en: 'Public tenders matching what you sell. Somebody wrote down what they intend to buy and by when — closest deadline first.'
	String get subtitle => 'Public tenders matching what you sell. Somebody wrote down what they intend to buy and by when — closest deadline first.';

	/// en: 'No monitoring set up'
	String get empty_no_watch => 'No monitoring set up';

	/// en: 'Tell us which words to watch for and open tenders will show up here on their own.'
	String get empty_no_watch_hint => 'Tell us which words to watch for and open tenders will show up here on their own.';

	/// en: 'Nothing open right now'
	String get empty_quiet => 'Nothing open right now';

	/// en: 'We check public tenders several times a day. New ones appear here as they are published.'
	String get empty_quiet_hint => 'We check public tenders several times a day. New ones appear here as they are published.';

	/// en: '$days days left'
	String days_left({required Object days}) => '${days} days left';

	/// en: 'closes today'
	String get closes_today => 'closes today';

	/// en: 'value not disclosed'
	String get value_undisclosed => 'value not disclosed';

	/// en: 'Read the notice'
	String get open_notice => 'Read the notice';

	/// en: 'Not for us'
	String get dismiss => 'Not for us';

	/// en: 'Signals'
	String get tab => 'Signals';

	/// en: 'Choose keywords'
	String get watch_cta => 'Choose keywords';

	/// en: 'What you want to follow'
	String get watch_dialog_title => 'What you want to follow';

	/// en: 'Words that show up in what you sell. Separate with commas — we look for each one in published notices.'
	String get watch_dialog_body => 'Words that show up in what you sell. Separate with commas — we look for each one in published notices.';

	/// en: 'e.g. hospital supplies, syringe, exam gloves'
	String get watch_hint => 'e.g. hospital supplies, syringe, exam gloves';

	/// en: 'Where you deliver'
	String get watch_states_label => 'Where you deliver';

	/// en: 'e.g. MT, MS'
	String get watch_states_hint => 'e.g. MT, MS';

	/// en: 'Name at least one state (UF).'
	String get watch_states_error => 'Name at least one state (UF).';

	/// en: 'We only read tenders from the states you serve — without them the search sweeps the whole country and returns what you cannot deliver.'
	String get watch_states_body => 'We only read tenders from the states you serve — without them the search sweeps the whole country and returns what you cannot deliver.';

	/// en: 'Done. Matching notices will show up here on their own.'
	String get watch_saved => 'Done. Matching notices will show up here on their own.';

	/// en: 'Write at least one word.'
	String get watch_empty_error => 'Write at least one word.';

	/// en: 'Following: $words'
	String watch_current({required Object words}) => 'Following: ${words}';

	/// en: 'No products registered — these matches come from hand-typed words, not from what you sell. Import your catalog and the watch builds itself from it.'
	String get no_catalog_banner => 'No products registered — these matches come from hand-typed words, not from what you sell. Import your catalog and the watch builds itself from it.';

	/// en: 'Add products'
	String get no_catalog_cta => 'Add products';

	/// en: 'from your catalog'
	String get watch_origin_catalog => 'from your catalog';

	/// en: 'set manually'
	String get watch_origin_manual => 'set manually';
}

// Path: sales_record
class Translations$sales_record$en {
	Translations$sales_record$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Record sale'
	String get title => 'Record sale';

	/// en: 'Total sold'
	String get gross => 'Total sold';

	/// en: 'what the customer pays'
	String get gross_hint => 'what the customer pays';

	/// en: 'Cost of goods'
	String get cost => 'Cost of goods';

	/// en: 'what it cost you — commission is paid on the margin, not on the invoice'
	String get cost_hint => 'what it cost you — commission is paid on the margin, not on the invoice';

	/// en: 'Tax'
	String get tax => 'Tax';

	/// en: 'how much tax this sale pays, in currency'
	String get tax_hint => 'how much tax this sale pays, in currency';

	/// en: 'Repurchase cycle (days)'
	String get cycle => 'Repurchase cycle (days)';

	/// en: 'how often this customer buys again — leave empty if you do not know'
	String get cycle_hint => 'how often this customer buys again — leave empty if you do not know';

	/// en: 'Notes'
	String get notes => 'Notes';

	/// en: 'You keep'
	String get net => 'You keep';

	/// en: 'This sale costs more than it brings in. Recording it anyway is fine — just check the numbers.'
	String get underwater_warning => 'This sale costs more than it brings in. Recording it anyway is fine — just check the numbers.';

	/// en: 'Record sale'
	String get submit => 'Record sale';

	/// en: 'Sale recorded'
	String get saved => 'Sale recorded';

	/// en: 'Your commission on this sale'
	String get your_commission => 'Your commission on this sale';
}

// Path: portfolio
class Translations$portfolio$en {
	Translations$portfolio$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Portfolio'
	String get title => 'Portfolio';

	/// en: 'Your customers — imported or won. Ordered by who needs to hear from you first, not alphabetically.'
	String get subtitle => 'Your customers — imported or won. Ordered by who needs to hear from you first, not alphabetically.';

	/// en: 'No customers yet'
	String get empty => 'No customers yet';

	/// en: 'Import your spreadsheet, or record a sale on a lead, and they show up here — with a prediction of when they will need it again.'
	String get empty_hint => 'Import your spreadsheet, or record a sale on a lead, and they show up here — with a prediction of when they will need it again.';

	/// en: '$count purchase(s)'
	String purchases({required Object count}) => '${count} purchase(s)';

	/// en: 'ran out $days days ago'
	String ran_out({required Object days}) => 'ran out ${days} days ago';

	/// en: 'runs out today'
	String get runs_out_today => 'runs out today';

	/// en: 'runs out in $days days'
	String runs_out_in({required Object days}) => 'runs out in ${days} days';

	/// en: 'no contact for $days days'
	String silent_for({required Object days}) => 'no contact for ${days} days';

	/// en: 'Purchase history'
	String get history => 'Purchase history';

	/// en: 'Sale with no itemisation'
	String get sale_without_items => 'Sale with no itemisation';
}

// Path: cross_sell
class Translations$cross_sell$en {
	Translations$cross_sell$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'What else to offer'
	String get title => 'What else to offer';

	/// en: 'because they buy $item'
	String because({required Object item}) => 'because they buy ${item}';

	/// en: '$count customers who buy $item also take'
	String evidence({required Object count, required Object item}) => '${count} customers who buy ${item} also take';

	/// en: 'Not enough co-purchase evidence in this account's sales yet.'
	String get empty => 'Not enough co-purchase evidence in this account\'s sales yet.';

	/// en: 'Learned from this account's sales only.'
	String get learned_here => 'Learned from this account\'s sales only.';
}

// Path: relationship
class Translations$relationship$en {
	Translations$relationship$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Relationship dates'
	String get title => 'Relationship dates';

	/// en: 'Birthdays, company anniversaries, sector days and maintenance. On the day, the draft goes to the approval queue.'
	String get subtitle => 'Birthdays, company anniversaries, sector days and maintenance. On the day, the draft goes to the approval queue.';

	/// en: 'No dates yet'
	String get empty => 'No dates yet';

	/// en: 'Add a birthday or a sector day and the system remembers for you.'
	String get empty_hint => 'Add a birthday or a sector day and the system remembers for you.';

	/// en: 'New date'
	String get add => 'New date';

	/// en: 'Birthday'
	String get kind_birthday => 'Birthday';

	/// en: 'Company anniversary'
	String get kind_founded => 'Company anniversary';

	/// en: 'Sector day'
	String get kind_sector => 'Sector day';

	/// en: 'Preventive maintenance'
	String get kind_maintenance => 'Preventive maintenance';

	/// en: 'Other'
	String get kind_custom => 'Other';

	/// en: 'Pick a professional day'
	String get sector_hint => 'Pick a professional day';

	/// en: 'Description'
	String get label => 'Description';

	/// en: 'Day'
	String get day => 'Day';

	/// en: 'Month'
	String get month => 'Month';

	/// en: 'Applies to the whole account — it shows here but never becomes a message on its own.'
	String get account_wide => 'Applies to the whole account — it shows here but never becomes a message on its own.';

	/// en: 'Forget this date?'
	String get delete_confirm => 'Forget this date?';

	/// en: 'Date saved'
	String get saved => 'Date saved';

	/// en: 'Only greetings can go out unapproved, and only with autopilot on.'
	String get greeting_only => 'Only greetings can go out unapproved, and only with autopilot on.';
}

// Path: command_center
class Translations$command_center$en {
	Translations$command_center$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Meetings'
	String get tile_meetings => 'Meetings';

	/// en: 'Won'
	String get tile_won => 'Won';

	/// en: 'Funnel'
	String get funnel_title => 'Funnel';

	/// en: '+$count'
	String more_actions({required Object count}) => '+${count}';

	/// en: 'Part of the list did not load — there may be more waiting.'
	String get partial_load => 'Part of the list did not load — there may be more waiting.';

	/// en: 'Later'
	String get snooze => 'Later';

	/// en: 'Snoozed. Back in $days days.'
	String snoozed({required Object days}) => 'Snoozed. Back in ${days} days.';

	/// en: 'Approach waiting for approval'
	String get kind_approve_draft => 'Approach waiting for approval';

	/// en: 'Lead with no owner'
	String get kind_unowned_lead => 'Lead with no owner';

	/// en: 'Opened the contract, did not sign'
	String get kind_signature_viewed => 'Opened the contract, did not sign';

	/// en: 'Contract deadline'
	String get kind_contract_alert => 'Contract deadline';

	/// en: 'Sale waiting for confirmation'
	String get kind_confirm_sale => 'Sale waiting for confirmation';

	/// en: 'Should have reordered by now'
	String get kind_reorder_due => 'Should have reordered by now';

	/// en: 'Customer waiting for a reply'
	String get kind_waiting_reply => 'Customer waiting for a reply';

	/// en: 'Needs attention'
	String get kind_unknown => 'Needs attention';

	/// en: '$count day(s) overdue'
	String overdue_days({required Object count}) => '${count} day(s) overdue';

	/// en: 'due today'
	String get due_today => 'due today';

	/// en: 'due in $count day(s)'
	String due_in_days({required Object count}) => 'due in ${count} day(s)';

	/// en: 'waiting $count day(s)'
	String waiting_days({required Object count}) => 'waiting ${count} day(s)';

	/// en: 'arrived today'
	String get waiting_today => 'arrived today';

	/// en: 'Your first sale starts with a lead'
	String get activation_title => 'Your first sale starts with a lead';

	/// en: 'There is nothing in the funnel yet. Bring in companies to prospect, or watch the public signals from whoever already said they intend to buy.'
	String get activation_body => 'There is nothing in the funnel yet. Bring in companies to prospect, or watch the public signals from whoever already said they intend to buy.';

	/// en: 'See public signals'
	String get activation_signals => 'See public signals';

	List<String> get months => [
		'January',
		'February',
		'March',
		'April',
		'May',
		'June',
		'July',
		'August',
		'September',
		'October',
		'November',
		'December',
	];

	/// en: '$month $year'
	String month_of_year({required Object month, required Object year}) => '${month} ${year}';

	/// en: 'Never approached'
	String get tile_untouched => 'Never approached';

	/// en: 'in your territory'
	String get tile_untouched_footnote => 'in your territory';

	/// en: '$count $unit of what you sell, never approached'
	String untouched_target_row({required Object count, required Object unit}) => '${count} ${unit} of what you sell, never approached';

	/// en: 'Never approached'
	String get kind_untouched_target => 'Never approached';

	/// en: 'See on the radar'
	String get action_see_target => 'See on the radar';
}

// Path: ai_debounce
class Translations$ai_debounce$en {
	Translations$ai_debounce$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'The AI answers in ~${seconds}s, waiting for the customer to finish'
	String waiting({required Object seconds}) => 'The AI answers in ~${seconds}s, waiting for the customer to finish';

	/// en: 'Answering…'
	String get answering => 'Answering…';

	/// en: '$count messages will be answered together'
	String collapsed({required Object count}) => '${count} messages will be answered together';

	/// en: 'Answer now'
	String get answer_now => 'Answer now';
}

// Path: overnight
class Translations$overnight$en {
	Translations$overnight$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Your SDR worked overnight'
	String get title => 'Your SDR worked overnight';

	/// en: '$count approaches ready to review'
	String drafted({required Object count}) => '${count} approaches ready to review';

	/// en: '$count did not fit this month's allowance'
	String budget_hit({required Object count}) => '${count} did not fit this month\'s allowance';

	/// en: 'Review'
	String get review => 'Review';

	/// en: 'Got it'
	String get dismiss => 'Got it';

	/// en: 'Tonight's hunt did not run'
	String get blocked_title => 'Tonight\'s hunt did not run';

	/// en: 'No offer was written last night. The reason:'
	String get blocked_body => 'No offer was written last night. The reason:';

	/// en: 'Fix it'
	String get blocked_cta => 'Fix it';
}

// Path: export
class Translations$export$en {
	Translations$export$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Export'
	String get tooltip => 'Export';

	/// en: 'Export to Excel (.xlsx)'
	String get xlsx => 'Export to Excel (.xlsx)';

	/// en: 'Export to CSV'
	String get csv => 'Export to CSV';

	/// en: 'Every export is recorded'
	String get audited => 'Every export is recorded';

	/// en: '$rows rows exported'
	String done({required Object rows}) => '${rows} rows exported';

	/// en: '$rows rows — the file hit the ceiling and is not the whole list'
	String truncated({required Object rows}) => '${rows} rows — the file hit the ceiling and is not the whole list';
}

// Path: my_customers
class Translations$my_customers$en {
	Translations$my_customers$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'The customers you already have'
	String get title => 'The customers you already have';

	/// en: 'We read only who you wrote to and when — never what the messages say.'
	String get subtitle => 'We read only who you wrote to and when — never what the messages say.';

	/// en: 'Search my mailbox'
	String get scan_action => 'Search my mailbox';

	/// en: 'Reading your Sent folder…'
	String get scanning => 'Reading your Sent folder…';

	/// en: 'Read $scanned messages in $mailbox and found $found people.'
	String scan_result({required Object scanned, required Object mailbox, required Object found}) => 'Read ${scanned} messages in ${mailbox} and found ${found} people.';

	/// en: '$count robot addresses and your own were left out.'
	String scan_skipped({required Object count}) => '${count} robot addresses and your own were left out.';

	/// en: 'We stopped before the end of the folder, so this is not the whole list. Search again to continue.'
	String get scan_partial => 'We stopped before the end of the folder, so this is not the whole list. Search again to continue.';

	/// en: 'We could not read your Sent folder: $error'
	String scan_failed({required Object error}) => 'We could not read your Sent folder: ${error}';

	/// en: 'Nothing to review yet.'
	String get empty_title => 'Nothing to review yet.';

	/// en: 'Search your mailbox to see who you already work with.'
	String get empty_hint => 'Search your mailbox to see who you already work with.';

	/// en: '$count emails sent'
	String sent_count({required Object count}) => '${count} emails sent';

	/// en: 'since $date'
	String since({required Object date}) => 'since ${date}';

	/// en: 'last on $date'
	String last({required Object date}) => 'last on ${date}';

	/// en: 'They go into'
	String get funnel_label => 'They go into';

	/// en: 'Pick the funnel before importing.'
	String get funnel_required => 'Pick the funnel before importing.';

	/// en: 'Create a funnel first — it is where the customers land.'
	String get no_funnels => 'Create a funnel first — it is where the customers land.';

	/// en: '$count selected'
	String selected({required Object count}) => '${count} selected';

	/// en: 'Import selected'
	String get import_action => 'Import selected';

	/// en: 'Not customers'
	String get dismiss_action => 'Not customers';

	/// en: '$count customers imported.'
	String imported({required Object count}) => '${count} customers imported.';

	/// en: '$count imported. $skipped were already in the system.'
	String imported_partial({required Object count, required Object skipped}) => '${count} imported. ${skipped} were already in the system.';

	/// en: '$count removed from the list.'
	String dismissed({required Object count}) => '${count} removed from the list.';

	/// en: 'Select all'
	String get select_all => 'Select all';

	/// en: 'Clear'
	String get clear_selection => 'Clear';

	/// en: 'Showing $shown of $total.'
	String showing_partial({required Object shown, required Object total}) => 'Showing ${shown} of ${total}.';

	/// en: 'In my inbox'
	String get view_label => 'In my inbox';

	/// en: 'See customers from my mailbox'
	String get open_inbox_customers => 'See customers from my mailbox';
}

// Path: proposal
class Translations$proposal$en {
	Translations$proposal$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'New proposal'
	String get title => 'New proposal';

	/// en: 'Pick what you are offering and how many.'
	String get pick_products => 'Pick what you are offering and how many.';

	/// en: 'Total'
	String get total => 'Total';

	/// en: 'These products are priced in different currencies. One proposal carries one currency — otherwise the total would be a number nobody can defend.'
	String get mixed_currency => 'These products are priced in different currencies. One proposal carries one currency — otherwise the total would be a number nobody can defend.';

	/// en: 'Your message'
	String get body_label => 'Your message';

	/// en: 'Optional. The priced lines are added below whatever you write.'
	String get body_hint => 'Optional. The priced lines are added below whatever you write.';

	/// en: 'Send for approval'
	String get send_for_approval => 'Send for approval';

	/// en: 'Goes to the approval queue. Nothing reaches the customer until you approve it there.'
	String get approval_note => 'Goes to the approval queue. Nothing reaches the customer until you approve it there.';

	/// en: 'In the approval queue. This is what the reviewer will read:'
	String get queued_title => 'In the approval queue. This is what the reviewer will read:';

	/// en: 'ALREADY OFFERED'
	String get history => 'ALREADY OFFERED';

	/// en: 'Draft'
	String get status_draft => 'Draft';

	/// en: 'Sent on $date'
	String status_sent({required Object date}) => 'Sent on ${date}';

	/// en: 'No products yet'
	String get no_products => 'No products yet';

	/// en: 'A proposal is made of what you sell. Add a product and come back.'
	String get no_products_hint => 'A proposal is made of what you sell. Add a product and come back.';

	/// en: 'Add a product'
	String get go_to_products => 'Add a product';

	/// en: 'Make a proposal'
	String get open => 'Make a proposal';

	/// en: 'Download PDF'
	String get download_pdf => 'Download PDF';

	/// en: 'Could not save the file.'
	String get download_failed => 'Could not save the file.';

	/// en: 'A file to send right now, by WhatsApp or e-mail. Nothing is sent on its own.'
	String get download_hint => 'A file to send right now, by WhatsApp or e-mail. Nothing is sent on its own.';

	/// en: 'Prefilled with what fits this customer. Change quantities, drop what does not fit, add what is missing.'
	String get suggested_hint => 'Prefilled with what fits this customer. Change quantities, drop what does not fit, add what is missing.';
}

// Path: product_import
class Translations$product_import$en {
	Translations$product_import$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Import price list'
	String get title => 'Import price list';

	/// en: 'A CSV with a header row (name/product, price, description, currency) or a manufacturer catalogue PDF — the AI reads the PDF and you check every line before anything is created. You can pick several files at once.'
	String get what_we_read => 'A CSV with a header row (name/product, price, description, currency) or a manufacturer catalogue PDF — the AI reads the PDF and you check every line before anything is created. You can pick several files at once.';

	/// en: 'Reading $done of $total…'
	String reading_progress({required Object done, required Object total}) => 'Reading ${done} of ${total}…';

	/// en: '$count file(s) could not be read: $reason'
	String files_failed({required Object count, required Object reason}) => '${count} file(s) could not be read: ${reason}';

	/// en: '$count files read'
	String files_read({required Object count}) => '${count} files read';

	/// en: 'from $file'
	String row_from({required Object file}) => 'from ${file}';

	/// en: 'Choose file'
	String get pick_file => 'Choose file';

	/// en: 'Choose another file'
	String get pick_another => 'Choose another file';

	/// en: '$total lines read, $importable ready to import'
	String found({required Object total, required Object importable}) => '${total} lines read, ${importable} ready to import';

	/// en: '$count need a look'
	String with_problems({required Object count}) => '${count} need a look';

	/// en: '$count already in your catalogue'
	String already_here({required Object count}) => '${count} already in your catalogue';

	/// en: 'Import $count product(s)'
	String confirm({required Object count}) => 'Import ${count} product(s)';

	/// en: 'Nothing to import from this file yet — fix the lines above and choose it again.'
	String get nothing_importable => 'Nothing to import from this file yet — fix the lines above and choose it again.';

	/// en: 'line $line — $problem'
	String line_problem({required Object line, required Object problem}) => 'line ${line} — ${problem}';

	/// en: 'line $line — $price'
	String line_price({required Object line, required Object price}) => 'line ${line} — ${price}';

	/// en: '(no name)'
	String get blank_name => '(no name)';

	/// en: '$count products imported.'
	String imported({required Object count}) => '${count} products imported.';

	/// en: '$count imported, $skipped skipped.'
	String imported_partial({required Object count, required Object skipped}) => '${count} imported, ${skipped} skipped.';

	/// en: 'Import a spreadsheet'
	String get open => 'Import a spreadsheet';

	/// en: 'Or send the manufacturer's brochure as PDF — the AI reads it and you review every line before anything is created.'
	String get what_we_read_pdf => 'Or send the manufacturer\'s brochure as PDF — the AI reads it and you review every line before anything is created.';

	/// en: 'Import brochure (PDF)'
	String get pick_pdf => 'Import brochure (PDF)';

	/// en: 'Reading the brochure with AI — this can take up to a minute…'
	String get pdf_reading => 'Reading the brochure with AI — this can take up to a minute…';

	/// en: 'page $page — $problem'
	String page_problem({required Object page, required Object problem}) => 'page ${page} — ${problem}';

	/// en: 'page $page — $price'
	String page_price({required Object page, required Object price}) => 'page ${page} — ${price}';

	/// en: 'price not read — fill in later'
	String get price_missing => 'price not read — fill in later';
}

// Path: customer_import
class Translations$customer_import$en {
	Translations$customer_import$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Import customer list'
	String get title => 'Import customer list';

	/// en: 'A CSV with a header row. It reads the company/customer, contact/name, phone/mobile and email columns — comma or semicolon separated, however your spreadsheet saved it. Every customer needs a phone or an email.'
	String get what_we_read => 'A CSV with a header row. It reads the company/customer, contact/name, phone/mobile and email columns — comma or semicolon separated, however your spreadsheet saved it. Every customer needs a phone or an email.';

	/// en: 'Choose file'
	String get pick_file => 'Choose file';

	/// en: 'Choose another file'
	String get pick_another => 'Choose another file';

	/// en: '$total lines read, $importable ready to import'
	String found({required Object total, required Object importable}) => '${total} lines read, ${importable} ready to import';

	/// en: '$count need a look'
	String with_problems({required Object count}) => '${count} need a look';

	/// en: '$count are already in your base'
	String already_here({required Object count}) => '${count} are already in your base';

	/// en: 'Import $count customer(s)'
	String confirm({required Object count}) => 'Import ${count} customer(s)';

	/// en: 'Nothing to import from this file yet — fix the lines above and choose the file again.'
	String get nothing_importable => 'Nothing to import from this file yet — fix the lines above and choose the file again.';

	/// en: 'line $line — $problem'
	String line_problem({required Object line, required Object problem}) => 'line ${line} — ${problem}';

	/// en: 'line $line — $address'
	String line_address({required Object line, required Object address}) => 'line ${line} — ${address}';

	/// en: '(no name)'
	String get blank_name => '(no name)';

	/// en: '$count customers imported.'
	String imported({required Object count}) => '${count} customers imported.';

	/// en: '$count imported, $skipped skipped.'
	String imported_partial({required Object count, required Object skipped}) => '${count} imported, ${skipped} skipped.';

	/// en: 'Import spreadsheet'
	String get open => 'Import spreadsheet';
}

// Path: sales_import
class Translations$sales_import$en {
	Translations$sales_import$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Import sales history'
	String get title => 'Import sales history';

	/// en: 'Import sales'
	String get open => 'Import sales';

	/// en: 'Bring in past sales so repurchase dates and cross-sell have data'
	String get settings_subtitle => 'Bring in past sales so repurchase dates and cross-sell have data';

	/// en: 'A CSV with a header row. It reads the customer, product, date, amount and quantity columns — comma or semicolon separated, however your spreadsheet saved it. Customers must already be in your base: import the customer list first.'
	String get hint => 'A CSV with a header row. It reads the customer, product, date, amount and quantity columns — comma or semicolon separated, however your spreadsheet saved it. Customers must already be in your base: import the customer list first.';

	/// en: 'Choose file'
	String get pick_file => 'Choose file';

	/// en: 'Choose another file'
	String get pick_another => 'Choose another file';

	/// en: 'What the file says'
	String get preview_title => 'What the file says';

	/// en: '$total lines read, $importable ready to record'
	String found({required Object total, required Object importable}) => '${total} lines read, ${importable} ready to record';

	/// en: '$count need a look'
	String problems_title({required Object count}) => '${count} need a look';

	/// en: '$count are already recorded'
	String already_recorded({required Object count}) => '${count} are already recorded';

	/// en: '$count name customers not in your base — import the customer list first'
	String unknown_customers({required Object count}) => '${count} name customers not in your base — import the customer list first';

	/// en: 'Record $count sale(s)'
	String confirm({required Object count}) => 'Record ${count} sale(s)';

	/// en: 'Nothing to record from this file yet — fix the lines above and choose the file again.'
	String get nothing_importable => 'Nothing to record from this file yet — fix the lines above and choose the file again.';

	/// en: 'line $line — $problem'
	String line_problem({required Object line, required Object problem}) => 'line ${line} — ${problem}';

	/// en: 'line $line — $detail'
	String line_detail({required Object line, required Object detail}) => 'line ${line} — ${detail}';

	/// en: '(no customer)'
	String get blank_customer => '(no customer)';

	/// en: '$count sales recorded.'
	String result({required Object count}) => '${count} sales recorded.';

	/// en: '$count sales recorded, $skipped skipped.'
	String skipped({required Object count, required Object skipped}) => '${count} sales recorded, ${skipped} skipped.';

	/// en: 'View portfolio'
	String get view_portfolio => 'View portfolio';
}

// Path: lead_timeline
class Translations$lead_timeline$en {
	Translations$lead_timeline$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'CONVERSATION'
	String get title => 'CONVERSATION';

	/// en: 'Nothing has been sent to this customer yet.'
	String get empty => 'Nothing has been sent to this customer yet.';

	/// en: 'Showing the most recent messages.'
	String get truncated => 'Showing the most recent messages.';

	/// en: 'Customer'
	String get from_customer => 'Customer';

	/// en: 'Us'
	String get from_team => 'Us';

	/// en: 'Bot'
	String get from_bot => 'Bot';

	/// en: '[$kind]'
	String attachment({required Object kind}) => '[${kind}]';

	/// en: 'Not delivered'
	String get not_delivered => 'Not delivered';

	/// en: 'Write your reply…'
	String get reply_hint => 'Write your reply…';

	/// en: 'Reply'
	String get reply_send => 'Reply';

	/// en: 'Reply sent.'
	String get reply_sent => 'Reply sent.';

	/// en: 'Replying opens once there is a conversation — the first offer goes out from the queue.'
	String get no_conversation => 'Replying opens once there is a conversation — the first offer goes out from the queue.';
}

// Path: email_composer
class Translations$email_composer$en {
	Translations$email_composer$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Subject'
	String get subject_hint => 'Subject';

	/// en: 'Write your email…'
	String get body_hint => 'Write your email…';

	/// en: 'Send'
	String get send => 'Send';

	/// en: 'Reply'
	String get reply => 'Reply';
}

// Path: radar
class Translations$radar$en {
	Translations$radar$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Radar'
	String get tab => 'Radar';

	/// en: 'Today'
	String get tab_hoje => 'Today';

	/// en: 'Could not read: $sources. The rest of the feed is live.'
	String warnings({required Object sources}) => 'Could not read: ${sources}. The rest of the feed is live.';

	/// en: 'We could not read $source for $ago. What follows is from the last collection — it may not be what is open now.'
	String source_down({required Object source, required Object ago}) => 'We could not read ${source} for ${ago}. What follows is from the last collection — it may not be what is open now.';

	/// en: 'We have not been able to read $source yet. Nothing in this list came from it.'
	String source_down_never({required Object source}) => 'We have not been able to read ${source} yet. Nothing in this list came from it.';

	/// en: '$n min'
	String ago_minutes({required Object n}) => '${n} min';

	/// en: '$n h'
	String ago_hours({required Object n}) => '${n} h';

	/// en: '$n days'
	String ago_days({required Object n}) => '${n} days';

	/// en: 'All'
	String get filter_all => 'All';

	/// en: 'Tenders'
	String get type_licitacao => 'Tenders';

	/// en: 'Installed base'
	String get type_parque => 'Installed base';

	/// en: 'Reorders'
	String get type_recompra => 'Reorders';

	/// en: 'Dates'
	String get type_data => 'Dates';

	/// en: 'Nothing on the radar right now'
	String get empty => 'Nothing on the radar right now';

	/// en: 'Tenders, CNES installed-base hits, reorders coming due and relationship dates appear here as they are detected.'
	String get empty_hint => 'Tenders, CNES installed-base hits, reorders coming due and relationship dates appear here as they are detected.';

	/// en: 'Draft queued — waiting for your approval in Today.'
	String get offer_queued => 'Draft queued — waiting for your approval in Today.';

	/// en: 'today'
	String get deadline_today => 'today';

	/// en: 'in ${count}d'
	String deadline_days({required Object count}) => 'in ${count}d';

	/// en: 'Open draft'
	String get open_draft => 'Open draft';

	/// en: 'Prepare offer'
	String get prepare_offer => 'Prepare offer';

	/// en: 'View customer'
	String get open_customer => 'View customer';

	/// en: 'Stock the territory'
	String get hunt_title => 'Stock the territory';

	/// en: 'Searches CNES for who declares the equipment you serve — public data, counts, never equipment age. Phone and address come from the CNES registry.'
	String get hunt_hint => 'Searches CNES for who declares the equipment you serve — public data, counts, never equipment age. Phone and address come from the CNES registry.';

	/// en: 'e.g. tomógrafo, ventilador, ultrassom'
	String get hunt_placeholder => 'e.g. tomógrafo, ventilador, ultrassom';

	/// en: 'Nothing found.'
	String get hunt_empty => 'Nothing found.';

	/// en: '$count declared to CNES'
	String hunt_units({required Object count}) => '${count} declared to CNES';

	/// en: 'already in your contacts'
	String get hunt_known => 'already in your contacts';

	/// en: 'Cross-sell'
	String get type_cross_sell => 'Cross-sell';

	/// en: 'Reorders, cross-sell and dates appear after the first recorded sale.'
	String get carteira_hint => 'Reorders, cross-sell and dates appear after the first recorded sale.';

	/// en: 'Tenders and CNES installed-base hits appear here as they are detected.'
	String get empty_hint_prospecting => 'Tenders and CNES installed-base hits appear here as they are detected.';

	/// en: 'Everything turned into an offer lands in Today for your approval — nothing is sent from here.'
	String get footer_note => 'Everything turned into an offer lands in Today for your approval — nothing is sent from here.';

	/// en: 'catalog ($count products)'
	String watching_catalog({required Object count}) => 'catalog (${count} products)';

	/// en: 'This week:'
	String get week_prefix => 'This week:';

	/// en: 'sent'
	String get week_sent => 'sent';

	/// en: 'replies'
	String get week_replies => 'replies';

	/// en: 'proposals'
	String get week_proposals => 'proposals';

	/// en: 'CNES data from $month'
	String data_month({required Object month}) => 'CNES data from ${month}';

	/// en: 'in your territory ($cities)'
	String hunt_scope_cities({required Object cities}) => 'in your territory (${cities})';

	/// en: 'in $uf'
	String hunt_scope_uf({required Object uf}) => 'in ${uf}';

	/// en: 'Hunt with my catalogue'
	String get auto_hunt_cta => 'Hunt with my catalogue';

	/// en: 'It reads your products and brings the biggest hospitals in your territory you have never approached. Nothing is sent.'
	String get auto_hunt_hint => 'It reads your products and brings the biggest hospitals in your territory you have never approached. Nothing is sent.';

	/// en: '$created new leads in the funnel, out of $found found'
	String auto_hunt_result({required Object created, required Object found}) => '${created} new leads in the funnel, out of ${found} found';

	/// en: 'No new establishment in your territory right now'
	String get auto_hunt_none => 'No new establishment in your territory right now';

	/// en: '$total hospitals $scope declare equipment your catalogue matches. Showing the $shown biggest.'
	String park_universe({required Object total, required Object scope, required Object shown}) => '${total} hospitals ${scope} declare equipment your catalogue matches. Showing the ${shown} biggest.';

	/// en: '+$count more'
	String offer_more({required Object count}) => '+${count} more';

	/// en: 'Contract expiring'
	String get type_contrato => 'Contract expiring';

	/// en: 'Price registry'
	String get type_ata => 'Price registry';

	/// en: 'before the tender'
	String get edge_before_tender => 'before the tender';

	/// en: 'no tender needed'
	String get edge_no_tender => 'no tender needed';

	/// en: 'Read and send'
	String get read_and_send => 'Read and send';

	/// en: 'Back to the panel'
	String get back_to_panel => 'Back to the panel';

	/// en: 'What they are buying'
	String get panel_object => 'What they are buying';

	/// en: 'Who is buying'
	String get panel_buyer => 'Who is buying';

	/// en: 'Number in the public record'
	String get panel_reference => 'Number in the public record';

	/// en: 'Published'
	String get panel_published => 'Published';

	/// en: 'Deadline'
	String get panel_deadline => 'Deadline';

	/// en: 'Value'
	String get panel_value => 'Value';

	/// en: 'This notice published no link and no file. Copy the number above and search it on PNCP — it is the only reference there is.'
	String get panel_no_link => 'This notice published no link and no file. Copy the number above and search it on PNCP — it is the only reference there is.';

	/// en: 'What this hospital declares'
	String get panel_park_items => 'What this hospital declares';

	/// en: 'What you can offer'
	String get panel_offer_title => 'What you can offer';

	/// en: 'Close'
	String get panel_close => 'Close';

	/// en: 'Copy number'
	String get panel_copy_reference => 'Copy number';

	/// en: 'Number copied'
	String get panel_copied => 'Number copied';

	/// en: 'Tap an opportunity to see everything we know about it.'
	String get panel_empty => 'Tap an opportunity to see everything we know about it.';

	/// en: 'PNCP — Brazil's public procurement portal'
	String get panel_portal => 'PNCP — Brazil\'s public procurement portal';

	/// en: 'Open PNCP'
	String get panel_open_portal => 'Open PNCP';

	/// en: 'You have already sold to this buyer'
	String get panel_buyer_known => 'You have already sold to this buyer';

	/// en: 'This buyer has never heard of you'
	String get panel_buyer_cold => 'This buyer has never heard of you';

	/// en: 'What this body has contracted'
	String get panel_history => 'What this body has contracted';

	/// en: 'This buyer has $n contract(s) in your trade on the public record.'
	String panel_history_count({required Object n}) => 'This buyer has ${n} contract(s) in your trade on the public record.';

	/// en: 'No contract in your trade on the public record for this buyer.'
	String get panel_history_none => 'No contract in your trade on the public record for this buyer.';

	/// en: 'You have already sent an offer to this buyer'
	String get panel_buyer_contacted => 'You have already sent an offer to this buyer';

	/// en: 'Offer written, waiting for your approval — nothing sent'
	String get panel_buyer_queued => 'Offer written, waiting for your approval — nothing sent';

	/// en: 'Already on your board — nothing written yet'
	String get panel_buyer_listed => 'Already on your board — nothing written yet';

	/// en: 'How to reach them'
	String get panel_reach => 'How to reach them';

	/// en: 'On the board in: $stage'
	String panel_stage({required Object stage}) => 'On the board in: ${stage}';

	/// en: 'Open on the board'
	String get panel_open_board => 'Open on the board';

	/// en: 'Who buys for them'
	String get panel_purchaser => 'Who buys for them';

	/// en: '$org, which buys for them, has $n contract(s) in your trade on the public record.'
	String panel_history_via({required Object org, required Object n}) => '${org}, which buys for them, has ${n} contract(s) in your trade on the public record.';

	/// en: 'Beds it runs'
	String get panel_beds => 'Beds it runs';

	/// en: 'Funded by the Ministry'
	String get panel_habilitations => 'Funded by the Ministry';

	/// en: '$name — $n bed(s) funded'
	String panel_habilitation_beds({required Object name, required Object n}) => '${name} — ${n} bed(s) funded';

	/// en: 'Services it performs'
	String get panel_services => 'Services it performs';
}

// Path: start
class Translations$start$en {
	Translations$start$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Good morning, $name'
	String greeting_morning({required Object name}) => 'Good morning, ${name}';

	/// en: 'Good afternoon, $name'
	String greeting_afternoon({required Object name}) => 'Good afternoon, ${name}';

	/// en: 'Good evening, $name'
	String greeting_evening({required Object name}) => 'Good evening, ${name}';

	/// en: '$weekday, $month $day'
	String date_line({required Object weekday, required Object month, required Object day}) => '${weekday}, ${month} ${day}';

	List<String> get weekdays => [
		'Monday',
		'Tuesday',
		'Wednesday',
		'Thursday',
		'Friday',
		'Saturday',
		'Sunday',
	];

	/// en: '$count offer(s) ready'
	String summary_ready({required Object count}) => '${count} offer(s) ready';

	/// en: '$count customer(s) waiting'
	String summary_waiting({required Object count}) => '${count} customer(s) waiting';

	/// en: '$count due today'
	String summary_due_today({required Object count}) => '${count} due today';

	/// en: 'nothing due today'
	String get summary_due_none => 'nothing due today';

	/// en: 'Review the $count'
	String review_n({required Object count}) => 'Review the ${count}';

	/// en: 'See Radar'
	String get see_radar => 'See Radar';

	/// en: 'Reply'
	String get action_reply => 'Reply';

	/// en: 'Take it'
	String get action_assume => 'Take it';

	/// en: '✦ Distribute across the team'
	String get distribute_team => '✦ Distribute across the team';

	/// en: 'sorted by urgency'
	String get sorted_by_urgency => 'sorted by urgency';

	/// en: 'All $count'
	String filter_all({required Object count}) => 'All ${count}';

	/// en: 'Radar today'
	String get rail_radar_title => 'Radar today';

	/// en: 'open →'
	String get rail_radar_open => 'open →';

	/// en: 'Your week'
	String get rail_week_title => 'Your week';

	/// en: 'Start the engine'
	String get rail_engine_title => 'Start the engine';

	/// en: '$current of $total'
	String of_total({required Object current, required Object total}) => '${current} of ${total}';

	/// en: 'record a sale'
	String get register_sale_link => 'record a sale';

	/// en: '$count with no owner'
	String unowned_footnote({required Object count}) => '${count} with no owner';
}

// Path: calendar
class Translations$calendar$en {
	Translations$calendar$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Expiries'
	String get tab => 'Expiries';

	/// en: 'What runs out'
	String get title => 'What runs out';

	/// en: 'Public contracts in your state ending in the next $days days. When one ends the purchase has to happen again — and you know that months before any tender exists.'
	String subtitle({required Object days}) => 'Public contracts in your state ending in the next ${days} days. When one ends the purchase has to happen again — and you know that months before any tender exists.';

	/// en: 'Nothing expiring in this window'
	String get empty_title => 'Nothing expiring in this window';

	/// en: 'No contract for what you sell ends in the next $days days in your state.'
	String empty_body({required Object days}) => 'No contract for what you sell ends in the next ${days} days in your state.';

	/// en: 'Still reading your territory’s contracts'
	String get scanning_title => 'Still reading your territory’s contracts';

	/// en: 'We ask the portal hospital by hospital, without hammering their server. It fills in over the next few days.'
	String get scanning_body => 'We ask the portal hospital by hospital, without hammering their server. It fills in over the next few days.';

	/// en: 'Set where you sell'
	String get no_territory_title => 'Set where you sell';

	/// en: 'Without a state we cannot tell which contracts matter to you. Set it in Settings → Company.'
	String get no_territory_body => 'Without a state we cannot tell which contracts matter to you. Set it in Settings → Company.';

	/// en: 'Tell us what you sell'
	String get no_keywords_title => 'Tell us what you sell';

	/// en: 'The calendar uses the same words as the Radar. Add your products or adjust the watch.'
	String get no_keywords_body => 'The calendar uses the same words as the Radar. Add your products or adjust the watch.';

	/// en: 'ends in ${count}d'
	String days_left({required Object count}) => 'ends in ${count}d';

	/// en: 'ends today'
	String get days_today => 'ends today';

	/// en: 'ended ${count}d ago'
	String days_past({required Object count}) => 'ended ${count}d ago';

	/// en: 'held by $name'
	String holder({required Object name}) => 'held by ${name}';

	/// en: 'current supplier not published'
	String get holder_unknown => 'current supplier not published';

	/// en: 'price registry'
	String get price_registry => 'price registry';

	/// en: 'buys by adhesion'
	String get rides_others => 'buys by adhesion';

	/// en: 'This body rides other bodies’ price registries. You can sell without winning a tender.'
	String get rides_others_hint => 'This body rides other bodies’ price registries. You can sell without winning a tender.';

	/// en: 'Open record'
	String get open_record => 'Open record';

	/// en: 'value not published'
	String get value_unknown => 'value not published';

	/// en: 'Work this one'
	String get work => 'Work this one';

	/// en: '$name is in your funnel.'
	String worked({required Object name}) => '${name} is in your funnel.';

	/// en: 'Could not add it to the funnel.'
	String get work_failed => 'Could not add it to the funnel.';

	/// en: 'Open funnel'
	String get open_funnel => 'Open funnel';

	/// en: 'Added to your customers. The buyer still needs a contact.'
	String get work_queued => 'Added to your customers. The buyer still needs a contact.';
}

// Path: feedback.cancel_reasons
class Translations$feedback$cancel_reasons$en {
	Translations$feedback$cancel_reasons$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Too expensive'
	String get too_expensive => 'Too expensive';

	/// en: 'Missing features I need'
	String get missing_features => 'Missing features I need';

	/// en: 'I'm not using it enough'
	String get not_using => 'I\'m not using it enough';

	/// en: 'Switched to another tool'
	String get switched_competitor => 'Switched to another tool';

	/// en: 'Technical issues or bugs'
	String get technical_issues => 'Technical issues or bugs';

	/// en: 'Just pausing for now'
	String get temporary_pause => 'Just pausing for now';

	/// en: 'Other'
	String get other => 'Other';
}

// Path: sales.products
class Translations$sales$products$en {
	Translations$sales$products$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Products'
	String get title => 'Products';

	/// en: 'Manage your catalog products'
	String get subtitle => 'Manage your catalog products';

	/// en: 'New product'
	String get new_product => 'New product';

	/// en: 'Edit product'
	String get edit_product => 'Edit product';

	/// en: 'No products yet'
	String get no_products => 'No products yet';

	/// en: 'Add the products you sell so your AI agent knows what it's offering.'
	String get empty_subtitle => 'Add the products you sell so your AI agent knows what it\'s offering.';

	/// en: 'Create first product'
	String get create_first => 'Create first product';

	/// en: 'Failed to load products'
	String get load_error => 'Failed to load products';

	/// en: 'Active'
	String get active_label => 'Active';

	/// en: 'Inactive'
	String get inactive_label => 'Inactive';

	/// en: 'Name'
	String get field_name => 'Name';

	/// en: 'e.g. Infusion pump'
	String get field_name_hint => 'e.g. Infusion pump';

	/// en: 'Description'
	String get field_description => 'Description';

	/// en: 'e.g. Monthly rental with preventive maintenance included (optional)'
	String get field_description_hint => 'e.g. Monthly rental with preventive maintenance included (optional)';

	/// en: 'Price'
	String get field_price => 'Price';

	/// en: 'Repurchase cycle (days)'
	String get field_cycle => 'Repurchase cycle (days)';

	/// en: 'e.g. 30 — how often a customer reorders this. Empty means unknown, and it never shows up in reorders'
	String get field_cycle_hint => 'e.g. 30 — how often a customer reorders this. Empty means unknown, and it never shows up in reorders';

	/// en: 'Use a whole number of days, up to 3650'
	String get cycle_invalid => 'Use a whole number of days, up to 3650';

	/// en: 'Currency'
	String get field_currency => 'Currency';

	/// en: 'Active'
	String get field_active => 'Active';

	/// en: 'Inactive products are hidden from funnels'
	String get field_active_hint => 'Inactive products are hidden from funnels';

	/// en: 'Name is required'
	String get name_required => 'Name is required';

	/// en: 'Enter the price'
	String get price_required => 'Enter the price';

	/// en: 'Unreadable price — use 1,234.56'
	String get price_invalid => 'Unreadable price — use 1,234.56';

	/// en: 'Save'
	String get save => 'Save';

	/// en: 'Product created'
	String get created_success => 'Product created';

	/// en: 'Product updated'
	String get updated_success => 'Product updated';

	/// en: 'Product deleted'
	String get deleted_success => 'Product deleted';

	/// en: 'Delete product'
	String get delete_title => 'Delete product';

	/// en: 'Delete "$name"? This cannot be undone.'
	String delete_message({required Object name}) => 'Delete "${name}"? This cannot be undone.';

	/// en: 'Product not found'
	String get not_found => 'Product not found';

	/// en: 'It may have been deleted, or the address is wrong.'
	String get not_found_hint => 'It may have been deleted, or the address is wrong.';

	/// en: 'Import spreadsheet or PDF'
	String get import_first => 'Import spreadsheet or PDF';

	/// en: 'Leave empty — the rep quotes on the spot'
	String get field_price_optional => 'Leave empty — the rep quotes on the spot';

	/// en: 'From the manufacturer's brochure'
	String get sheet_title => 'From the manufacturer\'s brochure';

	/// en: 'Brand'
	String get sheet_brand => 'Brand';

	/// en: 'Model'
	String get sheet_model => 'Model';

	/// en: 'Code'
	String get sheet_sku => 'Code';

	/// en: 'Unit'
	String get sheet_unit => 'Unit';

	/// en: 'Technical sheet'
	String get sheet_specs => 'Technical sheet';

	/// en: 'Accessories'
	String get sheet_accessories => 'Accessories';

	/// en: 'Standards and certifications'
	String get sheet_standards => 'Standards and certifications';

	/// en: 'Open brochure (p. $page)'
	String open_brochure({required Object page}) => 'Open brochure (p. ${page})';

	/// en: 'Open brochure'
	String get open_brochure_no_page => 'Open brochure';

	/// en: 'This product did not come from an archived brochure'
	String get brochure_missing => 'This product did not come from an archived brochure';

	/// en: 'Images from the brochure'
	String get sheet_images => 'Images from the brochure';

	/// en: 'No usable image on this page'
	String get sheet_images_empty => 'No usable image on this page';

	/// en: 'p. $page'
	String image_page({required Object page}) => 'p. ${page}';

	/// en: 'Open image'
	String get image_open => 'Open image';
}

// Path: sales.funnels
class Translations$sales$funnels$en {
	Translations$sales$funnels$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Funnels'
	String get title => 'Funnels';

	/// en: 'New funnel'
	String get new_funnel => 'New funnel';

	/// en: 'Edit funnel'
	String get edit_funnel => 'Edit funnel';

	/// en: 'No funnels yet'
	String get no_funnels => 'No funnels yet';

	/// en: 'Create a funnel to define which data the AI should collect during conversations.'
	String get empty_subtitle => 'Create a funnel to define which data the AI should collect during conversations.';

	/// en: 'Create first funnel'
	String get create_first => 'Create first funnel';

	/// en: 'Failed to load funnels'
	String get load_error => 'Failed to load funnels';

	/// en: 'Active'
	String get active_label => 'Active';

	/// en: 'Inactive'
	String get inactive_label => 'Inactive';

	/// en: '$n fields'
	String fields_count({required Object n}) => '${n} fields';

	/// en: 'Name'
	String get field_name => 'Name';

	/// en: 'e.g. Public hospitals in MT'
	String get field_name_hint => 'e.g. Public hospitals in MT';

	/// en: 'Name is required'
	String get name_required => 'Name is required';

	/// en: 'Save'
	String get save => 'Save';

	/// en: 'Funnel created'
	String get created_success => 'Funnel created';

	/// en: 'Funnel updated'
	String get updated_success => 'Funnel updated';

	/// en: 'Funnel deleted'
	String get deleted_success => 'Funnel deleted';

	/// en: 'Delete funnel'
	String get delete_title => 'Delete funnel';

	/// en: 'Delete "$name"? This action cannot be undone.'
	String delete_message({required Object name}) => 'Delete "${name}"? This action cannot be undone.';

	/// en: 'Funnel not found'
	String get not_found => 'Funnel not found';

	/// en: 'It may have been deleted, or the address is wrong.'
	String get not_found_hint => 'It may have been deleted, or the address is wrong.';

	/// en: 'Default funnel — imports and new leads land here; it can't be deleted.'
	String get default_undeletable => 'Default funnel — imports and new leads land here; it can\'t be deleted.';
}

// Path: sales.schema
class Translations$sales$schema$en {
	Translations$sales$schema$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Data to collect'
	String get section_title => 'Data to collect';

	/// en: 'Define the fields the AI should collect during the conversation.'
	String get section_subtitle => 'Define the fields the AI should collect during the conversation.';

	/// en: 'Add field'
	String get add_field => 'Add field';

	/// en: 'Key'
	String get field_key => 'Key';

	/// en: 'e.g. customer_cpf'
	String get field_key_hint => 'e.g. customer_cpf';

	/// en: 'Label'
	String get field_label => 'Label';

	/// en: 'e.g. Customer CPF'
	String get field_label_hint => 'e.g. Customer CPF';

	/// en: 'Type'
	String get field_type => 'Type';

	/// en: 'Required'
	String get field_required => 'Required';

	/// en: 'AI hint'
	String get field_ai_hint => 'AI hint';

	/// en: 'How/when to collect (optional)'
	String get field_ai_hint_hint => 'How/when to collect (optional)';

	/// en: 'Options (comma-separated)'
	String get field_enum_options => 'Options (comma-separated)';

	/// en: 'e.g. small, medium, large'
	String get field_enum_options_hint => 'e.g. small, medium, large';

	/// en: 'Remove field'
	String get remove_field => 'Remove field';

	/// en: 'No fields defined yet.'
	String get no_fields => 'No fields defined yet.';

	/// en: 'Key is required'
	String get error_key_required => 'Key is required';

	/// en: 'Use snake_case (lowercase letters, numbers and _)'
	String get error_key_format => 'Use snake_case (lowercase letters, numbers and _)';

	/// en: 'Duplicate key'
	String get error_key_duplicate => 'Duplicate key';

	/// en: 'Define at least one option'
	String get error_enum_required => 'Define at least one option';

	late final Translations$sales$schema$types$en types = Translations$sales$schema$types$en.internal(_root);
}

// Path: sales.webhook
class Translations$sales$webhook$en {
	Translations$sales$webhook$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Webhook (optional)'
	String get section_title => 'Webhook (optional)';

	/// en: 'Only fill this in if you have your own system to receive these events. The funnel works without it.'
	String get section_subtitle => 'Only fill this in if you have your own system to receive these events. The funnel works without it.';

	/// en: 'Webhook URL'
	String get field_url => 'Webhook URL';

	/// en: 'https://your-server.com/webhooks/replider'
	String get field_url_hint => 'https://your-server.com/webhooks/replider';

	/// en: 'Enter a valid http(s) URL'
	String get url_invalid => 'Enter a valid http(s) URL';

	/// en: 'Secret'
	String get field_secret => 'Secret';

	/// en: 'Used to sign the payload (HMAC)'
	String get field_secret_hint => 'Used to sign the payload (HMAC)';

	/// en: 'Generate'
	String get generate_secret => 'Generate';

	/// en: 'Regenerate'
	String get regenerate_secret => 'Regenerate';

	/// en: 'Secret copied'
	String get secret_copied => 'Secret copied';

	/// en: 'Fire when required fields are complete'
	String get trigger_required_complete => 'Fire when required fields are complete';

	/// en: 'Sends a lead.qualified event once all required fields are filled.'
	String get trigger_required_complete_hint => 'Sends a lead.qualified event once all required fields are filled.';

	/// en: 'Fire on buy intent'
	String get trigger_buy_intent => 'Fire on buy intent';

	/// en: 'Sends a lead.buy_intent event when the AI detects purchase intent.'
	String get trigger_buy_intent_hint => 'Sends a lead.buy_intent event when the AI detects purchase intent.';

	/// en: 'Send test event'
	String get send_test => 'Send test event';

	/// en: 'Save the funnel before sending a test event.'
	String get test_save_first => 'Save the funnel before sending a test event.';

	/// en: 'Set a webhook URL before sending a test event.'
	String get test_url_first => 'Set a webhook URL before sending a test event.';

	/// en: 'Test event sent (HTTP $status)'
	String test_success({required Object status}) => 'Test event sent (HTTP ${status})';

	/// en: 'Test event failed'
	String get test_failed => 'Test event failed';
}

// Path: sales.lead_profile
class Translations$sales$lead_profile$en {
	Translations$sales$lead_profile$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Sales profile'
	String get section_title => 'Sales profile';

	/// en: 'This conversation isn't part of any funnel.'
	String get empty => 'This conversation isn\'t part of any funnel.';

	/// en: 'Funnel'
	String get funnel_label => 'Funnel';

	/// en: 'Lead status'
	String get status_label => 'Lead status';

	/// en: 'Score'
	String get score_label => 'Score';

	/// en: 'Buy intent detected'
	String get buy_intent => 'Buy intent detected';

	/// en: 'Not collected yet'
	String get no_value => 'Not collected yet';

	/// en: 'Pending'
	String get pending => 'Pending';

	/// en: 'AI'
	String get origin_ai => 'AI';

	/// en: 'Human'
	String get origin_human => 'Human';

	/// en: 'Couldn't load the sales profile.'
	String get load_error => 'Couldn\'t load the sales profile.';

	late final Translations$sales$lead_profile$lead_status$en lead_status = Translations$sales$lead_profile$lead_status$en.internal(_root);

	/// en: 'Edit field'
	String get edit_field => 'Edit field';

	/// en: 'Couldn't save: $error'
	String edit_error({required Object error}) => 'Couldn\'t save: ${error}';

	/// en: 'Pick date'
	String get pick_date => 'Pick date';

	/// en: 'New proposal'
	String get new_proposal => 'New proposal';
}

// Path: sales.leads
class Translations$sales$leads$en {
	Translations$sales$leads$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'New'
	String get status_new => 'New';

	/// en: 'Prospected'
	String get status_prospected => 'Prospected';

	/// en: 'Enriching'
	String get status_enriching => 'Enriching';

	/// en: 'Enriched'
	String get status_enriched => 'Enriched';

	/// en: 'Pending approval'
	String get status_pending_approval => 'Pending approval';

	/// en: 'Contacted'
	String get status_contacted => 'Contacted';

	/// en: 'Responded'
	String get status_responded => 'Responded';

	/// en: 'Qualified'
	String get status_qualified => 'Qualified';

	/// en: 'Won'
	String get status_won => 'Won';

	/// en: 'Lost'
	String get status_lost => 'Lost';

	/// en: 'Discard'
	String get discard => 'Discard';

	/// en: 'WhatsApp template'
	String get template_label => 'WhatsApp template';

	/// en: 'Choose a template'
	String get template_select => 'Choose a template';

	/// en: 'Select an approved template before sending'
	String get template_required => 'Select an approved template before sending';

	/// en: 'No approved templates on this channel yet'
	String get template_none => 'No approved templates on this channel yet';

	/// en: 'Link a WhatsApp channel to this funnel first'
	String get template_no_channel => 'Link a WhatsApp channel to this funnel first';

	/// en: 'Couldn't load templates'
	String get template_load_error => 'Couldn\'t load templates';
}

// Path: sales.prospecting
class Translations$sales$prospecting$en {
	Translations$sales$prospecting$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Search Google Places for businesses and import them as leads in a funnel.'
	String get intro => 'Search Google Places for businesses and import them as leads in a funnel.';

	/// en: 'Target funnel'
	String get funnel_label => 'Target funnel';

	/// en: 'Keyword'
	String get keyword_label => 'Keyword';

	/// en: 'e.g. hospital, imaging clinic, ICU'
	String get keyword_hint => 'e.g. hospital, imaging clinic, ICU';

	/// en: 'Location'
	String get location_label => 'Location';

	/// en: 'e.g. Cuiabá, MT'
	String get location_hint => 'e.g. Cuiabá, MT';

	/// en: 'Radius (meters, optional)'
	String get radius_label => 'Radius (meters, optional)';

	/// en: 'e.g. 5000'
	String get radius_hint => 'e.g. 5000';

	/// en: 'Search & import'
	String get search_import => 'Search & import';

	/// en: 'Enter a keyword or location and pick a funnel'
	String get query_required => 'Enter a keyword or location and pick a funnel';

	/// en: 'Create a funnel first to import prospected leads into it.'
	String get no_funnels => 'Create a funnel first to import prospected leads into it.';

	/// en: 'Search complete'
	String get result_title => 'Search complete';

	/// en: '$n of $total results imported as leads.'
	String result_created({required Object n, required Object total}) => '${n} of ${total} results imported as leads.';

	/// en: '$n skipped (already in this funnel).'
	String result_skipped({required Object n}) => '${n} skipped (already in this funnel).';

	/// en: 'View pipeline'
	String get view_pipeline => 'View pipeline';

	/// en: '$total found · $duplicates already in your base'
	String preview_found({required Object total, required Object duplicates}) => '${total} found · ${duplicates} already in your base';

	/// en: '$count leads left this month'
	String quota_left({required Object count}) => '${count} leads left this month';

	/// en: 'already in your base'
	String get already_lead => 'already in your base';

	/// en: 'Import $count leads'
	String import_n({required Object count}) => 'Import ${count} leads';

	/// en: '$count leads imported'
	String imported({required Object count}) => '${count} leads imported';

	/// en: 'Select all'
	String get select_all => 'Select all';

	/// en: 'Clear selection'
	String get select_none => 'Clear selection';

	/// en: '$n of $total selected'
	String selected_of({required Object n, required Object total}) => '${n} of ${total} selected';

	/// en: '$created of $selected imported — your plan's monthly lead allowance stopped the rest.'
	String imported_partial({required Object created, required Object selected}) => '${created} of ${selected} imported — your plan\'s monthly lead allowance stopped the rest.';

	/// en: 'No companies found'
	String get no_results_title => 'No companies found';

	/// en: 'Try another keyword or a wider radius — 'clínica', 'hospital', a neighbouring city.'
	String get no_results_hint => 'Try another keyword or a wider radius — \'clínica\', \'hospital\', a neighbouring city.';
}

// Path: sales.schema.types
class Translations$sales$schema$types$en {
	Translations$sales$schema$types$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'Text'
	String get text => 'Text';

	/// en: 'Number'
	String get number => 'Number';

	/// en: 'CPF'
	String get cpf => 'CPF';

	/// en: 'CNPJ'
	String get cnpj => 'CNPJ';

	/// en: 'Date'
	String get date => 'Date';

	/// en: 'Yes/No'
	String get bool => 'Yes/No';

	/// en: 'Options'
	String get kEnum => 'Options';
}

// Path: sales.lead_profile.lead_status
class Translations$sales$lead_profile$lead_status$en {
	Translations$sales$lead_profile$lead_status$en.internal(this._root);

	final Translations _root; // ignore: unused_field

	// Translations

	/// en: 'New'
	String get kNew => 'New';

	/// en: 'Prospected'
	String get prospected => 'Prospected';

	/// en: 'Enriching'
	String get enriching => 'Enriching';

	/// en: 'Enriched'
	String get enriched => 'Enriched';

	/// en: 'Pending approval'
	String get pending_approval => 'Pending approval';

	/// en: 'Contacted'
	String get contacted => 'Contacted';

	/// en: 'Responded'
	String get responded => 'Responded';

	/// en: 'Qualified'
	String get qualified => 'Qualified';

	/// en: 'Won'
	String get won => 'Won';

	/// en: 'Lost'
	String get lost => 'Lost';
}

/// The flat map containing all translations for locale <en>.
/// Only for edge cases! For simple maps, use the map function of this library.
///
/// The Dart AOT compiler has issues with very large switch statements,
/// so the map is split into smaller functions (512 entries each).
extension on Translations {
	dynamic _flatMapFunction(String path) {
		return switch (path) {
			'home.onboarding_checklist_title' => ({required Object current, required Object total}) => 'Getting started — ${current} of ${total}',
			'home.onboarding_checklist_dismiss' => 'Hide for now',
			'home.onboarding_checklist_agent' => 'Create your first agent',
			'home.onboarding_checklist_preview' => 'Try your agent in the preview chat',
			'home.onboarding_checklist_preview_cta' => 'Try now',
			'home.onboarding_checklist_channel' => 'Connect a messaging channel',
			'home.onboarding_checklist_channel_cta' => 'Connect',
			'home.onboarding_checklist_knowledge' => 'Teach your agent about your business',
			'home.onboarding_checklist_knowledge_cta' => 'Add context',
			'home.onboarding_checklist_inbound' => 'Receive your first real message',
			'home.welcome_user' => ({required Object name}) => 'Welcome, ${name}',
			'home.onboarding_checklist_products' => 'Bring in what you sell',
			'home.onboarding_checklist_products_cta' => 'Import price list',
			'home.onboarding_checklist_customers' => 'Bring in your customers',
			'home.onboarding_checklist_customers_cta' => 'Import spreadsheet',
			'home.onboarding_checklist_proposal' => 'Make your first proposal',
			'home.onboarding_checklist_proposal_cta' => 'Open the funnel',
			'home.onboarding_checklist_team' => 'Invite your seller',
			'home.onboarding_checklist_team_cta' => 'Invite',
			'auth.login_hero_title' => 'Offers ready to approve, every day.',
			'auth.login_eyebrow' => 'YOUR AUTONOMOUS SELLER',
			'auth.login_title' => 'Sign in to your account',
			'auth.email_placeholder' => 'Enter your email',
			'auth.password_placeholder' => 'Enter your password',
			'auth.remember_me' => 'Remember me',
			'auth.forgot_password' => 'Forgot password',
			'auth.login_button' => 'Login',
			'auth.or_login_with' => 'Or',
			'auth.google' => 'Google',
			'auth.meta' => 'Facebook',
			'auth.dont_have_account' => 'Don\'t have an account?',
			'auth.create_account' => 'Create an account',
			'auth.register_title' => 'Create Account',
			'auth.register_subtitle' => 'Start using Replider today',
			'auth.register_paused_title' => 'Sign-ups opening soon',
			'auth.register_paused_body' => 'We\'re building the new Replider and sign-ups are temporarily closed. Already have an account? Just log in.',
			'auth.register_paused_error' => 'Sign-ups are temporarily closed.',
			'auth.account_name' => 'Company name',
			'auth.full_name' => 'Full name',
			'auth.confirm_password' => 'Confirm password',
			'auth.already_have_account' => 'Already have an account?',
			'auth.sign_in' => 'Sign in',
			'auth.field_required' => 'Required field',
			'auth.invalid_email' => 'Invalid email',
			'auth.password_too_short' => 'Password too short',
			'auth.min_characters' => ({required Object n}) => 'Minimum ${n} characters',
			'auth.passwords_dont_match' => 'Passwords don\'t match',
			'auth.forgot_title' => 'Reset password',
			'auth.forgot_subtitle' => 'Enter your email and we\'ll send you reset instructions.',
			'auth.forgot_send_link' => 'Send reset link',
			'auth.forgot_resend_in' => ({required Object seconds}) => 'Resend in ${seconds}s',
			'auth.forgot_success' => 'If an account exists, you\'ll receive an email shortly.',
			'auth.reset_title' => 'Set a new password',
			'auth.reset_subtitle' => 'Choose a new password for your account.',
			'auth.reset_new_password' => 'New password',
			'auth.reset_confirm_password' => 'Confirm new password',
			'auth.reset_min_length' => 'Password must be at least 8 characters.',
			'auth.reset_mismatch' => 'Passwords don\'t match.',
			'auth.reset_submit' => 'Reset password',
			'auth.reset_success' => 'Password reset! You can now log in with your new password.',
			'auth.reset_invalid_link' => 'This reset link is invalid or incomplete. Request a new one.',
			'auth.back_to_login' => 'Back to login',
			'auth.terms_required' => 'You must accept the Terms of Service and Privacy Policy.',
			'auth.retry' => 'Try again',
			'auth.mfa_title' => 'Two-factor verification',
			'auth.mfa_subtitle' => 'Enter the 6-digit code from your authenticator app, or one of your recovery codes.',
			'auth.mfa_code_hint' => 'Code',
			'auth.mfa_verify_button' => 'Verify and continue',
			'auth.mfa_cancel' => 'Back to sign in',
			'auth.coupon_banner_valid' => ({required Object code, required Object percent}) => 'Coupon ${code} applied — ${percent}% off at checkout.',
			'auth.coupon_banner_invalid' => ({required Object code}) => 'Coupon ${code} is no longer valid.',
			'auth.session_expired_notice' => 'Your session expired for security. Sign in again — you\'ll return to the screen you were on.',
			'auth.session_ending_title' => ({required Object minutes}) => 'Your session ends in ${minutes} min',
			'auth.session_ending_body' => 'Sign in again now, on your own terms, instead of losing whatever you are in the middle of.',
			'auth.session_ending_cta' => 'Sign in again',
			'auth.invite_banner' => ({required Object email}) => 'Invite for ${email} — your account starts on the contracted plan.',
			'auth.invite_invalid' => 'This invite link is invalid, expired or already used. Ask for a new link.',
			'auth.invite_invalid_title' => 'Invite link expired',
			'auth.register_subtitle_invite' => 'Activate the account on the plan we agreed with you.',
			'auth.invite_probe_failed' => 'We couldn\'t verify your invite right now — a connection problem, not the link. Try again.',
			'auth.verify_email_unknown' => 'your account\'s e-mail',
			'auth.consent_prefix' => 'I have read and accept the ',
			'auth.consent_terms' => 'Terms of Use',
			'auth.consent_and' => ' and the ',
			'auth.consent_privacy' => 'Privacy Policy',
			'profile.title' => 'My Profile',
			'profile.user_not_found' => 'User not found',
			'profile.personal_info' => 'PERSONAL INFORMATION',
			'profile.name' => 'Name',
			'profile.email' => 'Email',
			'profile.save_info' => 'Save Information',
			'profile.change_password' => 'CHANGE PASSWORD',
			'profile.current_password' => 'Current password',
			'profile.new_password' => 'New password',
			'profile.confirm_new_password' => 'Confirm new password',
			'profile.change_password_button' => 'Change Password',
			'profile.fill_all_fields' => 'Fill in all fields',
			'profile.fill_all_password_fields' => 'Fill in all password fields',
			'profile.passwords_dont_match' => 'Passwords don\'t match',
			'profile.min_password_length' => 'New password must be at least 6 characters',
			'profile.profile_updated' => 'Profile updated successfully!',
			'profile.password_changed' => 'Password changed successfully!',
			'profile.profile_update_unavailable' => 'Profile update not yet available in the API',
			'profile.password_change_unavailable' => 'Password change not yet available in the API',
			'profile.incorrect_current_password' => 'Incorrect current password',
			'profile.unexpected_error' => 'Unexpected error',
			'profile.network_error' => 'Network error',
			'profile.error_prefix' => 'Error',
			'profile.two_factor_section_label' => 'TWO-FACTOR AUTHENTICATION',
			'profile.two_factor_label' => 'Two-factor authentication',
			'profile.two_factor_desc' => 'Adds a second step at login using a 6-digit code from an authenticator app (Google Authenticator, 1Password, Authy).',
			'profile.two_factor_status_off' => 'OFF',
			'profile.two_factor_status_on' => 'ON',
			'profile.two_factor_setup_cta' => 'Set up 2FA',
			'profile.two_factor_step1' => '1. Scan this QR with your authenticator app',
			'profile.two_factor_step2' => '2. Enter the 6-digit code shown in the app',
			'profile.two_factor_manual_secret_label' => 'Or enter this secret manually:',
			'profile.two_factor_verify_cta' => 'Verify and enable',
			'profile.two_factor_disable_prompt' => 'Enter a current 2FA code (or recovery code) to disable.',
			'profile.two_factor_code_hint' => '6-digit code or recovery code',
			'profile.two_factor_disable_cta' => 'Disable 2FA',
			'profile.two_factor_recovery_title' => 'Save your recovery codes',
			'profile.two_factor_recovery_warning' => 'Each code works once. Store them somewhere safe — without them, losing your phone locks you out.',
			'profile.two_factor_recovery_copy_cta' => 'Copy all',
			'profile.two_factor_recovery_done_cta' => 'I\'ve saved them',
			'profile.signature_label' => 'Signature',
			'profile.signature_help' => 'Optional sign-off offered in the composer on manual replies. Never added to AI responses.',
			'onboarding.welcome_title' => 'Welcome to Replider!',
			'onboarding.welcome_subtitle' => 'Let\'s set up your workspace in a few quick steps.',
			'onboarding.company_name_hint' => 'Your company name',
			'onboarding.step_company_title' => 'About your company',
			'onboarding.step_company_subtitle' => 'The AI writes its outreach from this. Without it, all it knows is the name.',
			'onboarding.step_company_placeholder' => 'e.g. Your Company Hospital Equipment',
			'onboarding.company_about_label' => 'What you do',
			'onboarding.company_about_placeholder' => 'e.g. Sales, rental and technical service of hospital equipment in Mato Grosso.',
			'onboarding.company_about_help' => 'One sentence. Who you serve and what you solve.',
			'onboarding.company_tone_label' => 'How you speak (optional)',
			'onboarding.company_lines_label' => 'How does your company earn?',
			'onboarding.company_line_sell' => 'I sell equipment',
			'onboarding.company_line_rent' => 'I rent equipment',
			'onboarding.company_line_service' => 'I do technical service',
			'onboarding.company_territory_label' => 'Where do you sell?',
			'onboarding.company_territory_uf' => 'State',
			'onboarding.company_territory_cities' => 'Cities (optional)',
			'onboarding.company_territory_cities_hint' => 'Cuiabá, Várzea Grande',
			'onboarding.company_tone_direct' => 'Direct',
			'onboarding.company_tone_direct_prose' => 'Write directly and to the point, no filler and no sales adjectives.',
			'onboarding.company_tone_consultative' => 'Consultative',
			'onboarding.company_tone_consultative_prose' => 'Write like a specialist who understands the customer\'s problem, explaining the why before the what.',
			'onboarding.company_tone_warm' => 'Warm',
			'onboarding.company_tone_warm_prose' => 'Write warmly and personally, like someone who already knows them, without being over-familiar.',
			'onboarding.company_tone_formal' => 'Formal',
			'onboarding.company_tone_formal_prose' => 'Write formally and respectfully, avoiding slang.',
			'onboarding.step_agent_subtitle' => 'Give your agent a name and choose its AI brain.',
			'onboarding.provider_label' => 'AI Provider',
			'onboarding.step_channel_title' => 'Where do you send from?',
			'onboarding.step_channel_subtitle' => 'The first offer goes out by email: connect the account you already use. WhatsApp comes later, for the relationship.',
			'onboarding.step_channel_badge_telegram' => 'Fastest',
			'onboarding.step_channel_badge_whatsapp' => 'Popular · ~10 min',
			'onboarding.step_channel_badge_coming_soon' => 'Coming soon',
			'onboarding.step_channel_whatsapp_help' => 'Not sure where to start?',
			'onboarding.step_channel_whatsapp_help_failed' => 'Couldn\'t open the help video. Please check your browser settings.',
			'onboarding.telegram_bot_token' => 'Telegram Bot Token',
			'onboarding.whatsapp_phone_number_id' => 'WhatsApp Phone Number ID',
			'onboarding.whatsapp_access_token' => 'WhatsApp Access Token',
			'onboarding.instagram_page_id' => 'Instagram Page ID',
			'onboarding.instagram_access_token' => 'Instagram Access Token',
			'onboarding.step_channel_instagram_hint' => 'Requires an Instagram professional account (Business or Creator) and an access token with the instagram_business_manage_messages permission.',
			'onboarding.sms_account_sid' => 'Twilio Account SID',
			'onboarding.sms_auth_token' => 'Twilio Auth Token',
			'onboarding.sms_from_number' => 'Twilio Phone Number (E.164)',
			'onboarding.step_channel_sms_hint' => 'Bring your own Twilio number: paste the Account SID, Auth Token, and phone number from the Twilio console. After finishing, copy the channel\'s webhook URL in Settings → Channels and set it on your number in Twilio.',
			'onboarding.invalid_sms_account_sid' => 'The Account SID must start with AC followed by 32 characters. Copy it from the Twilio console.',
			'onboarding.invalid_sms_auth_token' => 'Paste your Twilio Auth Token to connect the SMS channel.',
			'onboarding.invalid_sms_from_number' => 'The phone number must be in international E.164 format, e.g. +15551234567.',
			'onboarding.create_agent_first_for_file' => 'Create the agent before uploading the file.',
			'onboarding.create_agent_first_for_url' => 'Create the agent before importing the URL.',
			'onboarding.knowledge_summary_file' => ({required Object fileName, required Object count}) => '${fileName} · ${count} snippets indexed',
			'onboarding.knowledge_summary_url' => ({required Object pages, required Object count}) => '${pages} pages · ${count} snippets indexed',
			'onboarding.step_done_title' => 'You\'re all set!',
			'onboarding.step_done_subtitle' => 'Your workspace is ready. Messages will start flowing as soon as customers reach out.',
			'onboarding.step_done_tip_1' => 'Test your agent in Settings → Agent → Preview',
			'onboarding.step_done_tip_2' => 'Monitor conversations in the Dashboard',
			'onboarding.step_done_tip_3' => 'Add more knowledge anytime in Settings',
			'onboarding.go_to_dashboard' => 'Go to Dashboard',
			'onboarding.next' => 'Next',
			'onboarding.back' => 'Back',
			'onboarding.skip' => 'Skip for now',
			'onboarding.step_of' => ({required Object current, required Object total}) => 'Step ${current} of ${total}',
			'onboarding.field_required' => 'Required field',
			'onboarding.step_preview_chat_next' => 'It works! Next',
			'onboarding.step_preview_chat_skip' => 'Test later',
			'onboarding.step_channel_whatsapp_manual_hint' => 'Enter the Phone Number ID from Meta for Developers. The access token is managed automatically.',
			'onboarding.step_channel_whatsapp_connected' => 'WhatsApp connected! Click "Next" to continue.',
			'onboarding.step_channel_whatsapp_or_manual' => 'or connect manually',
			'onboarding.step_channel_badge_email' => 'Main',
			'onboarding.email_from_address' => 'Sending address',
			'onboarding.email_smtp_host' => 'SMTP server',
			'onboarding.email_smtp_port' => 'Port',
			'onboarding.email_smtp_password' => 'SMTP password',
			'onboarding.email_dns_title' => 'Next: authorise your domain to send',
			'onboarding.email_dns_body' => 'Your domain must publish three records (SPF, DKIM and DMARC) so the mail reaches the inbox instead of spam. When you finish, Replider shows the exact records to copy. Propagation takes minutes to a few hours.',
			'onboarding.products_title' => 'What do you sell?',
			'onboarding.products_subtitle' => 'Bring your price list as a file. Every proposal is made of these, and you can add or fix them later.',
			'onboarding.aikey_title' => 'Your AI key',
			'onboarding.aikey_subtitle' => 'On your contract the AI runs on YOUR key — the usage is yours, with nobody in between. Paste the Google AI Studio (Gemini) key so it can read catalogues and write offers.',
			'onboarding.aikey_field' => 'API key (Gemini)',
			'onboarding.aikey_hint' => 'Stored encrypted and never shown back — only the last digits.',
			'onboarding.aikey_saved' => 'Key configured ✓',
			'onboarding.aikey_saved_hint' => 'This account already has a key. You can move on.',
			'onboarding.aikey_save' => 'Save key',
			'onboarding.aikey_skip_note' => 'You can move on without one: the price list comes in as CSV, which uses no AI. Catalogue PDFs and AI offers wait until you paste a key.',
			'onboarding.products_needs_key' => 'Reading a PDF needs the account\'s AI key. For now bring the list as CSV — it uses no AI.',
			'onboarding.email_smtp_toggle' => 'I have my own mail server (SMTP)',
			'onboarding.step_channel_other_channels' => 'Other channels',
			'onboarding.step_channel_other_channels_hide' => 'Hide other channels',
			'onboarding.customers_title' => 'Who already buys from you?',
			'onboarding.customers_subtitle' => 'Bring your customer list as a file. Repurchases, cross-sell and the first offer all start from it — you can add more later.',
			'onboarding.first_offer_title' => 'Your first offer',
			'onboarding.first_offer_writing' => 'Reading your products and your customers and writing the first message…',
			'onboarding.first_offer_for_customer' => ({required Object name}) => 'For ${name}, one of your customers:',
			'onboarding.first_offer_for_prospect' => ({required Object name}) => 'For ${name}, found in your territory:',
			'onboarding.first_offer_in_queue' => 'This is how opportunities will arrive every day. This one is already in your Opportunities list — approve it, edit it, or ask for another approach there. Nothing leaves without you.',
			'onboarding.first_offer_nothing' => 'Nothing to write yet: bring a customer list, or say which state you sell in, and the first opportunity will be waiting in Opportunities.',
			'onboarding.first_offer_no_business' => 'We could not find a company in your territory right now. Bring a customer list, or try the search in Opportunities later.',
			'onboarding.finish_to_queue' => 'See my offer queue',
			'onboarding.unconfirmed_import_title' => 'File not imported',
			'onboarding.unconfirmed_import_body' => 'You reviewed the file but haven\'t confirmed the import. If you continue now, nothing will be created.',
			'onboarding.unconfirmed_import_confirm' => 'Continue without importing',
			'onboarding.company_required_hint' => 'Fields marked * are required — Next unlocks once all four are filled.',
			'onboarding.first_offer_import_cta' => 'Go back and import customers',
			'onboarding.first_offer_unavailable' => 'Company search isn\'t available right now — import a customer list, or skip: prospecting works normally later.',
			'onboarding.first_offer_prospecting_not_configured' => 'Company search isn\'t set up on the platform yet — import a customer list, or skip: prospecting is enabled later.',
			'onboarding.first_offer_cta_products' => 'Import products',
			'onboarding.first_offer_cta_customers' => 'Fix the customer list',
			'onboarding.first_offer_cta_settings' => 'Open Settings',
			'email_verification.title' => 'Check your email',
			'email_verification.subtitle' => 'We\'ve sent a verification link to',
			'email_verification.instruction' => 'Click the link in the email to activate your account. The link expires in 24 hours.',
			'email_verification.back_to_login' => 'Back to Login',
			'email_verification.didnt_receive' => 'Didn\'t receive the email?',
			'email_verification.resend' => 'Resend email',
			'email_verification.resent' => 'Email sent. Check your inbox in a minute.',
			'email_verification.resend_failed' => 'Could not resend. Try again in a moment.',
			'email_verification.check_spam' => 'Also check your spam folder.',
			'email_verification.banner_message' => 'Confirm your email to secure your account.',
			'email_verification.verifying' => 'Verifying your email…',
			'email_verification.verified_title' => 'Email verified',
			'email_verification.verified_subtitle' => 'Your email is confirmed. You can log in now.',
			'email_verification.verify_failed_title' => 'Verification failed',
			'email_verification.verify_failed_subtitle' => 'This link is invalid or has expired. Request a new one from the app.',
			'dashboard.conversations_title' => 'Conversations',
			'dashboard.unknown_contact' => 'Customer',
			'dashboard.reply_reminder_toast' => ({required Object contact, required Object minutes}) => '${contact} has been waiting ${minutes} min — reply now',
			'dashboard.reply_reminder_open' => 'Open',
			'dashboard.search_conversations' => 'Search conversations...',
			'dashboard.search_in_conversation' => 'Search in conversation...',
			'dashboard.no_conversation_selected' => 'Select a conversation',
			'dashboard.no_conversation_selected_subtitle' => 'Pick a conversation from the list to view its messages.',
			'dashboard.settings' => 'Settings',
			'dashboard.message_placeholder' => 'Type a message...',
			'dashboard.no_conversations_found' => 'No conversations found',
			'dashboard.empty_inbox_hint_setup' => 'Your inbox is empty. Connect an email to receive and reply to customer messages here.',
			'dashboard.empty_inbox_cta_setup' => 'Connect email',
			'dashboard.empty_inbox_hint_share' => 'Your inbox is empty. Customer replies to your offers show up here.',
			'dashboard.empty_inbox_cta_share' => 'View offers',
			'dashboard.no_messages' => 'No messages',
			'dashboard.today' => 'Today',
			'dashboard.yesterday' => 'Yesterday',
			'dashboard.now' => 'Now',
			'dashboard.bot_active' => 'AI active',
			'dashboard.waiting_agent' => 'Waiting for agent',
			'dashboard.closed' => 'Closed',
			'dashboard.agent_label' => 'Agent',
			'dashboard.search_hint' => 'Search...',
			'dashboard.close_search' => 'Close search',
			'dashboard.search' => 'Search',
			'dashboard.analytics' => 'Analytics',
			'dashboard.settings_tooltip' => 'Settings',
			'dashboard.search_in_messages' => 'Search...',
			'dashboard.no_conversations_for_query' => ({required Object query}) => 'No conversations matching "${query}"',
			'dashboard.results_for_query' => ({required Object count, required Object query}) => '${count} result(s) for "${query}"',
			'dashboard.error_loading_conversations' => 'Failed to load conversations',
			'dashboard.check_connection' => 'Check your connection and try again.',
			'dashboard.filter_all' => 'All',
			'dashboard.filter_open' => 'Open',
			'dashboard.filter_pending' => 'Pending',
			'dashboard.filter_closed' => 'Closed',
			'dashboard.assignment_mine' => 'Mine',
			'dashboard.assignment_unassigned' => 'Unassigned',
			'dashboard.assignment_all' => 'All',
			'dashboard.sort_recent' => 'Recent',
			'dashboard.sort_first_in_conv' => 'Order received',
			'dashboard.sort_oldest' => 'Oldest',
			'dashboard.select_conversation_title' => 'Select a conversation',
			'dashboard.select_conversation_subtitle' => 'Pick a conversation from the list to\nview the messages.',
			'dashboard.drop_to_send' => 'Drop to send',
			'dashboard.search_messages_hint' => 'Search...',
			'dashboard.summary_tooltip' => 'Conversation summary (AI)',
			'dashboard.send_file_tooltip' => 'Send file',
			'dashboard.record_audio_tooltip' => 'Record audio',
			'dashboard.send_flow_tooltip' => 'Send Flow',
			'dashboard.type_a_message' => 'Type a message...',
			'dashboard.bot_paused_pill' => 'AI paused',
			'dashboard.human_handling' => 'Under human handling',
			'dashboard.status_open_bot' => 'AI active',
			'dashboard.status_pending_human' => 'Waiting for agent',
			'dashboard.status_closed' => 'Closed',
			'dashboard.day_today' => 'Today',
			'dashboard.day_yesterday' => 'Yesterday',
			'dashboard.summary_title' => 'Conversation summary',
			'dashboard.summary_failed' => ({required Object error}) => 'Failed to generate summary: ${error}',
			'dashboard.summary_empty' => 'Not enough messages to summarize.',
			'dashboard.summary_refresh' => 'Refresh',
			'dashboard.wa_24h_window_expired' => '24h window expired. Send an approved template to reopen the conversation.',
			'dashboard.wa_send_template' => 'Send template',
			'dashboard.retry_send' => 'Retry',
			'dashboard.flow_sent' => ({required Object name}) => 'Flow "${name}" sent.',
			'dashboard.token_usage_warning_snackbar' => ({required Object percent}) => 'Token usage at ${percent}% of limit.',
			'dashboard.reconnecting' => 'Reconnecting...',
			'dashboard.tokens_exhausted_banner' => 'AI credits exhausted. Buy an extra pack or upgrade.',
			'dashboard.tokens_high_usage_banner' => ({required Object percent}) => 'You\'ve used ${percent}% of your AI credits this month.',
			'dashboard.upgrade_btn' => 'Talk to us',
			'dashboard.trial_ended' => 'Your trial period has ended. Upgrade to keep using Replider.',
			'dashboard.trial_last_day' => 'Last day of free trial. Upgrade to keep service running.',
			'dashboard.trial_days_remaining' => ({required Object days, required Object used, required Object max}) => '${days} days left in your free trial (${used}/${max} conversations). Upgrade to unlock more.',
			'dashboard.trial_free_plan' => ({required Object used, required Object max}) => 'You\'re on a trial (${used} of ${max} conversations used). Want the full product working for your company? Let\'s talk.',
			'dashboard.insert_signature_tooltip' => 'Insert signature',
			'dashboard.channel_all' => 'All channels',
			'dashboard.channel_email' => 'Email',
			'dashboard.channel_whatsapp' => 'WhatsApp',
			'dashboard.channel_telegram' => 'Telegram',
			'dashboard.channel_instagram' => 'Instagram',
			'dashboard.channel_sms' => 'SMS',
			'dashboard.sort_importance' => 'Most important',
			'dashboard.category_all' => 'All categories',
			'dashboard.category_hot_lead' => 'Hot lead',
			'dashboard.category_customer' => 'Customer',
			'dashboard.category_financial' => 'Financial',
			'dashboard.category_supplier' => 'Supplier',
			'dashboard.category_newsletter' => 'Newsletter',
			'dashboard.category_quarantine' => 'Quarantine',
			'dashboard.category_other' => 'Other',
			'dashboard.filters_button' => 'Filters',
			'dashboard.filters_button_n' => ({required Object count}) => 'Filters · ${count}',
			'dashboard.filters_clear' => 'Clear filters',
			'dashboard.filters_title' => 'Filter conversations',
			'dashboard.filter_label_status' => 'Status',
			'dashboard.filter_label_channel' => 'Channel',
			'dashboard.filter_label_sort' => 'Order',
			'dashboard.filter_label_category' => 'Category',
			'dashboard.no_conversations_for_filters' => 'No conversations match these filters.',
			'dashboard.clear_filters' => 'Clear filters',
			'dashboard.media_file_label' => 'File',
			'dashboard.media_location_label' => 'Location',
			'dashboard.wa_unsupported_title' => 'Content not delivered by WhatsApp',
			'dashboard.wa_unsupported_body' => 'Polls, live locations and view-once media never reach the service side. Ask the customer to resend it another way.',
			'dashboard.wa_window_composer_back' => 'The message field comes back when the customer replies.',
			'dashboard.brief_agreed' => 'AGREED',
			'dashboard.brief_pending' => 'PENDING',
			'dashboard.brief_next_step' => 'NEXT STEP',
			'dashboard.brief_generated_now' => 'generated just now',
			'dashboard.brief_generated_ago' => ({required Object when}) => 'generated ${when} ago',
			'dashboard.brief_copied' => 'Summary copied.',
			'dashboard.summary_too_short' => 'This conversation is still short — open the messages, the summary would be longer than the thread.',
			'settings.title' => 'Settings',
			'settings.account' => 'Account',
			'settings.subscription' => 'Subscription',
			'settings.team' => 'Team',
			'settings.manage_team' => 'Manage Team',
			'settings.manage_team_subtitle' => 'Invite members and manage roles',
			'settings.ai_agents' => 'AI Agents',
			'settings.kNew' => 'New',
			'settings.no_agents' => 'No agents configured',
			'settings.no_agent_selected' => 'No agent selected',
			'settings.error_loading_team' => 'Failed to load team',
			'settings.error_loading_tags' => 'Failed to load tags',
			'settings.error_loading_responses' => 'Failed to load canned responses',
			'settings.error_loading_calendars' => 'Failed to load calendars',
			'settings.error_loading_templates' => 'Failed to load templates',
			'settings.error_loading_webhooks' => 'Failed to load',
			'settings.agent_not_found' => 'Agent not found',
			'settings.create_agent' => 'Create Agent',
			'settings.active' => 'Active',
			'settings.inactive' => 'Inactive',
			'settings.error_loading_agents' => 'Error loading agents',
			'settings.retry' => 'Try again',
			'settings.logout' => 'Log out',
			'settings.plan_and_usage' => 'Plan & Usage',
			'settings.manage_plan' => 'Manage your plan and track usage',
			'settings.delete_agent_title' => 'Delete Agent',
			'settings.delete_agent_confirm' => ({required Object name}) => 'Are you sure you want to delete "${name}"? This action cannot be undone.',
			'settings.cancel' => 'Cancel',
			'settings.delete' => 'Delete',
			'settings.agent_deleted' => 'Agent deleted successfully',
			'settings.new_agent' => 'New Agent',
			'settings.agent_name' => 'Agent name',
			'settings.ai_provider' => 'AI Provider',
			'settings.create' => 'Create',
			'settings.agent_created' => 'Agent created successfully!',
			'settings.delete_agent_tooltip' => 'Delete agent',
			'settings.no_channels' => 'No channels connected',
			'settings.no_messages' => 'No messages',
			'settings.typing' => 'typing...',
			'settings.account_subtitle' => 'Company name, workspace and danger zone',
			'settings.company_section_title' => 'Company',
			'settings.company_name_label' => 'Company name',
			'settings.company_name_helper' => 'How the AI agent introduces itself and how your workspace appears to your team.',
			'settings.company_save_success' => 'Company name updated',
			'settings.company_save_cta' => 'Save',
			'settings.branding_section_title' => 'Branding',
			'settings.branding_section_subtitle' => 'Logo and timezone used by the agent.',
			'settings.branding_logo_label' => 'Workspace logo',
			'settings.branding_logo_helper' => 'PNG or JPG, up to 2 MB. Shown in the header and future customer channels.',
			'settings.branding_logo_upload_cta' => 'Upload logo',
			'settings.branding_logo_change_cta' => 'Replace',
			'settings.branding_logo_remove_cta' => 'Remove',
			'settings.branding_logo_uploading' => 'Uploading logo...',
			'settings.branding_logo_too_large' => 'Logo larger than 2 MB',
			'settings.branding_logo_upload_failed' => 'Failed to upload logo',
			'settings.branding_timezone_label' => 'Default timezone',
			'settings.branding_timezone_helper' => 'Default when creating new agents. Existing agents keep their current timezone.',
			'settings.branding_save_cta' => 'Save branding',
			'settings.branding_save_success' => 'Branding updated',
			'settings.danger_zone_title' => 'Danger zone',
			'settings.danger_zone_subtitle' => 'Irreversible actions on the workspace.',
			'settings.delete_account_cta' => 'Delete account permanently',
			'settings.delete_account_dialog_title' => ({required Object company}) => 'Delete ${company}?',
			'settings.delete_account_dialog_body' => 'This permanently removes every agent, channel, conversation and message. It cannot be undone. Type the company name below to confirm.',
			'settings.delete_account_type_hint' => ({required Object company}) => 'Type "${company}" to unlock',
			'settings.delete_account_confirm_button' => 'Delete workspace',
			'settings.delete_account_success' => 'Account deleted.',
			'settings.agents_subtitle' => 'Manage the AI agents that reply to customers',
			'settings.channels_subtitle' => 'E-mail for offers · WhatsApp for the relationship',
			'settings.tags_subtitle' => 'Organize conversations by theme or priority',
			'settings.canned_responses_subtitle' => 'Quick replies attendants can reuse',
			'settings.campaigns_subtitle' => 'Outbound templates (WhatsApp)',
			'settings.compliance_subtitle' => 'LGPD and audit log',
			'settings.smtp_test_title' => 'SMTP test',
			'settings.smtp_test_subtitle' => 'Sends an email to you to confirm the server can deliver messages.',
			'settings.smtp_test_cta' => 'Send test email',
			'settings.smtp_test_success' => 'Test email sent. Check your inbox.',
			'settings.smtp_test_not_configured' => 'SMTP is not configured on the server.',
			'settings.smtp_test_failed' => 'Failed to send test email',
			'settings.wa_templates_pending_banner_one' => '1 template under Meta review',
			'settings.wa_templates_pending_banner_other' => ({required Object count}) => '${count} templates under Meta review',
			'settings.wa_templates_polling_hint' => 'Refreshing every 30 seconds.',
			'settings.wa_phone_numbers_title' => 'Numbers under WABA',
			'settings.wa_phone_numbers_subtitle' => 'All phone numbers registered under your WhatsApp Business Account. Each number can become its own Replider channel.',
			'settings.wa_phone_numbers_empty' => 'No other numbers found in this WABA.',
			'settings.wa_phone_numbers_failed' => 'Failed to list numbers',
			'settings.wa_phone_numbers_current' => 'Currently connected',
			'settings.audit_filter_action' => 'Action (exact)',
			'settings.audit_filter_entity' => 'Entity type',
			'settings.audit_filter_actor' => 'Actor ID',
			'settings.audit_filter_apply' => 'Apply',
			'settings.audit_filter_clear' => 'Clear',
			'settings.audit_export_cta' => 'Export CSV',
			'settings.audit_export_copied' => 'CSV copied. Paste into a .csv file to open in Excel.',
			'settings.audit_export_failed' => 'Failed to export CSV',
			'settings.calendar_test_connection_cta' => 'Test connection',
			'settings.calendar_test_connection_ok' => 'Connected successfully',
			'settings.calendar_test_connection_fail' => 'Connection failed',
			'settings.calendar_disconnect_cta' => 'Disconnect Google',
			'settings.calendar_disconnect_confirm_title' => ({required Object name}) => 'Disconnect ${name} from Google?',
			'settings.calendar_disconnect_confirm_body' => 'The calendar stays in Replider but the agent will no longer pull Google events. You can reconnect later by editing the calendar.',
			'settings.calendar_disconnect_confirm_cta' => 'Disconnect',
			'settings.calendar_disconnect_success' => 'Disconnected from Google.',
			'settings.gcal_title' => 'Google Calendar',
			'settings.gcal_connect_desc' => 'Connect your Google account so the agent can check free slots and create events directly on your calendar.',
			'settings.gcal_connect_cta' => 'Connect Google Calendar',
			'settings.gcal_connected_as' => ({required Object email}) => 'Connected as ${email}',
			'settings.gcal_disconnect_cta' => 'Disconnect',
			'settings.gcal_disconnect_confirm_title' => 'Disconnect Google Calendar?',
			'settings.gcal_disconnect_confirm_body' => 'The agent loses access to your calendar. Bookings fall back to the calendar shared with the platform.',
			'settings.gcal_open_browser' => 'Finish connecting in the browser tab we opened. Come back here when you\'re done.',
			'settings.gcal_connected_snackbar' => 'Google Calendar connected!',
			'settings.gcal_error_snackbar' => 'Failed to connect Google Calendar. Try again.',
			'settings.gcal_denied_snackbar' => 'Google connection cancelled.',
			'settings.gcal_pick_calendar_label' => 'Google calendar',
			'settings.gcal_pick_calendar_hint' => 'Choose which calendar appointments are booked on',
			'settings.gcal_manual_id_toggle' => 'Enter ID manually',
			'settings.team_metrics_period_today' => 'Today',
			'settings.team_metrics_period_7d' => '7d',
			'settings.team_metrics_period_30d' => '30d',
			'settings.team_metrics_period_mtd' => 'MTD',
			'settings.team_metrics_col_convs' => 'Convs',
			'settings.team_metrics_col_tmpr' => 'TTFR',
			'settings.team_metrics_col_csat' => 'CSAT',
			'settings.team_metrics_status_online' => 'Online',
			'settings.team_metrics_status_offline' => 'Offline',
			'settings.team_metrics_export_cta' => 'Export CSV',
			'settings.team_metrics_export_copied' => 'CSV copied. Paste into a .csv file to open in Excel.',
			'settings.team_metrics_export_failed' => 'Failed to export',
			'settings.team_metrics_drill_title' => ({required Object name}) => '${name}\'s metrics',
			'settings.team_metrics_card_volume' => 'Volume',
			'settings.team_metrics_card_speed' => 'Speed',
			'settings.team_metrics_card_quality' => 'Quality',
			'settings.team_metrics_card_activity' => 'Activity',
			'settings.team_metrics_conversations_assigned' => 'Conversations assigned',
			'settings.team_metrics_conversations_resolved' => 'Resolved',
			'settings.team_metrics_messages_sent' => 'Messages sent',
			'settings.team_metrics_avg_first_response' => 'Avg first response time',
			'settings.team_metrics_avg_resolution' => 'Avg resolution time',
			'settings.team_metrics_csat_avg' => 'CSAT avg',
			'settings.team_metrics_csat_count' => 'Responses',
			'settings.team_metrics_last_seen' => 'Last activity',
			'settings.team_metrics_empty' => 'No data for the selected period.',
			'settings.team_metrics_no_data' => '—',
			'settings.live_supervision_online' => 'Online',
			'settings.live_supervision_offline' => 'Offline',
			'settings.channels_title' => 'E-mail & channels',
			_ => null,
		} ?? switch (path) {
			'settings.business_hours_title' => 'Business hours',
			'settings.business_hours_enabled' => 'Only reply during business hours',
			'settings.business_hours_enabled_subtitle' => 'Outside the window the AI stays quiet — attendants can still reply manually.',
			'settings.business_hours_start' => 'Start',
			'settings.business_hours_end' => 'End',
			'settings.business_hours_days' => 'Days of the week',
			'settings.business_hours_save_success' => 'Business hours saved',
			'settings.back_tooltip' => 'Back',
			'settings.plan_expired_banner' => 'Your contract is inactive — sending and edits are paused. Talk to us to reactivate.',
			'settings.view_plans' => 'View plans',
			'settings.channels_count_offline' => ({required Object count}) => '${count} offline',
			'settings.section_channels' => 'Channels',
			'settings.section_attendance' => 'Support',
			'settings.section_compliance' => 'Compliance',
			'settings.campaigns_title' => 'Campaigns',
			'settings.flows_title' => 'WhatsApp Flows',
			'settings.flows_subtitle' => 'Native forms for lead capture and bookings.',
			'settings.webhooks_title' => 'Webhooks',
			'settings.webhooks_subtitle' => 'Connect Zapier, n8n, your CRM or your own backend.',
			'settings.compliance_title' => 'LGPD & Audit',
			'settings.logout_title' => 'Log out',
			'settings.logout_subtitle' => 'End this session',
			'settings.logout_confirm_title' => 'Log out of account',
			'settings.logout_confirm_body' => 'Are you sure? You\'ll need to sign in again to access the dashboard.',
			'settings.trial_expired_title' => 'Trial expired',
			'settings.plan_inactive_title' => 'Plan inactive',
			'settings.plan_view_plans' => 'View plans',
			'settings.plan_label' => ({required Object name}) => 'Plan: ${name}',
			'settings.tokens_conversations_usage' => ({required Object tokens, required Object conv}) => 'Tokens: ${tokens}% | Conversations: ${conv}%',
			'settings.agent_name_placeholder' => 'e.g. Sales Assistant',
			'settings.ai_provider_label' => 'Provider',
			'settings.agent_active_title' => 'Agent Active',
			'settings.agent_active_subtitle' => 'The agent is replying to messages',
			'settings.agent_paused_subtitle' => 'The agent is paused',
			'settings.agent_test_title' => 'Test Agent',
			'settings.agent_test_subtitle' => 'Start a test conversation with this agent',
			'settings.agent_calendars_title' => 'Calendars',
			'settings.agent_calendars_subtitle' => 'Google Calendar for bookings',
			'settings.agent_kb_title' => 'Knowledge Base',
			'settings.agent_kb_subtitle' => 'Texts and PDFs for the AI',
			'settings.agent_section_name' => 'AGENT NAME',
			'settings.agent_section_ai' => 'ARTIFICIAL INTELLIGENCE',
			'settings.agent_section_prompt' => 'SYSTEM PROMPT',
			'settings.agent_section_personality' => 'LEARNED PERSONALITY',
			'settings.agent_section_business_hours' => 'BUSINESS HOURS',
			'settings.agent_section_behavior' => 'BEHAVIOR',
			'settings.agent_section_info' => 'ADDITIONAL INFO',
			'settings.agent_prompt_helper_text' => 'Define the personality and instructions of the AI agent.',
			'settings.agent_prompt_library_match' => ({required Object name}) => 'Using the saved prompt: ${name}',
			'settings.agent_prompt_library_open' => 'Open library',
			'settings.agent_prompt_hint' => 'You are a customer service assistant for company X. Your goal is to help customers with questions about products and orders...',
			'settings.agent_save' => 'Save',
			'settings.agent_saved' => 'Saved',
			'settings.upgrade_plan_action' => 'Upgrade plan',
			'settings.agent_not_found_message' => 'Agent not found',
			'settings.agent_updated_success' => 'Agent updated successfully!',
			'settings.personality_empty_hint' => 'No personality learned yet. Reply to a few conversations manually and the bot will start mimicking your team\'s style.',
			'settings.personality_active_hint' => 'Personality learned from the most recent human replies. Applied automatically to the bot.',
			'settings.personality_updated' => 'Personality updated.',
			'settings.bh_limit_label' => 'Limit auto-replies to a schedule',
			'settings.bh_explainer' => 'The bot only replies inside this schedule. Outside the window the conversation goes to a human.',
			'settings.bh_start' => 'Start',
			'settings.bh_end' => 'End',
			'settings.bh_time_placeholder' => '--:--',
			'settings.bh_dom_d' => 'S',
			'settings.bh_d_sun' => 'S',
			'settings.bh_d_mon' => 'M',
			'settings.bh_d_tue' => 'T',
			'settings.bh_d_wed' => 'W',
			'settings.bh_d_thu' => 'T',
			'settings.bh_d_fri' => 'F',
			'settings.bh_d_sat' => 'S',
			'settings.behavior_hide_ai_title' => 'Human style (doesn\'t announce it\'s AI)',
			'settings.behavior_hide_ai_subtitle' => 'Bot writes like a human attendant and never volunteers that it\'s an AI. If the customer asks directly, it answers honestly — required by WhatsApp (Meta) policy.',
			'settings.behavior_silent_title' => 'Wait for human when unsure',
			'settings.behavior_silent_subtitle' => 'When the bot doesn\'t know the answer, it stays silent and hands the conversation to a human without telling the customer. Turn off so the bot says "I\'ll check" in the customer\'s language.',
			'settings.agent_id_label' => 'ID',
			'settings.agent_timezone_label' => 'Timezone',
			'settings.agent_created_label' => 'Created on',
			'settings.channels_back_tooltip' => 'Back',
			'settings.channels_screen_title' => 'Channels',
			'settings.channels_add_tooltip' => 'Add channel',
			'settings.channels_empty_subtitle' => 'Connect the email you already use so offers go out, and WhatsApp for the relationship.',
			'settings.channel_dialog_edit_title' => 'Edit channel',
			'settings.channel_dialog_add_title' => 'Add channel',
			'settings.channel_dialog_edit_subtitle' => 'Update name, credentials or assigned agent.',
			'settings.channel_dialog_add_subtitle' => 'Where your offers go out and the replies come in.',
			'settings.channel_name_label' => 'Channel name',
			'settings.channel_name_hint' => 'e.g. Sales email',
			'settings.channel_or_manual' => 'or connect manually',
			'settings.channel_responsible_agent' => 'Responsible agent',
			'settings.channel_no_agent_human' => 'None (human only)',
			'settings.channel_save_btn' => 'Save',
			'settings.channel_connect_btn' => 'Connect',
			'settings.channel_partial_credentials' => 'Fill in every field to rotate credentials, or leave them all blank to keep the current ones.',
			'settings.channel_invalid_phone_id' => 'Invalid Phone Number ID (must be 12-16 numeric digits).',
			'settings.channel_invalid_waba_id' => 'Invalid WABA ID (must be 12-18 numeric digits).',
			'settings.channel_invalid_token' => 'Access Token too short. Paste the full Meta token.',
			'settings.channel_invalid_account_sid' => 'Invalid Account SID (must start with AC followed by 32 hex characters).',
			'settings.channel_invalid_from_number' => 'Invalid phone number. Use E.164 format, e.g. +15551234567.',
			'settings.channel_updated_success' => 'Channel updated successfully!',
			'settings.channel_created_success' => 'Channel created successfully!',
			'settings.channel_deleted_success' => 'Channel deleted',
			'settings.channel_delete_confirm' => ({required Object name}) => 'Delete "${name}"? Messages received on this channel will stop.',
			'settings.channel_status_connected' => 'Connected',
			'settings.channel_status_disabled' => 'Disabled',
			'settings.channel_status_incomplete' => 'Incomplete',
			'settings.channel_edit_tooltip' => 'Edit',
			'settings.channel_test_btn' => 'Test Connection',
			'settings.channel_testing' => 'Testing...',
			'settings.channel_configure_btn' => 'Configure',
			'settings.channel_snippet_btn' => 'Snippet',
			'settings.channel_test_success_named' => ({required Object label}) => 'Connected! ${label}',
			'settings.channel_test_success' => 'Connection OK!',
			'settings.channel_test_failed' => ({required Object error}) => 'Failed: ${error}',
			'settings.channel_test_unknown_error' => 'Unknown error',
			'settings.channel_reconnect_action' => 'Reconnect',
			'settings.channel_google_connected' => ({required Object address}) => 'Connected through Google: ${address}',
			'settings.channel_google_edit_hint' => 'This mailbox was connected by consent — there is no server or password to edit. If sending or reading stops, connect it again.',
			'settings.channel_google_reconnect' => 'Reconnect with Google',
			'settings.channel_imap_failing' => ({required Object count, required Object error}) => 'Reading replies has failed ${count} time(s): ${error}',
			'settings.channel_webhook_re_registered' => 'Webhook re-registered.',
			'settings.channel_webhook_register_failed' => ({required Object error}) => 'Failed to re-register webhook: ${error}',
			'settings.channels_select_telegram_badge' => '2 min',
			'settings.channels_select_popular_badge' => 'Popular',
			'settings.channels_select_coming_soon' => 'Coming soon',
			'settings.channel_instagram_help_link' => 'How to get your Page ID and access token',
			'settings.channel_sms_help_link' => 'How to get a compliant texting number (Twilio guide)',
			'settings.channel_webhook_url_label' => 'Webhook URL (Twilio)',
			'settings.channel_webhook_url_help' => 'Paste this URL as the "A message comes in" webhook for your number in the Twilio console.',
			'settings.channel_webhook_url_copied' => 'Webhook URL copied',
			'settings.channel_webhook_url_btn' => 'Webhook URL',
			'settings.wa_settings_back_tooltip' => 'Back',
			'settings.wa_settings_title' => 'WhatsApp Settings',
			'settings.wa_settings_tab_profile' => 'Profile',
			'settings.wa_settings_tab_templates' => 'Templates',
			'settings.wa_section_diagnostics' => 'DIAGNOSTICS',
			'settings.wa_section_phones' => 'NUMBERS IN WABA',
			'settings.wa_section_business' => 'ABOUT THE BUSINESS',
			'settings.wa_field_about' => 'About',
			'settings.wa_field_about_hint' => 'Short description (shown on your profile)',
			'settings.wa_field_description' => 'Full description',
			'settings.wa_field_description_hint' => 'Detailed description of your business',
			'settings.wa_field_email' => 'Email',
			'settings.wa_field_email_hint' => 'contact@company.com',
			'settings.wa_field_address' => 'Address',
			'settings.wa_field_address_hint' => '123 Main St — City/State',
			'settings.wa_field_site_one' => 'Site 1',
			'settings.wa_field_site_one_hint' => 'https://...',
			'settings.wa_field_site_two' => 'Site 2',
			'settings.wa_field_site_two_hint' => 'https://... (optional)',
			'settings.wa_profile_helper' => 'These details show publicly on your WhatsApp Business profile.',
			'settings.wa_profile_save_btn' => 'Save changes',
			'settings.wa_profile_saving' => 'Saving...',
			'settings.wa_profile_saved_success' => 'Profile updated successfully!',
			'settings.wa_picture_replace' => 'Replace photo',
			'settings.wa_picture_set' => 'Change photo',
			'settings.wa_picture_apply_hint' => 'The photo will be applied when you click "Save changes".',
			'settings.wa_picture_remove_button' => 'Remove cached photo',
			'settings.wa_picture_remove_dialog_title' => 'Remove cached photo?',
			'settings.wa_picture_remove_dialog_body' => 'Drops the local copy of the profile picture. The photo on the WhatsApp number itself stays — to remove it there, use Meta Business Manager. Use this once Meta is no longer returning the picture so the dashboard stops serving the old one.',
			'settings.wa_picture_remove_dialog_confirm' => 'Remove',
			'settings.wa_picture_removed_success' => 'Cached photo removed.',
			'settings.wa_templates_no_templates_subtitle' => 'Templates are pre-approved messages from Meta. You need them to start conversations with customers outside the 24h window.',
			'settings.wa_templates_create_loading_error_title' => 'Connection with Meta needs to be renewed',
			'settings.wa_templates_visualize_tooltip' => 'Preview',
			'settings.wa_templates_delete_tooltip' => 'Delete',
			'settings.wa_templates_delete_confirm' => ({required Object name}) => 'Delete "${name}"? This also removes the template on Meta.',
			'settings.wa_templates_delete_success' => 'Template deleted',
			'settings.wa_templates_rejected_reason' => ({required Object reason}) => 'Reason: ${reason}',
			'settings.wa_templates_no_examples_warning' => 'No examples saved — variables show as placeholders.',
			'settings.wa_create_dialog_title' => 'New template',
			'settings.wa_create_section_name' => '1. TECHNICAL NAME',
			'settings.wa_create_name_hint' => 'e.g. welcome_message',
			'settings.wa_create_name_helper' => 'Lowercase letters, numbers and underscore only. Customer never sees it.',
			'settings.wa_create_section_category' => '2. CATEGORY',
			'settings.wa_create_section_language' => '3. LANGUAGE',
			'settings.wa_create_section_header' => 'HEADER (optional)',
			'settings.wa_create_section_message' => '4. MESSAGE',
			'settings.wa_create_message_hint' => 'Hello {{1}}, your order #{{2}} has been confirmed! 🎉',
			'settings.wa_create_message_helper' => 'Use {{1}}, {{2}}... for variables. Formatting: *bold*, _italic_, ~strike~, `mono` (WhatsApp uses 1 asterisk, not 2).',
			'settings.wa_create_double_asterisk_warn' => 'WhatsApp uses *text* (one asterisk), not **text**. Fix before sending.',
			'settings.wa_create_section_examples' => '5. EXAMPLES FOR VARIABLES',
			'settings.wa_create_example_for' => ({required Object n}) => 'Example for {{${n}}}',
			'settings.wa_create_example_first' => 'John Smith',
			'settings.wa_create_example_other' => 'Value',
			'settings.wa_create_examples_helper' => 'Meta uses these examples to review the template.',
			'settings.wa_create_section_footer' => 'FOOTER (optional)',
			'settings.wa_create_footer_hint' => 'e.g. Reply STOP to opt out',
			'settings.wa_create_section_buttons' => 'BUTTONS (optional, max 3)',
			'settings.wa_create_section_preview' => 'PREVIEW',
			'settings.wa_create_submit' => 'Submit for review',
			'settings.wa_create_submitting' => 'Submitting...',
			'settings.wa_create_validation_name' => 'Name must contain only lowercase letters, numbers and _.',
			'settings.wa_create_validation_body_required' => 'Message is required.',
			'settings.wa_create_validation_double_asterisk' => 'Remove the ** from the text — WhatsApp uses *text* (one asterisk) for bold.',
			'settings.wa_create_validation_no_leading_var' => 'Text cannot start with a variable. Add a word before {{1}} (e.g. "Hello {{1}}").',
			'settings.wa_create_validation_no_trailing_var' => 'Text cannot end with a variable. Add a word or punctuation after the last {{N}}.',
			'settings.wa_create_validation_examples_required' => 'Fill in the examples for the variables.',
			'settings.wa_create_validation_header_empty' => 'Text header cannot be empty.',
			'settings.wa_create_validation_header_too_long' => 'Text header exceeds 60 characters.',
			'settings.wa_create_validation_header_one_var' => 'Header accepts at most one variable.',
			'settings.wa_create_validation_header_example' => 'Fill in the header example.',
			'settings.wa_create_validation_media_uploading' => 'Wait for the header upload to finish.',
			'settings.wa_create_validation_media_required' => 'Pick and upload a file for the header.',
			'settings.wa_create_validation_button_text_required' => 'Every button needs text.',
			'settings.wa_create_validation_button_url' => 'URL buttons need a link.',
			'settings.wa_create_validation_button_phone' => 'Phone buttons need an E.164 number (e.g. +14155551212).',
			'settings.wa_create_warning_promo_for_utility' => 'Text looks promotional ("news", "offer"...). Meta usually rejects UTILITY with this content — switch to MARKETING.',
			'settings.wa_create_warning_tx_for_marketing' => 'Text looks transactional ("confirmation", "order"...). Consider UTILITY to reduce the chance of rejection.',
			'settings.wa_category_marketing' => 'Promotions and offers',
			'settings.wa_category_utility' => 'Confirmations and updates',
			'settings.wa_lang_pt_br' => 'Portuguese (Brazil)',
			'settings.wa_lang_en_us' => 'English (US)',
			'settings.wa_lang_es' => 'Spanish',
			'settings.wa_header_none' => 'No header',
			'settings.wa_header_text' => 'Text',
			'settings.wa_header_image' => 'Image',
			'settings.wa_header_video' => 'Video',
			'settings.wa_header_document' => 'Document',
			'settings.wa_header_text_hint' => 'e.g. Order confirmation {{1}}',
			'settings.wa_header_text_helper' => 'Max 60 characters. Allows up to one variable {{1}}.',
			'settings.wa_header_example_label' => 'Example for {{1}} in the header',
			'settings.wa_header_example_hint' => '#12345',
			'settings.wa_header_pick_file' => 'Pick file',
			'settings.wa_header_uploading' => 'Uploading...',
			'settings.wa_header_remove_tooltip' => 'Remove',
			'settings.wa_header_upload_failed_processing' => 'Failed to process the file on Meta. Try another.',
			'settings.wa_header_upload_failed_generic' => ({required Object error}) => 'Upload failed: ${error}',
			'settings.wa_button_text_label' => 'Button text (max 25)',
			'settings.wa_button_url_label' => 'URL',
			'settings.wa_button_url_hint' => 'https://...',
			'settings.wa_button_phone_label' => 'Number (E.164)',
			'settings.wa_button_phone_hint' => '+14155551212',
			'settings.wa_button_call' => 'Call number',
			'settings.wa_preview_message_placeholder' => 'Your message appears here...',
			'settings.wa_preview_button_placeholder' => 'Button text',
			'settings.wa_template_submit_success' => 'Template submitted! Meta review usually takes up to 24 hours.',
			'settings.wa_test_template_explain' => 'Sends a real message to validate the connection end-to-end. Useful for the Meta App Review screencast.',
			'settings.wa_diag_phone_not_registered_no_platform' => 'Meta has not confirmed the number registration. Inbound messages won\'t reach Replider.',
			'settings.wa_diag_phone_not_registered_with_platform' => ({required Object platform}) => 'platform_type = ${platform}. Register the number to receive messages.',
			'settings.wa_diag_cooldown_title' => 'Locked',
			'settings.wa_diag_cooldown_snackbar' => ({required Object time}) => 'Wait ${time} before the next attempt. Each click resets the Meta lock.',
			'settings.wa_diag_atualizar_tooltip' => 'Refresh',
			'settings.csat_auto_title' => 'Automatic CSAT',
			'settings.csat_auto_subtitle' => 'Ask 1 to 5 after the conversation goes idle',
			'settings.team_metrics_relative_min' => ({required Object count}) => '${count} min',
			'settings.team_metrics_relative_hours' => ({required Object count}) => '${count} h',
			'settings.team_metrics_relative_days' => ({required Object count}) => '${count} d',
			'settings.team_metrics_relative_weeks' => ({required Object count}) => '${count} w',
			'settings.csat_auto_helper' => 'Sent in the conversation\'s detected language (pt/en/es). Turn off in B2B flows where the agent already debriefs manually.',
			'settings.reply_reminder_title' => 'Reply reminder',
			'settings.reply_reminder_subtitle' => 'Never forget a waiting customer',
			'settings.reply_reminder_helper' => 'When the AI is paused and a customer is waiting longer than the threshold, every team member with a registered device gets a push + dashboard alert. Escalates at 1x, 2x, and 4x the threshold, then stops.',
			'settings.reply_reminder_threshold_label' => 'Notify after',
			'settings.claude_title' => 'Claude (AI)',
			'settings.claude_subtitle' => 'Control your account by chatting with Claude (MCP).',
			'settings.brand_tile' => 'Company identity',
			'settings.brand_tile_sub' => 'Voice, what may and may never be said',
			'settings.templates_tile' => 'Email templates',
			'settings.templates_tile_sub' => 'Ready-made email texts, with how well each one replies',
			'settings.suppressions_tile' => 'Blocked addresses',
			'settings.suppressions_tile_sub' => 'Who stopped receiving your email, and why',
			'settings.autopilot_tile_sub' => 'When approaches may go out without your approval',
			'settings.tokens_conversations_used' => ({required Object tokens, required Object conv}) => 'Tokens: ${tokens} | Conversations: ${conv}',
			'settings.contacts_subtitle' => 'Everyone who has talked to you, in one place',
			'settings.analytics_subtitle' => 'Volume, response time and AI performance',
			'settings.preview_handoff_note' => 'The agent would hand this conversation to a person. In a real conversation the bot is paused and someone takes over — in the test there is nobody to hand off to.',
			'settings.channel_test_email_success' => ({required Object to, required Object cap}) => 'Test sent to ${to}. Check that inbox — no customer was contacted. You can now approve up to ${cap} approaches a day.',
			'settings.channel_test_email_not_unlocked' => ({required Object to}) => 'Test sent to ${to}, but we could not record the release. Run the test again.',
			'settings.section_company' => 'Company and brand',
			'settings.section_email' => 'Email and sending',
			'settings.section_catalog' => 'Catalogue and funnels',
			'settings.section_documents' => 'Documents',
			'settings.section_integrations' => 'Integrations',
			'settings.section_contract' => 'Subscribe or grow',
			'settings.section_support_bot' => 'Automated support',
			'settings.funnels_tile_sub' => 'Stages, the data to collect and the webhook',
			'settings.section_advanced' => 'Advanced',
			'settings.section_advanced_sub' => 'Funnels and relationship dates. Already set up for you; open only if you want to change them.',
			'settings.relationship_tile_sub' => 'Birthdays, anniversaries and sector days that turn into a draft on the day',
			'settings.support_bot_tile_sub' => 'Agents, prompts, flows, tags and the bot\'s own metrics',
			'settings.support_bot_intro' => 'The chatbot side of Replider. If you are here to sell, none of this is needed — the selling tools are on the previous screen.',
			'settings.email_warmup_not_started' => 'Warm-up not started — verify the mailbox (DNS or reconnect) to enable cold outreach',
			'settings.email_warmup_progress' => ({required Object day, required Object total, required Object sent, required Object cap}) => 'Warm-up: day ${day} of ${total} · ${sent} of ${cap} sends today',
			'settings.email_warmup_done' => ({required Object cap}) => 'Warmed up — up to ${cap} sends per day',
			'settings.channel_name_required' => 'Give the channel a name',
			'settings.channel_required_missing' => ({required Object fields}) => 'Fill in: ${fields}',
			'settings.channel_email_help_link' => 'How to set up your domain\'s e-mail',
			'settings.usage_costs_title' => 'Usage & AI costs',
			'settings.usage_costs_subtitle' => 'Monthly usage, AI spend and your budget ceiling',
			'common.cancel' => 'Cancel',
			'common.save' => 'Save',
			'common.create' => 'Create',
			'common.delete' => 'Delete',
			'common.edit' => 'Edit',
			'common.retry' => 'Try again',
			'common.subscribe' => 'Subscribe',
			'common.per_month' => '/mo',
			'common.free' => 'Free',
			'common.success' => 'Success',
			'common.confirm_delete' => 'Are you sure you want to delete this?',
			'common.no_results' => 'No results found.',
			'common.confirm' => 'Confirm',
			'common.remove' => 'Remove',
			'common.close' => 'Close',
			'common.add' => 'Add',
			'common.search' => 'Search',
			'common.copy' => 'Copy',
			'common.change' => 'Change',
			'common.load_more' => 'Load more',
			'common.error_generic' => ({required Object error}) => 'Error: ${error}',
			'common.back' => 'Back',
			'common.saving' => 'Saving...',
			'common.sending' => 'Sending...',
			'common.sent' => 'Sent',
			'common.loading' => 'Loading...',
			'common.ok' => 'OK',
			'common.open' => 'Open',
			'common.send' => 'Send',
			'common.discard' => 'Discard',
			'common.keep_editing' => 'Keep editing',
			'common.discard_changes_title' => 'Discard changes?',
			'common.discard_changes_body' => 'You have unsaved changes. Leaving now will discard them.',
			'common.yes' => 'Yes',
			'common.no' => 'No',
			'common.logout' => 'Log out',
			'common.refresh' => 'Refresh',
			'common.select' => 'Select',
			'common.name_label' => 'Name',
			'common.color' => 'Color',
			'common.new_short' => 'New',
			'common.unreadable_content' => 'Unreadable content',
			'common.load_failed_tap' => 'Couldn\'t load. Tap to try again.',
			'billing.title' => 'Usage & AI costs',
			'billing.checkout_error' => 'Failed to start checkout.',
			'billing.manage_payment_title' => 'Manage payment',
			'billing.manage_payment_subtitle' => 'Open the Stripe portal to update your card, download invoices, or cancel.',
			'billing.manage_payment_cta' => 'Open portal',
			'billing.portal_opening' => 'Opening portal...',
			'billing.upgrade_modal_title' => 'Pick your plan',
			'billing.upgrade_modal_subtitle' => 'Subscribe directly from here. Secure payment via Stripe.',
			'billing.upgrade_current_plan_badge' => 'Current plan',
			'billing.upgrade_popular_badge' => 'Most popular',
			'billing.upgrade_cta' => 'Subscribe',
			'billing.upgrade_cta_contact_sales' => 'Contact sales',
			'billing.contract_badge' => 'Sold under contract',
			'billing.contract_subtitle' => 'Custom limits + bring your own LLM API key. Talk to a specialist for pricing and setup.',
			'billing.contract_email_cta' => 'Email',
			'billing.contract_whatsapp_cta' => 'WhatsApp',
			'billing.contract_email_subject' => ({required Object plan}) => 'Replider ${plan} plan — interest',
			'billing.contract_link_failed' => 'Couldn\'t open the link. Email us at support@replider.com.',
			'billing.upgrade_per_month' => '/mo',
			'billing.upgrade_interval_monthly' => 'Monthly',
			'billing.upgrade_interval_annual' => 'Annual',
			'billing.upgrade_annual_save' => 'Save 20%',
			'billing.upgrade_billed_annually' => 'billed annually',
			'billing.upgrade_compare_current' => 'Current',
			'billing.upgrade_compare_new' => 'New',
			'billing.upgrade_compare_conversations' => 'Conversations/mo',
			'billing.upgrade_compare_agents' => 'AI agents',
			'billing.upgrade_compare_channels' => 'Channels',
			'billing.upgrade_compare_tokens' => 'AI tokens/mo',
			'billing.upgrade_checkout_opening' => 'Opening checkout...',
			'billing.upgrade_feature_agents' => ({required Object count}) => '${count} AI agent(s)',
			'billing.upgrade_feature_channels' => ({required Object count}) => '${count} channel(s)',
			'billing.upgrade_feature_conversations' => ({required Object count}) => '${count} conversations/mo',
			'billing.upgrade_feature_tokens' => ({required Object tokens}) => '${tokens} AI tokens/mo',
			'billing.billing_pill_trial_days' => ({required Object days}) => 'Trial: ${days} days',
			'billing.billing_pill_trial_last_day' => 'Trial ends today',
			'billing.billing_pill_trial_expired' => 'Trial expired',
			'billing.billing_pill_past_due' => 'Payment pending',
			'billing.billing_pill_manage' => 'Manage',
			'billing.no_customer_yet' => 'Complete your first subscription before opening the billing portal.',
			'billing.checkout_open_failed' => 'Failed to start checkout.',
			'billing.addon_buy_failed' => 'Failed to purchase the pack.',
			'billing.portal_open_failed' => 'Failed to open the billing portal.',
			'billing.quota_conversations_label' => 'Conversations',
			'billing.quota_tokens_label' => 'AI credits',
			'billing.quota_agents_label' => 'AI agents',
			'billing.quota_wall_title' => 'AI limit reached',
			'billing.quota_wall_body' => 'You\'ve used all your AI credits for this cycle. The agent stopped replying automatically until you add more credits or the cycle resets.',
			'billing.quota_wall_cta_later' => 'Wait for monthly reset',
			'billing.quota_wall_reset_hint' => ({required Object day}) => 'Cycle resets on day ${day} of next month.',
			'billing.back_tooltip' => 'Back',
			'billing.section_usage' => 'CURRENT USAGE',
			'billing.section_addon_packs' => 'ADDON PACKS',
			'billing.section_plans' => 'AVAILABLE PLANS',
			'billing.addon_packs_unavailable' => 'Addon packs unavailable.',
			'billing.load_usage_failed' => 'Failed to load usage',
			'billing.load_plans_failed' => 'Failed to load plans',
			'billing.trial_view_plans' => 'View plans',
			'billing.usage_ai_tokens' => 'AI tokens',
			'billing.usage_includes_addon_tokens' => ({required Object count}) => 'Includes ${count} extra tokens',
			'billing.usage_input' => 'Input',
			'billing.usage_output' => 'Output',
			'billing.usage_conversations' => 'Conversations',
			'billing.overage_active_title' => 'Paying for extra conversations',
			'billing.overage_active_body' => ({required Object count, required Object amount}) => '${count} conversations beyond your plan · ${amount} billed this cycle',
			'billing.overage_rate_note' => ({required Object rate}) => '${rate} per extra conversation',
			'billing.overage_cap_reached' => 'Overage cap reached — extra conversations are paused until your cycle resets.',
			'billing.plan_free' => 'Free',
			'billing.plan_per_month_short' => '/mo',
			'billing.plan_feature_agents' => ({required Object count}) => '${count} AI agent(s)',
			'billing.plan_feature_channels' => ({required Object count}) => '${count} support channel(s)',
			'billing.plan_feature_conversations' => ({required Object count}) => '${count} conversations/mo',
			'billing.plan_feature_tokens' => ({required Object tokens}) => '${tokens} AI tokens/mo',
			'billing.plan_feature_storage' => ({required Object mb}) => '${mb} MB storage',
			'billing.plan_feature_storage_gb' => ({required Object gb}) => '${gb} GB of storage',
			'billing.plan_feature_seats' => ({required Object count}) => '${count} attendant seat(s)',
			'billing.plan_feature_broadcast' => ({required Object count}) => '${count} broadcast messages/mo',
			'billing.plan_feature_recipients' => ({required Object count}) => '${count} recipients per campaign',
			'billing.plan_feature_overage' => ({required Object cents, required Object cap}) => 'Extra conversations: ${cents}¢ each (capped at ${cap}× plan price)',
			'billing.effective_limits_title' => ({required Object plan}) => 'Your ${plan} contract limits',
			'billing.effective_limits_subtitle' => 'Resolved from your contract overrides — billed off-platform.',
			'billing.byok_label' => 'Bring your own key',
			'billing.byok_configured' => ({required Object providers}) => 'Configured: ${providers}',
			'billing.byok_pending' => 'Not configured — the agent won\'t reply until you set a key.',
			'billing.byok_manage_cta' => 'Manage',
			'billing.cancel_subscription' => 'Cancel subscription',
			'billing.plan_contract' => 'Custom',
			'billing.plan_feature_leads' => ({required Object count}) => '${count} leads/month',
			'billing.plan_feature_enrichments' => ({required Object count}) => '${count} company lookups/month',
			'billing.plan_feature_drafts' => ({required Object count}) => '${count} AI drafts/month',
			'billing.plan_feature_funnels' => ({required Object count}) => '${count} funnel(s)',
			'billing.plan_feature_mailboxes' => ({required Object count}) => '${count} mailbox(es)',
			'billing.usage_sdr_title' => 'YOUR MONTH',
			'billing.usage_sdr_leads' => 'Leads in the pipeline',
			'billing.usage_sdr_enrichments' => 'Company lookups',
			'billing.usage_sdr_approaches' => 'Approaches sent',
			'billing.usage_sdr_meetings' => 'Meetings booked',
			'billing.usage_sdr_of_limit' => ({required Object used, required Object limit}) => '${used} of ${limit}',
			'billing.usage_sdr_no_limit' => ({required Object used}) => '${used} this month',
			'billing.talk_title' => 'Want to subscribe or grow?',
			'billing.talk_body' => 'We build the proposal in the conversation, sized to your operation. No price list: every company sells differently.',
			'billing.talk_book_cta' => 'Book a call',
			'billing.talk_email_subject' => 'I want to know Replider',
			'billing.ai_cost_title' => 'AI cost (your own key)',
			'billing.ai_cost_cycle' => 'This cycle',
			'billing.ai_cost_projection' => 'Monthly pace',
			'billing.ai_cost_budget' => 'Your ceiling',
			'billing.ai_cost_no_budget' => 'none set',
			'billing.ai_cost_set_budget' => 'Set monthly ceiling',
			'billing.ai_cost_budget_dialog_title' => 'Monthly AI ceiling',
			'billing.ai_cost_budget_help' => 'Hard stop: once spend reaches this, AI generation pauses until the cycle resets. 0 removes the ceiling.',
			'team.title' => 'Team',
			'team.invite_member' => 'Invite Member',
			'team.member_invited' => 'Member invited successfully.',
			'team.member_removed' => 'Member removed.',
			'team.role_updated' => 'Role updated.',
			'team.change_role' => 'Change role',
			'team.remove_member' => 'Remove member',
			'team.inactive' => 'Inactive',
			'team.pending_badge' => 'Pending',
			'team.resend_invite' => 'Resend invite',
			'team.invite_resent' => 'Invite resent.',
			'team.active_now' => 'Active now',
			'team.last_seen_ago' => ({required Object when}) => 'Last seen ${when} ago',
			'team.invite_short' => 'Invite',
			'team.empty_subtitle' => 'No team members yet',
			'team.role_label' => 'Role',
			'team.role_admin' => 'Admin',
			'team.role_attendant' => 'Attendant',
			'team.change_role_title' => 'Change Role',
			'team.change_role_body' => ({required Object name, required Object role}) => 'Change "${name}"\'s role to ${role}?',
			'team.remove_title' => 'Remove Member',
			'team.remove_body' => ({required Object name}) => 'Really remove "${name}" from the team? This person will lose access to the system.',
			'team.ago_minutes' => ({required Object count}) => '${count} min',
			'team.ago_hours' => ({required Object count}) => '${count}h',
			'team.ago_days' => ({required Object count}) => '${count} days',
			'team.ago_months' => ({required Object count}) => '${count} months',
			'team.role_owner' => 'Owner',
			'team.role_gestor' => 'Manager',
			'team.role_vendedor' => 'Sales rep',
			'team.role_vendedor_hint' => 'Works their own leads and offers; no general inbox',
			'team.role_gestor_hint' => 'Oversees the team and the pipeline; no billing or integrations',
			'team.role_attendant_hint' => 'Answers inbox conversations; no pipeline access',
			'team.role_admin_hint' => 'Everything: settings, team, channels and billing',
			'team.invite_fields_required' => 'Fill in name and e-mail',
			'team.invite_email_invalid' => 'Invalid e-mail — check the address',
			'channels.title' => 'Channels',
			'channels.connect_description' => 'Connect WhatsApp, Instagram, or Telegram to receive customer messages.',
			'channels.channel_created' => 'Channel created successfully.',
			'channels.channel_deleted' => 'Channel deleted.',
			'channels.webhook_down' => 'Webhook down',
			'channels.test_connection' => 'Test connection',
			'channels.delete_channel' => 'Delete channel',
			'channels.type_webchat_label' => 'Site (Webchat)',
			'channels.setup_whatsapp' => 'Enter the Phone Number ID of your number on the Meta for Developers panel. The access token is managed automatically by Replider.',
			'channels.setup_telegram' => 'Create a bot with @BotFather on Telegram and paste the token here. The webhook is configured automatically.',
			'channels.setup_instagram' => 'Connect your Instagram professional account (Business or Creator). Enter the account ID and an access token with the instagram_business_manage_messages permission. Direct messages only.',
			'channels.setup_webchat' => 'Live chat for your website. After creating, copy the snippet and paste it in your site\'s HTML.',
			'channels.setup_sms' => 'Connect your own Twilio number for two-way SMS conversations. Paste the Account SID, Auth Token, and phone number (E.164) from the Twilio console. After connecting, copy the channel\'s webhook URL and set it as the "A message comes in" webhook for your number in Twilio.',
			'channels.embedded_signup_missing_config' => 'Build configuration incomplete. Set WHATSAPP_APP_ID and WHATSAPP_EMBEDDED_CONFIG_ID via --dart-define.',
			'channels.embedded_signup_unknown_failure' => 'Unknown signup failure.',
			'channels.embedded_signup_complete_failed' => ({required Object error}) => 'Failed to finish connection: ${error}',
			'channels.embedded_signup_success' => 'WhatsApp connected successfully!',
			'channels.embedded_signup_connecting' => 'Connecting...',
			'channels.embedded_signup_cta' => 'Connect WhatsApp via Meta',
			'channels.embedded_signup_missing_env' => 'WHATSAPP_APP_ID or WHATSAPP_EMBEDDED_CONFIG_ID not set in build.',
			'channels.embedded_signup_no_response' => 'No response from the Facebook SDK.',
			'channels.embedded_signup_user_cancelled' => 'Login cancelled by the user.',
			'channels.embedded_signup_no_code' => ({required Object status}) => 'Facebook didn\'t return an authorisation code (status=${status}).',
			'channels.embedded_signup_failure' => ({required Object error}) => 'Embedded Signup failed: ${error}',
			'channels.setup_email' => 'Send and receive email from your own domain. You need your provider’s SMTP details and SPF, DKIM and DMARC published in DNS — without them Gmail and Outlook drop the message silently.',
			'channels.email_dns_verified' => 'Domain authenticated — ready to send.',
			'channels.email_dns_pending' => 'The domain still needs authentication. Without it, your customers\' providers (Gmail, Outlook and others) discard your messages.',
			'channels.email_dns_recheck' => 'Check again',
			'channels.email_dns_optional' => 'optional',
			'channels.email_dns_host' => 'Name',
			'channels.email_dns_value' => 'Value',
			'channels.email_dns_found' => 'Currently published',
			_ => null,
		} ?? switch (path) {
			'channels.email_dns_copied' => 'Copied.',
			'channels.email_dns_provider_managed' => 'Google authenticates this address\'s sending — nothing to publish.',
			'channels.email_dns_already_published' => 'Already published — nothing to do here.',
			'channels.email_dns_detected' => ({required Object provider}) => 'DNS at ${provider}',
			'channels.email_dns_open_guide' => 'Open guide',
			'channels.email_dns_auto_configure' => 'Set up automatically',
			'channels.email_dns_token_help' => ({required Object provider}) => 'In ${provider}, create an API token with the Zone → DNS → Edit permission for this domain, and paste it here. We use it once to publish the records and never store it.',
			'channels.email_dns_token_hint' => 'Paste the token',
			'channels.email_dns_auto_done' => 'Records published',
			'channels.email_dns_auto_partial' => 'Partly published',
			'channels.dns_action_created' => 'created',
			'channels.dns_action_updated' => 'updated',
			'channels.dns_action_merged' => 'merged into your existing record',
			'channels.dns_action_kept' => 'yours kept, untouched',
			'channels.dns_action_failed' => 'failed',
			'channels.email_dns_need_help' => 'Need help?',
			'channels.email_dns_provider_unknown' => 'not detected',
			'channels.email_dns_help_draft' => ({required Object domain, required Object provider, required Object records}) => 'I need help setting up my email domain.\n\nDomain: ${domain}\nDNS provider: ${provider}\nMissing records: ${records}',
			'channels.webhook_health_unknown' => 'Health not verified',
			'channels.gmail_connect_title' => 'Connect the account you already use',
			'channels.gmail_connect_help' => 'One click, no server or password to type. Works with Gmail and Google Workspace, and replies keep landing in the inbox you already read.',
			'channels.gmail_connect_cta' => 'Connect with Google',
			'channels.gmail_connected' => 'Email connected.',
			'channels.gmail_reconnected' => 'Email connection renewed.',
			'channels.gmail_denied' => 'Connection cancelled. Nothing changed.',
			'channels.gmail_failed' => 'Could not finish the connection. Try again.',
			'channels.gmail_or_smtp' => 'or set up SMTP manually',
			'channels.gmail_already_connected' => ({required Object name}) => 'Mailbox connected: ${name}',
			'channels.gmail_connect_check' => 'I authorized it — check',
			'channels.email_dns_token_title_named' => ({required Object provider}) => '${provider} API token',
			'channels.email_field_from' => 'Sending e-mail',
			'channels.email_field_smtp_host' => 'SMTP server',
			'channels.email_field_smtp_port' => 'Port',
			'channels.email_field_smtp_user' => 'SMTP username',
			'channels.email_field_smtp_password' => 'SMTP password',
			'channels.email_field_imap_host' => 'IMAP server (optional)',
			'channels.email_field_imap_hint' => 'same as SMTP if empty',
			'channels.email_field_imap_port' => 'IMAP port (optional)',
			'channels.email_dns_generic_steps' => 'We could not auto-detect your DNS provider. No problem: open the panel of the company that manages the domain (whoever sold it or hosts the site), look for "DNS", "DNS zone" or "Manage records", and create the records below.',
			'channels.email_dns_how_to' => 'Create each record below in the domain\'s DNS zone — the buttons copy name and value. After saving, propagation takes minutes to a few hours; this screen re-checks by itself.',
			'channels.fb_signup_mobile_only' => 'The Embedded Signup flow only works in the browser. On mobile, use the manual form under "Add channel".',
			'channels.email_dns_poll_stopped' => 'We stopped auto-checking — after ~10 minutes the usual cause is a wrong record, not propagation. Fix and tap refresh to check again.',
			'whatsapp.profile_load_error' => 'Error loading profile',
			'whatsapp.segment' => 'Segment',
			'whatsapp.new_template' => 'New template',
			'whatsapp.reconnect_meta' => 'Reconnect via Meta',
			'whatsapp.reconnect_meta_help' => 'Re-authorise the Meta connection to refresh the API token. Takes about a minute — you\'ll stay on this screen.',
			'whatsapp.reconnect_success' => 'Connection refreshed.',
			'whatsapp.no_templates' => 'No templates yet',
			'whatsapp.create_first_template' => 'Create first template',
			'whatsapp.delete_template' => 'Delete template',
			'whatsapp.add_button' => 'Add button',
			'whatsapp.quick_reply' => 'Quick reply',
			'whatsapp.open_url' => 'Open URL',
			'whatsapp.template_sent_success' => ({required Object phone}) => 'Template sent to ${phone}',
			'whatsapp.send_template_title' => 'Send template',
			'whatsapp.send_template_to_with_name' => ({required Object name, required Object phone}) => 'To ${name} (+${phone})',
			'whatsapp.send_template_to' => ({required Object phone}) => 'To +${phone}',
			'whatsapp.approved_template_label' => 'Approved template',
			'whatsapp.choose_approved_template' => 'Choose an approved template.',
			'whatsapp.no_approved_templates_hint' => 'No approved templates on this channel. Create one in the Templates tab and wait for Meta approval.',
			'whatsapp.loading_templates_error' => ({required Object error}) => 'Failed to load templates: ${error}',
			'whatsapp.sending' => 'Sending...',
			'whatsapp.send' => 'Send',
			'whatsapp.template_status_approved' => 'Approved',
			'whatsapp.template_status_pending' => 'In review',
			'whatsapp.template_status_rejected' => 'Rejected',
			'whatsapp.template_status_paused' => 'Paused',
			'whatsapp.template_status_disabled' => 'Disabled',
			'whatsapp.category_authentication' => 'Authentication',
			'whatsapp.language_pt_br' => 'Portuguese (BR)',
			'whatsapp.language_en_us' => 'English (US)',
			'whatsapp.language_es' => 'Spanish',
			'whatsapp.error_token_expired' => 'The WhatsApp token expired. Reconnect via Meta to restore access.',
			'whatsapp.error_outside_window' => '24h window expired. Use an approved template to reopen the conversation.',
			'whatsapp.error_missing_permission' => 'Meta denied the permission. Check that the app has whatsapp_business_management enabled.',
			'whatsapp.error_recipient' => ({required Object message}) => 'Invalid or unregistered WhatsApp recipient: ${message}',
			'whatsapp.error_template' => ({required Object message}) => 'Template error: ${message}',
			'whatsapp.error_missing_waba' => 'This channel has no WABA ID. Reconnect via Meta Embedded Signup.',
			'whatsapp.error_missing_token' => 'This channel has no access token. Reconnect via Meta Embedded Signup.',
			'whatsapp.error_register_rate_limited' => 'Meta blocked register attempts on this number due to too many calls. Wait ~24h before trying again. Meanwhile, check that the WABA has a payment method and that two-step verification is off.',
			'whatsapp.error_pin_mismatch' => 'Wrong PIN. Use the "Forgot PIN" tab to receive a code via SMS.',
			'whatsapp.error_already_verified' => 'This number is already verified on another Cloud API app. Use "Release number" to free it before registering here.',
			'whatsapp.error_cert_2fa_blocked' => 'Meta blocked the registration because two-step verification (2FA) is on for the WABA. Turn it off in Meta Business → Account settings → Two-step verification and try again.',
			'whatsapp.error_ownership_not_verified' => 'Meta hasn\'t verified ownership of this number yet. Switch to the "Forgot PIN" tab to receive a code via SMS or voice and finish registration in one go.',
			'whatsapp.error_verify_code_invalid' => 'Wrong, expired or already-used code. Click "Send code" again to receive a fresh one (Meta SMS codes only last ~10 minutes).',
			'whatsapp.error_register_failed' => ({required Object detail}) => 'Couldn\'t register the number on Cloud API. ${detail}',
			'whatsapp.error_deregister_failed' => ({required Object detail}) => 'Failed to release the number. ${detail}',
			'whatsapp.error_request_code_failed' => ({required Object detail}) => 'Meta couldn\'t send the verification code. ${detail} Try again in 1 minute, or switch between SMS and voice.',
			'whatsapp.error_verify_code_failed' => 'Invalid or expired code. Request a new one and try again.',
			'whatsapp.error_subscribe_failed' => ({required Object detail}) => 'Couldn\'t subscribe Replider to receive webhooks. ${detail}',
			'whatsapp.error_webhook_status_failed' => ({required Object detail}) => 'Couldn\'t check the webhook status. ${detail}',
			'whatsapp.error_token_exchange_failed' => 'Meta refused the token exchange during connection. Try Embedded Signup again.',
			'whatsapp.error_template_create_failed' => ({required Object detail}) => 'Meta rejected the template creation. ${detail}',
			'whatsapp.error_template_delete_failed' => ({required Object detail}) => 'Meta rejected the template deletion. ${detail}',
			'whatsapp.error_profile_fetch_failed' => 'Couldn\'t load the WhatsApp Business profile. Try reloading.',
			'whatsapp.error_profile_update_failed' => ({required Object detail}) => 'Couldn\'t save the WhatsApp Business profile. ${detail}',
			'whatsapp.error_picture_upload_failed' => 'Failed to upload the profile picture. Check that it\'s a JPG/PNG under 5MB.',
			'whatsapp.error_phone_owned_elsewhere' => 'This number is already connected to another Replider account. Contact support to release it.',
			'whatsapp.error_missing_creds' => 'Channel without valid credentials. Reconnect via Embedded Signup.',
			'whatsapp.error_not_supported' => 'Operation not supported for this channel type.',
			'whatsapp.error_trial_expired' => 'Your free trial has ended. Upgrade your plan to keep using WhatsApp.',
			'whatsapp.error_subscription_inactive' => 'Your subscription is not active. Update the payment method to reactivate.',
			'whatsapp.error_meta_code' => ({required Object code}) => 'Meta code ${code}.',
			'whatsapp.error_unknown_with_code' => ({required Object code}) => 'Unknown error (${code}).',
			'whatsapp.error_invalid_param_duplicate_name' => 'A template with this name and language already exists. Pick another name or delete the existing one before resubmitting.',
			'whatsapp.error_invalid_param_paused' => 'Template creation is paused for this account. Wait a few hours or contact Meta support.',
			'whatsapp.error_invalid_param_language' => 'This language isn\'t enabled for templates on this account yet. Try pt_BR or en_US.',
			'whatsapp.error_invalid_param_category_mismatch' => 'Meta rejected the template. The text likely doesn\'t match the chosen category: MARKETING for promotions/news, UTILITY only for confirmations/transactional notices, AUTHENTICATION only for verification codes. Also review variables {{1}} {{2}} and examples.',
			'whatsapp.error_invalid_param_default' => ({required Object message, required Object trace}) => 'Meta rejected the parameters: ${message}${trace}',
			'whatsapp.template_preview_label' => 'HOW THE CUSTOMER WILL RECEIVE IT',
			'whatsapp.create_template_link' => '+ Create new template',
			'whatsapp.create_template_title' => 'Create template',
			'whatsapp.create_template_approval_notice' => 'Meta must approve every template — usually minutes to a day. It cannot be used right NOW, but stays available here for every future conversation.',
			'whatsapp.suggestion_reopen' => 'Resume conversation',
			'whatsapp.suggestion_followup' => 'Proposal follow-up',
			'whatsapp.suggestion_blank' => 'From scratch',
			'whatsapp.create_template_name_label' => 'Name',
			'whatsapp.create_template_name_helper' => 'lowercase, numbers and _ only',
			'whatsapp.create_template_body_label' => 'Body',
			'whatsapp.create_template_body_helper' => '{{1}}, {{2}} become fill-in fields at send time. Cannot start or end with a variable.',
			'whatsapp.create_template_missing_fields' => 'Fill in the name and the body.',
			'whatsapp.create_template_submitted' => 'Template submitted — it appears in the list once Meta approves it.',
			'whatsapp.create_template_submit' => 'Submit for approval',
			'notifications.title' => 'Notifications',
			'notifications.subtitle_default' => 'Get alerts even when the tab is in the background',
			'notifications.subtitle_subscribed' => 'You\'ll get an alert when a customer writes',
			'notifications.subtitle_denied' => 'Permission blocked. Allow it in Site settings > Notifications',
			'notifications.subtitle_unsupported' => 'Browser doesn\'t support push notifications',
			'notifications.subtitle_ios_install' => 'On iPhone, install the app on the home screen to receive notifications',
			'notifications.error_loading' => ({required Object error}) => 'Couldn\'t load notification state: ${error}',
			'notifications.test_btn' => 'Send a test',
			'notifications.test_no_subs' => 'No active subscription on this device. Toggle notifications on first.',
			'notifications.test_success' => ({required Object ok, required Object failed}) => 'Test sent: ${ok} delivered, ${failed} failed.',
			'notifications.test_all_failed' => ({required Object count}) => 'All ${count} subscriptions failed. Re-enable notifications to refresh keys.',
			'notifications.test_disabled_some' => ({required Object count}) => '${count} subscription(s) disabled — likely VAPID key mismatch. Re-enable on each device.',
			'notifications.test_request_failed' => ({required Object error}) => 'Test request failed: ${error}',
			'api_keys.title' => 'AI provider keys',
			'api_keys.subtitle' => 'Your contract plan uses your own API keys for AI requests. Replider stores them encrypted and never shares them.',
			'api_keys.not_configured' => 'Not configured',
			'api_keys.input_hint' => 'Paste the API key',
			'api_keys.add' => 'Add',
			'api_keys.edit' => 'Change',
			'api_keys.save' => 'Save',
			'api_keys.clear' => 'Clear',
			'api_keys.saved' => 'Key updated.',
			'api_keys.save_failed' => ({required Object error}) => 'Couldn\'t save the key: ${error}',
			'notes.add_error' => 'Error adding note',
			'notes.delete_error' => 'Error deleting note',
			'notes.internal_notes' => 'Internal notes',
			'notes.no_notes' => 'No notes yet',
			'notes.internal_note' => 'Internal note',
			'notes.load_error' => 'Error loading notes',
			'notes.add_hint' => 'Add note...',
			'notes.delete_confirm_title' => 'Delete note?',
			'notes.delete_confirm_body' => 'This internal note will be removed for the whole team.',
			'message_actions.copy_text' => 'Copy text',
			'message_actions.message_copied' => 'Message copied',
			'message_actions.copy_message_id' => 'Copy message ID',
			'message_actions.id_copied' => 'ID copied',
			'contact_sheet.tab_info' => 'Info',
			'contact_sheet.tab_files' => 'Files',
			'contact_sheet.stat_conversations' => 'Conversations',
			'contact_sheet.stat_messages' => 'Messages',
			'contact_sheet.stat_customer_since' => 'Customer since',
			'contact_sheet.files_section_images' => 'Images & videos',
			'contact_sheet.files_section_documents' => 'Documents & audio',
			'contact_sheet.files_empty' => 'No media exchanged in this conversation yet.',
			'contact_sheet.files_error' => 'Could not load files.',
			'contact_sheet.files_unnamed' => 'Unnamed file',
			'contact_sheet.conversation_section' => 'Conversation',
			'contact_sheet.footer_stats' => ({required Object convs, required Object msgs, required Object since}) => '${convs} conversations · ${msgs} messages · first contact ${since}',
			'contact_sheet.files_section_attachments' => 'Email attachments',
			'contact_sheet.files_email_no_subject' => '(no subject)',
			'conversation.send_failed_recovered' => 'Failed to send. Message recovered — please try again.',
			'conversation.message_send_failed' => 'Not delivered',
			'conversation.resume_bot' => 'Resume Bot',
			'conversation.file_read_error' => 'Could not read the file.',
			'conversation.file_send_error' => ({required Object error}) => 'Error sending file: ${error}',
			'conversation.nobody' => 'Nobody',
			'conversation.assume' => 'Take over',
			'conversation.release' => 'Return to bot',
			'conversation.under_takeover_by' => ({required Object name}) => 'Being handled by ${name}',
			'conversation.under_takeover_anonymous' => 'Under human handling',
			'conversation.input_disabled_tooltip' => 'Take over the conversation to reply manually',
			'conversation.resolve_action' => 'Resolve',
			'conversation.reopen_action' => 'Reopen',
			'conversation.load_older_messages' => 'Load earlier messages',
			'conversation.resolve_confirm_title' => 'Resolve conversation?',
			'conversation.resolve_confirm_body' => 'The conversation will move to "Closed". You can reopen it later from the list filter.',
			'conversation.convert_lead_action' => 'Convert lead',
			'conversation.convert_lead_title' => 'Convert lead into customer',
			'conversation.convert_lead_help' => 'Links a customer account to this conversation\'s assigned attendant as seller (inbound lead, 20% track).',
			'conversation.convert_lead_search_hint' => 'Account name or email',
			'conversation.convert_lead_search_min' => 'Type at least 2 characters to search.',
			'conversation.convert_lead_no_results' => 'No account matches this search.',
			'conversation.convert_lead_already_linked' => 'Already linked to a seller',
			'conversation.convert_lead_cta' => 'Convert',
			'conversation.convert_lead_success' => ({required Object account}) => 'Lead converted: ${account} is now in the assigned attendant\'s portfolio.',
			'conversation.convert_lead_error' => ({required Object error}) => 'Could not convert lead: ${error}',
			'conversation.priority_urgent' => 'Urgent',
			'conversation.priority_high' => 'High',
			'conversation.priority_normal' => 'Normal',
			'conversation.priority_low' => 'Low',
			'conversation.assign_label' => 'Assign',
			'conversation.tags_label' => 'Tags',
			'conversation.tag_picker_title' => 'Select Tags',
			'conversation.tag_picker_new' => 'New tag',
			'conversation.tag_picker_search' => 'Search tag...',
			'conversation.tag_picker_empty_title' => 'You don\'t have any tags yet.',
			'conversation.tag_picker_empty_body' => 'Create your first tag using the button above.',
			'conversation.tag_picker_no_match' => ({required Object query}) => 'No tag matching "${query}".',
			'conversation.tag_update_error' => ({required Object error}) => 'Error updating tag: ${error}',
			'conversation.tag_create_error' => ({required Object error}) => 'Error creating tag: ${error}',
			'conversation.tag_color_label' => 'Color',
			'conversation.tag_dialog_title' => 'New tag',
			'conversation.tag_name_label' => 'Name',
			'conversation.tag_name_hint' => 'e.g. VIP, Support, Question',
			'conversation.tag_dialog_create' => 'Create',
			'conversation.contact_bio' => 'Bio',
			'conversation.contact_status_label' => 'Status',
			'conversation.contact_bot_label' => 'Bot',
			'conversation.contact_bot_paused' => 'Paused',
			'conversation.contact_bot_active' => 'Active',
			'conversation.contact_handled_by' => 'Handled by',
			'conversation.contact_last_message' => 'Last message',
			'conversation.contact_close_btn' => 'Close',
			'conversation.status_open' => 'Open',
			'conversation.status_pending' => 'Waiting for agent',
			'conversation.source_whatsapp' => 'WhatsApp',
			'conversation.source_telegram' => 'Telegram',
			'conversation.source_instagram' => 'Instagram',
			'conversation.source_sms' => 'SMS',
			'conversation.source_internal' => 'Internal support',
			'conversation.source_email' => 'Email',
			'conversation.shared_contact' => 'Shared contact',
			'conversation.shared_contact_copied' => 'Number copied',
			'conversation.media_video' => 'Video',
			'conversation.resume_bot_other_confirm' => ({required Object name}) => 'This conversation is being handled by ${name}. Hand it back to the bot anyway?',
			'conversation.contact_handled_by_inline' => ({required Object name}) => 'handled by ${name}',
			'conversation.contact_bot_paused_inline' => 'bot paused',
			'conversation.contact_bot_active_inline' => 'bot active',
			'lgpd.anonymize_title' => 'Anonymize data?',
			'lgpd.anonymize' => 'Anonymize',
			'lgpd.copy_json' => 'Copy JSON',
			'lgpd.no_actions' => 'No actions recorded yet.',
			'lgpd.audit_action_export' => 'GDPR export',
			'lgpd.audit_action_anonymize' => 'GDPR anonymisation',
			'lgpd.screen_title' => 'Data privacy & audit',
			'lgpd.section_data_subject_rights' => 'DATA SUBJECT RIGHTS (Art. 18 of the Brazilian LGPD)',
			'lgpd.section_infrastructure' => 'INFRASTRUCTURE',
			'lgpd.section_audit_trail' => 'AUDIT TRAIL',
			'lgpd.anonymize_confirm_body' => ({required Object externalId}) => 'This action is IRREVERSIBLE. All personal data for contact ${externalId} will be erased (name, phone, messages). Aggregate metrics (counts, campaigns) are preserved. The action is recorded in the audit log.',
			'lgpd.json_copied_hint' => ({required Object id}) => 'JSON copied. Save it to a file lgpd-export-${id}.json',
			'lgpd.anonymize_success' => 'Contact data anonymised. The action was recorded in the audit log.',
			'lgpd.titular_search_hint' => 'Search a contact by phone (E.164, e.g. 5511999999999) to export or erase their data.',
			'lgpd.data_found_for' => ({required Object id}) => 'Data found for ${id}',
			'lgpd.stat_conversations' => 'Conversations',
			'lgpd.stat_messages' => 'Messages',
			'lgpd.stat_campaigns' => 'Campaigns',
			'lgpd.audit_records_summary' => ({required Object shown, required Object total}) => '${shown} of ${total} records',
			'lgpd.intro_title' => 'What this screen is for',
			'lgpd.intro_body' => 'Admin panel for privacy and audit. This is where you respond to customer requests under data privacy law, test the server\'s email delivery, and review everything that account admins have done.',
			'lgpd.intro_bullet_rights_title' => 'Data subject rights',
			'lgpd.intro_bullet_rights_body' => 'Export or anonymise a contact\'s data (name, phone, messages) — handles Art. 18 of the Brazilian LGPD in one click.',
			'lgpd.intro_bullet_infra_title' => 'Infrastructure',
			'lgpd.intro_bullet_infra_body' => 'Quick SMTP server test to make sure verification, password recovery and alert emails are going out.',
			'lgpd.intro_bullet_audit_title' => 'Audit trail',
			'lgpd.intro_bullet_audit_body' => 'Timeline of every administrative action (takeover, anonymisation, channel creation, etc.). Use the filters to track an actor or a specific action; export CSV to attach to reports.',
			'feedback.nps_title' => 'How likely are you to recommend Replider?',
			'feedback.nps_subtitle' => 'On a scale from 0 to 10, how likely are you to recommend Replider to a colleague or friend?',
			'feedback.nps_scale_low' => 'Not likely',
			'feedback.nps_scale_high' => 'Very likely',
			'feedback.nps_comment_hint' => 'What\'s the main reason for your score? (optional)',
			'feedback.nps_submit' => 'Send feedback',
			'feedback.nps_later' => 'Maybe later',
			'feedback.nps_thanks' => 'Thanks for your feedback!',
			'feedback.cancel_title' => 'Before you go — why are you cancelling?',
			'feedback.cancel_subtitle' => 'Your feedback helps us improve Replider. This takes just a few seconds.',
			'feedback.cancel_comment_hint' => 'Anything else you\'d like to share? (optional)',
			'feedback.cancel_submit' => 'Send feedback',
			'feedback.cancel_thanks' => 'Thanks for your feedback!',
			'feedback.cancel_reasons.too_expensive' => 'Too expensive',
			'feedback.cancel_reasons.missing_features' => 'Missing features I need',
			'feedback.cancel_reasons.not_using' => 'I\'m not using it enough',
			'feedback.cancel_reasons.switched_competitor' => 'Switched to another tool',
			'feedback.cancel_reasons.technical_issues' => 'Technical issues or bugs',
			'feedback.cancel_reasons.temporary_pause' => 'Just pausing for now',
			'feedback.cancel_reasons.other' => 'Other',
			'product_tour.skip' => 'Skip tour',
			'product_tour.next' => 'Next',
			'product_tour.done' => 'Got it',
			'product_tour.step_counter' => ({required Object current, required Object total}) => '${current} of ${total}',
			'product_tour.step1_title' => 'Welcome to Replider 🎉',
			'product_tour.step1_body' => 'Your AI agent is ready. Here\'s a quick 30-second tour of where everything lives so you can get the most out of it.',
			'product_tour.step2_title' => 'Your inbox',
			'product_tour.step2_body' => 'Every conversation from your channels lands here in real time. Jump in anytime to take over from the agent and reply yourself.',
			'product_tour.step3_title' => 'Train your agent',
			'product_tour.step3_body' => 'Add knowledge — prices, hours, policies — in Settings so the agent answers accurately instead of guessing. The more you add, the smarter it gets.',
			'product_tour.step4_title' => 'Watch your usage',
			'product_tour.step4_body' => 'The bar at the bottom tracks conversations and tokens for your billing cycle. Upgrade anytime to lift your limits as you grow.',
			'feature_announcement.title' => 'What\'s new',
			'feature_announcement.release_label' => 'Latest updates',
			'feature_announcement.cta' => 'Got it',
			'feature_announcement.item1_title' => 'Manage your subscription anytime',
			'feature_announcement.item1_body' => 'Open the secure billing portal right from the app to update your card, switch plans or cancel — no support ticket needed.',
			'feature_announcement.item2_title' => 'Save 20% with annual billing',
			'feature_announcement.item2_body' => 'Switch any plan to yearly and pay less per month. Toggle the interval on the billing screen to see your new price.',
			'feature_announcement.item3_title' => 'Usage alerts before you hit the limit',
			'feature_announcement.item3_body' => 'We now warn you at 80% of your monthly quota, so a busy week never blocks your agent by surprise.',
			'errors.AUTH_INVALID_CREDENTIALS' => 'Invalid email or password.',
			'errors.WEBHOOK_URL_BLOCKED' => 'The webhook URL cannot point at an internal network address.',
			'errors.AUTH_EMAIL_ALREADY_EXISTS' => 'This email is already registered.',
			'errors.REGISTRATION_PAUSED' => 'Sign-ups are temporarily closed. Already have an account? Just log in.',
			'errors.OWNER_EMAIL_EXISTS' => 'This email already belongs to an account. Use the "existing account" mode to apply a contract to it.',
			'errors.OWNER_SELLER_NOT_FOUND' => 'No active seller with this email on the platform account.',
			'errors.COMMISSION_SELLER_NOT_FOUND' => 'This user was not found on the platform account.',
			'errors.OWNER_SELLER_INACTIVE' => 'This seller is deactivated.',
			'errors.OWNER_INVALID_LEAD_SOURCE' => 'Invalid lead source — use prospected (ref) or inbound.',
			'errors.OWNER_PLATFORM_NOT_CONFIGURED' => 'PLATFORM_ACCOUNT_ID is not configured in this environment.',
			'errors.SELLER_CONV_UNASSIGNED' => 'This conversation has no assigned attendant. Take it over (or assign it) before converting.',
			'errors.SELLER_NOT_CONV_ASSIGNEE' => 'Only the assigned attendant or an admin can convert this lead.',
			'errors.SELLER_ACCOUNT_ALREADY_LINKED' => 'This account is already linked to a seller. Reallocation is done from the owner panel.',
			'errors.SELLER_CANNOT_CONVERT_PLATFORM' => 'The platform account itself can\'t be converted.',
			'errors.SELLER_NOT_FLAGGED' => 'You are not a registered seller. Ask the owner to grant seller access.',
			'errors.SELLER_CONV_TARGET_UNRELATED' => 'This account already has payment history and the conversation\'s contact does not match it. Conversion blocked.',
			'errors.AUTH_UNAUTHORIZED' => 'Session expired. Please login again.',
			'errors.AUTH_SOCIAL_FAILED' => 'Social authentication failed. Please try again.',
			'errors.AUTH_INVALID_RESET_TOKEN' => 'Invalid or expired reset link.',
			'errors.AUTH_EMAIL_NOT_VERIFIED' => 'Please verify your email before logging in. Check your inbox or request a new link.',
			'errors.AGENT_NOT_FOUND' => 'Agent not found.',
			'errors.AGENT_LIMIT_REACHED' => 'You\'ve reached the agent limit for your plan. Upgrade to create more agents.',
			'errors.PLAN_LIMIT_EXCEEDED' => 'You reached your contract\'s limit. Talk to us to expand it.',
			'errors.PLAN_LIMIT_EXCEEDED_agents' => ({required Object limit, required Object current}) => 'Your contract covers ${limit} agents (you have ${current}). Talk to us to expand it.',
			'errors.PLAN_LIMIT_EXCEEDED_channels' => ({required Object limit, required Object current}) => 'Your contract covers ${limit} channels (you have ${current}). Talk to us to expand it.',
			'errors.PLAN_LIMIT_EXCEEDED_seats' => ({required Object limit, required Object current}) => 'Your contract covers ${limit} seats (you have ${current}). Talk to us to expand it.',
			'errors.CONV_NOT_FOUND' => 'Conversation not found.',
			'errors.CONV_OUTSIDE_24H' => 'Outside the WhatsApp 24h messaging window.',
			'errors.USAGE_INSUFFICIENT_TOKENS' => 'Insufficient AI tokens.',
			'errors.USAGE_NO_CREDITS' => 'AI usage limit reached.',
			'errors.USAGE_LIMIT_EXCEEDED' => 'This cycle\'s usage limit was reached. Talk to us to expand it, or wait for the next cycle.',
			'errors.ACCOUNT_SUSPENDED' => 'Account suspended due to an unpaid invoice. Update your payment method to continue.',
			'errors.TEAM_CANNOT_REMOVE_LAST_ADMIN' => 'Cannot remove the last admin. Transfer the admin role first.',
			'errors.TEAM_INVALID_PARTICIPANT' => 'Selected participant is not an active member of the team.',
			'errors.RATE_LIMITED' => 'Too many requests. Please slow down and try again in a minute.',
			'errors.TRIAL_EXPIRED' => 'Your trial has ended. Talk to us to activate your contract.',
			'errors.SUBSCRIPTION_INACTIVE' => 'Your account is not active. Talk to us to sort it out.',
			'errors.TERMS_NOT_ACCEPTED' => 'You must accept the Terms of Service and Privacy Policy.',
			'errors.CONNECTION_TIMEOUT' => 'The server took too long to respond. Try again in a moment.',
			'errors.CONNECTION_ERROR' => 'We couldn\'t reach the server. It may be your connection or a problem on our side — try again in a moment.',
			'errors.SERVER_ERROR' => 'Server temporarily unavailable. Please try again in a few minutes.',
			'errors.REQUEST_CANCELED' => 'Request canceled.',
			'errors.BAD_REQUEST' => 'Invalid input. Please check the fields and try again.',
			'errors.UNKNOWN' => 'An unexpected error occurred. Please try again.',
			'errors.TOTP_INVALID_CODE' => 'Code did not match. Check the time on your device and try again.',
			'errors.OWNER_2FA_STALE' => 'Your two-factor proof expired. Confirm the code to continue.',
			'errors.OWNER_2FA_REQUIRED' => 'Enable 2FA in your profile to use the owner panel.',
			'errors.TOTP_NOT_STAGED' => 'Start the 2FA setup before confirming.',
			'errors.TOTP_NOT_ENABLED' => 'Two-factor authentication isn\'t enabled.',
			'errors.TOTP_CIPHER_UNCONFIGURED' => 'Two-factor authentication is unavailable right now. Try again later.',
			'errors.MESSAGE_SEND_FAILED' => 'The message was saved but could not be delivered to the customer. Check the channel and try again.',
			'errors.PLAN_NOT_FOUND' => 'Plan not found. Refresh the page and try again.',
			'errors.PLAN_NOT_PURCHASABLE' => 'This plan is sold under contract. Talk to a specialist to subscribe.',
			'errors.PLAN_CURRENCY_MISMATCH' => 'This plan is not available in your account\'s billing currency.',
			'errors.COUPON_EXHAUSTED' => 'This coupon has reached its redemption limit.',
			'errors.ADDON_CURRENCY_UNSUPPORTED' => 'Extra credit packs are not available in your account\'s billing currency.',
			'errors.BROADCAST_NOT_ALLOWED' => 'Your plan doesn\'t include bulk campaigns. Upgrade to enable broadcasts to contact lists.',
			'errors.RECIPIENTS_OVER_CAMPAIGN_CAP' => ({required Object current, required Object limit}) => 'This campaign has more recipients than your plan allows (${current}/${limit}). Split it into smaller campaigns or upgrade.',
			'errors.MONTHLY_BROADCAST_CAP' => ({required Object current, required Object limit}) => 'You\'ve reached the monthly broadcast cap (${current}/${limit}). Wait for renewal or upgrade.',
			'errors.EMPTY_NAME' => 'Name is empty.',
			'errors.UNEXPECTED' => 'Unexpected error.',
			'errors.JSON_INVALID_OBJECT' => 'flow.json must be a valid JSON object.',
			'errors.JSON_INVALID_FORMAT' => ({required Object message}) => 'Invalid JSON: ${message}',
			'errors.PROVIDER_COST_NOT_FOUND' => 'Price row not found.',
			'errors.PROVIDER_COST_DUPLICATE' => 'A price for this provider/model/date already exists.',
			'errors.PROVIDER_COST_INVALID_RATE' => 'Rate must be between 1.00 and 100.00 BRL per USD.',
			'errors.CANNED_RESPONSE_FORBIDDEN' => 'Only admins can manage team-wide quick replies.',
			'errors.MCP_TOKEN_NOT_FOUND' => 'Token not found or already revoked.',
			'errors.MCP_TOKEN_LIMIT_REACHED' => 'Active token limit reached (10). Revoke an old token before creating another.',
			'errors.LEAD_INVALID_PHONE' => 'Invalid phone. Use area code + number, like 51 98888-7766.',
			'errors.LEAD_NOT_PENDING_APPROVAL' => 'That approach is no longer waiting for approval.',
			'errors.LEAD_SEND_FAILED' => 'Couldn\'t send the approach. Try again in a moment.',
			'errors.LEAD_NOT_FOUND' => 'Lead not found.',
			'errors.APPROACH_HAS_PLACEHOLDER' => 'The message still has a field to fill in.',
			'errors.EMAIL_ADDRESS_SUPPRESSED' => 'This address no longer receives email: it bounced permanently or reported us as spam. Check Settings → Suppressed addresses.',
			'errors.AI_DRAFTS_NOT_ALLOWED' => 'Your plan does not include AI drafts. Upgrade to have the AI write your approaches.',
			'errors.DOC_EXTRACTION_NOT_ALLOWED' => 'Your plan does not include AI contract reading. Upgrade to have contracts read automatically.',
			'errors.SPEND_BUDGET_EXCEEDED' => 'You have reached this feature\'s limit for the cycle. Wait for the renewal or upgrade.',
			'errors.route_not_found_title' => 'This screen doesn\'t exist',
			'errors.route_not_found_body' => 'The address may have a typo, or the link is out of date.',
			'errors.route_not_found_cta' => 'Go to start',
			'errors.PRODUCT_PDF_UNREADABLE' => 'I could not open that file as a PDF. Send the manufacturer\'s original brochure.',
			'errors.PRODUCT_PDF_SCAN_UNAVAILABLE' => 'The antivirus is not available right now — PDF upload is disabled. Try again later.',
			'errors.PRODUCT_PDF_INFECTED' => 'The antivirus refused this file.',
			'errors.PRODUCT_PDF_EXTRACT_FAILED' => 'The automatic reading failed. Try again, or use the CSV import.',
			'errors.PRODUCT_PDF_NO_PRODUCTS' => 'The reading found no products in this PDF.',
			'errors.PRODUCT_PDF_PROVIDER_CANT_READ' => 'This account\'s AI provider cannot read files. Use the CSV import instead.',
			'errors.PRODUCT_PDF_IMPORT_UNAVAILABLE' => 'PDF import is not configured in this environment.',
			'errors.EMAIL_WARMUP_NOT_STARTED' => 'This mailbox never started its warm-up — verify the domain (DNS) or reconnect the mailbox before cold outreach.',
			'errors.BULK_SENDING_NOT_READY' => 'Batch approvals need volume sending configured for your domain. Enable it in Settings → Channels (email channel), or approve one by one through your own mailbox.',
			'errors.BULK_SENDING_UNAVAILABLE' => 'Volume sending is not available on the platform right now — approve one by one, or talk to us.',
			'errors.EMAIL_DAILY_CAP_REACHED' => 'The mailbox reached its daily cold-send cap. Sending resumes tomorrow.',
			'errors.LEAD_APPROACH_TEMPLATE_REQUIRED' => 'This send needs an approved WhatsApp template — open the lead and pick one.',
			'errors.INVITE_INVALID' => 'Invalid, expired or already-used invite — ask for a new link from whoever invited you.',
			'errors.SALES_FUNNEL_IS_DEFAULT' => 'The default funnel can\'t be deleted — it\'s where imports and new leads land.',
			'errors.PROSPECTING_NOT_CONFIGURED' => 'Company search isn\'t available on the platform right now — talk to support.',
			'errors.PROSPECTING_RATE_LIMITED' => 'Too many searches in a row — wait a moment and try again.',
			'errors.PROSPECTING_INPUT_REQUIRED' => 'Tell us what to search: a segment and a city.',
			'errors.PROSPECTING_SEARCH_FAILED' => 'The company search failed — try again in a moment.',
			'errors.APPROACH_NEEDS_PRODUCTS' => 'Register your products first — with an empty catalog the draft comes out generic.',
			'errors.LEAD_FUNNEL_NO_CHANNEL' => 'Connect an e-mail in Settings → Channels to send.',
			'errors.LEAD_CONTACT_NOT_MESSAGEABLE' => 'This customer has no e-mail on file.',
			'errors.LEAD_ALREADY_CONTACTED' => 'This customer was already approached — reply on the conversation instead.',
			'errors.CONTACT_OPTED_OUT' => 'This customer asked not to be contacted.',
			'errors.BYOK_NOT_CONFIGURED' => 'Add your AI key in Settings → Contract.',
			'errors.FORBIDDEN' => 'Your role has no access to this.',
			'errors.LEAD_NO_CONVERSATION' => 'Nobody has written to this customer yet — send the first offer from the queue.',
			'errors.AI_PROVIDER_UNAVAILABLE' => 'The AI provider is unavailable right now. Try again in a moment.',
			'errors.FUNNEL_STAGE_BACKWARD' => 'A lead that was already contacted cannot go back to the start of the funnel. Mark it lost or create another lead.',
			'audio_recorder.permission_blocked_title' => 'Microphone blocked',
			'audio_recorder.permission_blocked_body' => 'Allow microphone access in your browser/system settings and try again.',
			'audio_recorder.ok' => 'OK',
			'audio_recorder.cancel_tooltip' => 'Cancel',
			'audio_recorder.pause_tooltip' => 'Pause',
			'audio_recorder.resume_tooltip' => 'Continue',
			'audio_recorder.stop_tooltip' => 'Finish recording',
			'audio_recorder.discard_tooltip' => 'Discard',
			'audio_recorder.redo_tooltip' => 'Re-record',
			'audio_recorder.play_tooltip' => 'Play',
			'audio_recorder.pause_play_tooltip' => 'Pause',
			'audio_recorder.send_tooltip' => 'Send audio',
			'audio_recorder.start_failed' => ({required Object error}) => 'Failed to start recording: ${error}',
			'audio_recorder.pause_failed' => ({required Object error}) => 'Failed to pause: ${error}',
			'audio_recorder.stop_failed' => ({required Object error}) => 'Failed to stop recording: ${error}',
			'audio_recorder.send_failed' => ({required Object error}) => 'Failed to send audio: ${error}',
			'audio_recorder.empty_recording' => 'Empty recording.',
			'audio_recorder.empty_audio' => 'Empty audio.',
			'bulk_actions.clear_tooltip' => 'Clear selection',
			'bulk_actions.selected_count' => ({required Object count}) => '${count} selected',
			'bulk_actions.select_all' => 'Select all',
			'bulk_actions.close_btn' => 'Close',
			'bulk_actions.close_confirm_title' => ({required Object count}) => 'Close ${count} conversations?',
			'bulk_actions.close_confirm_body' => 'The conversations will move to "Closed". New customer messages reopen them automatically.',
			'bulk_actions.close_done' => ({required Object ok, required Object total}) => '${ok} of ${total} closed.',
			'bulk_actions.close_partial_warning' => ' Some failed — check permissions.',
			'bulk_actions.close_failed' => ({required Object error}) => 'Failed: ${error}',
			'image_viewer.default_filename' => 'Image',
			'image_viewer.default_save_filename' => 'image.jpg',
			'image_viewer.download_btn' => 'Download',
			'image_viewer.close_tooltip' => 'Close',
			'image_viewer.download_failed' => 'Could not download the image.',
			'media_send.file_too_large' => ({required Object actualMb, required Object maxMb}) => 'File too large (${actualMb} MB). The limit for this type is ${maxMb} MB.',
			'media_send.sending_file' => ({required Object filename}) => 'Sending ${filename}...',
			'media_send.send_file_dialog_title' => 'Send file',
			'media_send.caption_label' => 'Caption (optional)',
			'media_send.send_btn' => 'Send',
			'media_send.cancel_btn' => 'Cancel',
			'media_send.upload_in_progress' => 'Another file is already uploading on this conversation. Wait for it to finish.',
			'reply_suggestions.loading' => 'Generating suggestions...',
			'reply_suggestions.load_failed' => 'Failed to generate suggestions. Try again.',
			'reply_suggestions.retry' => 'Retry',
			'reply_suggestions.suggest_replies' => 'Suggest replies',
			'reply_suggestions.refresh_tooltip' => 'Refresh suggestions',
			'reply_suggestions.hide_tooltip' => 'Hide',
			'pwa.install_title' => 'Install the app',
			'pwa.install_body' => 'Get notified when customers message you, even when the app is closed.',
			'pwa.install_btn' => 'Install',
			'pwa.not_now' => 'Not now',
			'pwa.ios_title' => 'Add to home screen',
			'pwa.ios_body' => 'Required to receive iPhone notifications — Safari only allows alerts inside installed apps.',
			'pwa.dismiss_tooltip' => 'Dismiss',
			'pwa.ios_step_1' => 'Tap Share in Safari\'s bottom bar',
			'pwa.ios_step_2' => 'Choose "Add to Home Screen"',
			'pwa.ios_step_3' => 'Open Replider from the installed icon and turn on notifications',
			'audio_player.transcribe_tooltip' => 'Transcribe',
			'audio_player.transcribing' => 'Transcribing...',
			'audio_player.transcribe_failed' => 'Failed to transcribe, try again.',
			'audio_player.load_failed' => 'Couldn\'t load the audio. It may have expired.',
			'media_preview.audio' => 'Audio',
			'media_preview.image' => 'Image',
			'media_preview.video' => 'Video',
			'media_preview.document' => 'File',
			'media_preview.location' => 'Location',
			'media_preview.sticker' => 'Sticker',
			'sentiment.detected_tooltip' => ({required Object label}) => 'Detected sentiment: ${label}',
			'sentiment.angry' => 'Angry',
			'sentiment.negative' => 'Negative',
			'sentiment.positive' => 'Positive',
			'owner.menu_label' => 'Owner panel',
			'owner.accounts_title' => 'Accounts',
			'owner.search_hint' => 'Search by company name or email',
			'owner.create_contract_cta' => 'New contract',
			'owner.empty_title' => 'No accounts yet',
			'owner.empty_desc' => 'Use "New contract" to provision your first B2B client.',
			'owner.badge_suspended' => 'suspended',
			'owner.detail_title' => 'Account',
			'owner.field_id' => 'ID',
			'owner.field_status' => 'Status',
			'owner.field_owner' => 'Owner',
			'owner.field_plan' => 'Plan',
			'owner.field_created' => 'Created',
			'owner.field_internal_threads' => 'Active internal support threads',
			'owner.field_suspended_reason' => 'Suspended reason',
			'owner.seller_label' => 'Seller',
			'owner.seller_none' => 'No seller linked',
			'owner.seller_source_label' => 'Lead source',
			'owner.seller_source_seller_ref' => 'Prospected (ref) — 50%',
			'owner.seller_source_inbound' => 'Inbound — 20%',
			'owner.seller_linked_at' => 'Linked at',
			'owner.seller_edit_cta' => 'Link / reallocate',
			'owner.seller_self_match_warning' => 'Seller matches this account\'s own admin (self-referral) — review the link',
			'owner.seller_dialog_title' => 'Link seller',
			_ => null,
		} ?? switch (path) {
			'owner.seller_email_hint' => 'Seller email (platform-account user)',
			'owner.seller_unlink_cta' => 'Unlink',
			'owner.seller_save_cta' => 'Save',
			'owner.seller_saved' => 'Seller updated.',
			'owner.overrides_label' => 'Caps (overrides)',
			'owner.overrides_clear_hint' => 'Leave a field empty to clear the override and fall back to the plan default.',
			'owner.edit_overrides_cta' => 'Edit caps',
			'owner.cap_max_agents' => 'Max agents',
			'owner.cap_max_channels' => 'Max channels',
			'owner.cap_max_conversations' => 'Max conversations / month',
			'owner.cap_max_tokens' => 'Max AI tokens / month',
			'owner.usage_label' => 'Usage this period',
			'owner.usage_tokens' => 'Tokens used',
			'owner.usage_addon' => 'Addon tokens',
			'owner.usage_conversations' => 'Conversations',
			'owner.usage_messages' => 'Messages',
			'owner.suspend_cta' => 'Suspend account',
			'owner.suspend_dialog_title' => 'Suspend account?',
			'owner.suspend_reason_hint' => 'Reason (required)',
			'owner.suspend_confirm_cta' => 'Suspend',
			'owner.reactivate_cta' => 'Reactivate account',
			'owner.reactivate_confirm_title' => 'Reactivate this account?',
			'owner.reactivate_confirm_msg' => 'Inbound messages and writes will resume immediately.',
			'owner.create_title' => 'New contract',
			'owner.create_company' => 'Company name',
			'owner.create_admin_email' => 'Admin email',
			'owner.create_admin_name' => 'Admin name',
			'owner.create_plan' => 'Plan',
			'owner.create_trial_bypass' => 'Skip trial gate',
			'owner.create_trial_bypass_hint' => 'The account will be usable immediately without a Stripe subscription.',
			'owner.create_submit' => 'Create contract',
			'owner.form_required' => 'Fill in company, admin email and plan.',
			'owner.top_label' => 'Top spenders this period',
			'owner.top_open_cta' => 'Open',
			'owner.grant_cta' => 'Grant credits',
			'owner.grant_dialog_title' => 'Grant addon tokens',
			'owner.grant_hint' => 'Number of tokens',
			'owner.extend_cta' => 'Extend trial',
			'owner.extend_dialog_title' => 'Extend trial by',
			'owner.extend_hint' => 'Days from now',
			'owner.impersonate_cta' => 'Login as account',
			'owner.impersonate_confirm_title' => 'Login as this account?',
			'owner.impersonate_confirm_msg' => 'You\'ll act as the account\'s admin user for 15 minutes. Every action is audited.',
			'owner.banner_acting_as' => ({required Object account}) => 'Acting as ${account} — every action is logged.',
			'owner.banner_end_cta' => 'End',
			'owner.banner_expires_in' => ({required Object time}) => 'expires in ${time}',
			'owner.filter_all' => 'All',
			'owner.filter_active' => 'Active',
			'owner.filter_trialing' => 'Trialing',
			'owner.filter_past_due' => 'Past due',
			'owner.filter_suspended' => 'Suspended',
			'owner.cap_badge' => ({required Object pct}) => '${pct}% of cap',
			'owner.logout_blocked_during_impersonate' => 'End the impersonation session before logging out.',
			'owner.conversations_label' => 'Recent conversations',
			'owner.conversations_empty' => 'This account has no conversations yet.',
			'owner.health_accounts' => 'Accounts',
			'owner.health_active' => 'Active',
			'owner.health_suspended' => 'Suspended',
			'owner.health_conv_24h' => 'Conv 24h',
			'owner.health_msg_24h' => 'Msg 24h',
			'owner.usage_trend_label' => 'Usage trend',
			'owner.usage_trend_desc' => 'AI tokens consumed per day, last 30 days.',
			'owner.usage_trend_empty' => 'No usage in the last 30 days.',
			'owner.usage_trend_total' => ({required Object tokens}) => '${tokens} tokens total',
			'owner.invoices_label' => 'Invoices',
			'owner.invoices_desc' => 'Last 10 Stripe invoices for this account.',
			'owner.invoices_empty' => 'No invoices yet — account has not transacted with Stripe.',
			'owner.invoice_status_paid' => 'Paid',
			'owner.invoice_status_open' => 'Open',
			'owner.invoice_status_draft' => 'Draft',
			'owner.invoice_status_uncollectible' => 'Uncollectible',
			'owner.invoice_status_void' => 'Void',
			'owner.invoice_open_hosted' => 'Open invoice page',
			'owner.invoice_open_pdf' => 'Download PDF',
			'owner.gate_enroll_title' => 'Enrol in 2FA to continue',
			'owner.gate_enroll_body' => 'Owner-panel access requires two-factor authentication. Set it up from your profile and come back.',
			'owner.gate_enroll_cta' => 'Go to profile',
			'owner.gate_refresh_title' => 'Re-confirm your 2FA code',
			'owner.gate_refresh_body' => 'Your two-factor proof expired. Enter the code from your authenticator to continue — no need to sign out.',
			'owner.gate_refresh_cta' => 'Confirm code',
			'owner.stepup_title' => 'Confirm your 2FA',
			'owner.stepup_body' => 'Your two-factor proof expired. Enter the code from your authenticator to continue — nothing you typed is lost.',
			'owner.stepup_code' => '6-digit code',
			'owner.stepup_confirm' => 'Confirm',
			'owner.stepup_hint' => 'You can also use a recovery code.',
			'owner.coupons_title' => 'Coupons',
			'owner.coupons_empty_title' => 'No coupons yet',
			'owner.coupons_empty_desc' => 'Mint a code and share the invite link to apply a discount at checkout.',
			'owner.coupon_create_cta' => 'New coupon',
			'owner.coupon_create_title' => 'Create coupon',
			'owner.coupon_code_label' => 'Code',
			'owner.coupon_percent_label' => 'Discount',
			'owner.coupon_max_label' => 'Max redemptions',
			'owner.coupon_max_hint' => 'Leave blank for unlimited',
			'owner.coupon_expires_pick' => 'Set expiry',
			'owner.coupon_expires_none' => 'No expiry',
			'owner.coupon_expires' => ({required Object date}) => 'Expires ${date}',
			'owner.coupon_save_cta' => 'Create',
			'owner.coupon_form_invalid' => 'Code and discount are required',
			'owner.coupon_created_toast' => 'Coupon created',
			'owner.coupon_status_active' => 'ACTIVE',
			'owner.coupon_status_inactive' => 'INACTIVE',
			'owner.coupon_percent_off' => ({required Object percent}) => '${percent}% off',
			'owner.coupon_redeemed_unlimited' => ({required Object used}) => '${used} redemptions',
			'owner.coupon_redeemed_capped' => ({required Object used, required Object cap}) => '${used} / ${cap} redeemed',
			'owner.coupon_deactivate_cta' => 'Deactivate',
			'owner.coupon_deactivate_title' => 'Deactivate coupon?',
			'owner.coupon_deactivate_body' => ({required Object code}) => 'Code ${code} will stop accepting new redemptions. Existing subscriptions keep their discount.',
			'owner.tab_overview' => 'Overview',
			'owner.tab_usage' => 'Usage',
			'owner.tab_invoices' => 'Invoices',
			'owner.tab_conversations' => 'Conversations',
			'owner.actions_menu' => 'Actions',
			'owner.conversations_tap_hint' => 'Tap a conversation to enter this account',
			'owner.sort_label' => 'Sort',
			'owner.sort_consumption' => 'Usage',
			'owner.sort_recent' => 'Recent',
			'owner.sort_status' => 'Status',
			'owner.overdue_first' => 'Overdue on top',
			'owner.usage_no_cap' => 'No limit',
			'owner.mode_new_account' => 'New account',
			'owner.mode_existing_account' => 'Existing account',
			'owner.pick_account_hint' => 'Search the account to apply the contract',
			'owner.pick_account_label' => 'Account',
			'owner.apply_contract_submit' => 'Apply contract',
			'owner.pick_account_required' => 'Select an account first',
			'owner.wa_risk_ban' => 'Ban risk',
			'owner.wa_quality_warning' => 'WhatsApp quality',
			'owner.apply_plan_cta' => 'Set plan / contract',
			'owner.use_our_key_title' => 'Run on our AI key',
			'owner.use_our_key_hint' => 'Bill on the platform key at the plan rate instead of requiring the client to bring their own.',
			'owner.provider_costs_title' => 'Provider costs',
			'owner.provider_costs_empty_title' => 'No prices yet',
			'owner.provider_costs_empty_desc' => 'Add the real API price per model so monthly profit is computed on actual cost.',
			'owner.provider_cost_add_cta' => 'Add price',
			'owner.provider_cost_add_title' => 'New provider price',
			'owner.provider_cost_edit_title' => 'Fix provider price',
			'owner.provider_cost_provider_label' => 'Provider',
			'owner.provider_cost_model_label' => 'Model',
			'owner.provider_cost_input_label' => 'Input / 1M tokens',
			'owner.provider_cost_output_label' => 'Output / 1M tokens',
			'owner.provider_cost_costs_line' => ({required Object input, required Object output}) => 'in ${input}¢ / out ${output}¢ USD per 1M tokens',
			'owner.provider_cost_effective_from' => ({required Object date}) => 'Effective from ${date}',
			'owner.provider_cost_pick_date' => 'Change date',
			'owner.provider_cost_form_invalid' => 'Fill every field with valid values.',
			'owner.provider_cost_saved_toast' => 'Price saved.',
			'owner.provider_cost_delete_title' => 'Delete price?',
			'owner.provider_cost_delete_body' => ({required Object model}) => 'Remove the ${model} price row? Past profit computations that used it will pick the previous effective price.',
			'owner.exchange_rate_title' => 'USD → BRL rate',
			'owner.exchange_rate_value' => ({required Object rate}) => 'R\$ ${rate} per US\$ 1',
			'owner.exchange_rate_edit_cta' => 'Edit',
			'owner.exchange_rate_label' => 'BRL per 1 USD',
			'owner.exchange_rate_saved' => 'Rate saved.',
			'owner.payouts_title' => 'Seller payouts',
			'owner.payouts_cycle_label' => 'Cycle',
			'owner.payouts_total_due' => 'Total due',
			'owner.payouts_total_paid' => 'Total paid',
			'owner.payouts_empty_title' => 'No payouts in this cycle',
			'owner.payouts_empty_desc' => 'No commission entries were found for this cycle.',
			'owner.payouts_pending_line' => ({required Object amount}) => 'Pending (cycle): ${amount}',
			'owner.payouts_adjustment_line' => ({required Object amount, required Object count}) => 'Adjustments from earlier cycles: ${amount} (${count} entries)',
			'owner.payouts_paid_line' => ({required Object amount}) => 'Paid in this cycle: ${amount}',
			'owner.payouts_total_due_line' => ({required Object amount}) => 'Total due: ${amount}',
			'owner.payouts_pix_label' => 'Copy PIX key',
			'owner.payouts_pix_missing' => 'No PIX key registered',
			'owner.payouts_pix_copied' => 'PIX key copied.',
			'owner.payouts_settled_chip' => 'Settled',
			'owner.payouts_mark_paid_cta' => 'Mark as paid',
			'owner.payouts_mark_paid_title' => 'Confirm payout',
			'owner.payouts_mark_paid_body' => ({required Object amount, required Object name}) => 'Settle ${amount} to ${name}? All pending entries through this cycle become paid.',
			'owner.payouts_note_label' => 'PIX transaction # (optional)',
			'owner.payouts_marked_toast' => 'Payout marked as paid.',
			'owner.payouts_open_cycle_title' => 'Cycle still open',
			'owner.payouts_open_cycle_body' => 'This cycle hasn\'t closed yet — you\'d be settling a partial amount, and any later recompute becomes an adjustment. Pay the open cycle anyway?',
			'owner.payouts_open_cycle_cta' => 'Pay open cycle',
			'owner.payouts_export_cta' => 'Export CSV',
			'owner.payouts_export_copied' => 'CSV copied to clipboard.',
			'owner.sellers_title' => 'Sellers',
			'owner.sellers_subtitle' => 'Platform account users. Turning the seller toggle on flags the user, mints the signup link and joins the lead rotation — all at once.',
			'owner.sellers_empty_title' => 'No platform users',
			'owner.sellers_empty_desc' => 'Create attendants in Settings → Team first, then flag them as sellers here.',
			'owner.sellers_is_seller_label' => 'Is seller',
			'owner.sellers_active_label' => 'Active (commission SLA)',
			'owner.sellers_in_rotation_chip' => 'In rotation',
			'owner.sellers_out_rotation_chip' => 'Out of rotation',
			'owner.sellers_ref_link_label' => 'Copy signup link',
			'owner.sellers_link_copied' => 'Signup link copied.',
			'owner.sellers_pix_missing' => 'No PIX key registered',
			'owner.sellers_carteira_line' => ({required Object count}) => '${count} account(s) in portfolio',
			'owner.sellers_onboarded_toast' => 'Seller onboarded — flag, link and rotation set.',
			'owner.sellers_offboarded_toast' => 'Seller offboarded — removed from rotation; history kept.',
			'owner.sellers_activity_toast' => 'Activity updated.',
			'owner.sellers_admin_warn_title' => 'Commission an admin?',
			'owner.sellers_admin_warn_body' => ({required Object name}) => '${name} is an admin — flagging them as a seller generates commission and SLA duties for them. Continue?',
			'owner.sellers_admin_warn_confirm' => 'Flag as seller',
			'owner.invite_title' => 'Invite customer',
			'owner.invite_email' => 'Customer e-mail',
			'owner.invite_plan' => 'Contracted plan',
			'owner.invite_days' => 'Valid for (days)',
			'owner.invite_create' => 'Generate invite',
			'owner.invite_hint' => 'The account can only be created with this link, by this e-mail, and is born on the chosen plan.',
			'owner.invite_link_title' => 'Invite link — shown once',
			'owner.invite_link_hint' => 'Copy it NOW and send it over the channel where the deal happened. The server keeps only a hash: closing this dialog loses the link (revoke and reissue).',
			'owner.invite_copy' => 'Copy link',
			'owner.invite_copied' => 'Link copied.',
			'sales.nav_tooltip' => 'Active Sales',
			'sales.back_tooltip' => 'Back',
			'sales.funnel_new_title' => 'New funnel',
			'sales.funnel_detail_title' => 'Funnel',
			'sales.products.title' => 'Products',
			'sales.products.subtitle' => 'Manage your catalog products',
			'sales.products.new_product' => 'New product',
			'sales.products.edit_product' => 'Edit product',
			'sales.products.no_products' => 'No products yet',
			'sales.products.empty_subtitle' => 'Add the products you sell so your AI agent knows what it\'s offering.',
			'sales.products.create_first' => 'Create first product',
			'sales.products.load_error' => 'Failed to load products',
			'sales.products.active_label' => 'Active',
			'sales.products.inactive_label' => 'Inactive',
			'sales.products.field_name' => 'Name',
			'sales.products.field_name_hint' => 'e.g. Infusion pump',
			'sales.products.field_description' => 'Description',
			'sales.products.field_description_hint' => 'e.g. Monthly rental with preventive maintenance included (optional)',
			'sales.products.field_price' => 'Price',
			'sales.products.field_cycle' => 'Repurchase cycle (days)',
			'sales.products.field_cycle_hint' => 'e.g. 30 — how often a customer reorders this. Empty means unknown, and it never shows up in reorders',
			'sales.products.cycle_invalid' => 'Use a whole number of days, up to 3650',
			'sales.products.field_currency' => 'Currency',
			'sales.products.field_active' => 'Active',
			'sales.products.field_active_hint' => 'Inactive products are hidden from funnels',
			'sales.products.name_required' => 'Name is required',
			'sales.products.price_required' => 'Enter the price',
			'sales.products.price_invalid' => 'Unreadable price — use 1,234.56',
			'sales.products.save' => 'Save',
			'sales.products.created_success' => 'Product created',
			'sales.products.updated_success' => 'Product updated',
			'sales.products.deleted_success' => 'Product deleted',
			'sales.products.delete_title' => 'Delete product',
			'sales.products.delete_message' => ({required Object name}) => 'Delete "${name}"? This cannot be undone.',
			'sales.products.not_found' => 'Product not found',
			'sales.products.not_found_hint' => 'It may have been deleted, or the address is wrong.',
			'sales.products.import_first' => 'Import spreadsheet or PDF',
			'sales.products.field_price_optional' => 'Leave empty — the rep quotes on the spot',
			'sales.products.sheet_title' => 'From the manufacturer\'s brochure',
			'sales.products.sheet_brand' => 'Brand',
			'sales.products.sheet_model' => 'Model',
			'sales.products.sheet_sku' => 'Code',
			'sales.products.sheet_unit' => 'Unit',
			'sales.products.sheet_specs' => 'Technical sheet',
			'sales.products.sheet_accessories' => 'Accessories',
			'sales.products.sheet_standards' => 'Standards and certifications',
			'sales.products.open_brochure' => ({required Object page}) => 'Open brochure (p. ${page})',
			'sales.products.open_brochure_no_page' => 'Open brochure',
			'sales.products.brochure_missing' => 'This product did not come from an archived brochure',
			'sales.products.sheet_images' => 'Images from the brochure',
			'sales.products.sheet_images_empty' => 'No usable image on this page',
			'sales.products.image_page' => ({required Object page}) => 'p. ${page}',
			'sales.products.image_open' => 'Open image',
			'sales.funnels.title' => 'Funnels',
			'sales.funnels.new_funnel' => 'New funnel',
			'sales.funnels.edit_funnel' => 'Edit funnel',
			'sales.funnels.no_funnels' => 'No funnels yet',
			'sales.funnels.empty_subtitle' => 'Create a funnel to define which data the AI should collect during conversations.',
			'sales.funnels.create_first' => 'Create first funnel',
			'sales.funnels.load_error' => 'Failed to load funnels',
			'sales.funnels.active_label' => 'Active',
			'sales.funnels.inactive_label' => 'Inactive',
			'sales.funnels.fields_count' => ({required Object n}) => '${n} fields',
			'sales.funnels.field_name' => 'Name',
			'sales.funnels.field_name_hint' => 'e.g. Public hospitals in MT',
			'sales.funnels.name_required' => 'Name is required',
			'sales.funnels.save' => 'Save',
			'sales.funnels.created_success' => 'Funnel created',
			'sales.funnels.updated_success' => 'Funnel updated',
			'sales.funnels.deleted_success' => 'Funnel deleted',
			'sales.funnels.delete_title' => 'Delete funnel',
			'sales.funnels.delete_message' => ({required Object name}) => 'Delete "${name}"? This action cannot be undone.',
			'sales.funnels.not_found' => 'Funnel not found',
			'sales.funnels.not_found_hint' => 'It may have been deleted, or the address is wrong.',
			'sales.funnels.default_undeletable' => 'Default funnel — imports and new leads land here; it can\'t be deleted.',
			'sales.schema.section_title' => 'Data to collect',
			'sales.schema.section_subtitle' => 'Define the fields the AI should collect during the conversation.',
			'sales.schema.add_field' => 'Add field',
			'sales.schema.field_key' => 'Key',
			'sales.schema.field_key_hint' => 'e.g. customer_cpf',
			'sales.schema.field_label' => 'Label',
			'sales.schema.field_label_hint' => 'e.g. Customer CPF',
			'sales.schema.field_type' => 'Type',
			'sales.schema.field_required' => 'Required',
			'sales.schema.field_ai_hint' => 'AI hint',
			'sales.schema.field_ai_hint_hint' => 'How/when to collect (optional)',
			'sales.schema.field_enum_options' => 'Options (comma-separated)',
			'sales.schema.field_enum_options_hint' => 'e.g. small, medium, large',
			'sales.schema.remove_field' => 'Remove field',
			'sales.schema.no_fields' => 'No fields defined yet.',
			'sales.schema.error_key_required' => 'Key is required',
			'sales.schema.error_key_format' => 'Use snake_case (lowercase letters, numbers and _)',
			'sales.schema.error_key_duplicate' => 'Duplicate key',
			'sales.schema.error_enum_required' => 'Define at least one option',
			'sales.schema.types.text' => 'Text',
			'sales.schema.types.number' => 'Number',
			'sales.schema.types.cpf' => 'CPF',
			'sales.schema.types.cnpj' => 'CNPJ',
			'sales.schema.types.date' => 'Date',
			'sales.schema.types.bool' => 'Yes/No',
			'sales.schema.types.kEnum' => 'Options',
			'sales.webhook.section_title' => 'Webhook (optional)',
			'sales.webhook.section_subtitle' => 'Only fill this in if you have your own system to receive these events. The funnel works without it.',
			'sales.webhook.field_url' => 'Webhook URL',
			'sales.webhook.field_url_hint' => 'https://your-server.com/webhooks/replider',
			'sales.webhook.url_invalid' => 'Enter a valid http(s) URL',
			'sales.webhook.field_secret' => 'Secret',
			'sales.webhook.field_secret_hint' => 'Used to sign the payload (HMAC)',
			'sales.webhook.generate_secret' => 'Generate',
			'sales.webhook.regenerate_secret' => 'Regenerate',
			'sales.webhook.secret_copied' => 'Secret copied',
			'sales.webhook.trigger_required_complete' => 'Fire when required fields are complete',
			'sales.webhook.trigger_required_complete_hint' => 'Sends a lead.qualified event once all required fields are filled.',
			'sales.webhook.trigger_buy_intent' => 'Fire on buy intent',
			'sales.webhook.trigger_buy_intent_hint' => 'Sends a lead.buy_intent event when the AI detects purchase intent.',
			'sales.webhook.send_test' => 'Send test event',
			'sales.webhook.test_save_first' => 'Save the funnel before sending a test event.',
			'sales.webhook.test_url_first' => 'Set a webhook URL before sending a test event.',
			'sales.webhook.test_success' => ({required Object status}) => 'Test event sent (HTTP ${status})',
			'sales.webhook.test_failed' => 'Test event failed',
			'sales.lead_profile.section_title' => 'Sales profile',
			'sales.lead_profile.empty' => 'This conversation isn\'t part of any funnel.',
			'sales.lead_profile.funnel_label' => 'Funnel',
			'sales.lead_profile.status_label' => 'Lead status',
			'sales.lead_profile.score_label' => 'Score',
			'sales.lead_profile.buy_intent' => 'Buy intent detected',
			'sales.lead_profile.no_value' => 'Not collected yet',
			'sales.lead_profile.pending' => 'Pending',
			'sales.lead_profile.origin_ai' => 'AI',
			'sales.lead_profile.origin_human' => 'Human',
			'sales.lead_profile.load_error' => 'Couldn\'t load the sales profile.',
			'sales.lead_profile.lead_status.kNew' => 'New',
			'sales.lead_profile.lead_status.prospected' => 'Prospected',
			'sales.lead_profile.lead_status.enriching' => 'Enriching',
			'sales.lead_profile.lead_status.enriched' => 'Enriched',
			'sales.lead_profile.lead_status.pending_approval' => 'Pending approval',
			'sales.lead_profile.lead_status.contacted' => 'Contacted',
			'sales.lead_profile.lead_status.responded' => 'Responded',
			'sales.lead_profile.lead_status.qualified' => 'Qualified',
			'sales.lead_profile.lead_status.won' => 'Won',
			'sales.lead_profile.lead_status.lost' => 'Lost',
			'sales.lead_profile.edit_field' => 'Edit field',
			'sales.lead_profile.edit_error' => ({required Object error}) => 'Couldn\'t save: ${error}',
			'sales.lead_profile.pick_date' => 'Pick date',
			'sales.lead_profile.new_proposal' => 'New proposal',
			'sales.leads.status_new' => 'New',
			'sales.leads.status_prospected' => 'Prospected',
			'sales.leads.status_enriching' => 'Enriching',
			'sales.leads.status_enriched' => 'Enriched',
			'sales.leads.status_pending_approval' => 'Pending approval',
			'sales.leads.status_contacted' => 'Contacted',
			'sales.leads.status_responded' => 'Responded',
			'sales.leads.status_qualified' => 'Qualified',
			'sales.leads.status_won' => 'Won',
			'sales.leads.status_lost' => 'Lost',
			'sales.leads.discard' => 'Discard',
			'sales.leads.template_label' => 'WhatsApp template',
			'sales.leads.template_select' => 'Choose a template',
			'sales.leads.template_required' => 'Select an approved template before sending',
			'sales.leads.template_none' => 'No approved templates on this channel yet',
			'sales.leads.template_no_channel' => 'Link a WhatsApp channel to this funnel first',
			'sales.leads.template_load_error' => 'Couldn\'t load templates',
			'sales.prospecting.intro' => 'Search Google Places for businesses and import them as leads in a funnel.',
			'sales.prospecting.funnel_label' => 'Target funnel',
			'sales.prospecting.keyword_label' => 'Keyword',
			'sales.prospecting.keyword_hint' => 'e.g. hospital, imaging clinic, ICU',
			'sales.prospecting.location_label' => 'Location',
			'sales.prospecting.location_hint' => 'e.g. Cuiabá, MT',
			'sales.prospecting.radius_label' => 'Radius (meters, optional)',
			'sales.prospecting.radius_hint' => 'e.g. 5000',
			'sales.prospecting.search_import' => 'Search & import',
			'sales.prospecting.query_required' => 'Enter a keyword or location and pick a funnel',
			'sales.prospecting.no_funnels' => 'Create a funnel first to import prospected leads into it.',
			'sales.prospecting.result_title' => 'Search complete',
			'sales.prospecting.result_created' => ({required Object n, required Object total}) => '${n} of ${total} results imported as leads.',
			'sales.prospecting.result_skipped' => ({required Object n}) => '${n} skipped (already in this funnel).',
			'sales.prospecting.view_pipeline' => 'View pipeline',
			'sales.prospecting.preview_found' => ({required Object total, required Object duplicates}) => '${total} found · ${duplicates} already in your base',
			'sales.prospecting.quota_left' => ({required Object count}) => '${count} leads left this month',
			'sales.prospecting.already_lead' => 'already in your base',
			'sales.prospecting.import_n' => ({required Object count}) => 'Import ${count} leads',
			'sales.prospecting.imported' => ({required Object count}) => '${count} leads imported',
			'sales.prospecting.select_all' => 'Select all',
			'sales.prospecting.select_none' => 'Clear selection',
			'sales.prospecting.selected_of' => ({required Object n, required Object total}) => '${n} of ${total} selected',
			'sales.prospecting.imported_partial' => ({required Object created, required Object selected}) => '${created} of ${selected} imported — your plan\'s monthly lead allowance stopped the rest.',
			'sales.prospecting.no_results_title' => 'No companies found',
			'sales.prospecting.no_results_hint' => 'Try another keyword or a wider radius — \'clínica\', \'hospital\', a neighbouring city.',
			'sales.hunt_tab' => 'Hunt',
			'support.title' => 'Support',
			'support.subtitle' => 'Your Replider account manager',
			'support.nav_tooltip' => 'Support',
			'support.empty_title' => 'Talk to us',
			'support.empty_subtitle' => 'Questions about your account, channels or billing? Send a message — our team replies right here.',
			'support.input_hint' => 'Type a message...',
			'support.load_error' => 'Could not load your support conversation.',
			'support.retry' => 'Try again',
			'support.push_prompt_title' => 'Get notified about replies',
			'support.push_prompt_body' => 'Enable notifications and we\'ll ping you when your account manager answers, even with the dashboard closed.',
			'support.push_prompt_enable' => 'Enable notifications',
			'support.push_prompt_later' => 'Not now',
			'claude_integration.title' => 'Claude (AI)',
			'claude_integration.back_tooltip' => 'Back',
			'claude_integration.load_failed' => ({required Object error}) => 'Failed to load: ${error}',
			'claude_integration.copy_tooltip' => 'Copy',
			'claude_integration.copied' => 'Copied.',
			'claude_integration.connect_title' => 'Connect to Claude',
			'claude_integration.connect_subtitle' => 'Manage your account by chatting with Claude — configure the agent, reply to customers, view metrics. Works on claude.ai and the mobile/desktop apps.',
			'claude_integration.tokens_title' => 'Access tokens',
			'claude_integration.tokens_subtitle' => 'Personal tokens for Claude Code. Replace SEU_TOKEN in the command above. The full token is shown only once.',
			'claude_integration.tokens_create' => 'Create token',
			'claude_integration.tokens_empty' => 'No tokens created.',
			'claude_integration.tokens_never_used' => 'never used',
			'claude_integration.tokens_last_used' => ({required Object date}) => 'used on ${date}',
			'claude_integration.tokens_revoke_tooltip' => 'Revoke token',
			'claude_integration.create_dialog_title' => 'New access token',
			'claude_integration.create_dialog_name_label' => 'Name (e.g. Tiago\'s Claude Code)',
			'claude_integration.create_dialog_confirm' => 'Create',
			'claude_integration.token_dialog_title' => 'Copy the token now',
			'claude_integration.token_dialog_body' => 'This is the only time the token will be shown. Copy it into Claude — it cannot be recovered later.',
			'claude_integration.token_dialog_copy' => 'Copy',
			'claude_integration.token_copied' => 'Token copied.',
			'claude_integration.token_dialog_done' => 'Done',
			'claude_integration.revoke_token_title' => 'Revoke token?',
			'claude_integration.revoke_token_body' => 'Claude connected with this token will lose access immediately.',
			'claude_integration.revoke_confirm' => 'Revoke',
			'claude_integration.connections_title' => 'Active connections',
			'claude_integration.connections_subtitle' => 'Apps authorized via OAuth (e.g. the claude.ai connector).',
			'claude_integration.connections_empty' => 'No active connections.',
			'claude_integration.connections_revoke' => 'Revoke',
			'claude_integration.revoke_connection_title' => 'Revoke connection?',
			'claude_integration.revoke_connection_body' => 'This app will lose access to your Replider account immediately.',
			'claude_integration.connect_step1' => 'Copy the address below.',
			'claude_integration.connect_step2' => 'In Claude, go to Settings → Connectors → Add custom connector and paste the address.',
			'claude_integration.connect_step3' => 'Click connect and sign in with the same email and password you use here on Replider.',
			'claude_integration.connect_no_token_note' => 'No token or install needed — just sign in with your email and password. (Custom connectors require a Claude Pro, Max, Team or Enterprise plan.)',
			'claude_integration.advanced_title' => 'Advanced — Claude Code',
			'claude_integration.advanced_subtitle' => 'For people using Claude Code in the terminal. If you use Claude on the web or apps, skip this section.',
			'claude_integration.cc_oauth_label' => 'Recommended — browser login (no token)',
			'claude_integration.cc_oauth_hint' => 'Run the command in the terminal. Claude Code opens the browser, you sign in with your Replider email and password, and it returns to the terminal automatically.',
			'claude_integration.cc_token_label' => 'Alternative — with token (for server/CI, no browser)',
			'claude_integration.cc_token_hint' => 'Use when no browser is available. Create a token below and replace SEU_TOKEN in the command.',
			'shell.start' => 'Home',
			'shell.funnel' => 'Customers',
			'shell.conversations' => 'Conversations',
			'shell.offers' => 'Opportunities',
			'shell.copilot' => 'Copilot',
			'shell.copilot_soon_title' => 'Copilot coming soon',
			'shell.copilot_soon_body' => 'For now, talk to Replider through the MCP connector in Claude or ChatGPT — the same actions live there.',
			'shell.start_title' => 'Your day',
			'shell.start_subtitle' => 'Shortcuts to where the work happens.',
			'shell.start_go_funnel' => 'Open the pipeline',
			'shell.start_go_conversations' => 'Open conversations',
			'shell.start_go_prospecting' => 'Find companies',
			'shell.stat_active_leads' => 'Active leads',
			'shell.stat_pending_approval' => 'Waiting approval',
			'shell.stat_unowned' => 'Unassigned',
			'shell.needs_you_title' => 'NEEDS YOU',
			'shell.needs_you_clear' => 'Nothing is waiting on you right now.',
			'shell.needs_open' => 'Open',
			'shell.needs_approvals' => ({required Object count}) => '${count} approach(es) waiting on your approval',
			'shell.needs_unowned' => ({required Object count}) => '${count} lead(s) with no owner — nobody is working them',
			'shell.needs_empty_pipeline' => 'Your pipeline is empty. Start by hunting for clients.',
			'board.move_to' => 'Move to',
			'board.score' => 'Score',
			'board.unassigned' => 'Unassigned',
			'board.empty_column' => 'No leads here',
			'board.empty_board' => 'This funnel has no stages yet.',
			'board.select_funnel' => 'Pick a funnel',
			'board.board_title' => 'Pipeline',
			'board.new_lead' => 'New lead',
			'board.new_lead_name' => 'Contact',
			'board.new_lead_name_hint' => 'Who decides or buys',
			'board.new_lead_phone' => 'Phone / WhatsApp',
			'board.new_lead_phone_hint' => '+55 65 99999-0000',
			'board.new_lead_created' => 'Lead added to the funnel.',
			'board.new_lead_existed' => 'That contact is already in the funnel.',
			'board.no_funnel_yet' => 'You don\'t have a funnel yet.',
			'board.search_hint' => 'Search company, city or contact',
			'board.search_clear' => 'Clear search',
			'board.filter_all' => ({required Object count}) => 'All · ${count}',
			'board.filter_responded' => ({required Object count}) => 'Replied · ${count}',
			'board.filter_awaiting' => ({required Object count}) => 'Awaiting approval · ${count}',
			'board.filter_stalled' => ({required Object count}) => 'Idle for over 7 days · ${count}',
			'board.column_showing' => ({required Object showing, required Object total}) => '${showing} of ${total} · longest idle first',
			'board.column_showing_manual' => ({required Object showing, required Object total}) => '${showing} of ${total} · manual order',
			'board.order_reset' => 'Sort by idle',
			'board.column_no_match' => ({required Object query}) => 'Nothing matching “${query}” in this stage.',
			'board.column_no_match_hint' => 'It may be in another stage, or not imported yet.',
			'board.truncated_notice' => ({required Object count}) => 'Showing this funnel\'s ${count} most recent leads. Search covers only those.',
			'board.idle_days' => ({required Object count}) => '${count} d idle',
			'board.idle_today' => 'Moved today',
			'board.idle_unknown' => 'No recorded activity',
			'board.view_pipeline' => 'Pipeline',
			'board.tag_draft_ready' => 'Draft in Today →',
			'board.tag_buy_intent' => 'Wants to buy',
			'board.tag_qualified' => 'Qualified',
			'board.tag_idle_risk' => 'Going cold',
			'board.field_required' => 'Required',
			'board.funnel_fields' => 'Funnel fields',
			'board.funnel_fields_saved' => 'Field saved',
			'board.funnel_fields_empty' => 'This funnel asks for no extra fields.',
			'board.owner' => 'Owner',
			'board.owner_none' => 'Nobody yet',
			'board.owner_assign' => 'Set owner',
			'board.visibility' => 'Visibility',
			'board.visibility_team' => 'Whole team',
			_ => null,
		} ?? switch (path) {
			'board.visibility_private' => 'Owner only',
			'board.visibility_private_hint' => 'Disappears from the pipeline of anyone who is not the owner.',
			'board.sources' => 'Where this came from',
			'board.sources_hint' => 'The AI built this profile from these pages.',
			'board.bulk_assign' => 'Assign the ownerless',
			'board.bulk_assign_title' => 'Assign leads with no owner',
			'board.bulk_assign_body' => ({required Object count}) => '${count} leads in this funnel have no owner. Hand them all to:',
			'board.bulk_assign_filtered' => 'Counts only what the current search and filter show.',
			'board.bulk_assign_confirm' => ({required Object count}) => 'Assign ${count}',
			'board.bulk_assign_done' => ({required Object count}) => '${count} leads assigned',
			'board.bulk_assign_partial' => ({required Object count, required Object skipped}) => '${count} assigned · ${skipped} were no longer there',
			'board.bulk_assign_cap' => '200 at a time, max. The first 200 will be assigned.',
			'board.bulk_assign_here' => 'Assign in this search',
			'board.new_lead_subtitle' => 'The company, and the person you talk to there.',
			'board.new_lead_company' => 'Company',
			'board.new_lead_company_hint' => 'e.g. Cuiabá Regional Hospital',
			'board.new_lead_company_required' => 'Enter the company.',
			'board.new_lead_role' => 'Role',
			'board.new_lead_role_hint' => 'e.g. Purchasing manager',
			'board.new_lead_email' => 'Email',
			'board.new_lead_email_hint' => 'purchasing@hospital.com',
			'board.new_lead_city' => 'City',
			'board.new_lead_city_hint' => 'e.g. Cuiabá, MT',
			'board.new_lead_reach_help' => 'Phone or email — at least one, so the offer has a way out.',
			'board.new_lead_reach_required' => 'Enter a phone or an email.',
			'board.new_lead_submit' => 'Add',
			'board.configure_stages' => 'Configure stages',
			'board.tag_responded' => 'Replied',
			'board.column_hint' => ({required Object status}) => 'Leads with status "${status}". They move on send and on reply — or drag them.',
			'board.column_hint_terminal' => ({required Object status}) => 'Final stage — leads with status "${status}".',
			'board.panel_open_in_queue' => 'Open draft in Today',
			'board.panel_cnes_title' => 'CNES installed base',
			'board.panel_cnes_by_phone' => 'public data · matched by phone · counts, not age',
			'board.panel_cnes_by_name' => 'public data · unique name match · counts, not age',
			'board.panel_cnes_month' => ({required Object month}) => 'CNES data from ${month}',
			'board.panel_cnes_code' => ({required Object code}) => 'CNES code ${code}',
			'board.panel_offer_title' => 'What you can offer',
			'board.panel_offer_line' => ({required Object product, required Object count, required Object unit, required Object equipment}) => '${product} — they have ${count} ${unit} of ${equipment}',
			'board.panel_offer_none' => 'No product in your catalogue matches the declared park',
			'board.panel_offer_free' => 'your catalogue crossed with the registry — no AI',
			'board.panel_offer_tail' => ({required Object count, required Object unit, required Object equipment}) => ' — they have ${count} ${unit} of ${equipment}',
			'board.panel_offer_open' => ({required Object product}) => 'Open ${product}',
			'lead_detail.profile' => 'Profile',
			'lead_detail.approach' => 'Approach',
			'lead_detail.approach_hint' => 'Write or generate the approach message…',
			'lead_detail.regenerate' => 'Generate another',
			'lead_detail.generate' => 'Generate approach',
			'lead_detail.approve_send' => 'Approve and send',
			'lead_detail.timeline' => 'Timeline',
			'lead_detail.confirm_title' => 'Confirm send',
			'lead_detail.confirm_to' => 'To',
			'lead_detail.sent' => 'Approach sent.',
			'lead_detail.read_only' => 'You can view this lead, but it belongs to another rep.',
			'lead_detail.queue_title' => 'Approval queue',
			'lead_detail.queue_empty' => 'Nothing to approve right now.',
			'lead_detail.queue_empty_hint' => 'When the AI drafts approaches, they wait for your approval here.',
			'lead_detail.queue_remaining' => ({required Object count}) => '${count} remaining',
			'lead_detail.discard' => 'Discard',
			'lead_detail.edit' => 'Edit',
			'lead_detail.discarded' => 'Draft discarded — the lead goes back to the pipeline.',
			'lead_detail.undo' => 'Undo',
			'lead_detail.send_channel' => 'Goes out via',
			'lead_detail.bulk_needs_template' => 'WhatsApp needs an approved template — open the lead',
			'lead_detail.check_title' => 'Automatic check',
			'lead_detail.check_clean' => 'No pattern promising a deadline, a price or a result was found in this text.',
			'lead_detail.check_found' => ({required Object count}) => '${count} passage(s) to look at before sending.',
			'lead_detail.check_edited' => 'You edited the text. The check below is of the saved draft — this text has not been checked.',
			'lead_detail.check_kind_promise' => 'A commitment made in your name',
			'lead_detail.check_kind_invented_fact' => 'A claim about the customer with nothing behind it',
			'lead_detail.check_kind_out_of_scope' => 'Outside what the company said it does',
			'lead_detail.check_kind_placeholder' => 'A fill-in field left behind',
			'lead_detail.check_kind_tone' => 'Tone that does not fit the context',
			'lead_detail.review_action' => 'Review with AI',
			'lead_detail.review_running' => 'Reading the message…',
			'lead_detail.review_clean' => 'Reviewed by a second AI: nothing to flag.',
			'lead_detail.review_found' => ({required Object count}) => 'The second AI flagged ${count} point(s).',
			'lead_detail.review_unavailable' => 'The AI review did not run. The automatic check above still stands.',
			'lead_detail.written_by_ai' => ({required Object source}) => 'Written by AI from: ${source}',
			'lead_detail.written_by_ai_unknown' => 'Written by AI',
			'lead_detail.shortcut_approve' => 'approve',
			'lead_detail.shortcut_edit' => 'edit',
			'lead_detail.shortcut_discard' => 'discard',
			'lead_detail.shortcut_move' => 'move',
			'lead_detail.contact' => 'Contact',
			'lead_detail.call' => 'Call',
			'lead_detail.open_conversation' => 'Open conversation',
			'lead_detail.no_conversation' => 'No conversation with this contact yet.',
			'lead_detail.enrich' => 'Research this company',
			'lead_detail.enrich_again' => 'Research again',
			'lead_detail.enriching' => 'Researching…',
			'lead_detail.enrich_started' => 'Researching the company. Check back shortly.',
			'lead_detail.profile_empty' => 'We know nothing about this company yet.',
			'lead_detail.profile_empty_hint' => 'Research looks up the decision maker, the line of business and the address, so the approach can cite something real instead of reading as a template.',
			'lead_detail.profile_researching' => 'Researching this company now. The result shows up here.',
			'lead_detail.profile_found_nothing' => 'The research found no public data about this company.',
			'lead_detail.profile_found_nothing_hint' => 'That happens with small companies and ones with no online presence. Write the approach with what you already know, or research again later.',
			'lead_detail.profile_blocked' => 'Nothing to research: this lead has no company name and no website.',
			'lead_detail.profile_blocked_hint' => 'The search starts from the name or the website to find the decision maker. With neither, there is nothing to look for.',
			'lead_detail.summary' => 'Conversation summary',
			'lead_detail.summary_generating' => 'Reading the conversation…',
			'lead_detail.summary_short' => 'Too short to summarise.',
			'lead_detail.summary_failed' => 'Could not summarise right now.',
			'lead_detail.website' => 'Website',
			'lead_detail.check_kind_missing_attachment' => 'The text says a file is attached, and none is',
			'lead_detail.blocked_placeholder' => 'Fill in the field before sending.',
			'lead_detail.subject' => 'Subject',
			'lead_detail.subject_hint' => 'What the recipient sees before opening',
			'lead_detail.use_template' => 'Use a template',
			'lead_detail.reply_rate_short' => 'reply',
			'lead_detail.bulk_selected' => ({required Object count}) => '${count} selected',
			'lead_detail.bulk_send' => 'Send selected',
			'lead_detail.bulk_confirm_title' => ({required Object count}) => 'Send ${count} approaches?',
			'lead_detail.bulk_review_title' => ({required Object count}) => 'Review ${count} approaches',
			'lead_detail.bulk_review_hint' => 'Untick whatever should not go. Each text is sent exactly as it reads here.',
			'lead_detail.bulk_review_send' => ({required Object count}) => 'Send ${count}',
			'lead_detail.bulk_review_none' => 'Nothing ticked',
			'lead_detail.bulk_sent' => ({required Object count}) => '${count} approaches sent',
			'lead_detail.bulk_partial' => ({required Object count, required Object failed}) => '${count} sent. Failed: ${failed}',
			'lead_detail.profile_facts_count' => ({required Object count}) => '${count} fact(s) about the company',
			'lead_detail.profile_nothing_yet' => 'Nothing researched yet',
			'lead_detail.timeline_events' => ({required Object count}) => '${count} event(s)',
			'lead_detail.section_expand' => 'Open',
			'lead_detail.section_collapse' => 'Close',
			'lead_detail.queue_filter_all' => 'All',
			'lead_detail.queue_no_funnel' => 'No funnel',
			'lead_detail.queue_sort_oldest' => 'Oldest first',
			'lead_detail.queue_sort_newest' => 'Newest first',
			'lead_detail.queue_flagged_only' => 'Flagged only',
			'lead_detail.queue_partial' => ({required Object shown, required Object total}) => 'Showing ${shown} of ${total}. Pick a funnel to work through the rest.',
			'lead_detail.queue_filtered_empty' => 'Nothing in this selection.',
			'lead_detail.queue_filtered_empty_hint' => 'The other drafts are still queued — clear the filter to see them.',
			'lead_detail.queue_waiting_minutes' => ({required Object count}) => 'queued ${count} min',
			'lead_detail.queue_waiting_hours' => ({required Object count}) => 'queued ${count}h',
			'lead_detail.queue_waiting_days' => ({required Object count}) => 'queued ${count}d',
			'lead_detail.queue_empty_cta' => 'Find companies',
			'lead_detail.email_warmup_banner_named' => ({required Object mailbox, required Object day, required Object total, required Object remaining}) => 'Mailbox ${mailbox} warming up (day ${day} of ${total}): ${remaining} cold sends left today. The cap grows daily to protect your domain.',
			'lead_detail.email_warmup_exhausted' => ({required Object mailbox, required Object day, required Object total}) => 'Mailbox ${mailbox} used up today\'s cold sends (day ${day} of ${total} of the warm-up). New approvals go out tomorrow.',
			'lead_detail.field_company' => 'Company',
			'lead_detail.field_website' => 'Website',
			'lead_detail.field_decision_maker' => 'Decision maker',
			'lead_detail.field_role' => 'Role',
			'lead_detail.field_address' => 'Address',
			'lead_detail.field_activity' => 'Activity',
			'lead_detail.bulk_subject_hint' => 'Subject for this batch\'s emails',
			'lead_detail.bulk_subject_missing' => 'Add a subject to send',
			'lead_detail.error_action_configure' => 'Configure',
			'lead_detail.approve_needs_queue_status' => 'This lead isn\'t awaiting approval — generate a draft to put it back in the queue.',
			'lead_detail.approach_locked_after_send' => 'This approach was already sent — the text is locked so history stays true.',
			'lead_detail.research_still_running' => 'The research is taking a while — it keeps running in the background; check back later.',
			'lead_detail.timeline_created' => ({required Object date}) => 'Created on ${date}',
			'lead_detail.timeline_qualified' => ({required Object date}) => 'Qualified on ${date}',
			'lead_detail.timeline_buy_intent' => ({required Object date}) => 'Signalled buying intent on ${date}',
			'lead_detail.timeline_researched' => ({required Object date}) => 'Researched on ${date}',
			'lead_detail.timeline_updated' => ({required Object date}) => 'Last updated on ${date}',
			'lead_detail.queue_group_carteira' => 'From your book',
			'lead_detail.queue_group_sinal' => 'From a radar signal',
			'lead_detail.queue_group_frio' => 'Cold, from the territory',
			'lead_detail.queue_frio_discard_all' => ({required Object count}) => 'Discard all ${count}',
			'lead_detail.queue_frio_redraft_all' => 'Rewrite with the catalog',
			'lead_detail.queue_frio_show_all' => ({required Object count}) => '+ ${count} more like these · show all',
			'lead_detail.queue_batch_discard_title' => 'Discard cold drafts',
			'lead_detail.queue_batch_discard_message' => ({required Object count}) => 'Discard ${count} queued drafts? The leads move to lost and nothing is sent.',
			'lead_detail.queue_batch_redraft_title' => 'Rewrite with the catalog',
			'lead_detail.queue_batch_redraft_message' => ({required Object count}) => 'Rewrite ${count} drafts using your product catalog? Each rewrite is an AI call billed to your key.',
			'lead_detail.queue_batch_discarded' => ({required Object count}) => '${count} drafts discarded.',
			'lead_detail.queue_batch_redrafted' => ({required Object count}) => '${count} drafts rewritten.',
			'lead_detail.queue_outcome_line' => ({required Object sent, required Object replies, required Object proposals}) => 'This week: ${sent} sent · ${replies} replies · ${proposals} proposals',
			'lead_detail.approve_scheduled' => ({required Object when}) => 'Today\'s limit is spent — send scheduled for ${when}.',
			'lead_detail.queue_scheduled_badge' => ({required Object when}) => 'Scheduled · goes out ${when}',
			'lead_detail.route_strip_title' => ({required Object n, required Object cap}) => 'Send ${n} of ${cap} today',
			'lead_detail.route_strip_exhausted' => 'Today\'s limit is spent — approvals now schedule for tomorrow morning',
			'lead_detail.route_strip_sub' => ({required Object mailbox, required Object day, required Object total}) => '${mailbox} warming up, day ${day} of ${total} — approving past the limit schedules by itself.',
			'lead_detail.queue_blocked_title' => 'Nothing can go out yet',
			'lead_detail.queue_blocked_body' => 'The offers are ready, but this account has no channel connected.',
			'lead_detail.queue_blocked_cta' => 'Connect a channel',
			'lead_detail.queue_tender_deadline_days' => ({required Object count}) => 'tender closes in ${count}d',
			'lead_detail.queue_tender_deadline_today' => 'tender closes today',
			'lead_detail.queue_tender_deadline_past' => 'tender closed',
			'lead_detail.queue_tender_value' => ({required Object value}) => 'estimated • ${value}',
			'lead_detail.queue_no_address_title' => 'No address to send to',
			'lead_detail.queue_no_address_body' => 'None of your channels can reach this contact. Find the buyer’s e-mail and fill the customer in — the offer is already written.',
			'lead_detail.queue_no_address_cta' => 'Open customer',
			'lead_detail.queue_wa_send' => 'Send from my WhatsApp',
			'lead_detail.queue_wa_hint' => 'opens on your number, text ready',
			'lead_detail.queue_wa_confirm_title' => 'Did you send it?',
			'lead_detail.queue_wa_confirm_body' => 'Nothing left from here — you sent it yourself. If you did, I will take this offer out of the queue and mark the customer as contacted.',
			'lead_detail.queue_wa_confirm_yes' => 'I sent it',
			'lead_detail.queue_wa_confirm_no' => 'Not yet',
			'account_menu.profile' => 'Profile',
			'account_menu.team' => 'Team',
			'account_menu.channels' => 'Channels',
			'account_menu.settings' => 'Settings',
			'account_menu.logout' => 'Log out',
			'account_menu.title' => 'Account',
			'copilot.empty_title' => 'What do you want to do?',
			'copilot.empty_body' => 'Ask in plain words. I do it and hand back a link. I never send anything without your approval.',
			'copilot.hint' => 'ask for something…',
			'copilot.thinking' => 'working…',
			'copilot.clear' => 'Clear conversation',
			'copilot.sample_prospect' => 'Find 20 [segment] companies in [city]',
			'copilot.sample_drafts' => 'Draft approaches for the new leads in the funnel',
			'copilot.sample_report' => 'How did my prospecting go in the last 30 days?',
			'copilot.failed' => 'I could not answer. Try again.',
			'copilot.tool_list_funnels' => 'looked up your funnels',
			'copilot.tool_pipeline_overview' => 'read the pipeline numbers',
			'copilot.tool_list_approval_queue' => 'read the approval queue',
			'copilot.tool_draft_approaches' => 'drafted approaches',
			'copilot.tool_case_report' => 'read your period report',
			'copilot.thinking_long' => 'Still working. Searches and drafts can take up to 2 minutes.',
			'copilot.sample_prospect_audience' => ({required Object keyword, required Object location}) => 'Find 20 ${keyword} in ${location}',
			'attachment.count' => ({required Object count}) => '${count} attachment(s)',
			'attachment.open' => 'Open',
			'attachment.download_failed' => 'Could not open the file right now.',
			'attachment.status_clean' => 'Scanned, nothing found',
			'attachment.status_pending' => 'Not scanned yet',
			'attachment.status_skipped' => 'Not scanned',
			'attachment.status_failed' => 'The scan did not finish',
			'attachment.status_infected' => 'Threat found',
			'attachment.blocked_title' => 'This file has not been released',
			'attachment.blocked_pending' => 'The scanner has not finished. Try again in a few minutes.',
			'attachment.blocked_skipped' => 'No scanner is configured on this installation, so nobody checked this file. It came from outside — open at your own risk.',
			'attachment.blocked_failed' => 'The scan failed. The system retries automatically; if it persists, contact support.',
			'attachment.blocked_infected' => ({required Object signature}) => 'The scanner found a threat (${signature}). This file cannot be opened. If you believe this is wrong, ask for the file another way.',
			'attachment.download_anyway' => 'Download anyway',
			'attachment.download_anyway_confirm' => 'Nobody checked this file. It was sent by someone outside your company. Open it?',
			'attachment.why_blocked' => 'Why?',
			'attachment.view_original' => 'View original',
			'attachment.view_original_images' => 'View original with images',
			'attachment.view_original_images_warning' => 'Loading images tells the sender you opened the message. Continue?',
			'sender_auth.badge_unknown' => 'Sender not verified',
			'sender_auth.badge_failed' => 'Suspicious sender',
			'sender_auth.title_unknown' => 'Nobody verified this sender',
			'sender_auth.title_failed' => 'This sender failed verification',
			'sender_auth.body_unknown' => 'The server that received this message did not report whether the domain authorised the send. This is common with older providers and does not mean it is fake — it means we cannot say it is genuine.',
			'sender_auth.body_failed' => 'The domain in the sender address did NOT authorise this send. Someone may be impersonating that company. Before sharing data, paying an invoice or changing bank details, confirm by phone using a number you already had.',
			'sender_auth.checks' => ({required Object checks}) => 'Checks: ${checks}',
			'email_inbox.no_subject' => '(no subject)',
			'email_inbox.empty' => 'No emails in this thread yet.',
			'email_inbox.you' => 'You',
			'email_inbox.back_to_list' => 'Back to emails',
			'email_inbox.loading_email' => 'Loading email…',
			'email_inbox.view_text' => 'Text',
			'email_inbox.view_original' => 'Original',
			'email_risk.payment_title' => 'Careful: the payment details changed',
			'email_risk.payment_action' => 'Confirm by phone before paying, using a number you already had — not the one in this email.',
			'email_risk.quarantine_title' => 'Message held for review',
			'email_risk.attention_title' => 'Worth a look before you act',
			'email_risk.legit' => 'This is legitimate',
			'email_risk.legit_confirm' => 'Confirm this sender is trustworthy? We will stop warning about them on this account.',
			'email_risk.legit_done' => 'Done. We will not warn about this sender again.',
			'email_risk.legit_failed' => 'Could not record that right now.',
			'email_risk.scam' => 'This is a scam',
			'email_risk.scam_title' => 'Report as a scam',
			'email_risk.scam_why' => 'Why do you think it is a scam? (optional)',
			'email_risk.scam_why_hint' => 'e.g. this supplier never sends invoices by email',
			'email_risk.scam_explain' => 'Your report protects the other companies on the network. When two companies report the same sender, it is blocked for everyone.',
			'email_risk.scam_done' => 'Reported. Thank you — this protects the other companies on the network.',
			'brand.title' => 'Company identity',
			'brand.subtitle' => 'What the AI knows about you, and what it may never say. Applies to every approach, from every seller.',
			'brand.identity_section' => 'Company identity',
			'brand.company_name' => 'Company name',
			'brand.about' => 'What you do',
			'brand.about_hint' => 'e.g. we distribute multiparameter monitors and infusion pumps to clinics in the interior of SP',
			'brand.tone' => 'How you speak',
			'brand.tone_hint' => 'e.g. direct, no jargon, informal',
			'brand.signature' => 'Default signature',
			'brand.allowed' => 'What we may claim',
			'brand.allowed_hint' => 'True things that help you sell. e.g. our own service team in Sorocaba',
			'brand.forbidden' => 'Never say',
			'brand.forbidden_hint' => 'The AI is blocked from writing these, and the text is checked before it reaches the queue. e.g. cures, approved by ANVISA',
			'brand.add' => 'Add',
			'brand.remove' => 'Remove',
			'brand.saved' => 'Identity saved.',
			'brand.personal' => 'Your own way',
			'brand.personal_subtitle' => 'Adds to the company\'s, never replaces it. You cannot unlock what the company forbade.',
			'brand.tone_note' => 'Your personal tone',
			'brand.tone_note_hint' => 'e.g. I speak slowly, I always ask about the team before price',
			'brand.personal_signature' => 'Your signature',
			'brand.personal_signature_hint' => 'e.g. João Silva · (15) 99999-0000',
			'brand.personal_saved' => 'Your own way was saved.',
			'brand.differentiators' => 'Your differentiators',
			'brand.differentiators_hint' => 'Why yours rather than the one the hospital already owns. Written by you — the AI uses what fits and invents none. Empty, the proposal argues from the product alone.',
			'suppressions.title' => 'Blocked addresses',
			'suppressions.subtitle' => 'Addresses that no longer receive email from you: they bounced permanently or reported you as spam. Writing to them again is what costs a sending domain its reputation.',
			'suppressions.empty' => 'No blocked addresses',
			'suppressions.empty_hint' => 'Anything that bounces permanently will show up here, with the reason the receiving server gave.',
			'suppressions.release' => 'Release',
			'suppressions.release_title' => 'Release this address?',
			'suppressions.release_warning' => 'A mail server refused this address, or the recipient reported you as spam. Only release it if you know the problem was fixed.',
			'suppressions.released' => 'Address released',
			'suppressions.reason_hard_bounce' => 'Permanent bounce — the address does not exist',
			'suppressions.reason_complaint' => 'Reported as spam by the recipient',
			'signals.title' => 'Buying signals',
			'signals.subtitle' => 'Public tenders matching what you sell. Somebody wrote down what they intend to buy and by when — closest deadline first.',
			'signals.empty_no_watch' => 'No monitoring set up',
			'signals.empty_no_watch_hint' => 'Tell us which words to watch for and open tenders will show up here on their own.',
			'signals.empty_quiet' => 'Nothing open right now',
			'signals.empty_quiet_hint' => 'We check public tenders several times a day. New ones appear here as they are published.',
			'signals.days_left' => ({required Object days}) => '${days} days left',
			'signals.closes_today' => 'closes today',
			'signals.value_undisclosed' => 'value not disclosed',
			'signals.open_notice' => 'Read the notice',
			'signals.dismiss' => 'Not for us',
			'signals.tab' => 'Signals',
			'signals.watch_cta' => 'Choose keywords',
			'signals.watch_dialog_title' => 'What you want to follow',
			'signals.watch_dialog_body' => 'Words that show up in what you sell. Separate with commas — we look for each one in published notices.',
			'signals.watch_hint' => 'e.g. hospital supplies, syringe, exam gloves',
			'signals.watch_states_label' => 'Where you deliver',
			'signals.watch_states_hint' => 'e.g. MT, MS',
			'signals.watch_states_error' => 'Name at least one state (UF).',
			'signals.watch_states_body' => 'We only read tenders from the states you serve — without them the search sweeps the whole country and returns what you cannot deliver.',
			'signals.watch_saved' => 'Done. Matching notices will show up here on their own.',
			'signals.watch_empty_error' => 'Write at least one word.',
			'signals.watch_current' => ({required Object words}) => 'Following: ${words}',
			'signals.no_catalog_banner' => 'No products registered — these matches come from hand-typed words, not from what you sell. Import your catalog and the watch builds itself from it.',
			'signals.no_catalog_cta' => 'Add products',
			'signals.watch_origin_catalog' => 'from your catalog',
			'signals.watch_origin_manual' => 'set manually',
			'sales_record.title' => 'Record sale',
			'sales_record.gross' => 'Total sold',
			'sales_record.gross_hint' => 'what the customer pays',
			'sales_record.cost' => 'Cost of goods',
			'sales_record.cost_hint' => 'what it cost you — commission is paid on the margin, not on the invoice',
			'sales_record.tax' => 'Tax',
			'sales_record.tax_hint' => 'how much tax this sale pays, in currency',
			'sales_record.cycle' => 'Repurchase cycle (days)',
			'sales_record.cycle_hint' => 'how often this customer buys again — leave empty if you do not know',
			'sales_record.notes' => 'Notes',
			'sales_record.net' => 'You keep',
			'sales_record.underwater_warning' => 'This sale costs more than it brings in. Recording it anyway is fine — just check the numbers.',
			'sales_record.submit' => 'Record sale',
			'sales_record.saved' => 'Sale recorded',
			'sales_record.your_commission' => 'Your commission on this sale',
			'portfolio.title' => 'Portfolio',
			'portfolio.subtitle' => 'Your customers — imported or won. Ordered by who needs to hear from you first, not alphabetically.',
			'portfolio.empty' => 'No customers yet',
			'portfolio.empty_hint' => 'Import your spreadsheet, or record a sale on a lead, and they show up here — with a prediction of when they will need it again.',
			'portfolio.purchases' => ({required Object count}) => '${count} purchase(s)',
			'portfolio.ran_out' => ({required Object days}) => 'ran out ${days} days ago',
			'portfolio.runs_out_today' => 'runs out today',
			'portfolio.runs_out_in' => ({required Object days}) => 'runs out in ${days} days',
			'portfolio.silent_for' => ({required Object days}) => 'no contact for ${days} days',
			'portfolio.history' => 'Purchase history',
			'portfolio.sale_without_items' => 'Sale with no itemisation',
			'cross_sell.title' => 'What else to offer',
			'cross_sell.because' => ({required Object item}) => 'because they buy ${item}',
			'cross_sell.evidence' => ({required Object count, required Object item}) => '${count} customers who buy ${item} also take',
			'cross_sell.empty' => 'Not enough co-purchase evidence in this account\'s sales yet.',
			'cross_sell.learned_here' => 'Learned from this account\'s sales only.',
			'relationship.title' => 'Relationship dates',
			'relationship.subtitle' => 'Birthdays, company anniversaries, sector days and maintenance. On the day, the draft goes to the approval queue.',
			'relationship.empty' => 'No dates yet',
			'relationship.empty_hint' => 'Add a birthday or a sector day and the system remembers for you.',
			'relationship.add' => 'New date',
			'relationship.kind_birthday' => 'Birthday',
			'relationship.kind_founded' => 'Company anniversary',
			'relationship.kind_sector' => 'Sector day',
			'relationship.kind_maintenance' => 'Preventive maintenance',
			'relationship.kind_custom' => 'Other',
			'relationship.sector_hint' => 'Pick a professional day',
			'relationship.label' => 'Description',
			'relationship.day' => 'Day',
			'relationship.month' => 'Month',
			'relationship.account_wide' => 'Applies to the whole account — it shows here but never becomes a message on its own.',
			'relationship.delete_confirm' => 'Forget this date?',
			'relationship.saved' => 'Date saved',
			'relationship.greeting_only' => 'Only greetings can go out unapproved, and only with autopilot on.',
			'command_center.tile_meetings' => 'Meetings',
			'command_center.tile_won' => 'Won',
			'command_center.funnel_title' => 'Funnel',
			'command_center.more_actions' => ({required Object count}) => '+${count}',
			'command_center.partial_load' => 'Part of the list did not load — there may be more waiting.',
			'command_center.snooze' => 'Later',
			'command_center.snoozed' => ({required Object days}) => 'Snoozed. Back in ${days} days.',
			'command_center.kind_approve_draft' => 'Approach waiting for approval',
			'command_center.kind_unowned_lead' => 'Lead with no owner',
			'command_center.kind_signature_viewed' => 'Opened the contract, did not sign',
			'command_center.kind_contract_alert' => 'Contract deadline',
			'command_center.kind_confirm_sale' => 'Sale waiting for confirmation',
			'command_center.kind_reorder_due' => 'Should have reordered by now',
			'command_center.kind_waiting_reply' => 'Customer waiting for a reply',
			'command_center.kind_unknown' => 'Needs attention',
			'command_center.overdue_days' => ({required Object count}) => '${count} day(s) overdue',
			'command_center.due_today' => 'due today',
			'command_center.due_in_days' => ({required Object count}) => 'due in ${count} day(s)',
			'command_center.waiting_days' => ({required Object count}) => 'waiting ${count} day(s)',
			'command_center.waiting_today' => 'arrived today',
			'command_center.activation_title' => 'Your first sale starts with a lead',
			'command_center.activation_body' => 'There is nothing in the funnel yet. Bring in companies to prospect, or watch the public signals from whoever already said they intend to buy.',
			'command_center.activation_signals' => 'See public signals',
			'command_center.months.0' => 'January',
			'command_center.months.1' => 'February',
			'command_center.months.2' => 'March',
			'command_center.months.3' => 'April',
			'command_center.months.4' => 'May',
			'command_center.months.5' => 'June',
			'command_center.months.6' => 'July',
			'command_center.months.7' => 'August',
			'command_center.months.8' => 'September',
			'command_center.months.9' => 'October',
			'command_center.months.10' => 'November',
			'command_center.months.11' => 'December',
			'command_center.month_of_year' => ({required Object month, required Object year}) => '${month} ${year}',
			'command_center.tile_untouched' => 'Never approached',
			'command_center.tile_untouched_footnote' => 'in your territory',
			'command_center.untouched_target_row' => ({required Object count, required Object unit}) => '${count} ${unit} of what you sell, never approached',
			'command_center.kind_untouched_target' => 'Never approached',
			'command_center.action_see_target' => 'See on the radar',
			'ai_debounce.waiting' => ({required Object seconds}) => 'The AI answers in ~${seconds}s, waiting for the customer to finish',
			'ai_debounce.answering' => 'Answering…',
			'ai_debounce.collapsed' => ({required Object count}) => '${count} messages will be answered together',
			'ai_debounce.answer_now' => 'Answer now',
			'overnight.title' => 'Your SDR worked overnight',
			'overnight.drafted' => ({required Object count}) => '${count} approaches ready to review',
			'overnight.budget_hit' => ({required Object count}) => '${count} did not fit this month\'s allowance',
			'overnight.review' => 'Review',
			'overnight.dismiss' => 'Got it',
			'overnight.blocked_title' => 'Tonight\'s hunt did not run',
			'overnight.blocked_body' => 'No offer was written last night. The reason:',
			'overnight.blocked_cta' => 'Fix it',
			'export.tooltip' => 'Export',
			'export.xlsx' => 'Export to Excel (.xlsx)',
			'export.csv' => 'Export to CSV',
			'export.audited' => 'Every export is recorded',
			'export.done' => ({required Object rows}) => '${rows} rows exported',
			'export.truncated' => ({required Object rows}) => '${rows} rows — the file hit the ceiling and is not the whole list',
			'my_customers.title' => 'The customers you already have',
			'my_customers.subtitle' => 'We read only who you wrote to and when — never what the messages say.',
			'my_customers.scan_action' => 'Search my mailbox',
			'my_customers.scanning' => 'Reading your Sent folder…',
			'my_customers.scan_result' => ({required Object scanned, required Object mailbox, required Object found}) => 'Read ${scanned} messages in ${mailbox} and found ${found} people.',
			'my_customers.scan_skipped' => ({required Object count}) => '${count} robot addresses and your own were left out.',
			'my_customers.scan_partial' => 'We stopped before the end of the folder, so this is not the whole list. Search again to continue.',
			'my_customers.scan_failed' => ({required Object error}) => 'We could not read your Sent folder: ${error}',
			'my_customers.empty_title' => 'Nothing to review yet.',
			'my_customers.empty_hint' => 'Search your mailbox to see who you already work with.',
			'my_customers.sent_count' => ({required Object count}) => '${count} emails sent',
			'my_customers.since' => ({required Object date}) => 'since ${date}',
			'my_customers.last' => ({required Object date}) => 'last on ${date}',
			'my_customers.funnel_label' => 'They go into',
			'my_customers.funnel_required' => 'Pick the funnel before importing.',
			'my_customers.no_funnels' => 'Create a funnel first — it is where the customers land.',
			'my_customers.selected' => ({required Object count}) => '${count} selected',
			'my_customers.import_action' => 'Import selected',
			'my_customers.dismiss_action' => 'Not customers',
			'my_customers.imported' => ({required Object count}) => '${count} customers imported.',
			'my_customers.imported_partial' => ({required Object count, required Object skipped}) => '${count} imported. ${skipped} were already in the system.',
			'my_customers.dismissed' => ({required Object count}) => '${count} removed from the list.',
			'my_customers.select_all' => 'Select all',
			'my_customers.clear_selection' => 'Clear',
			'my_customers.showing_partial' => ({required Object shown, required Object total}) => 'Showing ${shown} of ${total}.',
			'my_customers.view_label' => 'In my inbox',
			'my_customers.open_inbox_customers' => 'See customers from my mailbox',
			'proposal.title' => 'New proposal',
			'proposal.pick_products' => 'Pick what you are offering and how many.',
			'proposal.total' => 'Total',
			'proposal.mixed_currency' => 'These products are priced in different currencies. One proposal carries one currency — otherwise the total would be a number nobody can defend.',
			'proposal.body_label' => 'Your message',
			'proposal.body_hint' => 'Optional. The priced lines are added below whatever you write.',
			'proposal.send_for_approval' => 'Send for approval',
			'proposal.approval_note' => 'Goes to the approval queue. Nothing reaches the customer until you approve it there.',
			'proposal.queued_title' => 'In the approval queue. This is what the reviewer will read:',
			'proposal.history' => 'ALREADY OFFERED',
			'proposal.status_draft' => 'Draft',
			'proposal.status_sent' => ({required Object date}) => 'Sent on ${date}',
			'proposal.no_products' => 'No products yet',
			'proposal.no_products_hint' => 'A proposal is made of what you sell. Add a product and come back.',
			'proposal.go_to_products' => 'Add a product',
			'proposal.open' => 'Make a proposal',
			'proposal.download_pdf' => 'Download PDF',
			'proposal.download_failed' => 'Could not save the file.',
			'proposal.download_hint' => 'A file to send right now, by WhatsApp or e-mail. Nothing is sent on its own.',
			'proposal.suggested_hint' => 'Prefilled with what fits this customer. Change quantities, drop what does not fit, add what is missing.',
			'product_import.title' => 'Import price list',
			'product_import.what_we_read' => 'A CSV with a header row (name/product, price, description, currency) or a manufacturer catalogue PDF — the AI reads the PDF and you check every line before anything is created. You can pick several files at once.',
			'product_import.reading_progress' => ({required Object done, required Object total}) => 'Reading ${done} of ${total}…',
			'product_import.files_failed' => ({required Object count, required Object reason}) => '${count} file(s) could not be read: ${reason}',
			'product_import.files_read' => ({required Object count}) => '${count} files read',
			'product_import.row_from' => ({required Object file}) => 'from ${file}',
			'product_import.pick_file' => 'Choose file',
			'product_import.pick_another' => 'Choose another file',
			'product_import.found' => ({required Object total, required Object importable}) => '${total} lines read, ${importable} ready to import',
			'product_import.with_problems' => ({required Object count}) => '${count} need a look',
			'product_import.already_here' => ({required Object count}) => '${count} already in your catalogue',
			'product_import.confirm' => ({required Object count}) => 'Import ${count} product(s)',
			'product_import.nothing_importable' => 'Nothing to import from this file yet — fix the lines above and choose it again.',
			'product_import.line_problem' => ({required Object line, required Object problem}) => 'line ${line} — ${problem}',
			'product_import.line_price' => ({required Object line, required Object price}) => 'line ${line} — ${price}',
			'product_import.blank_name' => '(no name)',
			'product_import.imported' => ({required Object count}) => '${count} products imported.',
			'product_import.imported_partial' => ({required Object count, required Object skipped}) => '${count} imported, ${skipped} skipped.',
			'product_import.open' => 'Import a spreadsheet',
			'product_import.what_we_read_pdf' => 'Or send the manufacturer\'s brochure as PDF — the AI reads it and you review every line before anything is created.',
			'product_import.pick_pdf' => 'Import brochure (PDF)',
			'product_import.pdf_reading' => 'Reading the brochure with AI — this can take up to a minute…',
			'product_import.page_problem' => ({required Object page, required Object problem}) => 'page ${page} — ${problem}',
			'product_import.page_price' => ({required Object page, required Object price}) => 'page ${page} — ${price}',
			'product_import.price_missing' => 'price not read — fill in later',
			'customer_import.title' => 'Import customer list',
			'customer_import.what_we_read' => 'A CSV with a header row. It reads the company/customer, contact/name, phone/mobile and email columns — comma or semicolon separated, however your spreadsheet saved it. Every customer needs a phone or an email.',
			'customer_import.pick_file' => 'Choose file',
			'customer_import.pick_another' => 'Choose another file',
			'customer_import.found' => ({required Object total, required Object importable}) => '${total} lines read, ${importable} ready to import',
			'customer_import.with_problems' => ({required Object count}) => '${count} need a look',
			'customer_import.already_here' => ({required Object count}) => '${count} are already in your base',
			'customer_import.confirm' => ({required Object count}) => 'Import ${count} customer(s)',
			'customer_import.nothing_importable' => 'Nothing to import from this file yet — fix the lines above and choose the file again.',
			'customer_import.line_problem' => ({required Object line, required Object problem}) => 'line ${line} — ${problem}',
			'customer_import.line_address' => ({required Object line, required Object address}) => 'line ${line} — ${address}',
			'customer_import.blank_name' => '(no name)',
			'customer_import.imported' => ({required Object count}) => '${count} customers imported.',
			_ => null,
		} ?? switch (path) {
			'customer_import.imported_partial' => ({required Object count, required Object skipped}) => '${count} imported, ${skipped} skipped.',
			'customer_import.open' => 'Import spreadsheet',
			'sales_import.title' => 'Import sales history',
			'sales_import.open' => 'Import sales',
			'sales_import.settings_subtitle' => 'Bring in past sales so repurchase dates and cross-sell have data',
			'sales_import.hint' => 'A CSV with a header row. It reads the customer, product, date, amount and quantity columns — comma or semicolon separated, however your spreadsheet saved it. Customers must already be in your base: import the customer list first.',
			'sales_import.pick_file' => 'Choose file',
			'sales_import.pick_another' => 'Choose another file',
			'sales_import.preview_title' => 'What the file says',
			'sales_import.found' => ({required Object total, required Object importable}) => '${total} lines read, ${importable} ready to record',
			'sales_import.problems_title' => ({required Object count}) => '${count} need a look',
			'sales_import.already_recorded' => ({required Object count}) => '${count} are already recorded',
			'sales_import.unknown_customers' => ({required Object count}) => '${count} name customers not in your base — import the customer list first',
			'sales_import.confirm' => ({required Object count}) => 'Record ${count} sale(s)',
			'sales_import.nothing_importable' => 'Nothing to record from this file yet — fix the lines above and choose the file again.',
			'sales_import.line_problem' => ({required Object line, required Object problem}) => 'line ${line} — ${problem}',
			'sales_import.line_detail' => ({required Object line, required Object detail}) => 'line ${line} — ${detail}',
			'sales_import.blank_customer' => '(no customer)',
			'sales_import.result' => ({required Object count}) => '${count} sales recorded.',
			'sales_import.skipped' => ({required Object count, required Object skipped}) => '${count} sales recorded, ${skipped} skipped.',
			'sales_import.view_portfolio' => 'View portfolio',
			'lead_timeline.title' => 'CONVERSATION',
			'lead_timeline.empty' => 'Nothing has been sent to this customer yet.',
			'lead_timeline.truncated' => 'Showing the most recent messages.',
			'lead_timeline.from_customer' => 'Customer',
			'lead_timeline.from_team' => 'Us',
			'lead_timeline.from_bot' => 'Bot',
			'lead_timeline.attachment' => ({required Object kind}) => '[${kind}]',
			'lead_timeline.not_delivered' => 'Not delivered',
			'lead_timeline.reply_hint' => 'Write your reply…',
			'lead_timeline.reply_send' => 'Reply',
			'lead_timeline.reply_sent' => 'Reply sent.',
			'lead_timeline.no_conversation' => 'Replying opens once there is a conversation — the first offer goes out from the queue.',
			'email_composer.subject_hint' => 'Subject',
			'email_composer.body_hint' => 'Write your email…',
			'email_composer.send' => 'Send',
			'email_composer.reply' => 'Reply',
			'radar.tab' => 'Radar',
			'radar.tab_hoje' => 'Today',
			'radar.warnings' => ({required Object sources}) => 'Could not read: ${sources}. The rest of the feed is live.',
			'radar.source_down' => ({required Object source, required Object ago}) => 'We could not read ${source} for ${ago}. What follows is from the last collection — it may not be what is open now.',
			'radar.source_down_never' => ({required Object source}) => 'We have not been able to read ${source} yet. Nothing in this list came from it.',
			'radar.ago_minutes' => ({required Object n}) => '${n} min',
			'radar.ago_hours' => ({required Object n}) => '${n} h',
			'radar.ago_days' => ({required Object n}) => '${n} days',
			'radar.filter_all' => 'All',
			'radar.type_licitacao' => 'Tenders',
			'radar.type_parque' => 'Installed base',
			'radar.type_recompra' => 'Reorders',
			'radar.type_data' => 'Dates',
			'radar.empty' => 'Nothing on the radar right now',
			'radar.empty_hint' => 'Tenders, CNES installed-base hits, reorders coming due and relationship dates appear here as they are detected.',
			'radar.offer_queued' => 'Draft queued — waiting for your approval in Today.',
			'radar.deadline_today' => 'today',
			'radar.deadline_days' => ({required Object count}) => 'in ${count}d',
			'radar.open_draft' => 'Open draft',
			'radar.prepare_offer' => 'Prepare offer',
			'radar.open_customer' => 'View customer',
			'radar.hunt_title' => 'Stock the territory',
			'radar.hunt_hint' => 'Searches CNES for who declares the equipment you serve — public data, counts, never equipment age. Phone and address come from the CNES registry.',
			'radar.hunt_placeholder' => 'e.g. tomógrafo, ventilador, ultrassom',
			'radar.hunt_empty' => 'Nothing found.',
			'radar.hunt_units' => ({required Object count}) => '${count} declared to CNES',
			'radar.hunt_known' => 'already in your contacts',
			'radar.type_cross_sell' => 'Cross-sell',
			'radar.carteira_hint' => 'Reorders, cross-sell and dates appear after the first recorded sale.',
			'radar.empty_hint_prospecting' => 'Tenders and CNES installed-base hits appear here as they are detected.',
			'radar.footer_note' => 'Everything turned into an offer lands in Today for your approval — nothing is sent from here.',
			'radar.watching_catalog' => ({required Object count}) => 'catalog (${count} products)',
			'radar.week_prefix' => 'This week:',
			'radar.week_sent' => 'sent',
			'radar.week_replies' => 'replies',
			'radar.week_proposals' => 'proposals',
			'radar.data_month' => ({required Object month}) => 'CNES data from ${month}',
			'radar.hunt_scope_cities' => ({required Object cities}) => 'in your territory (${cities})',
			'radar.hunt_scope_uf' => ({required Object uf}) => 'in ${uf}',
			'radar.auto_hunt_cta' => 'Hunt with my catalogue',
			'radar.auto_hunt_hint' => 'It reads your products and brings the biggest hospitals in your territory you have never approached. Nothing is sent.',
			'radar.auto_hunt_result' => ({required Object created, required Object found}) => '${created} new leads in the funnel, out of ${found} found',
			'radar.auto_hunt_none' => 'No new establishment in your territory right now',
			'radar.park_universe' => ({required Object total, required Object scope, required Object shown}) => '${total} hospitals ${scope} declare equipment your catalogue matches. Showing the ${shown} biggest.',
			'radar.offer_more' => ({required Object count}) => '+${count} more',
			'radar.type_contrato' => 'Contract expiring',
			'radar.type_ata' => 'Price registry',
			'radar.edge_before_tender' => 'before the tender',
			'radar.edge_no_tender' => 'no tender needed',
			'radar.read_and_send' => 'Read and send',
			'radar.back_to_panel' => 'Back to the panel',
			'radar.panel_object' => 'What they are buying',
			'radar.panel_buyer' => 'Who is buying',
			'radar.panel_reference' => 'Number in the public record',
			'radar.panel_published' => 'Published',
			'radar.panel_deadline' => 'Deadline',
			'radar.panel_value' => 'Value',
			'radar.panel_no_link' => 'This notice published no link and no file. Copy the number above and search it on PNCP — it is the only reference there is.',
			'radar.panel_park_items' => 'What this hospital declares',
			'radar.panel_offer_title' => 'What you can offer',
			'radar.panel_close' => 'Close',
			'radar.panel_copy_reference' => 'Copy number',
			'radar.panel_copied' => 'Number copied',
			'radar.panel_empty' => 'Tap an opportunity to see everything we know about it.',
			'radar.panel_portal' => 'PNCP — Brazil\'s public procurement portal',
			'radar.panel_open_portal' => 'Open PNCP',
			'radar.panel_buyer_known' => 'You have already sold to this buyer',
			'radar.panel_buyer_cold' => 'This buyer has never heard of you',
			'radar.panel_history' => 'What this body has contracted',
			'radar.panel_history_count' => ({required Object n}) => 'This buyer has ${n} contract(s) in your trade on the public record.',
			'radar.panel_history_none' => 'No contract in your trade on the public record for this buyer.',
			'radar.panel_buyer_contacted' => 'You have already sent an offer to this buyer',
			'radar.panel_buyer_queued' => 'Offer written, waiting for your approval — nothing sent',
			'radar.panel_buyer_listed' => 'Already on your board — nothing written yet',
			'radar.panel_reach' => 'How to reach them',
			'radar.panel_stage' => ({required Object stage}) => 'On the board in: ${stage}',
			'radar.panel_open_board' => 'Open on the board',
			'radar.panel_purchaser' => 'Who buys for them',
			'radar.panel_history_via' => ({required Object org, required Object n}) => '${org}, which buys for them, has ${n} contract(s) in your trade on the public record.',
			'radar.panel_beds' => 'Beds it runs',
			'radar.panel_habilitations' => 'Funded by the Ministry',
			'radar.panel_habilitation_beds' => ({required Object name, required Object n}) => '${name} — ${n} bed(s) funded',
			'radar.panel_services' => 'Services it performs',
			'start.greeting_morning' => ({required Object name}) => 'Good morning, ${name}',
			'start.greeting_afternoon' => ({required Object name}) => 'Good afternoon, ${name}',
			'start.greeting_evening' => ({required Object name}) => 'Good evening, ${name}',
			'start.date_line' => ({required Object weekday, required Object month, required Object day}) => '${weekday}, ${month} ${day}',
			'start.weekdays.0' => 'Monday',
			'start.weekdays.1' => 'Tuesday',
			'start.weekdays.2' => 'Wednesday',
			'start.weekdays.3' => 'Thursday',
			'start.weekdays.4' => 'Friday',
			'start.weekdays.5' => 'Saturday',
			'start.weekdays.6' => 'Sunday',
			'start.summary_ready' => ({required Object count}) => '${count} offer(s) ready',
			'start.summary_waiting' => ({required Object count}) => '${count} customer(s) waiting',
			'start.summary_due_today' => ({required Object count}) => '${count} due today',
			'start.summary_due_none' => 'nothing due today',
			'start.review_n' => ({required Object count}) => 'Review the ${count}',
			'start.see_radar' => 'See Radar',
			'start.action_reply' => 'Reply',
			'start.action_assume' => 'Take it',
			'start.distribute_team' => '✦ Distribute across the team',
			'start.sorted_by_urgency' => 'sorted by urgency',
			'start.filter_all' => ({required Object count}) => 'All ${count}',
			'start.rail_radar_title' => 'Radar today',
			'start.rail_radar_open' => 'open →',
			'start.rail_week_title' => 'Your week',
			'start.rail_engine_title' => 'Start the engine',
			'start.of_total' => ({required Object current, required Object total}) => '${current} of ${total}',
			'start.register_sale_link' => 'record a sale',
			'start.unowned_footnote' => ({required Object count}) => '${count} with no owner',
			'calendar.tab' => 'Expiries',
			'calendar.title' => 'What runs out',
			'calendar.subtitle' => ({required Object days}) => 'Public contracts in your state ending in the next ${days} days. When one ends the purchase has to happen again — and you know that months before any tender exists.',
			'calendar.empty_title' => 'Nothing expiring in this window',
			'calendar.empty_body' => ({required Object days}) => 'No contract for what you sell ends in the next ${days} days in your state.',
			'calendar.scanning_title' => 'Still reading your territory’s contracts',
			'calendar.scanning_body' => 'We ask the portal hospital by hospital, without hammering their server. It fills in over the next few days.',
			'calendar.no_territory_title' => 'Set where you sell',
			'calendar.no_territory_body' => 'Without a state we cannot tell which contracts matter to you. Set it in Settings → Company.',
			'calendar.no_keywords_title' => 'Tell us what you sell',
			'calendar.no_keywords_body' => 'The calendar uses the same words as the Radar. Add your products or adjust the watch.',
			'calendar.days_left' => ({required Object count}) => 'ends in ${count}d',
			'calendar.days_today' => 'ends today',
			'calendar.days_past' => ({required Object count}) => 'ended ${count}d ago',
			'calendar.holder' => ({required Object name}) => 'held by ${name}',
			'calendar.holder_unknown' => 'current supplier not published',
			'calendar.price_registry' => 'price registry',
			'calendar.rides_others' => 'buys by adhesion',
			'calendar.rides_others_hint' => 'This body rides other bodies’ price registries. You can sell without winning a tender.',
			'calendar.open_record' => 'Open record',
			'calendar.value_unknown' => 'value not published',
			'calendar.work' => 'Work this one',
			'calendar.worked' => ({required Object name}) => '${name} is in your funnel.',
			'calendar.work_failed' => 'Could not add it to the funnel.',
			'calendar.open_funnel' => 'Open funnel',
			'calendar.work_queued' => 'Added to your customers. The buyer still needs a contact.',
			_ => null,
		};
	}
}
