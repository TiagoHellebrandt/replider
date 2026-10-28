package cnes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// OpenDataClient reads the ministry's open-data API for establishment
// records (name, phone, address, coordinates). No auth, JSON.
// The API has NO equipment endpoint — that is what the FTP is for
// (probed 2026-08-20, docs/spike-cnes.md).
type OpenDataClient struct {
	// BaseURL is overridable for httptest.
	BaseURL string
	HTTP    *http.Client
}

const openDataBaseURL = "https://apidadosabertos.saude.gov.br"

func (c *OpenDataClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// openDataEstablishment mirrors the fields we keep from
// GET /cnes/estabelecimentos/{cnes}.
type openDataEstablishment struct {
	CodigoCNES      int     `json:"codigo_cnes"`
	NomeFantasia    *string `json:"nome_fantasia"`
	NomeRazaoSocial *string `json:"nome_razao_social"`
	NumeroCNPJ      *string `json:"numero_cnpj"`
	// NumeroCNPJEntidade is the maintaining entity's CNPJ. It arrived in
	// every payload and was dropped on the floor: measured over the 20
	// biggest parks in MT, 11 of them publish only this one, so for more than
	// half the page the radar had no buyer key at all.
	NumeroCNPJEntidade *string  `json:"numero_cnpj_entidade"`
	Endereco           *string  `json:"endereco_estabelecimento"`
	Numero             *string  `json:"numero_estabelecimento"`
	Bairro             *string  `json:"bairro_estabelecimento"`
	CodigoMunicipio    *int     `json:"codigo_municipio"`
	Telefone           *string  `json:"numero_telefone_estabelecimento"`
	Email              *string  `json:"endereco_email_estabelecimento"`
	Latitude           *float64 `json:"latitude_estabelecimento_decimo_grau"`
	Longitude          *float64 `json:"longitude_estabelecimento_decimo_grau"`
	CodigoTipoUnidade  *int     `json:"codigo_tipo_unidade"`
	CentroCirurgico    *int     `json:"estabelecimento_possui_centro_cirurgico"`
	AtendHospitalar    *int     `json:"estabelecimento_possui_atendimento_hospitalar"`
	// Arriving all along, dropped all along. The neonatal and obstetric flags
	// are the qualifiers a neonatal catalogue is sold against; the sphere
	// decides whether this buyer buys by tender at all; the deactivation
	// reason is the difference between a hospital and a building that used to
	// be one.
	CentroNeonatal   *int    `json:"estabelecimento_possui_centro_neonatal"`
	CentroObstetrico *int    `json:"estabelecimento_possui_centro_obstetrico"`
	EsferaAdmin      *string `json:"descricao_esfera_administrativa"`
	MotivoDesab      *string `json:"codigo_motivo_desabilitacao_estabelecimento"`
	CEP              *string `json:"codigo_cep_estabelecimento"`
}

// FetchEstablishment resolves one CNES id. A 404 returns (nil, nil): ids do
// retire, and a vanished record must not fail a whole refresh batch.
func (c *OpenDataClient) FetchEstablishment(ctx context.Context, cnesID string) (*domain.CNESEstablishment, error) {
	base := c.BaseURL
	if base == "" {
		base = openDataBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/cnes/estabelecimentos/%s", base, cnesID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("open-data fetch %s: %w", cnesID, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response body
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-data fetch %s: status %d", cnesID, resp.StatusCode)
	}

	var raw openDataEstablishment
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("open-data decode %s: %w", cnesID, err)
	}

	e := &domain.CNESEstablishment{
		CNES:       cnesID,
		TradeName:  strOr(raw.NomeFantasia),
		LegalName:  strOr(raw.NomeRazaoSocial),
		CNPJ:       raw.NumeroCNPJ,
		EntityCNPJ: raw.NumeroCNPJEntidade,
		District:   strOr(raw.Bairro),
		Phone:      raw.Telefone,
		Email:      raw.Email,
		Latitude:   raw.Latitude,
		Longitude:  raw.Longitude,
	}
	if raw.Endereco != nil {
		addr := *raw.Endereco
		if raw.Numero != nil && *raw.Numero != "" {
			addr += ", " + *raw.Numero
		}
		e.Address = addr
	}
	if raw.CodigoMunicipio != nil {
		city := fmt.Sprintf("%06d", *raw.CodigoMunicipio)
		e.CityCode = &city
	}
	if raw.CodigoTipoUnidade != nil {
		ut := fmt.Sprintf("%d", *raw.CodigoTipoUnidade)
		e.UnitTypeCode = &ut
	}
	if raw.CentroCirurgico != nil {
		v := *raw.CentroCirurgico == 1
		e.HasSurgicalCenter = &v
	}
	if raw.AtendHospitalar != nil {
		v := *raw.AtendHospitalar == 1
		e.HasHospitalCare = &v
	}
	if raw.CentroNeonatal != nil {
		v := *raw.CentroNeonatal == 1
		e.HasNeonatalCentre = &v
	}
	if raw.CentroObstetrico != nil {
		v := *raw.CentroObstetrico == 1
		e.HasObstetricCentre = &v
	}
	e.AdminSphere = strings.TrimSpace(strOr(raw.EsferaAdmin))
	e.DisabledReason = strings.TrimSpace(strOr(raw.MotivoDesab))
	e.CEP = strings.TrimSpace(strOr(raw.CEP))
	return e, nil
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
