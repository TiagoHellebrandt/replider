package email

import (
	"fmt"
	"html/template"
	"strings"
)

// Localized copy lives here, not in the templates, so subjects and
// inline copy can be assembled with proper interpolation without
// fighting Go's html/template scoping. Keys mirror the per-email
// composer names below.
//
// Add new entries by appending to BOTH localeStrings maps so en-fallback
// callers don't get pt-BR text. We keep the two maps separate (instead
// of a single nested map) so missing-key bugs surface at compile time —
// a typo'd map key still compiles, but `localeFor(loc)[key]` returning
// "" is easy to spot in tests.

type stringMap = map[string]string

var stringsPtBR = stringMap{
	"footer_tagline":                         "Replider — atendimento omnichannel com IA",
	"signed_team":                            "Equipe Replider",
	"link_expires_one_hour":                  "O link expira em 1 hora.",
	"link_expires_24_hours":                  "O link expira em 24 horas.",
	"if_not_requested":                       "Se você não fez essa solicitação, ignore este e-mail. Sua conta segue segura.",
	"if_not_invited":                         "Se você não esperava esse convite, ignore este e-mail.",
	"smtp_test_subject":                      "Replider — teste de SMTP",
	"smtp_test_title":                        "SMTP funcionando ✓",
	"smtp_test_intro":                        "Esse é um e-mail de teste enviado pela tela de Conformidade. Se chegou na sua caixa, o servidor está conseguindo entregar mensagens corretamente.",
	"smtp_test_footnote":                     "Pode descartar este e-mail.",
	"verify_subject":                         "Confirme seu e-mail no Replider",
	"verify_title":                           "Confirme seu e-mail",
	"verify_intro":                           "Bem-vindo ao Replider! Para liberar sua conta, confirme seu e-mail clicando no botão abaixo.",
	"verify_cta":                             "Confirmar e-mail",
	"reset_subject":                          "Recuperação de senha — Replider",
	"reset_title":                            "Recuperação de senha",
	"reset_intro":                            "Recebemos um pedido para redefinir a senha da sua conta. Clique no botão abaixo para escolher uma nova senha.",
	"reset_cta":                              "Redefinir senha",
	"welcome_subject":                        "Bem-vindo ao Replider",
	"welcome_title":                          "Sua conta está pronta",
	"welcome_intro":                          "Seu e-mail foi confirmado. Você já pode acessar o painel e configurar seu primeiro agente de IA.",
	"welcome_cta":                            "Abrir o painel",
	"welcome_steps_html":                     `<p style="margin:8px 0 12px 0;">Por onde começar:</p><ol style="margin:0 0 16px 18px;padding:0;color:#D4D4D4;"><li style="margin-bottom:6px;">Crie seu agente de IA — defina nome, tom de voz e modelo.</li><li style="margin-bottom:6px;">Conecte um canal — WhatsApp, Telegram ou Instagram.</li><li>Adicione conhecimento — cole horários, preços, FAQs.</li></ol>`,
	"invite_subject":                         "Você foi convidado para um workspace no Replider",
	"invite_title":                           "Você foi convidado",
	"invite_intro_template":                  "%s convidou você para fazer parte da equipe no Replider. Clique no botão abaixo para definir sua senha e entrar.",
	"invite_cta":                             "Definir senha",
	"usage_warning_subject_template":         "Você usou %d%% dos seus créditos de IA",
	"usage_warning_title":                    "Seus créditos estão acabando",
	"usage_warning_intro_template":           "Você já consumiu <strong>%d%%</strong> dos créditos de IA do plano <strong>%s</strong> neste ciclo. Quando os créditos zerarem, o agente para de responder automaticamente.",
	"usage_warning_cta":                      "Fazer upgrade",
	"usage_limit_subject":                    "Seus créditos de IA acabaram",
	"usage_limit_title":                      "Créditos esgotados",
	"usage_limit_intro_template":             "Os créditos de IA do plano <strong>%s</strong> acabaram neste ciclo. <strong>Seu agente parou de responder automaticamente</strong> até a próxima recarga ou compra de pacote extra.",
	"usage_limit_cta":                        "Comprar créditos",
	"payment_failed_subject":                 "Falha no pagamento da sua assinatura",
	"payment_failed_title":                   "Pagamento não aprovado",
	"payment_failed_intro_template":          "O pagamento da sua assinatura <strong>%s</strong> não foi aprovado pela operadora. O agente continua funcionando por <strong>7 dias</strong>; depois disso o atendimento automático pausa até regularizar.",
	"payment_failed_cta":                     "Atualizar pagamento",
	"prebilling_subject":                     "Sua assinatura Replider renova em breve",
	"prebilling_title":                       "Renovação chegando",
	"prebilling_intro_template":              "Lembrete amigável: sua assinatura <strong>%s</strong> será renovada automaticamente em <strong>%s</strong>. Você não precisa fazer nada — é só para evitar surpresas na fatura.",
	"prebilling_when_today":                  "hoje",
	"prebilling_when_tomorrow":               "amanhã",
	"prebilling_when_days_template":          "%d dias",
	"prebilling_cta":                         "Gerenciar assinatura",
	"sdr_digest_subject":                     "O que precisa de você hoje",
	"sdr_digest_title":                       "Seu dia na Replider",
	"sdr_digest_intro":                       "Isto está esperando por você agora:",
	"sdr_digest_cta":                         "Abrir",
	"digest_subject":                         "Seu resumo semanal do Replider",
	"digest_title":                           "Resumo da semana",
	"digest_intro":                           "Veja o que seu agente de IA fez pela sua equipe nos últimos 7 dias:",
	"digest_metric_messages":                 "Mensagens respondidas automaticamente",
	"digest_metric_time_saved":               "Tempo economizado da equipe",
	"digest_metric_conversion":               "Conversas resolvidas pelo bot",
	"digest_faqs_title":                      "Perguntas mais frequentes",
	"digest_no_activity":                     "Seu agente ainda não teve atividade esta semana. Assim que as conversas começarem, este resumo ganha vida.",
	"digest_minutes_template":                "%d min",
	"digest_cta":                             "Abrir painel",
	"digest_unsubscribe":                     "Não quer mais receber este resumo? É só desativar em Configurações.",
	"reactivation_subject":                   "Seu agente de IA ainda está aqui — volte quando quiser",
	"reactivation_title":                     "Que tal recomeçar?",
	"reactivation_intro":                     "Faz %d dias que seu teste do Replider terminou, e sua conta continua pronta para uso.",
	"reactivation_body":                      "Reative seu plano e seu agente de IA volta a responder seus clientes automaticamente, no WhatsApp e nos demais canais. Seus agentes e conhecimento já configurados continuam salvos.",
	"reactivation_cta":                       "Reativar minha conta",
	"reactivation_footnote":                  "Se preferir não receber mais estes lembretes, é só ignorar este e-mail.",
	"winback_subject":                        "Volte com %d%% de desconto por %d meses 💙",
	"winback_title":                          "Sentimos sua falta",
	"winback_intro":                          "Preparamos uma oferta especial para você voltar ao Replider: %d%% de desconto nos próximos %d meses.",
	"winback_body":                           "É só usar o código %s ao reativar seu plano e seu agente de IA volta a responder seus clientes automaticamente. Seus agentes e conhecimento já configurados continuam salvos.",
	"winback_cta":                            "Reativar com desconto",
	"winback_footnote":                       "Se preferir não receber mais estes e-mails, é só ignorá-los.",
	"onboarding_nudge_channel_subject":       "Conecte seu e-mail e a primeira oferta sai hoje",
	"onboarding_nudge_channel_title":         "Falta pouco: conecte seu e-mail",
	"onboarding_nudge_channel_intro":         "Sua conta está pronta — só falta o e-mail de onde as ofertas vão sair.",
	"onboarding_nudge_channel_body":          "Conecte a conta que você já usa (Gmail ou Google Workspace) em um clique, ou informe seu SMTP. As respostas continuam caindo na caixa que você já lê.",
	"onboarding_nudge_channel_cta":           "Conectar e-mail",
	"onboarding_nudge_products_subject":      "Importe seus produtos para as ofertas terem o que oferecer",
	"onboarding_nudge_products_title":        "O que você vende?",
	"onboarding_nudge_products_intro":        "Sem catálogo, a oferta sai genérica — e uma oferta genérica não vende.",
	"onboarding_nudge_products_body":         "Importe uma planilha com seus produtos (nome, preço, descrição) e cada oferta passa a citar o item certo para cada cliente.",
	"onboarding_nudge_products_cta":          "Importar produtos",
	"onboarding_nudge_customers_subject":     "Traga seus clientes e a Replider escreve para eles",
	"onboarding_nudge_customers_title":       "Para quem vamos escrever?",
	"onboarding_nudge_customers_intro":       "Suas ofertas precisam de destinatários. Uma planilha com nome e e-mail já basta.",
	"onboarding_nudge_customers_body":        "Importe a lista de clientes que você já atende — ou deixe que a gente encontre hospitais e clínicas no seu território.",
	"onboarding_nudge_customers_cta":         "Importar clientes",
	"onboarding_nudge_proposal_subject":      "Sua primeira oferta está esperando aprovação",
	"onboarding_nudge_proposal_title":        "Aprove sua primeira oferta",
	"onboarding_nudge_proposal_intro":        "Já há um rascunho na sua fila de Ofertas. Nada sai sem a sua aprovação.",
	"onboarding_nudge_proposal_body":         "Leia, ajuste se quiser e aprove. A partir daí, novas ofertas chegam todo dia na fila para você revisar.",
	"onboarding_nudge_proposal_cta":          "Ver fila de ofertas",
	"onboarding_nudge_first_inbound_subject": "Mantenha as ofertas saindo para as respostas chegarem",
	"onboarding_nudge_first_inbound_title":   "As respostas vêm do volume",
	"onboarding_nudge_first_inbound_intro":   "Ainda não chegou nenhuma resposta — e isso é normal na primeira semana.",
	"onboarding_nudge_first_inbound_body":    "Aprove as ofertas da fila todos os dias: quanto mais mensagens saem, mais cedo a primeira resposta aparece na sua caixa de entrada.",
	"onboarding_nudge_first_inbound_cta":     "Aprovar ofertas de hoje",
	"onboarding_nudge_team_invited_subject":  "Convide quem aprova as ofertas",
	"onboarding_nudge_team_invited_title":    "Convide sua vendedora",
	"onboarding_nudge_team_invited_intro":    "Quem revisa e aprova as ofertas costuma não ser quem criou a conta.",
	"onboarding_nudge_team_invited_body":     "Convide a pessoa do time comercial para a conta: ela recebe a fila de ofertas e aprova direto do celular.",
	"onboarding_nudge_team_invited_cta":      "Convidar equipe",
	"onboarding_nudge_footnote":              "Não quer mais receber estas dicas de configuração? É só ignorar este e-mail.",
	"reminder_subject_template":              "%s está aguardando há %d min",
	"reminder_title":                         "Cliente aguardando resposta",
	"reminder_intro_template":                "<strong>%s</strong> enviou uma mensagem há <strong>%d min</strong> e ainda não foi respondido. A IA está pausada nessa conversa — você está encarregado.",
	"reminder_cta":                           "Abrir conversa",
	"support_unread_subject":                 "Você tem uma resposta do suporte Replider",
	"support_unread_title":                   "Seu gerente de conta respondeu",
	"support_unread_intro":                   "Você recebeu uma resposta na sua conversa de suporte dentro do Replider e ela ainda está esperando por você. Abra o painel para ler e continuar a conversa.",
	"support_unread_cta":                     "Abrir conversa de suporte",
	"support_unread_footnote":                "Você recebe este e-mail apenas quando uma resposta fica sem leitura por algumas horas.",
}

var stringsEn = stringMap{
	"footer_tagline":                         "Replider — AI-powered omnichannel support",
	"signed_team":                            "The Replider Team",
	"link_expires_one_hour":                  "This link expires in 1 hour.",
	"link_expires_24_hours":                  "This link expires in 24 hours.",
	"if_not_requested":                       "If you didn't request this, just ignore the email. Your account stays safe.",
	"if_not_invited":                         "If you weren't expecting this invitation, please ignore this email.",
	"smtp_test_subject":                      "Replider — SMTP test",
	"smtp_test_title":                        "SMTP is working ✓",
	"smtp_test_intro":                        "This is a test message triggered from the Compliance screen. If it landed in your inbox, the server can deliver email end-to-end.",
	"smtp_test_footnote":                     "You can safely discard this email.",
	"verify_subject":                         "Confirm your email on Replider",
	"verify_title":                           "Confirm your email",
	"verify_intro":                           "Welcome to Replider! To activate your account, confirm your email by clicking the button below.",
	"verify_cta":                             "Confirm email",
	"reset_subject":                          "Password reset — Replider",
	"reset_title":                            "Reset your password",
	"reset_intro":                            "We received a request to reset your account password. Click the button below to choose a new password.",
	"reset_cta":                              "Reset password",
	"welcome_subject":                        "Welcome to Replider",
	"welcome_title":                          "Your account is ready",
	"welcome_intro":                          "Your email is confirmed. You can sign in and set up your first AI agent now.",
	"welcome_cta":                            "Open dashboard",
	"welcome_steps_html":                     `<p style="margin:8px 0 12px 0;">Where to start:</p><ol style="margin:0 0 16px 18px;padding:0;color:#D4D4D4;"><li style="margin-bottom:6px;">Create your AI agent — set its name, voice, and model.</li><li style="margin-bottom:6px;">Connect a channel — WhatsApp, Telegram, or Instagram.</li><li>Add knowledge — paste hours, prices, FAQs.</li></ol>`,
	"invite_subject":                         "You've been invited to a Replider workspace",
	"invite_title":                           "You're invited",
	"invite_intro_template":                  "%s invited you to join their team on Replider. Click the button below to set your password and get started.",
	"invite_cta":                             "Set password",
	"usage_warning_subject_template":         "You've used %d%% of your AI credits",
	"usage_warning_title":                    "Your credits are running low",
	"usage_warning_intro_template":           "You've used <strong>%d%%</strong> of the AI credits in your <strong>%s</strong> plan this cycle. When credits hit zero your agent stops auto-replying.",
	"usage_warning_cta":                      "Upgrade plan",
	"usage_limit_subject":                    "Your AI credits are exhausted",
	"usage_limit_title":                      "Credits exhausted",
	"usage_limit_intro_template":             "The AI credits in your <strong>%s</strong> plan are spent for this cycle. <strong>Your agent has stopped auto-replying</strong> until the next reset or until you buy an add-on pack.",
	"usage_limit_cta":                        "Buy credits",
	"payment_failed_subject":                 "Payment for your subscription failed",
	"payment_failed_title":                   "Payment not approved",
	"payment_failed_intro_template":          "Your <strong>%s</strong> subscription payment was declined by the card issuer. Your agent keeps working for <strong>7 days</strong>; after that auto-replies pause until you fix the payment.",
	"prebilling_subject":                     "Your Replider subscription renews soon",
	"prebilling_title":                       "Renewal coming up",
	"prebilling_intro_template":              "Friendly heads-up: your <strong>%s</strong> subscription renews automatically in <strong>%s</strong>. You don't need to do anything — this is just so the charge isn't a surprise.",
	"prebilling_when_today":                  "today",
	"prebilling_when_tomorrow":               "tomorrow",
	"prebilling_when_days_template":          "%d days",
	"prebilling_cta":                         "Manage subscription",
	"payment_failed_cta":                     "Update payment",
	"sdr_digest_subject":                     "What needs you today",
	"sdr_digest_title":                       "Your day on Replider",
	"sdr_digest_intro":                       "This is waiting on you right now:",
	"sdr_digest_cta":                         "Open",
	"digest_subject":                         "Your weekly Replider recap",
	"digest_title":                           "This week's recap",
	"digest_intro":                           "Here's what your AI agent did for your team over the last 7 days:",
	"digest_metric_messages":                 "Messages answered automatically",
	"digest_metric_time_saved":               "Team time saved",
	"digest_metric_conversion":               "Conversations resolved by the bot",
	"digest_faqs_title":                      "Most frequent questions",
	"digest_no_activity":                     "Your agent had no activity this week yet. As soon as conversations start, this recap comes to life.",
	"digest_minutes_template":                "%d min",
	"digest_cta":                             "Open dashboard",
	"digest_unsubscribe":                     "Don't want this recap anymore? Just turn it off in Settings.",
	"reactivation_subject":                   "Your AI agent is still here — come back anytime",
	"reactivation_title":                     "Ready for a fresh start?",
	"reactivation_intro":                     "It's been %d days since your Replider trial ended, and your account is still ready to go.",
	"reactivation_body":                      "Reactivate your plan and your AI agent picks right back up, answering customers automatically on WhatsApp and your other channels. The agents and knowledge you already set up are still saved.",
	"reactivation_cta":                       "Reactivate my account",
	"reactivation_footnote":                  "Prefer not to get these reminders? Just ignore this email.",
	"winback_subject":                        "Come back — %d%% off for %d months 💙",
	"winback_title":                          "We'd love to have you back",
	"winback_intro":                          "We put together a special offer to welcome you back to Replider: %d%% off for the next %d months.",
	"winback_body":                           "Just use code %s when you reactivate your plan and your AI agent picks right back up, answering customers automatically. The agents and knowledge you already set up are still saved.",
	"winback_cta":                            "Reactivate with discount",
	"winback_footnote":                       "Prefer not to get these emails? Just ignore them.",
	"onboarding_nudge_channel_subject":       "Connect your email and the first offer goes out today",
	"onboarding_nudge_channel_title":         "Almost there: connect your email",
	"onboarding_nudge_channel_intro":         "Your account is ready — it just needs the mailbox the offers will go out from.",
	"onboarding_nudge_channel_body":          "Connect the account you already use (Gmail or Google Workspace) in one click, or enter your SMTP details. Replies keep landing in the inbox you already read.",
	"onboarding_nudge_channel_cta":           "Connect email",
	"onboarding_nudge_products_subject":      "Import your products so the offers have something to offer",
	"onboarding_nudge_products_title":        "What do you sell?",
	"onboarding_nudge_products_intro":        "Without a catalogue the offer comes out generic — and a generic offer does not sell.",
	"onboarding_nudge_products_body":         "Import a spreadsheet with your products (name, price, description) and every offer starts naming the right item for each customer.",
	"onboarding_nudge_products_cta":          "Import products",
	"onboarding_nudge_customers_subject":     "Bring your customers and Replider writes to them",
	"onboarding_nudge_customers_title":       "Who are we writing to?",
	"onboarding_nudge_customers_intro":       "Your offers need recipients. A spreadsheet with a name and an email is enough.",
	"onboarding_nudge_customers_body":        "Import the list of customers you already serve — or let us find hospitals and clinics in your territory.",
	"onboarding_nudge_customers_cta":         "Import customers",
	"onboarding_nudge_proposal_subject":      "Your first offer is waiting for approval",
	"onboarding_nudge_proposal_title":        "Approve your first offer",
	"onboarding_nudge_proposal_intro":        "There is already a draft in your Offers queue. Nothing goes out without your approval.",
	"onboarding_nudge_proposal_body":         "Read it, tweak it if you like and approve. From then on, new offers land in the queue every day for you to review.",
	"onboarding_nudge_proposal_cta":          "Open the offers queue",
	"onboarding_nudge_first_inbound_subject": "Keep the offers going out so the replies come in",
	"onboarding_nudge_first_inbound_title":   "Replies come from volume",
	"onboarding_nudge_first_inbound_intro":   "No reply has arrived yet — and that is normal in the first week.",
	"onboarding_nudge_first_inbound_body":    "Approve the queued offers every day: the more messages go out, the sooner the first reply shows up in your inbox.",
	"onboarding_nudge_first_inbound_cta":     "Approve today's offers",
	"onboarding_nudge_team_invited_subject":  "Invite the person who approves the offers",
	"onboarding_nudge_team_invited_title":    "Invite your salesperson",
	"onboarding_nudge_team_invited_intro":    "The person who reviews and approves the offers is usually not the one who created the account.",
	"onboarding_nudge_team_invited_body":     "Invite someone from the sales team to the account: they get the offers queue and approve straight from their phone.",
	"onboarding_nudge_team_invited_cta":      "Invite your team",
	"onboarding_nudge_footnote":              "Don't want these setup tips anymore? Just ignore this email.",
	"reminder_subject_template":              "%s has been waiting %d min",
	"reminder_title":                         "Customer waiting on your reply",
	"reminder_intro_template":                "<strong>%s</strong> messaged <strong>%d min</strong> ago and is still waiting. The AI is paused on this conversation — you are the one in charge.",
	"reminder_cta":                           "Open conversation",
	"support_unread_subject":                 "You have a reply from Replider support",
	"support_unread_title":                   "Your account manager replied",
	"support_unread_intro":                   "You received a reply in your Replider support conversation and it is still waiting for you. Open the dashboard to read it and continue the conversation.",
	"support_unread_cta":                     "Open support conversation",
	"support_unread_footnote":                "You only get this email when a reply goes unread for a few hours.",
}

func localeFor(loc Locale) stringMap {
	if loc == LocaleEn {
		return stringsEn
	}
	return stringsPtBR
}

func tr(loc Locale, key string, args ...any) string {
	raw := localeFor(loc)[key]
	if raw == "" {
		// Fallback to pt-BR so a typo'd key in en doesn't ship empty
		// copy. The pt-BR map is the source of truth.
		raw = stringsPtBR[key]
	}
	if len(args) == 0 {
		return raw
	}
	return fmt.Sprintf(raw, args...)
}

// plaintextFooter is a small formatted block appended to every plain-
// text fallback so the user has the brand name + a parsable signature
// even when their client refuses HTML.
func plaintextFooter(loc Locale) string {
	var b strings.Builder
	b.WriteString("\n\n— ")
	b.WriteString(tr(loc, "signed_team"))
	b.WriteString("\n")
	b.WriteString(tr(loc, "footer_tagline"))
	return b.String()
}

// asHTML wraps a string in template.HTML for safe interpolation. Use
// only for trusted strings (template-key lookups in this file). Never
// pass user-supplied content through this without escapeHTML first.
func asHTML(s string) template.HTML { return template.HTML(s) }
