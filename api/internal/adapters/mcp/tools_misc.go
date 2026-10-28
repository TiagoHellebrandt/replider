package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// --- calendars ---

func registerCalendarTools(s *mcpsdk.Server, deps Deps, admin bool) {
	type listCalendarsInput struct {
		AgentID string `json:"agent_id" jsonschema:"agente dono dos calendários"`
	}
	// structuredContent must be a JSON object, so the slice is wrapped —
	// returning the bare array fails schema validation on the client.
	type listCalendarsOutput struct {
		Calendars []calendarSummary `json:"calendars"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_calendars",
		Description: "Lista os calendários de agendamento de um agente. O booking automático só oferece " +
			"horários de calendários com google_calendar_id configurado.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listCalendarsInput) (*mcpsdk.CallToolResult, *listCalendarsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		cals, err := deps.ListCalendarsUC.Execute(ctx, id.AccountID, in.AgentID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listCalendarsOutput{Calendars: make([]calendarSummary, 0, len(cals))}
		for _, c := range cals {
			out.Calendars = append(out.Calendars, toCalendarSummary(c))
		}
		return nil, out, nil
	})

	if !admin {
		return
	}

	type upsertCalendarInput struct {
		CalendarID          string  `json:"calendar_id,omitempty" jsonschema:"vazio cria; preenchido atualiza"`
		AgentID             string  `json:"agent_id"`
		Name                string  `json:"name"`
		AvailableDays       []int32 `json:"available_days,omitempty" jsonschema:"dias da semana: 0=domingo ... 6=sábado"`
		SlotDurationMinutes *int32  `json:"slot_duration_minutes,omitempty"`
		GoogleCalendarID    *string `json:"google_calendar_id,omitempty" jsonschema:"ID do Google Calendar (e-mail da agenda ou 'primary'); sem ele o booking automático não oferece horários deste calendário"`
		IsActive            *bool   `json:"is_active,omitempty" jsonschema:"false desativa o calendário (só na atualização)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upsert_calendar",
		Description: "Cria ou atualiza um calendário de agendamento de um agente. A disponibilidade real vem do " +
			"Google Calendar: configure google_calendar_id, senão o agente não terá horários para oferecer.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in upsertCalendarInput) (*mcpsdk.CallToolResult, *calendarSummary, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if in.CalendarID == "" {
			cal, err := deps.CreateCalendarUC.Execute(ctx, usecases.CreateCalendarDTO{
				AgentID:             in.AgentID,
				AccountID:           id.AccountID,
				Name:                in.Name,
				AvailableDays:       in.AvailableDays,
				SlotDurationMinutes: in.SlotDurationMinutes,
				GoogleCalendarID:    in.GoogleCalendarID,
			})
			if err != nil {
				return nil, nil, toolError(err)
			}
			out := toCalendarSummary(cal)
			return nil, &out, nil
		}
		name := in.Name
		cal, err := deps.UpdateCalendarUC.Execute(ctx, usecases.UpdateCalendarDTO{
			CalendarID:          in.CalendarID,
			AgentID:             in.AgentID,
			AccountID:           id.AccountID,
			Name:                &name,
			AvailableDays:       in.AvailableDays,
			SlotDurationMinutes: in.SlotDurationMinutes,
			GoogleCalendarID:    in.GoogleCalendarID,
			IsActive:            in.IsActive,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := toCalendarSummary(cal)
		return nil, &out, nil
	})
}

// calendarSummary is a schema-friendly projection of domain.Calendar
// (TimeSlots is legacy interface{} JSON — booking availability comes
// from Google Calendar, so it is deliberately omitted here).
type calendarSummary struct {
	ID                  string  `json:"id"`
	AgentID             string  `json:"agent_id"`
	Name                string  `json:"name"`
	AvailableDays       []int32 `json:"available_days"`
	SlotDurationMinutes int32   `json:"slot_duration_minutes"`
	GoogleCalendarID    *string `json:"google_calendar_id,omitempty"`
	BookingReady        bool    `json:"booking_ready"`
	IsActive            bool    `json:"is_active"`
}

func toCalendarSummary(c *domain.Calendar) calendarSummary {
	return calendarSummary{
		ID:                  c.ID,
		AgentID:             c.AgentID,
		Name:                c.Name,
		AvailableDays:       c.AvailableDays,
		SlotDurationMinutes: c.SlotDurationMinutes,
		GoogleCalendarID:    c.GoogleCalendarID,
		BookingReady:        c.IsActive && c.GoogleCalendarID != nil,
		IsActive:            c.IsActive,
	}
}

// --- team ---

func registerTeamTools(s *mcpsdk.Server, deps Deps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_team_members",
		Description: "Lista os membros da equipe (admins e atendentes) e o status de cada um.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		members, err := deps.ListTeamUC.Execute(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, members, nil
	})

	type inviteInput struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role" jsonschema:"papel do convidado: admin ou attendant"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "invite_team_member",
		Description: "Convida um membro para a equipe por e-mail. Confirme o e-mail e o papel com o " +
			"usuário antes de enviar (o convite dispara um e-mail real).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in inviteInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		member, err := deps.InviteTeamUC.Execute(ctx, usecases.InviteTeamMemberDTO{
			AccountID:   id.AccountID,
			InviterName: id.UserName,
			Name:        in.Name,
			Email:       in.Email,
			Role:        in.Role,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, member, nil
	})
}

// --- billing ---

func registerBillingTools(s *mcpsdk.Server, deps Deps) {
	type billingOutput struct {
		CurrentPlan    string        `json:"current_plan"`
		Currency       string        `json:"currency"`
		IsTrialing     bool          `json:"is_trialing"`
		AvailablePlans []planSummary `json:"available_plans"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_billing",
		Description: "Plano atual da conta e catálogo de planos disponíveis, com limites e preços.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, *billingOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		account, err := deps.AccountRepo.GetByID(ctx, id.AccountID)
		if err != nil || account == nil {
			return nil, nil, toolError(fmt.Errorf("conta não encontrada"))
		}
		plans, err := deps.PlanRepo.ListAll(ctx)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &billingOutput{
			Currency:   account.Currency,
			IsTrialing: account.SubscriptionStatus == "trialing",
		}
		for _, p := range plans {
			if !p.IsListed || p.Currency != account.Currency {
				continue
			}
			out.AvailablePlans = append(out.AvailablePlans, planSummary{
				ID:                       p.ID,
				Name:                     p.Name,
				PriceCents:               p.Price,
				Currency:                 p.Currency,
				MaxAgents:                p.MaxAgents,
				MaxChannels:              p.MaxChannels,
				MaxConversationsPerMonth: p.MaxConversationsPerMonth,
			})
			if account.PlanID != nil && p.ID == *account.PlanID {
				out.CurrentPlan = p.Name
			}
		}
		// Contract/unlisted plans still need naming.
		if out.CurrentPlan == "" && account.PlanID != nil {
			if p, err := deps.PlanRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
				out.CurrentPlan = p.Name
			}
		}
		return nil, out, nil
	})

	type checkoutInput struct {
		PlanID string `json:"plan_id" jsonschema:"plano de destino (id do catálogo de get_billing)"`
	}
	type checkoutOutput struct {
		CheckoutURL string `json:"checkout_url"`
		Message     string `json:"message"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_checkout_link",
		Description: "Gera um link de checkout (Stripe) para upgrade de plano. Envie o link ao usuário — " +
			"o pagamento acontece no navegador, nunca por aqui.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in checkoutInput) (*mcpsdk.CallToolResult, *checkoutOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		checkoutURL, err := deps.CheckoutUC.Execute(ctx, usecases.CreateCheckoutSessionDTO{
			AccountID:  id.AccountID,
			PlanID:     in.PlanID,
			SuccessURL: deps.DashboardURL + "/billing?upgraded=1",
			CancelURL:  deps.DashboardURL + "/billing",
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &checkoutOutput{
			CheckoutURL: checkoutURL,
			Message:     "Envie este link ao usuário para concluir o upgrade no navegador.",
		}, nil
	})
}

type planSummary struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	PriceCents               int    `json:"price_cents"`
	Currency                 string `json:"currency"`
	MaxAgents                int    `json:"max_agents"`
	MaxChannels              int    `json:"max_channels"`
	MaxConversationsPerMonth int    `json:"max_conversations_per_month"`
}
