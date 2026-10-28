-- CNES installed park (tese-agente-autonomo / spike-cnes).
-- Public DATASUS data, deliberately NOT account-scoped: every account reads
-- the same national tables, filtered by its own territory at query time.

-- Equipment type dictionary, seeded from TAB_CNES's Equipam_grp.cnv
-- (code = TIPEQUIP zero-padded + CODEQUIP zero-padded). Re-run of this
-- migration's INSERT is safe; new ministry codes arrive via a future
-- migration regenerated from the CNV.
CREATE TABLE IF NOT EXISTS cnes_equipment_types (
    code       CHAR(4) PRIMARY KEY,
    name       TEXT NOT NULL,
    group_name TEXT NOT NULL
);

-- One row per establishment x equipment type x monthly competencia
-- (AAAAMM). Two competencias are kept so "hospital ganhou um tomografo"
-- can become a signal later; the worker prunes older ones.
CREATE TABLE IF NOT EXISTS cnes_equipment (
    cnes        CHAR(7)  NOT NULL,
    equip_code  CHAR(4)  NOT NULL,
    competencia CHAR(6)  NOT NULL,
    city_code   CHAR(6)  NOT NULL,
    uf          CHAR(2)  NOT NULL,
    qt_exist    INTEGER  NOT NULL DEFAULT 0,
    qt_uso      INTEGER  NOT NULL DEFAULT 0,
    sus         BOOLEAN  NOT NULL DEFAULT FALSE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cnes, equip_code, competencia)
);
CREATE INDEX IF NOT EXISTS idx_cnes_equipment_uf_comp
    ON cnes_equipment (uf, competencia, equip_code);
CREATE INDEX IF NOT EXISTS idx_cnes_equipment_city
    ON cnes_equipment (city_code, competencia);

-- Lazy cache of the open-data establishment record (name, phone, address).
-- fetched_at drives staleness; a row is refreshed after ~90 days.
CREATE TABLE IF NOT EXISTS cnes_establishments (
    cnes                CHAR(7) PRIMARY KEY,
    trade_name          TEXT NOT NULL DEFAULT '',
    legal_name          TEXT NOT NULL DEFAULT '',
    cnpj                TEXT,
    address             TEXT NOT NULL DEFAULT '',
    district            TEXT NOT NULL DEFAULT '',
    city_code           CHAR(6),
    phone               TEXT,
    email               TEXT,
    latitude            DOUBLE PRECISION,
    longitude           DOUBLE PRECISION,
    unit_type_code      TEXT,
    has_surgical_center BOOLEAN,
    has_hospital_care   BOOLEAN,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Idempotency ledger: one row per (uf, competencia) successfully loaded.
CREATE TABLE IF NOT EXISTS cnes_ingest_runs (
    uf          CHAR(2) NOT NULL,
    competencia CHAR(6) NOT NULL,
    rows_loaded INTEGER NOT NULL DEFAULT 0,
    finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uf, competencia)
);

INSERT INTO cnes_equipment_types (code, name, group_name) VALUES
('0101', 'Gama Camara', 'Equipamentos De Diagnóstico Por Imagem'),
('0102', 'Mamografo Com Comando Simples', 'Equipamentos De Diagnóstico Por Imagem'),
('0103', 'Mamografo Com Estereotaxia', 'Equipamentos De Diagnóstico Por Imagem'),
('0104', 'Raio X Ate 100 Ma', 'Equipamentos De Diagnóstico Por Imagem'),
('0105', 'Raio X De 100 A 500 Ma', 'Equipamentos De Diagnóstico Por Imagem'),
('0106', 'Raio X Mais De 500Ma', 'Equipamentos De Diagnóstico Por Imagem'),
('0107', 'Raio X Dentario', 'Equipamentos De Diagnóstico Por Imagem'),
('0108', 'Raio X Com Fluoroscopia', 'Equipamentos De Diagnóstico Por Imagem'),
('0109', 'Raio X Para Densitometria Ossea', 'Equipamentos De Diagnóstico Por Imagem'),
('0110', 'Raio X Para Hemodinamica', 'Equipamentos De Diagnóstico Por Imagem'),
('0111', 'Tomógrafo Computadorizado', 'Equipamentos De Diagnóstico Por Imagem'),
('0112', 'Ressonancia Magnetica', 'Equipamentos De Diagnóstico Por Imagem'),
('0113', 'Ultrassom Doppler Colorido', 'Equipamentos De Diagnóstico Por Imagem'),
('0114', 'Ultrassom Ecografo', 'Equipamentos De Diagnóstico Por Imagem'),
('0115', 'Ultrassom Convencional', 'Equipamentos De Diagnóstico Por Imagem'),
('0116', 'Processadora De Filme Exclusiva Para Mamografia', 'Equipamentos De Diagnóstico Por Imagem'),
('0117', 'Mamografo Computadorizado', 'Equipamentos De Diagnóstico Por Imagem'),
('0118', 'Pet/Ct', 'Equipamentos De Diagnóstico Por Imagem'),
('0221', 'Controle Ambiental/Ar-Condicionado Central', 'Equipamentos De Infra-Estrutura'),
('0222', 'Grupo Gerador', 'Equipamentos De Infra-Estrutura'),
('0223', 'Usina De Oxigenio', 'Equipamentos De Infra-Estrutura'),
('0331', 'Endoscopio Das Vias Respiratorias', 'Equipamentos Por Métodos Ópticos'),
('0332', 'Endoscopio Das Vias Urinarias', 'Equipamentos Por Métodos Ópticos'),
('0333', 'Endoscopio Digestivo', 'Equipamentos Por Métodos Ópticos'),
('0334', 'Equipamentos Para Optometria', 'Equipamentos Por Métodos Ópticos'),
('0335', 'Laparoscopio/Vídeo', 'Equipamentos Por Métodos Ópticos'),
('0336', 'Microscopio Cirurgico', 'Equipamentos Por Métodos Ópticos'),
('0337', 'Cadeira Oftalmologica', 'Equipamentos Por Métodos Ópticos'),
('0338', 'Coluna Oftalmologica', 'Equipamentos Por Métodos Ópticos'),
('0339', 'Refrator', 'Equipamentos Por Métodos Ópticos'),
('0340', 'Lensometro', 'Equipamentos Por Métodos Ópticos'),
('0344', 'Projetor Ou Tabela De Optotipos', 'Equipamentos Por Métodos Ópticos'),
('0345', 'Retinoscopio', 'Equipamentos Por Métodos Ópticos'),
('0346', 'Oftalmoscopio', 'Equipamentos Por Métodos Ópticos'),
('0347', 'Ceratometro', 'Equipamentos Por Métodos Ópticos'),
('0348', 'Tonometro De Aplanacao', 'Equipamentos Por Métodos Ópticos'),
('0349', 'Biomicroscopio (Lampada De Fenda)', 'Equipamentos Por Métodos Ópticos'),
('0350', 'Campimetro', 'Equipamentos Por Métodos Ópticos'),
('0441', 'Eletrocardiografo', 'Equipamentos Por Métodos Gráficos'),
('0442', 'Eletroencefalografo', 'Equipamentos Por Métodos Gráficos'),
('0551', 'Bomba/Balao Intra-Aortico', 'Equipamentos De Manutenção Da Vida'),
('0552', 'Bomba De Infusao', 'Equipamentos De Manutenção Da Vida'),
('0553', 'Berço Aquecido', 'Equipamentos De Manutenção Da Vida'),
('0554', 'Bilirrubinometro', 'Equipamentos De Manutenção Da Vida'),
('0555', 'Debitometro', 'Equipamentos De Manutenção Da Vida'),
('0556', 'Desfibrilador', 'Equipamentos De Manutenção Da Vida'),
('0557', 'Equipamento De Fototerapia', 'Equipamentos De Manutenção Da Vida'),
('0558', 'Incubadora', 'Equipamentos De Manutenção Da Vida'),
('0559', 'Marcapasso Temporario', 'Equipamentos De Manutenção Da Vida'),
('0560', 'Monitor De Ecg', 'Equipamentos De Manutenção Da Vida'),
('0561', 'Monitor De Pressao Invasivo', 'Equipamentos De Manutenção Da Vida'),
('0562', 'Monitor De Pressao Nao-Invasivo', 'Equipamentos De Manutenção Da Vida'),
('0563', 'Reanimador Pulmonar/Ambu', 'Equipamentos De Manutenção Da Vida'),
('0564', 'Respirador/Ventilador', 'Equipamentos De Manutenção Da Vida'),
('0671', 'Aparelho De Diatermia Por Ultrassom/Ondas Curtas', 'Outros Equipamentos'),
('0672', 'Aparelho De Eletroestimulacao', 'Outros Equipamentos'),
('0673', 'Bomba De Infusao De Hemoderivados', 'Outros Equipamentos'),
('0674', 'Equipamentos De Aferese', 'Outros Equipamentos'),
('0675', 'Equipamento Para Audiometria', 'Outros Equipamentos'),
('0676', 'Equipamento De Circulacao Extracorporea', 'Outros Equipamentos'),
('0677', 'Equipamento Para Hemodialise', 'Outros Equipamentos'),
('0678', 'Forno De Bier', 'Outros Equipamentos'),
('0780', 'Equipo Odontologico', 'Equipamentos De Odontologia'),
('0781', 'Compressor Odontologico', 'Equipamentos De Odontologia'),
('0782', 'Fotopolimerizador', 'Equipamentos De Odontologia'),
('0783', 'Caneta De Alta Rotacao', 'Equipamentos De Odontologia'),
('0784', 'Caneta De Baixa Rotacao', 'Equipamentos De Odontologia'),
('0785', 'Amalgamador', 'Equipamentos De Odontologia'),
('0786', 'Aparelho De Profilaxia C/ Jato De Bicarbonato', 'Equipamentos De Odontologia'),
('0887', 'Emissoes Otoacusticas Evocadas Transientes', 'Equipamentos De Audiologia'),
('0888', 'Emissoes Otoacusticas Evocadas Por Produto De Distorcao', 'Equipamentos De Audiologia'),
('0889', 'Potencial Evocado Auditivo De Tronco Encefalico Automatico', 'Equipamentos De Audiologia'),
('0890', 'Pot Evocado Aud Tronco Encef De Curta,Media E Longa Latencia', 'Equipamentos De Audiologia'),
('0891', 'Audiometro De Um Canal', 'Equipamentos De Audiologia'),
('0892', 'Audiometro De Dois Canais', 'Equipamentos De Audiologia'),
('0893', 'Imitanciometro', 'Equipamentos De Audiologia'),
('0894', 'Imitanciometro Multifrequencial', 'Equipamentos De Audiologia'),
('0895', 'Cabine Acustica', 'Equipamentos De Audiologia'),
('0896', 'Sistema De Campo Livre', 'Equipamentos De Audiologia'),
('0897', 'Sistema Completo De Reforço Visual(Vra)', 'Equipamentos De Audiologia'),
('0898', 'Ganho De Insercao', 'Equipamentos De Audiologia'),
('0899', 'Hi-Pro', 'Equipamentos De Audiologia'),
('0901', 'Camera Para Reconhecimento Facial', 'Equipamentos De Telessaude'),
('0902', 'Carrinho De Telemedicina De Videoconferencia', 'Equipamentos De Telessaude'),
('0903', 'Condensador', 'Equipamentos De Telessaude'),
('0904', 'Dermatoscopio', 'Equipamentos De Telessaude'),
('0905', 'Detector Fetal Portatil', 'Equipamentos De Telessaude'),
('0906', 'Kit Dermatoscopia', 'Equipamentos De Telessaude'),
('0907', 'Kit Medico De Diagnostico Audiologico Tab', 'Equipamentos De Telessaude'),
('0908', 'Mesa Digitalizadora', 'Equipamentos De Telessaude'),
('0909', 'Monitor Sinais Vitais Multif Port Telessaude De Grau Medico', 'Equipamentos De Telessaude'),
('0910', 'Retinografo Portatil', 'Equipamentos De Telessaude'),
('0911', 'Ultrassom Portatil', 'Equipamentos De Telessaude'),
('1001', 'Aparelho De Hemodialise - Ambulatorial', 'Dialise'),
('1002', 'Aparelho De Hemodialise - Hospitalar', 'Dialise'),
('1003', 'Aparelho De Hemodialise Reserva', 'Dialise'),
('1004', 'Aparelho Para Dialise Peritoneal', 'Dialise')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, group_name = EXCLUDED.group_name;
