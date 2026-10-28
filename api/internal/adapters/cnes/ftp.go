package cnes

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"

	"github.com/replider/api/internal/core/domain"
)

const (
	datasusFTPAddr = "ftp.datasus.gov.br:21"
	cnesDataDir    = "dissemin/publicos/CNES/200508_/Dados"
	eqDir          = cnesDataDir + "/EQ"
	// ltDir holds the bed census — the same tree, the same DBC format, one
	// file per UF per month, and never read until the audit that found it.
	ltDir = cnesDataDir + "/LT"
	// hbDir is capability the Ministry funds; srDir is what the establishment
	// says it performs. Same tree, same format, never read.
	hbDir      = cnesDataDir + "/HB"
	srDir      = cnesDataDir + "/SR"
	ftpTimeout = 60 * time.Second
	// maxDBCSize bounds the read: the largest state's EQ file is ~2 MB, so
	// 64 MB means "the server sent something that is not an EQ file".
	maxDBCSize = 64 << 20
)

// ErrCompetenciaNotPublished aliases the domain sentinel so same-package
// callers read naturally; the worker matches on the domain value.
var ErrCompetenciaNotPublished = domain.ErrCNESCompetenciaNotPublished

// FTPFetcher downloads EQ files from the DATASUS FTP. Zero value is ready;
// addr is overridable for tests.
type FTPFetcher struct {
	Addr string
}

// FetchEquipment downloads EQ{UF}{AAMM}.dbc for uf ("MT") and competencia
// ("202607" — converted to the file's AAMM form).
func (f *FTPFetcher) FetchEquipment(uf, competencia string) ([]byte, error) {
	return f.fetchFamily("EQ", eqDir, uf, competencia)
}

// FetchBeds downloads LT{UF}{AAMM}.dbc — the bed census.
func (f *FTPFetcher) FetchBeds(uf, competencia string) ([]byte, error) {
	return f.fetchFamily("LT", ltDir, uf, competencia)
}

// FetchHabilitations downloads HB{UF}{AAMM}.dbc.
func (f *FTPFetcher) FetchHabilitations(uf, competencia string) ([]byte, error) {
	return f.fetchFamily("HB", hbDir, uf, competencia)
}

// FetchServices downloads SR{UF}{AAMM}.dbc.
func (f *FTPFetcher) FetchServices(uf, competencia string) ([]byte, error) {
	return f.fetchFamily("SR", srDir, uf, competencia)
}

// fetchFamily is one file of one DATASUS dataset family. Extracted when the
// bed census joined the equipment load: two copies of an anonymous-FTP
// session with a 550-means-not-published rule is one copy too many.
func (f *FTPFetcher) fetchFamily(family, dir, uf, competencia string) ([]byte, error) {
	if len(competencia) != 6 {
		return nil, fmt.Errorf("competencia must be AAAAMM, got %q", competencia)
	}
	name := fmt.Sprintf("%s%s%s.dbc", family, strings.ToUpper(uf), competencia[2:])

	addr := f.Addr
	if addr == "" {
		addr = datasusFTPAddr
	}
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(ftpTimeout))
	if err != nil {
		return nil, fmt.Errorf("datasus ftp dial: %w", err)
	}
	defer conn.Quit() //nolint:errcheck // best-effort close on a read-only session

	if err := conn.Login("anonymous", "anonymous"); err != nil {
		return nil, fmt.Errorf("datasus ftp login: %w", err)
	}
	resp, err := conn.Retr(dir + "/" + name)
	if err != nil {
		// 550 is the FTP "no such file" family — the month isn't out yet.
		if strings.Contains(err.Error(), "550") {
			return nil, ErrCompetenciaNotPublished
		}
		return nil, fmt.Errorf("datasus ftp retr %s: %w", name, err)
	}
	defer resp.Close() //nolint:errcheck // read-only FTP data connection

	data, err := io.ReadAll(io.LimitReader(resp, maxDBCSize+1))
	if err != nil {
		return nil, fmt.Errorf("datasus ftp read %s: %w", name, err)
	}
	if len(data) > maxDBCSize {
		return nil, fmt.Errorf("datasus ftp %s exceeds size cap", name)
	}
	return data, nil
}

// FetchEquipmentRows downloads AND parses one UF+competencia — the one-call
// port the worker consumes, so core never touches raw DBC bytes.
func (f *FTPFetcher) FetchEquipmentRows(uf, competencia string) ([]domain.CNESEquipmentRow, error) {
	raw, err := f.FetchEquipment(uf, competencia)
	if err != nil {
		return nil, err
	}
	return ParseEquipmentDBC(raw, uf)
}

// FetchBedRows downloads AND parses one UF+competencia of the bed census.
func (f *FTPFetcher) FetchBedRows(uf, competencia string) ([]domain.CNESBedRow, error) {
	raw, err := f.FetchBeds(uf, competencia)
	if err != nil {
		return nil, err
	}
	return ParseBedsDBC(raw, uf)
}

// FetchHabilitationRows downloads AND parses one UF+competencia of the
// habilitation register.
func (f *FTPFetcher) FetchHabilitationRows(uf, competencia string) ([]domain.CNESHabilitationRow, error) {
	raw, err := f.FetchHabilitations(uf, competencia)
	if err != nil {
		return nil, err
	}
	return ParseHabilitationsDBC(raw, uf)
}

// FetchServiceRows downloads AND parses one UF+competencia of the specialized
// service register.
func (f *FTPFetcher) FetchServiceRows(uf, competencia string) ([]domain.CNESServiceRow, error) {
	raw, err := f.FetchServices(uf, competencia)
	if err != nil {
		return nil, err
	}
	return ParseServicesDBC(raw, uf)
}
