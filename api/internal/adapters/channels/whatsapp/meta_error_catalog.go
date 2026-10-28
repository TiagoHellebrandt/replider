package whatsapp

import "errors"

// MetaAction names the high-level Cloud API operation an error came
// from. Lets the catalog disambiguate generic Meta codes (most
// notably the bare 100 / Subcode=0 / "Invalid parameter" envelope
// that every endpoint can emit) into action-specific app codes the
// dashboard can humanize and the catalog can document.
type MetaAction string

const (
	ActionUnknown          MetaAction = ""
	ActionVerifyCode       MetaAction = "verify_code"
	ActionRegisterPhone    MetaAction = "register_phone"
	ActionRequestCode      MetaAction = "request_code"
	ActionDeregisterPhone  MetaAction = "deregister_phone"
	ActionSubscribeApp     MetaAction = "subscribe_app"
	ActionListSubscribed   MetaAction = "list_subscribed_apps"
	ActionListPhoneNumbers MetaAction = "list_phone_numbers"
	ActionGetPhoneStatus   MetaAction = "phone_status"
	ActionGetProfile       MetaAction = "get_profile"
	ActionUpdateProfile    MetaAction = "update_profile"
	ActionUploadPicture    MetaAction = "upload_picture"
	ActionUploadMedia      MetaAction = "upload_media"
	ActionCreateTemplate   MetaAction = "create_template"
	ActionDeleteTemplate   MetaAction = "delete_template"
	ActionListTemplates    MetaAction = "list_templates"
	ActionSendTemplate     MetaAction = "send_template"
	ActionSendMessage      MetaAction = "send_message"
	ActionTestChannel      MetaAction = "test_channel"
)

// CatalogEntry is one rule in the Meta error catalog. Any field left
// at its zero value is treated as "match anything" (so a Code-only
// entry matches every subcode / action). UserMsgSubstring is matched
// case-insensitively against both UserMsg and Message — Meta's
// localized user_msg sometimes carries the real reason while the
// top-level message stays generic ("Invalid parameter").
//
// Order matters: Resolve walks the slice top-down and returns the
// first match. Put more specific entries (action-scoped, subcode-
// pinned, user_msg-pinned) before generic Code-only entries.
type CatalogEntry struct {
	Code             int
	Subcode          int
	UserMsgSubstring string
	Action           MetaAction
	Kind             string
	// DefaultMessage is what the API surfaces to the dashboard when
	// Meta did not provide a useful localized user_msg. Pre-translated
	// to pt-BR because the dashboard is pt-BR by default; the Flutter
	// WhatsAppError humanizer is the only consumer that overrides
	// this with a Slang string.
	DefaultMessage string
}

// metaErrorCatalog drives both MetaError.Kind() (action-agnostic) and
// the per-handler mapMetaErrorAction helper. Keeping the rules in one
// slice — instead of scattering them across switch statements in
// Kind(), ClassifyRegisterError(), and individual handlers — means a
// new Meta code added by the team only needs one diff.
var metaErrorCatalog = []CatalogEntry{
	// ── Register / verify flow (most error-prone surface) ──────────────
	{
		Code:             100,
		Subcode:          2388001,
		UserMsgSubstring: "verify ownership",
		Kind:             "WA_OWNERSHIP_NOT_VERIFIED",
		DefaultMessage:   "A Meta ainda não verificou a posse deste número. Use o fluxo SMS/voz para verificar antes de registrar.",
	},
	{
		Code:           100,
		Subcode:        2388001,
		Kind:           "WA_CERT_2FA_BLOCKED",
		DefaultMessage: "A Meta bloqueou o registro porque a verificação em duas etapas (2FA) do WABA está ativa. Desligue em Meta Business → Configurações da conta → Verificação em duas etapas.",
	},
	{
		Code:           100,
		Action:         ActionVerifyCode,
		Kind:           "WA_VERIFY_CODE_INVALID",
		DefaultMessage: "Código incorreto, expirado ou já usado. Solicite um novo código por SMS ou voz.",
	},
	{
		Code:           100,
		Action:         ActionRequestCode,
		Kind:           "WA_REQUEST_CODE_INVALID",
		DefaultMessage: "Não foi possível enviar o código. Verifique se o número aceita SMS ou troque para voz e tente de novo.",
	},
	{
		Code:           133005,
		Kind:           "WA_PIN_MISMATCH",
		DefaultMessage: "PIN incorreto. Use o fluxo SMS/voz para definir um novo PIN.",
	},
	{
		Code:           133006,
		Kind:           "WA_NOT_REGISTERED",
		DefaultMessage: "Número ainda não registrado no Cloud API. Registre antes de enviar ou receber mensagens.",
	},
	{
		Code:           133008,
		Kind:           "WA_PIN_LOCKED",
		DefaultMessage: "PIN bloqueado por tentativas demais. Aguarde 24h e use o fluxo SMS/voz para resetar.",
	},
	{
		Code:           133010,
		Kind:           "WA_PHONE_IN_USE_ELSEWHERE",
		DefaultMessage: "Este número está registrado em outro app Cloud API. Faça /deregister no provedor anterior antes de reutilizar.",
	},
	{
		Code:           133015,
		Kind:           "WA_ACCOUNT_CHECK_PENDING",
		DefaultMessage: "Meta está revisando o número. O registro voltará a funcionar quando a análise terminar.",
	},
	{
		Code:           133016,
		Kind:           "WA_REGISTER_RATE_LIMITED",
		DefaultMessage: "Meta limitou tentativas de registro deste número por 24h. Aguarde antes de tentar de novo — cada nova tentativa renova o cooldown.",
	},
	{
		Code:           136024,
		Kind:           "WA_ALREADY_VERIFIED",
		DefaultMessage: "Este número já está verificado em outro app. Use \"Liberar número\" antes de registrar aqui.",
	},
	{
		Code:           136025,
		Kind:           "WA_NOT_ONBOARDED",
		DefaultMessage: "Número nunca passou pelo onboarding. Refaça o Embedded Signup do começo.",
	},

	// ── Auth & permissions (cross-cutting) ────────────────────────────
	{
		Code:           190,
		Kind:           "WA_TOKEN_EXPIRED",
		DefaultMessage: "Token de acesso expirou. Reconecte o canal via Meta Embedded Signup.",
	},
	{
		Code:           10,
		Kind:           "MISSING_PERMISSION",
		DefaultMessage: "Sem permissão para esta operação. Verifique escopos do app no Meta Business Manager.",
	},
	{
		Code:           200,
		Kind:           "MISSING_PERMISSION",
		DefaultMessage: "Sem permissão para esta operação.",
	},
	{
		Code:           210,
		Kind:           "MISSING_PERMISSION",
		DefaultMessage: "Sem permissão para esta operação.",
	},

	// ── Send-message specific ─────────────────────────────────────────
	{
		Code:           131000,
		Kind:           "RECIPIENT_ERROR",
		DefaultMessage: "Erro genérico no destinatário.",
	},
	{
		Code:           131005,
		Kind:           "RECIPIENT_ERROR",
		DefaultMessage: "Destinatário inválido ou inacessível.",
	},
	{
		Code:           131016,
		Kind:           "RECIPIENT_ERROR",
		DefaultMessage: "Destinatário não está disponível para mensagens.",
	},
	{
		Code:           131051,
		Kind:           "RECIPIENT_ERROR",
		DefaultMessage: "Tipo de mensagem não suportado para este destinatário.",
	},
	{
		Code:           131009,
		Kind:           "INVALID_PARAM",
		DefaultMessage: "Parâmetro inválido na chamada Cloud API.",
	},
	{
		Code:           131021,
		Kind:           "WA_SELF_MESSAGE",
		DefaultMessage: "Remetente e destinatário são o mesmo número.",
	},
	{
		Code:           131026,
		Kind:           "OUTSIDE_24H_WINDOW",
		DefaultMessage: "Fora da janela de 24h. Use um template aprovado.",
	},
	{
		Code:           131031,
		Kind:           "WA_ACCOUNT_LOCKED",
		DefaultMessage: "Conta bloqueada pela Meta integrity. Apele em Meta Business Manager.",
	},
	{
		Code:           131047,
		Kind:           "OUTSIDE_24H_WINDOW",
		DefaultMessage: "Fora da janela de 24h. Use um template aprovado.",
	},
	{
		Code:           131056,
		Kind:           "WA_PAIR_RATE_LIMIT",
		DefaultMessage: "Rate limit por par (este negócio + este contato). Aguarde antes de retomar.",
	},

	// ── Recipient / send-message extras ───────────────────────────────
	{Code: 131008, Kind: "WA_REQUIRED_PARAM_MISSING", DefaultMessage: "Parâmetro obrigatório ausente na chamada Cloud API."},
	{Code: 131022, Kind: "WA_INVALID_RECIPIENT_NUMBER", DefaultMessage: "Número de destinatário inválido para mensagens."},
	{Code: 131027, Kind: "WA_RECIPIENT_NOT_TEST_NUMBER", DefaultMessage: "Destinatário não está na lista de números de teste do app."},
	{Code: 131030, Kind: "WA_RECIPIENT_NOT_ALLOWLISTED", DefaultMessage: "Destinatário não está na allowlist do sandbox. Adicione o número em Meta Business → WhatsApp → Configuração da API."},
	{Code: 131048, Kind: "WA_SPAM_RATE_LIMIT", DefaultMessage: "Limite de spam atingido. Reduza o ritmo de envio para este número."},
	{Code: 131049, Kind: "WA_QUALITY_RATE_LIMIT", DefaultMessage: "Limite de qualidade Meta atingido para mensagens de marketing. A mensagem não foi entregue para preservar o engajamento saudável do ecossistema."},
	{Code: 131050, Kind: "WA_USER_OPTED_OUT", DefaultMessage: "Cliente optou por não receber mensagens da empresa."},
	{Code: 131057, Kind: "WA_ACCOUNT_ON_HOLD", DefaultMessage: "Conta WhatsApp em hold pela Meta. Aguarde liberação ou apele em Meta Business Manager."},
	{Code: 131058, Kind: "WA_RESOURCE_TOO_LARGE", DefaultMessage: "Tamanho do recurso (mídia/payload) excede o limite Meta."},
	{Code: 131062, Kind: "WA_INVALID_COMPONENT_COUNT", DefaultMessage: "Número inválido de componentes no payload do template."},
	{Code: 131063, Kind: "WA_STICKER_ERROR", DefaultMessage: "Sticker inválido ou em formato não suportado."},
	{Code: 131064, Kind: "WA_PHONE_NUMBER_ERROR", DefaultMessage: "Erro no phone_number_id da chamada."},
	{Code: 131065, Kind: "WA_SENDER_OR_RECIPIENT_MISSING", DefaultMessage: "Remetente ou destinatário ausente na requisição."},
	{Code: 131067, Kind: "WA_INVALID_COUNTRY_CODE", DefaultMessage: "Código de país inválido no número de destino."},

	// ── Account / phone state ────────────────────────────────────────
	{Code: 133017, Kind: "WA_PHONE_DISABLED", DefaultMessage: "Número desativado pela Meta. Verifique status em Business Manager."},
	{Code: 134011, Kind: "WA_SANDBOX_RESTRICTED", DefaultMessage: "Conta restrita ao sandbox. Conclua a verificação para enviar a destinatários reais."},

	// ── Rate limits adicionais ───────────────────────────────────────
	{Code: 4, Kind: "WA_APP_USAGE_LIMIT", DefaultMessage: "Limite de uso do app Meta atingido. Aguarde antes de tentar de novo."},
	{Code: 17, Kind: "WA_USER_REQUEST_LIMIT", DefaultMessage: "Limite de requests por usuário atingido."},
	{Code: 32, Kind: "WA_PAGE_RATE_LIMIT", DefaultMessage: "Limite por página atingido. Aguarde."},
	{Code: 80007, Kind: "WA_BUSINESS_RATE_LIMIT", DefaultMessage: "Limite por business atingido. Aguarde antes de retomar."},
	{Code: 130429, Kind: "WA_CAPACITY_THROTTLE", DefaultMessage: "Capacidade Cloud API limitada momentaneamente. Backoff exponencial recomendado."},
	{Code: 130472, Kind: "WA_USER_IN_EXPERIMENT", DefaultMessage: "Número do destinatário faz parte de experimento Meta — entrega pode ser inconsistente."},

	// ── WABA / objects ───────────────────────────────────────────────
	{Code: 803, Kind: "WA_OBJECT_NOT_EXISTS", DefaultMessage: "Objeto solicitado não existe (WABA, phone, template) ou foi removido."},
	{Code: 268, Kind: "WA_ACCOUNT_LOCKED", DefaultMessage: "Conta bloqueada. Apele via Meta Business Manager."},
	{Code: 3, Kind: "WA_API_METHOD_INVALID", DefaultMessage: "Método de API inválido ou descontinuado."},

	// ── Templates ─────────────────────────────────────────────────────
	{
		Code:           132045,
		Kind:           "WA_TEMPLATE_PAUSED",
		DefaultMessage: "Template pausado pela Meta por sinais de pacing/opt-out. Use outro template ou aguarde resumo automático (~24h).",
	},
	{Code: 132069, Kind: "WA_TEMPLATE_DISABLED", DefaultMessage: "Template foi desativado pela Meta. Crie um novo ou edite o existente."},
	{Code: 132002, Kind: "WA_TEMPLATE_FORMAT_VIOLATION", DefaultMessage: "Política de formato/caracteres do template violada."},
	{Code: 132003, Kind: "WA_TEMPLATE_CHAR_POLICY", DefaultMessage: "Política de caracteres do template violada."},
	{Code: 132004, Kind: "WA_TEMPLATE_NOT_FOUND", DefaultMessage: "Template não existe nesta WABA."},
	{Code: 132000, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132001, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132005, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132007, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132012, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132015, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132016, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	{Code: 132068, Kind: "TEMPLATE_ERROR", DefaultMessage: "Erro de template Meta."},
	// Subcoded template errors (Meta nests duplicates / language errors here)
	{Code: 100, Subcode: 2388023, Kind: "WA_TEMPLATE_NAME_DUPLICATE", DefaultMessage: "Já existe um template com este nome nesta WABA."},
	{Code: 100, Subcode: 2388042, Kind: "WA_TEMPLATE_LANGUAGE_INVALID", DefaultMessage: "Código de idioma do template inválido (use formato pt_BR, en_US, etc.)."},

	// ── Generic platform error ───────────────────────────────────────
	{Code: 1, Kind: "WA_PLATFORM_ERROR", DefaultMessage: "Erro genérico de plataforma Meta. Tente novamente em alguns minutos."},
	{Code: 2, Kind: "WA_PLATFORM_ERROR", DefaultMessage: "Erro temporário de serviço Meta."},
	{Code: 135000, Kind: "WA_PLATFORM_ERROR", DefaultMessage: "Erro de plataforma Meta."},

	// ── Billing / business-verification gates (BR chicken-and-egg) ────
	// Meta hides these failures inside error_data.details and ships
	// generic Code=100 / Subcode=0 at the top level. Substring match
	// on Details (catalog now scans UserMsg + Message + Details) is
	// the only way to surface them — without these entries the user
	// only sees "Invalid parameter" and has no idea the real blocker
	// is missing payment method or unverified business.
	{
		Code:             100,
		UserMsgSubstring: "payment method",
		Kind:             "WA_NO_PAYMENT_METHOD",
		DefaultMessage:   "Adicione um método de pagamento à WABA antes de registrar o número. No Brasil, exige Verificação de Negócio aprovada antes de aceitar cartão BR — adicione cartão internacional ou conclua a verificação primeiro.",
	},
	{
		Code:             100,
		UserMsgSubstring: "business verification",
		Kind:             "WA_BUSINESS_NOT_VERIFIED",
		DefaultMessage:   "Negócio ainda não foi verificado pela Meta. Conclua a Verificação de Negócio em Meta Business → Central de Segurança.",
	},
	{
		Code:             100,
		UserMsgSubstring: "not verified",
		Kind:             "WA_BUSINESS_NOT_VERIFIED",
		DefaultMessage:   "Negócio ainda não foi verificado pela Meta. Conclua a Verificação de Negócio em Meta Business → Central de Segurança.",
	},
	{
		Code:             100,
		UserMsgSubstring: "cannot migrate phone number",
		Kind:             "WA_MIGRATION_BLOCKED",
		DefaultMessage:   "Migração do número bloqueada pela Meta. Causa comum: WABA sem método de pagamento ou negócio não verificado.",
	},

	// ── Generic 100 fallback (last resort) ────────────────────────────
	// Must be LAST among Code=100 entries so action-scoped 100 wins.
	// Without this entry the bare (#100) would fall through to the
	// "unknown" branch and never get a stable Kind.
	{
		Code:           100,
		Kind:           "INVALID_PARAM",
		DefaultMessage: "Parâmetro inválido na chamada Cloud API.",
	},
}

// Resolve walks the catalog and returns the first matching entry for
// the (error, action) pair. Returns ok=false when no entry matches —
// callers should bump a counter and Sentry-capture in that branch so
// new Meta codes surface automatically instead of silently degrading
// to a generic message.
//
// Action is optional: pass ActionUnknown to skip the action filter
// (useful from MetaError.Kind() which has no caller context).
func ResolveMetaError(err error, action MetaAction) (*MetaError, CatalogEntry, bool) {
	if err == nil {
		return nil, CatalogEntry{}, false
	}
	var meta *MetaError
	if !errors.As(err, &meta) {
		return nil, CatalogEntry{}, false
	}
	for _, e := range metaErrorCatalog {
		if e.Code != 0 && e.Code != meta.Code {
			continue
		}
		if e.Subcode != 0 && e.Subcode != meta.Subcode {
			continue
		}
		if e.Action != "" && e.Action != action {
			continue
		}
		if e.UserMsgSubstring != "" &&
			!containsCaseInsensitive(meta.UserMsg, e.UserMsgSubstring) &&
			!containsCaseInsensitive(meta.Message, e.UserMsgSubstring) &&
			!containsCaseInsensitive(meta.Details, e.UserMsgSubstring) {
			continue
		}
		return meta, e, true
	}
	return meta, CatalogEntry{}, false
}
