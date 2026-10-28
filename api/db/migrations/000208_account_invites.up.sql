-- Convites de criação de conta (fluxo sales-led, docs/fluxo-criacao-conta.md).
--
-- Com REGISTRATION_ENABLED=false a porta pública fecha; o convite é a única
-- permissão para nascer uma conta. Tabela de PLATAFORMA: sem account_id — a
-- conta que o convite cria ainda não existe quando a linha nasce.
--
-- token_hash guarda SÓ o sha256 do segredo (mesmo padrão dos api_tokens):
-- um vazamento de banco não vaza convites utilizáveis.
CREATE TABLE account_invites (
    id CHAR(26) PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    -- Binding: o cadastro só aceita ESTE e-mail. Link encaminhado para
    -- terceiro não funciona; e-mail errado = o dono reemite.
    email TEXT NOT NULL,
    plan_id CHAR(26) NOT NULL REFERENCES plans(id),
    overrides JSONB NOT NULL DEFAULT '{}',
    invited_by CHAR(26) REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    -- Consumo one-shot: used_at é escrito por UPDATE ... WHERE used_at IS
    -- NULL RETURNING — replay de link é impossível por construção.
    used_at TIMESTAMPTZ,
    -- SEM FK de propósito: o consumo é gravado ANTES da conta existir
    -- (consume-first é o que impede duas contas numa corrida pelo mesmo
    -- link), então uma FK aqui recusaria todo resgate. É um ponteiro de
    -- auditoria, não integridade referencial — e sobrevive à exclusão
    -- LGPD da conta, como o commission_ledger.
    used_account_id CHAR(26),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- O painel lista pendentes; o resgate busca por hash (já coberto pelo UNIQUE).
CREATE INDEX idx_account_invites_open
    ON account_invites (created_at DESC)
    WHERE used_at IS NULL AND revoked_at IS NULL;
