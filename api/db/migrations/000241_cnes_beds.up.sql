-- What a hospital is BUILT to treat, beside what it owns.
--
-- The installed park says which machines are there; the bed census says how
-- much care the place is set up to deliver, and that is the number that
-- predicts demand. A hospital with twenty neonatal intensive-care beds needs
-- twenty incubators, twenty monitors and the ventilators to match, on a
-- replacement cycle — the park alone never says that.
--
-- Measured on 2026-09-18 over the LT file for Mato Grosso: 308 neonatal beds
-- across 29 hospitals. The account this was built for sells incubators, warm
-- cribs, neonatal resuscitators and infant ventilators, so those 29
-- establishments ARE its addressable market, named and counted — and the
-- product could not see a single one of them.
--
-- The file is LT{UF}{AAMM}.dbc, the same FTP tree and the same DBC format the
-- equipment load already reads.

-- The ministry's own bed dictionary, seeded from TAB_CNES/CNV/ESP_LEIT.CNV.
--
-- A TABLE, not ranges in Go, and the reason is that this audit got the ranges
-- wrong TWICE before reading the file: `TP_LEITO = 4` was read as intensive
-- care when it means obstetric, and code 86 was counted as neonatal when it
-- is coronary intensive care. A dictionary that ships with the data cannot
-- drift from it; a constant in a source file drifts the first time the
-- ministry adds a code.
CREATE TABLE IF NOT EXISTS cnes_bed_types (
    code       CHAR(2) PRIMARY KEY,
    name       TEXT NOT NULL,
    -- Derived from the ministry's own names, stored so a query can ask the
    -- question directly instead of re-deriving it from a LIKE.
    is_icu     BOOLEAN NOT NULL DEFAULT FALSE,
    is_neonatal BOOLEAN NOT NULL DEFAULT FALSE
);

INSERT INTO cnes_bed_types (code, name, is_icu, is_neonatal) VALUES
    ('01', 'BUCO MAXILO FACIAL', FALSE, FALSE),
    ('02', 'CARDIOLOGIA', FALSE, FALSE),
    ('03', 'CIRURGIA GERAL', FALSE, FALSE),
    ('04', 'ENDOCRINOLOGIA', FALSE, FALSE),
    ('05', 'GASTROENTEROLOGIA', FALSE, FALSE),
    ('06', 'GINECOLOGIA', FALSE, FALSE),
    ('07', 'CIRURGICO/DIAGNOSTICO/TERAPEUTICO', FALSE, FALSE),
    ('08', 'NEFROLOGIAUROLOGIA', FALSE, FALSE),
    ('09', 'NEUROCIRURGIA', FALSE, FALSE),
    ('10', 'OBSTETRICIA CIRURGICA', FALSE, FALSE),
    ('11', 'OFTALMOLOGIA', FALSE, FALSE),
    ('12', 'ONCOLOGIA', FALSE, FALSE),
    ('13', 'ORTOPEDIATRAUMATOLOGIA', FALSE, FALSE),
    ('14', 'OTORRINOLARINGOLOGIA', FALSE, FALSE),
    ('15', 'PLASTICA', FALSE, FALSE),
    ('16', 'TORAXICA', FALSE, FALSE),
    ('31', 'AIDS', FALSE, FALSE),
    ('32', 'CARDIOLOGIA', FALSE, FALSE),
    ('33', 'CLINICA GERAL', FALSE, FALSE),
    ('34', 'CRONICOS', FALSE, FALSE),
    ('35', 'DERMATOLOGIA', FALSE, FALSE),
    ('36', 'GERIATRIA', FALSE, FALSE),
    ('37', 'HANSENOLOGIA', FALSE, FALSE),
    ('38', 'HEMATOLOGIA', FALSE, FALSE),
    ('39', 'LEITO/DIA', FALSE, FALSE),
    ('40', 'NEFROUROLOGIA', FALSE, FALSE),
    ('41', 'NEONATOLOGIA', FALSE, TRUE),
    ('42', 'NEUROLOGIA', FALSE, FALSE),
    ('43', 'OBSTETRICIA CLINICA', FALSE, FALSE),
    ('44', 'ONCOLOGIA', FALSE, FALSE),
    ('45', 'PEDIATRIA CLINICA', FALSE, FALSE),
    ('46', 'PNEUMOLOGIA', FALSE, FALSE),
    ('47', 'PSIQUIATRIA', FALSE, FALSE),
    ('48', 'REABILITACAO', FALSE, FALSE),
    ('49', 'PNEUMOLOGIA SANITARIA', FALSE, FALSE),
    ('61', 'UTI ADULTO', TRUE, FALSE),
    ('62', 'UTI INFANTIL', TRUE, FALSE),
    ('63', 'UTI NEONATAL', TRUE, TRUE),
    ('64', 'UNIDADE INTERMEDIARIA', FALSE, FALSE),
    ('65', 'UNIDADE INTERMEDIARIA NEONATAL', FALSE, TRUE),
    ('66', 'UNIDADE ISOLAMENTO', FALSE, FALSE),
    ('67', 'TRANSPLANTE', FALSE, FALSE),
    ('68', 'PEDIATRIA CIRURGICA', FALSE, FALSE),
    ('69', 'AIDS', FALSE, FALSE),
    ('70', 'FIBROSE CISTICA', FALSE, FALSE),
    ('71', 'INTERCORRENCIA POS-TRANSPLANTE', FALSE, FALSE),
    ('72', 'GERIATRIA', FALSE, FALSE),
    ('73', 'SAUDE MENTAL', FALSE, FALSE),
    ('74', 'UTI ADULTO - TIPO I', TRUE, FALSE),
    ('75', 'UTI ADULTO - TIPO II', TRUE, FALSE),
    ('76', 'UTI ADULTO - TIPO III', TRUE, FALSE),
    ('77', 'UTI PEDIATRICA - TIPO I', TRUE, FALSE),
    ('78', 'UTI PEDIATRICA - TIPO II', TRUE, FALSE),
    ('79', 'UTI PEDIATRICA - TIPO III', TRUE, FALSE),
    ('80', 'UTI NEONATAL - TIPO I', TRUE, TRUE),
    ('81', 'UTI NEONATAL - TIPO II', TRUE, TRUE),
    ('82', 'UTI NEONATAL - TIPO III', TRUE, TRUE),
    ('83', 'UTI DE QUEIMADOS', TRUE, FALSE),
    ('84', 'ACOLHIMENTO NOTURNO', FALSE, FALSE),
    ('85', 'UTI CORONARIANA TIPO II - UCO TIPO II', TRUE, FALSE),
    ('86', 'UTI CORONARIANA TIPO III - UCO TIPO III', TRUE, FALSE),
    ('87', 'SAUDE MENTAL', FALSE, FALSE),
    ('88', 'QUEIMADO ADULTO', FALSE, FALSE),
    ('89', 'QUEIMADO PEDIATRICO', FALSE, FALSE),
    ('90', 'QUEIMADO ADULTO', FALSE, FALSE),
    ('91', 'QUEIMADO PEDIATRICO', FALSE, FALSE),
    ('92', 'UNIDADE DE CUIDADOS INTERMED NEONATAL CONVENCIONAL', FALSE, TRUE),
    ('93', 'UNIDADE DE CUIDADOS INTERMED NEONATAL CANGURU', FALSE, TRUE),
    ('94', 'UNIDADE DE CUIDADOS INTERMEDIARIOS PEDIATRICO', FALSE, FALSE),
    ('95', 'UNIDADE DE CUIDADOS INTERMED  ADULTO', FALSE, FALSE)
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name, is_icu = EXCLUDED.is_icu,
    is_neonatal = EXCLUDED.is_neonatal;

-- The census itself. Public data, not account-scoped, exactly like
-- cnes_equipment — accounts read it filtered by their own territory.
CREATE TABLE IF NOT EXISTS cnes_beds (
    cnes        CHAR(7) NOT NULL,
    -- TP_LEITO, the ministry's coarse grouping (1 cirúrgico … 7 hospital-dia).
    -- Kept in the key because the file's own grain is (type, specialty) and a
    -- specialty can legitimately appear under more than one type.
    bed_type    CHAR(1) NOT NULL,
    -- CODLEITO, the specialty. Joins cnes_bed_types.
    bed_code    CHAR(2) NOT NULL,
    competencia CHAR(6) NOT NULL,
    uf          CHAR(2) NOT NULL,
    city_code   TEXT,
    qt_exist    INTEGER NOT NULL DEFAULT 0,
    qt_sus      INTEGER NOT NULL DEFAULT 0,
    qt_contr    INTEGER NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cnes, bed_type, bed_code, competencia)
);

CREATE INDEX IF NOT EXISTS idx_cnes_beds_uf_comp ON cnes_beds(uf, competencia);
CREATE INDEX IF NOT EXISTS idx_cnes_beds_cnes ON cnes_beds(cnes);
