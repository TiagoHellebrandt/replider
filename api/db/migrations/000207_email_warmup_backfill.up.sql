-- Warmup fail-closed (Item 1): warmup_started_at nunca teve escritor — a
-- coluna ficava NULL e a guarda de aquecimento passava POR CAUSA disso (a
-- mesma doença do contact_ref_id). A guarda agora recusa cold send com NULL,
-- então as caixas JÁ VERIFICADAS em produção precisam do carimbo, ou o
-- conserto travaria quem já está conectado.
--
-- Data escolhida: a data da PRÓPRIA verificação mais antiga (DNS ou prova
-- SMTP, a que veio primeiro). Racional: a identidade começou a existir para
-- os provedores no momento em que foi provada — é o mesmo instante que os
-- escritores novos (MarkDNSVerified/MarkSMTPProven) passam a carimbar. Para
-- caixas verificadas há mais de 21 dias a rampa já terminou e nada muda no
-- limite diário; para as recentes, a rampa vale o resto que falta — que é o
-- comportamento que teriam tido se o carimbo existisse desde o início.
--
-- Caixas sem NENHUMA prova ficam NULL de propósito: elas não podem enviar
-- frio de qualquer jeito (gate de DNS/prova vem antes), e inventar uma data
-- para elas seria carimbar um fato que não aconteceu.
UPDATE email_mailboxes
SET warmup_started_at = LEAST(
        COALESCE(dns_verified_at, 'infinity'::timestamptz),
        COALESCE(smtp_verified_at, 'infinity'::timestamptz)
    ),
    updated_at = NOW()
WHERE warmup_started_at IS NULL
  AND (dns_verified_at IS NOT NULL OR smtp_verified_at IS NOT NULL);
