-- Logs consultáveis de produção (para diagnóstico remoto via MCP).
--
-- Hoje o log vai para o stdout do contêiner e morre no arquivo do Docker
-- (10MB × 5). Para investigar uma falha em produção é preciso SSH na VPS —
-- e a essa altura a linha que interessava já rolou para fora do arquivo.
--
-- Esta tabela guarda só o que serve para diagnóstico: WARN e acima. INFO em
-- volume de produção afogaria a tabela e a conta, e não é o que se procura
-- quando alguma coisa quebrou.
CREATE TABLE IF NOT EXISTS app_logs (
    id          BIGSERIAL PRIMARY KEY,
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    level       TEXT        NOT NULL,
    msg         TEXT        NOT NULL,
    -- O mesmo id que viaja no cabeçalho X-Request-ID e no corpo de erro 5xx,
    -- para amarrar o que o cliente viu ao que o servidor registrou.
    request_id  TEXT,
    account_id  TEXT,
    -- Rota e código de erro quando existirem: são por onde se agrupa "o que
    -- está quebrando agora".
    path        TEXT,
    code        TEXT,
    -- Os atributos do registro, já passados pela redação de PII (US-050).
    attrs       JSONB       NOT NULL DEFAULT '{}'::jsonb
);

-- A consulta normal é "as últimas N linhas", às vezes filtrando por nível.
CREATE INDEX IF NOT EXISTS idx_app_logs_at ON app_logs (at DESC);
CREATE INDEX IF NOT EXISTS idx_app_logs_level_at ON app_logs (level, at DESC);
-- Amarrar um erro relatado pelo cliente ao registro do servidor.
CREATE INDEX IF NOT EXISTS idx_app_logs_request_id ON app_logs (request_id)
    WHERE request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_app_logs_account_at ON app_logs (account_id, at DESC)
    WHERE account_id IS NOT NULL;
