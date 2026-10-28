package mcp

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Brand kit, email templates and their numbers (§6.7 / §6.8).
//
// Parity with the screens, per the spec's definition of done. These three in
// particular earn it: they are what an operator adjusts between calls, from
// a phone, when a template is not landing.

func registerBrandTools(s *mcpsdk.Server, deps Deps, admin bool) {
	registerBrandKitTools(s, deps, admin)
	registerEmailTemplateTools(s, deps, admin)
}

func registerBrandKitTools(s *mcpsdk.Server, deps Deps, admin bool) {
	if deps.BrandRepo == nil {
		return
	}

	type brandKitOutput struct {
		CompanyName  string `json:"company_name"`
		About        string `json:"about"`
		ToneOfVoice  string `json:"tone_of_voice"`
		Signature    string `json:"signature"`
		PrimaryColor string `json:"primary_color,omitempty"`
		LogoURL      string `json:"logo_url,omitempty"`
		// AllowedClaims are what a salesperson MAY say; ForbiddenClaims are
		// what nobody may, at any level. In healthcare the second list is
		// where a wrong sentence stops being a style problem and becomes a
		// regulatory one.
		AllowedClaims   []string `json:"allowed_claims"`
		ForbiddenClaims []string `json:"forbidden_claims"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_brand_kit",
		Description: "Mostra a identidade da empresa usada em toda mensagem: o que ela faz, tom de voz, " +
			"assinatura e — o mais importante — o que pode e o que NÃO pode ser afirmado.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *brandKitOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		kit, err := deps.BrandRepo.Get(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		if kit == nil {
			// An account that never filled it in. An empty kit is the honest
			// answer — inventing a tone of voice would put words in a
			// company's mouth.
			return nil, &brandKitOutput{}, nil
		}
		return nil, &brandKitOutput{
			CompanyName: kit.CompanyName, About: kit.About,
			ToneOfVoice: kit.ToneOfVoice, Signature: kit.Signature,
			PrimaryColor: kit.PrimaryColor, LogoURL: kit.LogoURL,
			AllowedClaims: kit.AllowedClaims, ForbiddenClaims: kit.ForbiddenClaims,
		}, nil
	})

	if !admin {
		// Writing the brand kit is an admin act: the forbidden-claims list is
		// a compliance control, not a preference, and a salesperson must not
		// be able to widen what they are allowed to promise.
		return
	}

	type updateBrandKitInput struct {
		CompanyName     string   `json:"company_name,omitempty"`
		About           string   `json:"about,omitempty"`
		ToneOfVoice     string   `json:"tone_of_voice,omitempty"`
		Signature       string   `json:"signature,omitempty"`
		PrimaryColor    string   `json:"primary_color,omitempty"`
		LogoURL         string   `json:"logo_url,omitempty"`
		AllowedClaims   []string `json:"allowed_claims,omitempty"`
		ForbiddenClaims []string `json:"forbidden_claims,omitempty"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "update_brand_kit",
		Description: "Atualiza a identidade da empresa. Só os campos enviados mudam — um campo ausente " +
			"mantém o valor atual, para que ajustar o tom de voz não apague a lista de proibições.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in updateBrandKitInput) (*mcpsdk.CallToolResult, *brandKitOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		kit, err := deps.BrandRepo.Get(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		if kit == nil {
			kit = &domain.BrandKit{AccountID: id.AccountID}
		}
		// Merge, never replace. A model calling this to fix a typo in the
		// tone of voice must not silently drop the forbidden-claims list it
		// did not mention.
		kit.CompanyName = orKeep(in.CompanyName, kit.CompanyName)
		kit.About = orKeep(in.About, kit.About)
		kit.ToneOfVoice = orKeep(in.ToneOfVoice, kit.ToneOfVoice)
		kit.Signature = orKeep(in.Signature, kit.Signature)
		kit.PrimaryColor = orKeep(in.PrimaryColor, kit.PrimaryColor)
		kit.LogoURL = orKeep(in.LogoURL, kit.LogoURL)
		if in.AllowedClaims != nil {
			kit.AllowedClaims = in.AllowedClaims
		}
		if in.ForbiddenClaims != nil {
			kit.ForbiddenClaims = in.ForbiddenClaims
		}
		kit.AccountID = id.AccountID

		if err := deps.BrandRepo.Save(ctx, kit); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &brandKitOutput{
			CompanyName: kit.CompanyName, About: kit.About,
			ToneOfVoice: kit.ToneOfVoice, Signature: kit.Signature,
			PrimaryColor: kit.PrimaryColor, LogoURL: kit.LogoURL,
			AllowedClaims: kit.AllowedClaims, ForbiddenClaims: kit.ForbiddenClaims,
		}, nil
	})
}

// orKeep returns the new value when it carries something, otherwise the old.
func orKeep(next, current string) string {
	if strings.TrimSpace(next) == "" {
		return current
	}
	return next
}

// templateStatsOut is one version's performance.
type templateStatsOut struct {
	Version int `json:"version"`
	Sent    int `json:"sent"`
	// Opened is a WEAK signal and labelled as such wherever it is shown:
	// Apple Mail Privacy Protection pre-fetches images, inflating opens by
	// roughly half.
	Opened  int `json:"opened"`
	Clicked int `json:"clicked"`
	Replied int `json:"replied"`
	Bounced int `json:"bounced"`
	// ReplyRate is nil when nothing was sent. Zero replies out of zero sends
	// is unknown, not a zero rate — rendering 0% would condemn a template
	// nobody has used yet.
	ReplyRate *float64 `json:"reply_rate,omitempty"`
	// DirtyList flags a bounce rate high enough to be hurting the sending
	// domain's reputation.
	DirtyList bool `json:"dirty_list"`
}

// templateOut is one template plus the facts the caller would otherwise have
// to recompute (and get differently).
type templateOut struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	Version   int      `json:"version"`
	Variables []string `json:"variables,omitempty"`
	WordCount int      `json:"word_count"`
	// TooLong flags a body past the point where a cold email stops being
	// read. A warning, not a limit — the person writing knows their market.
	TooLong bool               `json:"too_long"`
	Stats   []templateStatsOut `json:"stats,omitempty"`
}

func registerEmailTemplateTools(s *mcpsdk.Server, deps Deps, admin bool) {
	if deps.EmailTemplateRepo == nil {
		return
	}

	type listTemplatesOutput struct {
		Templates []templateOut `json:"templates"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_email_templates",
		Description: "Lista os modelos de e-mail da conta com as variáveis de cada um e a contagem de " +
			"palavras (cold email que funciona tem 25-100).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, *listTemplatesOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		list, err := deps.EmailTemplateRepo.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listTemplatesOutput{Templates: make([]templateOut, 0, len(list))}
		for _, tpl := range list {
			out.Templates = append(out.Templates, templateSummary(tpl, nil))
		}
		return nil, out, nil
	})

	type metricsInput struct {
		TemplateID string `json:"template_id"`
	}
	type metricsOutput struct {
		TemplateID string             `json:"template_id"`
		Stats      []templateStatsOut `json:"stats"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_email_metrics",
		Description: "Números de um modelo, por VERSÃO — porque '12% de resposta' não significa nada se o " +
			"texto mudou duas vezes desde que o número foi conquistado. Hierarquia do sinal: " +
			"resposta > clique > abertura (abertura é fraca, a Apple infla ~50%).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in metricsInput) (*mcpsdk.CallToolResult, *metricsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		// Ownership first: Stats takes only a template id, so without this a
		// caller could read another account's numbers by guessing one.
		tpl, err := deps.EmailTemplateRepo.Get(ctx, in.TemplateID, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		if tpl == nil {
			return nil, nil, toolError(&domain.AppError{
				Code: "EMAIL_TEMPLATE_NOT_FOUND", Message: "modelo não encontrado"})
		}
		stats, err := deps.EmailTemplateRepo.Stats(ctx, tpl.ID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &metricsOutput{TemplateID: tpl.ID, Stats: make([]templateStatsOut, 0, len(stats))}
		for _, st := range stats {
			out.Stats = append(out.Stats, statsSummary(st))
		}
		return nil, out, nil
	})

	if !admin {
		return
	}

	type upsertInput struct {
		// ID empty creates. Passing an existing id edits it, and editing the
		// TEXT bumps the version so the old numbers stay attached to the old
		// words.
		ID      string `json:"id,omitempty"`
		Name    string `json:"name"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upsert_email_template",
		Description: "Cria ou edita um modelo de e-mail. Editar o TEXTO cria uma nova versão, para que os " +
			"números conquistados continuem descrevendo as palavras que os conquistaram.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in upsertInput) (*mcpsdk.CallToolResult, *templateOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Body) == "" {
			return nil, nil, toolError(&domain.AppError{
				Code: "VALIDATION_ERROR", Message: "nome e corpo são obrigatórios"})
		}
		tplID := strings.TrimSpace(in.ID)
		if tplID == "" {
			tplID = ulid.Make().String()
		} else if existing, err := deps.EmailTemplateRepo.Get(ctx, tplID, id.AccountID); err != nil {
			return nil, nil, toolError(err)
		} else if existing == nil {
			// Editing something that is not ours must not silently create it
			// under our account.
			return nil, nil, toolError(&domain.AppError{
				Code: "EMAIL_TEMPLATE_NOT_FOUND", Message: "modelo não encontrado"})
		}

		tpl := &domain.EmailTemplate{
			ID: tplID, AccountID: id.AccountID,
			Name: in.Name, Subject: in.Subject, Body: in.Body, IsActive: true,
		}
		if err := deps.EmailTemplateRepo.Upsert(ctx, tpl); err != nil {
			return nil, nil, toolError(err)
		}
		saved, err := deps.EmailTemplateRepo.Get(ctx, tplID, id.AccountID)
		if err != nil || saved == nil {
			return nil, nil, toolError(err)
		}
		out := templateSummary(saved, nil)
		return nil, &out, nil
	})
}

func templateSummary(tpl *domain.EmailTemplate, stats []domain.EmailTemplateStats) templateOut {
	out := templateOut{
		ID: tpl.ID, Name: tpl.Name, Subject: tpl.Subject, Body: tpl.Body,
		Version:   tpl.Version,
		Variables: domain.TemplateVariables(tpl.Subject, tpl.Body),
		WordCount: domain.WordCount(tpl.Body),
	}
	out.TooLong = out.WordCount > domain.ColdEmailMaxWords
	for _, st := range stats {
		out.Stats = append(out.Stats, statsSummary(st))
	}
	return out
}

func statsSummary(st domain.EmailTemplateStats) templateStatsOut {
	return templateStatsOut{
		Version: st.Version, Sent: st.Sent, Opened: st.Opened,
		Clicked: st.Clicked, Replied: st.Replied, Bounced: st.Bounced,
		ReplyRate: st.ReplyRate(), DirtyList: st.NeedsListWarning(),
	}
}
