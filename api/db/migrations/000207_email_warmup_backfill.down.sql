-- Sem reversão de dados: o up preencheu warmup_started_at a partir das datas
-- de verificação, e "des-saber" quando uma identidade começou não devolve
-- estado nenhum — apenas reabriria o buraco fail-open que o backfill fechou.
-- No-op deliberado.
SELECT 1;
