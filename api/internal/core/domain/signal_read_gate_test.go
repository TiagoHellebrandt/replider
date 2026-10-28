package domain

import "testing"

// The four rows the seller actually saw on 15/09/2026, verbatim from the
// production database. Every one of them was collected by a derived watch
// while a pre-gate image was live.
const (
	prodDrone = "[LICITANET] - Registro de Preços, com fundamento na Lei Federal nº 14.133/2021 e demais normas aplicáveis, " +
		"objetivando a aquisição de solução integrada de aerolevantamento, composta por aeronave remotamente pilotada " +
		"(RPA/DRONE), sensor LiDAR e software de processamento fotogramétrico e LiDAR, incluindo acessórios, treinamento " +
		"técnico presencial, garantia e suporte técnico, destinados a atender às demandas técnicas da Secretaria " +
		"Municipal de Governo relacionadas às áreas de topografia, engenharia, planejamento urbano e monitoramento ambiental."
	prodRope = "FUTURA E EVENTUAL CONTRATAÇÃO DE EMPRESA ESPECIALIZADA PARA O FORNECIMENTO DE REDES DE PROTEÇÃO DE CORDA " +
		"TRANÇADA, INCLUINDO REDES PRONTAS, CONFECÇÃO MANUAL SOB MEDIDA E INSTALAÇÃO, COM O FORNECIMENTO DE TODOS OS " +
		"MATERIAIS, ACESSÓRIOS, EQUIPAMENTOS E MÃO DE OBRA NECESSÁRIOS À PERFEITA EXECUÇÃO DOS SERVIÇOS."
	prodTV      = "“Aquisição de aparelhos de televisão e ventiladores de teto para as secretarias do município."
	prodBudget  = "VALOR QUE SE EMPENHA PARA ATENDER DESPESAS COM A PRESTACAO DE SERVICO DE MANUTENCAO."
	prodGenuine = "Contratação de empresa especializada para a locação de equipamentos médico-hospitalares (berço aquecido " +
		"neonatal, incubadora neonatal de transporte, cpap nasal de bolhas neonato e ventilador pulmonar), destinados ao " +
		"atendimento de pacientes neonatais."
)

// The words that let each one in: the watch keywords come from the account's
// own product names, and half of them are ordinary Portuguese.
//
//	manual      → "CONFECÇÃO MANUAL SOB MEDIDA"   (safety netting)
//	sensor      → "sensor LiDAR"                  (drone survey)
//	ventilador  → "televisão e ventiladores"      (ceiling fans)
func TestSignalPassesDerivedGate_DropsWhatTheSellerSaw(t *testing.T) {
	junk := map[string]string{
		"drone survey":   prodDrone,
		"safety netting": prodRope,
		"ceiling fans":   prodTV,
		"budget entry":   prodBudget,
	}
	for name, title := range junk {
		s := &Signal{Title: title, Organization: "MUNICIPIO DE SAPEZAL", Source: SignalPNCP}
		if SignalPassesDerivedGate(s, true) {
			t.Errorf("%s still reaches the feed", name)
		}
	}
}

func TestSignalPassesDerivedGate_KeepsTheRealOne(t *testing.T) {
	s := &Signal{Title: prodGenuine, Organization: "INSTITUTO PATRIS", Source: SignalPNCP}
	if !SignalPassesDerivedGate(s, true) {
		t.Fatal("a neonatal equipment tender was filtered out")
	}
}

// Somebody typed those words on purpose. The product does not second-guess a
// hand-written watch — at ingest or on read.
func TestSignalPassesDerivedGate_AHandWrittenWatchIsExempt(t *testing.T) {
	s := &Signal{Title: prodRope, Organization: "MUNICIPIO DE SAPEZAL", Source: SignalPNCP}
	if !SignalPassesDerivedGate(s, false) {
		t.Fatal("a hand-written watch had its own words overruled")
	}
}

// A vendor feed the account pays for claimed the signal by pushing it. Judging
// those by a health heuristic would empty every integration the product
// supports — the same carve-out the orphan filter makes in SQL.
func TestSignalPassesDerivedGate_PushedSourcesAreExempt(t *testing.T) {
	s := &Signal{Title: prodRope, Organization: "Qualquer", Source: SignalNewCompany}
	if !SignalPassesDerivedGate(s, true) {
		t.Fatal("a pushed signal was judged by the health gate")
	}
}

func TestSignalPassesDerivedGate_NilIsNotAFeedRow(t *testing.T) {
	if SignalPassesDerivedGate(nil, true) {
		t.Fatal("nil passed")
	}
}
