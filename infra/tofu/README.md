# Infra as code — DNS (US-F014)

> **Estado:** módulo escrito e `tofu validate` limpo. **Falta o que depende de você:** bucket de state, token Cloudflare e o `tofu import` dos registros que já existem. ~15 min.

## Escopo (e por que ele é pequeno de propósito)

Só DNS. O VPS e o compose continuam na mão até a migração para DOKS ser decidida (`docs/infra-as-code.md`) — codificar um host que a gente pretende abandonar não paga o custo.

O que interessa hoje é o **botão da migração**: trocar `origin_ipv4`, aplicar, e o tráfego segue. Em código, com histórico de quem mudou o quê.

## Detalhe que morde: TTL em registro proxied

Cloudflare **ignora TTL** em registro laranja (proxied) — o cliente sempre resolve para o anycast da Cloudflare, e a troca de origem vale na hora. TTL baixo só importa nos cinzas (`status`, por exemplo, que não pode depender da nossa própria borda). O módulo faz isso explicitamente: `ttl = proxied ? 1 : var.ttl`, e a validação recusa TTL fora de 60–300.

## Setup

### 1. Bucket de state
```bash
gcloud storage buckets create gs://replider-tofu-state \
  --location=southamerica-east1 --uniform-bucket-level-access
gcloud storage buckets update gs://replider-tofu-state --versioning
```
Versionamento ligado: state corrompido tem volta.

### 2. Token Cloudflare
Dashboard → My Profile → API Tokens → Create → **Edit zone DNS**, escopo **só** a zona `replider.com`.
```bash
export CLOUDFLARE_API_TOKEN=...
export GOOGLE_APPLICATION_CREDENTIALS=~/.config/gcloud/tofu-state.json
```

### 3. Importar o que já existe
Os registros **já estão criados** (a zona está no Cloudflare desde sempre). Importar evita o cenário clássico: `apply` recriando o que já existe e derrubando o site.

```bash
cd infra/tofu/environments/prod-hostinger
cp terraform.tfvars.example terraform.tfvars   # preencha zone id + IP atual
tofu init

# Liste os ids atuais:
curl -s -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?per_page=100" \
  | jq -r '.result[] | "\(.name)\t\(.type)\t\(.id)"'

tofu import 'module.dns.cloudflare_record.apex'            $ZONE_ID/$RECORD_ID_APEX
tofu import 'module.dns.cloudflare_record.www'             $ZONE_ID/$RECORD_ID_WWW
tofu import 'module.dns.cloudflare_record.subdomain["api"]'    $ZONE_ID/$RECORD_ID_API
tofu import 'module.dns.cloudflare_record.subdomain["app"]'    $ZONE_ID/$RECORD_ID_APP
tofu import 'module.dns.cloudflare_record.subdomain["docs"]'   $ZONE_ID/$RECORD_ID_DOCS
tofu import 'module.dns.cloudflare_record.subdomain["status"]' $ZONE_ID/$RECORD_ID_STATUS
```

### 4. O plano tem que sair limpo
```bash
tofu plan   # objetivo: "No changes."
```
Um `plan` com mudanças depois do import significa que a declaração diverge da realidade — **ajuste o código para a realidade**, não o contrário. Só depois disso o repositório vira a fonte da verdade.

> Se algum subdomínio não existir hoje (ex.: `status`, `docs`), remova da variável `subdomains` ou deixe o apply criar — mas decida conscientemente: um apply que cria DNS novo é barato, um que RECRIA DNS existente derruba o site pelo tempo da propagação.

## Migração (o botão)

```bash
tofu apply -var origin_ipv4=NOVO.IP.DA.MAQUINA
```
Ordem completa da troca de máquina (réplica lógica → read-only → promove → vira DNS → mantém a antiga 72h) está em `docs/infra-as-code.md`. Ensaiar 1×/trimestre.

## CI

`tofu validate` roda no workflow `infra-validate.yml` a cada push que toca `infra/tofu/**`. Não roda `plan` (exigiria credencial de produção num runner).
