package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// registerLogTools exposes the production log for remote diagnosis.
//
// Without these tools, investigating a production failure meant SSH onto the VPS
// and `docker logs` — and by then the line that mattered had already rolled out
// of the 10MB file. They read the app_logs table, which keeps WARN and above for
// seven days, with attributes already through PII redaction.
//
// A nil repo leaves the tools off the list: a log tool that reaches no log would
// only make the model waste a turn discovering that.
func registerLogTools(s *mcpsdk.Server, deps Deps) {
	if deps.AppLogUC == nil {
		return
	}

	type logsInput struct {
		Mode      string `json:"mode,omitempty" jsonschema:"o que devolver: recent (linhas, padrão) | summary (agrupado por falha)"`
		Minutes   int    `json:"minutes,omitempty" jsonschema:"janela em minutos olhando para trás (padrão 60, máximo 10080 = 7 dias)"`
		Level     string `json:"level,omitempty" jsonschema:"filtra por nível exato: WARN | ERROR"`
		Contains  string `json:"contains,omitempty" jsonschema:"texto a procurar na mensagem ou nos atributos"`
		RequestID string `json:"request_id,omitempty" jsonschema:"amarra ao id que o cliente recebeu no cabeçalho X-Request-ID ou no corpo do erro"`
		AccountID string `json:"account_id,omitempty" jsonschema:"restringe a uma conta"`
		Limit     int    `json:"limit,omitempty" jsonschema:"quantas linhas (padrão 50, máximo 200)"`
	}

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "query_logs",
		Description: "Log de produção da API, para diagnosticar falhas. Guarda WARN e ERROR dos " +
			"últimos 7 dias. mode=summary agrupa por falha (qual, quantas vezes, quando foi a " +
			"última) e é por onde começar: mode=recent devolve linha a linha e serve depois, " +
			"quando já se sabe o que procurar. Amarre ao relato de quem viu o erro pelo " +
			"request_id — é o mesmo id do cabeçalho X-Request-ID e do corpo dos erros 5xx. " +
			"Os atributos passam pela redação de PII, então telefone, e-mail e CPF vêm como " +
			"[REDACTED]: a ausência é proposital, não perda de dado. " +
			"Só operadores da plataforma leem — para os demais a ferramenta recusa, e recusa " +
			"não é o mesmo que 'não há nada acontecendo'.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in logsInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}

		minutes := in.Minutes
		if minutes <= 0 {
			minutes = 60
		}
		if maxMinutes := int(domain.AppLogRetention / time.Minute); minutes > maxMinutes {
			// Asking for more than exists would return a window promising a past
			// the table does not keep.
			minutes = maxMinutes
		}
		since := time.Now().Add(-time.Duration(minutes) * time.Minute)

		if strings.EqualFold(in.Mode, "summary") {
			groups, err := deps.AppLogUC.Summarize(ctx, id.UserID, since, time.Time{}, in.Limit)
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, struct {
				Window string                `json:"window"`
				Note   string                `json:"note,omitempty"`
				Groups []*domain.AppLogGroup `json:"groups"`
			}{
				Window: fmt.Sprintf("últimos %d minutos", minutes),
				Note: noteFor(len(groups),
					"Nenhum WARN ou ERROR na janela — o silêncio aqui é ausência de falha registrada, não prova de que tudo funcionou."),
				Groups: groups,
			}, nil
		}

		logs, err := deps.AppLogUC.Query(ctx, id.UserID, domain.AppLogFilter{
			Since:     since,
			Level:     in.Level,
			Contains:  in.Contains,
			RequestID: in.RequestID,
			AccountID: in.AccountID,
			Limit:     in.Limit,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, struct {
			Window string           `json:"window"`
			Note   string           `json:"note,omitempty"`
			Lines  []*domain.AppLog `json:"lines"`
		}{
			Window: fmt.Sprintf("últimos %d minutos", minutes),
			Note: noteFor(len(logs),
				"Nenhuma linha com esse recorte. Amplie a janela (minutes) ou tire um filtro antes de concluir que não houve falha."),
			Lines: logs,
		}, nil
	})
}

// noteFor returns the caveat only when there is nothing to show. An empty result
// has to say what it means: without that, "no lines" reads as "everything is
// fine", which is a different conclusion and sometimes a false one.
func noteFor(n int, empty string) string {
	if n == 0 {
		return empty
	}
	return ""
}
