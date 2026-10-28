package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
)

// Universal export (§6.18, I3).
//
// Same pipeline as the download button, including the audit row: an export run
// by the copilot is as traceable as one run by a person, which is the only way
// the trail is worth keeping.
func registerExportTools(s *mcpsdk.Server, deps Deps) {
	if deps.ExportUC == nil {
		return
	}

	type exportInput struct {
		Entity  string            `json:"entity" jsonschema:"o que exportar: leads | contacts | sales | conversations | signals"`
		Format  string            `json:"format,omitempty" jsonschema:"csv (padrão) ou xlsx"`
		Filters map[string]string `json:"filters,omitempty" jsonschema:"filtros da tela, ex: {\"status\":\"responded\"}. Filtro que a entidade não conhece é ignorado."`
		From    string            `json:"from,omitempty" jsonschema:"início do período (YYYY-MM-DD)"`
		To      string            `json:"to,omitempty" jsonschema:"fim do período, exclusivo (YYYY-MM-DD)"`
		Limit   int               `json:"limit,omitempty" jsonschema:"teto de linhas (padrão 5000, máximo 50000)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "export_data",
		Description: "Exporta uma lista em CSV ou XLSX, aplicando os mesmos filtros que a tela aplicaria. " +
			"Todo download é REGISTRADO (quem, o quê, com quais filtros, quantas linhas) — diga isso ao " +
			"usuário quando ele exportar dado de cliente. " +
			"Se `truncated` vier true, o arquivo bateu no teto de linhas e NÃO é a lista inteira: avise, " +
			"em vez de deixar a pessoa conferir contra um painel e concluir que o painel está errado.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in exportInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}

		req := domain.ExportRequest{
			AccountID: id.AccountID,
			UserID:    id.UserID,
			Entity:    domain.ExportEntity(strings.TrimSpace(in.Entity)),
			Format:    domain.ExportFormat(strings.TrimSpace(in.Format)),
			Filters:   in.Filters,
			Limit:     in.Limit,
		}
		if from, err := parseExportDay(in.From); err != nil {
			return nil, nil, err
		} else if from != nil {
			req.From = from
		}
		if to, err := parseExportDay(in.To); err != nil {
			return nil, nil, err
		} else if to != nil {
			req.To = to
		}

		result, err := deps.ExportUC.Execute(ctx, req)
		if err != nil {
			return nil, nil, toolError(err)
		}

		// The file goes back base64-encoded: an MCP result is JSON, and a
		// spreadsheet is bytes. The row count and the truncation flag travel
		// beside it so the model can describe the file without decoding it.
		return nil, map[string]any{
			"filename":       result.Filename,
			"content_type":   result.ContentType,
			"rows":           result.Rows,
			"truncated":      result.Truncated,
			"content_base64": base64.StdEncoding.EncodeToString(result.Body),
		}, nil
	})
}

// parseExportDay reads a YYYY-MM-DD bound.
//
// Refused rather than ignored when unparseable: an export silently covering a
// period nobody asked for is a file that gets forwarded to somebody.
func parseExportDay(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("data inválida %q — use YYYY-MM-DD", raw)
}
