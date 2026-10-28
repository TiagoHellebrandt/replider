## O que muda

<!-- Uma frase. Se precisar de mais, a PR provavelmente faz coisa demais. -->

## Checklist de fechamento (obrigatório se a PR toca banco, rota ou tela)

Este repositório já conviveu com três "pontas soltas" simultâneas — coluna
indexada que ninguém escrevia, dependência construída e descartada, import
sem porta para o funil — que 439 arquivos de teste não pegaram. Cada item
abaixo existe para fechar um desses buracos. Responda em texto, não com ✓.

**1. Qual ação do usuário, em português claro, esta mudança completa?**
<!-- "A vendedora importa o CSV e os clientes viram leads no funil."
     Se a resposta não nomeia uma pessoa fazendo algo, a mudança está solta. -->

**2. A rota nova/alterada está chamada pelo app? Onde? Se não, por quê?**
<!-- Arquivo em app/lib/ que a chama — ou a linha adicionada em
     api/scripts/dangling_route_exceptions.txt com justificativa. -->

**3. A coluna nova é escrita por algum INSERT/UPDATE? Onde?**
<!-- Query em db/queries/ ou repo raw-SQL que a grava. Backfill de migração
     NÃO conta como escritor — foi assim que contact_ref_id ficou NULL por
     187 migrações. -->

**4. A interface (porta) nova tem adapter ligado no main.go? Onde?**
<!-- Linha do cmd/api/main.go que constrói o adapter E o entrega a um
     consumidor. `_ = coisaConstruida` é o bug, não a ligação. -->

**5. Existe teste que exercita isto pelo caminho do usuário, sem semear o banco?**
<!-- Passo/asserção em internal/e2e/ (padrão: journey_first_customer_test.go).
     Teste que faz INSERT para montar o estado prova que as pontas existem,
     não que o meio liga. -->

## Verificação

- [ ] `go build ./...` · `go test ./...` (api/)
- [ ] `python3 scripts/check_dangling.py` sem ocorrência NOVA
- [ ] `python3 scripts/check_tenant_queries.py` verde
