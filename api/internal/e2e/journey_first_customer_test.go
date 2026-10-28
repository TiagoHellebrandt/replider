package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/replider/api/internal/adapters/repository/postgres"
	"github.com/replider/api/internal/core/domain"
)

// The whole first-customer journey, over HTTP, in the order a person does it.
//
// This test exists because three "dangling middle" bugs — the customer import
// with no door into the funnel, conversations.contact_ref_id indexed but never
// written, and a provider constructed and discarded in main.go — lived for
// months under 439 test files. Every one of those tests seeded the database
// into a state the product could not reach, so the missing middle was never
// walked. The rule here is therefore non-negotiable:
//
//	NO direct INSERT builds state. Every fact in the database was put there
//	by a real HTTP call, in the same order a human would make them.
//
// Reads (SELECT) are allowed for ASSERTIONS about what the pipeline wrote.
// If a step below cannot be done through a public route, the test fails with
// a message naming the missing route — that failure IS the finding.
//
// The only fakes are the process's real boundaries, wired in the harness at
// the same seams main.go uses: the AI provider (journey_harness_test.go, on
// by the account-name marker), the registrar's DNS, and the outbound mail hop.

const (
	journeyAccountName = "Clínica Jornada " + e2eAIAccountMarker
	journeyAdminEmail  = "dono@clinicajornada.com.br"
	journeyPassword    = "senha-forte-123"

	journeyFromAddress   = "vendas@clinicajornada.com.br"
	journeyCustomerEmail = "compras@hospitalsaolucas.com.br"
	journeyOptOutEmail   = "contato@clinicadovale.com.br"
)

// doMultipartFile posts bytes as the file upload the import routes expect.
func doMultipartFile(t *testing.T, path, filename string, data []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

func doMultipartCSV(t *testing.T, path, csv, token string) *httptest.ResponseRecorder {
	t.Helper()
	return doMultipartFile(t, path, "planilha.csv", []byte(csv), token)
}

// stepf turns a failed step into the finding it is: which user action broke,
// on which route.
func stepf(step, method, path string) string {
	return fmt.Sprintf(
		"JORNADA quebrada no passo %q (%s %s). Se a rota não existe ou não faz o que o passo precisa, ISSO é o achado: o caminho do usuário não fecha por HTTP.",
		step, method, path)
}

func TestJourney_FirstCustomerEndToEnd(t *testing.T) {
	cleanDB(t)
	e2eEmailOutbox.Reset()
	e2eDNSResolver.Reset()
	ctx := context.Background()

	// ── 1. A venda fechou: o dono emite o CONVITE ─────────────────────────
	// Fluxo sales-led (docs/fluxo-criacao-conta.md): a conta do cliente só
	// existe porque o dono decidiu — o convite de uso único carrega o plano
	// SDR contratado e é a permissão que passa pela porta fechada. Trial
	// DELIBERADAMENTE não envia abordagem fria, então nascer contratado é o
	// que faz o resto da jornada existir.
	registerUser(t, "Replider Platform", "owner@replider.com", journeyPassword)
	ownerToken := login(t, "owner@replider.com", journeyPassword)

	w0 := doRequest("POST", "/v1/owner/invites", map[string]any{
		"email":   journeyAdminEmail,
		"plan_id": "01PLAN_ENTERPRISE_00000",
	}, ownerToken)
	require.Equal(t, http.StatusCreated, w0.Code,
		stepf("emitir convite", "POST", "/v1/owner/invites")+" resposta: "+w0.Body.String())
	var created struct {
		Token  string `json:"token"`
		Invite struct {
			ID string `json:"id"`
		} `json:"invite"`
	}
	require.NoError(t, json.Unmarshal(w0.Body.Bytes(), &created))
	require.NotEmpty(t, created.Token, "o segredo aparece UMA vez, na emissão")

	// ── 1b. O cliente resgata: sonda o link, cadastra com o token ─────────
	w0 = doRequest("GET", "/v1/invites/probe?token="+created.Token, nil, "")
	require.Equal(t, http.StatusOK, w0.Code)
	require.Contains(t, w0.Body.String(), `"valid":true`)
	require.Contains(t, w0.Body.String(), journeyAdminEmail,
		"a sonda devolve o e-mail para a tela travar o campo")

	w0 = doRequest("POST", "/v1/accounts", map[string]any{
		"account_name":   journeyAccountName,
		"admin_name":     journeyAccountName,
		"admin_email":    journeyAdminEmail,
		"password":       journeyPassword,
		"terms_accepted": true,
		"invite_token":   created.Token,
	}, "")
	require.Equal(t, http.StatusCreated, w0.Code,
		stepf("cadastrar com convite", "POST", "/v1/accounts")+" resposta: "+w0.Body.String())
	token := login(t, journeyAdminEmail, journeyPassword)

	w0 = doRequest("GET", "/v1/me", nil, token)
	require.Equal(t, http.StatusOK, w0.Code, w0.Body.String())
	var me struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
	}
	require.NoError(t, json.Unmarshal(w0.Body.Bytes(), &me))
	require.NotEmpty(t, me.Account.ID)

	// ASSERÇÕES: a conta nasceu CONTRATADA (não trial) e o convite morreu no
	// uso — replay do mesmo link não cria uma segunda conta.
	var planID, subStatus string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT COALESCE(plan_id, ''), subscription_status FROM accounts WHERE id = $1`,
		me.Account.ID).Scan(&planID, &subStatus))
	assert.Equal(t, "01PLAN_ENTERPRISE_00000", strings.TrimSpace(planID),
		"a conta convidada nasce no plano do convite, sem passar por trial")
	assert.Equal(t, "active", subStatus)
	var usedBy *string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT used_account_id FROM account_invites WHERE id = $1`,
		created.Invite.ID).Scan(&usedBy))
	require.NotNil(t, usedBy)
	assert.Equal(t, me.Account.ID, *usedBy)

	w0 = doRequest("POST", "/v1/accounts", map[string]any{
		"account_name":   "Clone",
		"admin_name":     "Clone",
		"admin_email":    journeyAdminEmail,
		"password":       journeyPassword,
		"terms_accepted": true,
		"invite_token":   created.Token,
	}, "")
	require.Equal(t, http.StatusBadRequest, w0.Code,
		"replay do convite tem que morrer — one-shot é a propriedade central")
	require.Contains(t, w0.Body.String(), "INVITE_INVALID")

	// ── 2. Conectar o canal de e-mail ─────────────────────────────────────
	w := doRequest("POST", "/v1/channels", map[string]any{
		"name": "Comercial",
		"type": "email",
		"config": map[string]any{
			"from_address":  journeyFromAddress,
			"smtp_host":     "smtp.clinicajornada.com.br",
			"smtp_username": "vendas",
			"smtp_password": "segredo-smtp",
		},
	}, token)
	require.Equal(t, http.StatusCreated, w.Code,
		stepf("conectar canal de e-mail", "POST", "/v1/channels")+" resposta: "+w.Body.String())
	channel := parseJSON(t, w)
	channelID, _ := channel["id"].(string)
	webhookToken, _ := channel["webhook_token"].(string)
	require.NotEmpty(t, channelID)
	require.NotEmpty(t, webhookToken,
		"o canal criado não devolveu webhook_token — sem ele não há como o mundo externo entregar um inbound")

	// ── 2a. O funil padrão já aponta para o canal recém-criado ────────────
	// Ninguém configura funil na jornada real: o vínculo tem que nascer com o
	// canal (FunnelChannelLinker). Sem ele toda aprovação morria em
	// LEAD_FUNNEL_NO_CHANNEL — em prod 7 de 7 funis padrão estavam sem canal.
	w = doRequest("GET", "/v1/funnels", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("listar funis após criar o canal", "GET", "/v1/funnels")+" resposta: "+w.Body.String())
	var funnelsAfterChannel []struct {
		ID        string  `json:"id"`
		IsDefault bool    `json:"is_default"`
		ChannelID *string `json:"channel_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &funnelsAfterChannel))
	defaultLinked := false
	for _, f := range funnelsAfterChannel {
		if f.IsDefault {
			require.NotNil(t, f.ChannelID,
				"o funil padrão ficou sem canal depois de criar o canal de e-mail — o vínculo durável não aconteceu")
			assert.Equal(t, channelID, *f.ChannelID,
				"o funil padrão aponta para outro canal que não o de e-mail recém-criado")
			defaultLinked = true
		}
	}
	require.True(t, defaultLinked, "a conta não tem funil padrão logo após criar o canal")

	// ── 2b. Publicar e verificar o DNS do domínio remetente ───────────────
	// O mesmo ciclo do cliente real: a tela diz o que publicar, o registrador
	// (fake) recebe os registros, a verificação REAL confere e carimba a
	// mailbox — sem isso o approve-send recusa com dns_unverified.
	w = doRequest("GET", "/v1/channels/"+channelID+"/email-dns?verify=false", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("ler instruções de DNS", "GET", "/v1/channels/:id/email-dns")+" resposta: "+w.Body.String())
	var dnsSetup struct {
		Records []struct {
			Type  string `json:"type"`
			Host  string `json:"host"`
			Value string `json:"value"`
		} `json:"records"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &dnsSetup))
	require.NotEmpty(t, dnsSetup.Records, "a tela de DNS não devolveu registro nenhum")
	for _, rec := range dnsSetup.Records {
		e2eDNSResolver.Publish(rec.Type, rec.Host, rec.Value)
	}
	w = doRequest("GET", "/v1/channels/"+channelID+"/email-dns", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"verified":true`,
		"os registros publicados não verificaram — o carimbo dns_verified_at nunca acontece e todo cold send morre em dns_unverified")

	// ASSERÇÃO: a verificação CARIMBA o início do aquecimento — a coluna que
	// nunca teve escritor (a doença do contact_ref_id de novo). Antes do
	// conserto este SELECT devolvia NULL, e com a guarda fail-closed o
	// approve-send mais abaixo recusaria com EMAIL_WARMUP_NOT_STARTED.
	var warmupStarted *time.Time
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT warmup_started_at FROM email_mailboxes
		  WHERE account_id = $1 AND address = $2`,
		me.Account.ID, journeyFromAddress).Scan(&warmupStarted))
	require.NotNil(t, warmupStarted,
		"verificar a caixa deve iniciar o aquecimento — NULL aqui significa que a rampa nunca governa nada")

	// E a tela vê o MESMO estado que o portão de envio aplica: o canal de
	// e-mail carrega o aquecimento e o orçamento do dia.
	w = doRequest("GET", "/v1/channels", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"email_warmup"`,
		"o canal de e-mail expõe o estado de aquecimento para a tela")
	assert.Contains(t, w.Body.String(), `"warming":true`,
		"caixa recém-verificada está em rampa — dia 1")

	// ── 3. Importar produtos por CSV (com linha de preço ambíguo) ─────────
	productsCSV := strings.Join([]string{
		"nome,preco",
		"Tomógrafo Portátil TX-1,\"150.000,00\"",
		"Ventilador Pulmonar V-200,1.234", // ambíguo: 1234? 1,234? o parser deve recusar a adivinhar
	}, "\n")
	w = doMultipartCSV(t, "/v1/products/import/preview", productsCSV, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("pré-visualizar produtos", "POST", "/v1/products/import/preview")+" resposta: "+w.Body.String())
	var prodPreview struct {
		Rows []struct {
			Line       int    `json:"line"`
			Name       string `json:"name"`
			PriceCents int64  `json:"price_cents"`
			Currency   string `json:"currency"`
			Problem    string `json:"problem"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prodPreview))
	importableProducts := []map[string]any{}
	ambiguousFlagged := false
	for _, r := range prodPreview.Rows {
		if r.Problem == "" {
			importableProducts = append(importableProducts, map[string]any{
				"line": r.Line, "name": r.Name,
				"price_cents": r.PriceCents, "currency": r.Currency,
			})
		} else {
			ambiguousFlagged = true
		}
	}
	require.NotEmpty(t, importableProducts, "nenhum produto importável no preview: %s", w.Body.String())
	assert.True(t, ambiguousFlagged,
		"a linha de preço ambíguo (1.234) deveria vir marcada com problem — importá-la calada grava preço 1000x errado")
	w = doRequest("POST", "/v1/products/import", map[string]any{"rows": importableProducts}, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("importar produtos", "POST", "/v1/products/import")+" resposta: "+w.Body.String())

	// ── 3b. Importar produtos a partir do PDF do fabricante ───────────────
	// A marca não manda CSV — manda folheto. O PDF real (fixture) atravessa
	// os mesmos portões (antivírus, teto de páginas) e cai NA MESMA tela de
	// preview; só a fronteira do modelo é fake. Nada entra sem confirmação.
	pdfBytes, err := os.ReadFile("testdata/folheto_produtos.pdf")
	require.NoError(t, err)
	w = doMultipartFile(t, "/v1/products/import/pdf", "folheto_produtos.pdf", pdfBytes, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("extrair produtos do PDF", "POST", "/v1/products/import/pdf")+" resposta: "+w.Body.String())
	var pdfPreview struct {
		Rows []struct {
			Name       string `json:"name"`
			PriceCents int64  `json:"price_cents"`
			SKU        string `json:"sku"`
			Page       int    `json:"page"`
			Problem    string `json:"problem"`
			Duplicate  bool   `json:"duplicate"`
		} `json:"rows"`
		Importable int `json:"importable"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pdfPreview))
	require.Len(t, pdfPreview.Rows, 3)
	byName := map[string]int{}
	for i, r := range pdfPreview.Rows {
		byName[r.Name] = i
	}
	monitor := pdfPreview.Rows[byName["Monitor Multiparâmetro M-10"]]
	assert.Empty(t, monitor.Problem)
	assert.Equal(t, int64(3_250_000), monitor.PriceCents)
	assert.Equal(t, 1, monitor.Page, "cada linha carrega a página do folheto para o humano conferir")
	assert.Equal(t, domain.ProblemAmbiguousPrice,
		pdfPreview.Rows[byName["Bomba de Infusão B-3"]].Problem,
		"o preço 1.234 do folheto é recusado, não adivinhado — igual ao CSV")
	// O TX-1 já entrou pelo CSV acima — e mesmo assim a linha do FOLHETO é
	// importável, porque ela carrega o que o CSV não tinha: a página de onde
	// veio, e o vínculo com o arquivo arquivado. O dedup continua rodando (a
	// linha vem marcada como já existente, então a tela diz "vai atualizar" em
	// vez de "vai criar"); o que mudou é que "duplicado" deixou de significar
	// "descartável". Antes a regra comparava só preço e descrição, e por isso
	// reimportar a pasta de folhetos — a única ação que preencheria as fichas
	// técnicas — era exatamente a ação garantida a não fazer nada.
	tx1 := pdfPreview.Rows[byName["Tomógrafo Portátil TX-1"]]
	assert.Empty(t, tx1.Problem,
		"a linha do folheto atualiza o produto que veio do CSV, não é descartada")
	assert.True(t, tx1.Duplicate,
		"e a tela precisa dizer que é atualização, não criação")
	require.Equal(t, 2, pdfPreview.Importable)

	// Confirmação humana: a MESMA rota do CSV, só com as linhas aprovadas.
	m := pdfPreview.Rows[byName["Monitor Multiparâmetro M-10"]]
	w = doRequest("POST", "/v1/products/import", map[string]any{"rows": []map[string]any{{
		"line": 1, "name": m.Name, "price_cents": m.PriceCents,
		"currency": "BRL", "sku": m.SKU, "page": m.Page,
	}}}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"created":1`)
	// ASSERÇÃO (leitura): os campos do folheto aterrissam nos attributes.
	var skuAttr string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT attributes->>'sku' FROM products WHERE name = $1`, m.Name).Scan(&skuAttr))
	assert.Equal(t, "M10", skuAttr, "o SKU extraído do PDF é gravado no produto confirmado")

	// ── 4. Importar clientes por CSV (linhas sujas de propósito) ──────────
	customersCSV := strings.Join([]string{
		"empresa,nome,telefone,email",
		"Hospital São Lucas,Setor de Compras,+55 65 99999-0001," + journeyCustomerEmail,
		"Clínica do Vale,Financeiro,," + journeyOptOutEmail,                             // e-mail sem telefone
		"Hospital São Lucas,Setor de Compras,+55 65 99999-0001," + journeyCustomerEmail, // duplicada
		"Fisioterapia São João,João Açúcar,+55 65 99999-0002,",                          // acento, só telefone
	}, "\n")
	w = doMultipartCSV(t, "/v1/contacts/import/preview", customersCSV, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("pré-visualizar clientes", "POST", "/v1/contacts/import/preview")+" resposta: "+w.Body.String())
	var custPreview struct {
		Rows []struct {
			Line    int    `json:"line"`
			Company string `json:"company"`
			Name    string `json:"name"`
			Phone   string `json:"phone"`
			Email   string `json:"email"`
			Problem string `json:"problem"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &custPreview))
	importableCustomers := []map[string]any{}
	for _, r := range custPreview.Rows {
		if r.Problem == "" {
			importableCustomers = append(importableCustomers, map[string]any{
				"line": r.Line, "company": r.Company, "name": r.Name,
				"phone": r.Phone, "email": r.Email,
			})
		}
	}
	require.NotEmpty(t, importableCustomers, "nenhum cliente importável no preview: %s", w.Body.String())
	w = doRequest("POST", "/v1/contacts/import", map[string]any{"rows": importableCustomers}, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("importar clientes", "POST", "/v1/contacts/import")+" resposta: "+w.Body.String())
	var custResult struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &custResult))
	assert.GreaterOrEqual(t, custResult.Created, 3, "as três pessoas distintas viram contatos")

	// O contato importado, achado pela LISTA (não por SQL): é assim que a
	// tela de Clientes o encontraria.
	w = doRequest("GET", "/v1/contacts?limit=50", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("listar contatos", "GET", "/v1/contacts")+" resposta: "+w.Body.String())
	var contacts []struct {
		ID    string  `json:"id"`
		Email *string `json:"email"`
		Name  *string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &contacts))
	customerContactID := ""
	customerContactCount := 0
	valeContactID := ""
	accentImported := false
	for _, ct := range contacts {
		if ct.Email != nil && *ct.Email == journeyCustomerEmail {
			customerContactID = ct.ID
			customerContactCount++
		}
		if ct.Email != nil && *ct.Email == journeyOptOutEmail {
			valeContactID = ct.ID
		}
		if ct.Name != nil && strings.Contains(*ct.Name, "Açúcar") {
			accentImported = true
		}
	}
	require.NotEmpty(t, valeContactID, "a cliente só-email também virou contato")
	require.NotEmpty(t, customerContactID, "o cliente importado não aparece na lista de contatos")
	// A linha duplicada pode cair no preview ou no import — o invariante que
	// importa é que o cliente exista UMA vez na base.
	assert.Equal(t, 1, customerContactCount,
		"a linha duplicada do CSV virou um segundo contato — a resposta dele vai parar num estranho")
	assert.True(t, accentImported, "o nome acentuado sobreviveu ao import sem virar mojibake")

	// ── 5. Contato importado → lead no funil padrão ───────────────────────
	w = doRequest("GET", "/v1/funnels", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("listar funis", "GET", "/v1/funnels")+" resposta: "+w.Body.String())
	var funnels []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &funnels))
	funnelID := ""
	for _, f := range funnels {
		if f.IsDefault {
			funnelID = f.ID
		}
	}
	require.NotEmpty(t, funnelID,
		"a conta recém-criada não tem funil padrão — o import de clientes fica sem porta para o funil (o bug original)")
	// O funil já sabe POR ONDE aborda: o vínculo com o canal de e-mail nasceu
	// na criação do canal (passo 2a) — nenhum PUT manual aqui.

	// Desde 2026-09-01 o IMPORT já criou o lead no funil padrão (a tela de
	// contatos foi fundida na Carteira; nenhum segundo gesto). A rota
	// from-contacts continua idempotente: reconhece o lead existente e
	// devolve o id em vez de duplicar.
	w = doRequest("POST", "/v1/funnels/"+funnelID+"/leads/from-contacts", map[string]any{
		"contact_ids": []string{customerContactID},
	}, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("transformar contato em lead", "POST", "/v1/funnels/:id/leads/from-contacts")+" resposta: "+w.Body.String())
	var ctf struct {
		Created int      `json:"created"`
		LeadIDs []string `json:"lead_ids"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ctf))
	require.Equal(t, 0, ctf.Created,
		"o import já criou o lead — um segundo from-contacts não pode duplicar: "+w.Body.String())
	require.Len(t, ctf.LeadIDs, 1)
	leadID := ctf.LeadIDs[0]

	// ASSERÇÃO: o lead aponta para O MESMO contato importado — sem isso a
	// resposta do cliente nunca acha o lead.
	w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	lead := parseJSON(t, w)
	assert.Equal(t, customerContactID, lead["contact_id"],
		"o lead criado de contato importado deve carregar o contact_id do contato")

	// ── 6. Enriquecer ─────────────────────────────────────────────────────
	w = doRequest("POST", "/v1/leads/"+leadID+"/enrich", nil, token)
	require.Equal(t, http.StatusAccepted, w.Code,
		stepf("enriquecer o lead", "POST", "/v1/leads/:id/enrich")+" resposta: "+w.Body.String())
	deadline := time.Now().Add(5 * time.Second)
	for {
		w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
		require.Equal(t, http.StatusOK, w.Code)
		lead = parseJSON(t, w)
		if lead["status"] == "enriched" {
			break
		}
		require.True(t, time.Now().Before(deadline),
			"o enriquecimento não terminou: status=%v — o 202 prometeu um trabalho que nunca aterrissa", lead["status"])
		time.Sleep(50 * time.Millisecond)
	}

	// ── 7. Gerar rascunho ─────────────────────────────────────────────────
	w = doRequest("POST", "/v1/leads/"+leadID+"/draft", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("gerar rascunho por IA", "POST", "/v1/leads/:id/draft")+" resposta: "+w.Body.String())
	lead = parseJSON(t, w)
	require.Equal(t, "pending_approval", lead["status"], "o rascunho entra na fila de aprovação")
	require.Equal(t, e2eFakeDraftText, lead["approach_draft"])

	// ── 8. Aprovar e enviar (texto final editado pelo humano) ─────────────
	finalText := "Olá! Somos a Clínica Jornada e fornecemos equipamentos hospitalares com assistência em Cuiabá. Podemos conversar na próxima semana?"
	w = doRequest("POST", "/v1/leads/"+leadID+"/approve-send", map[string]any{
		"text":    finalText,
		"subject": "Equipamentos com assistência local",
	}, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("aprovar e enviar", "POST", "/v1/leads/:id/approve-send")+" resposta: "+w.Body.String())
	sends := e2eEmailOutbox.Sends()
	require.Len(t, sends, 1, "exatamente um e-mail saiu")
	assert.Equal(t, journeyCustomerEmail, sends[0].To)
	assert.Equal(t, finalText, sends[0].Body, "o que sai é o texto que o humano aprovou, não o rascunho")
	w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "contacted", parseJSON(t, w)["status"])

	// ── 9. A resposta chega PELO WEBHOOK, nunca por INSERT ────────────────
	replyText := "Recebi, temos interesse. Pode mandar a proposta do tomógrafo?"
	w = postInboundEmail(t, webhookToken, map[string]any{
		"from":       "Compras <" + journeyCustomerEmail + ">",
		"to":         journeyFromAddress,
		"subject":    "Re: Equipamentos com assistência local",
		"text":       replyText,
		"message_id": "<jornada-resposta-1@hospitalsaolucas.com.br>",
	})
	require.Equal(t, http.StatusOK, w.Code,
		stepf("resposta do cliente via webhook", "POST", "/v1/webhooks/:token")+" resposta: "+w.Body.String())

	// ASSERÇÃO (leitura, não fixture): o pipeline carimbou contact_ref_id —
	// a coluna que ficou NULL de 000009 a 000196 e deixou timeline e guarda
	// LGPD juntando em nada.
	var refID *string
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT contact_ref_id FROM conversations WHERE channel_id = $1`, channelID).Scan(&refID))
	require.NotNil(t, refID, "conversations.contact_ref_id ficou NULL depois do inbound — a ponta voltou a se soltar")
	assert.Equal(t, customerContactID, *refID,
		"contact_ref_id aponta para o contato importado, não para um estranho")

	// O funil não pode mentir: a resposta que chegou pelo webhook avança o
	// lead para "responded". Antes desta correção o hook tentava casar/criar
	// o lead pelo remetente cru (violando leads_contact_id_fkey) e o lead de
	// e-mail ficava em "contacted" para sempre — a vendedora via a resposta
	// na timeline e o funil dizia que ninguém respondeu.
	w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "responded", parseJSON(t, w)["status"],
		"o lead do contato que respondeu deve estar em responded — funil que mente destrói a confiança")

	// ── 10. A timeline do lead mostra a resposta ──────────────────────────
	w = doRequest("GET", "/v1/leads/"+leadID+"/timeline", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("ler a timeline do lead", "GET", "/v1/leads/:id/timeline")+" resposta: "+w.Body.String())
	var timeline struct {
		Messages []struct {
			Author string `json:"author"`
			Text   string `json:"text"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &timeline))
	foundReply := false
	for _, m := range timeline.Messages {
		if m.Text == replyText {
			foundReply = true
			assert.Equal(t, "customer", m.Author)
		}
	}
	require.True(t, foundReply,
		"a resposta que chegou pelo webhook não aparece na timeline DESTE lead — era exatamente o sintoma do contact_ref_id órfão")

	// ── 11. Abrir a conversa a partir do lead ─────────────────────────────
	w = doRequest("GET", "/v1/leads/"+leadID, nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	lead = parseJSON(t, w)
	conversationID, _ := lead["conversation_id"].(string)
	require.NotEmpty(t, conversationID,
		"o lead não expõe conversation_id — o painel do lead é um beco sem saída para a conversa")
	// Não existe GET /v1/conversations/:id — o app abre a thread lendo as
	// mensagens, então a jornada abre do mesmo jeito.
	w = doRequest("GET", "/v1/conversations/"+conversationID+"/messages", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("abrir a conversa do lead", "GET", "/v1/conversations/:id/messages")+" resposta: "+w.Body.String())
	assert.Contains(t, w.Body.String(), replyText,
		"a conversa aberta a partir do lead mostra a resposta do cliente")

	// ── 12. Opt-out não recebe auto-resume, e o lead vai para "lost" ──────
	// A segunda cliente importada também está no funil — o PARAR precisa
	// tirá-la da esteira de abordagens, não só pausar a conversa.
	w = doRequest("POST", "/v1/funnels/"+funnelID+"/leads/from-contacts", map[string]any{
		"contact_ids": []string{valeContactID},
	}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var valeCtf struct {
		LeadIDs []string `json:"lead_ids"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &valeCtf))
	require.Len(t, valeCtf.LeadIDs, 1)
	valeLeadID := valeCtf.LeadIDs[0]

	// Ela pede para parar — pelo webhook, como na vida real. O opt-out pausa
	// o bot, carimba o contato e mata o lead.
	w = postInboundEmail(t, webhookToken, map[string]any{
		"from":       "Financeiro <" + journeyOptOutEmail + ">",
		"to":         journeyFromAddress,
		"subject":    "PARAR",
		"text":       "PARAR",
		"message_id": "<jornada-optout-1@clinicadovale.com.br>",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var optOutConvID string
	var optOutStamped bool
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT c.id, ct.opted_out_at IS NOT NULL
		   FROM conversations c JOIN contacts ct ON ct.id = c.contact_ref_id
		  WHERE c.channel_id = $1 AND c.contact_id = $2`,
		channelID, journeyOptOutEmail).Scan(&optOutConvID, &optOutStamped))
	require.True(t, optOutStamped,
		"o PARAR pelo webhook não carimbou opted_out_at no contato que a conversa referencia — a guarda LGPD do auto-resume lê exatamente esse join")

	// ASSERÇÃO (Item 3): o lead de quem pediu PARAR sai da esteira. Antes do
	// conserto o hook procurava o lead pelo remetente cru e nunca achava o
	// lead chaveado por ULID — o funil seguia oferecendo para quem disse não.
	w = doRequest("GET", "/v1/leads/"+valeLeadID, nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "lost", parseJSON(t, w)["status"],
		"o PARAR deve mover o lead para lost — um funil que continua abordando quem optou por sair é violação, não persistência")

	// A conversa do Hospital São Lucas é assumida por um humano (pausa o
	// bot) e abandonada — o cenário que o auto-resume existe para consertar.
	w = doRequest("POST", "/v1/conversations/"+conversationID+"/assign", nil, token)
	require.Equal(t, http.StatusOK, w.Code,
		stepf("assumir a conversa", "POST", "/v1/conversations/:id/assign")+" resposta: "+w.Body.String())

	// O worker roda (mesma chamada do cron, janela zero — todo estado acima
	// nasceu por HTTP): a abandonada volta, a do opt-out fica parada.
	// Um chat abandonado ao lado, para que a query prove que ainda retoma
	// alguma coisa: e-mail fica de fora do auto-resume (a IA nunca responde
	// e-mail, então um e-mail pausado é um humano que assumiu de propósito).
	chatConvID := telegramInboundThenPause(t)
	resumed, err := postgres.NewConversationRepository(testPool).
		AutoResumeStuckConversations(ctx, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resumed, int64(1),
		"nenhuma conversa voltou — o teste estaria passando sobre uma query que não retoma nada")

	var paused bool
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT is_bot_paused FROM conversations WHERE id = $1`, optOutConvID).Scan(&paused))
	assert.True(t, paused,
		"a conversa de quem pediu PARAR foi retomada pelo auto-resume — violação de LGPD, a guarda por contact_ref_id falhou")
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT is_bot_paused FROM conversations WHERE id = $1`, conversationID).Scan(&paused))
	assert.True(t, paused, "a conversa de e-mail assumida por um humano fica com o humano — o auto-resume não mexe em e-mail")
	require.NoError(t, testPool.QueryRow(ctx,
		`SELECT is_bot_paused FROM conversations WHERE id = $1`, chatConvID).Scan(&paused))
	assert.False(t, paused, "a conversa de chat abandonada sem opt-out volta para o bot")
}
