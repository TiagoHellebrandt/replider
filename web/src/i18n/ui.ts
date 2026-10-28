// Landing v2 (2026-08-20): pt-only. O GTM americano foi descartado na
// auditoria de julho e cada texto em dobro custava revisão em dobro —
// /en/* agora responde 301 para /pt/* (astro.config redirects).
export const languages = {
  pt: "Português",
};

export const defaultLang = "pt";

export const ui = {
  pt: {
    "seo.title":
      "Replider — o agente de vendas de quem vende equipamento hospitalar",
    "seo.description":
      "O Replider sabe que equipamento cada hospital tem, o que ele está licitando — e escreve a oferta. Você aprova e manda. Para quem vende, aluga ou dá assistência em equipamento hospitalar.",
    "nav.home": "Início",
    "nav.como": "Como funciona",
    "nav.agendar": "Agendar conversa",
    "nav.try_free": "Agendar conversa",
    "hero.title":
      "O Replider sabe que equipamento cada hospital tem, o que ele está licitando — e escreve a oferta.",
    "hero.highlight": "Você aprova e manda.",
    "hero.subtitle":
      "Para quem vende, aluga ou dá assistência técnica em equipamento hospitalar. Dado público do CNES e do PNCP, cruzado com o seu catálogo e a sua carteira.",
    "hero.cta_book": "Agendar conversa",
    "hero.cta_whats": "Falar no WhatsApp",
    "hero.whats_prefill":
      "Olá! Vendo equipamento hospitalar e quero ver o Replider funcionando na minha operação.",
    "problem.eyebrow": "O problema, do jeito que o dono sente",
    "problem.l1": "Seu vendedor passa o dia correndo atrás de quem já pediu.",
    "problem.l2":
      "Os hospitais que nunca ouviram falar de você continuam comprando do concorrente.",
    "problem.l3": "E você só descobre como está a venda quando pergunta.",
    "how.eyebrow": "Como funciona",
    "how.title": "Três passos. O terceiro é seu.",
    "how.s1_title": "1. Traga o que você já tem",
    "how.s1_body":
      "Sua lista de produtos e de clientes, em planilha. O e-mail que a empresa já usa. Sem implantação, sem consultoria.",
    "how.s2_title": "2. A gente acha e escreve",
    "how.s2_body":
      "O parque instalado de cada hospital do seu território (CNES), os pregões abertos do que você vende (PNCP), o cliente que está pra repor. Para cada um: uma oferta escrita, com o produto certo do SEU catálogo, o motivo, e as objeções prováveis já antecipadas no texto.",
    "how.s3_title": "3. Você aprova",
    "how.s3_body":
      "Pelo painel, no computador ou no celular: aprova, edita ou pula cada oferta com um toque. Nada sai sem um humano dizer sim — nunca.",
    "owner.eyebrow": "O que o dono vê",
    "owner.title":
      "Cada negociação com estágio real, sem depender do relato de ninguém",
    "owner.body":
      "Quem parou há 7 dias, quem respondeu, quanto tem em aberto, qual cliente está esperando resposta. O funil é o mesmo que o vendedor usa — não existe versão pra reunião.",
    "lines.eyebrow": "Para as três linhas do negócio",
    "lines.venda_title": "Venda",
    "lines.venda_signal":
      "Pregão aberto do que você vende. Hospital do seu território com parque declarado no CNES — quantos equipamentos, de que tipo.",
    "lines.venda_offer":
      "Proposta pronta pra cotação, com prazo do edital no texto.",
    "lines.locacao_title": "Locação",
    "lines.locacao_signal": "Pregão de locação aberto. Parque de UTI declarado sem locação conhecida.",
    "lines.locacao_offer": "Oferta de locação com disponibilidade imediata.",
    "lines.assist_title": "Assistência",
    "lines.assist_signal":
      "Parque declarado no CNES sem contrato de manutenção conhecido.",
    "lines.assist_offer":
      "Contrato de manutenção com calibração e laudo pra vigilância.",
    "sources.eyebrow": "De onde vem o dado",
    "sources.title": "Dado público, cruzado com o seu catálogo",
    "sources.body":
      "CNES/DATASUS: o parque de equipamentos que cada estabelecimento de saúde declara ao Ministério, atualizado mensalmente. PNCP: toda contratação pública publicada, com objeto, valor e prazo. E a sua própria base: catálogo, clientes, histórico. Nada de scraping, nada de dado comprado.",
    "faq.title": "Perguntas frequentes",
    "faq.subtitle":
      "O que todo dono de distribuidora pergunta antes de agendar.",
    "faq.items": [
      {
        question: "Funciona com licitação?",
        answer:
          "Sim — é o coração do produto. O Replider vigia o PNCP com palavras derivadas do seu próprio catálogo; quando abre um pregão do que você vende no seu território, a oportunidade já chega com proposta rascunhada, valor estimado e prazo do edital.",
        category: "sales",
      },
      {
        question: "Meu vendedor precisa aprender um sistema novo?",
        answer:
          "Não. A fila de ofertas abre no celular como um app: ele aprova, edita ou pula cada oferta com um toque. O painel completo existe pra quem quiser.",
        category: "sales",
      },
      {
        question: "Vocês mandam mensagem sem eu ver?",
        answer:
          "Nunca. Cada mensagem a um cliente passa por aprovação humana, sempre. É regra de produto, não configuração.",
        category: "sales",
      },
      {
        question: "De onde vem a lista de hospitais?",
        answer:
          "Do CNES/DATASUS — o cadastro público onde cada estabelecimento de saúde declara seu parque de equipamentos, atualizado mensalmente pelo Ministério da Saúde. Cruzamos com o seu catálogo: quem tem ventilador precisa de manutenção; quem tem tomógrafo consome insumo.",
        category: "technical",
      },
      {
        question: "E o e-mail que eu já uso?",
        answer:
          "Conecta o Gmail da empresa (ou qualquer SMTP) e as ofertas saem da SUA caixa, com a sua assinatura. As respostas voltam para a mesma conversa, dentro do Replider — e o vendedor responde dali.",
        category: "technical",
      },
      {
        question: "Quanto custa?",
        answer:
          "Cada operação vende de um jeito, então o preço nasce na conversa — 30 minutos, olhando o seu catálogo e o seu território. Sem fidelidade.",
        category: "sales",
      },
      {
        question: "Meus dados ficam seguros?",
        answer:
          "Seu catálogo e seus clientes ficam na sua conta, criptografados, e não treinam modelo nenhum. LGPD por padrão: opt-out automático e trilha de auditoria.",
        category: "technical",
      },
    ],
    "book.cta": "Agendar conversa",
    "book.eyebrow": "Vamos conversar",
    "book.note":
      "30 minutos. Olhamos seu catálogo e seus clientes e mostramos como ficaria a primeira oferta.",
    "book.subtitle":
      "Sem tabela de preço. Cada distribuidora vende de um jeito, então a proposta nasce na conversa — do tamanho do seu time, do seu território e do que você vende.",
    "book.title": "Vamos conversar sobre a sua operação",
    "book.whatsapp": "Falar no WhatsApp",
    "book.write": "Escrever pra vendas",
    "cookie.accept": "Aceitar",
    "cookie.decline": "Recusar",
    "cookie.message":
      "Utilizamos cookies para melhorar sua experiência e analisar o tráfego do site.",
    "dpa.description":
      "Como o Replider trata dados pessoais em nome de seus clientes, em conformidade com a LGPD e o GDPR.",
    "dpa.heading": "Acordo de Tratamento de Dados",
    "dpa.intro":
      'Este Acordo de Tratamento de Dados ("DPA") integra os Termos de Serviço entre a Replider AI ("Replider", "nós") e o cliente ("Cliente", "você"). Ele rege o tratamento de dados pessoais que a Replider realiza em nome do Cliente quando o Cliente utiliza a plataforma. Quando o Cliente estiver sujeito à Lei Geral de Proteção de Dados (LGPD, Lei nº 13.709/2018) ou ao Regulamento Geral de Proteção de Dados europeu (GDPR), este DPA se aplica a esse tratamento. Em caso de conflito entre este DPA e os Termos de Serviço sobre uma questão de proteção de dados, prevalece este DPA.',
    "dpa.lastUpdated": "Última Atualização: Maio de 2026",
    "dpa.section1.items": [
      "Dados Pessoais — qualquer informação relacionada a uma pessoa natural identificada ou identificável tratada por meio da plataforma.",
      "Tratamento — qualquer operação realizada com dados pessoais, como coleta, armazenamento, uso, compartilhamento ou exclusão.",
      "Controlador — a parte que decide as finalidades e os meios do tratamento. O Cliente é o Controlador dos seus dados de contatos e de clientes finais.",
      "Operador (Processador, sob a LGPD) — a parte que trata dados pessoais em nome do Controlador. A Replider atua como Operadora dos dados que o Cliente direciona pela plataforma.",
      "Suboperador — terceiro contratado pela Replider para apoiar o tratamento de dados pessoais.",
      "Titular — a pessoa natural a quem os dados pessoais se referem, incluindo os clientes finais que contatam o Cliente pelos canais conectados.",
    ],
    "dpa.section1.title": "1. Definições",
    "dpa.section10.text":
      "Os dados pessoais podem ser tratados em infraestrutura ou por suboperadores localizados fora do país do Cliente, inclusive nos Estados Unidos e na União Europeia. Quando dados pessoais sujeitos à LGPD ou ao GDPR forem transferidos internacionalmente, a Replider se baseia em um mecanismo de transferência reconhecido — como uma decisão de adequação ou cláusulas contratuais padrão — e aplica salvaguardas compatíveis com a legislação aplicável.",
    "dpa.section10.title": "10. Transferências Internacionais de Dados",
    "dpa.section11.text":
      "A Replider disponibiliza ao Cliente as informações razoavelmente necessárias para demonstrar conformidade com este DPA. Mediante aviso prévio razoável por escrito, no máximo uma vez por ano, salvo exigência de uma autoridade fiscalizadora, e sujeito a confidencialidade, a Replider responderá a uma solicitação de auditoria razoável ou questionário de segurança relativo ao tratamento de dados pessoais sob este DPA.",
    "dpa.section11.title": "11. Auditorias e Conformidade",
    "dpa.section12.email": "E-mail: support@replider.com",
    "dpa.section12.text1":
      "Este DPA permanece em vigor enquanto a Replider tratar dados pessoais em nome do Cliente. Ao término da assinatura, o Cliente pode exportar seus dados por um período limitado; após esse período, ou mediante solicitação do Cliente, a Replider exclui permanente e irreversivelmente os dados pessoais tratados sob este DPA por meio de um processo de exclusão em cascata, exceto quando a retenção for exigida por lei (por exemplo, registros de cobrança mantidos para fins fiscais).",
    "dpa.section12.text2":
      "Para dúvidas sobre este DPA, para solicitar uma cópia assinada ou para tratar de uma questão de proteção de dados, entre em contato com o responsável pela proteção de dados da Replider em:",
    "dpa.section12.title": "12. Vigência, Devolução e Exclusão dos Dados",
    "dpa.section2.text":
      "O Cliente é o Controlador dos dados pessoais que envia ou coleta por meio da plataforma — incluindo o conteúdo das conversas, dados de contato e material da base de conhecimento. A Replider é a Operadora e trata esses dados apenas para fornecer a plataforma, e somente seguindo instruções documentadas do Cliente, dadas por meio dos Termos de Serviço, deste DPA e das escolhas de configuração feitas no produto. A Replider é Controladora independente dos dados de conta limitados de que precisa para operar o negócio — credenciais de login, registros de cobrança e métricas de uso da plataforma — os quais são regidos pela Política de Privacidade, e não por este DPA.",
    "dpa.section2.title": "2. Papéis e Escopo",
    "dpa.section3.items": [
      "Objeto e duração — tratamento de dados pessoais durante a vigência da assinatura e até que os dados sejam excluídos conforme a Seção 12.",
      "Natureza e finalidade — receber, armazenar e encaminhar mensagens; gerar respostas de IA; busca semântica na base de conhecimento; histórico de conversas; e análises, tudo para entregar a plataforma.",
      "Categorias de dados pessoais — identificadores e dados de contato (nome, telefone, identificadores de mensageria), o conteúdo das mensagens trocadas e quaisquer dados adicionados à base de conhecimento ou à configuração do agente.",
      "Categorias de titulares — os clientes finais e contatos do Cliente, e os membros da equipe do Cliente que usam o painel.",
      "Dados sensíveis — a plataforma não foi projetada para tratar dados pessoais sensíveis ou de categorias especiais; o Cliente não deve direcionar tais dados sem um acordo escrito separado.",
    ],
    "dpa.section3.text":
      "O tratamento que a Replider realiza como Operadora possui as seguintes características:",
    "dpa.section3.title": "3. Detalhes do Tratamento",
    "dpa.section4.items": [
      "Tratar dados pessoais apenas seguindo as instruções documentadas do Cliente, salvo exigência legal em contrário, caso em que a Replider informará o Cliente quando legalmente permitido.",
      "Garantir que o pessoal autorizado a tratar dados pessoais esteja sujeito a obrigações de confidencialidade.",
      "Implementar e manter as medidas de segurança técnicas e organizacionais descritas na Seção 7.",
      "Auxiliar o Cliente, considerando a natureza do tratamento, no atendimento às solicitações dos titulares e no cumprimento de suas obrigações de segurança, comunicação de incidentes e avaliação de impacto.",
      "Disponibilizar as informações razoavelmente necessárias para demonstrar conformidade com este DPA.",
      "Contratar suboperadores apenas nas condições previstas na Seção 6.",
      "Excluir ou devolver os dados pessoais ao fim da relação, conforme a Seção 12.",
    ],
    "dpa.section4.title": "4. Obrigações da Replider como Operadora",
    "dpa.section5.items": [
      "Estabelecer e manter uma base legal para o tratamento dos dados pessoais direcionados pela plataforma.",
      "Fornecer os avisos exigidos aos titulares e, quando necessário, obter o consentimento deles.",
      "Emitir apenas instruções de tratamento lícitas e configurar a plataforma de acordo.",
      "Não enviar dados pessoais sensíveis ou de categorias especiais, nem dados de crianças, exceto se acordado por escrito separadamente.",
      "Responder às solicitações dos titulares na qualidade de Controlador, com o auxílio que a Replider presta sob este DPA.",
    ],
    "dpa.section5.title": "5. Obrigações do Cliente como Controlador",
    "dpa.section6.text1":
      "O Cliente autoriza a Replider a contratar suboperadores para apoiar a plataforma. A Replider permanece responsável pelo desempenho de seus suboperadores e lhes impõe obrigações de proteção de dados não menos protetivas do que as deste DPA. Os suboperadores atuais incluem provedores de infraestrutura e armazenamento em nuvem, provedores de modelos de IA (OpenAI e Google), o processador de pagamentos (Stripe), o provedor de envio de e-mail e as plataformas de mensageria que o Cliente opta por conectar (Meta — WhatsApp e Instagram —, Telegram e Twilio para SMS).",
    "dpa.section6.text2":
      "A Replider dará aviso prévio ao Cliente sobre qualquer pretensão de adicionar ou substituir um suboperador. Se o Cliente se opuser de forma razoável por motivos de proteção de dados, as partes trabalharão de boa-fé para encontrar uma solução; se nenhuma for encontrada, o Cliente poderá rescindir o serviço afetado.",
    "dpa.section6.title": "6. Suboperadores",
    "dpa.section7.items": [
      "Criptografia dos dados pessoais em trânsito (TLS 1.2 ou superior) e em repouso.",
      "Isolamento multi-tenant rigoroso, de modo que cada conta acesse apenas os próprios dados.",
      "Controles de acesso baseados em papéis e princípio do menor privilégio para o acesso interno.",
      "Credenciais com hash (bcrypt) e URLs assinadas com tempo limitado para acesso a mídia.",
      "Registro de auditoria, monitoramento estruturado e revisões de segurança regulares.",
      "Backups de banco de dados criptografados e testados regularmente.",
    ],
    "dpa.section7.text":
      "A Replider mantém medidas técnicas e organizacionais apropriadas ao risco, incluindo:",
    "dpa.section7.title": "7. Medidas de Segurança",
    "dpa.section8.text":
      "Se a Replider tomar conhecimento de um incidente de segurança que afete dados pessoais tratados sob este DPA, ela notificará o Cliente sem demora indevida e, em todo caso, em até 48 horas após confirmar o incidente. A notificação descreverá a natureza do incidente, os dados e titulares afetados na medida do conhecido, as consequências prováveis e as medidas tomadas ou propostas. A Replider cooperará com o Cliente para que este possa cumprir suas próprias obrigações de comunicação às autoridades e aos titulares.",
    "dpa.section8.title": "8. Incidente com Dados Pessoais",
    "dpa.section9.text":
      "Os titulares exercem seus direitos — acesso, correção, exclusão, portabilidade, oposição e informação sobre o tratamento — junto ao Controlador. Quando um titular contatar a Replider diretamente, a Replider encaminhará a solicitação ao Cliente, salvo se legalmente obrigada a responder. Considerando a natureza do tratamento, a Replider fornece ferramentas e auxílio para que o Cliente atenda a essas solicitações, incluindo a possibilidade de exportar e excluir permanentemente os dados da conta pelo painel.",
    "dpa.section9.title": "9. Solicitações dos Titulares",
    "dpa.title": "Acordo de Tratamento de Dados - Replider",
    "faq.tab_billing": "Cobrança",
    "faq.tab_sales": "Vendas",
    "faq.tab_technical": "Técnico",
    "footer.contact": "Contato",
    "footer.dpa": "Tratamento de Dados",
    "footer.privacy": "Privacidade",
    "footer.rights": "© 2026 Replider AI. Todos os direitos reservados.",
    "footer.terms": "Termos",
    "pricing.sales_form_channels": "Canais que você usa",
    "pricing.sales_form_close": "Fechar",
    "pricing.sales_form_company": "Nome da empresa",
    "pricing.sales_form_email": "E-mail corporativo",
    "pricing.sales_form_error":
      "Algo deu errado. Tente novamente ou escreva para sales@replider.com.",
    "pricing.sales_form_message": "Mais alguma coisa? (opcional)",
    "pricing.sales_form_required":
      "Preencha todos os campos obrigatórios com um e-mail válido.",
    "pricing.sales_form_size": "Tamanho da empresa",
    "pricing.sales_form_size_placeholder": "Selecione o tamanho…",
    "pricing.sales_form_submit": "Enviar para vendas",
    "pricing.sales_form_submitting": "Enviando…",
    "pricing.sales_form_subtitle":
      "Conte sobre a sua operação e nosso time monta um plano Enterprise sob medida.",
    "pricing.sales_form_success":
      "Obrigado! Nosso time de vendas entrará em contato em breve.",
    "pricing.sales_form_title": "Falar com vendas",
    "pricing.sales_form_volume": "Volume mensal de mensagens",
    "pricing.sales_form_volume_placeholder": "Selecione o volume…",
    "privacy.description":
      "Saiba como o Replider coleta, utiliza e protege seus dados pessoais.",
    "privacy.heading": "Política de Privacidade",
    "privacy.intro":
      'No Replider ("nós" ou "nosso"), estamos comprometidos em proteger sua privacidade e seus dados pessoais. Esta Política de Privacidade explica como coletamos, usamos, armazenamos e compartilhamos suas informações quando você utiliza nossa plataforma e serviços.',
    "privacy.lastUpdated": "Última Atualização: Abril de 2026",
    "privacy.section1.subtitle1": "Dados da Conta",
    "privacy.section1.subtitle2": "Dados de Uso",
    "privacy.section1.subtitle3": "Cookies e Rastreamento",
    "privacy.section1.subtitle4": "Dados de Conversas",
    "privacy.section1.text1":
      "Ao criar uma conta, coletamos seu nome, endereço de e-mail e senha (criptografada). Se você fizer login via Google ou Meta/Facebook, recebemos suas informações de perfil desses provedores.",
    "privacy.section1.text2":
      "Coletamos automaticamente informações sobre como você interage com nossa plataforma, incluindo páginas visitadas, recursos utilizados, volumes de conversas, consumo de tokens de IA e métricas de desempenho.",
    "privacy.section1.text3":
      "Utilizamos cookies essenciais para autenticação e gerenciamento de sessão. Cookies de análise são configurados apenas com seu consentimento. Você pode gerenciar suas preferências de cookies a qualquer momento.",
    "privacy.section1.text4":
      "Mensagens trocadas através dos seus canais conectados (WhatsApp, Telegram, Instagram, SMS) são armazenadas para permitir respostas de IA, histórico de conversas e recursos de base de conhecimento.",
    "privacy.section1.title": "1. Informações que Coletamos",
    "privacy.section2.items": [
      "Fornecer, manter e melhorar nossa plataforma de atendimento ao cliente com IA",
      "Processar respostas de IA usando sua base de conhecimento e histórico de conversas",
      "Gerenciar sua conta, cobrança e assinatura",
      "Enviar e-mails transacionais (verificação de conta, recuperação de senha, notificações de cobrança)",
      "Monitorar o desempenho da plataforma e prevenir abusos",
      "Cumprir obrigações legais",
    ],
    "privacy.section2.title": "2. Como Usamos Suas Informações",
    "privacy.section3.text":
      "Seus dados são armazenados em infraestrutura de nuvem segura com criptografia em repouso e em trânsito (TLS 1.2+). Implementamos medidas de segurança padrão da indústria, incluindo controles de acesso, registro de auditoria e revisões de segurança regulares. As senhas são criptografadas usando bcrypt. Arquivos de mídia são armazenados em armazenamento privado em nuvem (Google Cloud Storage) e acessados apenas por meio de URLs assinadas com tempo limitado.",
    "privacy.section3.title": "3. Armazenamento e Segurança dos Dados",
    "privacy.section4.items": [
      "Stripe — Processamento de pagamentos. Sujeito à Política de Privacidade do Stripe.",
      "Google Cloud Platform — Infraestrutura e armazenamento.",
      "Google OAuth e Calendar — Autenticação e integração de calendário.",
      "Meta/WhatsApp Business API — Canal de mensagens WhatsApp.",
      "Meta/Instagram Messaging API — Canal de DMs do Instagram.",
      "Telegram Bot API — Canal de mensagens Telegram.",
      "Twilio — Canal de mensagens SMS, conectado através da sua própria conta Twilio.",
      "OpenAI e Google AI — Provedores de modelos de IA para geração de respostas.",
    ],
    "privacy.section4.note": "Não vendemos seus dados pessoais a terceiros.",
    "privacy.section4.text":
      "Integramos com os seguintes serviços de terceiros para fornecer nossa plataforma:",
    "privacy.section4.title": "4. Serviços de Terceiros",
    "privacy.section5.items": [
      "Acesso — Solicitar uma cópia dos seus dados pessoais",
      "Correção — Solicitar a correção de dados imprecisos",
      "Exclusão — Solicitar a exclusão da sua conta e todos os dados associados",
      "Portabilidade — Solicitar seus dados em formato portável",
      "Oposição — Opor-se a determinadas atividades de processamento de dados",
      "Revogação de Consentimento — Retirar o consentimento para processamento de dados opcional a qualquer momento",
    ],
    "privacy.section5.note":
      "Para exercer qualquer um desses direitos, entre em contato conosco em support@replider.com. A exclusão da conta também pode ser realizada diretamente nas configurações do seu painel. Processamos todas as solicitações em até 15 dias úteis.",
    "privacy.section5.text":
      "De acordo com a Lei Geral de Proteção de Dados (LGPD) e o Regulamento Geral de Proteção de Dados (GDPR), você possui os seguintes direitos:",
    "privacy.section5.title": "5. Seus Direitos (LGPD/GDPR)",
    "privacy.section6.text":
      "Mantemos seus dados enquanto sua conta estiver ativa. Quando você exclui sua conta, todos os dados associados (conversas, agentes, canais, base de conhecimento, mídia) são permanente e irreversivelmente excluídos por meio de um processo de exclusão em cascata. Registros de cobrança podem ser mantidos por até 5 anos, conforme exigido pela legislação tributária.",
    "privacy.section6.title": "6. Retenção de Dados",
    "privacy.section7.text":
      "Nossa plataforma não é destinada ao uso por menores de 18 anos. Não coletamos intencionalmente dados pessoais de crianças.",
    "privacy.section7.title": "7. Privacidade de Menores",
    "privacy.section8.text":
      "Podemos atualizar esta Política de Privacidade periodicamente. Notificaremos você sobre alterações significativas por e-mail ou pela plataforma. O uso continuado da plataforma após as alterações constitui aceitação da política atualizada.",
    "privacy.section8.title": "8. Alterações nesta Política",
    "privacy.gmail.title": "10. Acesso ao Gmail (escopo restrito do Google)",
    "privacy.gmail.text":
      "Quando você conecta a caixa Gmail da sua empresa, o Replider pede o escopo https://mail.google.com/ exclusivamente para: (a) enviar as mensagens que um humano da sua conta aprovou, em seu nome; e (b) ler as respostas recebidas para exibi-las na sua caixa unificada. O token OAuth fica criptografado em repouso (AES-256-GCM), nunca é exposto a terceiros e não é usado para treinar modelos. Você revoga o acesso a qualquer momento nas configurações da sua Conta Google (myaccount.google.com/permissions) ou desconectando o canal no painel — a revogação apaga o token imediatamente. O uso de dados recebidos das APIs do Google segue a Política de Dados do Usuário dos Serviços de API do Google, incluindo os requisitos de Uso Limitado.",
    "privacy.section9.dpo":
      "Encarregado de Dados (DPO, LGPD Art. 41): dpo@replider.com",
    "privacy.section9.email": "E-mail: support@replider.com",
    "privacy.section9.text":
      "Para dúvidas sobre esta Política de Privacidade ou para exercer seus direitos de dados, entre em contato conosco em:",
    "privacy.section9.title": "9. Informações de Contato",
    "privacy.title": "Política de Privacidade - Replider",
    "terms.description":
      "Leia os termos e condições que regem o uso dos serviços do Replider.",
    "terms.heading": "Termos de Serviço",
    "terms.intro":
      'Estes Termos de Serviço ("Termos") regem seu acesso e uso da plataforma e serviços do Replider ("Serviço") operados pelo Replider ("nós" ou "nosso"). Ao criar uma conta ou usar o Serviço, você concorda em ficar vinculado a estes Termos.',
    "terms.lastUpdated": "Última Atualização: Abril de 2026",
    "terms.section1.text":
      "O Replider é uma plataforma omnichannel de atendimento ao cliente com IA que permite que empresas criem agentes inteligentes, conectem canais de mensagens (WhatsApp, Telegram, Instagram, SMS), gerenciem conversas por meio de um painel em tempo real e automatizem interações com clientes usando uma base de conhecimento alimentada por tecnologia de geração aumentada por recuperação (RAG).",
    "terms.section1.title": "1. Descrição do Serviço",
    "terms.section10.email": "E-mail: support@replider.com",
    "terms.section10.text":
      "Para dúvidas sobre estes Termos de Serviço, entre em contato conosco em:",
    "terms.section10.title": "10. Informações de Contato",
    "terms.section2.items": [
      "Você deve ter pelo menos 18 anos para criar uma conta.",
      "Você deve fornecer informações de registro precisas e completas.",
      "Você é responsável por manter a segurança das credenciais da sua conta.",
      "Você deve verificar seu endereço de e-mail antes de acessar a plataforma.",
      "Uma pessoa ou entidade jurídica pode manter várias contas, cada uma com uma assinatura separada.",
      "Você é responsável por toda a atividade que ocorre em sua conta.",
    ],
    "terms.section2.title": "2. Termos da Conta",
    "terms.section3.items": [
      "Planos pagos são cobrados mensalmente através do Stripe.",
      "Os preços são listados na moeda local da sua região (USD ou BRL).",
      "Um teste gratuito com uso limitado é fornecido no registro (sem necessidade de cartão de crédito).",
      "Upgrades ou downgrades entram em vigor imediatamente.",
      "Contadores de uso (conversas, tokens de IA) são redefinidos mensalmente após o pagamento da fatura.",
      "Pacotes de créditos adicionais podem ser adquiridos separadamente.",
      "Reservamo-nos o direito de alterar preços com 30 dias de aviso prévio.",
    ],
    "terms.section3.title": "3. Pagamento e Cobrança",
    "terms.section4.items": [
      "Violar quaisquer leis ou regulamentos aplicáveis",
      "Enviar spam, mensagens não solicitadas ou comunicações em massa",
      "Distribuir malware, tentativas de phishing ou conteúdo prejudicial",
      "Se passar por outros indivíduos ou organizações",
      "Infringir direitos de propriedade intelectual de terceiros",
      "Tentar obter acesso não autorizado ao Serviço ou a outras contas",
      "Usar a plataforma para fins ilegais, abusivos ou discriminatórios",
      "Contornar limites de uso ou restrições da plataforma",
    ],
    "terms.section4.text": "Você concorda em não usar o Serviço para:",
    "terms.section4.title": "4. Política de Uso Aceitável",
    "terms.section5.text1":
      "O Serviço, incluindo seu design, código, recursos e marca, é de propriedade do Replider e protegido por leis de propriedade intelectual. Você mantém total propriedade do seu conteúdo, incluindo documentos da base de conhecimento, dados de conversas e qualquer mídia enviada para a plataforma.",
    "terms.section5.text2":
      "Você nos concede uma licença limitada para processar seu conteúdo exclusivamente com o propósito de fornecer o Serviço (por exemplo, gerar respostas de IA, armazenar histórico de conversas).",
    "terms.section5.title": "5. Propriedade Intelectual",
    "terms.section6.text1":
      'O Serviço é fornecido "como está" sem garantias de qualquer tipo, expressas ou implícitas. Não garantimos operação ininterrupta ou livre de erros do Serviço.',
    "terms.section6.text2":
      "Na extensão máxima permitida por lei, o Replider não será responsável por quaisquer danos indiretos, incidentais, especiais, consequenciais ou punitivos, incluindo, mas não se limitando a, perda de lucros, dados ou oportunidades de negócio, decorrentes do uso do Serviço.",
    "terms.section6.text3":
      "Nossa responsabilidade total por qualquer reclamação decorrente destes Termos ou do Serviço não excederá o valor que você nos pagou nos 12 meses anteriores à reclamação.",
    "terms.section6.text4":
      "Respostas geradas por IA são fornecidas como assistência e podem conter imprecisões. Você é responsável por revisar e verificar as saídas da IA conforme apropriado para seu caso de uso.",
    "terms.section6.title": "6. Limitação de Responsabilidade",
    "terms.section7.text1":
      "Você pode cancelar sua assinatura e excluir sua conta a qualquer momento nas configurações do seu painel. Após a exclusão da conta, todos os seus dados serão permanentemente removidos de acordo com nossa Política de Privacidade.",
    "terms.section7.text2":
      "Podemos suspender ou encerrar sua conta se você violar estes Termos, se envolver em conduta abusiva ou não pagar as taxas aplicáveis. Forneceremos aviso razoável antes do encerramento, exceto em casos de violações graves.",
    "terms.section7.title": "7. Rescisão",
    "terms.section8.text":
      "Estes Termos serão regidos e interpretados de acordo com as leis da República Federativa do Brasil. Quaisquer disputas decorrentes destes Termos serão submetidas aos tribunais do Brasil, sem prejuízo de qualquer outra jurisdição que possa se aplicar conforme as leis de proteção ao consumidor aplicáveis.",
    "terms.section8.title": "8. Lei Aplicável",
    "terms.section9.text":
      "Podemos atualizar estes Termos periodicamente. Alterações significativas serão comunicadas por e-mail ou pela plataforma com pelo menos 30 dias de antecedência. O uso continuado do Serviço após as alterações entrarem em vigor constitui aceitação dos Termos atualizados.",
    "terms.section9.title": "9. Alterações nestes Termos",
    "terms.title": "Termos de Serviço - Replider",
    "wa_fab.cta": "Iniciar conversa",
    "wa_fab.label": "Fale com a gente no WhatsApp",
    "wa_fab.prefill": "Olá! Tenho uma dúvida sobre o Replider.",
    "wa_fab.subtitle":
      "Vendas, suporte ou tirar uma dúvida rápida — respondemos em poucas horas em dias úteis.",
  },
} as const;
