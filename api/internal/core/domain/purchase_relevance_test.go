package domain

import "testing"

// Every string here is verbatim from a contract PNCP published, collected for
// one account on 2026-09-14. The rule is judged on real notices or it is
// judged on nothing.
func TestIsGoodsPurchaseInOurTrade(t *testing.T) {
	tests := []struct {
		name, object, buyer string
		want                bool
	}{
		// --- keep: the twelve that would make a salesperson's day ---
		{
			"a ventilator acquisition for a university hospital",
			"AQUISIÇÃO DE  10 (DEZ) VENTILADORES DE TRANSPORTE PARA O HOSPITAL UNIVERSITÁRIO",
			"EMPRESA BRASILEIRA DE SERVIÇOS HOSPITALARES", true,
		},
		{
			"literally their product, named in the object",
			"AQUISIÇÃO DE EQUIPAMENTOS MÉDICOS HOSPITALARES (BERÇO AQUECIDO)",
			"EMPRESA BRASILEIRA DE SERVIÇOS HOSPITALARES", true,
		},
		{
			"consumables for health",
			"O OBJETO DO PRESENTE INSTRUMENTO É AQUISIÇÃO DE PRODUTOS PARA SAÚDE, DO TIPO: BOLSAS E FRASCOS",
			"EMPRESA BRASILEIRA DE SERVIÇOS HOSPITALARES", true,
		},
		{
			"medicinal gases, bought as goods",
			"O OBJETO DO PRESENTE INSTRUMENTO É AQUISIÇÃO, SOB DEMANDA, DE GASES MEDICINAIS EM CILINDROS",
			"EMPRESA BRASILEIRA DE SERVIÇOS HOSPITALARES", true,
		},

		// --- cut: labour, not equipment ---
		{
			"medical services are not a supplier's contract",
			"PRESTAÇÃO DE SERVIÇOS MÉDICOS ESPECIALIZADOS EM ANGIOLOGIA/CIRURGIA VASCULAR, CARDIOLOGIA",
			"MUNICIPIO DE ITIQUIRA", false,
		},
		{
			"maintenance is work even when it says aquisição",
			"AQUISIÇÃO DE PEÇAS DE REPOSIÇÃO ORIGINAIS PARA MANUTENÇÃO DO LABORATÓRIO MUNICIPAL",
			"MUNICIPIO DE ITIQUIRA", false,
		},
		{
			"accreditation of clinics is not a purchase of things",
			"CREDENCIAMENTO DE EMPRESA ESPECIALIZADA NA REALIZAÇÃO DE EXAMES LABORATORIAIS",
			"MUNICIPIO DE ITIQUIRA", false,
		},

		// --- cut: a hospital buying somebody else's trade ---
		{
			"computer monitors at a hospital are the embarrassing false positive",
			"O OBJETO DO PRESENTE INSTRUMENTO É O FORNECIMENTO DE MICROCOMPUTADORES, NOTEBOOKS E MONITORES",
			"EMPRESA BRASILEIRA DE SERVIÇOS HOSPITALARES", false,
		},
		{
			"ceiling fans in a bundle of assorted equipment",
			"EMPENHO PARA AQUISIÇÃO DE MAQUINAS, UTENSILIOS E EQUIPAMENTOS DIVERSOS, VENTILADOR",
			"IFMT", false,
		},
		{
			"a heated pass-through for a student restaurant",
			"AQUISIÇÃO DE EQUIPAMENTO (PASS THROUGH AQUECIDO) PARA O RESTAURANTE ESTUDANTIL",
			"IFMT", false,
		},

		// --- cut: not a purchase at all ---
		{
			"a tax-monitoring platform matched 'monitor' and is a service",
			"CONTRATAÇÃO DE EMPRESA ESPECIALIZADA PARA FORNECIMENTO DE SOLUÇÃO INTEGRADA DE MONITORAMENTO FISCAL",
			"MUNICIPIO DE PORTO ESPERIDIAO", false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsGoodsPurchaseInOurTrade(tc.object, tc.buyer); got != tc.want {
				t.Fatalf("IsGoodsPurchaseInOurTrade = %v, want %v\n  %s", got, tc.want, tc.object)
			}
		})
	}
}
