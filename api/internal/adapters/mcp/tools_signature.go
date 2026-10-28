package mcp

import (
	"context"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Electronic signature (§6.11, G7).
//
// Admin-only. create_signature_request puts a real document in front of a real
// person and can end in a Sale that lands in somebody's commission — so the tool
// description tells the co-pilot to confirm the signers and the value out loud
// first, the same rule that governs every outbound send in this product.
func registerSignatureTools(s *mcpsdk.Server, deps Deps) {
	if deps.SignatureUC == nil {
		return
	}

	type signerIn struct {
		Name  string `json:"name" jsonschema:"nome de quem assina"`
		Email string `json:"email,omitempty" jsonschema:"e-mail para receber o link"`
		Phone string `json:"phone,omitempty" jsonschema:"telefone em E.164 para receber por WhatsApp"`
	}
	type signerOut struct {
		Name   string `json:"name"`
		Status string `json:"status,omitempty"`
		// SignURL is the authority to sign. Returned so a PERSON can hand it over;
		// the co-pilot is told never to send it to the customer itself.
		SignURL string `json:"sign_url,omitempty"`
	}
	type requestOut struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
		// SaleID says the signature already became a Sale — the fact that makes
		// this feature worth having.
		SaleID string `json:"sale_id,omitempty"`
		// SignedAt and ValueCents let the co-pilot say what closed and for how much.
		SignedAt   string      `json:"signed_at,omitempty"`
		ValueCents int64       `json:"value_cents"`
		Signers    []signerOut `json:"signers,omitempty"`
		// FailureReason is our side failing (a rejected token, a provider outage),
		// which is different from the customer refusing to sign.
		FailureReason string `json:"failure_reason,omitempty"`
	}

	summarize := func(r *domain.SignatureRequest) requestOut {
		out := requestOut{
			ID: r.ID, Title: r.Title, Status: string(r.Status),
			SaleID: r.SaleID, ValueCents: r.ValueCents,
			FailureReason: r.FailureReason,
		}
		if r.SignedAt != nil {
			out.SignedAt = r.SignedAt.Format(time.RFC3339)
		}
		for _, signer := range r.Signers {
			out.Signers = append(out.Signers, signerOut{
				Name: signer.Name, Status: signer.Status, SignURL: signer.SignURL,
			})
		}
		return out
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_signature_request",
		Description: "Envia um documento para assinatura eletrônica na conta do PRÓPRIO cliente " +
			"(consome a franquia de assinaturas dele). Confirme com o usuário, ANTES de chamar: " +
			"o documento, quem vai assinar e o valor — porque ao ser assinado isso registra uma " +
			"VENDA, que entra na carteira e na comissão de alguém. Informe value_cents em " +
			"CENTAVOS, ou 0 para apenas acompanhar a assinatura sem registrar venda. " +
			"O link de assinatura volta na resposta para uma PESSOA entregar; você não envia.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		ContractID string     `json:"contract_id,omitempty" jsonschema:"id do contrato cujo PDF será assinado"`
		ObjectPath string     `json:"object_path,omitempty" jsonschema:"caminho do PDF já enviado, quando não é um contrato"`
		Title      string     `json:"title,omitempty" jsonschema:"nome do documento (padrão: título do contrato)"`
		ContactID  string     `json:"contact_id,omitempty" jsonschema:"cliente da venda que a assinatura vai gerar"`
		ValueCents int64      `json:"value_cents,omitempty" jsonschema:"valor em CENTAVOS; 0 acompanha sem registrar venda"`
		Signers    []signerIn `json:"signers" jsonschema:"quem assina"`
	}) (*mcpsdk.CallToolResult, *requestOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		input := usecases.SignatureInput{
			ContractID: in.ContractID, ObjectPath: in.ObjectPath,
			Title: in.Title, ContactID: in.ContactID, ValueCents: in.ValueCents,
		}
		for _, signer := range in.Signers {
			input.Signers = append(input.Signers, domain.SignatureSigner{
				Name:  strings.TrimSpace(signer.Name),
				Email: strings.TrimSpace(signer.Email),
				Phone: strings.TrimSpace(signer.Phone),
			})
		}

		req, err := deps.SignatureUC.Send(ctx, id.AccountID, id.UserID, input)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := summarize(req)
		return nil, &out, nil
	})

	type statusOut struct {
		Requests []requestOut `json:"requests"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "get_signature_status",
		Description: "Situação das assinaturas: enviado, visto, assinado, recusado ou expirado. " +
			"Com signature_id, relê a situação direto no provedor (fonte da verdade) e, se " +
			"tiver sido assinado, arquiva a via assinada e registra a venda. " +
			"\"Visto e não assinado\" é o estado que muda uma ligação de follow-up — vale dizer.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in struct {
		SignatureID string `json:"signature_id,omitempty" jsonschema:"id do pedido; vazio lista os últimos"`
		Limit       int    `json:"limit,omitempty" jsonschema:"máximo de pedidos (padrão 20)"`
	}) (*mcpsdk.CallToolResult, *statusOut, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}

		if strings.TrimSpace(in.SignatureID) != "" {
			// Asking about ONE document re-reads it from the provider: the webhook
			// can be missed, and a stale "enviado" on a document signed an hour ago
			// is the answer that makes somebody call the wrong customer.
			req, err := deps.SignatureUC.Refresh(ctx, id.AccountID, in.SignatureID)
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, &statusOut{Requests: []requestOut{summarize(req)}}, nil
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > maxConversationPageSize {
			limit = maxConversationPageSize
		}
		list, err := deps.SignatureUC.List(ctx, id.AccountID, limit, 0)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &statusOut{Requests: make([]requestOut, 0, len(list))}
		for _, req := range list {
			out.Requests = append(out.Requests, summarize(req))
		}
		return nil, out, nil
	})
}
