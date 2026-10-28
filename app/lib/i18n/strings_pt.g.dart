///
/// Generated file. Do not edit.
///
// coverage:ignore-file
// ignore_for_file: type=lint, unused_import
// dart format off

import 'package:flutter/widgets.dart';
import 'package:intl/intl.dart';
import 'package:slang/generated.dart';
import 'strings.g.dart';

// Path: <root>
class TranslationsPt extends Translations with BaseTranslations<AppLocale, Translations> {
	/// You can call this constructor and build your own translation instance of this locale.
	/// Constructing via the enum [AppLocale.build] is preferred.
	TranslationsPt({Map<String, Node>? overrides, PluralResolver? cardinalResolver, PluralResolver? ordinalResolver, TranslationMetadata<AppLocale, Translations>? meta})
		: assert(overrides == null, 'Set "translation_overrides: true" in order to enable this feature.'),
		  $meta = meta ?? TranslationMetadata(
		    locale: AppLocale.pt,
		    overrides: overrides ?? {},
		    cardinalResolver: cardinalResolver,
		    ordinalResolver: ordinalResolver,
		  ),
		  super(cardinalResolver: cardinalResolver, ordinalResolver: ordinalResolver) {
		super.$meta.setFlatMapFunction($meta.getTranslation); // copy base translations to super.$meta
		$meta.setFlatMapFunction(_flatMapFunction);
	}

	/// Metadata for the translations of <pt>.
	@override final TranslationMetadata<AppLocale, Translations> $meta;

	/// Access flat map
	@override dynamic operator[](String key) => $meta.getTranslation(key) ?? super.$meta.getTranslation(key);

	late final TranslationsPt _root = this; // ignore: unused_field

	@override 
	TranslationsPt $copyWith({TranslationMetadata<AppLocale, Translations>? meta}) => TranslationsPt(meta: meta ?? this.$meta);

	// Translations
	@override late final _Translations$home$pt home = _Translations$home$pt._(_root);
	@override late final _Translations$auth$pt auth = _Translations$auth$pt._(_root);
	@override late final _Translations$profile$pt profile = _Translations$profile$pt._(_root);
	@override late final _Translations$onboarding$pt onboarding = _Translations$onboarding$pt._(_root);
	@override late final _Translations$dashboard$pt dashboard = _Translations$dashboard$pt._(_root);
	@override late final _Translations$email_verification$pt email_verification = _Translations$email_verification$pt._(_root);
	@override late final _Translations$settings$pt settings = _Translations$settings$pt._(_root);
	@override late final _Translations$common$pt common = _Translations$common$pt._(_root);
	@override late final _Translations$billing$pt billing = _Translations$billing$pt._(_root);
	@override late final _Translations$team$pt team = _Translations$team$pt._(_root);
	@override late final _Translations$channels$pt channels = _Translations$channels$pt._(_root);
	@override late final _Translations$whatsapp$pt whatsapp = _Translations$whatsapp$pt._(_root);
	@override late final _Translations$notifications$pt notifications = _Translations$notifications$pt._(_root);
	@override late final _Translations$api_keys$pt api_keys = _Translations$api_keys$pt._(_root);
	@override late final _Translations$notes$pt notes = _Translations$notes$pt._(_root);
	@override late final _Translations$message_actions$pt message_actions = _Translations$message_actions$pt._(_root);
	@override late final _Translations$contact_sheet$pt contact_sheet = _Translations$contact_sheet$pt._(_root);
	@override late final _Translations$conversation$pt conversation = _Translations$conversation$pt._(_root);
	@override late final _Translations$lgpd$pt lgpd = _Translations$lgpd$pt._(_root);
	@override late final _Translations$feedback$pt feedback = _Translations$feedback$pt._(_root);
	@override late final _Translations$product_tour$pt product_tour = _Translations$product_tour$pt._(_root);
	@override late final _Translations$feature_announcement$pt feature_announcement = _Translations$feature_announcement$pt._(_root);
	@override late final _Translations$errors$pt errors = _Translations$errors$pt._(_root);
	@override late final _Translations$audio_recorder$pt audio_recorder = _Translations$audio_recorder$pt._(_root);
	@override late final _Translations$bulk_actions$pt bulk_actions = _Translations$bulk_actions$pt._(_root);
	@override late final _Translations$image_viewer$pt image_viewer = _Translations$image_viewer$pt._(_root);
	@override late final _Translations$media_send$pt media_send = _Translations$media_send$pt._(_root);
	@override late final _Translations$reply_suggestions$pt reply_suggestions = _Translations$reply_suggestions$pt._(_root);
	@override late final _Translations$pwa$pt pwa = _Translations$pwa$pt._(_root);
	@override late final _Translations$audio_player$pt audio_player = _Translations$audio_player$pt._(_root);
	@override late final _Translations$media_preview$pt media_preview = _Translations$media_preview$pt._(_root);
	@override late final _Translations$sentiment$pt sentiment = _Translations$sentiment$pt._(_root);
	@override late final _Translations$owner$pt owner = _Translations$owner$pt._(_root);
	@override late final _Translations$sales$pt sales = _Translations$sales$pt._(_root);
	@override late final _Translations$support$pt support = _Translations$support$pt._(_root);
	@override late final _Translations$claude_integration$pt claude_integration = _Translations$claude_integration$pt._(_root);
	@override late final _Translations$shell$pt shell = _Translations$shell$pt._(_root);
	@override late final _Translations$board$pt board = _Translations$board$pt._(_root);
	@override late final _Translations$lead_detail$pt lead_detail = _Translations$lead_detail$pt._(_root);
	@override late final _Translations$account_menu$pt account_menu = _Translations$account_menu$pt._(_root);
	@override late final _Translations$copilot$pt copilot = _Translations$copilot$pt._(_root);
	@override late final _Translations$attachment$pt attachment = _Translations$attachment$pt._(_root);
	@override late final _Translations$sender_auth$pt sender_auth = _Translations$sender_auth$pt._(_root);
	@override late final _Translations$email_inbox$pt email_inbox = _Translations$email_inbox$pt._(_root);
	@override late final _Translations$email_risk$pt email_risk = _Translations$email_risk$pt._(_root);
	@override late final _Translations$brand$pt brand = _Translations$brand$pt._(_root);
	@override late final _Translations$suppressions$pt suppressions = _Translations$suppressions$pt._(_root);
	@override late final _Translations$signals$pt signals = _Translations$signals$pt._(_root);
	@override late final _Translations$sales_record$pt sales_record = _Translations$sales_record$pt._(_root);
	@override late final _Translations$portfolio$pt portfolio = _Translations$portfolio$pt._(_root);
	@override late final _Translations$cross_sell$pt cross_sell = _Translations$cross_sell$pt._(_root);
	@override late final _Translations$relationship$pt relationship = _Translations$relationship$pt._(_root);
	@override late final _Translations$command_center$pt command_center = _Translations$command_center$pt._(_root);
	@override late final _Translations$ai_debounce$pt ai_debounce = _Translations$ai_debounce$pt._(_root);
	@override late final _Translations$overnight$pt overnight = _Translations$overnight$pt._(_root);
	@override late final _Translations$export$pt export = _Translations$export$pt._(_root);
	@override late final _Translations$my_customers$pt my_customers = _Translations$my_customers$pt._(_root);
	@override late final _Translations$proposal$pt proposal = _Translations$proposal$pt._(_root);
	@override late final _Translations$product_import$pt product_import = _Translations$product_import$pt._(_root);
	@override late final _Translations$customer_import$pt customer_import = _Translations$customer_import$pt._(_root);
	@override late final _Translations$sales_import$pt sales_import = _Translations$sales_import$pt._(_root);
	@override late final _Translations$lead_timeline$pt lead_timeline = _Translations$lead_timeline$pt._(_root);
	@override late final _Translations$email_composer$pt email_composer = _Translations$email_composer$pt._(_root);
	@override late final _Translations$radar$pt radar = _Translations$radar$pt._(_root);
	@override late final _Translations$start$pt start = _Translations$start$pt._(_root);
	@override late final _Translations$calendar$pt calendar = _Translations$calendar$pt._(_root);
}

// Path: home
class _Translations$home$pt extends Translations$home$en {
	_Translations$home$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String onboarding_checklist_title({required Object current, required Object total}) => 'Primeiros passos — ${current} de ${total}';
	@override String get onboarding_checklist_dismiss => 'Dispensar por enquanto';
	@override String get onboarding_checklist_agent => 'Crie seu primeiro agente';
	@override String get onboarding_checklist_preview => 'Experimente seu agente no preview';
	@override String get onboarding_checklist_preview_cta => 'Testar agora';
	@override String get onboarding_checklist_channel => 'Conecte um canal de mensagens';
	@override String get onboarding_checklist_channel_cta => 'Conectar';
	@override String get onboarding_checklist_knowledge => 'Ensine o agente sobre seu negócio';
	@override String get onboarding_checklist_knowledge_cta => 'Adicionar contexto';
	@override String get onboarding_checklist_inbound => 'Receba a primeira mensagem real';
	@override String welcome_user({required Object name}) => 'Bem-vindo, ${name}';
	@override String get onboarding_checklist_products => 'Traga o que você vende';
	@override String get onboarding_checklist_products_cta => 'Importar lista de preços';
	@override String get onboarding_checklist_customers => 'Traga seus clientes';
	@override String get onboarding_checklist_customers_cta => 'Importar planilha';
	@override String get onboarding_checklist_proposal => 'Faça sua primeira proposta';
	@override String get onboarding_checklist_proposal_cta => 'Abrir o funil';
	@override String get onboarding_checklist_team => 'Convide seu vendedor';
	@override String get onboarding_checklist_team_cta => 'Convidar';
}

// Path: auth
class _Translations$auth$pt extends Translations$auth$en {
	_Translations$auth$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get login_hero_title => 'Ofertas prontas para aprovar, todo dia.';
	@override String get login_eyebrow => 'SEU VENDEDOR AUTÔNOMO';
	@override String get login_title => 'Entre na sua conta';
	@override String get email_placeholder => 'Digite seu e-mail';
	@override String get password_placeholder => 'Digite sua senha';
	@override String get remember_me => 'Lembrar-me';
	@override String get forgot_password => 'Esqueci a senha';
	@override String get login_button => 'Login';
	@override String get or_login_with => 'Ou';
	@override String get google => 'Google';
	@override String get meta => 'Facebook';
	@override String get dont_have_account => 'Não tem uma conta?';
	@override String get create_account => 'Criar conta';
	@override String get register_title => 'Criar Conta';
	@override String get register_subtitle => 'Comece a usar o Replider hoje mesmo';
	@override String get register_paused_title => 'Cadastro em breve';
	@override String get register_paused_body => 'Estamos preparando a nova Replider e os cadastros estão temporariamente fechados. Já tem conta? É só entrar.';
	@override String get register_paused_error => 'Os cadastros estão temporariamente fechados.';
	@override String get account_name => 'Nome da empresa';
	@override String get full_name => 'Nome completo';
	@override String get confirm_password => 'Confirmar senha';
	@override String get already_have_account => 'Já tem uma conta?';
	@override String get sign_in => 'Entrar';
	@override String get field_required => 'Campo obrigatório';
	@override String get invalid_email => 'E-mail inválido';
	@override String get password_too_short => 'Senha muito curta';
	@override String min_characters({required Object n}) => 'Mínimo ${n} caracteres';
	@override String get passwords_dont_match => 'Senhas não coincidem';
	@override String get forgot_title => 'Recuperar senha';
	@override String get forgot_subtitle => 'Informe seu e-mail e enviaremos as instruções.';
	@override String get forgot_send_link => 'Enviar link';
	@override String forgot_resend_in({required Object seconds}) => 'Reenviar em ${seconds}s';
	@override String get forgot_success => 'Se a conta existir, você receberá um e-mail em instantes.';
	@override String get reset_title => 'Defina uma nova senha';
	@override String get reset_subtitle => 'Escolha uma nova senha para a sua conta.';
	@override String get reset_new_password => 'Nova senha';
	@override String get reset_confirm_password => 'Confirme a nova senha';
	@override String get reset_min_length => 'A senha deve ter ao menos 8 caracteres.';
	@override String get reset_mismatch => 'As senhas não coincidem.';
	@override String get reset_submit => 'Redefinir senha';
	@override String get reset_success => 'Senha redefinida! Agora você pode entrar com a nova senha.';
	@override String get reset_invalid_link => 'Este link de redefinição é inválido ou incompleto. Solicite um novo.';
	@override String get back_to_login => 'Voltar para o login';
	@override String get terms_required => 'Você precisa aceitar os Termos de Uso e a Política de Privacidade.';
	@override String get retry => 'Tentar novamente';
	@override String get mfa_title => 'Verificação em duas etapas';
	@override String get mfa_subtitle => 'Digite o código de 6 dígitos do app autenticador, ou um dos seus códigos de recuperação.';
	@override String get mfa_code_hint => 'Código';
	@override String get mfa_verify_button => 'Validar e continuar';
	@override String get mfa_cancel => 'Voltar ao login';
	@override String coupon_banner_valid({required Object code, required Object percent}) => 'Cupom ${code} aplicado — ${percent}% off no checkout.';
	@override String coupon_banner_invalid({required Object code}) => 'Cupom ${code} não é mais válido.';
	@override String get session_expired_notice => 'Sua sessão expirou por segurança. Entre novamente — você volta para a tela onde estava.';
	@override String session_ending_title({required Object minutes}) => 'Sua sessão termina em ${minutes} min';
	@override String get session_ending_body => 'Entre de novo agora, na sua hora, em vez de perder o que estiver fazendo no meio.';
	@override String get session_ending_cta => 'Entrar de novo';
	@override String invite_banner({required Object email}) => 'Convite para ${email} — sua conta já nasce no plano contratado.';
	@override String get invite_invalid => 'Este link de convite é inválido, expirou ou já foi usado. Peça um novo link.';
	@override String get invite_invalid_title => 'Link de convite expirado';
	@override String get register_subtitle_invite => 'Ative a conta do plano que combinamos com você.';
	@override String get invite_probe_failed => 'Não conseguimos verificar seu convite agora — problema de conexão, não do link. Tente de novo.';
	@override String get verify_email_unknown => 'o e-mail da sua conta';
	@override String get consent_prefix => 'Li e aceito os ';
	@override String get consent_terms => 'Termos de Uso';
	@override String get consent_and => ' e a ';
	@override String get consent_privacy => 'Política de Privacidade';
}

// Path: profile
class _Translations$profile$pt extends Translations$profile$en {
	_Translations$profile$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Meu Perfil';
	@override String get user_not_found => 'Usuário não encontrado';
	@override String get personal_info => 'INFORMAÇÕES PESSOAIS';
	@override String get name => 'Nome';
	@override String get email => 'E-mail';
	@override String get save_info => 'Salvar Informações';
	@override String get change_password => 'ALTERAR SENHA';
	@override String get current_password => 'Senha atual';
	@override String get new_password => 'Nova senha';
	@override String get confirm_new_password => 'Confirmar nova senha';
	@override String get change_password_button => 'Alterar Senha';
	@override String get fill_all_fields => 'Preencha todos os campos';
	@override String get fill_all_password_fields => 'Preencha todos os campos de senha';
	@override String get passwords_dont_match => 'As senhas não coincidem';
	@override String get min_password_length => 'A nova senha deve ter pelo menos 6 caracteres';
	@override String get profile_updated => 'Perfil atualizado com sucesso!';
	@override String get password_changed => 'Senha alterada com sucesso!';
	@override String get profile_update_unavailable => 'Atualização de perfil ainda não disponível na API';
	@override String get password_change_unavailable => 'Alteração de senha ainda não disponível na API';
	@override String get incorrect_current_password => 'Senha atual incorreta';
	@override String get unexpected_error => 'Erro inesperado';
	@override String get network_error => 'Erro de rede';
	@override String get error_prefix => 'Erro';
	@override String get two_factor_section_label => 'AUTENTICAÇÃO EM DUAS ETAPAS';
	@override String get two_factor_label => 'Autenticação em duas etapas';
	@override String get two_factor_desc => 'Adiciona uma segunda etapa no login com um código de 6 dígitos de um app autenticador (Google Authenticator, 1Password, Authy).';
	@override String get two_factor_status_off => 'DESATIVADA';
	@override String get two_factor_status_on => 'ATIVA';
	@override String get two_factor_setup_cta => 'Ativar 2FA';
	@override String get two_factor_step1 => '1. Escaneie este QR no app autenticador';
	@override String get two_factor_step2 => '2. Digite o código de 6 dígitos que aparece no app';
	@override String get two_factor_manual_secret_label => 'Ou digite este segredo manualmente:';
	@override String get two_factor_verify_cta => 'Validar e ativar';
	@override String get two_factor_disable_prompt => 'Digite um código 2FA atual (ou código de recuperação) para desativar.';
	@override String get two_factor_code_hint => 'Código de 6 dígitos ou de recuperação';
	@override String get two_factor_disable_cta => 'Desativar 2FA';
	@override String get two_factor_recovery_title => 'Salve seus códigos de recuperação';
	@override String get two_factor_recovery_warning => 'Cada código funciona uma vez. Guarde em local seguro — sem eles, perder o celular tranca o acesso.';
	@override String get two_factor_recovery_copy_cta => 'Copiar todos';
	@override String get two_factor_recovery_done_cta => 'Já salvei';
	@override String get signature_label => 'Assinatura';
	@override String get signature_help => 'Assinatura opcional oferecida no composer em respostas manuais. Nunca aplicada às respostas da IA.';
}

// Path: onboarding
class _Translations$onboarding$pt extends Translations$onboarding$en {
	_Translations$onboarding$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get welcome_title => 'Bem-vindo ao Replider!';
	@override String get welcome_subtitle => 'Vamos configurar seu workspace em poucos passos.';
	@override String get company_name_hint => 'Nome da sua empresa';
	@override String get step_company_title => 'Sobre a sua empresa';
	@override String get step_company_subtitle => 'A IA escreve as abordagens a partir daqui. Sem isso, ela só sabe o nome.';
	@override String get step_company_placeholder => 'Ex.: Sua Empresa Equipamentos Hospitalares';
	@override String get company_about_label => 'O que vocês fazem';
	@override String get company_about_placeholder => 'Ex.: Venda, locação e assistência técnica de equipamentos hospitalares em Mato Grosso.';
	@override String get company_about_help => 'Uma frase. Quem você atende e o que resolve.';
	@override String get company_tone_label => 'Como vocês falam (opcional)';
	@override String get company_lines_label => 'Como a sua empresa ganha dinheiro?';
	@override String get company_line_sell => 'Vendo equipamentos';
	@override String get company_line_rent => 'Alugo equipamentos';
	@override String get company_line_service => 'Faço assistência técnica';
	@override String get company_territory_label => 'Onde você vende?';
	@override String get company_territory_uf => 'Estado';
	@override String get company_territory_cities => 'Cidades (opcional)';
	@override String get company_territory_cities_hint => 'Cuiabá, Várzea Grande';
	@override String get company_tone_direct => 'Direto';
	@override String get company_tone_direct_prose => 'Escreva de forma direta e objetiva, sem rodeios nem adjetivos de venda.';
	@override String get company_tone_consultative => 'Consultivo';
	@override String get company_tone_consultative_prose => 'Escreva como um especialista que entende do problema do cliente, explicando o porquê antes do que.';
	@override String get company_tone_warm => 'Próximo';
	@override String get company_tone_warm_prose => 'Escreva de forma calorosa e próxima, como quem já conhece a pessoa, sem ser informal demais.';
	@override String get company_tone_formal => 'Formal';
	@override String get company_tone_formal_prose => 'Escreva de forma formal e respeitosa, tratando por você e evitando gírias.';
	@override String get step_agent_subtitle => '';
	@override String get provider_label => 'Provedor de IA';
	@override String get step_channel_title => 'Por onde você manda?';
	@override String get step_channel_subtitle => 'A primeira oferta sai por e-mail: conecte a conta que você já usa. WhatsApp entra depois, para o relacionamento.';
	@override String get step_channel_badge_telegram => 'Mais rápido';
	@override String get step_channel_badge_whatsapp => 'Popular · ~10 min';
	@override String get step_channel_badge_coming_soon => 'Em breve';
	@override String get step_channel_whatsapp_help => 'Não sei por onde começar';
	@override String get step_channel_whatsapp_help_failed => 'Não consegui abrir o vídeo de ajuda. Confira as configurações do navegador.';
	@override String get telegram_bot_token => 'Token do Bot Telegram';
	@override String get whatsapp_phone_number_id => 'ID do Número WhatsApp';
	@override String get whatsapp_access_token => 'Token de Acesso WhatsApp';
	@override String get instagram_page_id => 'Page ID do Instagram';
	@override String get instagram_access_token => 'Token de Acesso do Instagram';
	@override String get step_channel_instagram_hint => 'Requer uma conta profissional do Instagram (Business ou Creator) e um token de acesso com a permissão instagram_business_manage_messages.';
	@override String get sms_account_sid => 'Account SID da Twilio';
	@override String get sms_auth_token => 'Auth Token da Twilio';
	@override String get sms_from_number => 'Número Twilio (E.164)';
	@override String get step_channel_sms_hint => 'Use seu próprio número Twilio: cole o Account SID, o Auth Token e o número de telefone do console da Twilio. Ao finalizar, copie a URL de webhook do canal em Configurações → Canais e configure-a no seu número na Twilio.';
	@override String get invalid_sms_account_sid => 'O Account SID deve começar com AC seguido de 32 caracteres. Copie do console da Twilio.';
	@override String get invalid_sms_auth_token => 'Cole seu Auth Token da Twilio para conectar o canal de SMS.';
	@override String get invalid_sms_from_number => 'O número deve estar no formato internacional E.164, ex.: +5565999999999.';
	@override String get create_agent_first_for_file => 'Crie o agente antes de enviar o arquivo.';
	@override String get create_agent_first_for_url => 'Crie o agente antes de importar a URL.';
	@override String knowledge_summary_file({required Object fileName, required Object count}) => '${fileName} · ${count} trechos indexados';
	@override String knowledge_summary_url({required Object pages, required Object count}) => '${pages} páginas · ${count} trechos indexados';
	@override String get step_done_title => 'Tudo pronto!';
	@override String get step_done_subtitle => 'Seu workspace está configurado. As mensagens começarão a chegar assim que seus clientes entrarem em contato.';
	@override String get step_done_tip_1 => 'Teste seu agente em Configurações → Agente → Preview';
	@override String get step_done_tip_2 => 'Monitore conversas no Dashboard';
	@override String get step_done_tip_3 => 'Adicione mais conhecimento a qualquer momento em Configurações';
	@override String get go_to_dashboard => 'Ir para o Dashboard';
	@override String get next => 'Próximo';
	@override String get back => 'Voltar';
	@override String get skip => 'Pular por agora';
	@override String step_of({required Object current, required Object total}) => 'Passo ${current} de ${total}';
	@override String get field_required => 'Campo obrigatório';
	@override String get step_preview_chat_next => 'Funcionou! Avançar';
	@override String get step_preview_chat_skip => 'Testar depois';
	@override String get step_channel_whatsapp_manual_hint => 'Informe o Phone Number ID do painel Meta for Developers. O token de acesso é gerenciado automaticamente.';
	@override String get step_channel_whatsapp_connected => 'WhatsApp conectado! Clique em "Próximo" para continuar.';
	@override String get step_channel_whatsapp_or_manual => 'ou conecte manualmente';
	@override String get step_channel_badge_email => 'Principal';
	@override String get email_from_address => 'E-mail de envio';
	@override String get email_smtp_host => 'Servidor SMTP';
	@override String get email_smtp_port => 'Porta';
	@override String get email_smtp_password => 'Senha SMTP';
	@override String get email_dns_title => 'Depois: liberar seu domínio para enviar';
	@override String get email_dns_body => 'Seu domínio precisa publicar três registros (SPF, DKIM e DMARC) para que os e-mails cheguem na caixa de entrada em vez do spam. Ao terminar, o Replider mostra os registros prontos para copiar. A propagação leva de minutos a algumas horas.';
	@override String get products_title => 'O que você vende?';
	@override String get products_subtitle => 'Traga sua lista de preços como arquivo. Toda proposta é feita disso, e você pode corrigir ou adicionar depois.';
	@override String get aikey_title => 'Sua chave de IA';
	@override String get aikey_subtitle => 'No seu contrato, a IA roda na SUA chave — o consumo é seu, sem intermediário. Cole a chave do Google AI Studio (Gemini) para ler catálogos e escrever ofertas.';
	@override String get aikey_field => 'Chave da API (Gemini)';
	@override String get aikey_hint => 'Guardamos criptografada e nunca mostramos de volta — só os últimos dígitos.';
	@override String get aikey_saved => 'Chave configurada ✓';
	@override String get aikey_saved_hint => 'Já tem chave nesta conta. Pode seguir.';
	@override String get aikey_save => 'Salvar chave';
	@override String get aikey_skip_note => 'Pode seguir sem cadastrar: a lista de preços entra por CSV, que não usa IA. O PDF de catálogo e as ofertas por IA ficam para quando você colar a chave.';
	@override String get products_needs_key => 'Leitura de PDF precisa da chave de IA da conta. Por enquanto, traga a lista em CSV — ela não usa IA.';
	@override String get email_smtp_toggle => 'Tenho servidor próprio (SMTP)';
	@override String get step_channel_other_channels => 'Outros canais';
	@override String get step_channel_other_channels_hide => 'Ocultar outros canais';
	@override String get customers_title => 'Quem já compra de você?';
	@override String get customers_subtitle => 'Traga sua lista de clientes como arquivo. Recompra, cross-sell e a primeira oferta começam daqui — dá pra adicionar mais depois.';
	@override String get first_offer_title => 'Sua primeira oferta';
	@override String get first_offer_writing => 'Lendo seus produtos e seus clientes e escrevendo a primeira mensagem…';
	@override String first_offer_for_customer({required Object name}) => 'Para ${name}, um dos seus clientes:';
	@override String first_offer_for_prospect({required Object name}) => 'Para ${name}, encontrado no seu território:';
	@override String get first_offer_in_queue => 'É assim que as oportunidades vão chegar todo dia. Esta já está na sua lista de Oportunidades — aprove, edite ou peça outra abordagem lá. Nada sai sem você.';
	@override String get first_offer_nothing => 'Ainda não há o que escrever: traga uma lista de clientes, ou diga em que estado você vende, e a primeira oportunidade vai estar esperando em Oportunidades.';
	@override String get first_offer_no_business => 'Não conseguimos encontrar uma empresa no seu território agora. Traga uma lista de clientes, ou tente a busca em Oportunidades mais tarde.';
	@override String get finish_to_queue => 'Ver minha fila de ofertas';
	@override String get unconfirmed_import_title => 'Arquivo não importado';
	@override String get unconfirmed_import_body => 'Você revisou o arquivo mas ainda não confirmou a importação. Se avançar agora, nada será criado.';
	@override String get unconfirmed_import_confirm => 'Avançar sem importar';
	@override String get company_required_hint => 'Os campos com * são obrigatórios — o Próximo libera quando os quatro estiverem preenchidos.';
	@override String get first_offer_import_cta => 'Voltar e importar clientes';
	@override String get first_offer_unavailable => 'A busca de empresas não está disponível agora — importe uma lista de clientes, ou pule: a prospecção funciona normalmente depois.';
	@override String get first_offer_prospecting_not_configured => 'A busca de empresas ainda não está configurada na plataforma — importe uma lista de clientes, ou pule: a prospecção é liberada depois.';
	@override String get first_offer_cta_products => 'Importar produtos';
	@override String get first_offer_cta_customers => 'Corrigir lista de clientes';
	@override String get first_offer_cta_settings => 'Abrir Configurações';
}

// Path: dashboard
class _Translations$dashboard$pt extends Translations$dashboard$en {
	_Translations$dashboard$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get conversations_title => 'Conversas';
	@override String get unknown_contact => 'Cliente';
	@override String reply_reminder_toast({required Object contact, required Object minutes}) => '${contact} está aguardando há ${minutes} min — responda agora';
	@override String get reply_reminder_open => 'Abrir';
	@override String get search_conversations => 'Buscar conversas...';
	@override String get search_in_conversation => 'Buscar na conversa...';
	@override String get no_conversation_selected => 'Selecione uma conversa';
	@override String get no_conversation_selected_subtitle => 'Escolha uma conversa da lista para visualizar as mensagens.';
	@override String get settings => 'Configurações';
	@override String get message_placeholder => 'Digite uma mensagem...';
	@override String get no_conversations_found => 'Nenhuma conversa encontrada';
	@override String get empty_inbox_hint_setup => 'Sua caixa está vazia. Conecte um e-mail para receber e responder as mensagens dos clientes aqui.';
	@override String get empty_inbox_cta_setup => 'Conectar e-mail';
	@override String get empty_inbox_hint_share => 'Sua caixa está vazia. As respostas dos clientes às suas ofertas aparecem aqui.';
	@override String get empty_inbox_cta_share => 'Ver ofertas';
	@override String get no_messages => 'Nenhuma mensagem';
	@override String get today => 'Hoje';
	@override String get yesterday => 'Ontem';
	@override String get now => 'Agora';
	@override String get bot_active => 'IA ativa';
	@override String get waiting_agent => 'Aguardando atendente';
	@override String get closed => 'Encerrada';
	@override String get agent_label => 'Atendente';
	@override String get search_hint => 'Buscar...';
	@override String get close_search => 'Fechar busca';
	@override String get search => 'Buscar';
	@override String get analytics => 'Analytics';
	@override String get settings_tooltip => 'Configurações';
	@override String get search_in_messages => 'Buscar...';
	@override String no_conversations_for_query({required Object query}) => 'Nenhuma conversa com "${query}"';
	@override String results_for_query({required Object count, required Object query}) => '${count} resultado(s) para "${query}"';
	@override String get error_loading_conversations => 'Erro ao carregar conversas';
	@override String get check_connection => 'Verifique sua conexão e tente novamente.';
	@override String get filter_all => 'Todas';
	@override String get filter_open => 'Abertas';
	@override String get filter_pending => 'Pendentes';
	@override String get filter_closed => 'Fechadas';
	@override String get assignment_mine => 'Minhas';
	@override String get assignment_unassigned => 'Não atribuídas';
	@override String get assignment_all => 'Todas';
	@override String get sort_recent => 'Recentes';
	@override String get sort_first_in_conv => 'Ordem de chegada';
	@override String get sort_oldest => 'Mais antigas';
	@override String get select_conversation_title => 'Selecione uma conversa';
	@override String get select_conversation_subtitle => 'Escolha uma conversa da lista para\nvisualizar as mensagens.';
	@override String get drop_to_send => 'Solte para enviar';
	@override String get search_messages_hint => 'Buscar...';
	@override String get summary_tooltip => 'Resumo da conversa (IA)';
	@override String get send_file_tooltip => 'Enviar arquivo';
	@override String get record_audio_tooltip => 'Gravar áudio';
	@override String get send_flow_tooltip => 'Enviar Flow';
	@override String get type_a_message => 'Digite uma mensagem...';
	@override String get bot_paused_pill => 'IA pausada';
	@override String get human_handling => 'Em atendimento humano';
	@override String get status_open_bot => 'IA ativa';
	@override String get status_pending_human => 'Aguardando atendente';
	@override String get status_closed => 'Encerrada';
	@override String get day_today => 'Hoje';
	@override String get day_yesterday => 'Ontem';
	@override String get summary_title => 'Resumo da conversa';
	@override String summary_failed({required Object error}) => 'Falha ao gerar resumo: ${error}';
	@override String get summary_empty => 'Sem mensagens suficientes para resumir.';
	@override String get summary_refresh => 'Atualizar';
	@override String get wa_24h_window_expired => 'Janela de 24h expirou. Envie um template aprovado para reabrir a conversa.';
	@override String get wa_send_template => 'Enviar template';
	@override String get retry_send => 'Tentar';
	@override String flow_sent({required Object name}) => 'Flow "${name}" enviado.';
	@override String token_usage_warning_snackbar({required Object percent}) => 'Uso de tokens em ${percent}% do limite.';
	@override String get reconnecting => 'Reconectando...';
	@override String get tokens_exhausted_banner => 'Créditos de IA esgotados. Adquira extras ou faça upgrade.';
	@override String tokens_high_usage_banner({required Object percent}) => 'Você já usou ${percent}% dos seus créditos de IA este mês.';
	@override String get upgrade_btn => 'Falar com a gente';
	@override String get trial_ended => 'Seu período de teste terminou. Faça upgrade para continuar usando o Replider.';
	@override String get trial_last_day => 'Último dia de teste gratuito. Faça upgrade para não interromper o atendimento.';
	@override String trial_days_remaining({required Object days, required Object used, required Object max}) => 'Restam ${days} dias do seu teste gratuito (${used}/${max} conversas). Faça upgrade para desbloquear mais.';
	@override String trial_free_plan({required Object used, required Object max}) => 'Você está em período de teste (${used} de ${max} conversas usadas). Quer o produto completo rodando na sua empresa? Vamos conversar.';
	@override String get insert_signature_tooltip => 'Inserir assinatura';
	@override String get channel_all => 'Todos os canais';
	@override String get channel_email => 'E-mail';
	@override String get channel_whatsapp => 'WhatsApp';
	@override String get channel_telegram => 'Telegram';
	@override String get channel_instagram => 'Instagram';
	@override String get channel_sms => 'SMS';
	@override String get sort_importance => 'Mais importante';
	@override String get category_all => 'Todas as categorias';
	@override String get category_hot_lead => 'Lead quente';
	@override String get category_customer => 'Cliente';
	@override String get category_financial => 'Financeiro';
	@override String get category_supplier => 'Fornecedor';
	@override String get category_newsletter => 'Newsletter';
	@override String get category_quarantine => 'Quarentena';
	@override String get category_other => 'Outros';
	@override String get filters_button => 'Filtros';
	@override String filters_button_n({required Object count}) => 'Filtros · ${count}';
	@override String get filters_clear => 'Limpar filtros';
	@override String get filters_title => 'Filtrar conversas';
	@override String get filter_label_status => 'Situação';
	@override String get filter_label_channel => 'Canal';
	@override String get filter_label_sort => 'Ordem';
	@override String get filter_label_category => 'Categoria';
	@override String get no_conversations_for_filters => 'Nenhuma conversa com estes filtros.';
	@override String get clear_filters => 'Limpar filtros';
	@override String get media_file_label => 'Arquivo';
	@override String get media_location_label => 'Localização';
	@override String get wa_unsupported_title => 'Conteúdo não entregue pelo WhatsApp';
	@override String get wa_unsupported_body => 'Enquete, localização ao vivo ou visualização única não chegam ao atendimento. Peça para o cliente reenviar de outra forma.';
	@override String get wa_window_composer_back => 'O campo de mensagem volta quando o cliente responder.';
	@override String get brief_agreed => 'ACORDADO';
	@override String get brief_pending => 'PENDENTE';
	@override String get brief_next_step => 'PRÓXIMO PASSO';
	@override String get brief_generated_now => 'gerado agora';
	@override String brief_generated_ago({required Object when}) => 'gerado há ${when}';
	@override String get brief_copied => 'Resumo copiado.';
	@override String get summary_too_short => 'Esta conversa ainda é curta — leia as mensagens, o resumo seria maior que a própria thread.';
}

// Path: email_verification
class _Translations$email_verification$pt extends Translations$email_verification$en {
	_Translations$email_verification$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Verifique seu email';
	@override String get subtitle => 'Enviamos um link de verificação para';
	@override String get instruction => 'Clique no link enviado para ativar sua conta. O link expira em 24 horas.';
	@override String get back_to_login => 'Voltar para o login';
	@override String get didnt_receive => 'Não recebeu o e-mail?';
	@override String get resend => 'Reenviar e-mail';
	@override String get resent => 'E-mail enviado. Confira a caixa em alguns instantes.';
	@override String get resend_failed => 'Não conseguimos reenviar. Tente novamente em um minuto.';
	@override String get check_spam => 'Verifique também a pasta de spam.';
	@override String get banner_message => 'Confirme seu e-mail para garantir o acesso à conta.';
	@override String get verifying => 'Verificando seu e-mail…';
	@override String get verified_title => 'E-mail verificado';
	@override String get verified_subtitle => 'Seu e-mail foi confirmado. Você já pode entrar.';
	@override String get verify_failed_title => 'Falha na verificação';
	@override String get verify_failed_subtitle => 'Este link é inválido ou expirou. Solicite um novo pelo app.';
}

// Path: settings
class _Translations$settings$pt extends Translations$settings$en {
	_Translations$settings$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Configurações';
	@override String get account => 'Conta';
	@override String get subscription => 'Assinatura';
	@override String get team => 'Equipe';
	@override String get manage_team => 'Gerenciar Equipe';
	@override String get manage_team_subtitle => 'Convide membros e gerencie cargos';
	@override String get ai_agents => 'Agentes de IA';
	@override String get kNew => 'Novo';
	@override String get no_agents => 'Nenhum agente configurado';
	@override String get no_agent_selected => 'Agente não selecionado';
	@override String get error_loading_team => 'Erro ao carregar equipe';
	@override String get error_loading_tags => 'Erro ao carregar tags';
	@override String get error_loading_responses => 'Erro ao carregar respostas';
	@override String get error_loading_calendars => 'Erro ao carregar calendários';
	@override String get error_loading_templates => 'Erro ao carregar templates';
	@override String get error_loading_webhooks => 'Falha ao carregar';
	@override String get agent_not_found => 'Agente não encontrado';
	@override String get create_agent => 'Criar Agente';
	@override String get active => 'Ativo';
	@override String get inactive => 'Inativo';
	@override String get error_loading_agents => 'Erro ao carregar agentes';
	@override String get retry => 'Tentar novamente';
	@override String get logout => 'Sair da conta';
	@override String get plan_and_usage => 'Plano e Uso';
	@override String get manage_plan => 'Gerencie seu plano e acompanhe o uso';
	@override String get delete_agent_title => 'Excluir Agente';
	@override String delete_agent_confirm({required Object name}) => 'Deseja realmente excluir "${name}"? Esta ação não pode ser desfeita.';
	@override String get cancel => 'Cancelar';
	@override String get delete => 'Excluir';
	@override String get agent_deleted => 'Agente excluído com sucesso';
	@override String get new_agent => 'Novo Agente';
	@override String get agent_name => 'Nome do agente';
	@override String get ai_provider => 'Provedor de IA';
	@override String get create => 'Criar';
	@override String get agent_created => 'Agente criado com sucesso!';
	@override String get delete_agent_tooltip => 'Excluir agente';
	@override String get no_channels => 'Nenhum canal conectado';
	@override String get no_messages => 'Nenhuma mensagem';
	@override String get typing => 'digitando...';
	@override String get account_subtitle => 'Nome da empresa, workspace e zona de perigo';
	@override String get company_section_title => 'Empresa';
	@override String get company_name_label => 'Nome da empresa';
	@override String get company_name_helper => 'É assim que o agente de IA se apresenta e como seu workspace aparece para o time.';
	@override String get company_save_success => 'Nome da empresa atualizado';
	@override String get company_save_cta => 'Salvar';
	@override String get branding_section_title => 'Identidade visual';
	@override String get branding_section_subtitle => 'Logo e fuso horário usados pelo agente.';
	@override String get branding_logo_label => 'Logo do workspace';
	@override String get branding_logo_helper => 'PNG ou JPG, tamanho máximo 2 MB. Aparece no cabeçalho e em canais futuros.';
	@override String get branding_logo_upload_cta => 'Enviar logo';
	@override String get branding_logo_change_cta => 'Substituir';
	@override String get branding_logo_remove_cta => 'Remover';
	@override String get branding_logo_uploading => 'Enviando logo...';
	@override String get branding_logo_too_large => 'Logo acima de 2 MB';
	@override String get branding_logo_upload_failed => 'Falha ao enviar logo';
	@override String get branding_timezone_label => 'Fuso horário padrão';
	@override String get branding_timezone_helper => 'Usado como padrão ao criar novos agentes. Agentes existentes mantêm o fuso atual.';
	@override String get branding_save_cta => 'Salvar branding';
	@override String get branding_save_success => 'Branding atualizado';
	@override String get danger_zone_title => 'Zona de perigo';
	@override String get danger_zone_subtitle => 'Ações irreversíveis no workspace.';
	@override String get delete_account_cta => 'Excluir conta permanentemente';
	@override String delete_account_dialog_title({required Object company}) => 'Excluir ${company}?';
	@override String get delete_account_dialog_body => 'Isso apaga todos os agentes, canais, conversas e mensagens permanentemente. Não dá pra desfazer. Digite o nome da empresa abaixo pra confirmar.';
	@override String delete_account_type_hint({required Object company}) => 'Digite "${company}" para liberar';
	@override String get delete_account_confirm_button => 'Excluir workspace';
	@override String get delete_account_success => 'Conta excluída.';
	@override String get agents_subtitle => 'Gerencie os agentes de IA que respondem aos clientes';
	@override String get channels_subtitle => 'E-mail para ofertas · WhatsApp para relacionamento';
	@override String get tags_subtitle => 'Organize conversas por tema ou prioridade';
	@override String get canned_responses_subtitle => 'Respostas rápidas que os atendentes podem reutilizar';
	@override String get campaigns_subtitle => 'Templates de envio (WhatsApp)';
	@override String get compliance_subtitle => 'LGPD e log de auditoria';
	@override String get smtp_test_title => 'Teste de SMTP';
	@override String get smtp_test_subtitle => 'Envia um e-mail pra você confirmar que o servidor consegue entregar mensagens.';
	@override String get smtp_test_cta => 'Enviar e-mail de teste';
	@override String get smtp_test_success => 'E-mail de teste enviado. Confira sua caixa.';
	@override String get smtp_test_not_configured => 'SMTP não está configurado no servidor.';
	@override String get smtp_test_failed => 'Falha ao enviar e-mail';
	@override String get wa_templates_pending_banner_one => '1 template em análise pela Meta';
	@override String wa_templates_pending_banner_other({required Object count}) => '${count} templates em análise pela Meta';
	@override String get wa_templates_polling_hint => 'Atualizando automaticamente a cada 30 segundos.';
	@override String get wa_phone_numbers_title => 'Números no WABA';
	@override String get wa_phone_numbers_subtitle => 'Todos os números registrados na sua WhatsApp Business Account. Cada número pode virar um canal separado no Replider.';
	@override String get wa_phone_numbers_empty => 'Nenhum outro número encontrado neste WABA.';
	@override String get wa_phone_numbers_failed => 'Falha ao listar números';
	@override String get wa_phone_numbers_current => 'Conectado agora';
	@override String get audit_filter_action => 'Ação (exata)';
	@override String get audit_filter_entity => 'Tipo de entidade';
	@override String get audit_filter_actor => 'ID do autor';
	@override String get audit_filter_apply => 'Aplicar';
	@override String get audit_filter_clear => 'Limpar';
	@override String get audit_export_cta => 'Exportar CSV';
	@override String get audit_export_copied => 'CSV copiado. Cole num arquivo .csv e abra no Excel.';
	@override String get audit_export_failed => 'Falha ao exportar CSV';
	@override String get calendar_test_connection_cta => 'Testar conexão';
	@override String get calendar_test_connection_ok => 'Conectado com sucesso';
	@override String get calendar_test_connection_fail => 'Falha de conexão';
	@override String get calendar_disconnect_cta => 'Desconectar do Google';
	@override String calendar_disconnect_confirm_title({required Object name}) => 'Desconectar ${name} do Google?';
	@override String get calendar_disconnect_confirm_body => 'O calendário continuará existindo no Replider, mas o agente não vai mais consultar eventos do Google. Você pode reconectar depois editando o calendário.';
	@override String get calendar_disconnect_confirm_cta => 'Desconectar';
	@override String get calendar_disconnect_success => 'Desconectado do Google.';
	@override String get gcal_title => 'Google Calendar';
	@override String get gcal_connect_desc => 'Conecte sua conta Google para o agente consultar horários livres e criar eventos direto na sua agenda.';
	@override String get gcal_connect_cta => 'Conectar Google Calendar';
	@override String gcal_connected_as({required Object email}) => 'Conectado como ${email}';
	@override String get gcal_disconnect_cta => 'Desconectar';
	@override String get gcal_disconnect_confirm_title => 'Desconectar o Google Calendar?';
	@override String get gcal_disconnect_confirm_body => 'O agente deixa de acessar sua agenda. Agendamentos voltam a depender da agenda compartilhada com a plataforma.';
	@override String get gcal_open_browser => 'Continue a conexão na aba que abrimos no navegador. Volte aqui quando terminar.';
	@override String get gcal_connected_snackbar => 'Google Calendar conectado!';
	@override String get gcal_error_snackbar => 'Falha ao conectar o Google Calendar. Tente de novo.';
	@override String get gcal_denied_snackbar => 'Conexão com o Google cancelada.';
	@override String get gcal_pick_calendar_label => 'Agenda do Google';
	@override String get gcal_pick_calendar_hint => 'Escolha em qual agenda os compromissos serão marcados';
	@override String get gcal_manual_id_toggle => 'Inserir ID manualmente';
	@override String get team_metrics_period_today => 'Hoje';
	@override String get team_metrics_period_7d => '7d';
	@override String get team_metrics_period_30d => '30d';
	@override String get team_metrics_period_mtd => 'Mês';
	@override String get team_metrics_col_convs => 'Convs';
	@override String get team_metrics_col_tmpr => 'TMPR';
	@override String get team_metrics_col_csat => 'CSAT';
	@override String get team_metrics_status_online => 'Online';
	@override String get team_metrics_status_offline => 'Offline';
	@override String get team_metrics_export_cta => 'Exportar CSV';
	@override String get team_metrics_export_copied => 'CSV copiado. Cole num arquivo .csv e abra no Excel.';
	@override String get team_metrics_export_failed => 'Falha ao exportar';
	@override String team_metrics_drill_title({required Object name}) => 'Métricas de ${name}';
	@override String get team_metrics_card_volume => 'Volume';
	@override String get team_metrics_card_speed => 'Velocidade';
	@override String get team_metrics_card_quality => 'Qualidade';
	@override String get team_metrics_card_activity => 'Atividade';
	@override String get team_metrics_conversations_assigned => 'Conversas atribuídas';
	@override String get team_metrics_conversations_resolved => 'Resolvidas';
	@override String get team_metrics_messages_sent => 'Mensagens enviadas';
	@override String get team_metrics_avg_first_response => 'Tempo médio de 1ª resposta';
	@override String get team_metrics_avg_resolution => 'Tempo médio de resolução';
	@override String get team_metrics_csat_avg => 'CSAT médio';
	@override String get team_metrics_csat_count => 'Respostas';
	@override String get team_metrics_last_seen => 'Última atividade';
	@override String get team_metrics_empty => 'Nenhum dado no período selecionado.';
	@override String get team_metrics_no_data => '—';
	@override String get live_supervision_online => 'Online';
	@override String get live_supervision_offline => 'Offline';
	@override String get channels_title => 'E-mail e canais';
	@override String get business_hours_title => 'Horário de atendimento';
	@override String get business_hours_enabled => 'Responder só dentro do horário';
	@override String get business_hours_enabled_subtitle => 'Fora do horário a IA fica quieta — atendentes ainda podem responder manualmente.';
	@override String get business_hours_start => 'Início';
	@override String get business_hours_end => 'Fim';
	@override String get business_hours_days => 'Dias da semana';
	@override String get business_hours_save_success => 'Horário salvo';
	@override String get back_tooltip => 'Voltar';
	@override String get plan_expired_banner => 'Seu contrato está inativo — envios e edições estão pausados. Fale com a gente para reativar.';
	@override String get view_plans => 'Ver planos';
	@override String channels_count_offline({required Object count}) => '${count} offline';
	@override String get section_channels => 'Canais';
	@override String get section_attendance => 'Atendimento';
	@override String get section_compliance => 'Conformidade';
	@override String get campaigns_title => 'Campanhas';
	@override String get flows_title => 'Flows do WhatsApp';
	@override String get flows_subtitle => 'Formulários nativos para captura de leads e agendamentos.';
	@override String get webhooks_title => 'Webhooks';
	@override String get webhooks_subtitle => 'Integre com Zapier, n8n, CRM ou seu próprio backend.';
	@override String get compliance_title => 'LGPD & Auditoria';
	@override String get logout_title => 'Sair';
	@override String get logout_subtitle => 'Encerra esta sessão';
	@override String get logout_confirm_title => 'Sair da conta';
	@override String get logout_confirm_body => 'Tem certeza? Você precisará entrar novamente para acessar o painel.';
	@override String get trial_expired_title => 'Trial expirado';
	@override String get plan_inactive_title => 'Plano inativo';
	@override String get plan_view_plans => 'Ver planos';
	@override String plan_label({required Object name}) => 'Plano: ${name}';
	@override String tokens_conversations_usage({required Object tokens, required Object conv}) => 'Tokens: ${tokens}% | Conversas: ${conv}%';
	@override String get agent_name_placeholder => 'Ex: Assistente de Vendas';
	@override String get ai_provider_label => 'Provedor';
	@override String get agent_active_title => 'Agente Ativo';
	@override String get agent_active_subtitle => 'O agente está respondendo mensagens';
	@override String get agent_paused_subtitle => 'O agente está pausado';
	@override String get agent_test_title => 'Testar Agente';
	@override String get agent_test_subtitle => 'Inicie uma conversa de teste com este agente';
	@override String get agent_calendars_title => 'Calendários';
	@override String get agent_calendars_subtitle => 'Google Calendar para agendamentos';
	@override String get agent_kb_title => 'Base de Conhecimento';
	@override String get agent_kb_subtitle => 'Textos e PDFs para inteligência artificial';
	@override String get agent_section_name => 'NOME DO AGENTE';
	@override String get agent_section_ai => 'INTELIGÊNCIA ARTIFICIAL';
	@override String get agent_section_prompt => 'PROMPT DO SISTEMA';
	@override String get agent_section_personality => 'PERSONALIDADE APRENDIDA';
	@override String get agent_section_business_hours => 'HORÁRIO DE ATENDIMENTO';
	@override String get agent_section_behavior => 'COMPORTAMENTO';
	@override String get agent_section_info => 'INFORMAÇÕES ADICIONAIS';
	@override String get agent_prompt_helper_text => 'Defina a personalidade e as instruções do agente de IA.';
	@override String agent_prompt_library_match({required Object name}) => 'Usando o prompt salvo: ${name}';
	@override String get agent_prompt_library_open => 'Abrir biblioteca';
	@override String get agent_prompt_hint => 'Você é um assistente de atendimento ao cliente da empresa X. Seu objetivo é ajudar os clientes com dúvidas sobre produtos e pedidos...';
	@override String get agent_save => 'Salvar';
	@override String get agent_saved => 'Salvo';
	@override String get upgrade_plan_action => 'Atualizar plano';
	@override String get agent_not_found_message => 'Agente não encontrado';
	@override String get agent_updated_success => 'Agente atualizado com sucesso!';
	@override String get personality_empty_hint => 'Nenhuma personalidade aprendida ainda. Responda manualmente algumas conversas e o bot passa a imitar o estilo do seu time.';
	@override String get personality_active_hint => 'Personalidade derivada das respostas humanas mais recentes. É aplicada automaticamente ao bot.';
	@override String get personality_updated => 'Personalidade atualizada.';
	@override String get bh_limit_label => 'Limitar resposta automática a um horário';
	@override String get bh_explainer => 'O bot só responde dentro deste horário. Fora da janela a conversa fica para um humano atender.';
	@override String get bh_start => 'Início';
	@override String get bh_end => 'Fim';
	@override String get bh_time_placeholder => '--:--';
	@override String get bh_dom_d => 'D';
	@override String get bh_d_sun => 'D';
	@override String get bh_d_mon => 'S';
	@override String get bh_d_tue => 'T';
	@override String get bh_d_wed => 'Q';
	@override String get bh_d_thu => 'Q';
	@override String get bh_d_fri => 'S';
	@override String get bh_d_sat => 'S';
	@override String get behavior_hide_ai_title => 'Estilo humano (não anuncia que é IA)';
	@override String get behavior_hide_ai_subtitle => 'Bot escreve como um atendente humano e não diz espontaneamente que é IA. Se o cliente perguntar diretamente, responde com honestidade — exigência da política do WhatsApp (Meta).';
	@override String get behavior_silent_title => 'Aguardar humano quando não souber';
	@override String get behavior_silent_subtitle => 'Quando o bot não tiver a resposta, ele fica em silêncio e passa a conversa pro atendente sem avisar o cliente. Desligue para o bot dizer "vou verificar" no idioma do cliente.';
	@override String get agent_id_label => 'ID';
	@override String get agent_timezone_label => 'Timezone';
	@override String get agent_created_label => 'Criado em';
	@override String get channels_back_tooltip => 'Voltar';
	@override String get channels_screen_title => 'Canais';
	@override String get channels_add_tooltip => 'Adicionar canal';
	@override String get channels_empty_subtitle => 'Conecte o e-mail que você já usa para as ofertas saírem, e o WhatsApp para o relacionamento.';
	@override String get channel_dialog_edit_title => 'Editar canal';
	@override String get channel_dialog_add_title => 'Adicionar canal';
	@override String get channel_dialog_edit_subtitle => 'Atualize nome, credenciais ou agente responsável.';
	@override String get channel_dialog_add_subtitle => 'Por onde suas ofertas saem e as respostas chegam.';
	@override String get channel_name_label => 'Nome do canal';
	@override String get channel_name_hint => 'Ex: E-mail de vendas';
	@override String get channel_or_manual => 'ou conecte manualmente';
	@override String get channel_responsible_agent => 'Agente responsável';
	@override String get channel_no_agent_human => 'Nenhum (atendimento humano)';
	@override String get channel_save_btn => 'Salvar';
	@override String get channel_connect_btn => 'Conectar';
	@override String get channel_partial_credentials => 'Preencha todos os campos pra trocar credenciais, ou deixe todos em branco pra manter as atuais.';
	@override String get channel_invalid_phone_id => 'Phone Number ID inválido (deve ter 12-16 dígitos numéricos).';
	@override String get channel_invalid_waba_id => 'WABA ID inválido (deve ter 12-18 dígitos numéricos).';
	@override String get channel_invalid_token => 'Access Token muito curto. Cole o token completo da Meta.';
	@override String get channel_invalid_account_sid => 'Account SID inválido (deve começar com AC seguido de 32 caracteres hexadecimais).';
	@override String get channel_invalid_from_number => 'Número de telefone inválido. Use o formato E.164, ex.: +15551234567.';
	@override String get channel_updated_success => 'Canal atualizado com sucesso!';
	@override String get channel_created_success => 'Canal criado com sucesso!';
	@override String get channel_deleted_success => 'Canal excluído';
	@override String channel_delete_confirm({required Object name}) => 'Deseja excluir "${name}"? As mensagens recebidas por este canal serão interrompidas.';
	@override String get channel_status_connected => 'Conectado';
	@override String get channel_status_disabled => 'Desativado';
	@override String get channel_status_incomplete => 'Incompleto';
	@override String get channel_edit_tooltip => 'Editar';
	@override String get channel_test_btn => 'Testar Conexão';
	@override String get channel_testing => 'Testando...';
	@override String get channel_configure_btn => 'Configurar';
	@override String get channel_snippet_btn => 'Snippet';
	@override String channel_test_success_named({required Object label}) => 'Conectado! ${label}';
	@override String get channel_test_success => 'Conexão OK!';
	@override String channel_test_failed({required Object error}) => 'Falha: ${error}';
	@override String get channel_test_unknown_error => 'Erro desconhecido';
	@override String get channel_reconnect_action => 'Reconectar';
	@override String channel_google_connected({required Object address}) => 'Conectado pelo Google: ${address}';
	@override String get channel_google_edit_hint => 'Esta caixa foi conectada por consentimento — não há servidor nem senha para editar. Se o envio ou a leitura pararem, conecte de novo.';
	@override String get channel_google_reconnect => 'Reconectar pelo Google';
	@override String channel_imap_failing({required Object count, required Object error}) => 'A leitura das respostas falhou ${count} vez(es): ${error}';
	@override String get channel_webhook_re_registered => 'Webhook re-registrado.';
	@override String channel_webhook_register_failed({required Object error}) => 'Falha ao re-registrar webhook: ${error}';
	@override String get channels_select_telegram_badge => '2 min';
	@override String get channels_select_popular_badge => 'Popular';
	@override String get channels_select_coming_soon => 'Em breve';
	@override String get channel_instagram_help_link => 'Como obter seu Page ID e token de acesso';
	@override String get channel_sms_help_link => 'Como obter um número de SMS compatível (guia Twilio)';
	@override String get channel_webhook_url_label => 'URL de webhook (Twilio)';
	@override String get channel_webhook_url_help => 'Cole esta URL como webhook de "A message comes in" do seu número no console da Twilio.';
	@override String get channel_webhook_url_copied => 'URL de webhook copiada';
	@override String get channel_webhook_url_btn => 'URL de webhook';
	@override String get wa_settings_back_tooltip => 'Voltar';
	@override String get wa_settings_title => 'Configurações WhatsApp';
	@override String get wa_settings_tab_profile => 'Perfil';
	@override String get wa_settings_tab_templates => 'Templates';
	@override String get wa_section_diagnostics => 'DIAGNÓSTICO';
	@override String get wa_section_phones => 'NÚMEROS NO WABA';
	@override String get wa_section_business => 'SOBRE O NEGÓCIO';
	@override String get wa_field_about => 'Sobre';
	@override String get wa_field_about_hint => 'Breve descrição (aparece no perfil)';
	@override String get wa_field_description => 'Descrição completa';
	@override String get wa_field_description_hint => 'Descrição detalhada do seu negócio';
	@override String get wa_field_email => 'Email';
	@override String get wa_field_email_hint => 'contato@empresa.com';
	@override String get wa_field_address => 'Endereço';
	@override String get wa_field_address_hint => 'Rua Exemplo, 123 — Cidade/UF';
	@override String get wa_field_site_one => 'Site 1';
	@override String get wa_field_site_one_hint => 'https://...';
	@override String get wa_field_site_two => 'Site 2';
	@override String get wa_field_site_two_hint => 'https://... (opcional)';
	@override String get wa_profile_helper => 'Estes dados aparecem publicamente no perfil do seu WhatsApp Business.';
	@override String get wa_profile_save_btn => 'Salvar alterações';
	@override String get wa_profile_saving => 'Salvando...';
	@override String get wa_profile_saved_success => 'Perfil atualizado com sucesso!';
	@override String get wa_picture_replace => 'Trocar foto';
	@override String get wa_picture_set => 'Alterar foto';
	@override String get wa_picture_apply_hint => 'A foto será aplicada ao clicar em "Salvar alterações"';
	@override String get wa_picture_remove_button => 'Remover foto cacheada';
	@override String get wa_picture_remove_dialog_title => 'Remover foto cacheada?';
	@override String get wa_picture_remove_dialog_body => 'Apaga a cópia local da foto de perfil. A foto no número do WhatsApp em si permanece — para removê-la lá, use o Meta Business Manager. Use isso quando a Meta parar de retornar a foto para o dashboard parar de servir a antiga.';
	@override String get wa_picture_remove_dialog_confirm => 'Remover';
	@override String get wa_picture_removed_success => 'Foto cacheada removida.';
	@override String get wa_templates_no_templates_subtitle => 'Templates são mensagens pré-aprovadas pela Meta. Você precisa deles para iniciar conversas com clientes fora da janela de 24h.';
	@override String get wa_templates_create_loading_error_title => 'Conexão com a Meta precisa ser renovada';
	@override String get wa_templates_visualize_tooltip => 'Visualizar';
	@override String get wa_templates_delete_tooltip => 'Excluir';
	@override String wa_templates_delete_confirm({required Object name}) => 'Excluir "${name}"? Esta ação também remove o template na Meta.';
	@override String get wa_templates_delete_success => 'Template excluído';
	@override String wa_templates_rejected_reason({required Object reason}) => 'Motivo: ${reason}';
	@override String get wa_templates_no_examples_warning => 'Sem exemplos gravados — as variáveis aparecem como placeholders.';
	@override String get wa_create_dialog_title => 'Novo template';
	@override String get wa_create_section_name => '1. NOME TÉCNICO';
	@override String get wa_create_name_hint => 'ex: boas_vindas';
	@override String get wa_create_name_helper => 'Apenas letras minúsculas, números e underscore. Não aparece para o cliente.';
	@override String get wa_create_section_category => '2. CATEGORIA';
	@override String get wa_create_section_language => '3. IDIOMA';
	@override String get wa_create_section_header => 'CABEÇALHO (opcional)';
	@override String get wa_create_section_message => '4. MENSAGEM';
	@override String get wa_create_message_hint => 'Olá {{1}}, seu pedido #{{2}} foi confirmado! 🎉';
	@override String get wa_create_message_helper => 'Use {{1}}, {{2}}... para variáveis. Formatação: *negrito*, _itálico_, ~tachado~, `mono` (WhatsApp usa 1 asterisco, não 2).';
	@override String get wa_create_double_asterisk_warn => 'WhatsApp usa *texto* (um asterisco), não **texto**. Corrija antes de enviar.';
	@override String get wa_create_section_examples => '5. EXEMPLOS PARA VARIÁVEIS';
	@override String wa_create_example_for({required Object n}) => 'Exemplo para {{${n}}}';
	@override String get wa_create_example_first => 'João Silva';
	@override String get wa_create_example_other => 'Valor';
	@override String get wa_create_examples_helper => 'A Meta usa estes exemplos para analisar o template.';
	@override String get wa_create_section_footer => 'RODAPÉ (opcional)';
	@override String get wa_create_footer_hint => 'ex: Responda PARAR para sair';
	@override String get wa_create_section_buttons => 'BOTÕES (opcional, máx. 3)';
	@override String get wa_create_section_preview => 'PRÉ-VISUALIZAÇÃO';
	@override String get wa_create_submit => 'Enviar para aprovação';
	@override String get wa_create_submitting => 'Enviando...';
	@override String get wa_create_validation_name => 'Nome deve conter apenas letras minúsculas, números e _.';
	@override String get wa_create_validation_body_required => 'Mensagem é obrigatória.';
	@override String get wa_create_validation_double_asterisk => 'Remova os ** do texto — WhatsApp usa *texto* (um asterisco) para negrito.';
	@override String get wa_create_validation_no_leading_var => 'Texto não pode começar com variável. Adicione uma palavra antes de {{1}} (ex: "Olá {{1}}").';
	@override String get wa_create_validation_no_trailing_var => 'Texto não pode terminar com variável. Adicione uma palavra ou pontuação depois da última {{N}}.';
	@override String get wa_create_validation_examples_required => 'Preencha os exemplos para as variáveis.';
	@override String get wa_create_validation_header_empty => 'Cabeçalho de texto não pode ficar vazio.';
	@override String get wa_create_validation_header_too_long => 'Cabeçalho de texto excede 60 caracteres.';
	@override String get wa_create_validation_header_one_var => 'Cabeçalho aceita no máximo uma variável.';
	@override String get wa_create_validation_header_example => 'Preencha o exemplo do cabeçalho.';
	@override String get wa_create_validation_media_uploading => 'Aguarde o upload do cabeçalho terminar.';
	@override String get wa_create_validation_media_required => 'Selecione e envie um arquivo para o cabeçalho.';
	@override String get wa_create_validation_button_text_required => 'Todos os botões precisam de texto.';
	@override String get wa_create_validation_button_url => 'Botões de URL precisam de link.';
	@override String get wa_create_validation_button_phone => 'Botões de telefone precisam de número E.164 (ex: +5511999999999).';
	@override String get wa_create_warning_promo_for_utility => 'Texto parece promocional ("novidades", "oferta"...). Meta costuma rejeitar UTILITY com esse conteúdo — troque para MARKETING.';
	@override String get wa_create_warning_tx_for_marketing => 'Texto parece transacional ("confirmação", "pedido"...). Considere UTILITY para reduzir chance de rejeição.';
	@override String get wa_category_marketing => 'Promoções e ofertas';
	@override String get wa_category_utility => 'Confirmações e atualizações';
	@override String get wa_lang_pt_br => 'Português (Brasil)';
	@override String get wa_lang_en_us => 'Inglês (EUA)';
	@override String get wa_lang_es => 'Espanhol';
	@override String get wa_header_none => 'Sem cabeçalho';
	@override String get wa_header_text => 'Texto';
	@override String get wa_header_image => 'Imagem';
	@override String get wa_header_video => 'Vídeo';
	@override String get wa_header_document => 'Documento';
	@override String get wa_header_text_hint => 'ex: Confirmação do pedido {{1}}';
	@override String get wa_header_text_helper => 'Máx. 60 caracteres. Aceita até uma única variável {{1}}.';
	@override String get wa_header_example_label => 'Exemplo para {{1}} no cabeçalho';
	@override String get wa_header_example_hint => '#12345';
	@override String get wa_header_pick_file => 'Selecionar arquivo';
	@override String get wa_header_uploading => 'Enviando...';
	@override String get wa_header_remove_tooltip => 'Remover';
	@override String get wa_header_upload_failed_processing => 'Falha ao processar o arquivo no Meta. Tente outro.';
	@override String wa_header_upload_failed_generic({required Object error}) => 'Falha no upload: ${error}';
	@override String get wa_button_text_label => 'Texto do botão (máx. 25)';
	@override String get wa_button_url_label => 'URL';
	@override String get wa_button_url_hint => 'https://...';
	@override String get wa_button_phone_label => 'Número (E.164)';
	@override String get wa_button_phone_hint => '+5511999999999';
	@override String get wa_button_call => 'Ligar para número';
	@override String get wa_preview_message_placeholder => 'Sua mensagem aparece aqui...';
	@override String get wa_preview_button_placeholder => 'Texto do botão';
	@override String get wa_template_submit_success => 'Template enviado! A análise da Meta costuma levar até 24 horas.';
	@override String get wa_test_template_explain => 'Dispara uma mensagem real para validar a conexão end-to-end. Útil para a gravação de tela do App Review da Meta.';
	@override String get wa_diag_phone_not_registered_no_platform => 'Meta não confirmou o registro do número. Sem isso, mensagens de entrada não chegam ao Replider.';
	@override String wa_diag_phone_not_registered_with_platform({required Object platform}) => 'platform_type = ${platform}. Registre o número para receber mensagens.';
	@override String get wa_diag_cooldown_title => 'Bloqueado';
	@override String wa_diag_cooldown_snackbar({required Object time}) => 'Aguarde ${time} antes da próxima tentativa. Cada clique renova o bloqueio da Meta.';
	@override String get wa_diag_atualizar_tooltip => 'Atualizar';
	@override String get csat_auto_title => 'CSAT automático';
	@override String get csat_auto_subtitle => 'Perguntar de 1 a 5 após conversa inativa';
	@override String team_metrics_relative_min({required Object count}) => '${count} min';
	@override String team_metrics_relative_hours({required Object count}) => '${count} h';
	@override String team_metrics_relative_days({required Object count}) => '${count} d';
	@override String team_metrics_relative_weeks({required Object count}) => '${count} sem';
	@override String get csat_auto_helper => 'Enviado na língua detectada da conversa (pt/en/es). Desligue em fluxos B2B onde o atendente já debrifa manualmente.';
	@override String get reply_reminder_title => 'Lembrete de resposta';
	@override String get reply_reminder_subtitle => 'Nunca esqueça um cliente aguardando';
	@override String get reply_reminder_helper => 'Quando a IA está pausada e o cliente espera mais do que o limite configurado, todo membro da equipe com push ativado recebe alerta no celular e no painel. Escala em 1x, 2x e 4x o limite, depois para.';
	@override String get reply_reminder_threshold_label => 'Avisar após';
	@override String get claude_title => 'Claude (IA)';
	@override String get claude_subtitle => 'Controle sua conta por conversa com o Claude (MCP).';
	@override String get brand_tile => 'Identidade da empresa';
	@override String get brand_tile_sub => 'Voz, o que pode e o que nunca pode ser dito';
	@override String get templates_tile => 'Modelos de e-mail';
	@override String get templates_tile_sub => 'Textos prontos de e-mail, com quanto cada um responde';
	@override String get suppressions_tile => 'Endereços bloqueados';
	@override String get suppressions_tile_sub => 'Quem parou de receber seus e-mails, e por quê';
	@override String get autopilot_tile_sub => 'Quando abordagens podem sair sem sua aprovação';
	@override String tokens_conversations_used({required Object tokens, required Object conv}) => 'Tokens: ${tokens} | Conversas: ${conv}';
	@override String get contacts_subtitle => 'Quem já falou com você, num só lugar';
	@override String get analytics_subtitle => 'Volume, tempo de resposta e desempenho da IA';
	@override String get preview_handoff_note => 'O agente passaria a conversa para uma pessoa aqui. Numa conversa de verdade, o bot é pausado e alguém do time assume — no teste não há para quem passar.';
	@override String channel_test_email_success({required Object to, required Object cap}) => 'Teste enviado para ${to}. Confira a caixa — nenhum cliente foi contatado. Você já pode aprovar até ${cap} abordagens por dia.';
	@override String channel_test_email_not_unlocked({required Object to}) => 'Teste enviado para ${to}, mas não conseguimos registrar a liberação. Rode o teste de novo.';
	@override String get section_company => 'Empresa e marca';
	@override String get section_email => 'E-mail e envio';
	@override String get section_catalog => 'Catálogo e funis';
	@override String get section_documents => 'Documentos';
	@override String get section_integrations => 'Integrações';
	@override String get section_contract => 'Contratar ou ampliar';
	@override String get section_support_bot => 'Atendimento automático';
	@override String get funnels_tile_sub => 'Etapas, os dados a coletar e o webhook';
	@override String get section_advanced => 'Avançado';
	@override String get section_advanced_sub => 'Funis e datas de relacionamento. Já vêm prontos; abra só se quiser mudar.';
	@override String get relationship_tile_sub => 'Aniversários, datas da empresa e do setor que viram rascunho no dia';
	@override String get support_bot_tile_sub => 'Agentes, prompts, flows, tags e as métricas do bot';
	@override String get support_bot_intro => 'O lado chatbot da Replider. Se você veio para vender, nada disso é necessário — as ferramentas de venda ficam na tela anterior.';
	@override String get email_warmup_not_started => 'Aquecimento não iniciado — verifique a caixa (DNS ou reconexão) para liberar a prospecção';
	@override String email_warmup_progress({required Object day, required Object total, required Object sent, required Object cap}) => 'Aquecimento: dia ${day} de ${total} · ${sent} de ${cap} envios hoje';
	@override String email_warmup_done({required Object cap}) => 'Aquecida — até ${cap} envios por dia';
	@override String get channel_name_required => 'Dê um nome ao canal';
	@override String channel_required_missing({required Object fields}) => 'Preencha: ${fields}';
	@override String get channel_email_help_link => 'Como configurar o e-mail do seu domínio';
	@override String get usage_costs_title => 'Uso e custos de IA';
	@override String get usage_costs_subtitle => 'Uso do mês, gasto de IA e seu teto de orçamento';
}

// Path: common
class _Translations$common$pt extends Translations$common$en {
	_Translations$common$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get cancel => 'Cancelar';
	@override String get save => 'Salvar';
	@override String get create => 'Criar';
	@override String get delete => 'Excluir';
	@override String get edit => 'Editar';
	@override String get retry => 'Tentar novamente';
	@override String get subscribe => 'Assinar';
	@override String get per_month => '/mês';
	@override String get free => 'Grátis';
	@override String get success => 'Sucesso';
	@override String get confirm_delete => 'Tem certeza que deseja excluir?';
	@override String get no_results => 'Nenhum resultado encontrado.';
	@override String get confirm => 'Confirmar';
	@override String get remove => 'Remover';
	@override String get close => 'Fechar';
	@override String get add => 'Adicionar';
	@override String get search => 'Buscar';
	@override String get copy => 'Copiar';
	@override String get change => 'Alterar';
	@override String get load_more => 'Carregar mais';
	@override String error_generic({required Object error}) => 'Erro: ${error}';
	@override String get back => 'Voltar';
	@override String get saving => 'Salvando...';
	@override String get sending => 'Enviando...';
	@override String get sent => 'Enviado';
	@override String get loading => 'Carregando...';
	@override String get ok => 'OK';
	@override String get open => 'Abrir';
	@override String get send => 'Enviar';
	@override String get discard => 'Descartar';
	@override String get keep_editing => 'Continuar editando';
	@override String get discard_changes_title => 'Descartar alterações?';
	@override String get discard_changes_body => 'Você fez mudanças que ainda não foram salvas. Sair agora as perde.';
	@override String get yes => 'Sim';
	@override String get no => 'Não';
	@override String get logout => 'Sair';
	@override String get refresh => 'Atualizar';
	@override String get select => 'Selecionar';
	@override String get name_label => 'Nome';
	@override String get color => 'Cor';
	@override String get new_short => 'Nova';
	@override String get unreadable_content => 'Conteúdo ilegível';
	@override String get load_failed_tap => 'Não foi possível carregar. Toque para tentar de novo.';
}

// Path: billing
class _Translations$billing$pt extends Translations$billing$en {
	_Translations$billing$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Uso e custos de IA';
	@override String get checkout_error => 'Falha ao iniciar o checkout.';
	@override String get manage_payment_title => 'Gerenciar pagamento';
	@override String get manage_payment_subtitle => 'Abra o portal do Stripe pra atualizar o cartão, baixar notas ou cancelar.';
	@override String get manage_payment_cta => 'Abrir portal';
	@override String get portal_opening => 'Abrindo portal...';
	@override String get upgrade_modal_title => 'Escolha seu plano';
	@override String get upgrade_modal_subtitle => 'Assine direto por aqui. O pagamento é seguro via Stripe.';
	@override String get upgrade_current_plan_badge => 'Plano atual';
	@override String get upgrade_popular_badge => 'Mais popular';
	@override String get upgrade_cta => 'Assinar';
	@override String get upgrade_cta_contact_sales => 'Falar com vendas';
	@override String get contract_badge => 'Plano sob contrato';
	@override String get contract_subtitle => 'Limites customizados + você usa sua própria chave de API. Fale com um especialista para combinar valor e ativação.';
	@override String get contract_email_cta => 'E-mail';
	@override String get contract_whatsapp_cta => 'WhatsApp';
	@override String contract_email_subject({required Object plan}) => 'Plano ${plan} do Replider — interesse';
	@override String get contract_link_failed => 'Não foi possível abrir o link. Envie e-mail para support@replider.com.';
	@override String get upgrade_per_month => '/mês';
	@override String get upgrade_interval_monthly => 'Mensal';
	@override String get upgrade_interval_annual => 'Anual';
	@override String get upgrade_annual_save => 'Economize 20%';
	@override String get upgrade_billed_annually => 'cobrado anualmente';
	@override String get upgrade_compare_current => 'Atual';
	@override String get upgrade_compare_new => 'Novo';
	@override String get upgrade_compare_conversations => 'Conversas/mês';
	@override String get upgrade_compare_agents => 'Agentes de IA';
	@override String get upgrade_compare_channels => 'Canais';
	@override String get upgrade_compare_tokens => 'Tokens de IA/mês';
	@override String get upgrade_checkout_opening => 'Abrindo checkout...';
	@override String upgrade_feature_agents({required Object count}) => '${count} agente(s) IA';
	@override String upgrade_feature_channels({required Object count}) => '${count} canal(is)';
	@override String upgrade_feature_conversations({required Object count}) => '${count} conversas/mês';
	@override String upgrade_feature_tokens({required Object tokens}) => '${tokens} tokens IA/mês';
	@override String billing_pill_trial_days({required Object days}) => 'Trial: ${days} dias';
	@override String get billing_pill_trial_last_day => 'Trial acaba hoje';
	@override String get billing_pill_trial_expired => 'Trial expirado';
	@override String get billing_pill_past_due => 'Pagamento pendente';
	@override String get billing_pill_manage => 'Gerenciar';
	@override String get no_customer_yet => 'Conclua a primeira assinatura antes de abrir o portal de pagamento.';
	@override String get checkout_open_failed => 'Falha ao iniciar o checkout.';
	@override String get addon_buy_failed => 'Falha ao comprar o pack.';
	@override String get portal_open_failed => 'Falha ao abrir o portal de pagamento.';
	@override String get quota_conversations_label => 'Conversas';
	@override String get quota_tokens_label => 'Créditos IA';
	@override String get quota_agents_label => 'Agentes IA';
	@override String get quota_wall_title => 'Limite de IA atingido';
	@override String get quota_wall_body => 'Seus créditos de IA deste ciclo acabaram. O agente parou de responder automaticamente até você liberar mais créditos ou o ciclo resetar.';
	@override String get quota_wall_cta_later => 'Esperar reset do mês';
	@override String quota_wall_reset_hint({required Object day}) => 'O ciclo reinicia dia ${day} do próximo mês.';
	@override String get back_tooltip => 'Voltar';
	@override String get section_usage => 'USO ATUAL';
	@override String get section_addon_packs => 'PACOTES EXTRAS';
	@override String get section_plans => 'PLANOS DISPONIVEIS';
	@override String get addon_packs_unavailable => 'Pacotes extras indisponiveis.';
	@override String get load_usage_failed => 'Erro ao carregar uso';
	@override String get load_plans_failed => 'Erro ao carregar planos';
	@override String get trial_view_plans => 'Ver planos';
	@override String get usage_ai_tokens => 'Tokens de IA';
	@override String usage_includes_addon_tokens({required Object count}) => 'Inclui ${count} tokens extras';
	@override String get usage_input => 'Input';
	@override String get usage_output => 'Output';
	@override String get usage_conversations => 'Conversas';
	@override String get overage_active_title => 'Pagando por conversas extras';
	@override String overage_active_body({required Object count, required Object amount}) => '${count} conversas além do plano · ${amount} cobrado neste ciclo';
	@override String overage_rate_note({required Object rate}) => '${rate} por conversa extra';
	@override String get overage_cap_reached => 'Limite de excedente atingido — conversas extras pausadas até o próximo ciclo.';
	@override String get plan_free => 'Grátis';
	@override String get plan_per_month_short => '/mês';
	@override String plan_feature_agents({required Object count}) => '${count} agente(s) de IA';
	@override String plan_feature_channels({required Object count}) => '${count} canal(is) de atendimento';
	@override String plan_feature_conversations({required Object count}) => '${count} conversas/mês';
	@override String plan_feature_tokens({required Object tokens}) => '${tokens} tokens IA/mês';
	@override String plan_feature_storage({required Object mb}) => '${mb} MB de armazenamento';
	@override String plan_feature_storage_gb({required Object gb}) => '${gb} GB de armazenamento';
	@override String plan_feature_seats({required Object count}) => '${count} assento(s) de atendente';
	@override String plan_feature_broadcast({required Object count}) => '${count} disparos/mês';
	@override String plan_feature_recipients({required Object count}) => '${count} destinatários por campanha';
	@override String plan_feature_overage({required Object cents, required Object cap}) => 'Conversas extras: ${cents}¢ cada (limite ${cap}× do preço do plano)';
	@override String effective_limits_title({required Object plan}) => 'Seus limites do contrato ${plan}';
	@override String get effective_limits_subtitle => 'Resolvidos a partir dos overrides do contrato — cobrança off-platform.';
	@override String get byok_label => 'Use sua chave de IA (BYOK)';
	@override String byok_configured({required Object providers}) => 'Configurada: ${providers}';
	@override String get byok_pending => 'Não configurada — o agente não responde até você cadastrar a chave.';
	@override String get byok_manage_cta => 'Gerenciar';
	@override String get cancel_subscription => 'Cancelar assinatura';
	@override String get plan_contract => 'Sob consulta';
	@override String plan_feature_leads({required Object count}) => '${count} leads/mês';
	@override String plan_feature_enrichments({required Object count}) => '${count} pesquisas de empresa/mês';
	@override String plan_feature_drafts({required Object count}) => '${count} rascunhos de IA/mês';
	@override String plan_feature_funnels({required Object count}) => '${count} funil(is)';
	@override String plan_feature_mailboxes({required Object count}) => '${count} caixa(s) de e-mail';
	@override String get usage_sdr_title => 'SEU MÊS';
	@override String get usage_sdr_leads => 'Leads no funil';
	@override String get usage_sdr_enrichments => 'Pesquisas de empresa';
	@override String get usage_sdr_approaches => 'Abordagens enviadas';
	@override String get usage_sdr_meetings => 'Reuniões marcadas';
	@override String usage_sdr_of_limit({required Object used, required Object limit}) => '${used} de ${limit}';
	@override String usage_sdr_no_limit({required Object used}) => '${used} este mês';
	@override String get talk_title => 'Quer contratar ou ampliar?';
	@override String get talk_body => 'A gente monta a proposta na conversa, do tamanho da sua operação. Sem tabela de preço: cada empresa vende de um jeito.';
	@override String get talk_book_cta => 'Agendar conversa';
	@override String get talk_email_subject => 'Quero conhecer o Replider';
	@override String get ai_cost_title => 'Custo de IA (sua chave)';
	@override String get ai_cost_cycle => 'Neste ciclo';
	@override String get ai_cost_projection => 'Ritmo mensal';
	@override String get ai_cost_budget => 'Seu teto';
	@override String get ai_cost_no_budget => 'sem teto';
	@override String get ai_cost_set_budget => 'Definir teto mensal';
	@override String get ai_cost_budget_dialog_title => 'Teto mensal de IA';
	@override String get ai_cost_budget_help => 'Corte duro: quando o gasto chegar nesse valor, a geração de IA pausa até o ciclo reiniciar. 0 remove o teto.';
}

// Path: team
class _Translations$team$pt extends Translations$team$en {
	_Translations$team$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Equipe';
	@override String get invite_member => 'Convidar Membro';
	@override String get member_invited => 'Membro convidado com sucesso.';
	@override String get member_removed => 'Membro removido.';
	@override String get role_updated => 'Função atualizada.';
	@override String get change_role => 'Alterar função';
	@override String get remove_member => 'Remover membro';
	@override String get inactive => 'Inativo';
	@override String get pending_badge => 'Pendente';
	@override String get resend_invite => 'Reenviar convite';
	@override String get invite_resent => 'Convite reenviado.';
	@override String get active_now => 'Ativo agora';
	@override String last_seen_ago({required Object when}) => 'Visto há ${when}';
	@override String get invite_short => 'Convidar';
	@override String get empty_subtitle => 'Nenhum membro na equipe';
	@override String get role_label => 'Cargo';
	@override String get role_admin => 'Admin';
	@override String get role_attendant => 'Atendente';
	@override String get change_role_title => 'Alterar Cargo';
	@override String change_role_body({required Object name, required Object role}) => 'Deseja alterar o cargo de "${name}" para ${role}?';
	@override String get remove_title => 'Remover Membro';
	@override String remove_body({required Object name}) => 'Deseja realmente remover "${name}" da equipe? Esta pessoa perderá acesso ao sistema.';
	@override String ago_minutes({required Object count}) => '${count} min';
	@override String ago_hours({required Object count}) => '${count}h';
	@override String ago_days({required Object count}) => '${count} dias';
	@override String ago_months({required Object count}) => '${count} meses';
	@override String get role_owner => 'Dono';
	@override String get role_gestor => 'Gestor';
	@override String get role_vendedor => 'Vendedor';
	@override String get role_vendedor_hint => 'Trabalha os próprios leads e ofertas; não abre o inbox geral';
	@override String get role_gestor_hint => 'Acompanha o time e o funil; sem cobrança e integrações';
	@override String get role_attendant_hint => 'Atende as conversas do inbox; não vê o pipeline';
	@override String get role_admin_hint => 'Tudo: configurações, equipe, canais e cobrança';
	@override String get invite_fields_required => 'Preencha nome e e-mail';
	@override String get invite_email_invalid => 'E-mail inválido — confira o endereço';
}

// Path: channels
class _Translations$channels$pt extends Translations$channels$en {
	_Translations$channels$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Canais';
	@override String get connect_description => 'Conecte WhatsApp, Instagram ou Telegram para receber mensagens de clientes.';
	@override String get channel_created => 'Canal criado com sucesso.';
	@override String get channel_deleted => 'Canal excluído.';
	@override String get webhook_down => 'Webhook fora';
	@override String get test_connection => 'Testar conexão';
	@override String get delete_channel => 'Excluir canal';
	@override String get type_webchat_label => 'Site (Webchat)';
	@override String get setup_whatsapp => 'Informe o Phone Number ID do seu número no painel Meta for Developers. O token de acesso é gerenciado automaticamente pela Replider.';
	@override String get setup_telegram => 'Crie um bot no @BotFather no Telegram e cole o token aqui. O webhook será configurado automaticamente.';
	@override String get setup_instagram => 'Conecte sua conta profissional do Instagram (Business ou Creator). Informe o ID da conta e um token de acesso com a permissão instagram_business_manage_messages. Apenas mensagens diretas.';
	@override String get setup_webchat => 'Atendimento via chat no seu site. Após criar, copie o snippet e cole no HTML do seu site.';
	@override String get setup_sms => 'Conecte seu próprio número Twilio para conversas de SMS bidirecionais. Cole o Account SID, o Auth Token e o número de telefone (E.164) do console da Twilio. Depois de conectar, copie a URL de webhook do canal e configure-a como webhook de "A message comes in" do seu número na Twilio.';
	@override String get embedded_signup_missing_config => 'Configuração incompleta no build. Defina WHATSAPP_APP_ID e WHATSAPP_EMBEDDED_CONFIG_ID via --dart-define.';
	@override String get embedded_signup_unknown_failure => 'Falha desconhecida no signup.';
	@override String embedded_signup_complete_failed({required Object error}) => 'Erro ao concluir conexão: ${error}';
	@override String get embedded_signup_success => 'WhatsApp conectado com sucesso!';
	@override String get embedded_signup_connecting => 'Conectando...';
	@override String get embedded_signup_cta => 'Conectar WhatsApp via Meta';
	@override String get embedded_signup_missing_env => 'WHATSAPP_APP_ID ou WHATSAPP_EMBEDDED_CONFIG_ID não configurado no build.';
	@override String get embedded_signup_no_response => 'Nenhuma resposta do Facebook SDK.';
	@override String get embedded_signup_user_cancelled => 'Login cancelado pelo usuário.';
	@override String embedded_signup_no_code({required Object status}) => 'Facebook não retornou código de autorização (status=${status}).';
	@override String embedded_signup_failure({required Object error}) => 'Falha no Embedded Signup: ${error}';
	@override String get setup_email => 'Envie e receba e-mails pelo seu próprio domínio. Você precisa do SMTP do seu provedor e de publicar SPF, DKIM e DMARC no DNS — sem isso o Gmail e o Outlook descartam a mensagem sem avisar.';
	@override String get email_dns_verified => 'Domínio autenticado — pronto para enviar.';
	@override String get email_dns_pending => 'Falta autenticar o domínio. Sem isso, os provedores dos seus clientes (Gmail, Outlook e outros) descartam suas mensagens.';
	@override String get email_dns_recheck => 'Verificar de novo';
	@override String get email_dns_optional => 'opcional';
	@override String get email_dns_host => 'Nome';
	@override String get email_dns_value => 'Valor';
	@override String get email_dns_found => 'Publicado atualmente';
	@override String get email_dns_copied => 'Copiado.';
	@override String get email_dns_provider_managed => 'O Google autentica os envios deste endereço — nada para publicar.';
	@override String get email_dns_already_published => 'Já publicado — nada a fazer aqui.';
	@override String email_dns_detected({required Object provider}) => 'DNS na ${provider}';
	@override String get email_dns_open_guide => 'Abrir guia';
	@override String get email_dns_auto_configure => 'Configurar automaticamente';
	@override String email_dns_token_help({required Object provider}) => 'Na ${provider}, crie um token de API com permissão Zone → DNS → Edit para este domínio e cole aqui. Usamos uma vez para publicar os registros e não guardamos em lugar nenhum.';
	@override String get email_dns_token_hint => 'Cole o token';
	@override String get email_dns_auto_done => 'Registros publicados';
	@override String get email_dns_auto_partial => 'Publicado em parte';
	@override String get dns_action_created => 'criado';
	@override String get dns_action_updated => 'atualizado';
	@override String get dns_action_merged => 'unido ao registro que já existia';
	@override String get dns_action_kept => 'o seu foi mantido, sem alteração';
	@override String get dns_action_failed => 'falhou';
	@override String get email_dns_need_help => 'Precisa de ajuda?';
	@override String get email_dns_provider_unknown => 'não detectado';
	@override String email_dns_help_draft({required Object domain, required Object provider, required Object records}) => 'Preciso de ajuda para configurar o domínio de e-mail.\n\nDomínio: ${domain}\nProvedor de DNS: ${provider}\nRegistros faltando: ${records}';
	@override String get webhook_health_unknown => 'Saúde não verificada';
	@override String get gmail_connect_title => 'Conectar a conta que você já usa';
	@override String get gmail_connect_help => 'Um clique, sem servidor nem senha para digitar. Funciona com Gmail e Google Workspace, e as respostas continuam caindo na sua caixa de sempre.';
	@override String get gmail_connect_cta => 'Conectar com o Google';
	@override String get gmail_connected => 'E-mail conectado.';
	@override String get gmail_reconnected => 'Conexão de e-mail renovada.';
	@override String get gmail_denied => 'Conexão cancelada. Nada foi alterado.';
	@override String get gmail_failed => 'Não consegui concluir a conexão. Tente de novo.';
	@override String get gmail_or_smtp => 'ou configure o SMTP manualmente';
	@override String gmail_already_connected({required Object name}) => 'Caixa conectada: ${name}';
	@override String get gmail_connect_check => 'Já autorizei — verificar';
	@override String email_dns_token_title_named({required Object provider}) => 'Token de API da ${provider}';
	@override String get email_field_from => 'E-mail de envio';
	@override String get email_field_smtp_host => 'Servidor SMTP';
	@override String get email_field_smtp_port => 'Porta';
	@override String get email_field_smtp_user => 'Usuário SMTP';
	@override String get email_field_smtp_password => 'Senha SMTP';
	@override String get email_field_imap_host => 'Servidor IMAP (opcional)';
	@override String get email_field_imap_hint => 'igual ao SMTP se vazio';
	@override String get email_field_imap_port => 'Porta IMAP (opcional)';
	@override String get email_dns_generic_steps => 'Não reconhecemos automaticamente o seu provedor de DNS. Sem problema: entre no painel da empresa onde o domínio é administrado (quem vendeu o domínio ou hospeda o site — Locaweb, KingHost, GoDaddy, etc.), procure por "DNS", "Zona DNS" ou "Gerenciar registros" e crie os registros abaixo.';
	@override String get email_dns_how_to => 'Crie cada registro abaixo na zona DNS do domínio — os botões copiam nome e valor. Depois de salvar, a propagação leva de minutos a algumas horas; esta tela verifica sozinha.';
	@override String get fb_signup_mobile_only => 'O fluxo Embedded Signup só funciona no navegador. No mobile, use o formulário manual em "Adicionar canal".';
	@override String get email_dns_poll_stopped => 'Paramos de verificar sozinhos — depois de ~10 minutos a causa comum é registro errado, não propagação. Corrija e toque em atualizar para checar de novo.';
}

// Path: whatsapp
class _Translations$whatsapp$pt extends Translations$whatsapp$en {
	_Translations$whatsapp$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get profile_load_error => 'Erro ao carregar perfil';
	@override String get segment => 'Segmento';
	@override String get new_template => 'Novo template';
	@override String get reconnect_meta => 'Reconectar via Meta';
	@override String get reconnect_meta_help => 'Reautorize a conexão com a Meta pra renovar o token da API. Leva uns 60 segundos — você continua nessa tela.';
	@override String get reconnect_success => 'Conexão renovada.';
	@override String get no_templates => 'Nenhum template ainda';
	@override String get create_first_template => 'Criar primeiro template';
	@override String get delete_template => 'Excluir template';
	@override String get add_button => 'Adicionar botão';
	@override String get quick_reply => 'Resposta rápida';
	@override String get open_url => 'Abrir URL';
	@override String template_sent_success({required Object phone}) => 'Template enviado para ${phone}';
	@override String get send_template_title => 'Enviar template';
	@override String send_template_to_with_name({required Object name, required Object phone}) => 'Para ${name} (+${phone})';
	@override String send_template_to({required Object phone}) => 'Para +${phone}';
	@override String get approved_template_label => 'Template aprovado';
	@override String get choose_approved_template => 'Escolha um template aprovado.';
	@override String get no_approved_templates_hint => 'Nenhum template aprovado neste canal. Crie um na aba Templates e aguarde a aprovação da Meta.';
	@override String loading_templates_error({required Object error}) => 'Erro ao carregar templates: ${error}';
	@override String get sending => 'Enviando...';
	@override String get send => 'Enviar';
	@override String get template_status_approved => 'Aprovado';
	@override String get template_status_pending => 'Em análise';
	@override String get template_status_rejected => 'Rejeitado';
	@override String get template_status_paused => 'Pausado';
	@override String get template_status_disabled => 'Desativado';
	@override String get category_authentication => 'Autenticação';
	@override String get language_pt_br => 'Português (BR)';
	@override String get language_en_us => 'Inglês (US)';
	@override String get language_es => 'Espanhol';
	@override String get error_token_expired => 'O token do WhatsApp expirou. Reconecte pela Meta para restaurar o acesso.';
	@override String get error_outside_window => 'Janela de 24h expirou. Use um template aprovado para reabrir a conversa.';
	@override String get error_missing_permission => 'A Meta negou a permissão. Verifique se o app tem whatsapp_business_management habilitado.';
	@override String error_recipient({required Object message}) => 'Destinatário inválido ou não registrado no WhatsApp: ${message}';
	@override String error_template({required Object message}) => 'Erro no template: ${message}';
	@override String get error_missing_waba => 'Este canal não tem WABA ID. Reconecte via Meta Embedded Signup.';
	@override String get error_missing_token => 'Este canal não tem access token. Reconecte via Meta Embedded Signup.';
	@override String get error_register_rate_limited => 'A Meta bloqueou tentativas de registro neste número por excesso de chamadas. Aguarde ~24h antes de tentar de novo. Enquanto isso, confira se o WABA tem método de pagamento e se a verificação em duas etapas está desligada.';
	@override String get error_pin_mismatch => 'PIN incorreto. Use a aba "Esqueci o PIN" para receber um código por SMS.';
	@override String get error_already_verified => 'Este número já está verificado em outro app Cloud API. Use "Liberar número" para soltá-lo antes de registrar aqui.';
	@override String get error_cert_2fa_blocked => 'A Meta bloqueou o registro porque a verificação em duas etapas (2FA) do WABA está ativa. Desligue em Meta Business → Configurações da conta → Verificação em duas etapas e tente de novo.';
	@override String get error_ownership_not_verified => 'A Meta ainda não verificou a posse deste número. Use a aba "Esqueci o PIN" para receber um código por SMS ou voz e finalizar o registro em um único passo.';
	@override String get error_verify_code_invalid => 'Código incorreto, expirado ou já usado. Clique em "Enviar código" novamente para receber um novo (códigos da Meta expiram em ~10 minutos).';
	@override String error_register_failed({required Object detail}) => 'Não foi possível registrar o número no Cloud API. ${detail}';
	@override String error_deregister_failed({required Object detail}) => 'Falha ao liberar o número. ${detail}';
	@override String error_request_code_failed({required Object detail}) => 'A Meta não conseguiu enviar o código de verificação. ${detail} Tente novamente em 1 minuto, ou alterne entre SMS e ligação.';
	@override String get error_verify_code_failed => 'Código inválido ou expirado. Solicite um novo e tente de novo.';
	@override String error_subscribe_failed({required Object detail}) => 'Não foi possível inscrever a Replider para receber webhooks. ${detail}';
	@override String error_webhook_status_failed({required Object detail}) => 'Não foi possível verificar o status do webhook. ${detail}';
	@override String get error_token_exchange_failed => 'A Meta recusou a troca de token durante a conexão. Tente o Embedded Signup novamente.';
	@override String error_template_create_failed({required Object detail}) => 'A Meta rejeitou a criação do template. ${detail}';
	@override String error_template_delete_failed({required Object detail}) => 'A Meta rejeitou a exclusão do template. ${detail}';
	@override String get error_profile_fetch_failed => 'Não foi possível carregar o perfil do WhatsApp Business. Tente recarregar.';
	@override String error_profile_update_failed({required Object detail}) => 'Não foi possível salvar o perfil do WhatsApp Business. ${detail}';
	@override String get error_picture_upload_failed => 'Falha ao enviar a foto de perfil. Verifique se é JPG/PNG abaixo de 5MB.';
	@override String get error_phone_owned_elsewhere => 'Este número já está conectado a outra conta Replider. Solicite a liberação para o suporte.';
	@override String get error_missing_creds => 'Canal sem credenciais válidas. Reconecte pelo Embedded Signup.';
	@override String get error_not_supported => 'Operação não suportada para este tipo de canal.';
	@override String get error_trial_expired => 'Seu período de testes terminou. Faça upgrade do plano para continuar usando o WhatsApp.';
	@override String get error_subscription_inactive => 'Sua assinatura não está ativa. Atualize o método de pagamento para reativar.';
	@override String error_meta_code({required Object code}) => 'Código Meta ${code}.';
	@override String error_unknown_with_code({required Object code}) => 'Erro desconhecido (${code}).';
	@override String get error_invalid_param_duplicate_name => 'Já existe um template com esse nome e idioma. Escolha outro nome ou apague o existente antes de reenviar.';
	@override String get error_invalid_param_paused => 'A criação de templates está pausada para esta conta. Aguarde algumas horas ou contate o suporte da Meta.';
	@override String get error_invalid_param_language => 'Este idioma ainda não está habilitado para templates nesta conta. Tente pt_BR ou en_US.';
	@override String get error_invalid_param_category_mismatch => 'Meta rejeitou o template. Provavelmente o texto não combina com a categoria escolhida: MARKETING para promoções/novidades, UTILITY só para confirmações/avisos transacionais, AUTHENTICATION só para códigos de verificação. Revise também variáveis {{1}} {{2}} e exemplos.';
	@override String error_invalid_param_default({required Object message, required Object trace}) => 'A Meta rejeitou os parâmetros: ${message}${trace}';
	@override String get template_preview_label => 'COMO O CLIENTE VAI RECEBER';
	@override String get create_template_link => '+ Criar novo template';
	@override String get create_template_title => 'Criar template';
	@override String get create_template_approval_notice => 'A Meta precisa aprovar todo template — costuma levar de minutos a 1 dia. Ele não serve para AGORA, mas fica disponível aqui para todas as próximas conversas.';
	@override String get suggestion_reopen => 'Retomar conversa';
	@override String get suggestion_followup => 'Follow-up de proposta';
	@override String get suggestion_blank => 'Do zero';
	@override String get create_template_name_label => 'Nome';
	@override String get create_template_name_helper => 'só minúsculas, números e _';
	@override String get create_template_body_label => 'Corpo';
	@override String get create_template_body_helper => '{{1}}, {{2}} viram campos na hora do envio. Não pode começar nem terminar com variável.';
	@override String get create_template_missing_fields => 'Preencha o nome e o corpo.';
	@override String get create_template_submitted => 'Template enviado — aparece na lista assim que a Meta aprovar.';
	@override String get create_template_submit => 'Enviar para aprovação';
}

// Path: notifications
class _Translations$notifications$pt extends Translations$notifications$en {
	_Translations$notifications$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Notificações';
	@override String get subtitle_default => 'Receba alertas mesmo com a aba em segundo plano';
	@override String get subtitle_subscribed => 'Você recebe um alerta quando um cliente escreve';
	@override String get subtitle_denied => 'Permissão bloqueada. Libere em Configurações do site > Notificações';
	@override String get subtitle_unsupported => 'Navegador não suporta notificações push';
	@override String get subtitle_ios_install => 'No iPhone, instale o app na tela de início para receber notificações';
	@override String error_loading({required Object error}) => 'Não foi possível carregar o estado das notificações: ${error}';
	@override String get test_btn => 'Enviar teste';
	@override String get test_no_subs => 'Sem inscrição ativa neste dispositivo. Ative as notificações primeiro.';
	@override String test_success({required Object ok, required Object failed}) => 'Teste enviado: ${ok} entregues, ${failed} com falha.';
	@override String test_all_failed({required Object count}) => 'Todas as ${count} inscrições falharam. Reative as notificações para renovar as chaves.';
	@override String test_disabled_some({required Object count}) => '${count} inscrição(ões) desativada(s) — provável incompatibilidade de chave VAPID. Reative em cada dispositivo.';
	@override String test_request_failed({required Object error}) => 'Falha na requisição de teste: ${error}';
}

// Path: api_keys
class _Translations$api_keys$pt extends Translations$api_keys$en {
	_Translations$api_keys$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Chaves dos provedores de IA';
	@override String get subtitle => 'Seu plano sob contrato usa suas próprias chaves de API. O Replider guarda criptografado e nunca compartilha.';
	@override String get not_configured => 'Não configurada';
	@override String get input_hint => 'Cole a chave de API';
	@override String get add => 'Adicionar';
	@override String get edit => 'Alterar';
	@override String get save => 'Salvar';
	@override String get clear => 'Remover';
	@override String get saved => 'Chave atualizada.';
	@override String save_failed({required Object error}) => 'Não foi possível salvar a chave: ${error}';
}

// Path: notes
class _Translations$notes$pt extends Translations$notes$en {
	_Translations$notes$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get add_error => 'Erro ao adicionar nota';
	@override String get delete_error => 'Erro ao deletar nota';
	@override String get internal_notes => 'Notas internas';
	@override String get no_notes => 'Sem notas ainda';
	@override String get internal_note => 'Nota interna';
	@override String get load_error => 'Erro ao carregar notas';
	@override String get add_hint => 'Adicionar nota...';
	@override String get delete_confirm_title => 'Excluir nota?';
	@override String get delete_confirm_body => 'Esta nota interna será removida para o time inteiro.';
}

// Path: message_actions
class _Translations$message_actions$pt extends Translations$message_actions$en {
	_Translations$message_actions$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get copy_text => 'Copiar texto';
	@override String get message_copied => 'Mensagem copiada';
	@override String get copy_message_id => 'Copiar ID da mensagem';
	@override String get id_copied => 'ID copiado';
}

// Path: contact_sheet
class _Translations$contact_sheet$pt extends Translations$contact_sheet$en {
	_Translations$contact_sheet$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get tab_info => 'Informações';
	@override String get tab_files => 'Arquivos';
	@override String get stat_conversations => 'Conversas';
	@override String get stat_messages => 'Mensagens';
	@override String get stat_customer_since => 'Cliente desde';
	@override String get files_section_images => 'Imagens e vídeos';
	@override String get files_section_documents => 'Documentos e áudios';
	@override String get files_empty => 'Nenhuma mídia foi trocada nesta conversa ainda.';
	@override String get files_error => 'Não foi possível carregar os arquivos.';
	@override String get files_unnamed => 'Arquivo sem nome';
	@override String get conversation_section => 'Conversa';
	@override String footer_stats({required Object convs, required Object msgs, required Object since}) => '${convs} conversas · ${msgs} mensagens · primeiro contato ${since}';
	@override String get files_section_attachments => 'Anexos de e-mail';
	@override String get files_email_no_subject => '(sem assunto)';
}

// Path: conversation
class _Translations$conversation$pt extends Translations$conversation$en {
	_Translations$conversation$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get send_failed_recovered => 'Falha ao enviar. Mensagem recuperada — tente novamente.';
	@override String get message_send_failed => 'Não entregue';
	@override String get resume_bot => 'Retomar Bot';
	@override String get file_read_error => 'Não foi possível ler o arquivo.';
	@override String file_send_error({required Object error}) => 'Erro ao enviar arquivo: ${error}';
	@override String get nobody => 'Ninguém';
	@override String get assume => 'Assumir conversa';
	@override String get release => 'Devolver ao bot';
	@override String under_takeover_by({required Object name}) => 'Em atendimento por ${name}';
	@override String get under_takeover_anonymous => 'Em atendimento humano';
	@override String get input_disabled_tooltip => 'Assuma a conversa para responder manualmente';
	@override String get resolve_action => 'Resolver';
	@override String get reopen_action => 'Reabrir';
	@override String get load_older_messages => 'Carregar mensagens anteriores';
	@override String get resolve_confirm_title => 'Resolver conversa?';
	@override String get resolve_confirm_body => 'A conversa vai para "Encerradas". Você pode reabrir depois pelo filtro da lista.';
	@override String get convert_lead_action => 'Converter lead';
	@override String get convert_lead_title => 'Converter lead em cliente';
	@override String get convert_lead_help => 'Vincula uma conta cliente ao atendente atribuído desta conversa como vendedor (lead inbound, trilho de 20%).';
	@override String get convert_lead_search_hint => 'Nome da conta ou email';
	@override String get convert_lead_search_min => 'Digite pelo menos 2 caracteres para buscar.';
	@override String get convert_lead_no_results => 'Nenhuma conta corresponde à busca.';
	@override String get convert_lead_already_linked => 'Já vinculada a um vendedor';
	@override String get convert_lead_cta => 'Converter';
	@override String convert_lead_success({required Object account}) => 'Lead convertido: ${account} entrou na carteira do atendente atribuído.';
	@override String convert_lead_error({required Object error}) => 'Não foi possível converter o lead: ${error}';
	@override String get priority_urgent => 'Urgente';
	@override String get priority_high => 'Alta';
	@override String get priority_normal => 'Normal';
	@override String get priority_low => 'Baixa';
	@override String get assign_label => 'Atribuir';
	@override String get tags_label => 'Tags';
	@override String get tag_picker_title => 'Selecionar Tags';
	@override String get tag_picker_new => 'Nova tag';
	@override String get tag_picker_search => 'Buscar tag...';
	@override String get tag_picker_empty_title => 'Você ainda não tem tags.';
	@override String get tag_picker_empty_body => 'Crie a primeira tag pelo botão acima.';
	@override String tag_picker_no_match({required Object query}) => 'Nenhuma tag para "${query}".';
	@override String tag_update_error({required Object error}) => 'Erro ao atualizar tag: ${error}';
	@override String tag_create_error({required Object error}) => 'Erro ao criar tag: ${error}';
	@override String get tag_color_label => 'Cor';
	@override String get tag_dialog_title => 'Nova tag';
	@override String get tag_name_label => 'Nome';
	@override String get tag_name_hint => 'Ex.: VIP, Suporte, Dúvida';
	@override String get tag_dialog_create => 'Criar';
	@override String get contact_bio => 'Bio';
	@override String get contact_status_label => 'Status';
	@override String get contact_bot_label => 'Bot';
	@override String get contact_bot_paused => 'Pausado';
	@override String get contact_bot_active => 'Ativo';
	@override String get contact_handled_by => 'Em atendimento por';
	@override String get contact_last_message => 'Última mensagem';
	@override String get contact_close_btn => 'Fechar';
	@override String get status_open => 'Aberta';
	@override String get status_pending => 'Aguardando atendente';
	@override String get source_whatsapp => 'WhatsApp';
	@override String get source_telegram => 'Telegram';
	@override String get source_instagram => 'Instagram';
	@override String get source_sms => 'SMS';
	@override String get source_internal => 'Suporte interno';
	@override String get source_email => 'E-mail';
	@override String get shared_contact => 'Contato compartilhado';
	@override String get shared_contact_copied => 'Número copiado';
	@override String get media_video => 'Vídeo';
	@override String resume_bot_other_confirm({required Object name}) => 'Esta conversa está com ${name}. Devolver ao bot mesmo assim?';
	@override String contact_handled_by_inline({required Object name}) => 'em atendimento por ${name}';
	@override String get contact_bot_paused_inline => 'bot pausado';
	@override String get contact_bot_active_inline => 'bot ativo';
}

// Path: lgpd
class _Translations$lgpd$pt extends Translations$lgpd$en {
	_Translations$lgpd$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get anonymize_title => 'Anonimizar dados?';
	@override String get anonymize => 'Anonimizar';
	@override String get copy_json => 'Copiar JSON';
	@override String get no_actions => 'Nenhuma ação registrada ainda.';
	@override String get audit_action_export => 'Export LGPD';
	@override String get audit_action_anonymize => 'Anonimização LGPD';
	@override String get screen_title => 'LGPD & Auditoria';
	@override String get section_data_subject_rights => 'DIREITOS DO TITULAR (Art. 18 LGPD)';
	@override String get section_infrastructure => 'INFRAESTRUTURA';
	@override String get section_audit_trail => 'TRILHA DE AUDITORIA';
	@override String anonymize_confirm_body({required Object externalId}) => 'Esta ação é IRREVERSÍVEL. Todos os dados pessoais do contato ${externalId} serão apagados (nome, telefone, mensagens). Métricas agregadas (contagens, campanhas) são preservadas. A ação é registrada na auditoria.';
	@override String json_copied_hint({required Object id}) => 'JSON copiado. Cole num arquivo lgpd-export-${id}.json';
	@override String get anonymize_success => 'Dados do titular anonimizados. A ação foi registrada na auditoria.';
	@override String get titular_search_hint => 'Busque um contato pelo telefone (E.164, ex: 5511999999999) para exportar ou apagar seus dados.';
	@override String data_found_for({required Object id}) => 'Dados encontrados para ${id}';
	@override String get stat_conversations => 'Conversas';
	@override String get stat_messages => 'Mensagens';
	@override String get stat_campaigns => 'Campanhas';
	@override String audit_records_summary({required Object shown, required Object total}) => '${shown} de ${total} registros';
	@override String get intro_title => 'O que é essa tela';
	@override String get intro_body => 'Painel administrativo de privacidade e auditoria. É aqui que você responde a pedidos de clientes baseados na LGPD, testa a entrega de e-mails do servidor e revisa tudo que os admins da conta fizeram.';
	@override String get intro_bullet_rights_title => 'Direitos do titular';
	@override String get intro_bullet_rights_body => 'Exporta ou anonimiza os dados de um contato (nome, telefone, mensagens) — atende Art. 18 da LGPD em um clique.';
	@override String get intro_bullet_infra_title => 'Infraestrutura';
	@override String get intro_bullet_infra_body => 'Teste rápido do servidor SMTP pra garantir que e-mails de verificação, recuperação de senha e alertas estão saindo.';
	@override String get intro_bullet_audit_title => 'Trilha de auditoria';
	@override String get intro_bullet_audit_body => 'Linha do tempo de todas as ações administrativas (takeover, anonimização, criação de canal, etc.). Use os filtros pra rastrear um autor ou uma ação específica; exporte CSV pra anexar em relatórios.';
}

// Path: feedback
class _Translations$feedback$pt extends Translations$feedback$en {
	_Translations$feedback$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get nps_title => 'Qual a chance de você recomendar a Replider?';
	@override String get nps_subtitle => 'Numa escala de 0 a 10, qual a probabilidade de você recomendar a Replider para um colega ou amigo?';
	@override String get nps_scale_low => 'Pouco provável';
	@override String get nps_scale_high => 'Muito provável';
	@override String get nps_comment_hint => 'Qual o principal motivo da sua nota? (opcional)';
	@override String get nps_submit => 'Enviar feedback';
	@override String get nps_later => 'Agora não';
	@override String get nps_thanks => 'Obrigado pelo seu feedback!';
	@override String get cancel_title => 'Antes de ir — por que está cancelando?';
	@override String get cancel_subtitle => 'Seu feedback nos ajuda a melhorar a Replider. Leva só alguns segundos.';
	@override String get cancel_comment_hint => 'Quer compartilhar mais alguma coisa? (opcional)';
	@override String get cancel_submit => 'Enviar feedback';
	@override String get cancel_thanks => 'Obrigado pelo seu feedback!';
	@override late final _Translations$feedback$cancel_reasons$pt cancel_reasons = _Translations$feedback$cancel_reasons$pt._(_root);
}

// Path: product_tour
class _Translations$product_tour$pt extends Translations$product_tour$en {
	_Translations$product_tour$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get skip => 'Pular tour';
	@override String get next => 'Próximo';
	@override String get done => 'Entendi';
	@override String step_counter({required Object current, required Object total}) => '${current} de ${total}';
	@override String get step1_title => 'Bem-vindo à Replider 🎉';
	@override String get step1_body => 'Seu agente de IA está pronto. Aqui vai um tour rápido de 30 segundos mostrando onde fica cada coisa para você aproveitar ao máximo.';
	@override String get step2_title => 'Sua caixa de entrada';
	@override String get step2_body => 'Todas as conversas dos seus canais chegam aqui em tempo real. Assuma a qualquer momento para responder você mesmo no lugar do agente.';
	@override String get step3_title => 'Treine seu agente';
	@override String get step3_body => 'Adicione conhecimento — preços, horários, políticas — em Configurações para o agente responder com precisão em vez de adivinhar. Quanto mais você adiciona, mais inteligente ele fica.';
	@override String get step4_title => 'Acompanhe seu uso';
	@override String get step4_body => 'A barra na parte de baixo mostra conversas e tokens do seu ciclo de cobrança. Faça upgrade quando quiser para aumentar seus limites conforme você cresce.';
}

// Path: feature_announcement
class _Translations$feature_announcement$pt extends Translations$feature_announcement$en {
	_Translations$feature_announcement$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Novidades';
	@override String get release_label => 'Últimas atualizações';
	@override String get cta => 'Entendi';
	@override String get item1_title => 'Gerencie sua assinatura quando quiser';
	@override String get item1_body => 'Abra o portal de cobrança seguro direto no app para atualizar o cartão, trocar de plano ou cancelar — sem precisar abrir um chamado.';
	@override String get item2_title => 'Economize 20% no plano anual';
	@override String get item2_body => 'Troque qualquer plano para anual e pague menos por mês. Alterne o período na tela de cobrança para ver seu novo preço.';
	@override String get item3_title => 'Avisos de uso antes de atingir o limite';
	@override String get item3_body => 'Agora avisamos quando você chega a 80% da sua cota mensal, para que uma semana movimentada nunca bloqueie seu agente de surpresa.';
}

// Path: errors
class _Translations$errors$pt extends Translations$errors$en {
	_Translations$errors$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get AUTH_INVALID_CREDENTIALS => 'E-mail ou senha incorretos.';
	@override String get WEBHOOK_URL_BLOCKED => 'A URL do webhook não pode apontar para a rede interna.';
	@override String get AUTH_EMAIL_ALREADY_EXISTS => 'Este e-mail já está cadastrado.';
	@override String get REGISTRATION_PAUSED => 'Os cadastros estão temporariamente fechados. Já tem conta? É só entrar.';
	@override String get OWNER_EMAIL_EXISTS => 'Este e-mail já pertence a uma conta. Use o modo "conta existente" para aplicar um contrato a ela.';
	@override String get OWNER_SELLER_NOT_FOUND => 'Nenhum vendedor ativo com esse e-mail na conta plataforma.';
	@override String get COMMISSION_SELLER_NOT_FOUND => 'Esse usuário não foi encontrado na conta plataforma.';
	@override String get OWNER_SELLER_INACTIVE => 'Esse vendedor está desativado.';
	@override String get OWNER_INVALID_LEAD_SOURCE => 'Origem do lead inválida — use prospecção (ref) ou inbound.';
	@override String get OWNER_PLATFORM_NOT_CONFIGURED => 'PLATFORM_ACCOUNT_ID não configurado neste ambiente.';
	@override String get SELLER_CONV_UNASSIGNED => 'Esta conversa não tem atendente atribuído. Assuma (ou atribua) antes de converter.';
	@override String get SELLER_NOT_CONV_ASSIGNEE => 'Só o atendente atribuído ou um admin pode converter este lead.';
	@override String get SELLER_ACCOUNT_ALREADY_LINKED => 'Esta conta já está vinculada a um vendedor. Realocação é feita pelo painel do owner.';
	@override String get SELLER_CANNOT_CONVERT_PLATFORM => 'A própria conta plataforma não pode ser convertida.';
	@override String get SELLER_NOT_FLAGGED => 'Você não é um vendedor cadastrado. Peça ao owner para liberar o acesso de vendedor.';
	@override String get SELLER_CONV_TARGET_UNRELATED => 'Esta conta já tem histórico de pagamento e o contato da conversa não corresponde a ela. Conversão bloqueada.';
	@override String get AUTH_UNAUTHORIZED => 'Sessão expirada. Faça login novamente.';
	@override String get AUTH_SOCIAL_FAILED => 'Falha na autenticação social. Tente novamente.';
	@override String get AUTH_INVALID_RESET_TOKEN => 'Link de redefinição inválido ou expirado.';
	@override String get AUTH_EMAIL_NOT_VERIFIED => 'Confirme seu e-mail antes de entrar. Verifique sua caixa de entrada ou solicite um novo link.';
	@override String get AGENT_NOT_FOUND => 'Agente não encontrado.';
	@override String get AGENT_LIMIT_REACHED => 'Você atingiu o limite de agentes do seu plano. Faça upgrade para criar mais agentes.';
	@override String get PLAN_LIMIT_EXCEEDED => 'Você atingiu o limite do seu contrato. Fale com a gente para ampliar.';
	@override String PLAN_LIMIT_EXCEEDED_agents({required Object limit, required Object current}) => 'Seu contrato cobre ${limit} agentes (você tem ${current}). Fale com a gente para ampliar.';
	@override String PLAN_LIMIT_EXCEEDED_channels({required Object limit, required Object current}) => 'Seu contrato cobre ${limit} canais (você tem ${current}). Fale com a gente para ampliar.';
	@override String PLAN_LIMIT_EXCEEDED_seats({required Object limit, required Object current}) => 'Seu contrato cobre ${limit} usuários (você tem ${current}). Fale com a gente para ampliar.';
	@override String get CONV_NOT_FOUND => 'Conversa não encontrada.';
	@override String get CONV_OUTSIDE_24H => 'Fora da janela de 24h do WhatsApp.';
	@override String get USAGE_INSUFFICIENT_TOKENS => 'Tokens de IA insuficientes.';
	@override String get USAGE_NO_CREDITS => 'Limite de IA atingido.';
	@override String get USAGE_LIMIT_EXCEEDED => 'Limite de uso do ciclo atingido. Fale com a gente para ampliar, ou aguarde o próximo ciclo.';
	@override String get ACCOUNT_SUSPENDED => 'Conta suspensa por fatura não paga. Atualize a forma de pagamento para continuar.';
	@override String get TEAM_CANNOT_REMOVE_LAST_ADMIN => 'Você não pode remover o último admin. Transfira a função primeiro.';
	@override String get TEAM_INVALID_PARTICIPANT => 'O participante selecionado não é um membro ativo da equipe.';
	@override String get RATE_LIMITED => 'Muitas requisições. Aguarde um minuto e tente novamente.';
	@override String get TRIAL_EXPIRED => 'Seu período de teste terminou. Fale com a gente para ativar seu contrato.';
	@override String get SUBSCRIPTION_INACTIVE => 'Sua conta não está ativa. Fale com a gente para regularizar.';
	@override String get TERMS_NOT_ACCEPTED => 'Você precisa aceitar os Termos de Uso e a Política de Privacidade.';
	@override String get CONNECTION_TIMEOUT => 'O servidor demorou demais para responder. Tente de novo em instantes.';
	@override String get CONNECTION_ERROR => 'Não conseguimos falar com o servidor. Pode ser sua conexão ou uma instabilidade nossa — tente de novo em instantes.';
	@override String get SERVER_ERROR => 'Servidor indisponível agora. Tente em alguns minutos.';
	@override String get REQUEST_CANCELED => 'Operação cancelada.';
	@override String get BAD_REQUEST => 'Dados inválidos. Confira os campos e tente novamente.';
	@override String get UNKNOWN => 'Ocorreu um erro inesperado. Tente novamente.';
	@override String get TOTP_INVALID_CODE => 'Código incorreto. Verifique a hora do seu dispositivo e tente novamente.';
	@override String get OWNER_2FA_STALE => 'Sua validação de 2 fatores expirou. Confirme o código para continuar.';
	@override String get OWNER_2FA_REQUIRED => 'Ative o 2FA no seu perfil para usar o painel do proprietário.';
	@override String get TOTP_NOT_STAGED => 'Inicie a configuração do 2FA antes de confirmar.';
	@override String get TOTP_NOT_ENABLED => 'A autenticação em duas etapas não está ativada.';
	@override String get TOTP_CIPHER_UNCONFIGURED => 'A autenticação em duas etapas está indisponível no momento. Tente novamente mais tarde.';
	@override String get MESSAGE_SEND_FAILED => 'A mensagem foi salva mas não pôde ser entregue ao cliente. Verifique o canal e tente novamente.';
	@override String get PLAN_NOT_FOUND => 'Plano não encontrado. Atualize a página e tente novamente.';
	@override String get PLAN_NOT_PURCHASABLE => 'Este plano é vendido sob contrato. Fale com um especialista para contratar.';
	@override String get PLAN_CURRENCY_MISMATCH => 'Este plano não está disponível na moeda de cobrança da sua conta.';
	@override String get COUPON_EXHAUSTED => 'Este cupom atingiu o limite de resgates.';
	@override String get ADDON_CURRENCY_UNSUPPORTED => 'Pacotes de créditos extras não estão disponíveis na moeda de cobrança da sua conta.';
	@override String get BROADCAST_NOT_ALLOWED => 'Seu plano não inclui campanhas em massa. Faça upgrade para ativar disparos para listas de contatos.';
	@override String RECIPIENTS_OVER_CAMPAIGN_CAP({required Object current, required Object limit}) => 'Esta campanha tem mais destinatários que seu plano permite (${current}/${limit}). Divida em campanhas menores ou faça upgrade.';
	@override String MONTHLY_BROADCAST_CAP({required Object current, required Object limit}) => 'Você atingiu o limite mensal de disparos (${current}/${limit}). Aguarde a renovação ou faça upgrade.';
	@override String get EMPTY_NAME => 'Nome vazio.';
	@override String get UNEXPECTED => 'Erro inesperado.';
	@override String get JSON_INVALID_OBJECT => 'flow.json deve ser um objeto JSON válido.';
	@override String JSON_INVALID_FORMAT({required Object message}) => 'JSON inválido: ${message}';
	@override String get PROVIDER_COST_NOT_FOUND => 'Linha de preço não encontrada.';
	@override String get PROVIDER_COST_DUPLICATE => 'Já existe preço para esse provider/modelo/data.';
	@override String get PROVIDER_COST_INVALID_RATE => 'Câmbio deve ficar entre 1,00 e 100,00 BRL por USD.';
	@override String get CANNED_RESPONSE_FORBIDDEN => 'Só admins podem gerenciar respostas rápidas da conta.';
	@override String get MCP_TOKEN_NOT_FOUND => 'Token não encontrado ou já revogado.';
	@override String get MCP_TOKEN_LIMIT_REACHED => 'Limite de tokens ativos atingido (10). Revogue um token antigo antes de criar outro.';
	@override String get LEAD_INVALID_PHONE => 'Telefone inválido. Use DDD + número, como 51 98888-7766.';
	@override String get LEAD_NOT_PENDING_APPROVAL => 'Essa abordagem não está mais aguardando aprovação.';
	@override String get LEAD_SEND_FAILED => 'Não consegui enviar a abordagem. Tente de novo em instantes.';
	@override String get LEAD_NOT_FOUND => 'Lead não encontrado.';
	@override String get APPROACH_HAS_PLACEHOLDER => 'A mensagem ainda tem um campo por preencher.';
	@override String get EMAIL_ADDRESS_SUPPRESSED => 'Este endereço não recebe mais e-mails: houve rejeição permanente ou marcação de spam. Veja em Configurações → Endereços bloqueados.';
	@override String get AI_DRAFTS_NOT_ALLOWED => 'Seu plano não inclui rascunhos por IA. Faça upgrade para a IA escrever as abordagens para você.';
	@override String get DOC_EXTRACTION_NOT_ALLOWED => 'Seu plano não inclui leitura de contratos por IA. Faça upgrade para o contrato ser lido automaticamente.';
	@override String get SPEND_BUDGET_EXCEEDED => 'Você atingiu o limite deste recurso no ciclo. Aguarde a renovação ou faça upgrade.';
	@override String get route_not_found_title => 'Essa tela não existe';
	@override String get route_not_found_body => 'O endereço pode ter sido digitado errado ou o link está desatualizado.';
	@override String get route_not_found_cta => 'Ir para o início';
	@override String get PRODUCT_PDF_UNREADABLE => 'Não consegui abrir o arquivo como PDF. Envie o folheto original do fabricante.';
	@override String get PRODUCT_PDF_SCAN_UNAVAILABLE => 'O antivírus está indisponível agora — upload de PDF desabilitado. Tente mais tarde.';
	@override String get PRODUCT_PDF_INFECTED => 'O antivírus recusou este arquivo.';
	@override String get PRODUCT_PDF_EXTRACT_FAILED => 'A leitura automática falhou. Tente de novo ou use o import por CSV.';
	@override String get PRODUCT_PDF_NO_PRODUCTS => 'A leitura não encontrou nenhum produto neste PDF.';
	@override String get PRODUCT_PDF_PROVIDER_CANT_READ => 'O provedor de IA desta conta não lê arquivos. Use o import por CSV.';
	@override String get PRODUCT_PDF_IMPORT_UNAVAILABLE => 'A importação por PDF não está configurada neste ambiente.';
	@override String get EMAIL_WARMUP_NOT_STARTED => 'Esta caixa nunca teve o aquecimento iniciado — verifique o domínio (DNS) ou reconecte a caixa antes de prospectar.';
	@override String get BULK_SENDING_NOT_READY => 'Aprovar em lote exige o envio em volume configurado para o seu domínio. Ative em Configurações → Canais (canal de e-mail), ou aprove uma a uma pela sua própria caixa.';
	@override String get BULK_SENDING_UNAVAILABLE => 'O envio em volume não está disponível na plataforma no momento — aprove uma a uma, ou fale com a gente.';
	@override String get EMAIL_DAILY_CAP_REACHED => 'A caixa atingiu o limite diário de envios frios. Os envios voltam amanhã.';
	@override String get LEAD_APPROACH_TEMPLATE_REQUIRED => 'Este envio precisa de um template aprovado do WhatsApp — abra o lead e escolha um.';
	@override String get INVITE_INVALID => 'Convite inválido, expirado ou já utilizado — peça um novo link para quem convidou você.';
	@override String get SALES_FUNNEL_IS_DEFAULT => 'O funil padrão não pode ser excluído — é onde importações e leads novos entram.';
	@override String get PROSPECTING_NOT_CONFIGURED => 'A busca de empresas não está disponível na plataforma no momento — fale com o suporte.';
	@override String get PROSPECTING_RATE_LIMITED => 'Muitas buscas seguidas — aguarde um instante e tente de novo.';
	@override String get PROSPECTING_INPUT_REQUIRED => 'Diga o que buscar: um segmento e uma cidade.';
	@override String get PROSPECTING_SEARCH_FAILED => 'A busca de empresas falhou — tente de novo em instantes.';
	@override String get APPROACH_NEEDS_PRODUCTS => 'Cadastre seus produtos primeiro — sem catálogo o rascunho sai genérico.';
	@override String get LEAD_FUNNEL_NO_CHANNEL => 'Conecte um e-mail em Configurações → Canais para enviar.';
	@override String get LEAD_CONTACT_NOT_MESSAGEABLE => 'Este cliente não tem e-mail cadastrado.';
	@override String get LEAD_ALREADY_CONTACTED => 'Este cliente já foi abordado — responda pela conversa.';
	@override String get CONTACT_OPTED_OUT => 'Este cliente pediu para não ser contatado.';
	@override String get BYOK_NOT_CONFIGURED => 'Cadastre sua chave de IA em Configurações → Contrato.';
	@override String get FORBIDDEN => 'Seu perfil não tem acesso a isso.';
	@override String get LEAD_NO_CONVERSATION => 'Ninguém escreveu para este cliente ainda — envie a primeira oferta pela fila.';
	@override String get AI_PROVIDER_UNAVAILABLE => 'O provedor de IA está indisponível agora. Tente de novo em instantes.';
	@override String get FUNNEL_STAGE_BACKWARD => 'Um lead já contatado não volta para o começo do funil. Marque como perdido ou crie outro lead.';
}

// Path: audio_recorder
class _Translations$audio_recorder$pt extends Translations$audio_recorder$en {
	_Translations$audio_recorder$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get permission_blocked_title => 'Microfone bloqueado';
	@override String get permission_blocked_body => 'Permita o acesso ao microfone nas configurações do navegador/sistema e tente novamente.';
	@override String get ok => 'OK';
	@override String get cancel_tooltip => 'Cancelar';
	@override String get pause_tooltip => 'Pausar';
	@override String get resume_tooltip => 'Continuar';
	@override String get stop_tooltip => 'Finalizar gravação';
	@override String get discard_tooltip => 'Descartar';
	@override String get redo_tooltip => 'Regravar';
	@override String get play_tooltip => 'Reproduzir';
	@override String get pause_play_tooltip => 'Pausar';
	@override String get send_tooltip => 'Enviar áudio';
	@override String start_failed({required Object error}) => 'Falha ao iniciar gravação: ${error}';
	@override String pause_failed({required Object error}) => 'Falha ao pausar: ${error}';
	@override String stop_failed({required Object error}) => 'Falha ao parar gravação: ${error}';
	@override String send_failed({required Object error}) => 'Falha ao enviar áudio: ${error}';
	@override String get empty_recording => 'Gravação vazia.';
	@override String get empty_audio => 'Áudio vazio.';
}

// Path: bulk_actions
class _Translations$bulk_actions$pt extends Translations$bulk_actions$en {
	_Translations$bulk_actions$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get clear_tooltip => 'Limpar seleção';
	@override String selected_count({required Object count}) => '${count} selecionada(s)';
	@override String get select_all => 'Selecionar tudo';
	@override String get close_btn => 'Encerrar';
	@override String close_confirm_title({required Object count}) => 'Encerrar ${count} conversas?';
	@override String get close_confirm_body => 'As conversas serão movidas para "Fechadas". Mensagens novas dos clientes reabrem automaticamente.';
	@override String close_done({required Object ok, required Object total}) => '${ok} de ${total} encerradas.';
	@override String get close_partial_warning => ' Algumas falharam — verifique permissão.';
	@override String close_failed({required Object error}) => 'Falha: ${error}';
}

// Path: image_viewer
class _Translations$image_viewer$pt extends Translations$image_viewer$en {
	_Translations$image_viewer$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get default_filename => 'Imagem';
	@override String get default_save_filename => 'imagem.jpg';
	@override String get download_btn => 'Baixar';
	@override String get close_tooltip => 'Fechar';
	@override String get download_failed => 'Não foi possível baixar a imagem.';
}

// Path: media_send
class _Translations$media_send$pt extends Translations$media_send$en {
	_Translations$media_send$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String file_too_large({required Object actualMb, required Object maxMb}) => 'Arquivo muito grande (${actualMb} MB). O limite para este tipo é ${maxMb} MB.';
	@override String sending_file({required Object filename}) => 'Enviando ${filename}...';
	@override String get send_file_dialog_title => 'Enviar arquivo';
	@override String get caption_label => 'Legenda (opcional)';
	@override String get send_btn => 'Enviar';
	@override String get cancel_btn => 'Cancelar';
	@override String get upload_in_progress => 'Já existe um arquivo sendo enviado nesta conversa. Aguarde concluir.';
}

// Path: reply_suggestions
class _Translations$reply_suggestions$pt extends Translations$reply_suggestions$en {
	_Translations$reply_suggestions$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get loading => 'Gerando sugestões...';
	@override String get load_failed => 'Falha ao gerar sugestões. Tente novamente.';
	@override String get retry => 'Repetir';
	@override String get suggest_replies => 'Sugerir respostas';
	@override String get refresh_tooltip => 'Atualizar sugestões';
	@override String get hide_tooltip => 'Esconder';
}

// Path: pwa
class _Translations$pwa$pt extends Translations$pwa$en {
	_Translations$pwa$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get install_title => 'Instale o app';
	@override String get install_body => 'Receba notificações quando seus clientes mandarem mensagem, mesmo com o app fechado.';
	@override String get install_btn => 'Instalar';
	@override String get not_now => 'Agora não';
	@override String get ios_title => 'Adicione à tela de início';
	@override String get ios_body => 'Necessário para receber notificações no iPhone — o Safari só permite alertas em apps instalados.';
	@override String get dismiss_tooltip => 'Dispensar';
	@override String get ios_step_1 => 'Toque em Compartilhar na barra inferior do Safari';
	@override String get ios_step_2 => 'Escolha "Adicionar à Tela de Início"';
	@override String get ios_step_3 => 'Abra o Replider pelo ícone instalado e ative as notificações';
}

// Path: audio_player
class _Translations$audio_player$pt extends Translations$audio_player$en {
	_Translations$audio_player$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get transcribe_tooltip => 'Transcrever';
	@override String get transcribing => 'Transcrevendo...';
	@override String get transcribe_failed => 'Falha ao transcrever, tente novamente.';
	@override String get load_failed => 'Não foi possível carregar o áudio. Ele pode ter expirado.';
}

// Path: media_preview
class _Translations$media_preview$pt extends Translations$media_preview$en {
	_Translations$media_preview$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get audio => 'Áudio';
	@override String get image => 'Imagem';
	@override String get video => 'Vídeo';
	@override String get document => 'Arquivo';
	@override String get location => 'Localização';
	@override String get sticker => 'Sticker';
}

// Path: sentiment
class _Translations$sentiment$pt extends Translations$sentiment$en {
	_Translations$sentiment$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String detected_tooltip({required Object label}) => 'Sentimento detectado: ${label}';
	@override String get angry => 'Irritado';
	@override String get negative => 'Negativo';
	@override String get positive => 'Positivo';
}

// Path: owner
class _Translations$owner$pt extends Translations$owner$en {
	_Translations$owner$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get menu_label => 'Painel do Owner';
	@override String get accounts_title => 'Contas';
	@override String get search_hint => 'Buscar por empresa ou e-mail';
	@override String get create_contract_cta => 'Novo contrato';
	@override String get empty_title => 'Nenhuma conta ainda';
	@override String get empty_desc => 'Use "Novo contrato" para provisionar seu primeiro cliente B2B.';
	@override String get badge_suspended => 'suspensa';
	@override String get detail_title => 'Conta';
	@override String get field_id => 'ID';
	@override String get field_status => 'Status';
	@override String get field_owner => 'Owner';
	@override String get field_plan => 'Plano';
	@override String get field_created => 'Criada em';
	@override String get field_internal_threads => 'Threads de suporte interno ativas';
	@override String get field_suspended_reason => 'Motivo da suspensão';
	@override String get seller_label => 'Vendedor';
	@override String get seller_none => 'Sem vendedor vinculado';
	@override String get seller_source_label => 'Origem do lead';
	@override String get seller_source_seller_ref => 'Prospecção (ref) — 50%';
	@override String get seller_source_inbound => 'Inbound — 20%';
	@override String get seller_linked_at => 'Vinculado em';
	@override String get seller_edit_cta => 'Vincular / realocar';
	@override String get seller_self_match_warning => 'Vendedor coincide com o próprio admin desta conta (autoindicação) — revise o vínculo';
	@override String get seller_dialog_title => 'Vincular vendedor';
	@override String get seller_email_hint => 'E-mail do vendedor (usuário da conta plataforma)';
	@override String get seller_unlink_cta => 'Desvincular';
	@override String get seller_save_cta => 'Salvar';
	@override String get seller_saved => 'Vendedor atualizado.';
	@override String get overrides_label => 'Limites (overrides)';
	@override String get overrides_clear_hint => 'Deixe vazio para remover o override e voltar ao default do plano.';
	@override String get edit_overrides_cta => 'Editar limites';
	@override String get cap_max_agents => 'Máx. agentes';
	@override String get cap_max_channels => 'Máx. canais';
	@override String get cap_max_conversations => 'Máx. conversas / mês';
	@override String get cap_max_tokens => 'Máx. tokens IA / mês';
	@override String get usage_label => 'Uso no período';
	@override String get usage_tokens => 'Tokens usados';
	@override String get usage_addon => 'Tokens adicionais';
	@override String get usage_conversations => 'Conversas';
	@override String get usage_messages => 'Mensagens';
	@override String get suspend_cta => 'Suspender conta';
	@override String get suspend_dialog_title => 'Suspender conta?';
	@override String get suspend_reason_hint => 'Motivo (obrigatório)';
	@override String get suspend_confirm_cta => 'Suspender';
	@override String get reactivate_cta => 'Reativar conta';
	@override String get reactivate_confirm_title => 'Reativar essa conta?';
	@override String get reactivate_confirm_msg => 'Mensagens recebidas e gravações voltam a funcionar imediatamente.';
	@override String get create_title => 'Novo contrato';
	@override String get create_company => 'Nome da empresa';
	@override String get create_admin_email => 'E-mail do admin';
	@override String get create_admin_name => 'Nome do admin';
	@override String get create_plan => 'Plano';
	@override String get create_trial_bypass => 'Pular o gate de trial';
	@override String get create_trial_bypass_hint => 'A conta fica utilizável imediatamente sem assinatura Stripe.';
	@override String get create_submit => 'Criar contrato';
	@override String get form_required => 'Preencha empresa, e-mail do admin e plano.';
	@override String get top_label => 'Maiores consumos no período';
	@override String get top_open_cta => 'Abrir';
	@override String get grant_cta => 'Conceder créditos';
	@override String get grant_dialog_title => 'Conceder tokens adicionais';
	@override String get grant_hint => 'Quantidade de tokens';
	@override String get extend_cta => 'Estender trial';
	@override String get extend_dialog_title => 'Estender trial por';
	@override String get extend_hint => 'Dias a partir de agora';
	@override String get impersonate_cta => 'Entrar como essa conta';
	@override String get impersonate_confirm_title => 'Entrar como essa conta?';
	@override String get impersonate_confirm_msg => 'Você vai atuar como o admin da conta por 15 minutos. Cada ação fica auditada.';
	@override String banner_acting_as({required Object account}) => 'Atuando como ${account} — cada ação está sendo registrada.';
	@override String get banner_end_cta => 'Sair';
	@override String banner_expires_in({required Object time}) => 'expira em ${time}';
	@override String get filter_all => 'Todas';
	@override String get filter_active => 'Ativas';
	@override String get filter_trialing => 'Em trial';
	@override String get filter_past_due => 'Em atraso';
	@override String get filter_suspended => 'Suspensas';
	@override String cap_badge({required Object pct}) => '${pct}% do limite';
	@override String get logout_blocked_during_impersonate => 'Encerre a sessão de impersonate antes de sair.';
	@override String get conversations_label => 'Conversas recentes';
	@override String get conversations_empty => 'Essa conta ainda não tem conversas.';
	@override String get health_accounts => 'Contas';
	@override String get health_active => 'Ativas';
	@override String get health_suspended => 'Suspensas';
	@override String get health_conv_24h => 'Conv 24h';
	@override String get health_msg_24h => 'Msg 24h';
	@override String get usage_trend_label => 'Tendência de uso';
	@override String get usage_trend_desc => 'Tokens de IA consumidos por dia, últimos 30 dias.';
	@override String get usage_trend_empty => 'Sem uso nos últimos 30 dias.';
	@override String usage_trend_total({required Object tokens}) => '${tokens} tokens no total';
	@override String get invoices_label => 'Faturas';
	@override String get invoices_desc => 'Últimas 10 faturas do Stripe para esta conta.';
	@override String get invoices_empty => 'Sem faturas — conta ainda não transacionou no Stripe.';
	@override String get invoice_status_paid => 'Paga';
	@override String get invoice_status_open => 'Em aberto';
	@override String get invoice_status_draft => 'Rascunho';
	@override String get invoice_status_uncollectible => 'Inadimplente';
	@override String get invoice_status_void => 'Anulada';
	@override String get invoice_open_hosted => 'Abrir página da fatura';
	@override String get invoice_open_pdf => 'Baixar PDF';
	@override String get gate_enroll_title => 'Ative o 2FA para continuar';
	@override String get gate_enroll_body => 'O painel do proprietário exige autenticação em duas etapas. Configure no seu perfil e volte aqui.';
	@override String get gate_enroll_cta => 'Ir para o perfil';
	@override String get gate_refresh_title => 'Reconfirme seu código 2FA';
	@override String get gate_refresh_body => 'Sua validação de 2 fatores expirou. Digite o código do autenticador para continuar — sem sair da sua sessão.';
	@override String get gate_refresh_cta => 'Confirmar código';
	@override String get stepup_title => 'Confirme seu 2FA';
	@override String get stepup_body => 'Sua validação de 2 fatores expirou. Digite o código do autenticador para continuar — nada do que você preencheu é perdido.';
	@override String get stepup_code => 'Código de 6 dígitos';
	@override String get stepup_confirm => 'Confirmar';
	@override String get stepup_hint => 'Você também pode usar um código de recuperação.';
	@override String get coupons_title => 'Cupons';
	@override String get coupons_empty_title => 'Sem cupons ainda';
	@override String get coupons_empty_desc => 'Crie um código e compartilhe o link de convite para aplicar desconto no checkout.';
	@override String get coupon_create_cta => 'Novo cupom';
	@override String get coupon_create_title => 'Criar cupom';
	@override String get coupon_code_label => 'Código';
	@override String get coupon_percent_label => 'Desconto';
	@override String get coupon_max_label => 'Máx. de resgates';
	@override String get coupon_max_hint => 'Deixe em branco para ilimitado';
	@override String get coupon_expires_pick => 'Definir expiração';
	@override String get coupon_expires_none => 'Sem expiração';
	@override String coupon_expires({required Object date}) => 'Expira em ${date}';
	@override String get coupon_save_cta => 'Criar';
	@override String get coupon_form_invalid => 'Código e desconto são obrigatórios';
	@override String get coupon_created_toast => 'Cupom criado';
	@override String get coupon_status_active => 'ATIVO';
	@override String get coupon_status_inactive => 'INATIVO';
	@override String coupon_percent_off({required Object percent}) => '${percent}% off';
	@override String coupon_redeemed_unlimited({required Object used}) => '${used} resgates';
	@override String coupon_redeemed_capped({required Object used, required Object cap}) => '${used} / ${cap} resgatados';
	@override String get coupon_deactivate_cta => 'Desativar';
	@override String get coupon_deactivate_title => 'Desativar cupom?';
	@override String coupon_deactivate_body({required Object code}) => 'O código ${code} vai parar de aceitar novos resgates. Assinaturas existentes mantêm o desconto.';
	@override String get tab_overview => 'Visão geral';
	@override String get tab_usage => 'Uso';
	@override String get tab_invoices => 'Faturas';
	@override String get tab_conversations => 'Conversas';
	@override String get actions_menu => 'Ações';
	@override String get conversations_tap_hint => 'Toque numa conversa para entrar nesta conta';
	@override String get sort_label => 'Ordenar';
	@override String get sort_consumption => 'Consumo';
	@override String get sort_recent => 'Recentes';
	@override String get sort_status => 'Status';
	@override String get overdue_first => 'Em atraso no topo';
	@override String get usage_no_cap => 'Sem limite';
	@override String get mode_new_account => 'Nova conta';
	@override String get mode_existing_account => 'Conta existente';
	@override String get pick_account_hint => 'Busque a conta para aplicar o contrato';
	@override String get pick_account_label => 'Conta';
	@override String get apply_contract_submit => 'Aplicar contrato';
	@override String get pick_account_required => 'Selecione uma conta primeiro';
	@override String get wa_risk_ban => 'Risco de ban';
	@override String get wa_quality_warning => 'Qualidade WhatsApp';
	@override String get apply_plan_cta => 'Definir plano / contrato';
	@override String get use_our_key_title => 'Usar nossa chave de IA';
	@override String get use_our_key_hint => 'Cobra pela chave da plataforma na tarifa do plano, sem exigir que o cliente traga a dele.';
	@override String get provider_costs_title => 'Custos de provider';
	@override String get provider_costs_empty_title => 'Nenhum preço ainda';
	@override String get provider_costs_empty_desc => 'Cadastre o preço real da API por modelo para o lucro mensal ser calculado sobre o custo verdadeiro.';
	@override String get provider_cost_add_cta => 'Adicionar preço';
	@override String get provider_cost_add_title => 'Novo preço de provider';
	@override String get provider_cost_edit_title => 'Corrigir preço';
	@override String get provider_cost_provider_label => 'Provider';
	@override String get provider_cost_model_label => 'Modelo';
	@override String get provider_cost_input_label => 'Input / 1M tokens';
	@override String get provider_cost_output_label => 'Output / 1M tokens';
	@override String provider_cost_costs_line({required Object input, required Object output}) => 'in ${input}¢ / out ${output}¢ USD por 1M tokens';
	@override String provider_cost_effective_from({required Object date}) => 'Vigente desde ${date}';
	@override String get provider_cost_pick_date => 'Mudar data';
	@override String get provider_cost_form_invalid => 'Preencha todos os campos com valores válidos.';
	@override String get provider_cost_saved_toast => 'Preço salvo.';
	@override String get provider_cost_delete_title => 'Excluir preço?';
	@override String provider_cost_delete_body({required Object model}) => 'Remover a linha de preço ${model}? Cálculos de lucro passados que a usaram passam a pegar o preço vigente anterior.';
	@override String get exchange_rate_title => 'Câmbio USD → BRL';
	@override String exchange_rate_value({required Object rate}) => 'R\$ ${rate} por US\$ 1';
	@override String get exchange_rate_edit_cta => 'Editar';
	@override String get exchange_rate_label => 'BRL por 1 USD';
	@override String get exchange_rate_saved => 'Câmbio salvo.';
	@override String get payouts_title => 'Pagamentos de vendedores';
	@override String get payouts_cycle_label => 'Ciclo';
	@override String get payouts_total_due => 'Total a pagar';
	@override String get payouts_total_paid => 'Total pago';
	@override String get payouts_empty_title => 'Nenhum pagamento neste ciclo';
	@override String get payouts_empty_desc => 'Nenhum lançamento de comissão encontrado para este ciclo.';
	@override String payouts_pending_line({required Object amount}) => 'Pendente (ciclo): ${amount}';
	@override String payouts_adjustment_line({required Object amount, required Object count}) => 'Ajustes de ciclos anteriores: ${amount} (${count} lançamentos)';
	@override String payouts_paid_line({required Object amount}) => 'Pago neste ciclo: ${amount}';
	@override String payouts_total_due_line({required Object amount}) => 'Total a pagar: ${amount}';
	@override String get payouts_pix_label => 'Copiar chave PIX';
	@override String get payouts_pix_missing => 'Sem chave PIX cadastrada';
	@override String get payouts_pix_copied => 'Chave PIX copiada.';
	@override String get payouts_settled_chip => 'Liquidado';
	@override String get payouts_mark_paid_cta => 'Marcar como pago';
	@override String get payouts_mark_paid_title => 'Confirmar pagamento';
	@override String payouts_mark_paid_body({required Object amount, required Object name}) => 'Liquidar ${amount} para ${name}? Todos os lançamentos pendentes até este ciclo ficam pagos.';
	@override String get payouts_note_label => 'Nº da transação PIX (opcional)';
	@override String get payouts_marked_toast => 'Pagamento marcado como pago.';
	@override String get payouts_open_cycle_title => 'Ciclo ainda aberto';
	@override String get payouts_open_cycle_body => 'Este ciclo ainda não fechou — você liquidaria um valor parcial, e qualquer recálculo posterior vira ajuste. Pagar o ciclo em aberto mesmo assim?';
	@override String get payouts_open_cycle_cta => 'Pagar ciclo aberto';
	@override String get payouts_export_cta => 'Exportar CSV';
	@override String get payouts_export_copied => 'CSV copiado para a área de transferência.';
	@override String get sellers_title => 'Vendedores';
	@override String get sellers_subtitle => 'Usuários da conta plataforma. Ligar o toggle de vendedor marca o usuário, gera o link de cadastro e entra no rodízio de leads — tudo de uma vez.';
	@override String get sellers_empty_title => 'Nenhum usuário na plataforma';
	@override String get sellers_empty_desc => 'Crie atendentes em Configurações → Equipe primeiro, depois marque-os como vendedores aqui.';
	@override String get sellers_is_seller_label => 'É vendedor';
	@override String get sellers_active_label => 'Ativo (SLA de comissão)';
	@override String get sellers_in_rotation_chip => 'Na rotação';
	@override String get sellers_out_rotation_chip => 'Fora da rotação';
	@override String get sellers_ref_link_label => 'Copiar link de cadastro';
	@override String get sellers_link_copied => 'Link de cadastro copiado.';
	@override String get sellers_pix_missing => 'Sem chave PIX cadastrada';
	@override String sellers_carteira_line({required Object count}) => '${count} conta(s) na carteira';
	@override String get sellers_onboarded_toast => 'Vendedor ativado — flag, link e rodízio configurados.';
	@override String get sellers_offboarded_toast => 'Vendedor desativado — removido do rodízio; histórico mantido.';
	@override String get sellers_activity_toast => 'Atividade atualizada.';
	@override String get sellers_admin_warn_title => 'Comissionar um admin?';
	@override String sellers_admin_warn_body({required Object name}) => '${name} é admin — marcar como vendedor gera comissão e SLA para ele. Continuar?';
	@override String get sellers_admin_warn_confirm => 'Marcar como vendedor';
	@override String get invite_title => 'Convidar cliente';
	@override String get invite_email => 'E-mail do cliente';
	@override String get invite_plan => 'Plano contratado';
	@override String get invite_days => 'Validade (dias)';
	@override String get invite_create => 'Gerar convite';
	@override String get invite_hint => 'A conta só pode ser criada com este link, por este e-mail, e já nasce no plano escolhido.';
	@override String get invite_link_title => 'Link do convite — aparece uma vez';
	@override String get invite_link_hint => 'Copie AGORA e mande pelo canal onde a venda aconteceu. O servidor guarda só um hash: fechar esta janela perde o link (revogue e reemita).';
	@override String get invite_copy => 'Copiar link';
	@override String get invite_copied => 'Link copiado.';
}

// Path: sales
class _Translations$sales$pt extends Translations$sales$en {
	_Translations$sales$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get nav_tooltip => 'Vendas Ativas';
	@override String get back_tooltip => 'Voltar';
	@override String get funnel_new_title => 'Novo funil';
	@override String get funnel_detail_title => 'Funil';
	@override late final _Translations$sales$products$pt products = _Translations$sales$products$pt._(_root);
	@override late final _Translations$sales$funnels$pt funnels = _Translations$sales$funnels$pt._(_root);
	@override late final _Translations$sales$schema$pt schema = _Translations$sales$schema$pt._(_root);
	@override late final _Translations$sales$webhook$pt webhook = _Translations$sales$webhook$pt._(_root);
	@override late final _Translations$sales$lead_profile$pt lead_profile = _Translations$sales$lead_profile$pt._(_root);
	@override late final _Translations$sales$leads$pt leads = _Translations$sales$leads$pt._(_root);
	@override late final _Translations$sales$prospecting$pt prospecting = _Translations$sales$prospecting$pt._(_root);
	@override String get hunt_tab => 'Caçar';
}

// Path: support
class _Translations$support$pt extends Translations$support$en {
	_Translations$support$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Suporte';
	@override String get subtitle => 'Seu gerente de conta Replider';
	@override String get nav_tooltip => 'Suporte';
	@override String get empty_title => 'Fale com a gente';
	@override String get empty_subtitle => 'Dúvidas sobre sua conta, canais ou cobrança? Envie uma mensagem — nosso time responde por aqui.';
	@override String get input_hint => 'Digite uma mensagem...';
	@override String get load_error => 'Não foi possível carregar sua conversa com o suporte.';
	@override String get retry => 'Tentar novamente';
	@override String get push_prompt_title => 'Receba avisos das respostas';
	@override String get push_prompt_body => 'Ative as notificações e avisamos quando seu gerente de conta responder, mesmo com o painel fechado.';
	@override String get push_prompt_enable => 'Ativar notificações';
	@override String get push_prompt_later => 'Agora não';
}

// Path: claude_integration
class _Translations$claude_integration$pt extends Translations$claude_integration$en {
	_Translations$claude_integration$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Claude (IA)';
	@override String get back_tooltip => 'Voltar';
	@override String load_failed({required Object error}) => 'Falha ao carregar: ${error}';
	@override String get copy_tooltip => 'Copiar';
	@override String get copied => 'Copiado.';
	@override String get connect_title => 'Conectar ao Claude';
	@override String get connect_subtitle => 'Gerencie sua conta conversando com o Claude — configurar o agente, responder clientes, ver métricas. Funciona no site claude.ai e nos apps de celular e computador.';
	@override String get tokens_title => 'Tokens de acesso';
	@override String get tokens_subtitle => 'Tokens pessoais para o Claude Code. Substitua SEU_TOKEN no comando acima. O token completo aparece só uma vez.';
	@override String get tokens_create => 'Criar token';
	@override String get tokens_empty => 'Nenhum token criado.';
	@override String get tokens_never_used => 'nunca usado';
	@override String tokens_last_used({required Object date}) => 'usado em ${date}';
	@override String get tokens_revoke_tooltip => 'Revogar token';
	@override String get create_dialog_title => 'Novo token de acesso';
	@override String get create_dialog_name_label => 'Nome (ex.: Claude Code do Tiago)';
	@override String get create_dialog_confirm => 'Criar';
	@override String get token_dialog_title => 'Copie o token agora';
	@override String get token_dialog_body => 'Esta é a única vez que o token será exibido. Copie e cole no Claude — não é possível recuperá-lo depois.';
	@override String get token_dialog_copy => 'Copiar';
	@override String get token_copied => 'Token copiado.';
	@override String get token_dialog_done => 'Já copiei';
	@override String get revoke_token_title => 'Revogar token?';
	@override String get revoke_token_body => 'O Claude conectado com este token perderá o acesso imediatamente.';
	@override String get revoke_confirm => 'Revogar';
	@override String get connections_title => 'Conexões ativas';
	@override String get connections_subtitle => 'Aplicativos autorizados via OAuth (ex.: conector do claude.ai).';
	@override String get connections_empty => 'Nenhuma conexão ativa.';
	@override String get connections_revoke => 'Revogar';
	@override String get revoke_connection_title => 'Revogar conexão?';
	@override String get revoke_connection_body => 'Este aplicativo perderá o acesso à sua conta Replider imediatamente.';
	@override String get connect_step1 => 'Copie o endereço abaixo.';
	@override String get connect_step2 => 'No Claude, vá em Configurações → Conectores → Adicionar conector personalizado e cole o endereço.';
	@override String get connect_step3 => 'Clique em conectar e entre com o mesmo e-mail e senha que você usa aqui na Replider.';
	@override String get connect_no_token_note => 'Não precisa de token nem instalar nada — é só entrar com seu e-mail e senha. (Conectores personalizados exigem um plano Claude Pro, Max, Team ou Enterprise.)';
	@override String get advanced_title => 'Avançado — Claude Code';
	@override String get advanced_subtitle => 'Para quem usa o Claude Code no terminal. Se você usa o Claude no site ou nos apps, ignore esta seção.';
	@override String get cc_oauth_label => 'Recomendado — login pelo navegador (sem token)';
	@override String get cc_oauth_hint => 'Rode o comando no terminal. O Claude Code abre o navegador, você entra com seu e-mail e senha da Replider e volta pro terminal automaticamente.';
	@override String get cc_token_label => 'Alternativa — com token (para servidor/CI, sem navegador)';
	@override String get cc_token_hint => 'Use quando não há navegador disponível. Crie um token abaixo e substitua SEU_TOKEN no comando.';
}

// Path: shell
class _Translations$shell$pt extends Translations$shell$en {
	_Translations$shell$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get start => 'Início';
	@override String get funnel => 'Clientes';
	@override String get conversations => 'Conversas';
	@override String get offers => 'Oportunidades';
	@override String get copilot => 'Copiloto';
	@override String get copilot_soon_title => 'Copiloto em construção';
	@override String get copilot_soon_body => 'Por enquanto, converse com a Replider pelo conector MCP no Claude ou no ChatGPT — as mesmas ações do app estão lá.';
	@override String get start_title => 'Seu dia';
	@override String get start_subtitle => 'Atalhos para onde o trabalho acontece.';
	@override String get start_go_funnel => 'Ver o funil';
	@override String get start_go_conversations => 'Abrir conversas';
	@override String get start_go_prospecting => 'Prospectar empresas';
	@override String get stat_active_leads => 'Leads ativos';
	@override String get stat_pending_approval => 'Aguardando aprovação';
	@override String get stat_unowned => 'Sem dono';
	@override String get needs_you_title => 'PRECISA DE VOCÊ';
	@override String get needs_you_clear => 'Nada esperando por você agora.';
	@override String get needs_open => 'Abrir';
	@override String needs_approvals({required Object count}) => '${count} abordagem(ns) aguardando sua aprovação';
	@override String needs_unowned({required Object count}) => '${count} lead(s) sem dono — ninguém está cuidando';
	@override String get needs_empty_pipeline => 'Seu funil está vazio. Comece caçando clientes.';
}

// Path: board
class _Translations$board$pt extends Translations$board$en {
	_Translations$board$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get move_to => 'Mover para';
	@override String get score => 'Score';
	@override String get unassigned => 'Sem dono';
	@override String get empty_column => 'Nenhum lead aqui';
	@override String get empty_board => 'Este funil ainda não tem etapas.';
	@override String get select_funnel => 'Escolha um funil';
	@override String get board_title => 'Funil';
	@override String get new_lead => 'Novo lead';
	@override String get new_lead_name => 'Contato';
	@override String get new_lead_name_hint => 'Quem decide ou compra';
	@override String get new_lead_phone => 'Telefone / WhatsApp';
	@override String get new_lead_phone_hint => '+55 65 99999-0000';
	@override String get new_lead_created => 'Lead adicionado ao funil.';
	@override String get new_lead_existed => 'Esse contato já está no funil.';
	@override String get no_funnel_yet => 'Você ainda não tem um funil.';
	@override String get search_hint => 'Buscar empresa, cidade ou contato';
	@override String get search_clear => 'Limpar busca';
	@override String filter_all({required Object count}) => 'Todos · ${count}';
	@override String filter_responded({required Object count}) => 'Responderam · ${count}';
	@override String filter_awaiting({required Object count}) => 'Esperando aprovação · ${count}';
	@override String filter_stalled({required Object count}) => 'Parados há mais de 7 dias · ${count}';
	@override String column_showing({required Object showing, required Object total}) => '${showing} de ${total} · mais parados primeiro';
	@override String column_showing_manual({required Object showing, required Object total}) => '${showing} de ${total} · ordem manual';
	@override String get order_reset => 'Ordenar por parados';
	@override String column_no_match({required Object query}) => 'Nenhum resultado para “${query}” nesta etapa.';
	@override String get column_no_match_hint => 'Pode estar em outra etapa, ou ainda não ter sido importado.';
	@override String truncated_notice({required Object count}) => 'Mostrando os ${count} leads mais recentes deste funil. A busca cobre só eles.';
	@override String idle_days({required Object count}) => '${count}d parado';
	@override String get idle_today => 'Movido hoje';
	@override String get idle_unknown => 'Sem movimentação registrada';
	@override String get view_pipeline => 'Pipeline';
	@override String get tag_draft_ready => 'Rascunho em Hoje →';
	@override String get tag_buy_intent => 'Quer comprar';
	@override String get tag_qualified => 'Qualificado';
	@override String get tag_idle_risk => 'Esfriando';
	@override String get field_required => 'Obrigatório';
	@override String get funnel_fields => 'Campos do funil';
	@override String get funnel_fields_saved => 'Campo salvo';
	@override String get funnel_fields_empty => 'Este funil não pede nenhum campo extra.';
	@override String get owner => 'Responsável';
	@override String get owner_none => 'Ninguém ainda';
	@override String get owner_assign => 'Definir responsável';
	@override String get visibility => 'Visibilidade';
	@override String get visibility_team => 'Time todo';
	@override String get visibility_private => 'Só o responsável';
	@override String get visibility_private_hint => 'Some do funil de quem não é o responsável.';
	@override String get sources => 'De onde veio';
	@override String get sources_hint => 'A IA montou o perfil a partir destas páginas.';
	@override String get bulk_assign => 'Atribuir sem responsável';
	@override String get bulk_assign_title => 'Atribuir leads sem responsável';
	@override String bulk_assign_body({required Object count}) => '${count} leads deste funil não têm responsável. Passar todos para:';
	@override String get bulk_assign_filtered => 'Conta só o que está visível com a busca e o filtro atuais.';
	@override String bulk_assign_confirm({required Object count}) => 'Atribuir ${count}';
	@override String bulk_assign_done({required Object count}) => '${count} leads atribuídos';
	@override String bulk_assign_partial({required Object count, required Object skipped}) => '${count} atribuídos · ${skipped} não estavam mais lá';
	@override String get bulk_assign_cap => 'Máximo de 200 por vez. Serão atribuídos os 200 primeiros.';
	@override String get bulk_assign_here => 'Atribuir nesta busca';
	@override String get new_lead_subtitle => 'A empresa e a pessoa com quem você fala lá.';
	@override String get new_lead_company => 'Empresa';
	@override String get new_lead_company_hint => 'ex: Hospital Regional de Cuiabá';
	@override String get new_lead_company_required => 'Informe a empresa.';
	@override String get new_lead_role => 'Cargo';
	@override String get new_lead_role_hint => 'ex: Gerente de compras';
	@override String get new_lead_email => 'E-mail';
	@override String get new_lead_email_hint => 'compras@hospital.com.br';
	@override String get new_lead_city => 'Cidade';
	@override String get new_lead_city_hint => 'ex: Cuiabá, MT';
	@override String get new_lead_reach_help => 'Telefone ou e-mail — pelo menos um, para a oferta ter por onde sair.';
	@override String get new_lead_reach_required => 'Informe um telefone ou um e-mail.';
	@override String get new_lead_submit => 'Adicionar';
	@override String get configure_stages => 'Configurar etapas';
	@override String get tag_responded => 'Respondeu';
	@override String column_hint({required Object status}) => 'Leads com status "${status}". Movem sozinhos no envio e na resposta — ou arraste.';
	@override String column_hint_terminal({required Object status}) => 'Etapa final — leads com status "${status}".';
	@override String get panel_open_in_queue => 'Abrir rascunho em Hoje';
	@override String get panel_cnes_title => 'Parque no CNES';
	@override String get panel_cnes_by_phone => 'dado público · bate pelo telefone · quantidade, não idade';
	@override String get panel_cnes_by_name => 'dado público · nome único no cadastro · quantidade, não idade';
	@override String panel_cnes_month({required Object month}) => 'dados CNES de ${month}';
	@override String panel_cnes_code({required Object code}) => 'código CNES ${code}';
	@override String get panel_offer_title => 'O que você pode oferecer';
	@override String panel_offer_line({required Object product, required Object count, required Object unit, required Object equipment}) => '${product} — eles têm ${count} ${unit} de ${equipment}';
	@override String get panel_offer_none => 'Nenhum produto do seu catálogo casa com o parque declarado';
	@override String get panel_offer_free => 'cruzamento do seu catálogo com o registro — sem IA';
	@override String panel_offer_tail({required Object count, required Object unit, required Object equipment}) => ' — eles têm ${count} ${unit} de ${equipment}';
	@override String panel_offer_open({required Object product}) => 'Abrir ficha de ${product}';
}

// Path: lead_detail
class _Translations$lead_detail$pt extends Translations$lead_detail$en {
	_Translations$lead_detail$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get profile => 'Perfil';
	@override String get approach => 'Abordagem';
	@override String get approach_hint => 'Escreva ou gere a mensagem de abordagem…';
	@override String get regenerate => 'Gerar outra';
	@override String get generate => 'Gerar abordagem';
	@override String get approve_send => 'Aprovar e enviar';
	@override String get timeline => 'Histórico';
	@override String get confirm_title => 'Confirmar envio';
	@override String get confirm_to => 'Para';
	@override String get sent => 'Abordagem enviada.';
	@override String get read_only => 'Você pode ver este lead, mas ele é de outro vendedor.';
	@override String get queue_title => 'Fila de aprovação';
	@override String get queue_empty => 'Nada para aprovar agora.';
	@override String get queue_empty_hint => 'Quando a IA rascunhar abordagens, elas esperam sua aprovação aqui.';
	@override String queue_remaining({required Object count}) => '${count} restantes';
	@override String get discard => 'Descartar';
	@override String get edit => 'Editar';
	@override String get discarded => 'Rascunho descartado — o lead volta ao funil.';
	@override String get undo => 'Desfazer';
	@override String get send_channel => 'Sai por';
	@override String get bulk_needs_template => 'WhatsApp precisa de template aprovado — abra o lead';
	@override String get check_title => 'Verificação automática';
	@override String get check_clean => 'Nenhum padrão de promessa de prazo, preço ou resultado encontrado no texto.';
	@override String check_found({required Object count}) => '${count} trecho(s) para você conferir antes de enviar.';
	@override String get check_edited => 'Você editou o texto. A verificação abaixo é do rascunho salvo — este texto ainda não foi verificado.';
	@override String get check_kind_promise => 'Compromisso assumido no seu nome';
	@override String get check_kind_invented_fact => 'Afirmação sobre o cliente sem base nos dados';
	@override String get check_kind_out_of_scope => 'Fora do que a empresa declarou atender';
	@override String get check_kind_placeholder => 'Campo de preencher que sobrou';
	@override String get check_kind_tone => 'Tratamento inadequado ao contexto';
	@override String get review_action => 'Revisar com IA';
	@override String get review_running => 'Lendo a mensagem…';
	@override String get review_clean => 'Revisado por uma segunda IA: nada a apontar.';
	@override String review_found({required Object count}) => 'A segunda IA apontou ${count} ponto(s).';
	@override String get review_unavailable => 'A revisão por IA não rodou agora. A verificação automática acima continua valendo.';
	@override String written_by_ai({required Object source}) => 'Escrito pela IA a partir de: ${source}';
	@override String get written_by_ai_unknown => 'Escrito pela IA';
	@override String get shortcut_approve => 'aprovar';
	@override String get shortcut_edit => 'editar';
	@override String get shortcut_discard => 'descartar';
	@override String get shortcut_move => 'navegar';
	@override String get contact => 'Contato';
	@override String get call => 'Ligar';
	@override String get open_conversation => 'Abrir conversa';
	@override String get no_conversation => 'Ainda não há conversa com este contato.';
	@override String get enrich => 'Pesquisar esta empresa';
	@override String get enrich_again => 'Pesquisar de novo';
	@override String get enriching => 'Pesquisando…';
	@override String get enrich_started => 'Pesquisando a empresa. Volte em instantes.';
	@override String get profile_empty => 'Ainda não sabemos nada sobre esta empresa.';
	@override String get profile_empty_hint => 'A pesquisa busca decisor, atividade e endereço para a abordagem citar algo real em vez de um texto genérico.';
	@override String get profile_researching => 'Pesquisando esta empresa agora. O resultado aparece aqui.';
	@override String get profile_found_nothing => 'A pesquisa não encontrou dados públicos sobre esta empresa.';
	@override String get profile_found_nothing_hint => 'Acontece com empresas pequenas ou sem presença online. Escreva a abordagem com o que você já sabe, ou pesquise de novo mais tarde.';
	@override String get profile_blocked => 'Não deu para pesquisar: este lead não tem nome de empresa nem site.';
	@override String get profile_blocked_hint => 'A pesquisa parte do nome ou do site para achar o decisor. Sem um dos dois não há o que buscar.';
	@override String get summary => 'Resumo da conversa';
	@override String get summary_generating => 'Lendo a conversa…';
	@override String get summary_short => 'Conversa curta demais para resumir.';
	@override String get summary_failed => 'Não consegui resumir agora.';
	@override String get website => 'Site';
	@override String get check_kind_missing_attachment => 'O texto diz que vai anexo, e não há anexo';
	@override String get blocked_placeholder => 'Preencha o campo antes de enviar.';
	@override String get subject => 'Assunto';
	@override String get subject_hint => 'O que o destinatário vê antes de abrir';
	@override String get use_template => 'Usar um modelo';
	@override String get reply_rate_short => 'de resposta';
	@override String bulk_selected({required Object count}) => '${count} selecionadas';
	@override String get bulk_send => 'Enviar selecionadas';
	@override String bulk_confirm_title({required Object count}) => 'Enviar ${count} abordagens?';
	@override String bulk_review_title({required Object count}) => 'Revisar ${count} abordagens';
	@override String get bulk_review_hint => 'Desmarque o que não deve ir. Cada texto sai exatamente como está aqui.';
	@override String bulk_review_send({required Object count}) => 'Enviar ${count}';
	@override String get bulk_review_none => 'Nada marcado';
	@override String bulk_sent({required Object count}) => '${count} abordagens enviadas';
	@override String bulk_partial({required Object count, required Object failed}) => '${count} enviadas. Falharam: ${failed}';
	@override String profile_facts_count({required Object count}) => '${count} fato(s) sobre a empresa';
	@override String get profile_nothing_yet => 'Nada pesquisado ainda';
	@override String timeline_events({required Object count}) => '${count} evento(s)';
	@override String get section_expand => 'Abrir';
	@override String get section_collapse => 'Fechar';
	@override String get queue_filter_all => 'Todos';
	@override String get queue_no_funnel => 'Sem funil';
	@override String get queue_sort_oldest => 'Mais antigos primeiro';
	@override String get queue_sort_newest => 'Mais recentes primeiro';
	@override String get queue_flagged_only => 'Só com alerta';
	@override String queue_partial({required Object shown, required Object total}) => 'Mostrando ${shown} de ${total}. Escolha um funil para trabalhar o resto.';
	@override String get queue_filtered_empty => 'Nada nesta seleção.';
	@override String get queue_filtered_empty_hint => 'Os outros rascunhos continuam na fila — tire o filtro para vê-los.';
	@override String queue_waiting_minutes({required Object count}) => 'na fila há ${count} min';
	@override String queue_waiting_hours({required Object count}) => 'na fila há ${count}h';
	@override String queue_waiting_days({required Object count}) => 'na fila há ${count}d';
	@override String get queue_empty_cta => 'Buscar empresas';
	@override String email_warmup_banner_named({required Object mailbox, required Object day, required Object total, required Object remaining}) => 'Caixa ${mailbox} em aquecimento (dia ${day} de ${total}): restam ${remaining} envios frios hoje. O limite cresce a cada dia para proteger seu domínio.';
	@override String email_warmup_exhausted({required Object mailbox, required Object day, required Object total}) => 'A caixa ${mailbox} esgotou os envios frios de hoje (dia ${day} de ${total} do aquecimento). Novas aprovações só saem amanhã.';
	@override String get field_company => 'Empresa';
	@override String get field_website => 'Site';
	@override String get field_decision_maker => 'Decisor';
	@override String get field_role => 'Cargo';
	@override String get field_address => 'Endereço';
	@override String get field_activity => 'Atividade';
	@override String get bulk_subject_hint => 'Assunto dos e-mails deste lote';
	@override String get bulk_subject_missing => 'Adicione um assunto para enviar';
	@override String get error_action_configure => 'Configurar';
	@override String get approve_needs_queue_status => 'Este lead não está aguardando aprovação — gere um rascunho para colocá-lo de volta na fila.';
	@override String get approach_locked_after_send => 'Esta abordagem já foi enviada — o texto fica travado para o histórico continuar verdadeiro.';
	@override String get research_still_running => 'A pesquisa está demorando — ela continua em segundo plano; volte mais tarde.';
	@override String timeline_created({required Object date}) => 'Criado em ${date}';
	@override String timeline_qualified({required Object date}) => 'Qualificado em ${date}';
	@override String timeline_buy_intent({required Object date}) => 'Sinalizou intenção de compra em ${date}';
	@override String timeline_researched({required Object date}) => 'Pesquisado em ${date}';
	@override String timeline_updated({required Object date}) => 'Última atualização em ${date}';
	@override String get queue_group_carteira => 'Da sua carteira';
	@override String get queue_group_sinal => 'De sinal do radar';
	@override String get queue_group_frio => 'Frio, do território';
	@override String queue_frio_discard_all({required Object count}) => 'Descartar os ${count}';
	@override String get queue_frio_redraft_all => 'Reescrever com o catálogo';
	@override String queue_frio_show_all({required Object count}) => '+ ${count} iguais a estes · mostrar todos';
	@override String get queue_batch_discard_title => 'Descartar rascunhos frios';
	@override String queue_batch_discard_message({required Object count}) => 'Descartar ${count} rascunhos da fila? Os leads vão para perdidos e nada é enviado.';
	@override String get queue_batch_redraft_title => 'Reescrever com o catálogo';
	@override String queue_batch_redraft_message({required Object count}) => 'Reescrever ${count} rascunhos usando seu catálogo de produtos? Cada reescrita é uma chamada de IA cobrada na sua chave.';
	@override String queue_batch_discarded({required Object count}) => '${count} rascunhos descartados.';
	@override String queue_batch_redrafted({required Object count}) => '${count} rascunhos reescritos.';
	@override String queue_outcome_line({required Object sent, required Object replies, required Object proposals}) => 'Esta semana: ${sent} enviadas · ${replies} respostas · ${proposals} propostas';
	@override String approve_scheduled({required Object when}) => 'Limite de hoje esgotado — envio agendado para ${when}.';
	@override String queue_scheduled_badge({required Object when}) => 'Agendada · sai ${when}';
	@override String route_strip_title({required Object n, required Object cap}) => 'Envio ${n} de ${cap} de hoje';
	@override String get route_strip_exhausted => 'Limite de hoje esgotado — aprovações agendam para amanhã de manhã';
	@override String route_strip_sub({required Object mailbox, required Object day, required Object total}) => '${mailbox} em aquecimento, dia ${day} de ${total} — aprovar além do limite agenda sozinho.';
	@override String get queue_blocked_title => 'Nada pode sair ainda';
	@override String get queue_blocked_body => 'As ofertas estão prontas, mas esta conta não tem canal conectado.';
	@override String get queue_blocked_cta => 'Conectar canal';
	@override String queue_tender_deadline_days({required Object count}) => 'edital fecha em ${count}d';
	@override String get queue_tender_deadline_today => 'edital fecha hoje';
	@override String get queue_tender_deadline_past => 'edital encerrado';
	@override String queue_tender_value({required Object value}) => 'estimado • ${value}';
	@override String get queue_no_address_title => 'Sem endereço para enviar';
	@override String get queue_no_address_body => 'Nenhum canal seu alcança este contato. Ache o e-mail de compras e preencha o cliente — a oferta já está pronta.';
	@override String get queue_no_address_cta => 'Abrir cliente';
	@override String get queue_wa_send => 'Mandar pelo meu WhatsApp';
	@override String get queue_wa_hint => 'abre no seu número, com o texto pronto';
	@override String get queue_wa_confirm_title => 'Você chegou a mandar?';
	@override String get queue_wa_confirm_body => 'Nada saiu por aqui — quem mandou foi você. Se mandou, tiro esta oferta da fila e marco o cliente como contactado.';
	@override String get queue_wa_confirm_yes => 'Mandei';
	@override String get queue_wa_confirm_no => 'Ainda não';
}

// Path: account_menu
class _Translations$account_menu$pt extends Translations$account_menu$en {
	_Translations$account_menu$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get profile => 'Perfil';
	@override String get team => 'Equipe';
	@override String get channels => 'Canais';
	@override String get settings => 'Configurações';
	@override String get logout => 'Sair';
	@override String get title => 'Conta';
}

// Path: copilot
class _Translations$copilot$pt extends Translations$copilot$en {
	_Translations$copilot$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get empty_title => 'O que você quer fazer?';
	@override String get empty_body => 'Peça em português. Eu executo e devolvo o link pra você conferir. Não envio nada sem sua aprovação.';
	@override String get hint => 'peça algo…';
	@override String get thinking => 'trabalhando…';
	@override String get clear => 'Limpar conversa';
	@override String get sample_prospect => 'Ache 20 empresas de [segmento] em [cidade]';
	@override String get sample_drafts => 'Gere abordagens para os leads novos do funil';
	@override String get sample_report => 'Como foi minha prospecção nos últimos 30 dias?';
	@override String get failed => 'Não consegui responder. Tente de novo.';
	@override String get tool_list_funnels => 'consultou seus funis';
	@override String get tool_pipeline_overview => 'leu os números do funil';
	@override String get tool_list_approval_queue => 'leu a fila de aprovação';
	@override String get tool_draft_approaches => 'gerou rascunhos de abordagem';
	@override String get tool_case_report => 'leu seu relatório do período';
	@override String get thinking_long => 'Ainda trabalhando. Buscas e rascunhos podem levar até 2 minutos.';
	@override String sample_prospect_audience({required Object keyword, required Object location}) => 'Ache 20 ${keyword} em ${location}';
}

// Path: attachment
class _Translations$attachment$pt extends Translations$attachment$en {
	_Translations$attachment$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String count({required Object count}) => '${count} anexo(s)';
	@override String get open => 'Abrir';
	@override String get download_failed => 'Não consegui abrir o arquivo agora.';
	@override String get status_clean => 'Verificado pelo antivírus';
	@override String get status_pending => 'Ainda não verificado';
	@override String get status_skipped => 'Não verificado';
	@override String get status_failed => 'A verificação não completou';
	@override String get status_infected => 'Ameaça encontrada';
	@override String get blocked_title => 'Este arquivo não foi liberado';
	@override String get blocked_pending => 'O antivírus ainda não terminou de verificar. Tente de novo em alguns minutos.';
	@override String get blocked_skipped => 'Não há antivírus configurado nesta instalação, então ninguém verificou este arquivo. Ele veio de fora — abra por sua conta e risco.';
	@override String get blocked_failed => 'A verificação falhou. O sistema tenta de novo automaticamente; se continuar, chame o suporte.';
	@override String blocked_infected({required Object signature}) => 'O antivírus identificou uma ameaça (${signature}). O arquivo não pode ser aberto. Se você acha que é engano, peça o arquivo por outro caminho.';
	@override String get download_anyway => 'Baixar mesmo assim';
	@override String get download_anyway_confirm => 'Ninguém verificou este arquivo. Ele foi enviado por alguém de fora da sua empresa. Abrir?';
	@override String get why_blocked => 'Por quê?';
	@override String get view_original => 'Ver original';
	@override String get view_original_images => 'Ver original com imagens';
	@override String get view_original_images_warning => 'Carregar as imagens avisa quem enviou que você abriu a mensagem. Continuar?';
}

// Path: sender_auth
class _Translations$sender_auth$pt extends Translations$sender_auth$en {
	_Translations$sender_auth$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get badge_unknown => 'Remetente não verificado';
	@override String get badge_failed => 'Remetente suspeito';
	@override String get title_unknown => 'Ninguém verificou este remetente';
	@override String get title_failed => 'Este remetente não passou na verificação';
	@override String get body_unknown => 'O servidor que recebeu esta mensagem não informou se o domínio autorizou o envio. Isso é comum com provedores antigos e não quer dizer que seja falso — só quer dizer que não dá para afirmar que é verdadeiro.';
	@override String get body_failed => 'O domínio que aparece no remetente NÃO autorizou este envio. Alguém pode estar se passando por essa empresa. Antes de responder dados, pagar boleto ou trocar conta bancária, confirme por telefone usando um número que você já tinha.';
	@override String checks({required Object checks}) => 'Verificações: ${checks}';
}

// Path: email_inbox
class _Translations$email_inbox$pt extends Translations$email_inbox$en {
	_Translations$email_inbox$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get no_subject => '(sem assunto)';
	@override String get empty => 'Nenhum e-mail nesta thread ainda.';
	@override String get you => 'Você';
	@override String get back_to_list => 'Voltar aos e-mails';
	@override String get loading_email => 'Carregando e-mail…';
	@override String get view_text => 'Texto';
	@override String get view_original => 'Original';
}

// Path: email_risk
class _Translations$email_risk$pt extends Translations$email_risk$en {
	_Translations$email_risk$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get payment_title => 'Atenção: dados de pagamento mudaram';
	@override String get payment_action => 'Confirme por telefone antes de pagar, usando um número que você já tinha — não o que está neste e-mail.';
	@override String get quarantine_title => 'Mensagem retida para conferência';
	@override String get attention_title => 'Vale conferir esta mensagem';
	@override String get legit => 'É legítimo';
	@override String get legit_confirm => 'Você confirma que este remetente é confiável? Paramos de avisar sobre ele nesta conta.';
	@override String get legit_done => 'Certo. Não vamos mais avisar sobre este remetente.';
	@override String get legit_failed => 'Não consegui registrar agora.';
	@override String get scam => 'É golpe';
	@override String get scam_title => 'Reportar como golpe';
	@override String get scam_why => 'Por que você acha que é golpe? (opcional)';
	@override String get scam_why_hint => 'Ex.: esse fornecedor nunca manda boleto por e-mail';
	@override String get scam_explain => 'Seu relato protege as outras empresas da rede. Quando duas empresas reportam o mesmo remetente, ele passa a ser bloqueado para todo mundo.';
	@override String get scam_done => 'Reportado. Obrigado — isso protege as outras empresas da rede.';
}

// Path: brand
class _Translations$brand$pt extends Translations$brand$en {
	_Translations$brand$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Identidade da empresa';
	@override String get subtitle => 'O que a IA sabe sobre vocês e o que ela nunca pode dizer. Vale para toda abordagem, de todo vendedor.';
	@override String get identity_section => 'Identidade da empresa';
	@override String get company_name => 'Nome da empresa';
	@override String get about => 'O que vocês fazem';
	@override String get about_hint => 'Ex.: distribuímos monitores multiparâmetro e bombas de infusão para clínicas do interior de SP';
	@override String get tone => 'Como vocês falam';
	@override String get tone_hint => 'Ex.: direto, sem jargão, trata por você';
	@override String get signature => 'Assinatura padrão';
	@override String get allowed => 'O que podemos afirmar';
	@override String get allowed_hint => 'Coisas verdadeiras que ajudam a vender. Ex.: assistência técnica própria em Sorocaba';
	@override String get forbidden => 'Nunca dizer';
	@override String get forbidden_hint => 'A IA é impedida de escrever isso, e o texto é conferido antes de chegar na fila. Ex.: cura, aprovado pela ANVISA';
	@override String get add => 'Adicionar';
	@override String get remove => 'Remover';
	@override String get saved => 'Identidade salva.';
	@override String get personal => 'Seu jeito';
	@override String get personal_subtitle => 'Some ao da empresa, não substitui. Você não consegue liberar o que a empresa proibiu.';
	@override String get tone_note => 'Seu tom pessoal';
	@override String get tone_note_hint => 'Ex.: falo mais devagar, sempre pergunto do time antes do preço';
	@override String get personal_signature => 'Sua assinatura';
	@override String get personal_signature_hint => 'Ex.: João Silva · (15) 99999-0000';
	@override String get personal_saved => 'Seu jeito foi salvo.';
	@override String get differentiators => 'Seus diferenciais';
	@override String get differentiators_hint => 'Por que o seu equipamento, e não o que o hospital já tem. Escrito por você — a IA usa o que couber e não inventa nenhum. Vazio, a proposta argumenta só pelo produto.';
}

// Path: suppressions
class _Translations$suppressions$pt extends Translations$suppressions$en {
	_Translations$suppressions$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Endereços bloqueados';
	@override String get subtitle => 'Endereços que não recebem mais e-mails seus: houve rejeição permanente ou marcação de spam. Insistir neles é o que queima a reputação do seu domínio.';
	@override String get empty => 'Nenhum endereço bloqueado';
	@override String get empty_hint => 'Tudo que for rejeitado em definitivo aparece aqui, com o motivo que o servidor de destino deu.';
	@override String get release => 'Liberar';
	@override String get release_title => 'Liberar este endereço?';
	@override String get release_warning => 'Um servidor de e-mail recusou este endereço, ou o destinatário marcou como spam. Só libere se souber que o problema foi resolvido.';
	@override String get released => 'Endereço liberado';
	@override String get reason_hard_bounce => 'Rejeição permanente — o endereço não existe';
	@override String get reason_complaint => 'Marcado como spam pelo destinatário';
}

// Path: signals
class _Translations$signals$pt extends Translations$signals$en {
	_Translations$signals$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Sinais de compra';
	@override String get subtitle => 'Licitações públicas que combinam com o que você vende. Alguém escreveu o que pretende comprar e até quando — prazo mais próximo primeiro.';
	@override String get empty_no_watch => 'Nenhum monitoramento configurado';
	@override String get empty_no_watch_hint => 'Diga quais palavras acompanhar e os editais abertos aparecem aqui sozinhos.';
	@override String get empty_quiet => 'Nada aberto no momento';
	@override String get empty_quiet_hint => 'Consultamos as licitações públicas várias vezes por dia. Os novos aparecem aqui assim que são publicados.';
	@override String days_left({required Object days}) => 'faltam ${days} dias';
	@override String get closes_today => 'encerra hoje';
	@override String get value_undisclosed => 'valor não divulgado';
	@override String get open_notice => 'Ler o edital';
	@override String get dismiss => 'Não é para nós';
	@override String get tab => 'Sinais';
	@override String get watch_cta => 'Escolher palavras';
	@override String get watch_dialog_title => 'O que você quer acompanhar';
	@override String get watch_dialog_body => 'Palavras que aparecem no que você vende. Separe por vírgula — buscamos cada uma nos editais publicados.';
	@override String get watch_hint => 'ex: material hospitalar, seringa, luva de procedimento';
	@override String get watch_states_label => 'Onde você entrega';
	@override String get watch_states_hint => 'ex: MT, MS';
	@override String get watch_states_error => 'Informe pelo menos um estado (UF).';
	@override String get watch_states_body => 'Só buscamos editais dos estados que você atende — sem isso a busca varre o país inteiro e traz o que você não pode entregar.';
	@override String get watch_saved => 'Pronto. Os editais que combinarem aparecem aqui sozinhos.';
	@override String get watch_empty_error => 'Escreva pelo menos uma palavra.';
	@override String watch_current({required Object words}) => 'Acompanhando: ${words}';
	@override String get no_catalog_banner => 'Nenhum produto cadastrado — estes resultados vêm de palavras digitadas à mão, não do que você vende. Importe seu catálogo e o monitoramento se monta sozinho a partir dele.';
	@override String get no_catalog_cta => 'Cadastrar produtos';
	@override String get watch_origin_catalog => 'do seu catálogo';
	@override String get watch_origin_manual => 'definido manualmente';
}

// Path: sales_record
class _Translations$sales_record$pt extends Translations$sales_record$en {
	_Translations$sales_record$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Registrar venda';
	@override String get gross => 'Valor total';
	@override String get gross_hint => 'o que o cliente paga';
	@override String get cost => 'Custo da mercadoria';
	@override String get cost_hint => 'o que te custou — a comissão sai da margem, não da nota';
	@override String get tax => 'Imposto';
	@override String get tax_hint => 'quanto de imposto sai desta venda, em reais';
	@override String get cycle => 'Ciclo de recompra (dias)';
	@override String get cycle_hint => 'de quanto em quanto tempo esse cliente recompra — deixe vazio se não souber';
	@override String get notes => 'Observações';
	@override String get net => 'Você fica com';
	@override String get underwater_warning => 'Esta venda custa mais do que traz. Pode registrar assim mesmo — só confira os números.';
	@override String get submit => 'Registrar venda';
	@override String get saved => 'Venda registrada';
	@override String get your_commission => 'Sua comissão nesta venda';
}

// Path: portfolio
class _Translations$portfolio$pt extends Translations$portfolio$en {
	_Translations$portfolio$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Carteira';
	@override String get subtitle => 'Seus clientes — importados ou conquistados. Ordenado por quem precisa ouvir você primeiro, não por ordem alfabética.';
	@override String get empty => 'Nenhum cliente ainda';
	@override String get empty_hint => 'Importe sua planilha, ou registre uma venda num lead, e eles aparecem aqui — com a previsão de quando vão precisar de novo.';
	@override String purchases({required Object count}) => '${count} compra(s)';
	@override String ran_out({required Object days}) => 'acabou há ${days} dias';
	@override String get runs_out_today => 'acaba hoje';
	@override String runs_out_in({required Object days}) => 'acaba em ${days} dias';
	@override String silent_for({required Object days}) => 'sem contato há ${days} dias';
	@override String get history => 'Histórico de compras';
	@override String get sale_without_items => 'Venda sem itens detalhados';
}

// Path: cross_sell
class _Translations$cross_sell$pt extends Translations$cross_sell$en {
	_Translations$cross_sell$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'O que mais oferecer';
	@override String because({required Object item}) => 'porque compra ${item}';
	@override String evidence({required Object count, required Object item}) => '${count} clientes que compram ${item} também levam';
	@override String get empty => 'Ainda não há padrão de co-compra suficiente nas vendas desta conta.';
	@override String get learned_here => 'Aprendido só com as vendas desta conta.';
}

// Path: relationship
class _Translations$relationship$pt extends Translations$relationship$en {
	_Translations$relationship$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Datas de relacionamento';
	@override String get subtitle => 'Aniversários, aberturas de empresa, datas do setor e manutenções. No dia, o rascunho vai para a fila de aprovação.';
	@override String get empty => 'Nenhuma data cadastrada';
	@override String get empty_hint => 'Cadastre um aniversário ou uma data do setor e o sistema lembra por você.';
	@override String get add => 'Nova data';
	@override String get kind_birthday => 'Aniversário';
	@override String get kind_founded => 'Aniversário da empresa';
	@override String get kind_sector => 'Data do setor';
	@override String get kind_maintenance => 'Manutenção preventiva';
	@override String get kind_custom => 'Outra';
	@override String get sector_hint => 'Escolha uma data profissional';
	@override String get label => 'Descrição';
	@override String get day => 'Dia';
	@override String get month => 'Mês';
	@override String get account_wide => 'Vale para a conta inteira — aparece aqui, mas não vira mensagem sozinha.';
	@override String get delete_confirm => 'Esquecer esta data?';
	@override String get saved => 'Data salva';
	@override String get greeting_only => 'Só saudações podem sair sem aprovação, e apenas com o autopilot ligado.';
}

// Path: command_center
class _Translations$command_center$pt extends Translations$command_center$en {
	_Translations$command_center$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get tile_meetings => 'Reuniões';
	@override String get tile_won => 'Ganhos';
	@override String get funnel_title => 'Funil';
	@override String more_actions({required Object count}) => '+${count}';
	@override String get partial_load => 'Parte da lista não carregou — pode haver mais coisas esperando.';
	@override String get snooze => 'Depois';
	@override String snoozed({required Object days}) => 'Adiado. Volta em ${days} dias.';
	@override String get kind_approve_draft => 'Abordagem esperando aprovação';
	@override String get kind_unowned_lead => 'Lead sem responsável';
	@override String get kind_signature_viewed => 'Abriu o contrato e não assinou';
	@override String get kind_contract_alert => 'Prazo de contrato';
	@override String get kind_confirm_sale => 'Venda esperando confirmação';
	@override String get kind_reorder_due => 'Já deveria ter recomprado';
	@override String get kind_waiting_reply => 'Cliente esperando resposta';
	@override String get kind_unknown => 'Precisa de atenção';
	@override String overdue_days({required Object count}) => 'venceu há ${count} dia(s)';
	@override String get due_today => 'vence hoje';
	@override String due_in_days({required Object count}) => 'vence em ${count} dia(s)';
	@override String waiting_days({required Object count}) => 'esperando há ${count} dia(s)';
	@override String get waiting_today => 'chegou hoje';
	@override String get activation_title => 'Sua primeira venda começa com um lead';
	@override String get activation_body => 'Ainda não há nada no funil. Traga empresas para prospectar ou acompanhe os sinais públicos de quem já disse que quer comprar.';
	@override String get activation_signals => 'Ver sinais públicos';
	@override List<String> get months => [
		'janeiro',
		'fevereiro',
		'março',
		'abril',
		'maio',
		'junho',
		'julho',
		'agosto',
		'setembro',
		'outubro',
		'novembro',
		'dezembro',
	];
	@override String month_of_year({required Object month, required Object year}) => '${month} de ${year}';
	@override String get tile_untouched => 'Nunca abordados';
	@override String get tile_untouched_footnote => 'no seu território';
	@override String untouched_target_row({required Object count, required Object unit}) => '${count} ${unit} do que você vende, e nunca foi abordado';
	@override String get kind_untouched_target => 'Nunca abordado';
	@override String get action_see_target => 'Ver no radar';
}

// Path: ai_debounce
class _Translations$ai_debounce$pt extends Translations$ai_debounce$en {
	_Translations$ai_debounce$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String waiting({required Object seconds}) => 'A IA responde em ~${seconds}s, aguardando o cliente terminar';
	@override String get answering => 'Respondendo…';
	@override String collapsed({required Object count}) => '${count} mensagens serão respondidas juntas';
	@override String get answer_now => 'Responder agora';
}

// Path: overnight
class _Translations$overnight$pt extends Translations$overnight$en {
	_Translations$overnight$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Seu SDR trabalhou de madrugada';
	@override String drafted({required Object count}) => '${count} abordagens prontas para revisar';
	@override String budget_hit({required Object count}) => '${count} não couberam na franquia do mês';
	@override String get review => 'Revisar';
	@override String get dismiss => 'Ok';
	@override String get blocked_title => 'A caça de hoje não rodou';
	@override String get blocked_body => 'Nenhuma oferta foi escrita esta noite. O motivo:';
	@override String get blocked_cta => 'Resolver';
}

// Path: export
class _Translations$export$pt extends Translations$export$en {
	_Translations$export$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get tooltip => 'Exportar';
	@override String get xlsx => 'Exportar para Excel (.xlsx)';
	@override String get csv => 'Exportar para CSV';
	@override String get audited => 'Toda exportação fica registrada';
	@override String done({required Object rows}) => '${rows} linhas exportadas';
	@override String truncated({required Object rows}) => '${rows} linhas — o arquivo bateu no teto e não é a lista inteira';
}

// Path: my_customers
class _Translations$my_customers$pt extends Translations$my_customers$en {
	_Translations$my_customers$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Seus clientes de sempre';
	@override String get subtitle => 'Lemos só quem você escreveu e quando — nunca o conteúdo das mensagens.';
	@override String get scan_action => 'Buscar na minha caixa';
	@override String get scanning => 'Lendo sua pasta de enviados…';
	@override String scan_result({required Object scanned, required Object mailbox, required Object found}) => 'Li ${scanned} mensagens em ${mailbox} e encontrei ${found} pessoas.';
	@override String scan_skipped({required Object count}) => '${count} endereços de robô e o seu próprio ficaram de fora.';
	@override String get scan_partial => 'Paramos antes do fim da pasta, então essa não é a lista inteira. Busque de novo para continuar.';
	@override String scan_failed({required Object error}) => 'Não conseguimos ler sua pasta de enviados: ${error}';
	@override String get empty_title => 'Nada para revisar ainda.';
	@override String get empty_hint => 'Busque na sua caixa para ver quem você já atende.';
	@override String sent_count({required Object count}) => '${count} e-mails enviados';
	@override String since({required Object date}) => 'desde ${date}';
	@override String last({required Object date}) => 'último em ${date}';
	@override String get funnel_label => 'Entram no funil';
	@override String get funnel_required => 'Escolha o funil antes de importar.';
	@override String get no_funnels => 'Crie um funil primeiro — é onde os clientes vão entrar.';
	@override String selected({required Object count}) => '${count} selecionados';
	@override String get import_action => 'Importar selecionados';
	@override String get dismiss_action => 'Não são clientes';
	@override String imported({required Object count}) => '${count} clientes importados.';
	@override String imported_partial({required Object count, required Object skipped}) => '${count} importados. ${skipped} já estavam no sistema.';
	@override String dismissed({required Object count}) => '${count} fora da lista.';
	@override String get select_all => 'Selecionar todos';
	@override String get clear_selection => 'Limpar';
	@override String showing_partial({required Object shown, required Object total}) => 'Mostrando ${shown} de ${total}.';
	@override String get view_label => 'Na minha caixa';
	@override String get open_inbox_customers => 'Ver clientes da minha caixa';
}

// Path: proposal
class _Translations$proposal$pt extends Translations$proposal$en {
	_Translations$proposal$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Nova proposta';
	@override String get pick_products => 'Escolha o que você está oferecendo e quantos.';
	@override String get total => 'Total';
	@override String get mixed_currency => 'Estes produtos estão em moedas diferentes. Uma proposta carrega uma moeda — senão o total seria um número que ninguém consegue defender.';
	@override String get body_label => 'Sua mensagem';
	@override String get body_hint => 'Opcional. As linhas com preço entram abaixo do que você escrever.';
	@override String get send_for_approval => 'Enviar para aprovação';
	@override String get approval_note => 'Vai para a fila de aprovação. Nada chega ao cliente até você aprovar lá.';
	@override String get queued_title => 'Na fila de aprovação. É isto que quem revisar vai ler:';
	@override String get history => 'JÁ OFERECIDO';
	@override String get status_draft => 'Rascunho';
	@override String status_sent({required Object date}) => 'Enviada em ${date}';
	@override String get no_products => 'Nenhum produto ainda';
	@override String get no_products_hint => 'Uma proposta é feita do que você vende. Cadastre um produto e volte.';
	@override String get go_to_products => 'Cadastrar produto';
	@override String get open => 'Fazer proposta';
	@override String get download_pdf => 'Baixar PDF';
	@override String get download_failed => 'Não consegui salvar o arquivo.';
	@override String get download_hint => 'Um arquivo para mandar agora, por WhatsApp ou e-mail. Não envia nada sozinho.';
	@override String get suggested_hint => 'Já preenchi o que combina com este cliente. Mude a quantidade, tire o que não serve, acrescente o que faltar.';
}

// Path: product_import
class _Translations$product_import$pt extends Translations$product_import$en {
	_Translations$product_import$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Importar lista de preços';
	@override String get what_we_read => 'CSV com linha de títulos (nome/produto, preço/valor, descrição, moeda) ou PDF de catálogo do fabricante — a IA lê o PDF e você confere linha por linha antes de criar nada. Pode escolher vários arquivos de uma vez.';
	@override String reading_progress({required Object done, required Object total}) => 'Lendo ${done} de ${total}…';
	@override String files_failed({required Object count, required Object reason}) => '${count} arquivo(s) não puderam ser lidos: ${reason}';
	@override String files_read({required Object count}) => '${count} arquivos lidos';
	@override String row_from({required Object file}) => 'de ${file}';
	@override String get pick_file => 'Escolher arquivo';
	@override String get pick_another => 'Escolher outro arquivo';
	@override String found({required Object total, required Object importable}) => '${total} linhas lidas, ${importable} prontas para importar';
	@override String with_problems({required Object count}) => '${count} precisam de um olhar';
	@override String already_here({required Object count}) => '${count} já estão no seu catálogo';
	@override String confirm({required Object count}) => 'Importar ${count} produto(s)';
	@override String get nothing_importable => 'Nada a importar deste arquivo ainda — corrija as linhas acima e escolha o arquivo de novo.';
	@override String line_problem({required Object line, required Object problem}) => 'linha ${line} — ${problem}';
	@override String line_price({required Object line, required Object price}) => 'linha ${line} — ${price}';
	@override String get blank_name => '(sem nome)';
	@override String imported({required Object count}) => '${count} produtos importados.';
	@override String imported_partial({required Object count, required Object skipped}) => '${count} importados, ${skipped} ignorados.';
	@override String get open => 'Importar planilha';
	@override String get what_we_read_pdf => 'Ou envie o folheto do fabricante em PDF — a IA lê e você revisa cada linha antes de qualquer coisa ser criada.';
	@override String get pick_pdf => 'Importar folheto (PDF)';
	@override String get pdf_reading => 'Lendo o folheto com IA — pode levar até um minuto…';
	@override String page_problem({required Object page, required Object problem}) => 'pág. ${page} — ${problem}';
	@override String page_price({required Object page, required Object price}) => 'pág. ${page} — ${price}';
	@override String get price_missing => 'preço não lido — preencha depois';
}

// Path: customer_import
class _Translations$customer_import$pt extends Translations$customer_import$en {
	_Translations$customer_import$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Importar lista de clientes';
	@override String get what_we_read => 'Um CSV com linha de títulos. Ele lê as colunas empresa/cliente, contato/nome, telefone/celular e e-mail — separado por vírgula ou ponto e vírgula, como sua planilha salvou. Cada cliente precisa de telefone ou e-mail.';
	@override String get pick_file => 'Escolher arquivo';
	@override String get pick_another => 'Escolher outro arquivo';
	@override String found({required Object total, required Object importable}) => '${total} linhas lidas, ${importable} prontas para importar';
	@override String with_problems({required Object count}) => '${count} precisam de um olhar';
	@override String already_here({required Object count}) => '${count} já estão na sua base';
	@override String confirm({required Object count}) => 'Importar ${count} cliente(s)';
	@override String get nothing_importable => 'Nada a importar deste arquivo ainda — corrija as linhas acima e escolha o arquivo de novo.';
	@override String line_problem({required Object line, required Object problem}) => 'linha ${line} — ${problem}';
	@override String line_address({required Object line, required Object address}) => 'linha ${line} — ${address}';
	@override String get blank_name => '(sem nome)';
	@override String imported({required Object count}) => '${count} clientes importados.';
	@override String imported_partial({required Object count, required Object skipped}) => '${count} importados, ${skipped} ignorados.';
	@override String get open => 'Importar planilha';
}

// Path: sales_import
class _Translations$sales_import$pt extends Translations$sales_import$en {
	_Translations$sales_import$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Importar histórico de vendas';
	@override String get open => 'Importar vendas';
	@override String get settings_subtitle => 'Traga as vendas passadas para alimentar recompra e cross-sell';
	@override String get hint => 'Um CSV com linha de títulos. Ele lê as colunas cliente, produto, data, valor e quantidade — separado por vírgula ou ponto e vírgula, como sua planilha salvou. Os clientes precisam já estar na sua base: importe a lista de clientes primeiro.';
	@override String get pick_file => 'Escolher arquivo';
	@override String get pick_another => 'Escolher outro arquivo';
	@override String get preview_title => 'O que o arquivo diz';
	@override String found({required Object total, required Object importable}) => '${total} linhas lidas, ${importable} prontas para registrar';
	@override String problems_title({required Object count}) => '${count} precisam de um olhar';
	@override String already_recorded({required Object count}) => '${count} já estão registradas';
	@override String unknown_customers({required Object count}) => '${count} citam clientes fora da sua base — importe a lista de clientes primeiro';
	@override String confirm({required Object count}) => 'Registrar ${count} venda(s)';
	@override String get nothing_importable => 'Nada a registrar deste arquivo ainda — corrija as linhas acima e escolha o arquivo de novo.';
	@override String line_problem({required Object line, required Object problem}) => 'linha ${line} — ${problem}';
	@override String line_detail({required Object line, required Object detail}) => 'linha ${line} — ${detail}';
	@override String get blank_customer => '(sem cliente)';
	@override String result({required Object count}) => '${count} vendas registradas.';
	@override String skipped({required Object count, required Object skipped}) => '${count} vendas registradas, ${skipped} puladas.';
	@override String get view_portfolio => 'Ver carteira';
}

// Path: lead_timeline
class _Translations$lead_timeline$pt extends Translations$lead_timeline$en {
	_Translations$lead_timeline$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'CONVERSA';
	@override String get empty => 'Nada foi enviado para este cliente ainda.';
	@override String get truncated => 'Mostrando as mensagens mais recentes.';
	@override String get from_customer => 'Cliente';
	@override String get from_team => 'Nós';
	@override String get from_bot => 'Bot';
	@override String attachment({required Object kind}) => '[${kind}]';
	@override String get not_delivered => 'Não entregue';
	@override String get reply_hint => 'Escreva sua resposta…';
	@override String get reply_send => 'Responder';
	@override String get reply_sent => 'Resposta enviada.';
	@override String get no_conversation => 'Responder abre quando houver conversa — a primeira oferta sai pela fila.';
}

// Path: email_composer
class _Translations$email_composer$pt extends Translations$email_composer$en {
	_Translations$email_composer$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get subject_hint => 'Assunto';
	@override String get body_hint => 'Escreva seu e-mail…';
	@override String get send => 'Enviar';
	@override String get reply => 'Responder';
}

// Path: radar
class _Translations$radar$pt extends Translations$radar$en {
	_Translations$radar$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get tab => 'Radar';
	@override String get tab_hoje => 'Hoje';
	@override String warnings({required Object sources}) => 'Não consegui ler: ${sources}. O resto do feed está no ar.';
	@override String source_down({required Object source, required Object ago}) => 'Não conseguimos ler o ${source} há ${ago}. O que aparece abaixo é da última coleta — pode não ser o que está aberto agora.';
	@override String source_down_never({required Object source}) => 'Ainda não conseguimos ler o ${source}. Nada nesta lista veio dele.';
	@override String ago_minutes({required Object n}) => '${n} min';
	@override String ago_hours({required Object n}) => '${n} h';
	@override String ago_days({required Object n}) => '${n} dias';
	@override String get filter_all => 'Todas';
	@override String get type_licitacao => 'Licitações';
	@override String get type_parque => 'Parque instalado';
	@override String get type_recompra => 'Recompras';
	@override String get type_data => 'Datas';
	@override String get empty => 'Nada no radar agora';
	@override String get empty_hint => 'Licitações, parque instalado no CNES, recompras vencendo e datas de relacionamento aparecem aqui conforme forem detectadas.';
	@override String get offer_queued => 'Rascunho na fila — esperando sua aprovação em Hoje.';
	@override String get deadline_today => 'hoje';
	@override String deadline_days({required Object count}) => 'em ${count}d';
	@override String get open_draft => 'Abrir rascunho';
	@override String get prepare_offer => 'Preparar oferta';
	@override String get open_customer => 'Ver cliente';
	@override String get hunt_title => 'Abastecer o território';
	@override String get hunt_hint => 'Busca no CNES quem declara o equipamento que você atende — dado público, quantidade, nunca idade do aparelho. Telefone e endereço vêm do cadastro CNES.';
	@override String get hunt_placeholder => 'ex.: tomógrafo, ventilador, ultrassom';
	@override String get hunt_empty => 'Nada encontrado.';
	@override String hunt_units({required Object count}) => '${count} declarados no CNES';
	@override String get hunt_known => 'já está nos seus contatos';
	@override String get type_cross_sell => 'Venda casada';
	@override String get carteira_hint => 'Recompra, cross-sell e datas aparecem depois da primeira venda registrada.';
	@override String get empty_hint_prospecting => 'Licitações e parque instalado no CNES aparecem aqui conforme forem detectados.';
	@override String get footer_note => 'Tudo que virar oferta cai em Hoje para sua aprovação — nada é enviado daqui.';
	@override String watching_catalog({required Object count}) => 'catálogo (${count} produtos)';
	@override String get week_prefix => 'Esta semana:';
	@override String get week_sent => 'enviadas';
	@override String get week_replies => 'respostas';
	@override String get week_proposals => 'propostas';
	@override String data_month({required Object month}) => 'dados CNES de ${month}';
	@override String hunt_scope_cities({required Object cities}) => 'no seu território (${cities})';
	@override String hunt_scope_uf({required Object uf}) => 'em ${uf}';
	@override String get auto_hunt_cta => 'Caçar com meu catálogo';
	@override String get auto_hunt_hint => 'Ele lê seus produtos e traz os maiores hospitais do território que você nunca abordou. Nada é enviado.';
	@override String auto_hunt_result({required Object created, required Object found}) => '${created} novos leads no funil, de ${found} encontrados';
	@override String get auto_hunt_none => 'Nenhum estabelecimento novo no seu território agora';
	@override String park_universe({required Object total, required Object scope, required Object shown}) => '${total} hospitais ${scope} declaram equipamento que seu catálogo atende. Mostrando os ${shown} maiores.';
	@override String offer_more({required Object count}) => '+${count} outros';
	@override String get type_contrato => 'Contrato vencendo';
	@override String get type_ata => 'Ata de registro';
	@override String get edge_before_tender => 'antes do edital';
	@override String get edge_no_tender => 'sem licitação';
	@override String get read_and_send => 'Ler e enviar';
	@override String get back_to_panel => 'Voltar ao painel';
	@override String get panel_object => 'O que estão comprando';
	@override String get panel_buyer => 'Quem está comprando';
	@override String get panel_reference => 'Número no registro público';
	@override String get panel_published => 'Publicado em';
	@override String get panel_deadline => 'Prazo';
	@override String get panel_value => 'Valor';
	@override String get panel_no_link => 'Este aviso não publicou link nem arquivo. Copie o número acima e busque no PNCP — é a única referência que existe.';
	@override String get panel_park_items => 'O que este hospital declara';
	@override String get panel_offer_title => 'O que você pode oferecer';
	@override String get panel_close => 'Fechar';
	@override String get panel_copy_reference => 'Copiar número';
	@override String get panel_copied => 'Número copiado';
	@override String get panel_empty => 'Toque numa oportunidade para ver tudo o que sabemos sobre ela.';
	@override String get panel_portal => 'PNCP — Portal Nacional de Contratações Públicas';
	@override String get panel_open_portal => 'Abrir o PNCP';
	@override String get panel_buyer_known => 'Você já vendeu para este comprador';
	@override String get panel_buyer_cold => 'Este comprador nunca ouviu falar de você';
	@override String get panel_history => 'O que este órgão já contratou';
	@override String panel_history_count({required Object n}) => 'Este comprador tem ${n} contrato(s) do seu ramo no registro público.';
	@override String get panel_history_none => 'Nenhum contrato do seu ramo no registro público deste comprador.';
	@override String get panel_buyer_contacted => 'Você já enviou uma oferta para este comprador';
	@override String get panel_buyer_queued => 'Oferta escrita, esperando sua aprovação — nada enviado';
	@override String get panel_buyer_listed => 'Já está no seu quadro — nada escrito ainda';
	@override String get panel_reach => 'Como falar com eles';
	@override String panel_stage({required Object stage}) => 'No quadro, em: ${stage}';
	@override String get panel_open_board => 'Abrir no quadro';
	@override String get panel_purchaser => 'Quem compra por eles';
	@override String panel_history_via({required Object org, required Object n}) => 'A ${org}, que compra por eles, tem ${n} contrato(s) do seu ramo no registro público.';
	@override String get panel_beds => 'Leitos que opera';
	@override String get panel_habilitations => 'Habilitado pelo Ministério';
	@override String panel_habilitation_beds({required Object name, required Object n}) => '${name} — ${n} leito(s) habilitado(s)';
	@override String get panel_services => 'Serviços que realiza';
}

// Path: start
class _Translations$start$pt extends Translations$start$en {
	_Translations$start$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String greeting_morning({required Object name}) => 'Bom dia, ${name}';
	@override String greeting_afternoon({required Object name}) => 'Boa tarde, ${name}';
	@override String greeting_evening({required Object name}) => 'Boa noite, ${name}';
	@override String date_line({required Object weekday, required Object day, required Object month}) => '${weekday}, ${day} de ${month}';
	@override List<String> get weekdays => [
		'segunda',
		'terça',
		'quarta',
		'quinta',
		'sexta',
		'sábado',
		'domingo',
	];
	@override String summary_ready({required Object count}) => '${count} oferta(s) pronta(s)';
	@override String summary_waiting({required Object count}) => '${count} cliente(s) esperando';
	@override String summary_due_today({required Object count}) => '${count} vencendo hoje';
	@override String get summary_due_none => 'nada vencendo hoje';
	@override String review_n({required Object count}) => 'Revisar as ${count}';
	@override String get see_radar => 'Ver Radar';
	@override String get action_reply => 'Responder';
	@override String get action_assume => 'Assumir';
	@override String get distribute_team => '✦ Distribuir entre a equipe';
	@override String get sorted_by_urgency => 'ordenado por urgência';
	@override String filter_all({required Object count}) => 'Todos ${count}';
	@override String get rail_radar_title => 'Radar hoje';
	@override String get rail_radar_open => 'abrir →';
	@override String get rail_week_title => 'Sua semana';
	@override String get rail_engine_title => 'Ligue o motor';
	@override String of_total({required Object current, required Object total}) => '${current} de ${total}';
	@override String get register_sale_link => 'registrar uma venda';
	@override String unowned_footnote({required Object count}) => '${count} sem dono';
}

// Path: calendar
class _Translations$calendar$pt extends Translations$calendar$en {
	_Translations$calendar$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get tab => 'Vencimentos';
	@override String get title => 'O que vence';
	@override String subtitle({required Object days}) => 'Contratos públicos do seu estado que acabam nos próximos ${days} dias. Quando um vence, a compra tem que acontecer de novo — e isso você sabe meses antes de sair edital.';
	@override String get empty_title => 'Nada vencendo nesta janela';
	@override String empty_body({required Object days}) => 'Nenhum contrato do que você vende acaba nos próximos ${days} dias no seu estado.';
	@override String get scanning_title => 'Ainda lendo os contratos do seu território';
	@override String get scanning_body => 'Consultamos o portal hospital por hospital, sem atropelar o servidor deles. Vai enchendo ao longo dos próximos dias.';
	@override String get no_territory_title => 'Defina onde você vende';
	@override String get no_territory_body => 'Sem estado não dá para saber quais contratos te interessam. Ajuste em Configurações → Empresa.';
	@override String get no_keywords_title => 'Falta dizer o que você vende';
	@override String get no_keywords_body => 'O calendário usa as mesmas palavras do Radar. Cadastre seus produtos ou ajuste o monitoramento.';
	@override String days_left({required Object count}) => 'vence em ${count}d';
	@override String get days_today => 'vence hoje';
	@override String days_past({required Object count}) => 'venceu há ${count}d';
	@override String holder({required Object name}) => 'hoje com ${name}';
	@override String get holder_unknown => 'fornecedor atual não publicado';
	@override String get price_registry => 'ata de registro de preços';
	@override String get rides_others => 'compra por adesão';
	@override String get rides_others_hint => 'Este órgão pega carona em ata de outro. Dá para vender sem ganhar licitação.';
	@override String get open_record => 'Ver no portal';
	@override String get value_unknown => 'valor não publicado';
	@override String get work => 'Trabalhar este';
	@override String worked({required Object name}) => '${name} entrou no seu funil.';
	@override String get work_failed => 'Não consegui colocar no funil.';
	@override String get open_funnel => 'Ver no funil';
	@override String get work_queued => 'Entrou na sua lista de clientes. Falta o contato do comprador.';
}

// Path: feedback.cancel_reasons
class _Translations$feedback$cancel_reasons$pt extends Translations$feedback$cancel_reasons$en {
	_Translations$feedback$cancel_reasons$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get too_expensive => 'Muito caro';
	@override String get missing_features => 'Faltam recursos que preciso';
	@override String get not_using => 'Não estou usando o suficiente';
	@override String get switched_competitor => 'Mudei para outra ferramenta';
	@override String get technical_issues => 'Problemas técnicos ou bugs';
	@override String get temporary_pause => 'Só pausando por enquanto';
	@override String get other => 'Outro';
}

// Path: sales.products
class _Translations$sales$products$pt extends Translations$sales$products$en {
	_Translations$sales$products$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Produtos';
	@override String get subtitle => 'Gerencie os produtos do seu catálogo';
	@override String get new_product => 'Novo produto';
	@override String get edit_product => 'Editar produto';
	@override String get no_products => 'Nenhum produto ainda';
	@override String get empty_subtitle => 'Adicione os produtos que você vende para que seu agente de IA saiba o que está oferecendo.';
	@override String get create_first => 'Criar primeiro produto';
	@override String get load_error => 'Falha ao carregar produtos';
	@override String get active_label => 'Ativo';
	@override String get inactive_label => 'Inativo';
	@override String get field_name => 'Nome';
	@override String get field_name_hint => 'ex: Bomba de infusão';
	@override String get field_description => 'Descrição';
	@override String get field_description_hint => 'ex: Locação mensal com manutenção preventiva inclusa (opcional)';
	@override String get field_price => 'Preço';
	@override String get field_cycle => 'Ciclo de recompra (dias)';
	@override String get field_cycle_hint => 'ex.: 30 — de quanto em quanto tempo um cliente repõe isto. Vazio é desconhecido, e o produto nunca aparece nas recompras';
	@override String get cycle_invalid => 'Use um número inteiro de dias, até 3650';
	@override String get field_currency => 'Moeda';
	@override String get field_active => 'Ativo';
	@override String get field_active_hint => 'Produtos inativos ficam ocultos dos funis';
	@override String get name_required => 'O nome é obrigatório';
	@override String get price_required => 'Informe o preço';
	@override String get price_invalid => 'Preço ilegível — use 1.234,56';
	@override String get save => 'Salvar';
	@override String get created_success => 'Produto criado';
	@override String get updated_success => 'Produto atualizado';
	@override String get deleted_success => 'Produto excluído';
	@override String get delete_title => 'Excluir produto';
	@override String delete_message({required Object name}) => 'Excluir "${name}"? Esta ação não pode ser desfeita.';
	@override String get not_found => 'Produto não encontrado';
	@override String get not_found_hint => 'Ele pode ter sido apagado, ou o endereço está errado.';
	@override String get import_first => 'Importar planilha ou PDF';
	@override String get field_price_optional => 'Vazio = o vendedor cota na hora';
	@override String get sheet_title => 'Do folheto do fabricante';
	@override String get sheet_brand => 'Marca';
	@override String get sheet_model => 'Modelo';
	@override String get sheet_sku => 'Código';
	@override String get sheet_unit => 'Unidade';
	@override String get sheet_specs => 'Ficha técnica';
	@override String get sheet_accessories => 'Acessórios';
	@override String get sheet_standards => 'Normas e certificações';
	@override String open_brochure({required Object page}) => 'Abrir folheto (pág. ${page})';
	@override String get open_brochure_no_page => 'Abrir folheto';
	@override String get brochure_missing => 'Este produto não veio de um folheto arquivado';
	@override String get sheet_images => 'Imagens do folheto';
	@override String get sheet_images_empty => 'Nenhuma imagem aproveitável nesta página';
	@override String image_page({required Object page}) => 'pág. ${page}';
	@override String get image_open => 'Abrir imagem';
}

// Path: sales.funnels
class _Translations$sales$funnels$pt extends Translations$sales$funnels$en {
	_Translations$sales$funnels$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get title => 'Funis';
	@override String get new_funnel => 'Novo funil';
	@override String get edit_funnel => 'Editar funil';
	@override String get no_funnels => 'Nenhum funil ainda';
	@override String get empty_subtitle => 'Crie um funil para definir quais dados a IA deve coletar durante as conversas.';
	@override String get create_first => 'Criar primeiro funil';
	@override String get load_error => 'Falha ao carregar funis';
	@override String get active_label => 'Ativo';
	@override String get inactive_label => 'Inativo';
	@override String fields_count({required Object n}) => '${n} campos';
	@override String get field_name => 'Nome';
	@override String get field_name_hint => 'ex: Hospitais públicos de MT';
	@override String get name_required => 'O nome é obrigatório';
	@override String get save => 'Salvar';
	@override String get created_success => 'Funil criado';
	@override String get updated_success => 'Funil atualizado';
	@override String get deleted_success => 'Funil excluído';
	@override String get delete_title => 'Excluir funil';
	@override String delete_message({required Object name}) => 'Excluir "${name}"? Esta ação não pode ser desfeita.';
	@override String get not_found => 'Funil não encontrado';
	@override String get not_found_hint => 'Ele pode ter sido apagado, ou o endereço está errado.';
	@override String get default_undeletable => 'Funil padrão — importações e leads novos entram aqui; não pode ser excluído.';
}

// Path: sales.schema
class _Translations$sales$schema$pt extends Translations$sales$schema$en {
	_Translations$sales$schema$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get section_title => 'Dados a coletar';
	@override String get section_subtitle => 'Defina os campos que a IA deve coletar durante a conversa.';
	@override String get add_field => 'Adicionar campo';
	@override String get field_key => 'Chave';
	@override String get field_key_hint => 'ex: cpf_cliente';
	@override String get field_label => 'Rótulo';
	@override String get field_label_hint => 'ex: CPF do cliente';
	@override String get field_type => 'Tipo';
	@override String get field_required => 'Obrigatório';
	@override String get field_ai_hint => 'Dica para a IA';
	@override String get field_ai_hint_hint => 'Como/quando coletar (opcional)';
	@override String get field_enum_options => 'Opções (separadas por vírgula)';
	@override String get field_enum_options_hint => 'ex: pequeno, médio, grande';
	@override String get remove_field => 'Remover campo';
	@override String get no_fields => 'Nenhum campo definido ainda.';
	@override String get error_key_required => 'A chave é obrigatória';
	@override String get error_key_format => 'Use snake_case (letras minúsculas, números e _)';
	@override String get error_key_duplicate => 'Chave duplicada';
	@override String get error_enum_required => 'Defina pelo menos uma opção';
	@override late final _Translations$sales$schema$types$pt types = _Translations$sales$schema$types$pt._(_root);
}

// Path: sales.webhook
class _Translations$sales$webhook$pt extends Translations$sales$webhook$en {
	_Translations$sales$webhook$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get section_title => 'Webhook (opcional)';
	@override String get section_subtitle => 'Só preencha se você tem um sistema próprio para receber esses eventos. O funil funciona sem isso.';
	@override String get field_url => 'URL do webhook';
	@override String get field_url_hint => 'https://seu-servidor.com/webhooks/replider';
	@override String get url_invalid => 'Informe uma URL http(s) válida';
	@override String get field_secret => 'Segredo';
	@override String get field_secret_hint => 'Usado para assinar o payload (HMAC)';
	@override String get generate_secret => 'Gerar';
	@override String get regenerate_secret => 'Regerar';
	@override String get secret_copied => 'Segredo copiado';
	@override String get trigger_required_complete => 'Disparar quando campos obrigatórios completos';
	@override String get trigger_required_complete_hint => 'Envia um evento lead.qualified quando todos os campos obrigatórios são preenchidos.';
	@override String get trigger_buy_intent => 'Disparar ao detectar intenção de compra';
	@override String get trigger_buy_intent_hint => 'Envia um evento lead.buy_intent quando a IA detecta intenção de compra.';
	@override String get send_test => 'Enviar evento de teste';
	@override String get test_save_first => 'Salve o funil antes de enviar um evento de teste.';
	@override String get test_url_first => 'Defina uma URL de webhook antes de enviar um evento de teste.';
	@override String test_success({required Object status}) => 'Evento de teste enviado (HTTP ${status})';
	@override String get test_failed => 'Falha no evento de teste';
}

// Path: sales.lead_profile
class _Translations$sales$lead_profile$pt extends Translations$sales$lead_profile$en {
	_Translations$sales$lead_profile$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get section_title => 'Perfil de vendas';
	@override String get empty => 'Esta conversa não faz parte de nenhum funil.';
	@override String get funnel_label => 'Funil';
	@override String get status_label => 'Status do lead';
	@override String get score_label => 'Score';
	@override String get buy_intent => 'Intenção de compra detectada';
	@override String get no_value => 'Ainda não coletado';
	@override String get pending => 'Pendente';
	@override String get origin_ai => 'IA';
	@override String get origin_human => 'Humano';
	@override String get load_error => 'Não foi possível carregar o perfil de vendas.';
	@override late final _Translations$sales$lead_profile$lead_status$pt lead_status = _Translations$sales$lead_profile$lead_status$pt._(_root);
	@override String get edit_field => 'Editar campo';
	@override String edit_error({required Object error}) => 'Não foi possível salvar: ${error}';
	@override String get pick_date => 'Escolher data';
	@override String get new_proposal => 'Nova proposta';
}

// Path: sales.leads
class _Translations$sales$leads$pt extends Translations$sales$leads$en {
	_Translations$sales$leads$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get status_new => 'Novo';
	@override String get status_prospected => 'Prospectado';
	@override String get status_enriching => 'Enriquecendo';
	@override String get status_enriched => 'Enriquecido';
	@override String get status_pending_approval => 'Aguardando aprovação';
	@override String get status_contacted => 'Contatado';
	@override String get status_responded => 'Respondeu';
	@override String get status_qualified => 'Qualificado';
	@override String get status_won => 'Ganho';
	@override String get status_lost => 'Perdido';
	@override String get discard => 'Descartar';
	@override String get template_label => 'Template do WhatsApp';
	@override String get template_select => 'Escolha um template';
	@override String get template_required => 'Selecione um template aprovado antes de enviar';
	@override String get template_none => 'Nenhum template aprovado neste canal ainda';
	@override String get template_no_channel => 'Vincule um canal WhatsApp a este funil primeiro';
	@override String get template_load_error => 'Não foi possível carregar os templates';
}

// Path: sales.prospecting
class _Translations$sales$prospecting$pt extends Translations$sales$prospecting$en {
	_Translations$sales$prospecting$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get intro => 'Busque empresas no Google Places e importe-as como leads em um funil.';
	@override String get funnel_label => 'Funil de destino';
	@override String get keyword_label => 'Palavra-chave';
	@override String get keyword_hint => 'ex.: hospital, clínica de imagem, UTI';
	@override String get location_label => 'Localização';
	@override String get location_hint => 'ex.: Cuiabá, MT';
	@override String get radius_label => 'Raio (metros, opcional)';
	@override String get radius_hint => 'ex.: 5000';
	@override String get search_import => 'Buscar e importar';
	@override String get query_required => 'Informe uma palavra-chave ou localização e escolha um funil';
	@override String get no_funnels => 'Crie um funil primeiro para importar os leads prospectados.';
	@override String get result_title => 'Busca concluída';
	@override String result_created({required Object n, required Object total}) => '${n} de ${total} resultados importados como leads.';
	@override String result_skipped({required Object n}) => '${n} ignorados (já estão neste funil).';
	@override String get view_pipeline => 'Ver pipeline';
	@override String preview_found({required Object total, required Object duplicates}) => '${total} encontradas · ${duplicates} já na sua base';
	@override String quota_left({required Object count}) => 'restam ${count} leads no mês';
	@override String get already_lead => 'já está na sua base';
	@override String import_n({required Object count}) => 'Importar ${count} leads';
	@override String imported({required Object count}) => '${count} leads importados';
	@override String get select_all => 'Selecionar todos';
	@override String get select_none => 'Limpar seleção';
	@override String selected_of({required Object n, required Object total}) => '${n} de ${total} selecionados';
	@override String imported_partial({required Object created, required Object selected}) => '${created} de ${selected} importados — a franquia mensal de leads do plano parou o restante.';
	@override String get no_results_title => 'Nenhuma empresa encontrada';
	@override String get no_results_hint => 'Tente outra palavra-chave ou um raio maior — \'clínica\', \'hospital\', uma cidade vizinha.';
}

// Path: sales.schema.types
class _Translations$sales$schema$types$pt extends Translations$sales$schema$types$en {
	_Translations$sales$schema$types$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get text => 'Texto';
	@override String get number => 'Número';
	@override String get cpf => 'CPF';
	@override String get cnpj => 'CNPJ';
	@override String get date => 'Data';
	@override String get bool => 'Sim/Não';
	@override String get kEnum => 'Opções';
}

// Path: sales.lead_profile.lead_status
class _Translations$sales$lead_profile$lead_status$pt extends Translations$sales$lead_profile$lead_status$en {
	_Translations$sales$lead_profile$lead_status$pt._(TranslationsPt root) : this._root = root, super.internal(root);

	final TranslationsPt _root; // ignore: unused_field

	// Translations
	@override String get kNew => 'Novo';
	@override String get prospected => 'Prospectado';
	@override String get enriching => 'Enriquecendo';
	@override String get enriched => 'Enriquecido';
	@override String get pending_approval => 'Aguardando aprovação';
	@override String get contacted => 'Contatado';
	@override String get responded => 'Respondeu';
	@override String get qualified => 'Qualificado';
	@override String get won => 'Ganho';
	@override String get lost => 'Perdido';
}

/// The flat map containing all translations for locale <pt>.
/// Only for edge cases! For simple maps, use the map function of this library.
///
/// The Dart AOT compiler has issues with very large switch statements,
/// so the map is split into smaller functions (512 entries each).
extension on TranslationsPt {
	dynamic _flatMapFunction(String path) {
		return switch (path) {
			'home.onboarding_checklist_title' => ({required Object current, required Object total}) => 'Primeiros passos — ${current} de ${total}',
			'home.onboarding_checklist_dismiss' => 'Dispensar por enquanto',
			'home.onboarding_checklist_agent' => 'Crie seu primeiro agente',
			'home.onboarding_checklist_preview' => 'Experimente seu agente no preview',
			'home.onboarding_checklist_preview_cta' => 'Testar agora',
			'home.onboarding_checklist_channel' => 'Conecte um canal de mensagens',
			'home.onboarding_checklist_channel_cta' => 'Conectar',
			'home.onboarding_checklist_knowledge' => 'Ensine o agente sobre seu negócio',
			'home.onboarding_checklist_knowledge_cta' => 'Adicionar contexto',
			'home.onboarding_checklist_inbound' => 'Receba a primeira mensagem real',
			'home.welcome_user' => ({required Object name}) => 'Bem-vindo, ${name}',
			'home.onboarding_checklist_products' => 'Traga o que você vende',
			'home.onboarding_checklist_products_cta' => 'Importar lista de preços',
			'home.onboarding_checklist_customers' => 'Traga seus clientes',
			'home.onboarding_checklist_customers_cta' => 'Importar planilha',
			'home.onboarding_checklist_proposal' => 'Faça sua primeira proposta',
			'home.onboarding_checklist_proposal_cta' => 'Abrir o funil',
			'home.onboarding_checklist_team' => 'Convide seu vendedor',
			'home.onboarding_checklist_team_cta' => 'Convidar',
			'auth.login_hero_title' => 'Ofertas prontas para aprovar, todo dia.',
			'auth.login_eyebrow' => 'SEU VENDEDOR AUTÔNOMO',
			'auth.login_title' => 'Entre na sua conta',
			'auth.email_placeholder' => 'Digite seu e-mail',
			'auth.password_placeholder' => 'Digite sua senha',
			'auth.remember_me' => 'Lembrar-me',
			'auth.forgot_password' => 'Esqueci a senha',
			'auth.login_button' => 'Login',
			'auth.or_login_with' => 'Ou',
			'auth.google' => 'Google',
			'auth.meta' => 'Facebook',
			'auth.dont_have_account' => 'Não tem uma conta?',
			'auth.create_account' => 'Criar conta',
			'auth.register_title' => 'Criar Conta',
			'auth.register_subtitle' => 'Comece a usar o Replider hoje mesmo',
			'auth.register_paused_title' => 'Cadastro em breve',
			'auth.register_paused_body' => 'Estamos preparando a nova Replider e os cadastros estão temporariamente fechados. Já tem conta? É só entrar.',
			'auth.register_paused_error' => 'Os cadastros estão temporariamente fechados.',
			'auth.account_name' => 'Nome da empresa',
			'auth.full_name' => 'Nome completo',
			'auth.confirm_password' => 'Confirmar senha',
			'auth.already_have_account' => 'Já tem uma conta?',
			'auth.sign_in' => 'Entrar',
			'auth.field_required' => 'Campo obrigatório',
			'auth.invalid_email' => 'E-mail inválido',
			'auth.password_too_short' => 'Senha muito curta',
			'auth.min_characters' => ({required Object n}) => 'Mínimo ${n} caracteres',
			'auth.passwords_dont_match' => 'Senhas não coincidem',
			'auth.forgot_title' => 'Recuperar senha',
			'auth.forgot_subtitle' => 'Informe seu e-mail e enviaremos as instruções.',
			'auth.forgot_send_link' => 'Enviar link',
			'auth.forgot_resend_in' => ({required Object seconds}) => 'Reenviar em ${seconds}s',
			'auth.forgot_success' => 'Se a conta existir, você receberá um e-mail em instantes.',
			'auth.reset_title' => 'Defina uma nova senha',
			'auth.reset_subtitle' => 'Escolha uma nova senha para a sua conta.',
			'auth.reset_new_password' => 'Nova senha',
			'auth.reset_confirm_password' => 'Confirme a nova senha',
			'auth.reset_min_length' => 'A senha deve ter ao menos 8 caracteres.',
			'auth.reset_mismatch' => 'As senhas não coincidem.',
			'auth.reset_submit' => 'Redefinir senha',
			'auth.reset_success' => 'Senha redefinida! Agora você pode entrar com a nova senha.',
			'auth.reset_invalid_link' => 'Este link de redefinição é inválido ou incompleto. Solicite um novo.',
			'auth.back_to_login' => 'Voltar para o login',
			'auth.terms_required' => 'Você precisa aceitar os Termos de Uso e a Política de Privacidade.',
			'auth.retry' => 'Tentar novamente',
			'auth.mfa_title' => 'Verificação em duas etapas',
			'auth.mfa_subtitle' => 'Digite o código de 6 dígitos do app autenticador, ou um dos seus códigos de recuperação.',
			'auth.mfa_code_hint' => 'Código',
			'auth.mfa_verify_button' => 'Validar e continuar',
			'auth.mfa_cancel' => 'Voltar ao login',
			'auth.coupon_banner_valid' => ({required Object code, required Object percent}) => 'Cupom ${code} aplicado — ${percent}% off no checkout.',
			'auth.coupon_banner_invalid' => ({required Object code}) => 'Cupom ${code} não é mais válido.',
			'auth.session_expired_notice' => 'Sua sessão expirou por segurança. Entre novamente — você volta para a tela onde estava.',
			'auth.session_ending_title' => ({required Object minutes}) => 'Sua sessão termina em ${minutes} min',
			'auth.session_ending_body' => 'Entre de novo agora, na sua hora, em vez de perder o que estiver fazendo no meio.',
			'auth.session_ending_cta' => 'Entrar de novo',
			'auth.invite_banner' => ({required Object email}) => 'Convite para ${email} — sua conta já nasce no plano contratado.',
			'auth.invite_invalid' => 'Este link de convite é inválido, expirou ou já foi usado. Peça um novo link.',
			'auth.invite_invalid_title' => 'Link de convite expirado',
			'auth.register_subtitle_invite' => 'Ative a conta do plano que combinamos com você.',
			'auth.invite_probe_failed' => 'Não conseguimos verificar seu convite agora — problema de conexão, não do link. Tente de novo.',
			'auth.verify_email_unknown' => 'o e-mail da sua conta',
			'auth.consent_prefix' => 'Li e aceito os ',
			'auth.consent_terms' => 'Termos de Uso',
			'auth.consent_and' => ' e a ',
			'auth.consent_privacy' => 'Política de Privacidade',
			'profile.title' => 'Meu Perfil',
			'profile.user_not_found' => 'Usuário não encontrado',
			'profile.personal_info' => 'INFORMAÇÕES PESSOAIS',
			'profile.name' => 'Nome',
			'profile.email' => 'E-mail',
			'profile.save_info' => 'Salvar Informações',
			'profile.change_password' => 'ALTERAR SENHA',
			'profile.current_password' => 'Senha atual',
			'profile.new_password' => 'Nova senha',
			'profile.confirm_new_password' => 'Confirmar nova senha',
			'profile.change_password_button' => 'Alterar Senha',
			'profile.fill_all_fields' => 'Preencha todos os campos',
			'profile.fill_all_password_fields' => 'Preencha todos os campos de senha',
			'profile.passwords_dont_match' => 'As senhas não coincidem',
			'profile.min_password_length' => 'A nova senha deve ter pelo menos 6 caracteres',
			'profile.profile_updated' => 'Perfil atualizado com sucesso!',
			'profile.password_changed' => 'Senha alterada com sucesso!',
			'profile.profile_update_unavailable' => 'Atualização de perfil ainda não disponível na API',
			'profile.password_change_unavailable' => 'Alteração de senha ainda não disponível na API',
			'profile.incorrect_current_password' => 'Senha atual incorreta',
			'profile.unexpected_error' => 'Erro inesperado',
			'profile.network_error' => 'Erro de rede',
			'profile.error_prefix' => 'Erro',
			'profile.two_factor_section_label' => 'AUTENTICAÇÃO EM DUAS ETAPAS',
			'profile.two_factor_label' => 'Autenticação em duas etapas',
			'profile.two_factor_desc' => 'Adiciona uma segunda etapa no login com um código de 6 dígitos de um app autenticador (Google Authenticator, 1Password, Authy).',
			'profile.two_factor_status_off' => 'DESATIVADA',
			'profile.two_factor_status_on' => 'ATIVA',
			'profile.two_factor_setup_cta' => 'Ativar 2FA',
			'profile.two_factor_step1' => '1. Escaneie este QR no app autenticador',
			'profile.two_factor_step2' => '2. Digite o código de 6 dígitos que aparece no app',
			'profile.two_factor_manual_secret_label' => 'Ou digite este segredo manualmente:',
			'profile.two_factor_verify_cta' => 'Validar e ativar',
			'profile.two_factor_disable_prompt' => 'Digite um código 2FA atual (ou código de recuperação) para desativar.',
			'profile.two_factor_code_hint' => 'Código de 6 dígitos ou de recuperação',
			'profile.two_factor_disable_cta' => 'Desativar 2FA',
			'profile.two_factor_recovery_title' => 'Salve seus códigos de recuperação',
			'profile.two_factor_recovery_warning' => 'Cada código funciona uma vez. Guarde em local seguro — sem eles, perder o celular tranca o acesso.',
			'profile.two_factor_recovery_copy_cta' => 'Copiar todos',
			'profile.two_factor_recovery_done_cta' => 'Já salvei',
			'profile.signature_label' => 'Assinatura',
			'profile.signature_help' => 'Assinatura opcional oferecida no composer em respostas manuais. Nunca aplicada às respostas da IA.',
			'onboarding.welcome_title' => 'Bem-vindo ao Replider!',
			'onboarding.welcome_subtitle' => 'Vamos configurar seu workspace em poucos passos.',
			'onboarding.company_name_hint' => 'Nome da sua empresa',
			'onboarding.step_company_title' => 'Sobre a sua empresa',
			'onboarding.step_company_subtitle' => 'A IA escreve as abordagens a partir daqui. Sem isso, ela só sabe o nome.',
			'onboarding.step_company_placeholder' => 'Ex.: Sua Empresa Equipamentos Hospitalares',
			'onboarding.company_about_label' => 'O que vocês fazem',
			'onboarding.company_about_placeholder' => 'Ex.: Venda, locação e assistência técnica de equipamentos hospitalares em Mato Grosso.',
			'onboarding.company_about_help' => 'Uma frase. Quem você atende e o que resolve.',
			'onboarding.company_tone_label' => 'Como vocês falam (opcional)',
			'onboarding.company_lines_label' => 'Como a sua empresa ganha dinheiro?',
			'onboarding.company_line_sell' => 'Vendo equipamentos',
			'onboarding.company_line_rent' => 'Alugo equipamentos',
			'onboarding.company_line_service' => 'Faço assistência técnica',
			'onboarding.company_territory_label' => 'Onde você vende?',
			'onboarding.company_territory_uf' => 'Estado',
			'onboarding.company_territory_cities' => 'Cidades (opcional)',
			'onboarding.company_territory_cities_hint' => 'Cuiabá, Várzea Grande',
			'onboarding.company_tone_direct' => 'Direto',
			'onboarding.company_tone_direct_prose' => 'Escreva de forma direta e objetiva, sem rodeios nem adjetivos de venda.',
			'onboarding.company_tone_consultative' => 'Consultivo',
			'onboarding.company_tone_consultative_prose' => 'Escreva como um especialista que entende do problema do cliente, explicando o porquê antes do que.',
			'onboarding.company_tone_warm' => 'Próximo',
			'onboarding.company_tone_warm_prose' => 'Escreva de forma calorosa e próxima, como quem já conhece a pessoa, sem ser informal demais.',
			'onboarding.company_tone_formal' => 'Formal',
			'onboarding.company_tone_formal_prose' => 'Escreva de forma formal e respeitosa, tratando por você e evitando gírias.',
			'onboarding.step_agent_subtitle' => '',
			'onboarding.provider_label' => 'Provedor de IA',
			'onboarding.step_channel_title' => 'Por onde você manda?',
			'onboarding.step_channel_subtitle' => 'A primeira oferta sai por e-mail: conecte a conta que você já usa. WhatsApp entra depois, para o relacionamento.',
			'onboarding.step_channel_badge_telegram' => 'Mais rápido',
			'onboarding.step_channel_badge_whatsapp' => 'Popular · ~10 min',
			'onboarding.step_channel_badge_coming_soon' => 'Em breve',
			'onboarding.step_channel_whatsapp_help' => 'Não sei por onde começar',
			'onboarding.step_channel_whatsapp_help_failed' => 'Não consegui abrir o vídeo de ajuda. Confira as configurações do navegador.',
			'onboarding.telegram_bot_token' => 'Token do Bot Telegram',
			'onboarding.whatsapp_phone_number_id' => 'ID do Número WhatsApp',
			'onboarding.whatsapp_access_token' => 'Token de Acesso WhatsApp',
			'onboarding.instagram_page_id' => 'Page ID do Instagram',
			'onboarding.instagram_access_token' => 'Token de Acesso do Instagram',
			'onboarding.step_channel_instagram_hint' => 'Requer uma conta profissional do Instagram (Business ou Creator) e um token de acesso com a permissão instagram_business_manage_messages.',
			'onboarding.sms_account_sid' => 'Account SID da Twilio',
			'onboarding.sms_auth_token' => 'Auth Token da Twilio',
			'onboarding.sms_from_number' => 'Número Twilio (E.164)',
			'onboarding.step_channel_sms_hint' => 'Use seu próprio número Twilio: cole o Account SID, o Auth Token e o número de telefone do console da Twilio. Ao finalizar, copie a URL de webhook do canal em Configurações → Canais e configure-a no seu número na Twilio.',
			'onboarding.invalid_sms_account_sid' => 'O Account SID deve começar com AC seguido de 32 caracteres. Copie do console da Twilio.',
			'onboarding.invalid_sms_auth_token' => 'Cole seu Auth Token da Twilio para conectar o canal de SMS.',
			'onboarding.invalid_sms_from_number' => 'O número deve estar no formato internacional E.164, ex.: +5565999999999.',
			'onboarding.create_agent_first_for_file' => 'Crie o agente antes de enviar o arquivo.',
			'onboarding.create_agent_first_for_url' => 'Crie o agente antes de importar a URL.',
			'onboarding.knowledge_summary_file' => ({required Object fileName, required Object count}) => '${fileName} · ${count} trechos indexados',
			'onboarding.knowledge_summary_url' => ({required Object pages, required Object count}) => '${pages} páginas · ${count} trechos indexados',
			'onboarding.step_done_title' => 'Tudo pronto!',
			'onboarding.step_done_subtitle' => 'Seu workspace está configurado. As mensagens começarão a chegar assim que seus clientes entrarem em contato.',
			'onboarding.step_done_tip_1' => 'Teste seu agente em Configurações → Agente → Preview',
			'onboarding.step_done_tip_2' => 'Monitore conversas no Dashboard',
			'onboarding.step_done_tip_3' => 'Adicione mais conhecimento a qualquer momento em Configurações',
			'onboarding.go_to_dashboard' => 'Ir para o Dashboard',
			'onboarding.next' => 'Próximo',
			'onboarding.back' => 'Voltar',
			'onboarding.skip' => 'Pular por agora',
			'onboarding.step_of' => ({required Object current, required Object total}) => 'Passo ${current} de ${total}',
			'onboarding.field_required' => 'Campo obrigatório',
			'onboarding.step_preview_chat_next' => 'Funcionou! Avançar',
			'onboarding.step_preview_chat_skip' => 'Testar depois',
			'onboarding.step_channel_whatsapp_manual_hint' => 'Informe o Phone Number ID do painel Meta for Developers. O token de acesso é gerenciado automaticamente.',
			'onboarding.step_channel_whatsapp_connected' => 'WhatsApp conectado! Clique em "Próximo" para continuar.',
			'onboarding.step_channel_whatsapp_or_manual' => 'ou conecte manualmente',
			'onboarding.step_channel_badge_email' => 'Principal',
			'onboarding.email_from_address' => 'E-mail de envio',
			'onboarding.email_smtp_host' => 'Servidor SMTP',
			'onboarding.email_smtp_port' => 'Porta',
			'onboarding.email_smtp_password' => 'Senha SMTP',
			'onboarding.email_dns_title' => 'Depois: liberar seu domínio para enviar',
			'onboarding.email_dns_body' => 'Seu domínio precisa publicar três registros (SPF, DKIM e DMARC) para que os e-mails cheguem na caixa de entrada em vez do spam. Ao terminar, o Replider mostra os registros prontos para copiar. A propagação leva de minutos a algumas horas.',
			'onboarding.products_title' => 'O que você vende?',
			'onboarding.products_subtitle' => 'Traga sua lista de preços como arquivo. Toda proposta é feita disso, e você pode corrigir ou adicionar depois.',
			'onboarding.aikey_title' => 'Sua chave de IA',
			'onboarding.aikey_subtitle' => 'No seu contrato, a IA roda na SUA chave — o consumo é seu, sem intermediário. Cole a chave do Google AI Studio (Gemini) para ler catálogos e escrever ofertas.',
			'onboarding.aikey_field' => 'Chave da API (Gemini)',
			'onboarding.aikey_hint' => 'Guardamos criptografada e nunca mostramos de volta — só os últimos dígitos.',
			'onboarding.aikey_saved' => 'Chave configurada ✓',
			'onboarding.aikey_saved_hint' => 'Já tem chave nesta conta. Pode seguir.',
			'onboarding.aikey_save' => 'Salvar chave',
			'onboarding.aikey_skip_note' => 'Pode seguir sem cadastrar: a lista de preços entra por CSV, que não usa IA. O PDF de catálogo e as ofertas por IA ficam para quando você colar a chave.',
			'onboarding.products_needs_key' => 'Leitura de PDF precisa da chave de IA da conta. Por enquanto, traga a lista em CSV — ela não usa IA.',
			'onboarding.email_smtp_toggle' => 'Tenho servidor próprio (SMTP)',
			'onboarding.step_channel_other_channels' => 'Outros canais',
			'onboarding.step_channel_other_channels_hide' => 'Ocultar outros canais',
			'onboarding.customers_title' => 'Quem já compra de você?',
			'onboarding.customers_subtitle' => 'Traga sua lista de clientes como arquivo. Recompra, cross-sell e a primeira oferta começam daqui — dá pra adicionar mais depois.',
			'onboarding.first_offer_title' => 'Sua primeira oferta',
			'onboarding.first_offer_writing' => 'Lendo seus produtos e seus clientes e escrevendo a primeira mensagem…',
			'onboarding.first_offer_for_customer' => ({required Object name}) => 'Para ${name}, um dos seus clientes:',
			'onboarding.first_offer_for_prospect' => ({required Object name}) => 'Para ${name}, encontrado no seu território:',
			'onboarding.first_offer_in_queue' => 'É assim que as oportunidades vão chegar todo dia. Esta já está na sua lista de Oportunidades — aprove, edite ou peça outra abordagem lá. Nada sai sem você.',
			'onboarding.first_offer_nothing' => 'Ainda não há o que escrever: traga uma lista de clientes, ou diga em que estado você vende, e a primeira oportunidade vai estar esperando em Oportunidades.',
			'onboarding.first_offer_no_business' => 'Não conseguimos encontrar uma empresa no seu território agora. Traga uma lista de clientes, ou tente a busca em Oportunidades mais tarde.',
			'onboarding.finish_to_queue' => 'Ver minha fila de ofertas',
			'onboarding.unconfirmed_import_title' => 'Arquivo não importado',
			'onboarding.unconfirmed_import_body' => 'Você revisou o arquivo mas ainda não confirmou a importação. Se avançar agora, nada será criado.',
			'onboarding.unconfirmed_import_confirm' => 'Avançar sem importar',
			'onboarding.company_required_hint' => 'Os campos com * são obrigatórios — o Próximo libera quando os quatro estiverem preenchidos.',
			'onboarding.first_offer_import_cta' => 'Voltar e importar clientes',
			'onboarding.first_offer_unavailable' => 'A busca de empresas não está disponível agora — importe uma lista de clientes, ou pule: a prospecção funciona normalmente depois.',
			'onboarding.first_offer_prospecting_not_configured' => 'A busca de empresas ainda não está configurada na plataforma — importe uma lista de clientes, ou pule: a prospecção é liberada depois.',
			'onboarding.first_offer_cta_products' => 'Importar produtos',
			'onboarding.first_offer_cta_customers' => 'Corrigir lista de clientes',
			'onboarding.first_offer_cta_settings' => 'Abrir Configurações',
			'dashboard.conversations_title' => 'Conversas',
			'dashboard.unknown_contact' => 'Cliente',
			'dashboard.reply_reminder_toast' => ({required Object contact, required Object minutes}) => '${contact} está aguardando há ${minutes} min — responda agora',
			'dashboard.reply_reminder_open' => 'Abrir',
			'dashboard.search_conversations' => 'Buscar conversas...',
			'dashboard.search_in_conversation' => 'Buscar na conversa...',
			'dashboard.no_conversation_selected' => 'Selecione uma conversa',
			'dashboard.no_conversation_selected_subtitle' => 'Escolha uma conversa da lista para visualizar as mensagens.',
			'dashboard.settings' => 'Configurações',
			'dashboard.message_placeholder' => 'Digite uma mensagem...',
			'dashboard.no_conversations_found' => 'Nenhuma conversa encontrada',
			'dashboard.empty_inbox_hint_setup' => 'Sua caixa está vazia. Conecte um e-mail para receber e responder as mensagens dos clientes aqui.',
			'dashboard.empty_inbox_cta_setup' => 'Conectar e-mail',
			'dashboard.empty_inbox_hint_share' => 'Sua caixa está vazia. As respostas dos clientes às suas ofertas aparecem aqui.',
			'dashboard.empty_inbox_cta_share' => 'Ver ofertas',
			'dashboard.no_messages' => 'Nenhuma mensagem',
			'dashboard.today' => 'Hoje',
			'dashboard.yesterday' => 'Ontem',
			'dashboard.now' => 'Agora',
			'dashboard.bot_active' => 'IA ativa',
			'dashboard.waiting_agent' => 'Aguardando atendente',
			'dashboard.closed' => 'Encerrada',
			'dashboard.agent_label' => 'Atendente',
			'dashboard.search_hint' => 'Buscar...',
			'dashboard.close_search' => 'Fechar busca',
			'dashboard.search' => 'Buscar',
			'dashboard.analytics' => 'Analytics',
			'dashboard.settings_tooltip' => 'Configurações',
			'dashboard.search_in_messages' => 'Buscar...',
			'dashboard.no_conversations_for_query' => ({required Object query}) => 'Nenhuma conversa com "${query}"',
			'dashboard.results_for_query' => ({required Object count, required Object query}) => '${count} resultado(s) para "${query}"',
			'dashboard.error_loading_conversations' => 'Erro ao carregar conversas',
			'dashboard.check_connection' => 'Verifique sua conexão e tente novamente.',
			'dashboard.filter_all' => 'Todas',
			'dashboard.filter_open' => 'Abertas',
			'dashboard.filter_pending' => 'Pendentes',
			'dashboard.filter_closed' => 'Fechadas',
			'dashboard.assignment_mine' => 'Minhas',
			'dashboard.assignment_unassigned' => 'Não atribuídas',
			'dashboard.assignment_all' => 'Todas',
			'dashboard.sort_recent' => 'Recentes',
			'dashboard.sort_first_in_conv' => 'Ordem de chegada',
			'dashboard.sort_oldest' => 'Mais antigas',
			'dashboard.select_conversation_title' => 'Selecione uma conversa',
			'dashboard.select_conversation_subtitle' => 'Escolha uma conversa da lista para\nvisualizar as mensagens.',
			'dashboard.drop_to_send' => 'Solte para enviar',
			'dashboard.search_messages_hint' => 'Buscar...',
			'dashboard.summary_tooltip' => 'Resumo da conversa (IA)',
			'dashboard.send_file_tooltip' => 'Enviar arquivo',
			'dashboard.record_audio_tooltip' => 'Gravar áudio',
			'dashboard.send_flow_tooltip' => 'Enviar Flow',
			'dashboard.type_a_message' => 'Digite uma mensagem...',
			'dashboard.bot_paused_pill' => 'IA pausada',
			'dashboard.human_handling' => 'Em atendimento humano',
			'dashboard.status_open_bot' => 'IA ativa',
			'dashboard.status_pending_human' => 'Aguardando atendente',
			'dashboard.status_closed' => 'Encerrada',
			'dashboard.day_today' => 'Hoje',
			'dashboard.day_yesterday' => 'Ontem',
			'dashboard.summary_title' => 'Resumo da conversa',
			'dashboard.summary_failed' => ({required Object error}) => 'Falha ao gerar resumo: ${error}',
			'dashboard.summary_empty' => 'Sem mensagens suficientes para resumir.',
			'dashboard.summary_refresh' => 'Atualizar',
			'dashboard.wa_24h_window_expired' => 'Janela de 24h expirou. Envie um template aprovado para reabrir a conversa.',
			'dashboard.wa_send_template' => 'Enviar template',
			'dashboard.retry_send' => 'Tentar',
			'dashboard.flow_sent' => ({required Object name}) => 'Flow "${name}" enviado.',
			'dashboard.token_usage_warning_snackbar' => ({required Object percent}) => 'Uso de tokens em ${percent}% do limite.',
			'dashboard.reconnecting' => 'Reconectando...',
			'dashboard.tokens_exhausted_banner' => 'Créditos de IA esgotados. Adquira extras ou faça upgrade.',
			'dashboard.tokens_high_usage_banner' => ({required Object percent}) => 'Você já usou ${percent}% dos seus créditos de IA este mês.',
			'dashboard.upgrade_btn' => 'Falar com a gente',
			'dashboard.trial_ended' => 'Seu período de teste terminou. Faça upgrade para continuar usando o Replider.',
			'dashboard.trial_last_day' => 'Último dia de teste gratuito. Faça upgrade para não interromper o atendimento.',
			'dashboard.trial_days_remaining' => ({required Object days, required Object used, required Object max}) => 'Restam ${days} dias do seu teste gratuito (${used}/${max} conversas). Faça upgrade para desbloquear mais.',
			'dashboard.trial_free_plan' => ({required Object used, required Object max}) => 'Você está em período de teste (${used} de ${max} conversas usadas). Quer o produto completo rodando na sua empresa? Vamos conversar.',
			'dashboard.insert_signature_tooltip' => 'Inserir assinatura',
			'dashboard.channel_all' => 'Todos os canais',
			'dashboard.channel_email' => 'E-mail',
			'dashboard.channel_whatsapp' => 'WhatsApp',
			'dashboard.channel_telegram' => 'Telegram',
			'dashboard.channel_instagram' => 'Instagram',
			'dashboard.channel_sms' => 'SMS',
			'dashboard.sort_importance' => 'Mais importante',
			'dashboard.category_all' => 'Todas as categorias',
			'dashboard.category_hot_lead' => 'Lead quente',
			'dashboard.category_customer' => 'Cliente',
			'dashboard.category_financial' => 'Financeiro',
			'dashboard.category_supplier' => 'Fornecedor',
			'dashboard.category_newsletter' => 'Newsletter',
			'dashboard.category_quarantine' => 'Quarentena',
			'dashboard.category_other' => 'Outros',
			'dashboard.filters_button' => 'Filtros',
			'dashboard.filters_button_n' => ({required Object count}) => 'Filtros · ${count}',
			'dashboard.filters_clear' => 'Limpar filtros',
			'dashboard.filters_title' => 'Filtrar conversas',
			'dashboard.filter_label_status' => 'Situação',
			'dashboard.filter_label_channel' => 'Canal',
			'dashboard.filter_label_sort' => 'Ordem',
			'dashboard.filter_label_category' => 'Categoria',
			'dashboard.no_conversations_for_filters' => 'Nenhuma conversa com estes filtros.',
			'dashboard.clear_filters' => 'Limpar filtros',
			'dashboard.media_file_label' => 'Arquivo',
			'dashboard.media_location_label' => 'Localização',
			'dashboard.wa_unsupported_title' => 'Conteúdo não entregue pelo WhatsApp',
			'dashboard.wa_unsupported_body' => 'Enquete, localização ao vivo ou visualização única não chegam ao atendimento. Peça para o cliente reenviar de outra forma.',
			'dashboard.wa_window_composer_back' => 'O campo de mensagem volta quando o cliente responder.',
			'dashboard.brief_agreed' => 'ACORDADO',
			'dashboard.brief_pending' => 'PENDENTE',
			'dashboard.brief_next_step' => 'PRÓXIMO PASSO',
			'dashboard.brief_generated_now' => 'gerado agora',
			'dashboard.brief_generated_ago' => ({required Object when}) => 'gerado há ${when}',
			'dashboard.brief_copied' => 'Resumo copiado.',
			'dashboard.summary_too_short' => 'Esta conversa ainda é curta — leia as mensagens, o resumo seria maior que a própria thread.',
			'email_verification.title' => 'Verifique seu email',
			'email_verification.subtitle' => 'Enviamos um link de verificação para',
			'email_verification.instruction' => 'Clique no link enviado para ativar sua conta. O link expira em 24 horas.',
			'email_verification.back_to_login' => 'Voltar para o login',
			'email_verification.didnt_receive' => 'Não recebeu o e-mail?',
			'email_verification.resend' => 'Reenviar e-mail',
			'email_verification.resent' => 'E-mail enviado. Confira a caixa em alguns instantes.',
			'email_verification.resend_failed' => 'Não conseguimos reenviar. Tente novamente em um minuto.',
			'email_verification.check_spam' => 'Verifique também a pasta de spam.',
			'email_verification.banner_message' => 'Confirme seu e-mail para garantir o acesso à conta.',
			'email_verification.verifying' => 'Verificando seu e-mail…',
			'email_verification.verified_title' => 'E-mail verificado',
			'email_verification.verified_subtitle' => 'Seu e-mail foi confirmado. Você já pode entrar.',
			'email_verification.verify_failed_title' => 'Falha na verificação',
			'email_verification.verify_failed_subtitle' => 'Este link é inválido ou expirou. Solicite um novo pelo app.',
			'settings.title' => 'Configurações',
			'settings.account' => 'Conta',
			'settings.subscription' => 'Assinatura',
			'settings.team' => 'Equipe',
			'settings.manage_team' => 'Gerenciar Equipe',
			'settings.manage_team_subtitle' => 'Convide membros e gerencie cargos',
			'settings.ai_agents' => 'Agentes de IA',
			'settings.kNew' => 'Novo',
			'settings.no_agents' => 'Nenhum agente configurado',
			'settings.no_agent_selected' => 'Agente não selecionado',
			'settings.error_loading_team' => 'Erro ao carregar equipe',
			'settings.error_loading_tags' => 'Erro ao carregar tags',
			'settings.error_loading_responses' => 'Erro ao carregar respostas',
			'settings.error_loading_calendars' => 'Erro ao carregar calendários',
			'settings.error_loading_templates' => 'Erro ao carregar templates',
			'settings.error_loading_webhooks' => 'Falha ao carregar',
			'settings.agent_not_found' => 'Agente não encontrado',
			'settings.create_agent' => 'Criar Agente',
			'settings.active' => 'Ativo',
			'settings.inactive' => 'Inativo',
			'settings.error_loading_agents' => 'Erro ao carregar agentes',
			'settings.retry' => 'Tentar novamente',
			'settings.logout' => 'Sair da conta',
			'settings.plan_and_usage' => 'Plano e Uso',
			'settings.manage_plan' => 'Gerencie seu plano e acompanhe o uso',
			'settings.delete_agent_title' => 'Excluir Agente',
			'settings.delete_agent_confirm' => ({required Object name}) => 'Deseja realmente excluir "${name}"? Esta ação não pode ser desfeita.',
			'settings.cancel' => 'Cancelar',
			'settings.delete' => 'Excluir',
			'settings.agent_deleted' => 'Agente excluído com sucesso',
			'settings.new_agent' => 'Novo Agente',
			'settings.agent_name' => 'Nome do agente',
			'settings.ai_provider' => 'Provedor de IA',
			'settings.create' => 'Criar',
			'settings.agent_created' => 'Agente criado com sucesso!',
			'settings.delete_agent_tooltip' => 'Excluir agente',
			'settings.no_channels' => 'Nenhum canal conectado',
			'settings.no_messages' => 'Nenhuma mensagem',
			'settings.typing' => 'digitando...',
			'settings.account_subtitle' => 'Nome da empresa, workspace e zona de perigo',
			'settings.company_section_title' => 'Empresa',
			'settings.company_name_label' => 'Nome da empresa',
			'settings.company_name_helper' => 'É assim que o agente de IA se apresenta e como seu workspace aparece para o time.',
			'settings.company_save_success' => 'Nome da empresa atualizado',
			'settings.company_save_cta' => 'Salvar',
			'settings.branding_section_title' => 'Identidade visual',
			'settings.branding_section_subtitle' => 'Logo e fuso horário usados pelo agente.',
			'settings.branding_logo_label' => 'Logo do workspace',
			'settings.branding_logo_helper' => 'PNG ou JPG, tamanho máximo 2 MB. Aparece no cabeçalho e em canais futuros.',
			'settings.branding_logo_upload_cta' => 'Enviar logo',
			'settings.branding_logo_change_cta' => 'Substituir',
			'settings.branding_logo_remove_cta' => 'Remover',
			'settings.branding_logo_uploading' => 'Enviando logo...',
			'settings.branding_logo_too_large' => 'Logo acima de 2 MB',
			'settings.branding_logo_upload_failed' => 'Falha ao enviar logo',
			'settings.branding_timezone_label' => 'Fuso horário padrão',
			'settings.branding_timezone_helper' => 'Usado como padrão ao criar novos agentes. Agentes existentes mantêm o fuso atual.',
			'settings.branding_save_cta' => 'Salvar branding',
			'settings.branding_save_success' => 'Branding atualizado',
			'settings.danger_zone_title' => 'Zona de perigo',
			'settings.danger_zone_subtitle' => 'Ações irreversíveis no workspace.',
			'settings.delete_account_cta' => 'Excluir conta permanentemente',
			'settings.delete_account_dialog_title' => ({required Object company}) => 'Excluir ${company}?',
			'settings.delete_account_dialog_body' => 'Isso apaga todos os agentes, canais, conversas e mensagens permanentemente. Não dá pra desfazer. Digite o nome da empresa abaixo pra confirmar.',
			'settings.delete_account_type_hint' => ({required Object company}) => 'Digite "${company}" para liberar',
			'settings.delete_account_confirm_button' => 'Excluir workspace',
			'settings.delete_account_success' => 'Conta excluída.',
			'settings.agents_subtitle' => 'Gerencie os agentes de IA que respondem aos clientes',
			'settings.channels_subtitle' => 'E-mail para ofertas · WhatsApp para relacionamento',
			'settings.tags_subtitle' => 'Organize conversas por tema ou prioridade',
			'settings.canned_responses_subtitle' => 'Respostas rápidas que os atendentes podem reutilizar',
			'settings.campaigns_subtitle' => 'Templates de envio (WhatsApp)',
			'settings.compliance_subtitle' => 'LGPD e log de auditoria',
			'settings.smtp_test_title' => 'Teste de SMTP',
			'settings.smtp_test_subtitle' => 'Envia um e-mail pra você confirmar que o servidor consegue entregar mensagens.',
			'settings.smtp_test_cta' => 'Enviar e-mail de teste',
			'settings.smtp_test_success' => 'E-mail de teste enviado. Confira sua caixa.',
			'settings.smtp_test_not_configured' => 'SMTP não está configurado no servidor.',
			'settings.smtp_test_failed' => 'Falha ao enviar e-mail',
			'settings.wa_templates_pending_banner_one' => '1 template em análise pela Meta',
			'settings.wa_templates_pending_banner_other' => ({required Object count}) => '${count} templates em análise pela Meta',
			'settings.wa_templates_polling_hint' => 'Atualizando automaticamente a cada 30 segundos.',
			'settings.wa_phone_numbers_title' => 'Números no WABA',
			'settings.wa_phone_numbers_subtitle' => 'Todos os números registrados na sua WhatsApp Business Account. Cada número pode virar um canal separado no Replider.',
			'settings.wa_phone_numbers_empty' => 'Nenhum outro número encontrado neste WABA.',
			'settings.wa_phone_numbers_failed' => 'Falha ao listar números',
			'settings.wa_phone_numbers_current' => 'Conectado agora',
			'settings.audit_filter_action' => 'Ação (exata)',
			'settings.audit_filter_entity' => 'Tipo de entidade',
			'settings.audit_filter_actor' => 'ID do autor',
			'settings.audit_filter_apply' => 'Aplicar',
			'settings.audit_filter_clear' => 'Limpar',
			'settings.audit_export_cta' => 'Exportar CSV',
			'settings.audit_export_copied' => 'CSV copiado. Cole num arquivo .csv e abra no Excel.',
			'settings.audit_export_failed' => 'Falha ao exportar CSV',
			'settings.calendar_test_connection_cta' => 'Testar conexão',
			'settings.calendar_test_connection_ok' => 'Conectado com sucesso',
			'settings.calendar_test_connection_fail' => 'Falha de conexão',
			'settings.calendar_disconnect_cta' => 'Desconectar do Google',
			'settings.calendar_disconnect_confirm_title' => ({required Object name}) => 'Desconectar ${name} do Google?',
			'settings.calendar_disconnect_confirm_body' => 'O calendário continuará existindo no Replider, mas o agente não vai mais consultar eventos do Google. Você pode reconectar depois editando o calendário.',
			'settings.calendar_disconnect_confirm_cta' => 'Desconectar',
			'settings.calendar_disconnect_success' => 'Desconectado do Google.',
			'settings.gcal_title' => 'Google Calendar',
			'settings.gcal_connect_desc' => 'Conecte sua conta Google para o agente consultar horários livres e criar eventos direto na sua agenda.',
			'settings.gcal_connect_cta' => 'Conectar Google Calendar',
			'settings.gcal_connected_as' => ({required Object email}) => 'Conectado como ${email}',
			'settings.gcal_disconnect_cta' => 'Desconectar',
			'settings.gcal_disconnect_confirm_title' => 'Desconectar o Google Calendar?',
			'settings.gcal_disconnect_confirm_body' => 'O agente deixa de acessar sua agenda. Agendamentos voltam a depender da agenda compartilhada com a plataforma.',
			'settings.gcal_open_browser' => 'Continue a conexão na aba que abrimos no navegador. Volte aqui quando terminar.',
			'settings.gcal_connected_snackbar' => 'Google Calendar conectado!',
			'settings.gcal_error_snackbar' => 'Falha ao conectar o Google Calendar. Tente de novo.',
			'settings.gcal_denied_snackbar' => 'Conexão com o Google cancelada.',
			'settings.gcal_pick_calendar_label' => 'Agenda do Google',
			'settings.gcal_pick_calendar_hint' => 'Escolha em qual agenda os compromissos serão marcados',
			'settings.gcal_manual_id_toggle' => 'Inserir ID manualmente',
			'settings.team_metrics_period_today' => 'Hoje',
			'settings.team_metrics_period_7d' => '7d',
			'settings.team_metrics_period_30d' => '30d',
			'settings.team_metrics_period_mtd' => 'Mês',
			'settings.team_metrics_col_convs' => 'Convs',
			'settings.team_metrics_col_tmpr' => 'TMPR',
			'settings.team_metrics_col_csat' => 'CSAT',
			'settings.team_metrics_status_online' => 'Online',
			'settings.team_metrics_status_offline' => 'Offline',
			'settings.team_metrics_export_cta' => 'Exportar CSV',
			'settings.team_metrics_export_copied' => 'CSV copiado. Cole num arquivo .csv e abra no Excel.',
			'settings.team_metrics_export_failed' => 'Falha ao exportar',
			'settings.team_metrics_drill_title' => ({required Object name}) => 'Métricas de ${name}',
			'settings.team_metrics_card_volume' => 'Volume',
			'settings.team_metrics_card_speed' => 'Velocidade',
			'settings.team_metrics_card_quality' => 'Qualidade',
			'settings.team_metrics_card_activity' => 'Atividade',
			'settings.team_metrics_conversations_assigned' => 'Conversas atribuídas',
			'settings.team_metrics_conversations_resolved' => 'Resolvidas',
			'settings.team_metrics_messages_sent' => 'Mensagens enviadas',
			'settings.team_metrics_avg_first_response' => 'Tempo médio de 1ª resposta',
			'settings.team_metrics_avg_resolution' => 'Tempo médio de resolução',
			'settings.team_metrics_csat_avg' => 'CSAT médio',
			'settings.team_metrics_csat_count' => 'Respostas',
			'settings.team_metrics_last_seen' => 'Última atividade',
			'settings.team_metrics_empty' => 'Nenhum dado no período selecionado.',
			'settings.team_metrics_no_data' => '—',
			'settings.live_supervision_online' => 'Online',
			'settings.live_supervision_offline' => 'Offline',
			'settings.channels_title' => 'E-mail e canais',
			_ => null,
		} ?? switch (path) {
			'settings.business_hours_title' => 'Horário de atendimento',
			'settings.business_hours_enabled' => 'Responder só dentro do horário',
			'settings.business_hours_enabled_subtitle' => 'Fora do horário a IA fica quieta — atendentes ainda podem responder manualmente.',
			'settings.business_hours_start' => 'Início',
			'settings.business_hours_end' => 'Fim',
			'settings.business_hours_days' => 'Dias da semana',
			'settings.business_hours_save_success' => 'Horário salvo',
			'settings.back_tooltip' => 'Voltar',
			'settings.plan_expired_banner' => 'Seu contrato está inativo — envios e edições estão pausados. Fale com a gente para reativar.',
			'settings.view_plans' => 'Ver planos',
			'settings.channels_count_offline' => ({required Object count}) => '${count} offline',
			'settings.section_channels' => 'Canais',
			'settings.section_attendance' => 'Atendimento',
			'settings.section_compliance' => 'Conformidade',
			'settings.campaigns_title' => 'Campanhas',
			'settings.flows_title' => 'Flows do WhatsApp',
			'settings.flows_subtitle' => 'Formulários nativos para captura de leads e agendamentos.',
			'settings.webhooks_title' => 'Webhooks',
			'settings.webhooks_subtitle' => 'Integre com Zapier, n8n, CRM ou seu próprio backend.',
			'settings.compliance_title' => 'LGPD & Auditoria',
			'settings.logout_title' => 'Sair',
			'settings.logout_subtitle' => 'Encerra esta sessão',
			'settings.logout_confirm_title' => 'Sair da conta',
			'settings.logout_confirm_body' => 'Tem certeza? Você precisará entrar novamente para acessar o painel.',
			'settings.trial_expired_title' => 'Trial expirado',
			'settings.plan_inactive_title' => 'Plano inativo',
			'settings.plan_view_plans' => 'Ver planos',
			'settings.plan_label' => ({required Object name}) => 'Plano: ${name}',
			'settings.tokens_conversations_usage' => ({required Object tokens, required Object conv}) => 'Tokens: ${tokens}% | Conversas: ${conv}%',
			'settings.agent_name_placeholder' => 'Ex: Assistente de Vendas',
			'settings.ai_provider_label' => 'Provedor',
			'settings.agent_active_title' => 'Agente Ativo',
			'settings.agent_active_subtitle' => 'O agente está respondendo mensagens',
			'settings.agent_paused_subtitle' => 'O agente está pausado',
			'settings.agent_test_title' => 'Testar Agente',
			'settings.agent_test_subtitle' => 'Inicie uma conversa de teste com este agente',
			'settings.agent_calendars_title' => 'Calendários',
			'settings.agent_calendars_subtitle' => 'Google Calendar para agendamentos',
			'settings.agent_kb_title' => 'Base de Conhecimento',
			'settings.agent_kb_subtitle' => 'Textos e PDFs para inteligência artificial',
			'settings.agent_section_name' => 'NOME DO AGENTE',
			'settings.agent_section_ai' => 'INTELIGÊNCIA ARTIFICIAL',
			'settings.agent_section_prompt' => 'PROMPT DO SISTEMA',
			'settings.agent_section_personality' => 'PERSONALIDADE APRENDIDA',
			'settings.agent_section_business_hours' => 'HORÁRIO DE ATENDIMENTO',
			'settings.agent_section_behavior' => 'COMPORTAMENTO',
			'settings.agent_section_info' => 'INFORMAÇÕES ADICIONAIS',
			'settings.agent_prompt_helper_text' => 'Defina a personalidade e as instruções do agente de IA.',
			'settings.agent_prompt_library_match' => ({required Object name}) => 'Usando o prompt salvo: ${name}',
			'settings.agent_prompt_library_open' => 'Abrir biblioteca',
			'settings.agent_prompt_hint' => 'Você é um assistente de atendimento ao cliente da empresa X. Seu objetivo é ajudar os clientes com dúvidas sobre produtos e pedidos...',
			'settings.agent_save' => 'Salvar',
			'settings.agent_saved' => 'Salvo',
			'settings.upgrade_plan_action' => 'Atualizar plano',
			'settings.agent_not_found_message' => 'Agente não encontrado',
			'settings.agent_updated_success' => 'Agente atualizado com sucesso!',
			'settings.personality_empty_hint' => 'Nenhuma personalidade aprendida ainda. Responda manualmente algumas conversas e o bot passa a imitar o estilo do seu time.',
			'settings.personality_active_hint' => 'Personalidade derivada das respostas humanas mais recentes. É aplicada automaticamente ao bot.',
			'settings.personality_updated' => 'Personalidade atualizada.',
			'settings.bh_limit_label' => 'Limitar resposta automática a um horário',
			'settings.bh_explainer' => 'O bot só responde dentro deste horário. Fora da janela a conversa fica para um humano atender.',
			'settings.bh_start' => 'Início',
			'settings.bh_end' => 'Fim',
			'settings.bh_time_placeholder' => '--:--',
			'settings.bh_dom_d' => 'D',
			'settings.bh_d_sun' => 'D',
			'settings.bh_d_mon' => 'S',
			'settings.bh_d_tue' => 'T',
			'settings.bh_d_wed' => 'Q',
			'settings.bh_d_thu' => 'Q',
			'settings.bh_d_fri' => 'S',
			'settings.bh_d_sat' => 'S',
			'settings.behavior_hide_ai_title' => 'Estilo humano (não anuncia que é IA)',
			'settings.behavior_hide_ai_subtitle' => 'Bot escreve como um atendente humano e não diz espontaneamente que é IA. Se o cliente perguntar diretamente, responde com honestidade — exigência da política do WhatsApp (Meta).',
			'settings.behavior_silent_title' => 'Aguardar humano quando não souber',
			'settings.behavior_silent_subtitle' => 'Quando o bot não tiver a resposta, ele fica em silêncio e passa a conversa pro atendente sem avisar o cliente. Desligue para o bot dizer "vou verificar" no idioma do cliente.',
			'settings.agent_id_label' => 'ID',
			'settings.agent_timezone_label' => 'Timezone',
			'settings.agent_created_label' => 'Criado em',
			'settings.channels_back_tooltip' => 'Voltar',
			'settings.channels_screen_title' => 'Canais',
			'settings.channels_add_tooltip' => 'Adicionar canal',
			'settings.channels_empty_subtitle' => 'Conecte o e-mail que você já usa para as ofertas saírem, e o WhatsApp para o relacionamento.',
			'settings.channel_dialog_edit_title' => 'Editar canal',
			'settings.channel_dialog_add_title' => 'Adicionar canal',
			'settings.channel_dialog_edit_subtitle' => 'Atualize nome, credenciais ou agente responsável.',
			'settings.channel_dialog_add_subtitle' => 'Por onde suas ofertas saem e as respostas chegam.',
			'settings.channel_name_label' => 'Nome do canal',
			'settings.channel_name_hint' => 'Ex: E-mail de vendas',
			'settings.channel_or_manual' => 'ou conecte manualmente',
			'settings.channel_responsible_agent' => 'Agente responsável',
			'settings.channel_no_agent_human' => 'Nenhum (atendimento humano)',
			'settings.channel_save_btn' => 'Salvar',
			'settings.channel_connect_btn' => 'Conectar',
			'settings.channel_partial_credentials' => 'Preencha todos os campos pra trocar credenciais, ou deixe todos em branco pra manter as atuais.',
			'settings.channel_invalid_phone_id' => 'Phone Number ID inválido (deve ter 12-16 dígitos numéricos).',
			'settings.channel_invalid_waba_id' => 'WABA ID inválido (deve ter 12-18 dígitos numéricos).',
			'settings.channel_invalid_token' => 'Access Token muito curto. Cole o token completo da Meta.',
			'settings.channel_invalid_account_sid' => 'Account SID inválido (deve começar com AC seguido de 32 caracteres hexadecimais).',
			'settings.channel_invalid_from_number' => 'Número de telefone inválido. Use o formato E.164, ex.: +15551234567.',
			'settings.channel_updated_success' => 'Canal atualizado com sucesso!',
			'settings.channel_created_success' => 'Canal criado com sucesso!',
			'settings.channel_deleted_success' => 'Canal excluído',
			'settings.channel_delete_confirm' => ({required Object name}) => 'Deseja excluir "${name}"? As mensagens recebidas por este canal serão interrompidas.',
			'settings.channel_status_connected' => 'Conectado',
			'settings.channel_status_disabled' => 'Desativado',
			'settings.channel_status_incomplete' => 'Incompleto',
			'settings.channel_edit_tooltip' => 'Editar',
			'settings.channel_test_btn' => 'Testar Conexão',
			'settings.channel_testing' => 'Testando...',
			'settings.channel_configure_btn' => 'Configurar',
			'settings.channel_snippet_btn' => 'Snippet',
			'settings.channel_test_success_named' => ({required Object label}) => 'Conectado! ${label}',
			'settings.channel_test_success' => 'Conexão OK!',
			'settings.channel_test_failed' => ({required Object error}) => 'Falha: ${error}',
			'settings.channel_test_unknown_error' => 'Erro desconhecido',
			'settings.channel_reconnect_action' => 'Reconectar',
			'settings.channel_google_connected' => ({required Object address}) => 'Conectado pelo Google: ${address}',
			'settings.channel_google_edit_hint' => 'Esta caixa foi conectada por consentimento — não há servidor nem senha para editar. Se o envio ou a leitura pararem, conecte de novo.',
			'settings.channel_google_reconnect' => 'Reconectar pelo Google',
			'settings.channel_imap_failing' => ({required Object count, required Object error}) => 'A leitura das respostas falhou ${count} vez(es): ${error}',
			'settings.channel_webhook_re_registered' => 'Webhook re-registrado.',
			'settings.channel_webhook_register_failed' => ({required Object error}) => 'Falha ao re-registrar webhook: ${error}',
			'settings.channels_select_telegram_badge' => '2 min',
			'settings.channels_select_popular_badge' => 'Popular',
			'settings.channels_select_coming_soon' => 'Em breve',
			'settings.channel_instagram_help_link' => 'Como obter seu Page ID e token de acesso',
			'settings.channel_sms_help_link' => 'Como obter um número de SMS compatível (guia Twilio)',
			'settings.channel_webhook_url_label' => 'URL de webhook (Twilio)',
			'settings.channel_webhook_url_help' => 'Cole esta URL como webhook de "A message comes in" do seu número no console da Twilio.',
			'settings.channel_webhook_url_copied' => 'URL de webhook copiada',
			'settings.channel_webhook_url_btn' => 'URL de webhook',
			'settings.wa_settings_back_tooltip' => 'Voltar',
			'settings.wa_settings_title' => 'Configurações WhatsApp',
			'settings.wa_settings_tab_profile' => 'Perfil',
			'settings.wa_settings_tab_templates' => 'Templates',
			'settings.wa_section_diagnostics' => 'DIAGNÓSTICO',
			'settings.wa_section_phones' => 'NÚMEROS NO WABA',
			'settings.wa_section_business' => 'SOBRE O NEGÓCIO',
			'settings.wa_field_about' => 'Sobre',
			'settings.wa_field_about_hint' => 'Breve descrição (aparece no perfil)',
			'settings.wa_field_description' => 'Descrição completa',
			'settings.wa_field_description_hint' => 'Descrição detalhada do seu negócio',
			'settings.wa_field_email' => 'Email',
			'settings.wa_field_email_hint' => 'contato@empresa.com',
			'settings.wa_field_address' => 'Endereço',
			'settings.wa_field_address_hint' => 'Rua Exemplo, 123 — Cidade/UF',
			'settings.wa_field_site_one' => 'Site 1',
			'settings.wa_field_site_one_hint' => 'https://...',
			'settings.wa_field_site_two' => 'Site 2',
			'settings.wa_field_site_two_hint' => 'https://... (opcional)',
			'settings.wa_profile_helper' => 'Estes dados aparecem publicamente no perfil do seu WhatsApp Business.',
			'settings.wa_profile_save_btn' => 'Salvar alterações',
			'settings.wa_profile_saving' => 'Salvando...',
			'settings.wa_profile_saved_success' => 'Perfil atualizado com sucesso!',
			'settings.wa_picture_replace' => 'Trocar foto',
			'settings.wa_picture_set' => 'Alterar foto',
			'settings.wa_picture_apply_hint' => 'A foto será aplicada ao clicar em "Salvar alterações"',
			'settings.wa_picture_remove_button' => 'Remover foto cacheada',
			'settings.wa_picture_remove_dialog_title' => 'Remover foto cacheada?',
			'settings.wa_picture_remove_dialog_body' => 'Apaga a cópia local da foto de perfil. A foto no número do WhatsApp em si permanece — para removê-la lá, use o Meta Business Manager. Use isso quando a Meta parar de retornar a foto para o dashboard parar de servir a antiga.',
			'settings.wa_picture_remove_dialog_confirm' => 'Remover',
			'settings.wa_picture_removed_success' => 'Foto cacheada removida.',
			'settings.wa_templates_no_templates_subtitle' => 'Templates são mensagens pré-aprovadas pela Meta. Você precisa deles para iniciar conversas com clientes fora da janela de 24h.',
			'settings.wa_templates_create_loading_error_title' => 'Conexão com a Meta precisa ser renovada',
			'settings.wa_templates_visualize_tooltip' => 'Visualizar',
			'settings.wa_templates_delete_tooltip' => 'Excluir',
			'settings.wa_templates_delete_confirm' => ({required Object name}) => 'Excluir "${name}"? Esta ação também remove o template na Meta.',
			'settings.wa_templates_delete_success' => 'Template excluído',
			'settings.wa_templates_rejected_reason' => ({required Object reason}) => 'Motivo: ${reason}',
			'settings.wa_templates_no_examples_warning' => 'Sem exemplos gravados — as variáveis aparecem como placeholders.',
			'settings.wa_create_dialog_title' => 'Novo template',
			'settings.wa_create_section_name' => '1. NOME TÉCNICO',
			'settings.wa_create_name_hint' => 'ex: boas_vindas',
			'settings.wa_create_name_helper' => 'Apenas letras minúsculas, números e underscore. Não aparece para o cliente.',
			'settings.wa_create_section_category' => '2. CATEGORIA',
			'settings.wa_create_section_language' => '3. IDIOMA',
			'settings.wa_create_section_header' => 'CABEÇALHO (opcional)',
			'settings.wa_create_section_message' => '4. MENSAGEM',
			'settings.wa_create_message_hint' => 'Olá {{1}}, seu pedido #{{2}} foi confirmado! 🎉',
			'settings.wa_create_message_helper' => 'Use {{1}}, {{2}}... para variáveis. Formatação: *negrito*, _itálico_, ~tachado~, `mono` (WhatsApp usa 1 asterisco, não 2).',
			'settings.wa_create_double_asterisk_warn' => 'WhatsApp usa *texto* (um asterisco), não **texto**. Corrija antes de enviar.',
			'settings.wa_create_section_examples' => '5. EXEMPLOS PARA VARIÁVEIS',
			'settings.wa_create_example_for' => ({required Object n}) => 'Exemplo para {{${n}}}',
			'settings.wa_create_example_first' => 'João Silva',
			'settings.wa_create_example_other' => 'Valor',
			'settings.wa_create_examples_helper' => 'A Meta usa estes exemplos para analisar o template.',
			'settings.wa_create_section_footer' => 'RODAPÉ (opcional)',
			'settings.wa_create_footer_hint' => 'ex: Responda PARAR para sair',
			'settings.wa_create_section_buttons' => 'BOTÕES (opcional, máx. 3)',
			'settings.wa_create_section_preview' => 'PRÉ-VISUALIZAÇÃO',
			'settings.wa_create_submit' => 'Enviar para aprovação',
			'settings.wa_create_submitting' => 'Enviando...',
			'settings.wa_create_validation_name' => 'Nome deve conter apenas letras minúsculas, números e _.',
			'settings.wa_create_validation_body_required' => 'Mensagem é obrigatória.',
			'settings.wa_create_validation_double_asterisk' => 'Remova os ** do texto — WhatsApp usa *texto* (um asterisco) para negrito.',
			'settings.wa_create_validation_no_leading_var' => 'Texto não pode começar com variável. Adicione uma palavra antes de {{1}} (ex: "Olá {{1}}").',
			'settings.wa_create_validation_no_trailing_var' => 'Texto não pode terminar com variável. Adicione uma palavra ou pontuação depois da última {{N}}.',
			'settings.wa_create_validation_examples_required' => 'Preencha os exemplos para as variáveis.',
			'settings.wa_create_validation_header_empty' => 'Cabeçalho de texto não pode ficar vazio.',
			'settings.wa_create_validation_header_too_long' => 'Cabeçalho de texto excede 60 caracteres.',
			'settings.wa_create_validation_header_one_var' => 'Cabeçalho aceita no máximo uma variável.',
			'settings.wa_create_validation_header_example' => 'Preencha o exemplo do cabeçalho.',
			'settings.wa_create_validation_media_uploading' => 'Aguarde o upload do cabeçalho terminar.',
			'settings.wa_create_validation_media_required' => 'Selecione e envie um arquivo para o cabeçalho.',
			'settings.wa_create_validation_button_text_required' => 'Todos os botões precisam de texto.',
			'settings.wa_create_validation_button_url' => 'Botões de URL precisam de link.',
			'settings.wa_create_validation_button_phone' => 'Botões de telefone precisam de número E.164 (ex: +5511999999999).',
			'settings.wa_create_warning_promo_for_utility' => 'Texto parece promocional ("novidades", "oferta"...). Meta costuma rejeitar UTILITY com esse conteúdo — troque para MARKETING.',
			'settings.wa_create_warning_tx_for_marketing' => 'Texto parece transacional ("confirmação", "pedido"...). Considere UTILITY para reduzir chance de rejeição.',
			'settings.wa_category_marketing' => 'Promoções e ofertas',
			'settings.wa_category_utility' => 'Confirmações e atualizações',
			'settings.wa_lang_pt_br' => 'Português (Brasil)',
			'settings.wa_lang_en_us' => 'Inglês (EUA)',
			'settings.wa_lang_es' => 'Espanhol',
			'settings.wa_header_none' => 'Sem cabeçalho',
			'settings.wa_header_text' => 'Texto',
			'settings.wa_header_image' => 'Imagem',
			'settings.wa_header_video' => 'Vídeo',
			'settings.wa_header_document' => 'Documento',
			'settings.wa_header_text_hint' => 'ex: Confirmação do pedido {{1}}',
			'settings.wa_header_text_helper' => 'Máx. 60 caracteres. Aceita até uma única variável {{1}}.',
			'settings.wa_header_example_label' => 'Exemplo para {{1}} no cabeçalho',
			'settings.wa_header_example_hint' => '#12345',
			'settings.wa_header_pick_file' => 'Selecionar arquivo',
			'settings.wa_header_uploading' => 'Enviando...',
			'settings.wa_header_remove_tooltip' => 'Remover',
			'settings.wa_header_upload_failed_processing' => 'Falha ao processar o arquivo no Meta. Tente outro.',
			'settings.wa_header_upload_failed_generic' => ({required Object error}) => 'Falha no upload: ${error}',
			'settings.wa_button_text_label' => 'Texto do botão (máx. 25)',
			'settings.wa_button_url_label' => 'URL',
			'settings.wa_button_url_hint' => 'https://...',
			'settings.wa_button_phone_label' => 'Número (E.164)',
			'settings.wa_button_phone_hint' => '+5511999999999',
			'settings.wa_button_call' => 'Ligar para número',
			'settings.wa_preview_message_placeholder' => 'Sua mensagem aparece aqui...',
			'settings.wa_preview_button_placeholder' => 'Texto do botão',
			'settings.wa_template_submit_success' => 'Template enviado! A análise da Meta costuma levar até 24 horas.',
			'settings.wa_test_template_explain' => 'Dispara uma mensagem real para validar a conexão end-to-end. Útil para a gravação de tela do App Review da Meta.',
			'settings.wa_diag_phone_not_registered_no_platform' => 'Meta não confirmou o registro do número. Sem isso, mensagens de entrada não chegam ao Replider.',
			'settings.wa_diag_phone_not_registered_with_platform' => ({required Object platform}) => 'platform_type = ${platform}. Registre o número para receber mensagens.',
			'settings.wa_diag_cooldown_title' => 'Bloqueado',
			'settings.wa_diag_cooldown_snackbar' => ({required Object time}) => 'Aguarde ${time} antes da próxima tentativa. Cada clique renova o bloqueio da Meta.',
			'settings.wa_diag_atualizar_tooltip' => 'Atualizar',
			'settings.csat_auto_title' => 'CSAT automático',
			'settings.csat_auto_subtitle' => 'Perguntar de 1 a 5 após conversa inativa',
			'settings.team_metrics_relative_min' => ({required Object count}) => '${count} min',
			'settings.team_metrics_relative_hours' => ({required Object count}) => '${count} h',
			'settings.team_metrics_relative_days' => ({required Object count}) => '${count} d',
			'settings.team_metrics_relative_weeks' => ({required Object count}) => '${count} sem',
			'settings.csat_auto_helper' => 'Enviado na língua detectada da conversa (pt/en/es). Desligue em fluxos B2B onde o atendente já debrifa manualmente.',
			'settings.reply_reminder_title' => 'Lembrete de resposta',
			'settings.reply_reminder_subtitle' => 'Nunca esqueça um cliente aguardando',
			'settings.reply_reminder_helper' => 'Quando a IA está pausada e o cliente espera mais do que o limite configurado, todo membro da equipe com push ativado recebe alerta no celular e no painel. Escala em 1x, 2x e 4x o limite, depois para.',
			'settings.reply_reminder_threshold_label' => 'Avisar após',
			'settings.claude_title' => 'Claude (IA)',
			'settings.claude_subtitle' => 'Controle sua conta por conversa com o Claude (MCP).',
			'settings.brand_tile' => 'Identidade da empresa',
			'settings.brand_tile_sub' => 'Voz, o que pode e o que nunca pode ser dito',
			'settings.templates_tile' => 'Modelos de e-mail',
			'settings.templates_tile_sub' => 'Textos prontos de e-mail, com quanto cada um responde',
			'settings.suppressions_tile' => 'Endereços bloqueados',
			'settings.suppressions_tile_sub' => 'Quem parou de receber seus e-mails, e por quê',
			'settings.autopilot_tile_sub' => 'Quando abordagens podem sair sem sua aprovação',
			'settings.tokens_conversations_used' => ({required Object tokens, required Object conv}) => 'Tokens: ${tokens} | Conversas: ${conv}',
			'settings.contacts_subtitle' => 'Quem já falou com você, num só lugar',
			'settings.analytics_subtitle' => 'Volume, tempo de resposta e desempenho da IA',
			'settings.preview_handoff_note' => 'O agente passaria a conversa para uma pessoa aqui. Numa conversa de verdade, o bot é pausado e alguém do time assume — no teste não há para quem passar.',
			'settings.channel_test_email_success' => ({required Object to, required Object cap}) => 'Teste enviado para ${to}. Confira a caixa — nenhum cliente foi contatado. Você já pode aprovar até ${cap} abordagens por dia.',
			'settings.channel_test_email_not_unlocked' => ({required Object to}) => 'Teste enviado para ${to}, mas não conseguimos registrar a liberação. Rode o teste de novo.',
			'settings.section_company' => 'Empresa e marca',
			'settings.section_email' => 'E-mail e envio',
			'settings.section_catalog' => 'Catálogo e funis',
			'settings.section_documents' => 'Documentos',
			'settings.section_integrations' => 'Integrações',
			'settings.section_contract' => 'Contratar ou ampliar',
			'settings.section_support_bot' => 'Atendimento automático',
			'settings.funnels_tile_sub' => 'Etapas, os dados a coletar e o webhook',
			'settings.section_advanced' => 'Avançado',
			'settings.section_advanced_sub' => 'Funis e datas de relacionamento. Já vêm prontos; abra só se quiser mudar.',
			'settings.relationship_tile_sub' => 'Aniversários, datas da empresa e do setor que viram rascunho no dia',
			'settings.support_bot_tile_sub' => 'Agentes, prompts, flows, tags e as métricas do bot',
			'settings.support_bot_intro' => 'O lado chatbot da Replider. Se você veio para vender, nada disso é necessário — as ferramentas de venda ficam na tela anterior.',
			'settings.email_warmup_not_started' => 'Aquecimento não iniciado — verifique a caixa (DNS ou reconexão) para liberar a prospecção',
			'settings.email_warmup_progress' => ({required Object day, required Object total, required Object sent, required Object cap}) => 'Aquecimento: dia ${day} de ${total} · ${sent} de ${cap} envios hoje',
			'settings.email_warmup_done' => ({required Object cap}) => 'Aquecida — até ${cap} envios por dia',
			'settings.channel_name_required' => 'Dê um nome ao canal',
			'settings.channel_required_missing' => ({required Object fields}) => 'Preencha: ${fields}',
			'settings.channel_email_help_link' => 'Como configurar o e-mail do seu domínio',
			'settings.usage_costs_title' => 'Uso e custos de IA',
			'settings.usage_costs_subtitle' => 'Uso do mês, gasto de IA e seu teto de orçamento',
			'common.cancel' => 'Cancelar',
			'common.save' => 'Salvar',
			'common.create' => 'Criar',
			'common.delete' => 'Excluir',
			'common.edit' => 'Editar',
			'common.retry' => 'Tentar novamente',
			'common.subscribe' => 'Assinar',
			'common.per_month' => '/mês',
			'common.free' => 'Grátis',
			'common.success' => 'Sucesso',
			'common.confirm_delete' => 'Tem certeza que deseja excluir?',
			'common.no_results' => 'Nenhum resultado encontrado.',
			'common.confirm' => 'Confirmar',
			'common.remove' => 'Remover',
			'common.close' => 'Fechar',
			'common.add' => 'Adicionar',
			'common.search' => 'Buscar',
			'common.copy' => 'Copiar',
			'common.change' => 'Alterar',
			'common.load_more' => 'Carregar mais',
			'common.error_generic' => ({required Object error}) => 'Erro: ${error}',
			'common.back' => 'Voltar',
			'common.saving' => 'Salvando...',
			'common.sending' => 'Enviando...',
			'common.sent' => 'Enviado',
			'common.loading' => 'Carregando...',
			'common.ok' => 'OK',
			'common.open' => 'Abrir',
			'common.send' => 'Enviar',
			'common.discard' => 'Descartar',
			'common.keep_editing' => 'Continuar editando',
			'common.discard_changes_title' => 'Descartar alterações?',
			'common.discard_changes_body' => 'Você fez mudanças que ainda não foram salvas. Sair agora as perde.',
			'common.yes' => 'Sim',
			'common.no' => 'Não',
			'common.logout' => 'Sair',
			'common.refresh' => 'Atualizar',
			'common.select' => 'Selecionar',
			'common.name_label' => 'Nome',
			'common.color' => 'Cor',
			'common.new_short' => 'Nova',
			'common.unreadable_content' => 'Conteúdo ilegível',
			'common.load_failed_tap' => 'Não foi possível carregar. Toque para tentar de novo.',
			'billing.title' => 'Uso e custos de IA',
			'billing.checkout_error' => 'Falha ao iniciar o checkout.',
			'billing.manage_payment_title' => 'Gerenciar pagamento',
			'billing.manage_payment_subtitle' => 'Abra o portal do Stripe pra atualizar o cartão, baixar notas ou cancelar.',
			'billing.manage_payment_cta' => 'Abrir portal',
			'billing.portal_opening' => 'Abrindo portal...',
			'billing.upgrade_modal_title' => 'Escolha seu plano',
			'billing.upgrade_modal_subtitle' => 'Assine direto por aqui. O pagamento é seguro via Stripe.',
			'billing.upgrade_current_plan_badge' => 'Plano atual',
			'billing.upgrade_popular_badge' => 'Mais popular',
			'billing.upgrade_cta' => 'Assinar',
			'billing.upgrade_cta_contact_sales' => 'Falar com vendas',
			'billing.contract_badge' => 'Plano sob contrato',
			'billing.contract_subtitle' => 'Limites customizados + você usa sua própria chave de API. Fale com um especialista para combinar valor e ativação.',
			'billing.contract_email_cta' => 'E-mail',
			'billing.contract_whatsapp_cta' => 'WhatsApp',
			'billing.contract_email_subject' => ({required Object plan}) => 'Plano ${plan} do Replider — interesse',
			'billing.contract_link_failed' => 'Não foi possível abrir o link. Envie e-mail para support@replider.com.',
			'billing.upgrade_per_month' => '/mês',
			'billing.upgrade_interval_monthly' => 'Mensal',
			'billing.upgrade_interval_annual' => 'Anual',
			'billing.upgrade_annual_save' => 'Economize 20%',
			'billing.upgrade_billed_annually' => 'cobrado anualmente',
			'billing.upgrade_compare_current' => 'Atual',
			'billing.upgrade_compare_new' => 'Novo',
			'billing.upgrade_compare_conversations' => 'Conversas/mês',
			'billing.upgrade_compare_agents' => 'Agentes de IA',
			'billing.upgrade_compare_channels' => 'Canais',
			'billing.upgrade_compare_tokens' => 'Tokens de IA/mês',
			'billing.upgrade_checkout_opening' => 'Abrindo checkout...',
			'billing.upgrade_feature_agents' => ({required Object count}) => '${count} agente(s) IA',
			'billing.upgrade_feature_channels' => ({required Object count}) => '${count} canal(is)',
			'billing.upgrade_feature_conversations' => ({required Object count}) => '${count} conversas/mês',
			'billing.upgrade_feature_tokens' => ({required Object tokens}) => '${tokens} tokens IA/mês',
			'billing.billing_pill_trial_days' => ({required Object days}) => 'Trial: ${days} dias',
			'billing.billing_pill_trial_last_day' => 'Trial acaba hoje',
			'billing.billing_pill_trial_expired' => 'Trial expirado',
			'billing.billing_pill_past_due' => 'Pagamento pendente',
			'billing.billing_pill_manage' => 'Gerenciar',
			'billing.no_customer_yet' => 'Conclua a primeira assinatura antes de abrir o portal de pagamento.',
			'billing.checkout_open_failed' => 'Falha ao iniciar o checkout.',
			'billing.addon_buy_failed' => 'Falha ao comprar o pack.',
			'billing.portal_open_failed' => 'Falha ao abrir o portal de pagamento.',
			'billing.quota_conversations_label' => 'Conversas',
			'billing.quota_tokens_label' => 'Créditos IA',
			'billing.quota_agents_label' => 'Agentes IA',
			'billing.quota_wall_title' => 'Limite de IA atingido',
			'billing.quota_wall_body' => 'Seus créditos de IA deste ciclo acabaram. O agente parou de responder automaticamente até você liberar mais créditos ou o ciclo resetar.',
			'billing.quota_wall_cta_later' => 'Esperar reset do mês',
			'billing.quota_wall_reset_hint' => ({required Object day}) => 'O ciclo reinicia dia ${day} do próximo mês.',
			'billing.back_tooltip' => 'Voltar',
			'billing.section_usage' => 'USO ATUAL',
			'billing.section_addon_packs' => 'PACOTES EXTRAS',
			'billing.section_plans' => 'PLANOS DISPONIVEIS',
			'billing.addon_packs_unavailable' => 'Pacotes extras indisponiveis.',
			'billing.load_usage_failed' => 'Erro ao carregar uso',
			'billing.load_plans_failed' => 'Erro ao carregar planos',
			'billing.trial_view_plans' => 'Ver planos',
			'billing.usage_ai_tokens' => 'Tokens de IA',
			'billing.usage_includes_addon_tokens' => ({required Object count}) => 'Inclui ${count} tokens extras',
			'billing.usage_input' => 'Input',
			'billing.usage_output' => 'Output',
			'billing.usage_conversations' => 'Conversas',
			'billing.overage_active_title' => 'Pagando por conversas extras',
			'billing.overage_active_body' => ({required Object count, required Object amount}) => '${count} conversas além do plano · ${amount} cobrado neste ciclo',
			'billing.overage_rate_note' => ({required Object rate}) => '${rate} por conversa extra',
			'billing.overage_cap_reached' => 'Limite de excedente atingido — conversas extras pausadas até o próximo ciclo.',
			'billing.plan_free' => 'Grátis',
			'billing.plan_per_month_short' => '/mês',
			'billing.plan_feature_agents' => ({required Object count}) => '${count} agente(s) de IA',
			'billing.plan_feature_channels' => ({required Object count}) => '${count} canal(is) de atendimento',
			'billing.plan_feature_conversations' => ({required Object count}) => '${count} conversas/mês',
			'billing.plan_feature_tokens' => ({required Object tokens}) => '${tokens} tokens IA/mês',
			'billing.plan_feature_storage' => ({required Object mb}) => '${mb} MB de armazenamento',
			'billing.plan_feature_storage_gb' => ({required Object gb}) => '${gb} GB de armazenamento',
			'billing.plan_feature_seats' => ({required Object count}) => '${count} assento(s) de atendente',
			'billing.plan_feature_broadcast' => ({required Object count}) => '${count} disparos/mês',
			'billing.plan_feature_recipients' => ({required Object count}) => '${count} destinatários por campanha',
			'billing.plan_feature_overage' => ({required Object cents, required Object cap}) => 'Conversas extras: ${cents}¢ cada (limite ${cap}× do preço do plano)',
			'billing.effective_limits_title' => ({required Object plan}) => 'Seus limites do contrato ${plan}',
			'billing.effective_limits_subtitle' => 'Resolvidos a partir dos overrides do contrato — cobrança off-platform.',
			'billing.byok_label' => 'Use sua chave de IA (BYOK)',
			'billing.byok_configured' => ({required Object providers}) => 'Configurada: ${providers}',
			'billing.byok_pending' => 'Não configurada — o agente não responde até você cadastrar a chave.',
			'billing.byok_manage_cta' => 'Gerenciar',
			'billing.cancel_subscription' => 'Cancelar assinatura',
			'billing.plan_contract' => 'Sob consulta',
			'billing.plan_feature_leads' => ({required Object count}) => '${count} leads/mês',
			'billing.plan_feature_enrichments' => ({required Object count}) => '${count} pesquisas de empresa/mês',
			'billing.plan_feature_drafts' => ({required Object count}) => '${count} rascunhos de IA/mês',
			'billing.plan_feature_funnels' => ({required Object count}) => '${count} funil(is)',
			'billing.plan_feature_mailboxes' => ({required Object count}) => '${count} caixa(s) de e-mail',
			'billing.usage_sdr_title' => 'SEU MÊS',
			'billing.usage_sdr_leads' => 'Leads no funil',
			'billing.usage_sdr_enrichments' => 'Pesquisas de empresa',
			'billing.usage_sdr_approaches' => 'Abordagens enviadas',
			'billing.usage_sdr_meetings' => 'Reuniões marcadas',
			'billing.usage_sdr_of_limit' => ({required Object used, required Object limit}) => '${used} de ${limit}',
			'billing.usage_sdr_no_limit' => ({required Object used}) => '${used} este mês',
			'billing.talk_title' => 'Quer contratar ou ampliar?',
			'billing.talk_body' => 'A gente monta a proposta na conversa, do tamanho da sua operação. Sem tabela de preço: cada empresa vende de um jeito.',
			'billing.talk_book_cta' => 'Agendar conversa',
			'billing.talk_email_subject' => 'Quero conhecer o Replider',
			'billing.ai_cost_title' => 'Custo de IA (sua chave)',
			'billing.ai_cost_cycle' => 'Neste ciclo',
			'billing.ai_cost_projection' => 'Ritmo mensal',
			'billing.ai_cost_budget' => 'Seu teto',
			'billing.ai_cost_no_budget' => 'sem teto',
			'billing.ai_cost_set_budget' => 'Definir teto mensal',
			'billing.ai_cost_budget_dialog_title' => 'Teto mensal de IA',
			'billing.ai_cost_budget_help' => 'Corte duro: quando o gasto chegar nesse valor, a geração de IA pausa até o ciclo reiniciar. 0 remove o teto.',
			'team.title' => 'Equipe',
			'team.invite_member' => 'Convidar Membro',
			'team.member_invited' => 'Membro convidado com sucesso.',
			'team.member_removed' => 'Membro removido.',
			'team.role_updated' => 'Função atualizada.',
			'team.change_role' => 'Alterar função',
			'team.remove_member' => 'Remover membro',
			'team.inactive' => 'Inativo',
			'team.pending_badge' => 'Pendente',
			'team.resend_invite' => 'Reenviar convite',
			'team.invite_resent' => 'Convite reenviado.',
			'team.active_now' => 'Ativo agora',
			'team.last_seen_ago' => ({required Object when}) => 'Visto há ${when}',
			'team.invite_short' => 'Convidar',
			'team.empty_subtitle' => 'Nenhum membro na equipe',
			'team.role_label' => 'Cargo',
			'team.role_admin' => 'Admin',
			'team.role_attendant' => 'Atendente',
			'team.change_role_title' => 'Alterar Cargo',
			'team.change_role_body' => ({required Object name, required Object role}) => 'Deseja alterar o cargo de "${name}" para ${role}?',
			'team.remove_title' => 'Remover Membro',
			'team.remove_body' => ({required Object name}) => 'Deseja realmente remover "${name}" da equipe? Esta pessoa perderá acesso ao sistema.',
			'team.ago_minutes' => ({required Object count}) => '${count} min',
			'team.ago_hours' => ({required Object count}) => '${count}h',
			'team.ago_days' => ({required Object count}) => '${count} dias',
			'team.ago_months' => ({required Object count}) => '${count} meses',
			'team.role_owner' => 'Dono',
			'team.role_gestor' => 'Gestor',
			'team.role_vendedor' => 'Vendedor',
			'team.role_vendedor_hint' => 'Trabalha os próprios leads e ofertas; não abre o inbox geral',
			'team.role_gestor_hint' => 'Acompanha o time e o funil; sem cobrança e integrações',
			'team.role_attendant_hint' => 'Atende as conversas do inbox; não vê o pipeline',
			'team.role_admin_hint' => 'Tudo: configurações, equipe, canais e cobrança',
			'team.invite_fields_required' => 'Preencha nome e e-mail',
			'team.invite_email_invalid' => 'E-mail inválido — confira o endereço',
			'channels.title' => 'Canais',
			'channels.connect_description' => 'Conecte WhatsApp, Instagram ou Telegram para receber mensagens de clientes.',
			'channels.channel_created' => 'Canal criado com sucesso.',
			'channels.channel_deleted' => 'Canal excluído.',
			'channels.webhook_down' => 'Webhook fora',
			'channels.test_connection' => 'Testar conexão',
			'channels.delete_channel' => 'Excluir canal',
			'channels.type_webchat_label' => 'Site (Webchat)',
			'channels.setup_whatsapp' => 'Informe o Phone Number ID do seu número no painel Meta for Developers. O token de acesso é gerenciado automaticamente pela Replider.',
			'channels.setup_telegram' => 'Crie um bot no @BotFather no Telegram e cole o token aqui. O webhook será configurado automaticamente.',
			'channels.setup_instagram' => 'Conecte sua conta profissional do Instagram (Business ou Creator). Informe o ID da conta e um token de acesso com a permissão instagram_business_manage_messages. Apenas mensagens diretas.',
			'channels.setup_webchat' => 'Atendimento via chat no seu site. Após criar, copie o snippet e cole no HTML do seu site.',
			'channels.setup_sms' => 'Conecte seu próprio número Twilio para conversas de SMS bidirecionais. Cole o Account SID, o Auth Token e o número de telefone (E.164) do console da Twilio. Depois de conectar, copie a URL de webhook do canal e configure-a como webhook de "A message comes in" do seu número na Twilio.',
			'channels.embedded_signup_missing_config' => 'Configuração incompleta no build. Defina WHATSAPP_APP_ID e WHATSAPP_EMBEDDED_CONFIG_ID via --dart-define.',
			'channels.embedded_signup_unknown_failure' => 'Falha desconhecida no signup.',
			'channels.embedded_signup_complete_failed' => ({required Object error}) => 'Erro ao concluir conexão: ${error}',
			'channels.embedded_signup_success' => 'WhatsApp conectado com sucesso!',
			'channels.embedded_signup_connecting' => 'Conectando...',
			'channels.embedded_signup_cta' => 'Conectar WhatsApp via Meta',
			'channels.embedded_signup_missing_env' => 'WHATSAPP_APP_ID ou WHATSAPP_EMBEDDED_CONFIG_ID não configurado no build.',
			'channels.embedded_signup_no_response' => 'Nenhuma resposta do Facebook SDK.',
			'channels.embedded_signup_user_cancelled' => 'Login cancelado pelo usuário.',
			'channels.embedded_signup_no_code' => ({required Object status}) => 'Facebook não retornou código de autorização (status=${status}).',
			'channels.embedded_signup_failure' => ({required Object error}) => 'Falha no Embedded Signup: ${error}',
			'channels.setup_email' => 'Envie e receba e-mails pelo seu próprio domínio. Você precisa do SMTP do seu provedor e de publicar SPF, DKIM e DMARC no DNS — sem isso o Gmail e o Outlook descartam a mensagem sem avisar.',
			'channels.email_dns_verified' => 'Domínio autenticado — pronto para enviar.',
			'channels.email_dns_pending' => 'Falta autenticar o domínio. Sem isso, os provedores dos seus clientes (Gmail, Outlook e outros) descartam suas mensagens.',
			'channels.email_dns_recheck' => 'Verificar de novo',
			'channels.email_dns_optional' => 'opcional',
			'channels.email_dns_host' => 'Nome',
			'channels.email_dns_value' => 'Valor',
			'channels.email_dns_found' => 'Publicado atualmente',
			_ => null,
		} ?? switch (path) {
			'channels.email_dns_copied' => 'Copiado.',
			'channels.email_dns_provider_managed' => 'O Google autentica os envios deste endereço — nada para publicar.',
			'channels.email_dns_already_published' => 'Já publicado — nada a fazer aqui.',
			'channels.email_dns_detected' => ({required Object provider}) => 'DNS na ${provider}',
			'channels.email_dns_open_guide' => 'Abrir guia',
			'channels.email_dns_auto_configure' => 'Configurar automaticamente',
			'channels.email_dns_token_help' => ({required Object provider}) => 'Na ${provider}, crie um token de API com permissão Zone → DNS → Edit para este domínio e cole aqui. Usamos uma vez para publicar os registros e não guardamos em lugar nenhum.',
			'channels.email_dns_token_hint' => 'Cole o token',
			'channels.email_dns_auto_done' => 'Registros publicados',
			'channels.email_dns_auto_partial' => 'Publicado em parte',
			'channels.dns_action_created' => 'criado',
			'channels.dns_action_updated' => 'atualizado',
			'channels.dns_action_merged' => 'unido ao registro que já existia',
			'channels.dns_action_kept' => 'o seu foi mantido, sem alteração',
			'channels.dns_action_failed' => 'falhou',
			'channels.email_dns_need_help' => 'Precisa de ajuda?',
			'channels.email_dns_provider_unknown' => 'não detectado',
			'channels.email_dns_help_draft' => ({required Object domain, required Object provider, required Object records}) => 'Preciso de ajuda para configurar o domínio de e-mail.\n\nDomínio: ${domain}\nProvedor de DNS: ${provider}\nRegistros faltando: ${records}',
			'channels.webhook_health_unknown' => 'Saúde não verificada',
			'channels.gmail_connect_title' => 'Conectar a conta que você já usa',
			'channels.gmail_connect_help' => 'Um clique, sem servidor nem senha para digitar. Funciona com Gmail e Google Workspace, e as respostas continuam caindo na sua caixa de sempre.',
			'channels.gmail_connect_cta' => 'Conectar com o Google',
			'channels.gmail_connected' => 'E-mail conectado.',
			'channels.gmail_reconnected' => 'Conexão de e-mail renovada.',
			'channels.gmail_denied' => 'Conexão cancelada. Nada foi alterado.',
			'channels.gmail_failed' => 'Não consegui concluir a conexão. Tente de novo.',
			'channels.gmail_or_smtp' => 'ou configure o SMTP manualmente',
			'channels.gmail_already_connected' => ({required Object name}) => 'Caixa conectada: ${name}',
			'channels.gmail_connect_check' => 'Já autorizei — verificar',
			'channels.email_dns_token_title_named' => ({required Object provider}) => 'Token de API da ${provider}',
			'channels.email_field_from' => 'E-mail de envio',
			'channels.email_field_smtp_host' => 'Servidor SMTP',
			'channels.email_field_smtp_port' => 'Porta',
			'channels.email_field_smtp_user' => 'Usuário SMTP',
			'channels.email_field_smtp_password' => 'Senha SMTP',
			'channels.email_field_imap_host' => 'Servidor IMAP (opcional)',
			'channels.email_field_imap_hint' => 'igual ao SMTP se vazio',
			'channels.email_field_imap_port' => 'Porta IMAP (opcional)',
			'channels.email_dns_generic_steps' => 'Não reconhecemos automaticamente o seu provedor de DNS. Sem problema: entre no painel da empresa onde o domínio é administrado (quem vendeu o domínio ou hospeda o site — Locaweb, KingHost, GoDaddy, etc.), procure por "DNS", "Zona DNS" ou "Gerenciar registros" e crie os registros abaixo.',
			'channels.email_dns_how_to' => 'Crie cada registro abaixo na zona DNS do domínio — os botões copiam nome e valor. Depois de salvar, a propagação leva de minutos a algumas horas; esta tela verifica sozinha.',
			'channels.fb_signup_mobile_only' => 'O fluxo Embedded Signup só funciona no navegador. No mobile, use o formulário manual em "Adicionar canal".',
			'channels.email_dns_poll_stopped' => 'Paramos de verificar sozinhos — depois de ~10 minutos a causa comum é registro errado, não propagação. Corrija e toque em atualizar para checar de novo.',
			'whatsapp.profile_load_error' => 'Erro ao carregar perfil',
			'whatsapp.segment' => 'Segmento',
			'whatsapp.new_template' => 'Novo template',
			'whatsapp.reconnect_meta' => 'Reconectar via Meta',
			'whatsapp.reconnect_meta_help' => 'Reautorize a conexão com a Meta pra renovar o token da API. Leva uns 60 segundos — você continua nessa tela.',
			'whatsapp.reconnect_success' => 'Conexão renovada.',
			'whatsapp.no_templates' => 'Nenhum template ainda',
			'whatsapp.create_first_template' => 'Criar primeiro template',
			'whatsapp.delete_template' => 'Excluir template',
			'whatsapp.add_button' => 'Adicionar botão',
			'whatsapp.quick_reply' => 'Resposta rápida',
			'whatsapp.open_url' => 'Abrir URL',
			'whatsapp.template_sent_success' => ({required Object phone}) => 'Template enviado para ${phone}',
			'whatsapp.send_template_title' => 'Enviar template',
			'whatsapp.send_template_to_with_name' => ({required Object name, required Object phone}) => 'Para ${name} (+${phone})',
			'whatsapp.send_template_to' => ({required Object phone}) => 'Para +${phone}',
			'whatsapp.approved_template_label' => 'Template aprovado',
			'whatsapp.choose_approved_template' => 'Escolha um template aprovado.',
			'whatsapp.no_approved_templates_hint' => 'Nenhum template aprovado neste canal. Crie um na aba Templates e aguarde a aprovação da Meta.',
			'whatsapp.loading_templates_error' => ({required Object error}) => 'Erro ao carregar templates: ${error}',
			'whatsapp.sending' => 'Enviando...',
			'whatsapp.send' => 'Enviar',
			'whatsapp.template_status_approved' => 'Aprovado',
			'whatsapp.template_status_pending' => 'Em análise',
			'whatsapp.template_status_rejected' => 'Rejeitado',
			'whatsapp.template_status_paused' => 'Pausado',
			'whatsapp.template_status_disabled' => 'Desativado',
			'whatsapp.category_authentication' => 'Autenticação',
			'whatsapp.language_pt_br' => 'Português (BR)',
			'whatsapp.language_en_us' => 'Inglês (US)',
			'whatsapp.language_es' => 'Espanhol',
			'whatsapp.error_token_expired' => 'O token do WhatsApp expirou. Reconecte pela Meta para restaurar o acesso.',
			'whatsapp.error_outside_window' => 'Janela de 24h expirou. Use um template aprovado para reabrir a conversa.',
			'whatsapp.error_missing_permission' => 'A Meta negou a permissão. Verifique se o app tem whatsapp_business_management habilitado.',
			'whatsapp.error_recipient' => ({required Object message}) => 'Destinatário inválido ou não registrado no WhatsApp: ${message}',
			'whatsapp.error_template' => ({required Object message}) => 'Erro no template: ${message}',
			'whatsapp.error_missing_waba' => 'Este canal não tem WABA ID. Reconecte via Meta Embedded Signup.',
			'whatsapp.error_missing_token' => 'Este canal não tem access token. Reconecte via Meta Embedded Signup.',
			'whatsapp.error_register_rate_limited' => 'A Meta bloqueou tentativas de registro neste número por excesso de chamadas. Aguarde ~24h antes de tentar de novo. Enquanto isso, confira se o WABA tem método de pagamento e se a verificação em duas etapas está desligada.',
			'whatsapp.error_pin_mismatch' => 'PIN incorreto. Use a aba "Esqueci o PIN" para receber um código por SMS.',
			'whatsapp.error_already_verified' => 'Este número já está verificado em outro app Cloud API. Use "Liberar número" para soltá-lo antes de registrar aqui.',
			'whatsapp.error_cert_2fa_blocked' => 'A Meta bloqueou o registro porque a verificação em duas etapas (2FA) do WABA está ativa. Desligue em Meta Business → Configurações da conta → Verificação em duas etapas e tente de novo.',
			'whatsapp.error_ownership_not_verified' => 'A Meta ainda não verificou a posse deste número. Use a aba "Esqueci o PIN" para receber um código por SMS ou voz e finalizar o registro em um único passo.',
			'whatsapp.error_verify_code_invalid' => 'Código incorreto, expirado ou já usado. Clique em "Enviar código" novamente para receber um novo (códigos da Meta expiram em ~10 minutos).',
			'whatsapp.error_register_failed' => ({required Object detail}) => 'Não foi possível registrar o número no Cloud API. ${detail}',
			'whatsapp.error_deregister_failed' => ({required Object detail}) => 'Falha ao liberar o número. ${detail}',
			'whatsapp.error_request_code_failed' => ({required Object detail}) => 'A Meta não conseguiu enviar o código de verificação. ${detail} Tente novamente em 1 minuto, ou alterne entre SMS e ligação.',
			'whatsapp.error_verify_code_failed' => 'Código inválido ou expirado. Solicite um novo e tente de novo.',
			'whatsapp.error_subscribe_failed' => ({required Object detail}) => 'Não foi possível inscrever a Replider para receber webhooks. ${detail}',
			'whatsapp.error_webhook_status_failed' => ({required Object detail}) => 'Não foi possível verificar o status do webhook. ${detail}',
			'whatsapp.error_token_exchange_failed' => 'A Meta recusou a troca de token durante a conexão. Tente o Embedded Signup novamente.',
			'whatsapp.error_template_create_failed' => ({required Object detail}) => 'A Meta rejeitou a criação do template. ${detail}',
			'whatsapp.error_template_delete_failed' => ({required Object detail}) => 'A Meta rejeitou a exclusão do template. ${detail}',
			'whatsapp.error_profile_fetch_failed' => 'Não foi possível carregar o perfil do WhatsApp Business. Tente recarregar.',
			'whatsapp.error_profile_update_failed' => ({required Object detail}) => 'Não foi possível salvar o perfil do WhatsApp Business. ${detail}',
			'whatsapp.error_picture_upload_failed' => 'Falha ao enviar a foto de perfil. Verifique se é JPG/PNG abaixo de 5MB.',
			'whatsapp.error_phone_owned_elsewhere' => 'Este número já está conectado a outra conta Replider. Solicite a liberação para o suporte.',
			'whatsapp.error_missing_creds' => 'Canal sem credenciais válidas. Reconecte pelo Embedded Signup.',
			'whatsapp.error_not_supported' => 'Operação não suportada para este tipo de canal.',
			'whatsapp.error_trial_expired' => 'Seu período de testes terminou. Faça upgrade do plano para continuar usando o WhatsApp.',
			'whatsapp.error_subscription_inactive' => 'Sua assinatura não está ativa. Atualize o método de pagamento para reativar.',
			'whatsapp.error_meta_code' => ({required Object code}) => 'Código Meta ${code}.',
			'whatsapp.error_unknown_with_code' => ({required Object code}) => 'Erro desconhecido (${code}).',
			'whatsapp.error_invalid_param_duplicate_name' => 'Já existe um template com esse nome e idioma. Escolha outro nome ou apague o existente antes de reenviar.',
			'whatsapp.error_invalid_param_paused' => 'A criação de templates está pausada para esta conta. Aguarde algumas horas ou contate o suporte da Meta.',
			'whatsapp.error_invalid_param_language' => 'Este idioma ainda não está habilitado para templates nesta conta. Tente pt_BR ou en_US.',
			'whatsapp.error_invalid_param_category_mismatch' => 'Meta rejeitou o template. Provavelmente o texto não combina com a categoria escolhida: MARKETING para promoções/novidades, UTILITY só para confirmações/avisos transacionais, AUTHENTICATION só para códigos de verificação. Revise também variáveis {{1}} {{2}} e exemplos.',
			'whatsapp.error_invalid_param_default' => ({required Object message, required Object trace}) => 'A Meta rejeitou os parâmetros: ${message}${trace}',
			'whatsapp.template_preview_label' => 'COMO O CLIENTE VAI RECEBER',
			'whatsapp.create_template_link' => '+ Criar novo template',
			'whatsapp.create_template_title' => 'Criar template',
			'whatsapp.create_template_approval_notice' => 'A Meta precisa aprovar todo template — costuma levar de minutos a 1 dia. Ele não serve para AGORA, mas fica disponível aqui para todas as próximas conversas.',
			'whatsapp.suggestion_reopen' => 'Retomar conversa',
			'whatsapp.suggestion_followup' => 'Follow-up de proposta',
			'whatsapp.suggestion_blank' => 'Do zero',
			'whatsapp.create_template_name_label' => 'Nome',
			'whatsapp.create_template_name_helper' => 'só minúsculas, números e _',
			'whatsapp.create_template_body_label' => 'Corpo',
			'whatsapp.create_template_body_helper' => '{{1}}, {{2}} viram campos na hora do envio. Não pode começar nem terminar com variável.',
			'whatsapp.create_template_missing_fields' => 'Preencha o nome e o corpo.',
			'whatsapp.create_template_submitted' => 'Template enviado — aparece na lista assim que a Meta aprovar.',
			'whatsapp.create_template_submit' => 'Enviar para aprovação',
			'notifications.title' => 'Notificações',
			'notifications.subtitle_default' => 'Receba alertas mesmo com a aba em segundo plano',
			'notifications.subtitle_subscribed' => 'Você recebe um alerta quando um cliente escreve',
			'notifications.subtitle_denied' => 'Permissão bloqueada. Libere em Configurações do site > Notificações',
			'notifications.subtitle_unsupported' => 'Navegador não suporta notificações push',
			'notifications.subtitle_ios_install' => 'No iPhone, instale o app na tela de início para receber notificações',
			'notifications.error_loading' => ({required Object error}) => 'Não foi possível carregar o estado das notificações: ${error}',
			'notifications.test_btn' => 'Enviar teste',
			'notifications.test_no_subs' => 'Sem inscrição ativa neste dispositivo. Ative as notificações primeiro.',
			'notifications.test_success' => ({required Object ok, required Object failed}) => 'Teste enviado: ${ok} entregues, ${failed} com falha.',
			'notifications.test_all_failed' => ({required Object count}) => 'Todas as ${count} inscrições falharam. Reative as notificações para renovar as chaves.',
			'notifications.test_disabled_some' => ({required Object count}) => '${count} inscrição(ões) desativada(s) — provável incompatibilidade de chave VAPID. Reative em cada dispositivo.',
			'notifications.test_request_failed' => ({required Object error}) => 'Falha na requisição de teste: ${error}',
			'api_keys.title' => 'Chaves dos provedores de IA',
			'api_keys.subtitle' => 'Seu plano sob contrato usa suas próprias chaves de API. O Replider guarda criptografado e nunca compartilha.',
			'api_keys.not_configured' => 'Não configurada',
			'api_keys.input_hint' => 'Cole a chave de API',
			'api_keys.add' => 'Adicionar',
			'api_keys.edit' => 'Alterar',
			'api_keys.save' => 'Salvar',
			'api_keys.clear' => 'Remover',
			'api_keys.saved' => 'Chave atualizada.',
			'api_keys.save_failed' => ({required Object error}) => 'Não foi possível salvar a chave: ${error}',
			'notes.add_error' => 'Erro ao adicionar nota',
			'notes.delete_error' => 'Erro ao deletar nota',
			'notes.internal_notes' => 'Notas internas',
			'notes.no_notes' => 'Sem notas ainda',
			'notes.internal_note' => 'Nota interna',
			'notes.load_error' => 'Erro ao carregar notas',
			'notes.add_hint' => 'Adicionar nota...',
			'notes.delete_confirm_title' => 'Excluir nota?',
			'notes.delete_confirm_body' => 'Esta nota interna será removida para o time inteiro.',
			'message_actions.copy_text' => 'Copiar texto',
			'message_actions.message_copied' => 'Mensagem copiada',
			'message_actions.copy_message_id' => 'Copiar ID da mensagem',
			'message_actions.id_copied' => 'ID copiado',
			'contact_sheet.tab_info' => 'Informações',
			'contact_sheet.tab_files' => 'Arquivos',
			'contact_sheet.stat_conversations' => 'Conversas',
			'contact_sheet.stat_messages' => 'Mensagens',
			'contact_sheet.stat_customer_since' => 'Cliente desde',
			'contact_sheet.files_section_images' => 'Imagens e vídeos',
			'contact_sheet.files_section_documents' => 'Documentos e áudios',
			'contact_sheet.files_empty' => 'Nenhuma mídia foi trocada nesta conversa ainda.',
			'contact_sheet.files_error' => 'Não foi possível carregar os arquivos.',
			'contact_sheet.files_unnamed' => 'Arquivo sem nome',
			'contact_sheet.conversation_section' => 'Conversa',
			'contact_sheet.footer_stats' => ({required Object convs, required Object msgs, required Object since}) => '${convs} conversas · ${msgs} mensagens · primeiro contato ${since}',
			'contact_sheet.files_section_attachments' => 'Anexos de e-mail',
			'contact_sheet.files_email_no_subject' => '(sem assunto)',
			'conversation.send_failed_recovered' => 'Falha ao enviar. Mensagem recuperada — tente novamente.',
			'conversation.message_send_failed' => 'Não entregue',
			'conversation.resume_bot' => 'Retomar Bot',
			'conversation.file_read_error' => 'Não foi possível ler o arquivo.',
			'conversation.file_send_error' => ({required Object error}) => 'Erro ao enviar arquivo: ${error}',
			'conversation.nobody' => 'Ninguém',
			'conversation.assume' => 'Assumir conversa',
			'conversation.release' => 'Devolver ao bot',
			'conversation.under_takeover_by' => ({required Object name}) => 'Em atendimento por ${name}',
			'conversation.under_takeover_anonymous' => 'Em atendimento humano',
			'conversation.input_disabled_tooltip' => 'Assuma a conversa para responder manualmente',
			'conversation.resolve_action' => 'Resolver',
			'conversation.reopen_action' => 'Reabrir',
			'conversation.load_older_messages' => 'Carregar mensagens anteriores',
			'conversation.resolve_confirm_title' => 'Resolver conversa?',
			'conversation.resolve_confirm_body' => 'A conversa vai para "Encerradas". Você pode reabrir depois pelo filtro da lista.',
			'conversation.convert_lead_action' => 'Converter lead',
			'conversation.convert_lead_title' => 'Converter lead em cliente',
			'conversation.convert_lead_help' => 'Vincula uma conta cliente ao atendente atribuído desta conversa como vendedor (lead inbound, trilho de 20%).',
			'conversation.convert_lead_search_hint' => 'Nome da conta ou email',
			'conversation.convert_lead_search_min' => 'Digite pelo menos 2 caracteres para buscar.',
			'conversation.convert_lead_no_results' => 'Nenhuma conta corresponde à busca.',
			'conversation.convert_lead_already_linked' => 'Já vinculada a um vendedor',
			'conversation.convert_lead_cta' => 'Converter',
			'conversation.convert_lead_success' => ({required Object account}) => 'Lead convertido: ${account} entrou na carteira do atendente atribuído.',
			'conversation.convert_lead_error' => ({required Object error}) => 'Não foi possível converter o lead: ${error}',
			'conversation.priority_urgent' => 'Urgente',
			'conversation.priority_high' => 'Alta',
			'conversation.priority_normal' => 'Normal',
			'conversation.priority_low' => 'Baixa',
			'conversation.assign_label' => 'Atribuir',
			'conversation.tags_label' => 'Tags',
			'conversation.tag_picker_title' => 'Selecionar Tags',
			'conversation.tag_picker_new' => 'Nova tag',
			'conversation.tag_picker_search' => 'Buscar tag...',
			'conversation.tag_picker_empty_title' => 'Você ainda não tem tags.',
			'conversation.tag_picker_empty_body' => 'Crie a primeira tag pelo botão acima.',
			'conversation.tag_picker_no_match' => ({required Object query}) => 'Nenhuma tag para "${query}".',
			'conversation.tag_update_error' => ({required Object error}) => 'Erro ao atualizar tag: ${error}',
			'conversation.tag_create_error' => ({required Object error}) => 'Erro ao criar tag: ${error}',
			'conversation.tag_color_label' => 'Cor',
			'conversation.tag_dialog_title' => 'Nova tag',
			'conversation.tag_name_label' => 'Nome',
			'conversation.tag_name_hint' => 'Ex.: VIP, Suporte, Dúvida',
			'conversation.tag_dialog_create' => 'Criar',
			'conversation.contact_bio' => 'Bio',
			'conversation.contact_status_label' => 'Status',
			'conversation.contact_bot_label' => 'Bot',
			'conversation.contact_bot_paused' => 'Pausado',
			'conversation.contact_bot_active' => 'Ativo',
			'conversation.contact_handled_by' => 'Em atendimento por',
			'conversation.contact_last_message' => 'Última mensagem',
			'conversation.contact_close_btn' => 'Fechar',
			'conversation.status_open' => 'Aberta',
			'conversation.status_pending' => 'Aguardando atendente',
			'conversation.source_whatsapp' => 'WhatsApp',
			'conversation.source_telegram' => 'Telegram',
			'conversation.source_instagram' => 'Instagram',
			'conversation.source_sms' => 'SMS',
			'conversation.source_internal' => 'Suporte interno',
			'conversation.source_email' => 'E-mail',
			'conversation.shared_contact' => 'Contato compartilhado',
			'conversation.shared_contact_copied' => 'Número copiado',
			'conversation.media_video' => 'Vídeo',
			'conversation.resume_bot_other_confirm' => ({required Object name}) => 'Esta conversa está com ${name}. Devolver ao bot mesmo assim?',
			'conversation.contact_handled_by_inline' => ({required Object name}) => 'em atendimento por ${name}',
			'conversation.contact_bot_paused_inline' => 'bot pausado',
			'conversation.contact_bot_active_inline' => 'bot ativo',
			'lgpd.anonymize_title' => 'Anonimizar dados?',
			'lgpd.anonymize' => 'Anonimizar',
			'lgpd.copy_json' => 'Copiar JSON',
			'lgpd.no_actions' => 'Nenhuma ação registrada ainda.',
			'lgpd.audit_action_export' => 'Export LGPD',
			'lgpd.audit_action_anonymize' => 'Anonimização LGPD',
			'lgpd.screen_title' => 'LGPD & Auditoria',
			'lgpd.section_data_subject_rights' => 'DIREITOS DO TITULAR (Art. 18 LGPD)',
			'lgpd.section_infrastructure' => 'INFRAESTRUTURA',
			'lgpd.section_audit_trail' => 'TRILHA DE AUDITORIA',
			'lgpd.anonymize_confirm_body' => ({required Object externalId}) => 'Esta ação é IRREVERSÍVEL. Todos os dados pessoais do contato ${externalId} serão apagados (nome, telefone, mensagens). Métricas agregadas (contagens, campanhas) são preservadas. A ação é registrada na auditoria.',
			'lgpd.json_copied_hint' => ({required Object id}) => 'JSON copiado. Cole num arquivo lgpd-export-${id}.json',
			'lgpd.anonymize_success' => 'Dados do titular anonimizados. A ação foi registrada na auditoria.',
			'lgpd.titular_search_hint' => 'Busque um contato pelo telefone (E.164, ex: 5511999999999) para exportar ou apagar seus dados.',
			'lgpd.data_found_for' => ({required Object id}) => 'Dados encontrados para ${id}',
			'lgpd.stat_conversations' => 'Conversas',
			'lgpd.stat_messages' => 'Mensagens',
			'lgpd.stat_campaigns' => 'Campanhas',
			'lgpd.audit_records_summary' => ({required Object shown, required Object total}) => '${shown} de ${total} registros',
			'lgpd.intro_title' => 'O que é essa tela',
			'lgpd.intro_body' => 'Painel administrativo de privacidade e auditoria. É aqui que você responde a pedidos de clientes baseados na LGPD, testa a entrega de e-mails do servidor e revisa tudo que os admins da conta fizeram.',
			'lgpd.intro_bullet_rights_title' => 'Direitos do titular',
			'lgpd.intro_bullet_rights_body' => 'Exporta ou anonimiza os dados de um contato (nome, telefone, mensagens) — atende Art. 18 da LGPD em um clique.',
			'lgpd.intro_bullet_infra_title' => 'Infraestrutura',
			'lgpd.intro_bullet_infra_body' => 'Teste rápido do servidor SMTP pra garantir que e-mails de verificação, recuperação de senha e alertas estão saindo.',
			'lgpd.intro_bullet_audit_title' => 'Trilha de auditoria',
			'lgpd.intro_bullet_audit_body' => 'Linha do tempo de todas as ações administrativas (takeover, anonimização, criação de canal, etc.). Use os filtros pra rastrear um autor ou uma ação específica; exporte CSV pra anexar em relatórios.',
			'feedback.nps_title' => 'Qual a chance de você recomendar a Replider?',
			'feedback.nps_subtitle' => 'Numa escala de 0 a 10, qual a probabilidade de você recomendar a Replider para um colega ou amigo?',
			'feedback.nps_scale_low' => 'Pouco provável',
			'feedback.nps_scale_high' => 'Muito provável',
			'feedback.nps_comment_hint' => 'Qual o principal motivo da sua nota? (opcional)',
			'feedback.nps_submit' => 'Enviar feedback',
			'feedback.nps_later' => 'Agora não',
			'feedback.nps_thanks' => 'Obrigado pelo seu feedback!',
			'feedback.cancel_title' => 'Antes de ir — por que está cancelando?',
			'feedback.cancel_subtitle' => 'Seu feedback nos ajuda a melhorar a Replider. Leva só alguns segundos.',
			'feedback.cancel_comment_hint' => 'Quer compartilhar mais alguma coisa? (opcional)',
			'feedback.cancel_submit' => 'Enviar feedback',
			'feedback.cancel_thanks' => 'Obrigado pelo seu feedback!',
			'feedback.cancel_reasons.too_expensive' => 'Muito caro',
			'feedback.cancel_reasons.missing_features' => 'Faltam recursos que preciso',
			'feedback.cancel_reasons.not_using' => 'Não estou usando o suficiente',
			'feedback.cancel_reasons.switched_competitor' => 'Mudei para outra ferramenta',
			'feedback.cancel_reasons.technical_issues' => 'Problemas técnicos ou bugs',
			'feedback.cancel_reasons.temporary_pause' => 'Só pausando por enquanto',
			'feedback.cancel_reasons.other' => 'Outro',
			'product_tour.skip' => 'Pular tour',
			'product_tour.next' => 'Próximo',
			'product_tour.done' => 'Entendi',
			'product_tour.step_counter' => ({required Object current, required Object total}) => '${current} de ${total}',
			'product_tour.step1_title' => 'Bem-vindo à Replider 🎉',
			'product_tour.step1_body' => 'Seu agente de IA está pronto. Aqui vai um tour rápido de 30 segundos mostrando onde fica cada coisa para você aproveitar ao máximo.',
			'product_tour.step2_title' => 'Sua caixa de entrada',
			'product_tour.step2_body' => 'Todas as conversas dos seus canais chegam aqui em tempo real. Assuma a qualquer momento para responder você mesmo no lugar do agente.',
			'product_tour.step3_title' => 'Treine seu agente',
			'product_tour.step3_body' => 'Adicione conhecimento — preços, horários, políticas — em Configurações para o agente responder com precisão em vez de adivinhar. Quanto mais você adiciona, mais inteligente ele fica.',
			'product_tour.step4_title' => 'Acompanhe seu uso',
			'product_tour.step4_body' => 'A barra na parte de baixo mostra conversas e tokens do seu ciclo de cobrança. Faça upgrade quando quiser para aumentar seus limites conforme você cresce.',
			'feature_announcement.title' => 'Novidades',
			'feature_announcement.release_label' => 'Últimas atualizações',
			'feature_announcement.cta' => 'Entendi',
			'feature_announcement.item1_title' => 'Gerencie sua assinatura quando quiser',
			'feature_announcement.item1_body' => 'Abra o portal de cobrança seguro direto no app para atualizar o cartão, trocar de plano ou cancelar — sem precisar abrir um chamado.',
			'feature_announcement.item2_title' => 'Economize 20% no plano anual',
			'feature_announcement.item2_body' => 'Troque qualquer plano para anual e pague menos por mês. Alterne o período na tela de cobrança para ver seu novo preço.',
			'feature_announcement.item3_title' => 'Avisos de uso antes de atingir o limite',
			'feature_announcement.item3_body' => 'Agora avisamos quando você chega a 80% da sua cota mensal, para que uma semana movimentada nunca bloqueie seu agente de surpresa.',
			'errors.AUTH_INVALID_CREDENTIALS' => 'E-mail ou senha incorretos.',
			'errors.WEBHOOK_URL_BLOCKED' => 'A URL do webhook não pode apontar para a rede interna.',
			'errors.AUTH_EMAIL_ALREADY_EXISTS' => 'Este e-mail já está cadastrado.',
			'errors.REGISTRATION_PAUSED' => 'Os cadastros estão temporariamente fechados. Já tem conta? É só entrar.',
			'errors.OWNER_EMAIL_EXISTS' => 'Este e-mail já pertence a uma conta. Use o modo "conta existente" para aplicar um contrato a ela.',
			'errors.OWNER_SELLER_NOT_FOUND' => 'Nenhum vendedor ativo com esse e-mail na conta plataforma.',
			'errors.COMMISSION_SELLER_NOT_FOUND' => 'Esse usuário não foi encontrado na conta plataforma.',
			'errors.OWNER_SELLER_INACTIVE' => 'Esse vendedor está desativado.',
			'errors.OWNER_INVALID_LEAD_SOURCE' => 'Origem do lead inválida — use prospecção (ref) ou inbound.',
			'errors.OWNER_PLATFORM_NOT_CONFIGURED' => 'PLATFORM_ACCOUNT_ID não configurado neste ambiente.',
			'errors.SELLER_CONV_UNASSIGNED' => 'Esta conversa não tem atendente atribuído. Assuma (ou atribua) antes de converter.',
			'errors.SELLER_NOT_CONV_ASSIGNEE' => 'Só o atendente atribuído ou um admin pode converter este lead.',
			'errors.SELLER_ACCOUNT_ALREADY_LINKED' => 'Esta conta já está vinculada a um vendedor. Realocação é feita pelo painel do owner.',
			'errors.SELLER_CANNOT_CONVERT_PLATFORM' => 'A própria conta plataforma não pode ser convertida.',
			'errors.SELLER_NOT_FLAGGED' => 'Você não é um vendedor cadastrado. Peça ao owner para liberar o acesso de vendedor.',
			'errors.SELLER_CONV_TARGET_UNRELATED' => 'Esta conta já tem histórico de pagamento e o contato da conversa não corresponde a ela. Conversão bloqueada.',
			'errors.AUTH_UNAUTHORIZED' => 'Sessão expirada. Faça login novamente.',
			'errors.AUTH_SOCIAL_FAILED' => 'Falha na autenticação social. Tente novamente.',
			'errors.AUTH_INVALID_RESET_TOKEN' => 'Link de redefinição inválido ou expirado.',
			'errors.AUTH_EMAIL_NOT_VERIFIED' => 'Confirme seu e-mail antes de entrar. Verifique sua caixa de entrada ou solicite um novo link.',
			'errors.AGENT_NOT_FOUND' => 'Agente não encontrado.',
			'errors.AGENT_LIMIT_REACHED' => 'Você atingiu o limite de agentes do seu plano. Faça upgrade para criar mais agentes.',
			'errors.PLAN_LIMIT_EXCEEDED' => 'Você atingiu o limite do seu contrato. Fale com a gente para ampliar.',
			'errors.PLAN_LIMIT_EXCEEDED_agents' => ({required Object limit, required Object current}) => 'Seu contrato cobre ${limit} agentes (você tem ${current}). Fale com a gente para ampliar.',
			'errors.PLAN_LIMIT_EXCEEDED_channels' => ({required Object limit, required Object current}) => 'Seu contrato cobre ${limit} canais (você tem ${current}). Fale com a gente para ampliar.',
			'errors.PLAN_LIMIT_EXCEEDED_seats' => ({required Object limit, required Object current}) => 'Seu contrato cobre ${limit} usuários (você tem ${current}). Fale com a gente para ampliar.',
			'errors.CONV_NOT_FOUND' => 'Conversa não encontrada.',
			'errors.CONV_OUTSIDE_24H' => 'Fora da janela de 24h do WhatsApp.',
			'errors.USAGE_INSUFFICIENT_TOKENS' => 'Tokens de IA insuficientes.',
			'errors.USAGE_NO_CREDITS' => 'Limite de IA atingido.',
			'errors.USAGE_LIMIT_EXCEEDED' => 'Limite de uso do ciclo atingido. Fale com a gente para ampliar, ou aguarde o próximo ciclo.',
			'errors.ACCOUNT_SUSPENDED' => 'Conta suspensa por fatura não paga. Atualize a forma de pagamento para continuar.',
			'errors.TEAM_CANNOT_REMOVE_LAST_ADMIN' => 'Você não pode remover o último admin. Transfira a função primeiro.',
			'errors.TEAM_INVALID_PARTICIPANT' => 'O participante selecionado não é um membro ativo da equipe.',
			'errors.RATE_LIMITED' => 'Muitas requisições. Aguarde um minuto e tente novamente.',
			'errors.TRIAL_EXPIRED' => 'Seu período de teste terminou. Fale com a gente para ativar seu contrato.',
			'errors.SUBSCRIPTION_INACTIVE' => 'Sua conta não está ativa. Fale com a gente para regularizar.',
			'errors.TERMS_NOT_ACCEPTED' => 'Você precisa aceitar os Termos de Uso e a Política de Privacidade.',
			'errors.CONNECTION_TIMEOUT' => 'O servidor demorou demais para responder. Tente de novo em instantes.',
			'errors.CONNECTION_ERROR' => 'Não conseguimos falar com o servidor. Pode ser sua conexão ou uma instabilidade nossa — tente de novo em instantes.',
			'errors.SERVER_ERROR' => 'Servidor indisponível agora. Tente em alguns minutos.',
			'errors.REQUEST_CANCELED' => 'Operação cancelada.',
			'errors.BAD_REQUEST' => 'Dados inválidos. Confira os campos e tente novamente.',
			'errors.UNKNOWN' => 'Ocorreu um erro inesperado. Tente novamente.',
			'errors.TOTP_INVALID_CODE' => 'Código incorreto. Verifique a hora do seu dispositivo e tente novamente.',
			'errors.OWNER_2FA_STALE' => 'Sua validação de 2 fatores expirou. Confirme o código para continuar.',
			'errors.OWNER_2FA_REQUIRED' => 'Ative o 2FA no seu perfil para usar o painel do proprietário.',
			'errors.TOTP_NOT_STAGED' => 'Inicie a configuração do 2FA antes de confirmar.',
			'errors.TOTP_NOT_ENABLED' => 'A autenticação em duas etapas não está ativada.',
			'errors.TOTP_CIPHER_UNCONFIGURED' => 'A autenticação em duas etapas está indisponível no momento. Tente novamente mais tarde.',
			'errors.MESSAGE_SEND_FAILED' => 'A mensagem foi salva mas não pôde ser entregue ao cliente. Verifique o canal e tente novamente.',
			'errors.PLAN_NOT_FOUND' => 'Plano não encontrado. Atualize a página e tente novamente.',
			'errors.PLAN_NOT_PURCHASABLE' => 'Este plano é vendido sob contrato. Fale com um especialista para contratar.',
			'errors.PLAN_CURRENCY_MISMATCH' => 'Este plano não está disponível na moeda de cobrança da sua conta.',
			'errors.COUPON_EXHAUSTED' => 'Este cupom atingiu o limite de resgates.',
			'errors.ADDON_CURRENCY_UNSUPPORTED' => 'Pacotes de créditos extras não estão disponíveis na moeda de cobrança da sua conta.',
			'errors.BROADCAST_NOT_ALLOWED' => 'Seu plano não inclui campanhas em massa. Faça upgrade para ativar disparos para listas de contatos.',
			'errors.RECIPIENTS_OVER_CAMPAIGN_CAP' => ({required Object current, required Object limit}) => 'Esta campanha tem mais destinatários que seu plano permite (${current}/${limit}). Divida em campanhas menores ou faça upgrade.',
			'errors.MONTHLY_BROADCAST_CAP' => ({required Object current, required Object limit}) => 'Você atingiu o limite mensal de disparos (${current}/${limit}). Aguarde a renovação ou faça upgrade.',
			'errors.EMPTY_NAME' => 'Nome vazio.',
			'errors.UNEXPECTED' => 'Erro inesperado.',
			'errors.JSON_INVALID_OBJECT' => 'flow.json deve ser um objeto JSON válido.',
			'errors.JSON_INVALID_FORMAT' => ({required Object message}) => 'JSON inválido: ${message}',
			'errors.PROVIDER_COST_NOT_FOUND' => 'Linha de preço não encontrada.',
			'errors.PROVIDER_COST_DUPLICATE' => 'Já existe preço para esse provider/modelo/data.',
			'errors.PROVIDER_COST_INVALID_RATE' => 'Câmbio deve ficar entre 1,00 e 100,00 BRL por USD.',
			'errors.CANNED_RESPONSE_FORBIDDEN' => 'Só admins podem gerenciar respostas rápidas da conta.',
			'errors.MCP_TOKEN_NOT_FOUND' => 'Token não encontrado ou já revogado.',
			'errors.MCP_TOKEN_LIMIT_REACHED' => 'Limite de tokens ativos atingido (10). Revogue um token antigo antes de criar outro.',
			'errors.LEAD_INVALID_PHONE' => 'Telefone inválido. Use DDD + número, como 51 98888-7766.',
			'errors.LEAD_NOT_PENDING_APPROVAL' => 'Essa abordagem não está mais aguardando aprovação.',
			'errors.LEAD_SEND_FAILED' => 'Não consegui enviar a abordagem. Tente de novo em instantes.',
			'errors.LEAD_NOT_FOUND' => 'Lead não encontrado.',
			'errors.APPROACH_HAS_PLACEHOLDER' => 'A mensagem ainda tem um campo por preencher.',
			'errors.EMAIL_ADDRESS_SUPPRESSED' => 'Este endereço não recebe mais e-mails: houve rejeição permanente ou marcação de spam. Veja em Configurações → Endereços bloqueados.',
			'errors.AI_DRAFTS_NOT_ALLOWED' => 'Seu plano não inclui rascunhos por IA. Faça upgrade para a IA escrever as abordagens para você.',
			'errors.DOC_EXTRACTION_NOT_ALLOWED' => 'Seu plano não inclui leitura de contratos por IA. Faça upgrade para o contrato ser lido automaticamente.',
			'errors.SPEND_BUDGET_EXCEEDED' => 'Você atingiu o limite deste recurso no ciclo. Aguarde a renovação ou faça upgrade.',
			'errors.route_not_found_title' => 'Essa tela não existe',
			'errors.route_not_found_body' => 'O endereço pode ter sido digitado errado ou o link está desatualizado.',
			'errors.route_not_found_cta' => 'Ir para o início',
			'errors.PRODUCT_PDF_UNREADABLE' => 'Não consegui abrir o arquivo como PDF. Envie o folheto original do fabricante.',
			'errors.PRODUCT_PDF_SCAN_UNAVAILABLE' => 'O antivírus está indisponível agora — upload de PDF desabilitado. Tente mais tarde.',
			'errors.PRODUCT_PDF_INFECTED' => 'O antivírus recusou este arquivo.',
			'errors.PRODUCT_PDF_EXTRACT_FAILED' => 'A leitura automática falhou. Tente de novo ou use o import por CSV.',
			'errors.PRODUCT_PDF_NO_PRODUCTS' => 'A leitura não encontrou nenhum produto neste PDF.',
			'errors.PRODUCT_PDF_PROVIDER_CANT_READ' => 'O provedor de IA desta conta não lê arquivos. Use o import por CSV.',
			'errors.PRODUCT_PDF_IMPORT_UNAVAILABLE' => 'A importação por PDF não está configurada neste ambiente.',
			'errors.EMAIL_WARMUP_NOT_STARTED' => 'Esta caixa nunca teve o aquecimento iniciado — verifique o domínio (DNS) ou reconecte a caixa antes de prospectar.',
			'errors.BULK_SENDING_NOT_READY' => 'Aprovar em lote exige o envio em volume configurado para o seu domínio. Ative em Configurações → Canais (canal de e-mail), ou aprove uma a uma pela sua própria caixa.',
			'errors.BULK_SENDING_UNAVAILABLE' => 'O envio em volume não está disponível na plataforma no momento — aprove uma a uma, ou fale com a gente.',
			'errors.EMAIL_DAILY_CAP_REACHED' => 'A caixa atingiu o limite diário de envios frios. Os envios voltam amanhã.',
			'errors.LEAD_APPROACH_TEMPLATE_REQUIRED' => 'Este envio precisa de um template aprovado do WhatsApp — abra o lead e escolha um.',
			'errors.INVITE_INVALID' => 'Convite inválido, expirado ou já utilizado — peça um novo link para quem convidou você.',
			'errors.SALES_FUNNEL_IS_DEFAULT' => 'O funil padrão não pode ser excluído — é onde importações e leads novos entram.',
			'errors.PROSPECTING_NOT_CONFIGURED' => 'A busca de empresas não está disponível na plataforma no momento — fale com o suporte.',
			'errors.PROSPECTING_RATE_LIMITED' => 'Muitas buscas seguidas — aguarde um instante e tente de novo.',
			'errors.PROSPECTING_INPUT_REQUIRED' => 'Diga o que buscar: um segmento e uma cidade.',
			'errors.PROSPECTING_SEARCH_FAILED' => 'A busca de empresas falhou — tente de novo em instantes.',
			'errors.APPROACH_NEEDS_PRODUCTS' => 'Cadastre seus produtos primeiro — sem catálogo o rascunho sai genérico.',
			'errors.LEAD_FUNNEL_NO_CHANNEL' => 'Conecte um e-mail em Configurações → Canais para enviar.',
			'errors.LEAD_CONTACT_NOT_MESSAGEABLE' => 'Este cliente não tem e-mail cadastrado.',
			'errors.LEAD_ALREADY_CONTACTED' => 'Este cliente já foi abordado — responda pela conversa.',
			'errors.CONTACT_OPTED_OUT' => 'Este cliente pediu para não ser contatado.',
			'errors.BYOK_NOT_CONFIGURED' => 'Cadastre sua chave de IA em Configurações → Contrato.',
			'errors.FORBIDDEN' => 'Seu perfil não tem acesso a isso.',
			'errors.LEAD_NO_CONVERSATION' => 'Ninguém escreveu para este cliente ainda — envie a primeira oferta pela fila.',
			'errors.AI_PROVIDER_UNAVAILABLE' => 'O provedor de IA está indisponível agora. Tente de novo em instantes.',
			'errors.FUNNEL_STAGE_BACKWARD' => 'Um lead já contatado não volta para o começo do funil. Marque como perdido ou crie outro lead.',
			'audio_recorder.permission_blocked_title' => 'Microfone bloqueado',
			'audio_recorder.permission_blocked_body' => 'Permita o acesso ao microfone nas configurações do navegador/sistema e tente novamente.',
			'audio_recorder.ok' => 'OK',
			'audio_recorder.cancel_tooltip' => 'Cancelar',
			'audio_recorder.pause_tooltip' => 'Pausar',
			'audio_recorder.resume_tooltip' => 'Continuar',
			'audio_recorder.stop_tooltip' => 'Finalizar gravação',
			'audio_recorder.discard_tooltip' => 'Descartar',
			'audio_recorder.redo_tooltip' => 'Regravar',
			'audio_recorder.play_tooltip' => 'Reproduzir',
			'audio_recorder.pause_play_tooltip' => 'Pausar',
			'audio_recorder.send_tooltip' => 'Enviar áudio',
			'audio_recorder.start_failed' => ({required Object error}) => 'Falha ao iniciar gravação: ${error}',
			'audio_recorder.pause_failed' => ({required Object error}) => 'Falha ao pausar: ${error}',
			'audio_recorder.stop_failed' => ({required Object error}) => 'Falha ao parar gravação: ${error}',
			'audio_recorder.send_failed' => ({required Object error}) => 'Falha ao enviar áudio: ${error}',
			'audio_recorder.empty_recording' => 'Gravação vazia.',
			'audio_recorder.empty_audio' => 'Áudio vazio.',
			'bulk_actions.clear_tooltip' => 'Limpar seleção',
			'bulk_actions.selected_count' => ({required Object count}) => '${count} selecionada(s)',
			'bulk_actions.select_all' => 'Selecionar tudo',
			'bulk_actions.close_btn' => 'Encerrar',
			'bulk_actions.close_confirm_title' => ({required Object count}) => 'Encerrar ${count} conversas?',
			'bulk_actions.close_confirm_body' => 'As conversas serão movidas para "Fechadas". Mensagens novas dos clientes reabrem automaticamente.',
			'bulk_actions.close_done' => ({required Object ok, required Object total}) => '${ok} de ${total} encerradas.',
			'bulk_actions.close_partial_warning' => ' Algumas falharam — verifique permissão.',
			'bulk_actions.close_failed' => ({required Object error}) => 'Falha: ${error}',
			'image_viewer.default_filename' => 'Imagem',
			'image_viewer.default_save_filename' => 'imagem.jpg',
			'image_viewer.download_btn' => 'Baixar',
			'image_viewer.close_tooltip' => 'Fechar',
			'image_viewer.download_failed' => 'Não foi possível baixar a imagem.',
			'media_send.file_too_large' => ({required Object actualMb, required Object maxMb}) => 'Arquivo muito grande (${actualMb} MB). O limite para este tipo é ${maxMb} MB.',
			'media_send.sending_file' => ({required Object filename}) => 'Enviando ${filename}...',
			'media_send.send_file_dialog_title' => 'Enviar arquivo',
			'media_send.caption_label' => 'Legenda (opcional)',
			'media_send.send_btn' => 'Enviar',
			'media_send.cancel_btn' => 'Cancelar',
			'media_send.upload_in_progress' => 'Já existe um arquivo sendo enviado nesta conversa. Aguarde concluir.',
			'reply_suggestions.loading' => 'Gerando sugestões...',
			'reply_suggestions.load_failed' => 'Falha ao gerar sugestões. Tente novamente.',
			'reply_suggestions.retry' => 'Repetir',
			'reply_suggestions.suggest_replies' => 'Sugerir respostas',
			'reply_suggestions.refresh_tooltip' => 'Atualizar sugestões',
			'reply_suggestions.hide_tooltip' => 'Esconder',
			'pwa.install_title' => 'Instale o app',
			'pwa.install_body' => 'Receba notificações quando seus clientes mandarem mensagem, mesmo com o app fechado.',
			'pwa.install_btn' => 'Instalar',
			'pwa.not_now' => 'Agora não',
			'pwa.ios_title' => 'Adicione à tela de início',
			'pwa.ios_body' => 'Necessário para receber notificações no iPhone — o Safari só permite alertas em apps instalados.',
			'pwa.dismiss_tooltip' => 'Dispensar',
			'pwa.ios_step_1' => 'Toque em Compartilhar na barra inferior do Safari',
			'pwa.ios_step_2' => 'Escolha "Adicionar à Tela de Início"',
			'pwa.ios_step_3' => 'Abra o Replider pelo ícone instalado e ative as notificações',
			'audio_player.transcribe_tooltip' => 'Transcrever',
			'audio_player.transcribing' => 'Transcrevendo...',
			'audio_player.transcribe_failed' => 'Falha ao transcrever, tente novamente.',
			'audio_player.load_failed' => 'Não foi possível carregar o áudio. Ele pode ter expirado.',
			'media_preview.audio' => 'Áudio',
			'media_preview.image' => 'Imagem',
			'media_preview.video' => 'Vídeo',
			'media_preview.document' => 'Arquivo',
			'media_preview.location' => 'Localização',
			'media_preview.sticker' => 'Sticker',
			'sentiment.detected_tooltip' => ({required Object label}) => 'Sentimento detectado: ${label}',
			'sentiment.angry' => 'Irritado',
			'sentiment.negative' => 'Negativo',
			'sentiment.positive' => 'Positivo',
			'owner.menu_label' => 'Painel do Owner',
			'owner.accounts_title' => 'Contas',
			'owner.search_hint' => 'Buscar por empresa ou e-mail',
			'owner.create_contract_cta' => 'Novo contrato',
			'owner.empty_title' => 'Nenhuma conta ainda',
			'owner.empty_desc' => 'Use "Novo contrato" para provisionar seu primeiro cliente B2B.',
			'owner.badge_suspended' => 'suspensa',
			'owner.detail_title' => 'Conta',
			'owner.field_id' => 'ID',
			'owner.field_status' => 'Status',
			'owner.field_owner' => 'Owner',
			'owner.field_plan' => 'Plano',
			'owner.field_created' => 'Criada em',
			'owner.field_internal_threads' => 'Threads de suporte interno ativas',
			'owner.field_suspended_reason' => 'Motivo da suspensão',
			'owner.seller_label' => 'Vendedor',
			'owner.seller_none' => 'Sem vendedor vinculado',
			'owner.seller_source_label' => 'Origem do lead',
			'owner.seller_source_seller_ref' => 'Prospecção (ref) — 50%',
			'owner.seller_source_inbound' => 'Inbound — 20%',
			'owner.seller_linked_at' => 'Vinculado em',
			'owner.seller_edit_cta' => 'Vincular / realocar',
			'owner.seller_self_match_warning' => 'Vendedor coincide com o próprio admin desta conta (autoindicação) — revise o vínculo',
			'owner.seller_dialog_title' => 'Vincular vendedor',
			_ => null,
		} ?? switch (path) {
			'owner.seller_email_hint' => 'E-mail do vendedor (usuário da conta plataforma)',
			'owner.seller_unlink_cta' => 'Desvincular',
			'owner.seller_save_cta' => 'Salvar',
			'owner.seller_saved' => 'Vendedor atualizado.',
			'owner.overrides_label' => 'Limites (overrides)',
			'owner.overrides_clear_hint' => 'Deixe vazio para remover o override e voltar ao default do plano.',
			'owner.edit_overrides_cta' => 'Editar limites',
			'owner.cap_max_agents' => 'Máx. agentes',
			'owner.cap_max_channels' => 'Máx. canais',
			'owner.cap_max_conversations' => 'Máx. conversas / mês',
			'owner.cap_max_tokens' => 'Máx. tokens IA / mês',
			'owner.usage_label' => 'Uso no período',
			'owner.usage_tokens' => 'Tokens usados',
			'owner.usage_addon' => 'Tokens adicionais',
			'owner.usage_conversations' => 'Conversas',
			'owner.usage_messages' => 'Mensagens',
			'owner.suspend_cta' => 'Suspender conta',
			'owner.suspend_dialog_title' => 'Suspender conta?',
			'owner.suspend_reason_hint' => 'Motivo (obrigatório)',
			'owner.suspend_confirm_cta' => 'Suspender',
			'owner.reactivate_cta' => 'Reativar conta',
			'owner.reactivate_confirm_title' => 'Reativar essa conta?',
			'owner.reactivate_confirm_msg' => 'Mensagens recebidas e gravações voltam a funcionar imediatamente.',
			'owner.create_title' => 'Novo contrato',
			'owner.create_company' => 'Nome da empresa',
			'owner.create_admin_email' => 'E-mail do admin',
			'owner.create_admin_name' => 'Nome do admin',
			'owner.create_plan' => 'Plano',
			'owner.create_trial_bypass' => 'Pular o gate de trial',
			'owner.create_trial_bypass_hint' => 'A conta fica utilizável imediatamente sem assinatura Stripe.',
			'owner.create_submit' => 'Criar contrato',
			'owner.form_required' => 'Preencha empresa, e-mail do admin e plano.',
			'owner.top_label' => 'Maiores consumos no período',
			'owner.top_open_cta' => 'Abrir',
			'owner.grant_cta' => 'Conceder créditos',
			'owner.grant_dialog_title' => 'Conceder tokens adicionais',
			'owner.grant_hint' => 'Quantidade de tokens',
			'owner.extend_cta' => 'Estender trial',
			'owner.extend_dialog_title' => 'Estender trial por',
			'owner.extend_hint' => 'Dias a partir de agora',
			'owner.impersonate_cta' => 'Entrar como essa conta',
			'owner.impersonate_confirm_title' => 'Entrar como essa conta?',
			'owner.impersonate_confirm_msg' => 'Você vai atuar como o admin da conta por 15 minutos. Cada ação fica auditada.',
			'owner.banner_acting_as' => ({required Object account}) => 'Atuando como ${account} — cada ação está sendo registrada.',
			'owner.banner_end_cta' => 'Sair',
			'owner.banner_expires_in' => ({required Object time}) => 'expira em ${time}',
			'owner.filter_all' => 'Todas',
			'owner.filter_active' => 'Ativas',
			'owner.filter_trialing' => 'Em trial',
			'owner.filter_past_due' => 'Em atraso',
			'owner.filter_suspended' => 'Suspensas',
			'owner.cap_badge' => ({required Object pct}) => '${pct}% do limite',
			'owner.logout_blocked_during_impersonate' => 'Encerre a sessão de impersonate antes de sair.',
			'owner.conversations_label' => 'Conversas recentes',
			'owner.conversations_empty' => 'Essa conta ainda não tem conversas.',
			'owner.health_accounts' => 'Contas',
			'owner.health_active' => 'Ativas',
			'owner.health_suspended' => 'Suspensas',
			'owner.health_conv_24h' => 'Conv 24h',
			'owner.health_msg_24h' => 'Msg 24h',
			'owner.usage_trend_label' => 'Tendência de uso',
			'owner.usage_trend_desc' => 'Tokens de IA consumidos por dia, últimos 30 dias.',
			'owner.usage_trend_empty' => 'Sem uso nos últimos 30 dias.',
			'owner.usage_trend_total' => ({required Object tokens}) => '${tokens} tokens no total',
			'owner.invoices_label' => 'Faturas',
			'owner.invoices_desc' => 'Últimas 10 faturas do Stripe para esta conta.',
			'owner.invoices_empty' => 'Sem faturas — conta ainda não transacionou no Stripe.',
			'owner.invoice_status_paid' => 'Paga',
			'owner.invoice_status_open' => 'Em aberto',
			'owner.invoice_status_draft' => 'Rascunho',
			'owner.invoice_status_uncollectible' => 'Inadimplente',
			'owner.invoice_status_void' => 'Anulada',
			'owner.invoice_open_hosted' => 'Abrir página da fatura',
			'owner.invoice_open_pdf' => 'Baixar PDF',
			'owner.gate_enroll_title' => 'Ative o 2FA para continuar',
			'owner.gate_enroll_body' => 'O painel do proprietário exige autenticação em duas etapas. Configure no seu perfil e volte aqui.',
			'owner.gate_enroll_cta' => 'Ir para o perfil',
			'owner.gate_refresh_title' => 'Reconfirme seu código 2FA',
			'owner.gate_refresh_body' => 'Sua validação de 2 fatores expirou. Digite o código do autenticador para continuar — sem sair da sua sessão.',
			'owner.gate_refresh_cta' => 'Confirmar código',
			'owner.stepup_title' => 'Confirme seu 2FA',
			'owner.stepup_body' => 'Sua validação de 2 fatores expirou. Digite o código do autenticador para continuar — nada do que você preencheu é perdido.',
			'owner.stepup_code' => 'Código de 6 dígitos',
			'owner.stepup_confirm' => 'Confirmar',
			'owner.stepup_hint' => 'Você também pode usar um código de recuperação.',
			'owner.coupons_title' => 'Cupons',
			'owner.coupons_empty_title' => 'Sem cupons ainda',
			'owner.coupons_empty_desc' => 'Crie um código e compartilhe o link de convite para aplicar desconto no checkout.',
			'owner.coupon_create_cta' => 'Novo cupom',
			'owner.coupon_create_title' => 'Criar cupom',
			'owner.coupon_code_label' => 'Código',
			'owner.coupon_percent_label' => 'Desconto',
			'owner.coupon_max_label' => 'Máx. de resgates',
			'owner.coupon_max_hint' => 'Deixe em branco para ilimitado',
			'owner.coupon_expires_pick' => 'Definir expiração',
			'owner.coupon_expires_none' => 'Sem expiração',
			'owner.coupon_expires' => ({required Object date}) => 'Expira em ${date}',
			'owner.coupon_save_cta' => 'Criar',
			'owner.coupon_form_invalid' => 'Código e desconto são obrigatórios',
			'owner.coupon_created_toast' => 'Cupom criado',
			'owner.coupon_status_active' => 'ATIVO',
			'owner.coupon_status_inactive' => 'INATIVO',
			'owner.coupon_percent_off' => ({required Object percent}) => '${percent}% off',
			'owner.coupon_redeemed_unlimited' => ({required Object used}) => '${used} resgates',
			'owner.coupon_redeemed_capped' => ({required Object used, required Object cap}) => '${used} / ${cap} resgatados',
			'owner.coupon_deactivate_cta' => 'Desativar',
			'owner.coupon_deactivate_title' => 'Desativar cupom?',
			'owner.coupon_deactivate_body' => ({required Object code}) => 'O código ${code} vai parar de aceitar novos resgates. Assinaturas existentes mantêm o desconto.',
			'owner.tab_overview' => 'Visão geral',
			'owner.tab_usage' => 'Uso',
			'owner.tab_invoices' => 'Faturas',
			'owner.tab_conversations' => 'Conversas',
			'owner.actions_menu' => 'Ações',
			'owner.conversations_tap_hint' => 'Toque numa conversa para entrar nesta conta',
			'owner.sort_label' => 'Ordenar',
			'owner.sort_consumption' => 'Consumo',
			'owner.sort_recent' => 'Recentes',
			'owner.sort_status' => 'Status',
			'owner.overdue_first' => 'Em atraso no topo',
			'owner.usage_no_cap' => 'Sem limite',
			'owner.mode_new_account' => 'Nova conta',
			'owner.mode_existing_account' => 'Conta existente',
			'owner.pick_account_hint' => 'Busque a conta para aplicar o contrato',
			'owner.pick_account_label' => 'Conta',
			'owner.apply_contract_submit' => 'Aplicar contrato',
			'owner.pick_account_required' => 'Selecione uma conta primeiro',
			'owner.wa_risk_ban' => 'Risco de ban',
			'owner.wa_quality_warning' => 'Qualidade WhatsApp',
			'owner.apply_plan_cta' => 'Definir plano / contrato',
			'owner.use_our_key_title' => 'Usar nossa chave de IA',
			'owner.use_our_key_hint' => 'Cobra pela chave da plataforma na tarifa do plano, sem exigir que o cliente traga a dele.',
			'owner.provider_costs_title' => 'Custos de provider',
			'owner.provider_costs_empty_title' => 'Nenhum preço ainda',
			'owner.provider_costs_empty_desc' => 'Cadastre o preço real da API por modelo para o lucro mensal ser calculado sobre o custo verdadeiro.',
			'owner.provider_cost_add_cta' => 'Adicionar preço',
			'owner.provider_cost_add_title' => 'Novo preço de provider',
			'owner.provider_cost_edit_title' => 'Corrigir preço',
			'owner.provider_cost_provider_label' => 'Provider',
			'owner.provider_cost_model_label' => 'Modelo',
			'owner.provider_cost_input_label' => 'Input / 1M tokens',
			'owner.provider_cost_output_label' => 'Output / 1M tokens',
			'owner.provider_cost_costs_line' => ({required Object input, required Object output}) => 'in ${input}¢ / out ${output}¢ USD por 1M tokens',
			'owner.provider_cost_effective_from' => ({required Object date}) => 'Vigente desde ${date}',
			'owner.provider_cost_pick_date' => 'Mudar data',
			'owner.provider_cost_form_invalid' => 'Preencha todos os campos com valores válidos.',
			'owner.provider_cost_saved_toast' => 'Preço salvo.',
			'owner.provider_cost_delete_title' => 'Excluir preço?',
			'owner.provider_cost_delete_body' => ({required Object model}) => 'Remover a linha de preço ${model}? Cálculos de lucro passados que a usaram passam a pegar o preço vigente anterior.',
			'owner.exchange_rate_title' => 'Câmbio USD → BRL',
			'owner.exchange_rate_value' => ({required Object rate}) => 'R\$ ${rate} por US\$ 1',
			'owner.exchange_rate_edit_cta' => 'Editar',
			'owner.exchange_rate_label' => 'BRL por 1 USD',
			'owner.exchange_rate_saved' => 'Câmbio salvo.',
			'owner.payouts_title' => 'Pagamentos de vendedores',
			'owner.payouts_cycle_label' => 'Ciclo',
			'owner.payouts_total_due' => 'Total a pagar',
			'owner.payouts_total_paid' => 'Total pago',
			'owner.payouts_empty_title' => 'Nenhum pagamento neste ciclo',
			'owner.payouts_empty_desc' => 'Nenhum lançamento de comissão encontrado para este ciclo.',
			'owner.payouts_pending_line' => ({required Object amount}) => 'Pendente (ciclo): ${amount}',
			'owner.payouts_adjustment_line' => ({required Object amount, required Object count}) => 'Ajustes de ciclos anteriores: ${amount} (${count} lançamentos)',
			'owner.payouts_paid_line' => ({required Object amount}) => 'Pago neste ciclo: ${amount}',
			'owner.payouts_total_due_line' => ({required Object amount}) => 'Total a pagar: ${amount}',
			'owner.payouts_pix_label' => 'Copiar chave PIX',
			'owner.payouts_pix_missing' => 'Sem chave PIX cadastrada',
			'owner.payouts_pix_copied' => 'Chave PIX copiada.',
			'owner.payouts_settled_chip' => 'Liquidado',
			'owner.payouts_mark_paid_cta' => 'Marcar como pago',
			'owner.payouts_mark_paid_title' => 'Confirmar pagamento',
			'owner.payouts_mark_paid_body' => ({required Object amount, required Object name}) => 'Liquidar ${amount} para ${name}? Todos os lançamentos pendentes até este ciclo ficam pagos.',
			'owner.payouts_note_label' => 'Nº da transação PIX (opcional)',
			'owner.payouts_marked_toast' => 'Pagamento marcado como pago.',
			'owner.payouts_open_cycle_title' => 'Ciclo ainda aberto',
			'owner.payouts_open_cycle_body' => 'Este ciclo ainda não fechou — você liquidaria um valor parcial, e qualquer recálculo posterior vira ajuste. Pagar o ciclo em aberto mesmo assim?',
			'owner.payouts_open_cycle_cta' => 'Pagar ciclo aberto',
			'owner.payouts_export_cta' => 'Exportar CSV',
			'owner.payouts_export_copied' => 'CSV copiado para a área de transferência.',
			'owner.sellers_title' => 'Vendedores',
			'owner.sellers_subtitle' => 'Usuários da conta plataforma. Ligar o toggle de vendedor marca o usuário, gera o link de cadastro e entra no rodízio de leads — tudo de uma vez.',
			'owner.sellers_empty_title' => 'Nenhum usuário na plataforma',
			'owner.sellers_empty_desc' => 'Crie atendentes em Configurações → Equipe primeiro, depois marque-os como vendedores aqui.',
			'owner.sellers_is_seller_label' => 'É vendedor',
			'owner.sellers_active_label' => 'Ativo (SLA de comissão)',
			'owner.sellers_in_rotation_chip' => 'Na rotação',
			'owner.sellers_out_rotation_chip' => 'Fora da rotação',
			'owner.sellers_ref_link_label' => 'Copiar link de cadastro',
			'owner.sellers_link_copied' => 'Link de cadastro copiado.',
			'owner.sellers_pix_missing' => 'Sem chave PIX cadastrada',
			'owner.sellers_carteira_line' => ({required Object count}) => '${count} conta(s) na carteira',
			'owner.sellers_onboarded_toast' => 'Vendedor ativado — flag, link e rodízio configurados.',
			'owner.sellers_offboarded_toast' => 'Vendedor desativado — removido do rodízio; histórico mantido.',
			'owner.sellers_activity_toast' => 'Atividade atualizada.',
			'owner.sellers_admin_warn_title' => 'Comissionar um admin?',
			'owner.sellers_admin_warn_body' => ({required Object name}) => '${name} é admin — marcar como vendedor gera comissão e SLA para ele. Continuar?',
			'owner.sellers_admin_warn_confirm' => 'Marcar como vendedor',
			'owner.invite_title' => 'Convidar cliente',
			'owner.invite_email' => 'E-mail do cliente',
			'owner.invite_plan' => 'Plano contratado',
			'owner.invite_days' => 'Validade (dias)',
			'owner.invite_create' => 'Gerar convite',
			'owner.invite_hint' => 'A conta só pode ser criada com este link, por este e-mail, e já nasce no plano escolhido.',
			'owner.invite_link_title' => 'Link do convite — aparece uma vez',
			'owner.invite_link_hint' => 'Copie AGORA e mande pelo canal onde a venda aconteceu. O servidor guarda só um hash: fechar esta janela perde o link (revogue e reemita).',
			'owner.invite_copy' => 'Copiar link',
			'owner.invite_copied' => 'Link copiado.',
			'sales.nav_tooltip' => 'Vendas Ativas',
			'sales.back_tooltip' => 'Voltar',
			'sales.funnel_new_title' => 'Novo funil',
			'sales.funnel_detail_title' => 'Funil',
			'sales.products.title' => 'Produtos',
			'sales.products.subtitle' => 'Gerencie os produtos do seu catálogo',
			'sales.products.new_product' => 'Novo produto',
			'sales.products.edit_product' => 'Editar produto',
			'sales.products.no_products' => 'Nenhum produto ainda',
			'sales.products.empty_subtitle' => 'Adicione os produtos que você vende para que seu agente de IA saiba o que está oferecendo.',
			'sales.products.create_first' => 'Criar primeiro produto',
			'sales.products.load_error' => 'Falha ao carregar produtos',
			'sales.products.active_label' => 'Ativo',
			'sales.products.inactive_label' => 'Inativo',
			'sales.products.field_name' => 'Nome',
			'sales.products.field_name_hint' => 'ex: Bomba de infusão',
			'sales.products.field_description' => 'Descrição',
			'sales.products.field_description_hint' => 'ex: Locação mensal com manutenção preventiva inclusa (opcional)',
			'sales.products.field_price' => 'Preço',
			'sales.products.field_cycle' => 'Ciclo de recompra (dias)',
			'sales.products.field_cycle_hint' => 'ex.: 30 — de quanto em quanto tempo um cliente repõe isto. Vazio é desconhecido, e o produto nunca aparece nas recompras',
			'sales.products.cycle_invalid' => 'Use um número inteiro de dias, até 3650',
			'sales.products.field_currency' => 'Moeda',
			'sales.products.field_active' => 'Ativo',
			'sales.products.field_active_hint' => 'Produtos inativos ficam ocultos dos funis',
			'sales.products.name_required' => 'O nome é obrigatório',
			'sales.products.price_required' => 'Informe o preço',
			'sales.products.price_invalid' => 'Preço ilegível — use 1.234,56',
			'sales.products.save' => 'Salvar',
			'sales.products.created_success' => 'Produto criado',
			'sales.products.updated_success' => 'Produto atualizado',
			'sales.products.deleted_success' => 'Produto excluído',
			'sales.products.delete_title' => 'Excluir produto',
			'sales.products.delete_message' => ({required Object name}) => 'Excluir "${name}"? Esta ação não pode ser desfeita.',
			'sales.products.not_found' => 'Produto não encontrado',
			'sales.products.not_found_hint' => 'Ele pode ter sido apagado, ou o endereço está errado.',
			'sales.products.import_first' => 'Importar planilha ou PDF',
			'sales.products.field_price_optional' => 'Vazio = o vendedor cota na hora',
			'sales.products.sheet_title' => 'Do folheto do fabricante',
			'sales.products.sheet_brand' => 'Marca',
			'sales.products.sheet_model' => 'Modelo',
			'sales.products.sheet_sku' => 'Código',
			'sales.products.sheet_unit' => 'Unidade',
			'sales.products.sheet_specs' => 'Ficha técnica',
			'sales.products.sheet_accessories' => 'Acessórios',
			'sales.products.sheet_standards' => 'Normas e certificações',
			'sales.products.open_brochure' => ({required Object page}) => 'Abrir folheto (pág. ${page})',
			'sales.products.open_brochure_no_page' => 'Abrir folheto',
			'sales.products.brochure_missing' => 'Este produto não veio de um folheto arquivado',
			'sales.products.sheet_images' => 'Imagens do folheto',
			'sales.products.sheet_images_empty' => 'Nenhuma imagem aproveitável nesta página',
			'sales.products.image_page' => ({required Object page}) => 'pág. ${page}',
			'sales.products.image_open' => 'Abrir imagem',
			'sales.funnels.title' => 'Funis',
			'sales.funnels.new_funnel' => 'Novo funil',
			'sales.funnels.edit_funnel' => 'Editar funil',
			'sales.funnels.no_funnels' => 'Nenhum funil ainda',
			'sales.funnels.empty_subtitle' => 'Crie um funil para definir quais dados a IA deve coletar durante as conversas.',
			'sales.funnels.create_first' => 'Criar primeiro funil',
			'sales.funnels.load_error' => 'Falha ao carregar funis',
			'sales.funnels.active_label' => 'Ativo',
			'sales.funnels.inactive_label' => 'Inativo',
			'sales.funnels.fields_count' => ({required Object n}) => '${n} campos',
			'sales.funnels.field_name' => 'Nome',
			'sales.funnels.field_name_hint' => 'ex: Hospitais públicos de MT',
			'sales.funnels.name_required' => 'O nome é obrigatório',
			'sales.funnels.save' => 'Salvar',
			'sales.funnels.created_success' => 'Funil criado',
			'sales.funnels.updated_success' => 'Funil atualizado',
			'sales.funnels.deleted_success' => 'Funil excluído',
			'sales.funnels.delete_title' => 'Excluir funil',
			'sales.funnels.delete_message' => ({required Object name}) => 'Excluir "${name}"? Esta ação não pode ser desfeita.',
			'sales.funnels.not_found' => 'Funil não encontrado',
			'sales.funnels.not_found_hint' => 'Ele pode ter sido apagado, ou o endereço está errado.',
			'sales.funnels.default_undeletable' => 'Funil padrão — importações e leads novos entram aqui; não pode ser excluído.',
			'sales.schema.section_title' => 'Dados a coletar',
			'sales.schema.section_subtitle' => 'Defina os campos que a IA deve coletar durante a conversa.',
			'sales.schema.add_field' => 'Adicionar campo',
			'sales.schema.field_key' => 'Chave',
			'sales.schema.field_key_hint' => 'ex: cpf_cliente',
			'sales.schema.field_label' => 'Rótulo',
			'sales.schema.field_label_hint' => 'ex: CPF do cliente',
			'sales.schema.field_type' => 'Tipo',
			'sales.schema.field_required' => 'Obrigatório',
			'sales.schema.field_ai_hint' => 'Dica para a IA',
			'sales.schema.field_ai_hint_hint' => 'Como/quando coletar (opcional)',
			'sales.schema.field_enum_options' => 'Opções (separadas por vírgula)',
			'sales.schema.field_enum_options_hint' => 'ex: pequeno, médio, grande',
			'sales.schema.remove_field' => 'Remover campo',
			'sales.schema.no_fields' => 'Nenhum campo definido ainda.',
			'sales.schema.error_key_required' => 'A chave é obrigatória',
			'sales.schema.error_key_format' => 'Use snake_case (letras minúsculas, números e _)',
			'sales.schema.error_key_duplicate' => 'Chave duplicada',
			'sales.schema.error_enum_required' => 'Defina pelo menos uma opção',
			'sales.schema.types.text' => 'Texto',
			'sales.schema.types.number' => 'Número',
			'sales.schema.types.cpf' => 'CPF',
			'sales.schema.types.cnpj' => 'CNPJ',
			'sales.schema.types.date' => 'Data',
			'sales.schema.types.bool' => 'Sim/Não',
			'sales.schema.types.kEnum' => 'Opções',
			'sales.webhook.section_title' => 'Webhook (opcional)',
			'sales.webhook.section_subtitle' => 'Só preencha se você tem um sistema próprio para receber esses eventos. O funil funciona sem isso.',
			'sales.webhook.field_url' => 'URL do webhook',
			'sales.webhook.field_url_hint' => 'https://seu-servidor.com/webhooks/replider',
			'sales.webhook.url_invalid' => 'Informe uma URL http(s) válida',
			'sales.webhook.field_secret' => 'Segredo',
			'sales.webhook.field_secret_hint' => 'Usado para assinar o payload (HMAC)',
			'sales.webhook.generate_secret' => 'Gerar',
			'sales.webhook.regenerate_secret' => 'Regerar',
			'sales.webhook.secret_copied' => 'Segredo copiado',
			'sales.webhook.trigger_required_complete' => 'Disparar quando campos obrigatórios completos',
			'sales.webhook.trigger_required_complete_hint' => 'Envia um evento lead.qualified quando todos os campos obrigatórios são preenchidos.',
			'sales.webhook.trigger_buy_intent' => 'Disparar ao detectar intenção de compra',
			'sales.webhook.trigger_buy_intent_hint' => 'Envia um evento lead.buy_intent quando a IA detecta intenção de compra.',
			'sales.webhook.send_test' => 'Enviar evento de teste',
			'sales.webhook.test_save_first' => 'Salve o funil antes de enviar um evento de teste.',
			'sales.webhook.test_url_first' => 'Defina uma URL de webhook antes de enviar um evento de teste.',
			'sales.webhook.test_success' => ({required Object status}) => 'Evento de teste enviado (HTTP ${status})',
			'sales.webhook.test_failed' => 'Falha no evento de teste',
			'sales.lead_profile.section_title' => 'Perfil de vendas',
			'sales.lead_profile.empty' => 'Esta conversa não faz parte de nenhum funil.',
			'sales.lead_profile.funnel_label' => 'Funil',
			'sales.lead_profile.status_label' => 'Status do lead',
			'sales.lead_profile.score_label' => 'Score',
			'sales.lead_profile.buy_intent' => 'Intenção de compra detectada',
			'sales.lead_profile.no_value' => 'Ainda não coletado',
			'sales.lead_profile.pending' => 'Pendente',
			'sales.lead_profile.origin_ai' => 'IA',
			'sales.lead_profile.origin_human' => 'Humano',
			'sales.lead_profile.load_error' => 'Não foi possível carregar o perfil de vendas.',
			'sales.lead_profile.lead_status.kNew' => 'Novo',
			'sales.lead_profile.lead_status.prospected' => 'Prospectado',
			'sales.lead_profile.lead_status.enriching' => 'Enriquecendo',
			'sales.lead_profile.lead_status.enriched' => 'Enriquecido',
			'sales.lead_profile.lead_status.pending_approval' => 'Aguardando aprovação',
			'sales.lead_profile.lead_status.contacted' => 'Contatado',
			'sales.lead_profile.lead_status.responded' => 'Respondeu',
			'sales.lead_profile.lead_status.qualified' => 'Qualificado',
			'sales.lead_profile.lead_status.won' => 'Ganho',
			'sales.lead_profile.lead_status.lost' => 'Perdido',
			'sales.lead_profile.edit_field' => 'Editar campo',
			'sales.lead_profile.edit_error' => ({required Object error}) => 'Não foi possível salvar: ${error}',
			'sales.lead_profile.pick_date' => 'Escolher data',
			'sales.lead_profile.new_proposal' => 'Nova proposta',
			'sales.leads.status_new' => 'Novo',
			'sales.leads.status_prospected' => 'Prospectado',
			'sales.leads.status_enriching' => 'Enriquecendo',
			'sales.leads.status_enriched' => 'Enriquecido',
			'sales.leads.status_pending_approval' => 'Aguardando aprovação',
			'sales.leads.status_contacted' => 'Contatado',
			'sales.leads.status_responded' => 'Respondeu',
			'sales.leads.status_qualified' => 'Qualificado',
			'sales.leads.status_won' => 'Ganho',
			'sales.leads.status_lost' => 'Perdido',
			'sales.leads.discard' => 'Descartar',
			'sales.leads.template_label' => 'Template do WhatsApp',
			'sales.leads.template_select' => 'Escolha um template',
			'sales.leads.template_required' => 'Selecione um template aprovado antes de enviar',
			'sales.leads.template_none' => 'Nenhum template aprovado neste canal ainda',
			'sales.leads.template_no_channel' => 'Vincule um canal WhatsApp a este funil primeiro',
			'sales.leads.template_load_error' => 'Não foi possível carregar os templates',
			'sales.prospecting.intro' => 'Busque empresas no Google Places e importe-as como leads em um funil.',
			'sales.prospecting.funnel_label' => 'Funil de destino',
			'sales.prospecting.keyword_label' => 'Palavra-chave',
			'sales.prospecting.keyword_hint' => 'ex.: hospital, clínica de imagem, UTI',
			'sales.prospecting.location_label' => 'Localização',
			'sales.prospecting.location_hint' => 'ex.: Cuiabá, MT',
			'sales.prospecting.radius_label' => 'Raio (metros, opcional)',
			'sales.prospecting.radius_hint' => 'ex.: 5000',
			'sales.prospecting.search_import' => 'Buscar e importar',
			'sales.prospecting.query_required' => 'Informe uma palavra-chave ou localização e escolha um funil',
			'sales.prospecting.no_funnels' => 'Crie um funil primeiro para importar os leads prospectados.',
			'sales.prospecting.result_title' => 'Busca concluída',
			'sales.prospecting.result_created' => ({required Object n, required Object total}) => '${n} de ${total} resultados importados como leads.',
			'sales.prospecting.result_skipped' => ({required Object n}) => '${n} ignorados (já estão neste funil).',
			'sales.prospecting.view_pipeline' => 'Ver pipeline',
			'sales.prospecting.preview_found' => ({required Object total, required Object duplicates}) => '${total} encontradas · ${duplicates} já na sua base',
			'sales.prospecting.quota_left' => ({required Object count}) => 'restam ${count} leads no mês',
			'sales.prospecting.already_lead' => 'já está na sua base',
			'sales.prospecting.import_n' => ({required Object count}) => 'Importar ${count} leads',
			'sales.prospecting.imported' => ({required Object count}) => '${count} leads importados',
			'sales.prospecting.select_all' => 'Selecionar todos',
			'sales.prospecting.select_none' => 'Limpar seleção',
			'sales.prospecting.selected_of' => ({required Object n, required Object total}) => '${n} de ${total} selecionados',
			'sales.prospecting.imported_partial' => ({required Object created, required Object selected}) => '${created} de ${selected} importados — a franquia mensal de leads do plano parou o restante.',
			'sales.prospecting.no_results_title' => 'Nenhuma empresa encontrada',
			'sales.prospecting.no_results_hint' => 'Tente outra palavra-chave ou um raio maior — \'clínica\', \'hospital\', uma cidade vizinha.',
			'sales.hunt_tab' => 'Caçar',
			'support.title' => 'Suporte',
			'support.subtitle' => 'Seu gerente de conta Replider',
			'support.nav_tooltip' => 'Suporte',
			'support.empty_title' => 'Fale com a gente',
			'support.empty_subtitle' => 'Dúvidas sobre sua conta, canais ou cobrança? Envie uma mensagem — nosso time responde por aqui.',
			'support.input_hint' => 'Digite uma mensagem...',
			'support.load_error' => 'Não foi possível carregar sua conversa com o suporte.',
			'support.retry' => 'Tentar novamente',
			'support.push_prompt_title' => 'Receba avisos das respostas',
			'support.push_prompt_body' => 'Ative as notificações e avisamos quando seu gerente de conta responder, mesmo com o painel fechado.',
			'support.push_prompt_enable' => 'Ativar notificações',
			'support.push_prompt_later' => 'Agora não',
			'claude_integration.title' => 'Claude (IA)',
			'claude_integration.back_tooltip' => 'Voltar',
			'claude_integration.load_failed' => ({required Object error}) => 'Falha ao carregar: ${error}',
			'claude_integration.copy_tooltip' => 'Copiar',
			'claude_integration.copied' => 'Copiado.',
			'claude_integration.connect_title' => 'Conectar ao Claude',
			'claude_integration.connect_subtitle' => 'Gerencie sua conta conversando com o Claude — configurar o agente, responder clientes, ver métricas. Funciona no site claude.ai e nos apps de celular e computador.',
			'claude_integration.tokens_title' => 'Tokens de acesso',
			'claude_integration.tokens_subtitle' => 'Tokens pessoais para o Claude Code. Substitua SEU_TOKEN no comando acima. O token completo aparece só uma vez.',
			'claude_integration.tokens_create' => 'Criar token',
			'claude_integration.tokens_empty' => 'Nenhum token criado.',
			'claude_integration.tokens_never_used' => 'nunca usado',
			'claude_integration.tokens_last_used' => ({required Object date}) => 'usado em ${date}',
			'claude_integration.tokens_revoke_tooltip' => 'Revogar token',
			'claude_integration.create_dialog_title' => 'Novo token de acesso',
			'claude_integration.create_dialog_name_label' => 'Nome (ex.: Claude Code do Tiago)',
			'claude_integration.create_dialog_confirm' => 'Criar',
			'claude_integration.token_dialog_title' => 'Copie o token agora',
			'claude_integration.token_dialog_body' => 'Esta é a única vez que o token será exibido. Copie e cole no Claude — não é possível recuperá-lo depois.',
			'claude_integration.token_dialog_copy' => 'Copiar',
			'claude_integration.token_copied' => 'Token copiado.',
			'claude_integration.token_dialog_done' => 'Já copiei',
			'claude_integration.revoke_token_title' => 'Revogar token?',
			'claude_integration.revoke_token_body' => 'O Claude conectado com este token perderá o acesso imediatamente.',
			'claude_integration.revoke_confirm' => 'Revogar',
			'claude_integration.connections_title' => 'Conexões ativas',
			'claude_integration.connections_subtitle' => 'Aplicativos autorizados via OAuth (ex.: conector do claude.ai).',
			'claude_integration.connections_empty' => 'Nenhuma conexão ativa.',
			'claude_integration.connections_revoke' => 'Revogar',
			'claude_integration.revoke_connection_title' => 'Revogar conexão?',
			'claude_integration.revoke_connection_body' => 'Este aplicativo perderá o acesso à sua conta Replider imediatamente.',
			'claude_integration.connect_step1' => 'Copie o endereço abaixo.',
			'claude_integration.connect_step2' => 'No Claude, vá em Configurações → Conectores → Adicionar conector personalizado e cole o endereço.',
			'claude_integration.connect_step3' => 'Clique em conectar e entre com o mesmo e-mail e senha que você usa aqui na Replider.',
			'claude_integration.connect_no_token_note' => 'Não precisa de token nem instalar nada — é só entrar com seu e-mail e senha. (Conectores personalizados exigem um plano Claude Pro, Max, Team ou Enterprise.)',
			'claude_integration.advanced_title' => 'Avançado — Claude Code',
			'claude_integration.advanced_subtitle' => 'Para quem usa o Claude Code no terminal. Se você usa o Claude no site ou nos apps, ignore esta seção.',
			'claude_integration.cc_oauth_label' => 'Recomendado — login pelo navegador (sem token)',
			'claude_integration.cc_oauth_hint' => 'Rode o comando no terminal. O Claude Code abre o navegador, você entra com seu e-mail e senha da Replider e volta pro terminal automaticamente.',
			'claude_integration.cc_token_label' => 'Alternativa — com token (para servidor/CI, sem navegador)',
			'claude_integration.cc_token_hint' => 'Use quando não há navegador disponível. Crie um token abaixo e substitua SEU_TOKEN no comando.',
			'shell.start' => 'Início',
			'shell.funnel' => 'Clientes',
			'shell.conversations' => 'Conversas',
			'shell.offers' => 'Oportunidades',
			'shell.copilot' => 'Copiloto',
			'shell.copilot_soon_title' => 'Copiloto em construção',
			'shell.copilot_soon_body' => 'Por enquanto, converse com a Replider pelo conector MCP no Claude ou no ChatGPT — as mesmas ações do app estão lá.',
			'shell.start_title' => 'Seu dia',
			'shell.start_subtitle' => 'Atalhos para onde o trabalho acontece.',
			'shell.start_go_funnel' => 'Ver o funil',
			'shell.start_go_conversations' => 'Abrir conversas',
			'shell.start_go_prospecting' => 'Prospectar empresas',
			'shell.stat_active_leads' => 'Leads ativos',
			'shell.stat_pending_approval' => 'Aguardando aprovação',
			'shell.stat_unowned' => 'Sem dono',
			'shell.needs_you_title' => 'PRECISA DE VOCÊ',
			'shell.needs_you_clear' => 'Nada esperando por você agora.',
			'shell.needs_open' => 'Abrir',
			'shell.needs_approvals' => ({required Object count}) => '${count} abordagem(ns) aguardando sua aprovação',
			'shell.needs_unowned' => ({required Object count}) => '${count} lead(s) sem dono — ninguém está cuidando',
			'shell.needs_empty_pipeline' => 'Seu funil está vazio. Comece caçando clientes.',
			'board.move_to' => 'Mover para',
			'board.score' => 'Score',
			'board.unassigned' => 'Sem dono',
			'board.empty_column' => 'Nenhum lead aqui',
			'board.empty_board' => 'Este funil ainda não tem etapas.',
			'board.select_funnel' => 'Escolha um funil',
			'board.board_title' => 'Funil',
			'board.new_lead' => 'Novo lead',
			'board.new_lead_name' => 'Contato',
			'board.new_lead_name_hint' => 'Quem decide ou compra',
			'board.new_lead_phone' => 'Telefone / WhatsApp',
			'board.new_lead_phone_hint' => '+55 65 99999-0000',
			'board.new_lead_created' => 'Lead adicionado ao funil.',
			'board.new_lead_existed' => 'Esse contato já está no funil.',
			'board.no_funnel_yet' => 'Você ainda não tem um funil.',
			'board.search_hint' => 'Buscar empresa, cidade ou contato',
			'board.search_clear' => 'Limpar busca',
			'board.filter_all' => ({required Object count}) => 'Todos · ${count}',
			'board.filter_responded' => ({required Object count}) => 'Responderam · ${count}',
			'board.filter_awaiting' => ({required Object count}) => 'Esperando aprovação · ${count}',
			'board.filter_stalled' => ({required Object count}) => 'Parados há mais de 7 dias · ${count}',
			'board.column_showing' => ({required Object showing, required Object total}) => '${showing} de ${total} · mais parados primeiro',
			'board.column_showing_manual' => ({required Object showing, required Object total}) => '${showing} de ${total} · ordem manual',
			'board.order_reset' => 'Ordenar por parados',
			'board.column_no_match' => ({required Object query}) => 'Nenhum resultado para “${query}” nesta etapa.',
			'board.column_no_match_hint' => 'Pode estar em outra etapa, ou ainda não ter sido importado.',
			'board.truncated_notice' => ({required Object count}) => 'Mostrando os ${count} leads mais recentes deste funil. A busca cobre só eles.',
			'board.idle_days' => ({required Object count}) => '${count}d parado',
			'board.idle_today' => 'Movido hoje',
			'board.idle_unknown' => 'Sem movimentação registrada',
			'board.view_pipeline' => 'Pipeline',
			'board.tag_draft_ready' => 'Rascunho em Hoje →',
			'board.tag_buy_intent' => 'Quer comprar',
			'board.tag_qualified' => 'Qualificado',
			'board.tag_idle_risk' => 'Esfriando',
			'board.field_required' => 'Obrigatório',
			'board.funnel_fields' => 'Campos do funil',
			'board.funnel_fields_saved' => 'Campo salvo',
			'board.funnel_fields_empty' => 'Este funil não pede nenhum campo extra.',
			'board.owner' => 'Responsável',
			'board.owner_none' => 'Ninguém ainda',
			'board.owner_assign' => 'Definir responsável',
			'board.visibility' => 'Visibilidade',
			'board.visibility_team' => 'Time todo',
			_ => null,
		} ?? switch (path) {
			'board.visibility_private' => 'Só o responsável',
			'board.visibility_private_hint' => 'Some do funil de quem não é o responsável.',
			'board.sources' => 'De onde veio',
			'board.sources_hint' => 'A IA montou o perfil a partir destas páginas.',
			'board.bulk_assign' => 'Atribuir sem responsável',
			'board.bulk_assign_title' => 'Atribuir leads sem responsável',
			'board.bulk_assign_body' => ({required Object count}) => '${count} leads deste funil não têm responsável. Passar todos para:',
			'board.bulk_assign_filtered' => 'Conta só o que está visível com a busca e o filtro atuais.',
			'board.bulk_assign_confirm' => ({required Object count}) => 'Atribuir ${count}',
			'board.bulk_assign_done' => ({required Object count}) => '${count} leads atribuídos',
			'board.bulk_assign_partial' => ({required Object count, required Object skipped}) => '${count} atribuídos · ${skipped} não estavam mais lá',
			'board.bulk_assign_cap' => 'Máximo de 200 por vez. Serão atribuídos os 200 primeiros.',
			'board.bulk_assign_here' => 'Atribuir nesta busca',
			'board.new_lead_subtitle' => 'A empresa e a pessoa com quem você fala lá.',
			'board.new_lead_company' => 'Empresa',
			'board.new_lead_company_hint' => 'ex: Hospital Regional de Cuiabá',
			'board.new_lead_company_required' => 'Informe a empresa.',
			'board.new_lead_role' => 'Cargo',
			'board.new_lead_role_hint' => 'ex: Gerente de compras',
			'board.new_lead_email' => 'E-mail',
			'board.new_lead_email_hint' => 'compras@hospital.com.br',
			'board.new_lead_city' => 'Cidade',
			'board.new_lead_city_hint' => 'ex: Cuiabá, MT',
			'board.new_lead_reach_help' => 'Telefone ou e-mail — pelo menos um, para a oferta ter por onde sair.',
			'board.new_lead_reach_required' => 'Informe um telefone ou um e-mail.',
			'board.new_lead_submit' => 'Adicionar',
			'board.configure_stages' => 'Configurar etapas',
			'board.tag_responded' => 'Respondeu',
			'board.column_hint' => ({required Object status}) => 'Leads com status "${status}". Movem sozinhos no envio e na resposta — ou arraste.',
			'board.column_hint_terminal' => ({required Object status}) => 'Etapa final — leads com status "${status}".',
			'board.panel_open_in_queue' => 'Abrir rascunho em Hoje',
			'board.panel_cnes_title' => 'Parque no CNES',
			'board.panel_cnes_by_phone' => 'dado público · bate pelo telefone · quantidade, não idade',
			'board.panel_cnes_by_name' => 'dado público · nome único no cadastro · quantidade, não idade',
			'board.panel_cnes_month' => ({required Object month}) => 'dados CNES de ${month}',
			'board.panel_cnes_code' => ({required Object code}) => 'código CNES ${code}',
			'board.panel_offer_title' => 'O que você pode oferecer',
			'board.panel_offer_line' => ({required Object product, required Object count, required Object unit, required Object equipment}) => '${product} — eles têm ${count} ${unit} de ${equipment}',
			'board.panel_offer_none' => 'Nenhum produto do seu catálogo casa com o parque declarado',
			'board.panel_offer_free' => 'cruzamento do seu catálogo com o registro — sem IA',
			'board.panel_offer_tail' => ({required Object count, required Object unit, required Object equipment}) => ' — eles têm ${count} ${unit} de ${equipment}',
			'board.panel_offer_open' => ({required Object product}) => 'Abrir ficha de ${product}',
			'lead_detail.profile' => 'Perfil',
			'lead_detail.approach' => 'Abordagem',
			'lead_detail.approach_hint' => 'Escreva ou gere a mensagem de abordagem…',
			'lead_detail.regenerate' => 'Gerar outra',
			'lead_detail.generate' => 'Gerar abordagem',
			'lead_detail.approve_send' => 'Aprovar e enviar',
			'lead_detail.timeline' => 'Histórico',
			'lead_detail.confirm_title' => 'Confirmar envio',
			'lead_detail.confirm_to' => 'Para',
			'lead_detail.sent' => 'Abordagem enviada.',
			'lead_detail.read_only' => 'Você pode ver este lead, mas ele é de outro vendedor.',
			'lead_detail.queue_title' => 'Fila de aprovação',
			'lead_detail.queue_empty' => 'Nada para aprovar agora.',
			'lead_detail.queue_empty_hint' => 'Quando a IA rascunhar abordagens, elas esperam sua aprovação aqui.',
			'lead_detail.queue_remaining' => ({required Object count}) => '${count} restantes',
			'lead_detail.discard' => 'Descartar',
			'lead_detail.edit' => 'Editar',
			'lead_detail.discarded' => 'Rascunho descartado — o lead volta ao funil.',
			'lead_detail.undo' => 'Desfazer',
			'lead_detail.send_channel' => 'Sai por',
			'lead_detail.bulk_needs_template' => 'WhatsApp precisa de template aprovado — abra o lead',
			'lead_detail.check_title' => 'Verificação automática',
			'lead_detail.check_clean' => 'Nenhum padrão de promessa de prazo, preço ou resultado encontrado no texto.',
			'lead_detail.check_found' => ({required Object count}) => '${count} trecho(s) para você conferir antes de enviar.',
			'lead_detail.check_edited' => 'Você editou o texto. A verificação abaixo é do rascunho salvo — este texto ainda não foi verificado.',
			'lead_detail.check_kind_promise' => 'Compromisso assumido no seu nome',
			'lead_detail.check_kind_invented_fact' => 'Afirmação sobre o cliente sem base nos dados',
			'lead_detail.check_kind_out_of_scope' => 'Fora do que a empresa declarou atender',
			'lead_detail.check_kind_placeholder' => 'Campo de preencher que sobrou',
			'lead_detail.check_kind_tone' => 'Tratamento inadequado ao contexto',
			'lead_detail.review_action' => 'Revisar com IA',
			'lead_detail.review_running' => 'Lendo a mensagem…',
			'lead_detail.review_clean' => 'Revisado por uma segunda IA: nada a apontar.',
			'lead_detail.review_found' => ({required Object count}) => 'A segunda IA apontou ${count} ponto(s).',
			'lead_detail.review_unavailable' => 'A revisão por IA não rodou agora. A verificação automática acima continua valendo.',
			'lead_detail.written_by_ai' => ({required Object source}) => 'Escrito pela IA a partir de: ${source}',
			'lead_detail.written_by_ai_unknown' => 'Escrito pela IA',
			'lead_detail.shortcut_approve' => 'aprovar',
			'lead_detail.shortcut_edit' => 'editar',
			'lead_detail.shortcut_discard' => 'descartar',
			'lead_detail.shortcut_move' => 'navegar',
			'lead_detail.contact' => 'Contato',
			'lead_detail.call' => 'Ligar',
			'lead_detail.open_conversation' => 'Abrir conversa',
			'lead_detail.no_conversation' => 'Ainda não há conversa com este contato.',
			'lead_detail.enrich' => 'Pesquisar esta empresa',
			'lead_detail.enrich_again' => 'Pesquisar de novo',
			'lead_detail.enriching' => 'Pesquisando…',
			'lead_detail.enrich_started' => 'Pesquisando a empresa. Volte em instantes.',
			'lead_detail.profile_empty' => 'Ainda não sabemos nada sobre esta empresa.',
			'lead_detail.profile_empty_hint' => 'A pesquisa busca decisor, atividade e endereço para a abordagem citar algo real em vez de um texto genérico.',
			'lead_detail.profile_researching' => 'Pesquisando esta empresa agora. O resultado aparece aqui.',
			'lead_detail.profile_found_nothing' => 'A pesquisa não encontrou dados públicos sobre esta empresa.',
			'lead_detail.profile_found_nothing_hint' => 'Acontece com empresas pequenas ou sem presença online. Escreva a abordagem com o que você já sabe, ou pesquise de novo mais tarde.',
			'lead_detail.profile_blocked' => 'Não deu para pesquisar: este lead não tem nome de empresa nem site.',
			'lead_detail.profile_blocked_hint' => 'A pesquisa parte do nome ou do site para achar o decisor. Sem um dos dois não há o que buscar.',
			'lead_detail.summary' => 'Resumo da conversa',
			'lead_detail.summary_generating' => 'Lendo a conversa…',
			'lead_detail.summary_short' => 'Conversa curta demais para resumir.',
			'lead_detail.summary_failed' => 'Não consegui resumir agora.',
			'lead_detail.website' => 'Site',
			'lead_detail.check_kind_missing_attachment' => 'O texto diz que vai anexo, e não há anexo',
			'lead_detail.blocked_placeholder' => 'Preencha o campo antes de enviar.',
			'lead_detail.subject' => 'Assunto',
			'lead_detail.subject_hint' => 'O que o destinatário vê antes de abrir',
			'lead_detail.use_template' => 'Usar um modelo',
			'lead_detail.reply_rate_short' => 'de resposta',
			'lead_detail.bulk_selected' => ({required Object count}) => '${count} selecionadas',
			'lead_detail.bulk_send' => 'Enviar selecionadas',
			'lead_detail.bulk_confirm_title' => ({required Object count}) => 'Enviar ${count} abordagens?',
			'lead_detail.bulk_review_title' => ({required Object count}) => 'Revisar ${count} abordagens',
			'lead_detail.bulk_review_hint' => 'Desmarque o que não deve ir. Cada texto sai exatamente como está aqui.',
			'lead_detail.bulk_review_send' => ({required Object count}) => 'Enviar ${count}',
			'lead_detail.bulk_review_none' => 'Nada marcado',
			'lead_detail.bulk_sent' => ({required Object count}) => '${count} abordagens enviadas',
			'lead_detail.bulk_partial' => ({required Object count, required Object failed}) => '${count} enviadas. Falharam: ${failed}',
			'lead_detail.profile_facts_count' => ({required Object count}) => '${count} fato(s) sobre a empresa',
			'lead_detail.profile_nothing_yet' => 'Nada pesquisado ainda',
			'lead_detail.timeline_events' => ({required Object count}) => '${count} evento(s)',
			'lead_detail.section_expand' => 'Abrir',
			'lead_detail.section_collapse' => 'Fechar',
			'lead_detail.queue_filter_all' => 'Todos',
			'lead_detail.queue_no_funnel' => 'Sem funil',
			'lead_detail.queue_sort_oldest' => 'Mais antigos primeiro',
			'lead_detail.queue_sort_newest' => 'Mais recentes primeiro',
			'lead_detail.queue_flagged_only' => 'Só com alerta',
			'lead_detail.queue_partial' => ({required Object shown, required Object total}) => 'Mostrando ${shown} de ${total}. Escolha um funil para trabalhar o resto.',
			'lead_detail.queue_filtered_empty' => 'Nada nesta seleção.',
			'lead_detail.queue_filtered_empty_hint' => 'Os outros rascunhos continuam na fila — tire o filtro para vê-los.',
			'lead_detail.queue_waiting_minutes' => ({required Object count}) => 'na fila há ${count} min',
			'lead_detail.queue_waiting_hours' => ({required Object count}) => 'na fila há ${count}h',
			'lead_detail.queue_waiting_days' => ({required Object count}) => 'na fila há ${count}d',
			'lead_detail.queue_empty_cta' => 'Buscar empresas',
			'lead_detail.email_warmup_banner_named' => ({required Object mailbox, required Object day, required Object total, required Object remaining}) => 'Caixa ${mailbox} em aquecimento (dia ${day} de ${total}): restam ${remaining} envios frios hoje. O limite cresce a cada dia para proteger seu domínio.',
			'lead_detail.email_warmup_exhausted' => ({required Object mailbox, required Object day, required Object total}) => 'A caixa ${mailbox} esgotou os envios frios de hoje (dia ${day} de ${total} do aquecimento). Novas aprovações só saem amanhã.',
			'lead_detail.field_company' => 'Empresa',
			'lead_detail.field_website' => 'Site',
			'lead_detail.field_decision_maker' => 'Decisor',
			'lead_detail.field_role' => 'Cargo',
			'lead_detail.field_address' => 'Endereço',
			'lead_detail.field_activity' => 'Atividade',
			'lead_detail.bulk_subject_hint' => 'Assunto dos e-mails deste lote',
			'lead_detail.bulk_subject_missing' => 'Adicione um assunto para enviar',
			'lead_detail.error_action_configure' => 'Configurar',
			'lead_detail.approve_needs_queue_status' => 'Este lead não está aguardando aprovação — gere um rascunho para colocá-lo de volta na fila.',
			'lead_detail.approach_locked_after_send' => 'Esta abordagem já foi enviada — o texto fica travado para o histórico continuar verdadeiro.',
			'lead_detail.research_still_running' => 'A pesquisa está demorando — ela continua em segundo plano; volte mais tarde.',
			'lead_detail.timeline_created' => ({required Object date}) => 'Criado em ${date}',
			'lead_detail.timeline_qualified' => ({required Object date}) => 'Qualificado em ${date}',
			'lead_detail.timeline_buy_intent' => ({required Object date}) => 'Sinalizou intenção de compra em ${date}',
			'lead_detail.timeline_researched' => ({required Object date}) => 'Pesquisado em ${date}',
			'lead_detail.timeline_updated' => ({required Object date}) => 'Última atualização em ${date}',
			'lead_detail.queue_group_carteira' => 'Da sua carteira',
			'lead_detail.queue_group_sinal' => 'De sinal do radar',
			'lead_detail.queue_group_frio' => 'Frio, do território',
			'lead_detail.queue_frio_discard_all' => ({required Object count}) => 'Descartar os ${count}',
			'lead_detail.queue_frio_redraft_all' => 'Reescrever com o catálogo',
			'lead_detail.queue_frio_show_all' => ({required Object count}) => '+ ${count} iguais a estes · mostrar todos',
			'lead_detail.queue_batch_discard_title' => 'Descartar rascunhos frios',
			'lead_detail.queue_batch_discard_message' => ({required Object count}) => 'Descartar ${count} rascunhos da fila? Os leads vão para perdidos e nada é enviado.',
			'lead_detail.queue_batch_redraft_title' => 'Reescrever com o catálogo',
			'lead_detail.queue_batch_redraft_message' => ({required Object count}) => 'Reescrever ${count} rascunhos usando seu catálogo de produtos? Cada reescrita é uma chamada de IA cobrada na sua chave.',
			'lead_detail.queue_batch_discarded' => ({required Object count}) => '${count} rascunhos descartados.',
			'lead_detail.queue_batch_redrafted' => ({required Object count}) => '${count} rascunhos reescritos.',
			'lead_detail.queue_outcome_line' => ({required Object sent, required Object replies, required Object proposals}) => 'Esta semana: ${sent} enviadas · ${replies} respostas · ${proposals} propostas',
			'lead_detail.approve_scheduled' => ({required Object when}) => 'Limite de hoje esgotado — envio agendado para ${when}.',
			'lead_detail.queue_scheduled_badge' => ({required Object when}) => 'Agendada · sai ${when}',
			'lead_detail.route_strip_title' => ({required Object n, required Object cap}) => 'Envio ${n} de ${cap} de hoje',
			'lead_detail.route_strip_exhausted' => 'Limite de hoje esgotado — aprovações agendam para amanhã de manhã',
			'lead_detail.route_strip_sub' => ({required Object mailbox, required Object day, required Object total}) => '${mailbox} em aquecimento, dia ${day} de ${total} — aprovar além do limite agenda sozinho.',
			'lead_detail.queue_blocked_title' => 'Nada pode sair ainda',
			'lead_detail.queue_blocked_body' => 'As ofertas estão prontas, mas esta conta não tem canal conectado.',
			'lead_detail.queue_blocked_cta' => 'Conectar canal',
			'lead_detail.queue_tender_deadline_days' => ({required Object count}) => 'edital fecha em ${count}d',
			'lead_detail.queue_tender_deadline_today' => 'edital fecha hoje',
			'lead_detail.queue_tender_deadline_past' => 'edital encerrado',
			'lead_detail.queue_tender_value' => ({required Object value}) => 'estimado • ${value}',
			'lead_detail.queue_no_address_title' => 'Sem endereço para enviar',
			'lead_detail.queue_no_address_body' => 'Nenhum canal seu alcança este contato. Ache o e-mail de compras e preencha o cliente — a oferta já está pronta.',
			'lead_detail.queue_no_address_cta' => 'Abrir cliente',
			'lead_detail.queue_wa_send' => 'Mandar pelo meu WhatsApp',
			'lead_detail.queue_wa_hint' => 'abre no seu número, com o texto pronto',
			'lead_detail.queue_wa_confirm_title' => 'Você chegou a mandar?',
			'lead_detail.queue_wa_confirm_body' => 'Nada saiu por aqui — quem mandou foi você. Se mandou, tiro esta oferta da fila e marco o cliente como contactado.',
			'lead_detail.queue_wa_confirm_yes' => 'Mandei',
			'lead_detail.queue_wa_confirm_no' => 'Ainda não',
			'account_menu.profile' => 'Perfil',
			'account_menu.team' => 'Equipe',
			'account_menu.channels' => 'Canais',
			'account_menu.settings' => 'Configurações',
			'account_menu.logout' => 'Sair',
			'account_menu.title' => 'Conta',
			'copilot.empty_title' => 'O que você quer fazer?',
			'copilot.empty_body' => 'Peça em português. Eu executo e devolvo o link pra você conferir. Não envio nada sem sua aprovação.',
			'copilot.hint' => 'peça algo…',
			'copilot.thinking' => 'trabalhando…',
			'copilot.clear' => 'Limpar conversa',
			'copilot.sample_prospect' => 'Ache 20 empresas de [segmento] em [cidade]',
			'copilot.sample_drafts' => 'Gere abordagens para os leads novos do funil',
			'copilot.sample_report' => 'Como foi minha prospecção nos últimos 30 dias?',
			'copilot.failed' => 'Não consegui responder. Tente de novo.',
			'copilot.tool_list_funnels' => 'consultou seus funis',
			'copilot.tool_pipeline_overview' => 'leu os números do funil',
			'copilot.tool_list_approval_queue' => 'leu a fila de aprovação',
			'copilot.tool_draft_approaches' => 'gerou rascunhos de abordagem',
			'copilot.tool_case_report' => 'leu seu relatório do período',
			'copilot.thinking_long' => 'Ainda trabalhando. Buscas e rascunhos podem levar até 2 minutos.',
			'copilot.sample_prospect_audience' => ({required Object keyword, required Object location}) => 'Ache 20 ${keyword} em ${location}',
			'attachment.count' => ({required Object count}) => '${count} anexo(s)',
			'attachment.open' => 'Abrir',
			'attachment.download_failed' => 'Não consegui abrir o arquivo agora.',
			'attachment.status_clean' => 'Verificado pelo antivírus',
			'attachment.status_pending' => 'Ainda não verificado',
			'attachment.status_skipped' => 'Não verificado',
			'attachment.status_failed' => 'A verificação não completou',
			'attachment.status_infected' => 'Ameaça encontrada',
			'attachment.blocked_title' => 'Este arquivo não foi liberado',
			'attachment.blocked_pending' => 'O antivírus ainda não terminou de verificar. Tente de novo em alguns minutos.',
			'attachment.blocked_skipped' => 'Não há antivírus configurado nesta instalação, então ninguém verificou este arquivo. Ele veio de fora — abra por sua conta e risco.',
			'attachment.blocked_failed' => 'A verificação falhou. O sistema tenta de novo automaticamente; se continuar, chame o suporte.',
			'attachment.blocked_infected' => ({required Object signature}) => 'O antivírus identificou uma ameaça (${signature}). O arquivo não pode ser aberto. Se você acha que é engano, peça o arquivo por outro caminho.',
			'attachment.download_anyway' => 'Baixar mesmo assim',
			'attachment.download_anyway_confirm' => 'Ninguém verificou este arquivo. Ele foi enviado por alguém de fora da sua empresa. Abrir?',
			'attachment.why_blocked' => 'Por quê?',
			'attachment.view_original' => 'Ver original',
			'attachment.view_original_images' => 'Ver original com imagens',
			'attachment.view_original_images_warning' => 'Carregar as imagens avisa quem enviou que você abriu a mensagem. Continuar?',
			'sender_auth.badge_unknown' => 'Remetente não verificado',
			'sender_auth.badge_failed' => 'Remetente suspeito',
			'sender_auth.title_unknown' => 'Ninguém verificou este remetente',
			'sender_auth.title_failed' => 'Este remetente não passou na verificação',
			'sender_auth.body_unknown' => 'O servidor que recebeu esta mensagem não informou se o domínio autorizou o envio. Isso é comum com provedores antigos e não quer dizer que seja falso — só quer dizer que não dá para afirmar que é verdadeiro.',
			'sender_auth.body_failed' => 'O domínio que aparece no remetente NÃO autorizou este envio. Alguém pode estar se passando por essa empresa. Antes de responder dados, pagar boleto ou trocar conta bancária, confirme por telefone usando um número que você já tinha.',
			'sender_auth.checks' => ({required Object checks}) => 'Verificações: ${checks}',
			'email_inbox.no_subject' => '(sem assunto)',
			'email_inbox.empty' => 'Nenhum e-mail nesta thread ainda.',
			'email_inbox.you' => 'Você',
			'email_inbox.back_to_list' => 'Voltar aos e-mails',
			'email_inbox.loading_email' => 'Carregando e-mail…',
			'email_inbox.view_text' => 'Texto',
			'email_inbox.view_original' => 'Original',
			'email_risk.payment_title' => 'Atenção: dados de pagamento mudaram',
			'email_risk.payment_action' => 'Confirme por telefone antes de pagar, usando um número que você já tinha — não o que está neste e-mail.',
			'email_risk.quarantine_title' => 'Mensagem retida para conferência',
			'email_risk.attention_title' => 'Vale conferir esta mensagem',
			'email_risk.legit' => 'É legítimo',
			'email_risk.legit_confirm' => 'Você confirma que este remetente é confiável? Paramos de avisar sobre ele nesta conta.',
			'email_risk.legit_done' => 'Certo. Não vamos mais avisar sobre este remetente.',
			'email_risk.legit_failed' => 'Não consegui registrar agora.',
			'email_risk.scam' => 'É golpe',
			'email_risk.scam_title' => 'Reportar como golpe',
			'email_risk.scam_why' => 'Por que você acha que é golpe? (opcional)',
			'email_risk.scam_why_hint' => 'Ex.: esse fornecedor nunca manda boleto por e-mail',
			'email_risk.scam_explain' => 'Seu relato protege as outras empresas da rede. Quando duas empresas reportam o mesmo remetente, ele passa a ser bloqueado para todo mundo.',
			'email_risk.scam_done' => 'Reportado. Obrigado — isso protege as outras empresas da rede.',
			'brand.title' => 'Identidade da empresa',
			'brand.subtitle' => 'O que a IA sabe sobre vocês e o que ela nunca pode dizer. Vale para toda abordagem, de todo vendedor.',
			'brand.identity_section' => 'Identidade da empresa',
			'brand.company_name' => 'Nome da empresa',
			'brand.about' => 'O que vocês fazem',
			'brand.about_hint' => 'Ex.: distribuímos monitores multiparâmetro e bombas de infusão para clínicas do interior de SP',
			'brand.tone' => 'Como vocês falam',
			'brand.tone_hint' => 'Ex.: direto, sem jargão, trata por você',
			'brand.signature' => 'Assinatura padrão',
			'brand.allowed' => 'O que podemos afirmar',
			'brand.allowed_hint' => 'Coisas verdadeiras que ajudam a vender. Ex.: assistência técnica própria em Sorocaba',
			'brand.forbidden' => 'Nunca dizer',
			'brand.forbidden_hint' => 'A IA é impedida de escrever isso, e o texto é conferido antes de chegar na fila. Ex.: cura, aprovado pela ANVISA',
			'brand.add' => 'Adicionar',
			'brand.remove' => 'Remover',
			'brand.saved' => 'Identidade salva.',
			'brand.personal' => 'Seu jeito',
			'brand.personal_subtitle' => 'Some ao da empresa, não substitui. Você não consegue liberar o que a empresa proibiu.',
			'brand.tone_note' => 'Seu tom pessoal',
			'brand.tone_note_hint' => 'Ex.: falo mais devagar, sempre pergunto do time antes do preço',
			'brand.personal_signature' => 'Sua assinatura',
			'brand.personal_signature_hint' => 'Ex.: João Silva · (15) 99999-0000',
			'brand.personal_saved' => 'Seu jeito foi salvo.',
			'brand.differentiators' => 'Seus diferenciais',
			'brand.differentiators_hint' => 'Por que o seu equipamento, e não o que o hospital já tem. Escrito por você — a IA usa o que couber e não inventa nenhum. Vazio, a proposta argumenta só pelo produto.',
			'suppressions.title' => 'Endereços bloqueados',
			'suppressions.subtitle' => 'Endereços que não recebem mais e-mails seus: houve rejeição permanente ou marcação de spam. Insistir neles é o que queima a reputação do seu domínio.',
			'suppressions.empty' => 'Nenhum endereço bloqueado',
			'suppressions.empty_hint' => 'Tudo que for rejeitado em definitivo aparece aqui, com o motivo que o servidor de destino deu.',
			'suppressions.release' => 'Liberar',
			'suppressions.release_title' => 'Liberar este endereço?',
			'suppressions.release_warning' => 'Um servidor de e-mail recusou este endereço, ou o destinatário marcou como spam. Só libere se souber que o problema foi resolvido.',
			'suppressions.released' => 'Endereço liberado',
			'suppressions.reason_hard_bounce' => 'Rejeição permanente — o endereço não existe',
			'suppressions.reason_complaint' => 'Marcado como spam pelo destinatário',
			'signals.title' => 'Sinais de compra',
			'signals.subtitle' => 'Licitações públicas que combinam com o que você vende. Alguém escreveu o que pretende comprar e até quando — prazo mais próximo primeiro.',
			'signals.empty_no_watch' => 'Nenhum monitoramento configurado',
			'signals.empty_no_watch_hint' => 'Diga quais palavras acompanhar e os editais abertos aparecem aqui sozinhos.',
			'signals.empty_quiet' => 'Nada aberto no momento',
			'signals.empty_quiet_hint' => 'Consultamos as licitações públicas várias vezes por dia. Os novos aparecem aqui assim que são publicados.',
			'signals.days_left' => ({required Object days}) => 'faltam ${days} dias',
			'signals.closes_today' => 'encerra hoje',
			'signals.value_undisclosed' => 'valor não divulgado',
			'signals.open_notice' => 'Ler o edital',
			'signals.dismiss' => 'Não é para nós',
			'signals.tab' => 'Sinais',
			'signals.watch_cta' => 'Escolher palavras',
			'signals.watch_dialog_title' => 'O que você quer acompanhar',
			'signals.watch_dialog_body' => 'Palavras que aparecem no que você vende. Separe por vírgula — buscamos cada uma nos editais publicados.',
			'signals.watch_hint' => 'ex: material hospitalar, seringa, luva de procedimento',
			'signals.watch_states_label' => 'Onde você entrega',
			'signals.watch_states_hint' => 'ex: MT, MS',
			'signals.watch_states_error' => 'Informe pelo menos um estado (UF).',
			'signals.watch_states_body' => 'Só buscamos editais dos estados que você atende — sem isso a busca varre o país inteiro e traz o que você não pode entregar.',
			'signals.watch_saved' => 'Pronto. Os editais que combinarem aparecem aqui sozinhos.',
			'signals.watch_empty_error' => 'Escreva pelo menos uma palavra.',
			'signals.watch_current' => ({required Object words}) => 'Acompanhando: ${words}',
			'signals.no_catalog_banner' => 'Nenhum produto cadastrado — estes resultados vêm de palavras digitadas à mão, não do que você vende. Importe seu catálogo e o monitoramento se monta sozinho a partir dele.',
			'signals.no_catalog_cta' => 'Cadastrar produtos',
			'signals.watch_origin_catalog' => 'do seu catálogo',
			'signals.watch_origin_manual' => 'definido manualmente',
			'sales_record.title' => 'Registrar venda',
			'sales_record.gross' => 'Valor total',
			'sales_record.gross_hint' => 'o que o cliente paga',
			'sales_record.cost' => 'Custo da mercadoria',
			'sales_record.cost_hint' => 'o que te custou — a comissão sai da margem, não da nota',
			'sales_record.tax' => 'Imposto',
			'sales_record.tax_hint' => 'quanto de imposto sai desta venda, em reais',
			'sales_record.cycle' => 'Ciclo de recompra (dias)',
			'sales_record.cycle_hint' => 'de quanto em quanto tempo esse cliente recompra — deixe vazio se não souber',
			'sales_record.notes' => 'Observações',
			'sales_record.net' => 'Você fica com',
			'sales_record.underwater_warning' => 'Esta venda custa mais do que traz. Pode registrar assim mesmo — só confira os números.',
			'sales_record.submit' => 'Registrar venda',
			'sales_record.saved' => 'Venda registrada',
			'sales_record.your_commission' => 'Sua comissão nesta venda',
			'portfolio.title' => 'Carteira',
			'portfolio.subtitle' => 'Seus clientes — importados ou conquistados. Ordenado por quem precisa ouvir você primeiro, não por ordem alfabética.',
			'portfolio.empty' => 'Nenhum cliente ainda',
			'portfolio.empty_hint' => 'Importe sua planilha, ou registre uma venda num lead, e eles aparecem aqui — com a previsão de quando vão precisar de novo.',
			'portfolio.purchases' => ({required Object count}) => '${count} compra(s)',
			'portfolio.ran_out' => ({required Object days}) => 'acabou há ${days} dias',
			'portfolio.runs_out_today' => 'acaba hoje',
			'portfolio.runs_out_in' => ({required Object days}) => 'acaba em ${days} dias',
			'portfolio.silent_for' => ({required Object days}) => 'sem contato há ${days} dias',
			'portfolio.history' => 'Histórico de compras',
			'portfolio.sale_without_items' => 'Venda sem itens detalhados',
			'cross_sell.title' => 'O que mais oferecer',
			'cross_sell.because' => ({required Object item}) => 'porque compra ${item}',
			'cross_sell.evidence' => ({required Object count, required Object item}) => '${count} clientes que compram ${item} também levam',
			'cross_sell.empty' => 'Ainda não há padrão de co-compra suficiente nas vendas desta conta.',
			'cross_sell.learned_here' => 'Aprendido só com as vendas desta conta.',
			'relationship.title' => 'Datas de relacionamento',
			'relationship.subtitle' => 'Aniversários, aberturas de empresa, datas do setor e manutenções. No dia, o rascunho vai para a fila de aprovação.',
			'relationship.empty' => 'Nenhuma data cadastrada',
			'relationship.empty_hint' => 'Cadastre um aniversário ou uma data do setor e o sistema lembra por você.',
			'relationship.add' => 'Nova data',
			'relationship.kind_birthday' => 'Aniversário',
			'relationship.kind_founded' => 'Aniversário da empresa',
			'relationship.kind_sector' => 'Data do setor',
			'relationship.kind_maintenance' => 'Manutenção preventiva',
			'relationship.kind_custom' => 'Outra',
			'relationship.sector_hint' => 'Escolha uma data profissional',
			'relationship.label' => 'Descrição',
			'relationship.day' => 'Dia',
			'relationship.month' => 'Mês',
			'relationship.account_wide' => 'Vale para a conta inteira — aparece aqui, mas não vira mensagem sozinha.',
			'relationship.delete_confirm' => 'Esquecer esta data?',
			'relationship.saved' => 'Data salva',
			'relationship.greeting_only' => 'Só saudações podem sair sem aprovação, e apenas com o autopilot ligado.',
			'command_center.tile_meetings' => 'Reuniões',
			'command_center.tile_won' => 'Ganhos',
			'command_center.funnel_title' => 'Funil',
			'command_center.more_actions' => ({required Object count}) => '+${count}',
			'command_center.partial_load' => 'Parte da lista não carregou — pode haver mais coisas esperando.',
			'command_center.snooze' => 'Depois',
			'command_center.snoozed' => ({required Object days}) => 'Adiado. Volta em ${days} dias.',
			'command_center.kind_approve_draft' => 'Abordagem esperando aprovação',
			'command_center.kind_unowned_lead' => 'Lead sem responsável',
			'command_center.kind_signature_viewed' => 'Abriu o contrato e não assinou',
			'command_center.kind_contract_alert' => 'Prazo de contrato',
			'command_center.kind_confirm_sale' => 'Venda esperando confirmação',
			'command_center.kind_reorder_due' => 'Já deveria ter recomprado',
			'command_center.kind_waiting_reply' => 'Cliente esperando resposta',
			'command_center.kind_unknown' => 'Precisa de atenção',
			'command_center.overdue_days' => ({required Object count}) => 'venceu há ${count} dia(s)',
			'command_center.due_today' => 'vence hoje',
			'command_center.due_in_days' => ({required Object count}) => 'vence em ${count} dia(s)',
			'command_center.waiting_days' => ({required Object count}) => 'esperando há ${count} dia(s)',
			'command_center.waiting_today' => 'chegou hoje',
			'command_center.activation_title' => 'Sua primeira venda começa com um lead',
			'command_center.activation_body' => 'Ainda não há nada no funil. Traga empresas para prospectar ou acompanhe os sinais públicos de quem já disse que quer comprar.',
			'command_center.activation_signals' => 'Ver sinais públicos',
			'command_center.months.0' => 'janeiro',
			'command_center.months.1' => 'fevereiro',
			'command_center.months.2' => 'março',
			'command_center.months.3' => 'abril',
			'command_center.months.4' => 'maio',
			'command_center.months.5' => 'junho',
			'command_center.months.6' => 'julho',
			'command_center.months.7' => 'agosto',
			'command_center.months.8' => 'setembro',
			'command_center.months.9' => 'outubro',
			'command_center.months.10' => 'novembro',
			'command_center.months.11' => 'dezembro',
			'command_center.month_of_year' => ({required Object month, required Object year}) => '${month} de ${year}',
			'command_center.tile_untouched' => 'Nunca abordados',
			'command_center.tile_untouched_footnote' => 'no seu território',
			'command_center.untouched_target_row' => ({required Object count, required Object unit}) => '${count} ${unit} do que você vende, e nunca foi abordado',
			'command_center.kind_untouched_target' => 'Nunca abordado',
			'command_center.action_see_target' => 'Ver no radar',
			'ai_debounce.waiting' => ({required Object seconds}) => 'A IA responde em ~${seconds}s, aguardando o cliente terminar',
			'ai_debounce.answering' => 'Respondendo…',
			'ai_debounce.collapsed' => ({required Object count}) => '${count} mensagens serão respondidas juntas',
			'ai_debounce.answer_now' => 'Responder agora',
			'overnight.title' => 'Seu SDR trabalhou de madrugada',
			'overnight.drafted' => ({required Object count}) => '${count} abordagens prontas para revisar',
			'overnight.budget_hit' => ({required Object count}) => '${count} não couberam na franquia do mês',
			'overnight.review' => 'Revisar',
			'overnight.dismiss' => 'Ok',
			'overnight.blocked_title' => 'A caça de hoje não rodou',
			'overnight.blocked_body' => 'Nenhuma oferta foi escrita esta noite. O motivo:',
			'overnight.blocked_cta' => 'Resolver',
			'export.tooltip' => 'Exportar',
			'export.xlsx' => 'Exportar para Excel (.xlsx)',
			'export.csv' => 'Exportar para CSV',
			'export.audited' => 'Toda exportação fica registrada',
			'export.done' => ({required Object rows}) => '${rows} linhas exportadas',
			'export.truncated' => ({required Object rows}) => '${rows} linhas — o arquivo bateu no teto e não é a lista inteira',
			'my_customers.title' => 'Seus clientes de sempre',
			'my_customers.subtitle' => 'Lemos só quem você escreveu e quando — nunca o conteúdo das mensagens.',
			'my_customers.scan_action' => 'Buscar na minha caixa',
			'my_customers.scanning' => 'Lendo sua pasta de enviados…',
			'my_customers.scan_result' => ({required Object scanned, required Object mailbox, required Object found}) => 'Li ${scanned} mensagens em ${mailbox} e encontrei ${found} pessoas.',
			'my_customers.scan_skipped' => ({required Object count}) => '${count} endereços de robô e o seu próprio ficaram de fora.',
			'my_customers.scan_partial' => 'Paramos antes do fim da pasta, então essa não é a lista inteira. Busque de novo para continuar.',
			'my_customers.scan_failed' => ({required Object error}) => 'Não conseguimos ler sua pasta de enviados: ${error}',
			'my_customers.empty_title' => 'Nada para revisar ainda.',
			'my_customers.empty_hint' => 'Busque na sua caixa para ver quem você já atende.',
			'my_customers.sent_count' => ({required Object count}) => '${count} e-mails enviados',
			'my_customers.since' => ({required Object date}) => 'desde ${date}',
			'my_customers.last' => ({required Object date}) => 'último em ${date}',
			'my_customers.funnel_label' => 'Entram no funil',
			'my_customers.funnel_required' => 'Escolha o funil antes de importar.',
			'my_customers.no_funnels' => 'Crie um funil primeiro — é onde os clientes vão entrar.',
			'my_customers.selected' => ({required Object count}) => '${count} selecionados',
			'my_customers.import_action' => 'Importar selecionados',
			'my_customers.dismiss_action' => 'Não são clientes',
			'my_customers.imported' => ({required Object count}) => '${count} clientes importados.',
			'my_customers.imported_partial' => ({required Object count, required Object skipped}) => '${count} importados. ${skipped} já estavam no sistema.',
			'my_customers.dismissed' => ({required Object count}) => '${count} fora da lista.',
			'my_customers.select_all' => 'Selecionar todos',
			'my_customers.clear_selection' => 'Limpar',
			'my_customers.showing_partial' => ({required Object shown, required Object total}) => 'Mostrando ${shown} de ${total}.',
			'my_customers.view_label' => 'Na minha caixa',
			'my_customers.open_inbox_customers' => 'Ver clientes da minha caixa',
			'proposal.title' => 'Nova proposta',
			'proposal.pick_products' => 'Escolha o que você está oferecendo e quantos.',
			'proposal.total' => 'Total',
			'proposal.mixed_currency' => 'Estes produtos estão em moedas diferentes. Uma proposta carrega uma moeda — senão o total seria um número que ninguém consegue defender.',
			'proposal.body_label' => 'Sua mensagem',
			'proposal.body_hint' => 'Opcional. As linhas com preço entram abaixo do que você escrever.',
			'proposal.send_for_approval' => 'Enviar para aprovação',
			'proposal.approval_note' => 'Vai para a fila de aprovação. Nada chega ao cliente até você aprovar lá.',
			'proposal.queued_title' => 'Na fila de aprovação. É isto que quem revisar vai ler:',
			'proposal.history' => 'JÁ OFERECIDO',
			'proposal.status_draft' => 'Rascunho',
			'proposal.status_sent' => ({required Object date}) => 'Enviada em ${date}',
			'proposal.no_products' => 'Nenhum produto ainda',
			'proposal.no_products_hint' => 'Uma proposta é feita do que você vende. Cadastre um produto e volte.',
			'proposal.go_to_products' => 'Cadastrar produto',
			'proposal.open' => 'Fazer proposta',
			'proposal.download_pdf' => 'Baixar PDF',
			'proposal.download_failed' => 'Não consegui salvar o arquivo.',
			'proposal.download_hint' => 'Um arquivo para mandar agora, por WhatsApp ou e-mail. Não envia nada sozinho.',
			'proposal.suggested_hint' => 'Já preenchi o que combina com este cliente. Mude a quantidade, tire o que não serve, acrescente o que faltar.',
			'product_import.title' => 'Importar lista de preços',
			'product_import.what_we_read' => 'CSV com linha de títulos (nome/produto, preço/valor, descrição, moeda) ou PDF de catálogo do fabricante — a IA lê o PDF e você confere linha por linha antes de criar nada. Pode escolher vários arquivos de uma vez.',
			'product_import.reading_progress' => ({required Object done, required Object total}) => 'Lendo ${done} de ${total}…',
			'product_import.files_failed' => ({required Object count, required Object reason}) => '${count} arquivo(s) não puderam ser lidos: ${reason}',
			'product_import.files_read' => ({required Object count}) => '${count} arquivos lidos',
			'product_import.row_from' => ({required Object file}) => 'de ${file}',
			'product_import.pick_file' => 'Escolher arquivo',
			'product_import.pick_another' => 'Escolher outro arquivo',
			'product_import.found' => ({required Object total, required Object importable}) => '${total} linhas lidas, ${importable} prontas para importar',
			'product_import.with_problems' => ({required Object count}) => '${count} precisam de um olhar',
			'product_import.already_here' => ({required Object count}) => '${count} já estão no seu catálogo',
			'product_import.confirm' => ({required Object count}) => 'Importar ${count} produto(s)',
			'product_import.nothing_importable' => 'Nada a importar deste arquivo ainda — corrija as linhas acima e escolha o arquivo de novo.',
			'product_import.line_problem' => ({required Object line, required Object problem}) => 'linha ${line} — ${problem}',
			'product_import.line_price' => ({required Object line, required Object price}) => 'linha ${line} — ${price}',
			'product_import.blank_name' => '(sem nome)',
			'product_import.imported' => ({required Object count}) => '${count} produtos importados.',
			'product_import.imported_partial' => ({required Object count, required Object skipped}) => '${count} importados, ${skipped} ignorados.',
			'product_import.open' => 'Importar planilha',
			'product_import.what_we_read_pdf' => 'Ou envie o folheto do fabricante em PDF — a IA lê e você revisa cada linha antes de qualquer coisa ser criada.',
			'product_import.pick_pdf' => 'Importar folheto (PDF)',
			'product_import.pdf_reading' => 'Lendo o folheto com IA — pode levar até um minuto…',
			'product_import.page_problem' => ({required Object page, required Object problem}) => 'pág. ${page} — ${problem}',
			'product_import.page_price' => ({required Object page, required Object price}) => 'pág. ${page} — ${price}',
			'product_import.price_missing' => 'preço não lido — preencha depois',
			'customer_import.title' => 'Importar lista de clientes',
			'customer_import.what_we_read' => 'Um CSV com linha de títulos. Ele lê as colunas empresa/cliente, contato/nome, telefone/celular e e-mail — separado por vírgula ou ponto e vírgula, como sua planilha salvou. Cada cliente precisa de telefone ou e-mail.',
			'customer_import.pick_file' => 'Escolher arquivo',
			'customer_import.pick_another' => 'Escolher outro arquivo',
			'customer_import.found' => ({required Object total, required Object importable}) => '${total} linhas lidas, ${importable} prontas para importar',
			'customer_import.with_problems' => ({required Object count}) => '${count} precisam de um olhar',
			'customer_import.already_here' => ({required Object count}) => '${count} já estão na sua base',
			'customer_import.confirm' => ({required Object count}) => 'Importar ${count} cliente(s)',
			'customer_import.nothing_importable' => 'Nada a importar deste arquivo ainda — corrija as linhas acima e escolha o arquivo de novo.',
			'customer_import.line_problem' => ({required Object line, required Object problem}) => 'linha ${line} — ${problem}',
			'customer_import.line_address' => ({required Object line, required Object address}) => 'linha ${line} — ${address}',
			'customer_import.blank_name' => '(sem nome)',
			'customer_import.imported' => ({required Object count}) => '${count} clientes importados.',
			_ => null,
		} ?? switch (path) {
			'customer_import.imported_partial' => ({required Object count, required Object skipped}) => '${count} importados, ${skipped} ignorados.',
			'customer_import.open' => 'Importar planilha',
			'sales_import.title' => 'Importar histórico de vendas',
			'sales_import.open' => 'Importar vendas',
			'sales_import.settings_subtitle' => 'Traga as vendas passadas para alimentar recompra e cross-sell',
			'sales_import.hint' => 'Um CSV com linha de títulos. Ele lê as colunas cliente, produto, data, valor e quantidade — separado por vírgula ou ponto e vírgula, como sua planilha salvou. Os clientes precisam já estar na sua base: importe a lista de clientes primeiro.',
			'sales_import.pick_file' => 'Escolher arquivo',
			'sales_import.pick_another' => 'Escolher outro arquivo',
			'sales_import.preview_title' => 'O que o arquivo diz',
			'sales_import.found' => ({required Object total, required Object importable}) => '${total} linhas lidas, ${importable} prontas para registrar',
			'sales_import.problems_title' => ({required Object count}) => '${count} precisam de um olhar',
			'sales_import.already_recorded' => ({required Object count}) => '${count} já estão registradas',
			'sales_import.unknown_customers' => ({required Object count}) => '${count} citam clientes fora da sua base — importe a lista de clientes primeiro',
			'sales_import.confirm' => ({required Object count}) => 'Registrar ${count} venda(s)',
			'sales_import.nothing_importable' => 'Nada a registrar deste arquivo ainda — corrija as linhas acima e escolha o arquivo de novo.',
			'sales_import.line_problem' => ({required Object line, required Object problem}) => 'linha ${line} — ${problem}',
			'sales_import.line_detail' => ({required Object line, required Object detail}) => 'linha ${line} — ${detail}',
			'sales_import.blank_customer' => '(sem cliente)',
			'sales_import.result' => ({required Object count}) => '${count} vendas registradas.',
			'sales_import.skipped' => ({required Object count, required Object skipped}) => '${count} vendas registradas, ${skipped} puladas.',
			'sales_import.view_portfolio' => 'Ver carteira',
			'lead_timeline.title' => 'CONVERSA',
			'lead_timeline.empty' => 'Nada foi enviado para este cliente ainda.',
			'lead_timeline.truncated' => 'Mostrando as mensagens mais recentes.',
			'lead_timeline.from_customer' => 'Cliente',
			'lead_timeline.from_team' => 'Nós',
			'lead_timeline.from_bot' => 'Bot',
			'lead_timeline.attachment' => ({required Object kind}) => '[${kind}]',
			'lead_timeline.not_delivered' => 'Não entregue',
			'lead_timeline.reply_hint' => 'Escreva sua resposta…',
			'lead_timeline.reply_send' => 'Responder',
			'lead_timeline.reply_sent' => 'Resposta enviada.',
			'lead_timeline.no_conversation' => 'Responder abre quando houver conversa — a primeira oferta sai pela fila.',
			'email_composer.subject_hint' => 'Assunto',
			'email_composer.body_hint' => 'Escreva seu e-mail…',
			'email_composer.send' => 'Enviar',
			'email_composer.reply' => 'Responder',
			'radar.tab' => 'Radar',
			'radar.tab_hoje' => 'Hoje',
			'radar.warnings' => ({required Object sources}) => 'Não consegui ler: ${sources}. O resto do feed está no ar.',
			'radar.source_down' => ({required Object source, required Object ago}) => 'Não conseguimos ler o ${source} há ${ago}. O que aparece abaixo é da última coleta — pode não ser o que está aberto agora.',
			'radar.source_down_never' => ({required Object source}) => 'Ainda não conseguimos ler o ${source}. Nada nesta lista veio dele.',
			'radar.ago_minutes' => ({required Object n}) => '${n} min',
			'radar.ago_hours' => ({required Object n}) => '${n} h',
			'radar.ago_days' => ({required Object n}) => '${n} dias',
			'radar.filter_all' => 'Todas',
			'radar.type_licitacao' => 'Licitações',
			'radar.type_parque' => 'Parque instalado',
			'radar.type_recompra' => 'Recompras',
			'radar.type_data' => 'Datas',
			'radar.empty' => 'Nada no radar agora',
			'radar.empty_hint' => 'Licitações, parque instalado no CNES, recompras vencendo e datas de relacionamento aparecem aqui conforme forem detectadas.',
			'radar.offer_queued' => 'Rascunho na fila — esperando sua aprovação em Hoje.',
			'radar.deadline_today' => 'hoje',
			'radar.deadline_days' => ({required Object count}) => 'em ${count}d',
			'radar.open_draft' => 'Abrir rascunho',
			'radar.prepare_offer' => 'Preparar oferta',
			'radar.open_customer' => 'Ver cliente',
			'radar.hunt_title' => 'Abastecer o território',
			'radar.hunt_hint' => 'Busca no CNES quem declara o equipamento que você atende — dado público, quantidade, nunca idade do aparelho. Telefone e endereço vêm do cadastro CNES.',
			'radar.hunt_placeholder' => 'ex.: tomógrafo, ventilador, ultrassom',
			'radar.hunt_empty' => 'Nada encontrado.',
			'radar.hunt_units' => ({required Object count}) => '${count} declarados no CNES',
			'radar.hunt_known' => 'já está nos seus contatos',
			'radar.type_cross_sell' => 'Venda casada',
			'radar.carteira_hint' => 'Recompra, cross-sell e datas aparecem depois da primeira venda registrada.',
			'radar.empty_hint_prospecting' => 'Licitações e parque instalado no CNES aparecem aqui conforme forem detectados.',
			'radar.footer_note' => 'Tudo que virar oferta cai em Hoje para sua aprovação — nada é enviado daqui.',
			'radar.watching_catalog' => ({required Object count}) => 'catálogo (${count} produtos)',
			'radar.week_prefix' => 'Esta semana:',
			'radar.week_sent' => 'enviadas',
			'radar.week_replies' => 'respostas',
			'radar.week_proposals' => 'propostas',
			'radar.data_month' => ({required Object month}) => 'dados CNES de ${month}',
			'radar.hunt_scope_cities' => ({required Object cities}) => 'no seu território (${cities})',
			'radar.hunt_scope_uf' => ({required Object uf}) => 'em ${uf}',
			'radar.auto_hunt_cta' => 'Caçar com meu catálogo',
			'radar.auto_hunt_hint' => 'Ele lê seus produtos e traz os maiores hospitais do território que você nunca abordou. Nada é enviado.',
			'radar.auto_hunt_result' => ({required Object created, required Object found}) => '${created} novos leads no funil, de ${found} encontrados',
			'radar.auto_hunt_none' => 'Nenhum estabelecimento novo no seu território agora',
			'radar.park_universe' => ({required Object total, required Object scope, required Object shown}) => '${total} hospitais ${scope} declaram equipamento que seu catálogo atende. Mostrando os ${shown} maiores.',
			'radar.offer_more' => ({required Object count}) => '+${count} outros',
			'radar.type_contrato' => 'Contrato vencendo',
			'radar.type_ata' => 'Ata de registro',
			'radar.edge_before_tender' => 'antes do edital',
			'radar.edge_no_tender' => 'sem licitação',
			'radar.read_and_send' => 'Ler e enviar',
			'radar.back_to_panel' => 'Voltar ao painel',
			'radar.panel_object' => 'O que estão comprando',
			'radar.panel_buyer' => 'Quem está comprando',
			'radar.panel_reference' => 'Número no registro público',
			'radar.panel_published' => 'Publicado em',
			'radar.panel_deadline' => 'Prazo',
			'radar.panel_value' => 'Valor',
			'radar.panel_no_link' => 'Este aviso não publicou link nem arquivo. Copie o número acima e busque no PNCP — é a única referência que existe.',
			'radar.panel_park_items' => 'O que este hospital declara',
			'radar.panel_offer_title' => 'O que você pode oferecer',
			'radar.panel_close' => 'Fechar',
			'radar.panel_copy_reference' => 'Copiar número',
			'radar.panel_copied' => 'Número copiado',
			'radar.panel_empty' => 'Toque numa oportunidade para ver tudo o que sabemos sobre ela.',
			'radar.panel_portal' => 'PNCP — Portal Nacional de Contratações Públicas',
			'radar.panel_open_portal' => 'Abrir o PNCP',
			'radar.panel_buyer_known' => 'Você já vendeu para este comprador',
			'radar.panel_buyer_cold' => 'Este comprador nunca ouviu falar de você',
			'radar.panel_history' => 'O que este órgão já contratou',
			'radar.panel_history_count' => ({required Object n}) => 'Este comprador tem ${n} contrato(s) do seu ramo no registro público.',
			'radar.panel_history_none' => 'Nenhum contrato do seu ramo no registro público deste comprador.',
			'radar.panel_buyer_contacted' => 'Você já enviou uma oferta para este comprador',
			'radar.panel_buyer_queued' => 'Oferta escrita, esperando sua aprovação — nada enviado',
			'radar.panel_buyer_listed' => 'Já está no seu quadro — nada escrito ainda',
			'radar.panel_reach' => 'Como falar com eles',
			'radar.panel_stage' => ({required Object stage}) => 'No quadro, em: ${stage}',
			'radar.panel_open_board' => 'Abrir no quadro',
			'radar.panel_purchaser' => 'Quem compra por eles',
			'radar.panel_history_via' => ({required Object org, required Object n}) => 'A ${org}, que compra por eles, tem ${n} contrato(s) do seu ramo no registro público.',
			'radar.panel_beds' => 'Leitos que opera',
			'radar.panel_habilitations' => 'Habilitado pelo Ministério',
			'radar.panel_habilitation_beds' => ({required Object name, required Object n}) => '${name} — ${n} leito(s) habilitado(s)',
			'radar.panel_services' => 'Serviços que realiza',
			'start.greeting_morning' => ({required Object name}) => 'Bom dia, ${name}',
			'start.greeting_afternoon' => ({required Object name}) => 'Boa tarde, ${name}',
			'start.greeting_evening' => ({required Object name}) => 'Boa noite, ${name}',
			'start.date_line' => ({required Object weekday, required Object day, required Object month}) => '${weekday}, ${day} de ${month}',
			'start.weekdays.0' => 'segunda',
			'start.weekdays.1' => 'terça',
			'start.weekdays.2' => 'quarta',
			'start.weekdays.3' => 'quinta',
			'start.weekdays.4' => 'sexta',
			'start.weekdays.5' => 'sábado',
			'start.weekdays.6' => 'domingo',
			'start.summary_ready' => ({required Object count}) => '${count} oferta(s) pronta(s)',
			'start.summary_waiting' => ({required Object count}) => '${count} cliente(s) esperando',
			'start.summary_due_today' => ({required Object count}) => '${count} vencendo hoje',
			'start.summary_due_none' => 'nada vencendo hoje',
			'start.review_n' => ({required Object count}) => 'Revisar as ${count}',
			'start.see_radar' => 'Ver Radar',
			'start.action_reply' => 'Responder',
			'start.action_assume' => 'Assumir',
			'start.distribute_team' => '✦ Distribuir entre a equipe',
			'start.sorted_by_urgency' => 'ordenado por urgência',
			'start.filter_all' => ({required Object count}) => 'Todos ${count}',
			'start.rail_radar_title' => 'Radar hoje',
			'start.rail_radar_open' => 'abrir →',
			'start.rail_week_title' => 'Sua semana',
			'start.rail_engine_title' => 'Ligue o motor',
			'start.of_total' => ({required Object current, required Object total}) => '${current} de ${total}',
			'start.register_sale_link' => 'registrar uma venda',
			'start.unowned_footnote' => ({required Object count}) => '${count} sem dono',
			'calendar.tab' => 'Vencimentos',
			'calendar.title' => 'O que vence',
			'calendar.subtitle' => ({required Object days}) => 'Contratos públicos do seu estado que acabam nos próximos ${days} dias. Quando um vence, a compra tem que acontecer de novo — e isso você sabe meses antes de sair edital.',
			'calendar.empty_title' => 'Nada vencendo nesta janela',
			'calendar.empty_body' => ({required Object days}) => 'Nenhum contrato do que você vende acaba nos próximos ${days} dias no seu estado.',
			'calendar.scanning_title' => 'Ainda lendo os contratos do seu território',
			'calendar.scanning_body' => 'Consultamos o portal hospital por hospital, sem atropelar o servidor deles. Vai enchendo ao longo dos próximos dias.',
			'calendar.no_territory_title' => 'Defina onde você vende',
			'calendar.no_territory_body' => 'Sem estado não dá para saber quais contratos te interessam. Ajuste em Configurações → Empresa.',
			'calendar.no_keywords_title' => 'Falta dizer o que você vende',
			'calendar.no_keywords_body' => 'O calendário usa as mesmas palavras do Radar. Cadastre seus produtos ou ajuste o monitoramento.',
			'calendar.days_left' => ({required Object count}) => 'vence em ${count}d',
			'calendar.days_today' => 'vence hoje',
			'calendar.days_past' => ({required Object count}) => 'venceu há ${count}d',
			'calendar.holder' => ({required Object name}) => 'hoje com ${name}',
			'calendar.holder_unknown' => 'fornecedor atual não publicado',
			'calendar.price_registry' => 'ata de registro de preços',
			'calendar.rides_others' => 'compra por adesão',
			'calendar.rides_others_hint' => 'Este órgão pega carona em ata de outro. Dá para vender sem ganhar licitação.',
			'calendar.open_record' => 'Ver no portal',
			'calendar.value_unknown' => 'valor não publicado',
			'calendar.work' => 'Trabalhar este',
			'calendar.worked' => ({required Object name}) => '${name} entrou no seu funil.',
			'calendar.work_failed' => 'Não consegui colocar no funil.',
			'calendar.open_funnel' => 'Ver no funil',
			'calendar.work_queued' => 'Entrou na sua lista de clientes. Falta o contato do comprador.',
			_ => null,
		};
	}
}
