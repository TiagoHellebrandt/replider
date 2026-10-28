package usecases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// Importing the price list a distributor does NOT have as a spreadsheet.
//
// Brands hand out brochures as PDFs; the distributor's "catalogue" is a folder
// of them. This use case turns one brochure into the SAME preview rows the CSV
// import produces, through the account's own AI provider (BYOK — the cost is
// on the customer's key), and then hands off to the SAME human confirmation:
// nothing enters the catalogue that a person did not review. The extraction is
// asked for strict JSON, forbidden to invent, and an ambiguous price is marked
// exactly like the CSV parser marks it — refused, never guessed.

var (
	ErrProductPDFImportUnavailable = &domain.AppError{
		Code:    "PRODUCT_PDF_IMPORT_UNAVAILABLE",
		Message: "importação por PDF não está configurada neste ambiente",
	}
	ErrProductPDFScanUnavailable = &domain.AppError{
		Code: "PRODUCT_PDF_SCAN_UNAVAILABLE",
		Message: "o antivírus não está configurado — upload de PDF desabilitado. " +
			"Fale com o suporte",
	}
	ErrProductPDFInfected = &domain.AppError{
		Code:    "PRODUCT_PDF_INFECTED",
		Message: "o antivírus recusou este arquivo",
	}
	ErrProductPDFUnreadable = &domain.AppError{
		Code:    "PRODUCT_PDF_UNREADABLE",
		Message: "não consegui abrir o arquivo como PDF. Envie o folheto original em PDF",
	}
	ErrProductPDFProviderCantRead = &domain.AppError{
		Code: "PRODUCT_PDF_PROVIDER_CANT_READ",
		Message: "o provedor de IA desta conta não lê arquivos. " +
			"Use o import por CSV ou configure um provedor com leitura de documentos",
	}
	ErrProductPDFNoProducts = &domain.AppError{
		Code:    "PRODUCT_PDF_NO_PRODUCTS",
		Message: "a leitura não encontrou nenhum produto neste PDF",
	}
	ErrProductPDFExtractFailed = &domain.AppError{
		Code:    "PRODUCT_PDF_EXTRACT_FAILED",
		Message: "a leitura automática do PDF falhou; tente de novo ou use o import por CSV",
	}
	// Quota and timeout used to be flattened into ExtractFailed. Both have an
	// action behind them and the generic message hid it: importing 19
	// brochures, the third hit the free-tier quota and the other sixteen were
	// then ground through, one doomed model call at a time, to arrive at
	// "tente de novo" — advice that could not work.
	ErrProductPDFQuotaExceeded = &domain.AppError{
		Code: "PRODUCT_PDF_QUOTA_EXCEEDED",
		Message: "a cota da chave de IA desta conta acabou. " +
			"Ative o faturamento na chave do Google AI Studio (o plano gratuito " +
			"tem limite baixo) ou espere a cota renovar",
	}
	ErrProductPDFTimeout = &domain.AppError{
		Code: "PRODUCT_PDF_TIMEOUT",
		Message: "este PDF é grande demais para ser lido dentro do tempo. " +
			"Envie ele sozinho, divida o arquivo, ou use o import por CSV",
	}
)

// classifyExtractionError turns a provider failure into an answer with an
// action behind it. Matching on message text rather than typed errors because
// both SDKs return opaque wrappers whose types drift between minor versions —
// the same reasoning as isTransient in the retry decorator.
func classifyExtractionError(err error) *domain.AppError {
	if err == nil {
		return ErrProductPDFExtractFailed
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrProductPDFTimeout
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "quota") || strings.Contains(msg, "error 429") ||
		strings.Contains(msg, "resource_exhausted"):
		// The provider says how long to wait ("Please retry in 57.4s"). That
		// number is the whole difference between "try again" and "try again
		// at 14:05", so carry it when it is there.
		if wait := providerRetryAfter(err.Error()); wait != "" {
			return &domain.AppError{
				Code:    ErrProductPDFQuotaExceeded.Code,
				Message: ErrProductPDFQuotaExceeded.Message + " (o provedor pede " + wait + ")",
			}
		}
		return ErrProductPDFQuotaExceeded
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout"):
		return ErrProductPDFTimeout
	}
	return ErrProductPDFExtractFailed
}

var retryInRe = regexp.MustCompile(`(?i)retry in ([0-9]+)(?:\.[0-9]+)?s`)

// providerRetryAfter extracts the wait the provider asked for, rounded to
// whole seconds. Empty when it did not say.
func providerRetryAfter(msg string) string {
	m := retryInRe.FindStringSubmatch(msg)
	if len(m) != 2 {
		return ""
	}
	return m[1] + "s"
}

// productPDFStore is the slice of the storage adapter the upload needs.
type productPDFStore interface {
	UploadBytes(ctx context.Context, objectPath string, data []byte, contentType string) error
}

// pdfPageCounter reports a PDF's page count. Injected (services.PDFPageCount
// in production) so the use case stays free of the pdf library import.
type pdfPageCounter func(data []byte) (int, error)

// PDFImage is one picture out of a brochure, as the extractor hands it over.
// Mirrored here (rather than importing services) to keep the dependency
// direction: usecases must not import the adapters that serve them.
type PDFImage struct {
	Page        int
	Width       int
	Height      int
	ContentType string
	SHA256      string
	Data        []byte
}

// pdfImageExtractor pulls the pictures out. Injected like pageCount.
type pdfImageExtractor func(data []byte) ([]PDFImage, error)

type ImportProductsPDFUseCase struct {
	// csv is the CSV import use case, reused for the dedup preload and the
	// preview counting — one implementation of "já está no catálogo".
	csv      *ImportProductsUseCase
	accounts domain.AccountRepository
	plans    domain.PlanRepository
	ai       domain.AIFactory

	pageCount pdfPageCounter
	maxBytes  int64
	maxPages  int

	// Optional wiring, same seams the contract extraction uses.
	sources domain.ProductSourceRepository
	images  pdfImageExtractor
	// dispatch runs the picture job off the request. Overridden in tests to
	// run inline; see SetImageExtractor.
	dispatch func(func())
	store    productPDFStore
	scanner  domain.AttachmentScanner
	meter    *AIMeter
	spend    *spendGate
}

func NewImportProductsPDFUseCase(
	csv *ImportProductsUseCase,
	accounts domain.AccountRepository,
	plans domain.PlanRepository,
	ai domain.AIFactory,
	pageCount pdfPageCounter,
	maxBytes int64,
	maxPages int,
) *ImportProductsPDFUseCase {
	return &ImportProductsPDFUseCase{
		csv: csv, accounts: accounts, plans: plans, ai: ai,
		pageCount: pageCount, maxBytes: maxBytes, maxPages: maxPages,
	}
}

// SetSources wires the brochure archive. Without it the file is still
// uploaded, just unreachable — which is exactly the state this replaced.
func (uc *ImportProductsPDFUseCase) SetSources(r domain.ProductSourceRepository) {
	uc.sources = r
}

// SetImageExtractor wires brochure-picture extraction.
//
// The job runs DETACHED: the person is waiting for the preview, and decoding
// forty images and pushing them to a bucket has nothing to do with the rows
// they are about to review. It also means a bucket outage delays pictures
// rather than failing an import.
func (uc *ImportProductsPDFUseCase) SetImageExtractor(f pdfImageExtractor) {
	uc.images = f
	if uc.dispatch == nil {
		uc.dispatch = func(fn func()) { go fn() }
	}
}

// SetImageDispatch overrides how the picture job is scheduled. Tests run it
// inline; production leaves the default goroutine.
func (uc *ImportProductsPDFUseCase) SetImageDispatch(d func(func())) { uc.dispatch = d }

// SetStore wires the audit copy of the upload (best-effort — see Extract).
func (uc *ImportProductsPDFUseCase) SetStore(s productPDFStore) { uc.store = s }

// SetScanner wires the antivirus. Extract REFUSES when this is nil: an
// unscanned upload pipeline is not a degraded mode, it is an open door.
func (uc *ImportProductsPDFUseCase) SetScanner(s domain.AttachmentScanner) { uc.scanner = s }

// SetMeter wires BYOK cost attribution for the extraction call.
func (uc *ImportProductsPDFUseCase) SetMeter(m *AIMeter) { uc.meter = m }

// SetSpendCounter wires the per-plan document-read ceiling and the account's
// AI budget (K4) — a brochure read shares the SpendDocumentExtraction unit.
func (uc *ImportProductsPDFUseCase) SetSpendCounter(c domain.SpendCounter) {
	uc.spend = newSpendGate(uc.accounts, uc.plans, c)
}

// productPDFSystemPrompt asks for data, not prose. The price is requested
// VERBATIM as printed so the ambiguity check happens HERE, by the same
// ParseMoney the CSV import trusts — a model that "helpfully" normalises
// "1.234" has already guessed.
const productPDFSystemPrompt = `Você extrai produtos de folhetos de fabricantes de equipamentos e insumos.
Responda APENAS com um array JSON, sem prosa e sem markdown. Cada item:
{"name": string, "description": string|null, "specs": string|null, "accessories": [string]|null, "standards": [string]|null, "sku": string|null, "brand": string|null, "model": string|null, "price": string|null, "currency": string|null, "unit": string|null, "page": number}
Regras:
- "description" é o resumo comercial curto do produto (1 a 3 frases).
- "specs" é a FICHA TÉCNICA COMPLETA, transcrita como está impressa: dimensões,
  peso, alimentação, faixas de operação, capacidades, materiais, itens de série.
  Não resuma, não reescreva, não corte. Se o folheto traz uma tabela, transcreva
  linha a linha. É melhor vir longo do que faltar.
- "accessories" são os itens opcionais/acessórios OFERECIDOS PARA aquele produto.
- "standards" são normas e certificações citadas (ex.: "NBR IEC 60601-2-19", "ANVISA").
- "price" é o texto EXATAMENTE como impresso (ex.: "R$ 1.234,56"). NUNCA normalize nem converta.
- "page" é a página (1-based) de onde o produto foi lido.
- Campo que não está no documento é null. É PROIBIDO inventar ou completar valores.
- Um produto por item; não repita o mesmo produto de páginas diferentes.
- ATENÇÃO: peça, acessório, componente ou item de série NÃO é produto. "Válvula
  reguladora", "tomadas auxiliares", "suporte de soro" pertencem ao equipamento
  que os acompanha — liste-os em "accessories" do produto pai, nunca como itens
  próprios do array.`

const productPDFUserPrompt = `Extraia todos os produtos deste folheto como JSON, seguindo as regras do sistema.`

// pdfExtractedItem is the model's answer, verbatim. Pointers on purpose: the
// contract is "absent is null", and a missing key must be distinguishable from
// an empty string when deciding what the human sees.
type pdfExtractedItem struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	// Specs is the brochure's technical block transcribed whole. The old
	// prompt asked only for a "description" and got 126 characters on
	// average — the ceiling was the question, not the document.
	Specs       *string  `json:"specs"`
	Accessories []string `json:"accessories"`
	Standards   []string `json:"standards"`
	SKU         *string  `json:"sku"`
	Brand       *string  `json:"brand"`
	Model       *string  `json:"model"`
	Price       *string  `json:"price"`
	Currency    *string  `json:"currency"`
	Unit        *string  `json:"unit"`
	Page        int      `json:"page"`
}

// Extract runs the whole gate ladder and returns the SAME preview the CSV
// import returns — the confirmation step is the existing POST /products/import.
func (uc *ImportProductsPDFUseCase) Extract(
	ctx context.Context, accountID string, data []byte, filename string,
) (*ProductImportPreview, error) {
	if uc.csv == nil || uc.ai == nil || uc.accounts == nil || uc.pageCount == nil {
		return nil, ErrProductPDFImportUnavailable
	}
	if uc.maxBytes > 0 && int64(len(data)) > uc.maxBytes {
		return nil, &domain.AppError{
			Code: "PRODUCT_PDF_TOO_LARGE",
			Message: fmt.Sprintf("o PDF passa do limite de %d MB. Divida o folheto ou comprima o arquivo",
				uc.maxBytes>>20),
		}
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, ErrProductPDFUnreadable
	}

	// Antivirus BEFORE anything touches the bytes. No scanner is a refusal,
	// not a pass-through: "sem antivírus configurado, recuse".
	if uc.scanner == nil {
		return nil, ErrProductPDFScanUnavailable
	}
	verdict, err := uc.scanner.Scan(ctx, data)
	if err != nil {
		// A scan that could not complete is not a clean scan. Fail closed.
		slog.WarnContext(ctx, "product pdf import: scan failed — refusing upload",
			"account_id", accountID, "error", err)
		return nil, ErrProductPDFScanUnavailable
	}
	if verdict.Status != domain.ScanClean {
		return nil, &domain.AppError{
			Code:    ErrProductPDFInfected.Code,
			Message: strings.TrimSpace(ErrProductPDFInfected.Message + " " + verdict.Signature),
		}
	}

	pages, err := uc.pageCount(data)
	if err != nil {
		return nil, ErrProductPDFUnreadable
	}
	if uc.maxPages > 0 && pages > uc.maxPages {
		return nil, &domain.AppError{
			Code: "PRODUCT_PDF_TOO_MANY_PAGES",
			Message: fmt.Sprintf("o PDF tem %d páginas; o limite é %d. Divida o folheto",
				pages, uc.maxPages),
		}
	}

	// Plan ceiling + AI budget, BEFORE the model call — same unit as a
	// contract read (one document through a model).
	if uc.spend != nil {
		if err := uc.spend.ensure(ctx, accountID, domain.SpendDocumentExtraction); err != nil {
			return nil, err
		}
	}

	// The brochure is KEPT, and — since this change — findable.
	//
	// It was already being uploaded on every import and orphaned in the same
	// breath: a fresh ULID for an object name, no row anywhere, so nothing
	// could ever say which file a product came from. A copy nobody can reach
	// is storage spend, not an archive. Now the upload is paired with a
	// product_sources row, and every row of the preview carries its id, so the
	// products created from it point back at the page they were read from.
	//
	// Still best-effort: the extraction is worth more than the archive, and a
	// bucket outage must not kill the import. Losing the link degrades the
	// product to what it was yesterday; failing the import would lose the
	// catalogue.
	sourceID := ""
	if uc.store != nil {
		path := fmt.Sprintf("accounts/%s/product-imports/%s.pdf", accountID, ulid.Make().String())
		if err := uc.store.UploadBytes(ctx, path, data, "application/pdf"); err != nil {
			slog.WarnContext(ctx, "product pdf import: audit copy failed — continuing",
				"account_id", accountID, "path", path, "error", err)
		} else if uc.sources != nil {
			// Hash the bytes, not the filename: the same brochure re-sent
			// under another name is the same source, and two rows claiming to
			// be the origin of the same page help nobody.
			sum := sha256.Sum256(data)
			id, err := uc.sources.Upsert(ctx, &domain.ProductSource{
				AccountID:   accountID,
				Filename:    filename,
				ObjectPath:  path,
				ContentHash: hex.EncodeToString(sum[:]),
				Pages:       pages,
				ByteSize:    int64(len(data)),
			})
			if err != nil {
				slog.WarnContext(ctx, "product pdf import: source record failed — continuing",
					"account_id", accountID, "error", err)
			} else {
				sourceID = id
				uc.storeBrochureImages(ctx, accountID, id, data)
			}
		}
	}

	provider, plan, err := uc.provider(ctx, accountID)
	if err != nil {
		return nil, err
	}
	reader, canRead := provider.(domain.DocumentReader)
	if !canRead {
		return nil, ErrProductPDFProviderCantRead
	}

	resp, err := reader.GenerateFromDocument(ctx, productPDFSystemPrompt, productPDFUserPrompt,
		domain.AIDocument{MIMEType: "application/pdf", Data: data, Filename: filename})
	// The decorators declare GenerateFromDocument unconditionally (they must —
	// see domain.ErrAIDocumentUnsupported), so "can't read" now arrives as an
	// error rather than a failed assertion. Same answer, same words.
	if errors.Is(err, domain.ErrAIDocumentUnsupported) {
		return nil, ErrProductPDFProviderCantRead
	}
	if err != nil || resp == nil {
		slog.WarnContext(ctx, "product pdf import: extraction call failed",
			"account_id", accountID, "error", err)
		return nil, classifyExtractionError(err)
	}
	if uc.meter != nil && plan != nil {
		uc.meter.Record(ctx, accountID, "product_pdf_extract", plan.AIProvider, plan.AIModel, resp.Usage)
	}

	rows, err := parseProductPDFExtraction(resp.Content, uc.csv.accountCurrency(ctx, accountID))
	if err != nil {
		return nil, err
	}
	// The id travels through the preview and comes back on confirmation, so
	// the products are linked to the brochure the person actually reviewed —
	// not to whatever was uploaded most recently.
	if sourceID != "" {
		for i := range rows {
			rows[i].SourceID = sourceID
		}
	}
	return uc.csv.previewFromRows(ctx, accountID, rows)
}

// storeBrochureImages pulls the pictures out of the brochure and files them
// against its source row, off the request.
//
// Every failure here is a WARN and nothing else. The catalogue is the point;
// pictures are a convenience on top of it, and an import that died because a
// bucket was slow would trade the thing that matters for the thing that does
// not. The one loud case is none at all — see the log line.
func (uc *ImportProductsPDFUseCase) storeBrochureImages(
	ctx context.Context, accountID, sourceID string, data []byte,
) {
	if uc.images == nil || uc.store == nil || uc.sources == nil || uc.dispatch == nil {
		return
	}
	// Detached from the request context, with its own ceiling: the person's
	// browser has the preview already and this must not be cancelled by them
	// navigating away, nor run forever.
	bg := context.WithoutCancel(ctx)
	uc.dispatch(func() {
		ctx, cancel := context.WithTimeout(bg, brochureImagesTimeout)
		defer cancel()

		pics, err := uc.images(data)
		if err != nil {
			slog.WarnContext(ctx, "product pdf import: image extraction failed",
				"account_id", accountID, "source_id", sourceID, "error", err)
			return
		}
		stored := 0
		for i, pic := range pics {
			// The object name carries the page and the hash: legible in a
			// bucket listing, and impossible to collide.
			path := fmt.Sprintf("accounts/%s/product-images/%s/p%03d-%s%s",
				accountID, sourceID, pic.Page, pic.SHA256[:12], imageExt(pic.ContentType))
			if err := uc.store.UploadBytes(ctx, path, pic.Data, pic.ContentType); err != nil {
				slog.WarnContext(ctx, "product pdf import: image upload failed",
					"account_id", accountID, "source_id", sourceID, "index", i, "error", err)
				continue
			}
			if err := uc.sources.AddImage(ctx, &domain.ProductSourceImage{
				AccountID:   accountID,
				SourceID:    sourceID,
				Page:        pic.Page,
				ObjectPath:  path,
				ContentType: pic.ContentType,
				Width:       pic.Width,
				Height:      pic.Height,
				ByteSize:    int64(len(pic.Data)),
				ContentHash: pic.SHA256,
			}); err != nil {
				// The object is in the bucket and the row is not: an orphan,
				// which costs storage and shows nowhere. Worth the WARN.
				slog.WarnContext(ctx, "product pdf import: image record failed — object orphaned",
					"account_id", accountID, "source_id", sourceID, "path", path, "error", err)
				continue
			}
			stored++
		}
		slog.InfoContext(ctx, "product pdf import: brochure images stored",
			"account_id", accountID, "source_id", sourceID, "found", len(pics), "stored", stored)
	})
}

// brochureImagesTimeout bounds the detached job. Generous: forty uploads to a
// bucket, on a machine that is also serving requests.
const brochureImagesTimeout = 3 * time.Minute

// imageExt is the extension for a content type we chose to store. The
// extractor refuses anything else, so the default is unreachable in practice
// and defensive here.
func imageExt(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	default:
		return ".jpg"
	}
}

// provider mirrors the contract extraction's resolution: account → plan →
// BYOK-aware factory. No model override — the brochure uses whatever the
// account's plan already pays for.
func (uc *ImportProductsPDFUseCase) provider(ctx context.Context, accountID string) (domain.AIProvider, *domain.Plan, error) {
	account, err := uc.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, nil, domain.ErrAccountNotFound
	}
	plan := domain.TrialPlan()
	if account.PlanID != nil && uc.plans != nil {
		if p, err := uc.plans.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	provider, err := uc.ai.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil || provider == nil {
		// Say WHOSE problem it is. A contract account with no BYOK key gets
		// domain.ErrBYOKKeyMissing here, and flattening it into "não está
		// configurada neste ambiente" blamed the deployment for something the
		// customer fixes in one screen — the onboarding's product step is the
		// first place that lands, before anyone was ever asked for a key.
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return nil, nil, appErr
		}
		return nil, nil, ErrProductPDFImportUnavailable
	}
	return provider, plan, nil
}

// parseProductPDFExtraction turns the model's answer into the same rows the
// CSV parser produces. Everything fails CLOSED with a legible error: a
// malformed answer, an empty array, or prose where JSON was demanded is a
// refused import, never a panic and never a half-guessed catalogue.
func parseProductPDFExtraction(content, fallbackCurrency string) ([]domain.ProductImportRow, error) {
	raw := strings.TrimSpace(content)
	// Models wrap JSON in fences no matter how firmly asked not to.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	// Tolerate prose around the array by slicing to the outermost brackets —
	// but never inventing one: no '[' means no data.
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil, ErrProductPDFExtractFailed
	}

	var items []pdfExtractedItem
	if err := json.Unmarshal([]byte(raw[start:end+1]), &items); err != nil {
		return nil, ErrProductPDFExtractFailed
	}
	if len(items) == 0 {
		return nil, ErrProductPDFNoProducts
	}

	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return strings.TrimSpace(*s)
	}

	// trimAll drops blanks from a list the model may pad with empty strings —
	// an accessory called "" is noise the review screen would have to render.
	trimAll := func(in []string) []string {
		if len(in) == 0 {
			return nil
		}
		out := make([]string, 0, len(in))
		for _, s := range in {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}

	seen := map[string]bool{}
	rows := make([]domain.ProductImportRow, 0, len(items))
	for i, item := range items {
		row := domain.ProductImportRow{
			// Line numbers the rows for the same review screen the CSV uses;
			// Page is the human's pointer back into the document.
			Line:        i + 1,
			Name:        deref(item.Name),
			Description: deref(item.Description),
			SKU:         deref(item.SKU),
			Brand:       deref(item.Brand),
			Model:       deref(item.Model),
			Unit:        deref(item.Unit),
			Specs:       deref(item.Specs),
			Accessories: trimAll(item.Accessories),
			Standards:   trimAll(item.Standards),
			Page:        item.Page,
			Currency:    fallbackCurrency,
		}
		if c := strings.ToUpper(deref(item.Currency)); c != "" {
			row.Currency = c
		}

		switch {
		case row.Name == "":
			row.Problem = domain.ProblemNoName
		case seen[strings.ToLower(row.Name)]:
			row.Problem = domain.ProblemDuplicateInFile
		default:
			seen[strings.ToLower(row.Name)] = true
		}

		// The price goes through the SAME parser as the CSV column, on the
		// verbatim string the model was told to preserve. Ambiguous is a
		// marked row for the human, exactly like the spreadsheet path.
		if price := deref(item.Price); row.Problem == "" && price != "" {
			cents, ok, ambiguous := domain.ParseMoney(price)
			switch {
			case ambiguous:
				row.Problem = domain.ProblemAmbiguousPrice
			case !ok:
				row.Problem = domain.ProblemBadPrice
			default:
				row.PriceCents = cents
			}
		}

		rows = append(rows, row)
	}
	return rows, nil
}
