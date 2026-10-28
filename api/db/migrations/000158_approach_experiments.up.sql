-- A/B de abordagem medido por APROVAÇÃO SEM EDIÇÃO (US-F216, recorte viável).
--
-- Por que não taxa de resposta: a 10% de base, detectar +30% exige ~3.500
-- envios — 17 meses no tier de entrada. Um bandit rodando com 20 respostas
-- não otimiza, ele persegue ruído e escolhe um perdedor com confiança.
--
-- Aprovação-sem-edição é denso e imediato: existe no instante em que o
-- humano aprova, e a 60% de base detecta +20% em ~490 rascunhos (2,4 meses).
-- Taxa de resposta continua acumulando como métrica secundária, e vira
-- critério quando houver volume.
CREATE TABLE IF NOT EXISTS approach_experiments (
    id                CHAR(26) PRIMARY KEY,
    account_id        CHAR(26) NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    funnel_id         CHAR(26) NOT NULL REFERENCES sales_funnels(id) ON DELETE CASCADE,
    name              VARCHAR(120) NOT NULL,
    status            VARCHAR(20) NOT NULL DEFAULT 'running',
    -- Cada variante é um trecho ACRESCENTADO à persona do funil, não uma
    -- substituição: o teste compara uma mudança, não dois produtos.
    variant_a_suffix  TEXT NOT NULL DEFAULT '',
    variant_b_suffix  TEXT NOT NULL DEFAULT '',
    variant_a_label   VARCHAR(40) NOT NULL DEFAULT 'A',
    variant_b_label   VARCHAR(40) NOT NULL DEFAULT 'B',
    -- Modelo por variante, opcional: permite testar MODELO em vez de texto
    -- sem uma segunda máquina de experimento.
    variant_a_model   VARCHAR(64) NOT NULL DEFAULT '',
    variant_b_model   VARCHAR(64) NOT NULL DEFAULT '',
    split_pct_b       SMALLINT NOT NULL DEFAULT 50 CHECK (split_pct_b BETWEEN 1 AND 99),
    winner            CHAR(1) CHECK (winner IN ('A','B')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Um experimento rodando por funil. Dois ao mesmo tempo tornariam impossível
-- atribuir o efeito a qualquer um deles.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_approach_experiments_running_per_funnel
    ON approach_experiments (funnel_id) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_approach_experiments_account
    ON approach_experiments (account_id, created_at DESC);

-- Uma linha por rascunho gerado sob experimento. Atribuição e desfecho no
-- mesmo lugar: separá-los criaria um estado onde o lead foi sorteado mas o
-- resultado se perdeu, e um braço encolheria sem ninguém notar.
CREATE TABLE IF NOT EXISTS approach_variant_results (
    id             CHAR(26) PRIMARY KEY,
    experiment_id  CHAR(26) NOT NULL REFERENCES approach_experiments(id) ON DELETE CASCADE,
    lead_id        CHAR(26) NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    variant        CHAR(1) NOT NULL CHECK (variant IN ('A','B')),
    -- O rascunho como saiu do modelo, guardado para comparar com o que foi
    -- efetivamente enviado. Sem isso não existe "sem edição".
    drafted_text   TEXT NOT NULL,
    -- pending | approved_unedited | approved_edited | discarded
    outcome        VARCHAR(24) NOT NULL DEFAULT 'pending',
    -- Fração do texto alterada (0 = idêntico, 1 = reescrito). Distingue a
    -- vírgula trocada da reescrita inteira, que a flag booleana achata.
    edit_ratio     REAL NOT NULL DEFAULT 0,
    -- Riscos apontados no rascunho. Uma variante que gera menos alerta é
    -- melhor por definição, e isso é medido já no primeiro rascunho — sem
    -- esperar ninguém aprovar.
    risk_count     SMALLINT NOT NULL DEFAULT 0,
    decided_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Um resultado por lead por experimento: regerar o rascunho substitui a
-- linha em vez de inflar o braço com o mesmo lead várias vezes.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_approach_result_per_lead
    ON approach_variant_results (experiment_id, lead_id);
CREATE INDEX IF NOT EXISTS idx_approach_result_experiment
    ON approach_variant_results (experiment_id, variant, outcome);
