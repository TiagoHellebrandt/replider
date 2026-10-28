-- Seed demo data: Clinica Saude+ Odontologia
-- Only insert if demo account doesn't exist
-- Uses fixed ULID-style IDs so data is predictable and deletable.
--
-- Demo credentials:
--   Email:    demo@replider.com
--   Password: demo1234

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM accounts WHERE id = '01DEMO_ACCOUNT_00000000') THEN

    -- Account (uses Starter plan = Bronze)
    INSERT INTO accounts (id, name, plan_id, subscription_status, created_at, updated_at)
    VALUES ('01DEMO_ACCOUNT_00000000', 'Clinica Saude+ Odontologia', '01PLAN_BRONZE_000000000', 'active',
            NOW() - INTERVAL '30 days', NOW());

    -- Admin user: Dr. Maria Silva (demo@replider.com / demo1234)
    -- bcrypt hash of "demo1234" with cost 12
    INSERT INTO users (id, account_id, name, email, password_hash, role, is_active, is_email_verified, created_at, updated_at)
    VALUES ('01DEMO_ADMIN_000000000', '01DEMO_ACCOUNT_00000000', 'Dr. Maria Silva', 'demo@replider.com',
            '$2a$12$Z3w1uue2sAwI0g9uXkqIkOtSLOtlU7GxWELfX3sXoK.2JEztK/Ry.', 'admin', true, true,
            NOW() - INTERVAL '30 days', NOW());

    -- Attendant user: Recepcao
    INSERT INTO users (id, account_id, name, email, password_hash, role, is_active, is_email_verified, created_at, updated_at)
    VALUES ('01DEMO_ATTEND_00000000', '01DEMO_ACCOUNT_00000000', 'Recepcao', 'recepcao@replider.com',
            '$2a$12$Z3w1uue2sAwI0g9uXkqIkOtSLOtlU7GxWELfX3sXoK.2JEztK/Ry.', 'attendant', true, true,
            NOW() - INTERVAL '25 days', NOW());

    -- Usage record
    INSERT INTO account_usages (id, account_id, billing_cycle_start, conversations_count, ai_tokens_used, ai_input_tokens_used, ai_output_tokens_used, storage_used_mb, warned_80_percent, warned_100_percent)
    VALUES ('01DEMO_USAGE_000000000', '01DEMO_ACCOUNT_00000000', DATE_TRUNC('month', NOW())::DATE,
            47, 38500, 19200, 19300, 5, false, false);

    -- Agent: Assistente Saude+
    INSERT INTO agents (id, account_id, name, system_prompt, provider, model, timezone, is_active, created_at, updated_at)
    VALUES ('01DEMO_AGENT_000000000', '01DEMO_ACCOUNT_00000000', 'Assistente Saude+',
            'Voce e a assistente virtual da Clinica Saude+ Odontologia. Seu nome e Sofia.

Informacoes da clinica:
- Nome: Clinica Saude+ Odontologia
- Endereco: Rua Augusta, 1200 - Consolacao, Sao Paulo - SP
- Telefone: (11) 3456-7890
- Horario: Segunda a Sexta, 8h as 18h. Sabado, 8h as 12h.
- Especialidades: Ortodontia, Implantes, Clareamento, Limpeza, Endodontia

Regras:
1. Sempre seja educada e profissional
2. Para agendamentos, colete: nome completo, telefone, procedimento desejado e preferencia de horario
3. Nao forneca diagnosticos - apenas informacoes gerais e agendamentos
4. Em caso de emergencia, oriente o paciente a ligar para (11) 3456-7890
5. Responda sempre em portugues',
            'google', 'gemini-3.1-flash-live-preview', 'America/Sao_Paulo', true,
            NOW() - INTERVAL '28 days', NOW());

    -- Channel: Telegram "Canal Principal"
    INSERT INTO channels (id, agent_id, provider, name, credentials, webhook_token, is_active, created_at, updated_at)
    VALUES ('01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000', 'telegram', 'Canal Principal',
            '{"bot_token": "0000000000:DEMO_TOKEN_NOT_REAL"}'::jsonb,
            '01DEMO_WEBHOOK_0000000', true,
            NOW() - INTERVAL '27 days', NOW());

    -- Knowledge source: FAQ Clinica
    INSERT INTO knowledge_sources (id, agent_id, name, description, created_at)
    VALUES ('01DEMO_KSOURCE_000000', '01DEMO_AGENT_000000000', 'FAQ Clinica',
            'Perguntas frequentes da clinica odontologica',
            NOW() - INTERVAL '26 days');

    -- Knowledge chunks
    INSERT INTO knowledge_chunks (id, source_id, content, provider, source_name, created_at) VALUES
    ('01DEMO_KCHUNK_0000001', '01DEMO_KSOURCE_000000',
     'Quanto custa uma limpeza dental? O valor da limpeza na Clinica Saude+ e de R$ 180,00. Para pacientes com plano odontologico, verificamos a cobertura. A limpeza inclui raspagem, polimento e aplicacao de fluor. Recomendamos fazer a cada 6 meses.',
     'google', 'FAQ Clinica', NOW() - INTERVAL '26 days'),
    ('01DEMO_KCHUNK_0000002', '01DEMO_KSOURCE_000000',
     'Como funciona o clareamento dental? Oferecemos duas opcoes: clareamento a laser (1 sessao de 1h, R$ 800) e clareamento com moldeira (kit para uso em casa por 2 semanas, R$ 500). Ambos os metodos sao seguros e eficazes. E necessaria uma avaliacao previa.',
     'google', 'FAQ Clinica', NOW() - INTERVAL '26 days'),
    ('01DEMO_KCHUNK_0000003', '01DEMO_KSOURCE_000000',
     'Quais formas de pagamento sao aceitas? Aceitamos dinheiro, PIX, cartao de debito e credito (parcelamos em ate 12x sem juros para valores acima de R$ 500). Tambem trabalhamos com os principais convenios odontologicos.',
     'google', 'FAQ Clinica', NOW() - INTERVAL '26 days'),
    ('01DEMO_KCHUNK_0000004', '01DEMO_KSOURCE_000000',
     'Como agendar uma consulta? Voce pode agendar pelo WhatsApp, telefone (11) 3456-7890, ou diretamente comigo aqui no chat. Preciso do seu nome completo, telefone para contato e o procedimento desejado. Temos horarios disponiveis de segunda a sexta das 8h as 18h.',
     'google', 'FAQ Clinica', NOW() - INTERVAL '26 days'),
    ('01DEMO_KCHUNK_0000005', '01DEMO_KSOURCE_000000',
     'Qual a idade minima para colocar aparelho ortodontico? A avaliacao ortodontica e recomendada a partir dos 7 anos. O tratamento com aparelho fixo geralmente inicia entre 11 e 14 anos, mas pode ser indicado em qualquer idade. Agende uma avaliacao com nosso ortodontista Dr. Carlos.',
     'google', 'FAQ Clinica', NOW() - INTERVAL '26 days');

    -- =========================================================================
    -- Conversations + Messages
    -- =========================================================================

    -- Conv 1: Joao Santos - Agendamento (open)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000001', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511987654321', 'Joao Santos', 'open', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000001', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000001', 'tg_5511987654321', 'Joao Santos', 'text', 'Ola, gostaria de agendar uma consulta de avaliacao', 'inbound', NOW() - INTERVAL '2 hours'),
    ('01DEMO_MSG_00000000002', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000001', 'bot', 'Assistente Saude+', 'text', 'Ola Joao! Que bom que voce entrou em contato com a Clinica Saude+! Ficaremos felizes em agendar sua avaliacao. Temos horarios disponiveis de segunda a sexta, das 8h as 18h, e sabado das 8h as 12h. Qual dia e horario seria melhor para voce?', 'outbound', NOW() - INTERVAL '2 hours' + INTERVAL '3 seconds'),
    ('01DEMO_MSG_00000000003', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000001', 'tg_5511987654321', 'Joao Santos', 'text', 'Tem horario na quinta-feira a tarde?', 'inbound', NOW() - INTERVAL '1 hour 30 minutes'),
    ('01DEMO_MSG_00000000004', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000001', 'bot', 'Assistente Saude+', 'text', 'Claro! Na quinta-feira temos os seguintes horarios disponiveis a tarde: 14h, 15h e 16h30. A consulta de avaliacao tem duracao de aproximadamente 40 minutos. Qual horario voce prefere?', 'outbound', NOW() - INTERVAL '1 hour 30 minutes' + INTERVAL '4 seconds'),
    ('01DEMO_MSG_00000000005', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000001', 'tg_5511987654321', 'Joao Santos', 'text', 'Vou querer as 15h. Preciso levar algum documento?', 'inbound', NOW() - INTERVAL '1 hour');

    -- Conv 2: Ana Costa - Duvida sobre exame (open)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000002', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511912345678', 'Ana Costa', 'open', NOW() - INTERVAL '4 hours', NOW() - INTERVAL '5 hours', NOW() - INTERVAL '4 hours');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000006', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000002', 'tg_5511912345678', 'Ana Costa', 'text', 'Boa tarde! Preciso fazer uma radiografia panoramica. Voces fazem?', 'inbound', NOW() - INTERVAL '5 hours'),
    ('01DEMO_MSG_00000000007', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000002', 'bot', 'Assistente Saude+', 'text', 'Boa tarde, Ana! Sim, realizamos radiografia panoramica aqui na clinica. O exame e rapido, leva cerca de 5 minutos, e o resultado sai na hora. Gostaria de agendar?', 'outbound', NOW() - INTERVAL '5 hours' + INTERVAL '3 seconds'),
    ('01DEMO_MSG_00000000008', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000002', 'tg_5511912345678', 'Ana Costa', 'text', 'Aceita convenio Amil?', 'inbound', NOW() - INTERVAL '4 hours');

    -- Conv 3: Carlos Lima - Cancelamento (open, bot paused)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, is_bot_paused, last_customer_message_at, waiting_since, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000003', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511955551234', 'Carlos Lima', 'open', true, NOW() - INTERVAL '23 hours', NOW() - INTERVAL '23 hours',
            NOW() - INTERVAL '1 day', NOW() - INTERVAL '23 hours');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000009', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000003', 'tg_5511955551234', 'Carlos Lima', 'text', 'Preciso cancelar minha consulta de amanha', 'inbound', NOW() - INTERVAL '1 day'),
    ('01DEMO_MSG_00000000010', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000003', 'bot', 'Assistente Saude+', 'text', 'Entendo, Carlos. Sem problemas! Para cancelar ou reagendar, preciso confirmar: voce tem consulta agendada para amanha com qual profissional? Assim consigo localizar no sistema.', 'outbound', NOW() - INTERVAL '1 day' + INTERVAL '3 seconds'),
    ('01DEMO_MSG_00000000011', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000003', 'tg_5511955551234', 'Carlos Lima', 'text', 'Com a Dra. Silva, as 10h. Estou com um problema pessoal e preciso cancelar.', 'inbound', NOW() - INTERVAL '23 hours'),
    ('01DEMO_MSG_00000000012', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000003', 'bot', 'Assistente Saude+', 'text', 'Compreendo perfeitamente. Vou encaminhar para nossa recepcao confirmar o cancelamento. Deseja reagendar para outro dia?', 'outbound', NOW() - INTERVAL '23 hours' + INTERVAL '3 seconds');

    -- Conv 4: Fernanda Souza - Convenio (open)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000004', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511944443333', 'Fernanda Souza', 'open', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000013', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000004', 'tg_5511944443333', 'Fernanda Souza', 'text', 'Oi! Voces aceitam convenio Porto Seguro?', 'inbound', NOW() - INTERVAL '3 hours'),
    ('01DEMO_MSG_00000000014', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000004', 'bot', 'Assistente Saude+', 'text', 'Ola Fernanda! Sim, aceitamos o convenio Porto Seguro! Nossos convenios aceitos sao: Amil, Bradesco Dental, SulAmerica, Porto Seguro e Unimed Odonto. Gostaria de agendar uma consulta?', 'outbound', NOW() - INTERVAL '3 hours' + INTERVAL '4 seconds');

    -- Conv 5: Roberto Alves - Horarios (closed)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000005', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511933332222', 'Roberto Alves', 'closed', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000015', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000005', 'tg_5511933332222', 'Roberto Alves', 'text', 'Qual o horario de funcionamento?', 'inbound', NOW() - INTERVAL '2 days'),
    ('01DEMO_MSG_00000000016', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000005', 'bot', 'Assistente Saude+', 'text', 'Nosso horario de funcionamento e: Segunda a Sexta, das 8h as 18h, e Sabado das 8h as 12h. Nao abrimos aos domingos e feriados. Para emergencias fora do horario, temos um plantao no numero (11) 99999-0000. Posso ajudar com mais alguma coisa?', 'outbound', NOW() - INTERVAL '2 days' + INTERVAL '3 seconds'),
    ('01DEMO_MSG_00000000017', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000005', 'tg_5511933332222', 'Roberto Alves', 'text', 'Obrigado, era so isso!', 'inbound', NOW() - INTERVAL '2 days' + INTERVAL '5 minutes');

    -- Conv 6: Patricia Santos - Urgencia (open, bot paused)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, is_bot_paused, last_customer_message_at, waiting_since, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000006', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511922221111', 'Patricia Santos', 'open', true, NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes',
            NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '30 minutes');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000018', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000006', 'tg_5511922221111', 'Patricia Santos', 'text', 'Socorro! Estou com muita dor de dente, preciso de atendimento urgente!', 'inbound', NOW() - INTERVAL '45 minutes'),
    ('01DEMO_MSG_00000000019', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000006', 'bot', 'Assistente Saude+', 'text', 'Sinto muito que voce esteja com dor, Patricia! Vou encaminhar seu caso imediatamente para nosso atendimento prioritario. Enquanto isso, se possivel, aplique gelo na regiao externa por 10 minutos. Evite alimentos muito quentes ou frios. Um de nossos profissionais vai entrar em contato em breve.', 'outbound', NOW() - INTERVAL '45 minutes' + INTERVAL '3 seconds'),
    ('01DEMO_MSG_00000000020', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000006', 'tg_5511922221111', 'Patricia Santos', 'text', 'Obrigada! A dor e muito forte, comecou ontem a noite', 'inbound', NOW() - INTERVAL '40 minutes'),
    ('01DEMO_MSG_00000000021', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000006', 'bot', 'Assistente Saude+', 'text', 'Entendo, isso pode indicar uma inflamacao. Ja encaminhei para atendimento humano prioritario. Enquanto aguarda, evite mastigar do lado da dor. Nossa equipe vai retornar o mais rapido possivel.', 'outbound', NOW() - INTERVAL '40 minutes' + INTERVAL '4 seconds'),
    ('01DEMO_MSG_00000000022', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000006', 'tg_5511922221111', 'Patricia Santos', 'text', 'Ta bom, vou aguardar. Obrigada pela atencao!', 'inbound', NOW() - INTERVAL '30 minutes');

    -- Conv 7: Marcos Oliveira - Orcamento implante (open)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000007', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511911110000', 'Marcos Oliveira', 'open', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000023', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000007', 'tg_5511911110000', 'Marcos Oliveira', 'text', 'Quanto custa um implante dentario?', 'inbound', NOW() - INTERVAL '6 hours'),
    ('01DEMO_MSG_00000000024', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000007', 'bot', 'Assistente Saude+', 'text', 'Ola Marcos! O valor do implante dentario varia conforme cada caso. Em media, um implante unitario na Clinica Saude+ fica entre R$ 2.500 e R$ 4.500, incluindo a coroa protetica. O valor exato depende de uma avaliacao presencial. Gostaria de agendar?', 'outbound', NOW() - INTERVAL '6 hours' + INTERVAL '5 seconds'),
    ('01DEMO_MSG_00000000025', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000007', 'tg_5511911110000', 'Marcos Oliveira', 'text', 'Voces parcelam?', 'inbound', NOW() - INTERVAL '6 hours' + INTERVAL '10 minutes');

    -- Conv 8: Juliana Ferreira - Retorno pos-canal (closed)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000008', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511900009999', 'Juliana Ferreira', 'closed', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000026', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000008', 'tg_5511900009999', 'Juliana Ferreira', 'text', 'Ola! Fiz um tratamento de canal semana passada e queria saber se o incomodo que estou sentindo e normal', 'inbound', NOW() - INTERVAL '3 days'),
    ('01DEMO_MSG_00000000027', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000008', 'bot', 'Assistente Saude+', 'text', 'Ola Juliana! E normal sentir algum desconforto leve nos primeiros dias apos o tratamento de canal. Se a dor for suportavel e estiver diminuindo, nao se preocupe. Porem, se houver inchaco, dor intensa ou febre, entre em contato imediatamente. Recomendo uma consulta de retorno se o desconforto persistir por mais de 5 dias. Posso agendar um retorno para voce?', 'outbound', NOW() - INTERVAL '3 days' + INTERVAL '4 seconds'),
    ('01DEMO_MSG_00000000028', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000008', 'tg_5511900009999', 'Juliana Ferreira', 'text', 'Esta diminuindo sim, obrigada! Se piorar eu volto a falar', 'inbound', NOW() - INTERVAL '3 days' + INTERVAL '10 minutes'),
    ('01DEMO_MSG_00000000029', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000008', 'bot', 'Assistente Saude+', 'text', 'Otimo, que bom! Estamos aqui caso precise. Melhoras!', 'outbound', NOW() - INTERVAL '3 days' + INTERVAL '10 minutes' + INTERVAL '3 seconds');

    -- Conv 9: Lucas Mendes - Clareamento (open)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000009', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511988887777', 'Lucas Mendes', 'open', NOW() - INTERVAL '4 hours', NOW() - INTERVAL '4 hours', NOW() - INTERVAL '4 hours');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000030', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000009', 'tg_5511988887777', 'Lucas Mendes', 'text', 'Quero fazer clareamento dental. Como funciona?', 'inbound', NOW() - INTERVAL '4 hours'),
    ('01DEMO_MSG_00000000031', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000009', 'bot', 'Assistente Saude+', 'text', 'Ola Lucas! O clareamento dental e um dos procedimentos mais procurados na nossa clinica! Oferecemos duas opcoes: 1) Clareamento a laser (1 sessao de 1h, R$ 800) e 2) Clareamento com moldeira para uso em casa (2 semanas, R$ 500). Os resultados sao otimos em ambos os casos! Gostaria de saber mais sobre alguma das opcoes?', 'outbound', NOW() - INTERVAL '4 hours' + INTERVAL '5 seconds'),
    ('01DEMO_MSG_00000000032', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000009', 'tg_5511988887777', 'Lucas Mendes', 'text', 'Quanto custa o clareamento a laser?', 'inbound', NOW() - INTERVAL '4 hours' + INTERVAL '15 minutes');

    -- Conv 10: Camila Rodrigues - Ortodontia infantil (open)
    INSERT INTO conversations (id, account_id, channel_id, agent_id, contact_id, contact_name, status, last_customer_message_at, created_at, updated_at)
    VALUES ('01DEMO_CONV_0000000010', '01DEMO_ACCOUNT_00000000', '01DEMO_CHANNEL_0000000', '01DEMO_AGENT_000000000',
            'tg_5511977776666', 'Camila Rodrigues', 'open', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour 30 minutes', NOW() - INTERVAL '1 hour');

    INSERT INTO messages (id, channel_id, conversation_id, sender, sender_name, type, content, direction, created_at) VALUES
    ('01DEMO_MSG_00000000033', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000010', 'tg_5511977776666', 'Camila Rodrigues', 'text', 'Boa tarde! Meu filho de 8 anos precisa de aparelho. Voces atendem criancas?', 'inbound', NOW() - INTERVAL '1 hour 30 minutes'),
    ('01DEMO_MSG_00000000034', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000010', 'bot', 'Assistente Saude+', 'text', 'Boa tarde, Camila! Sim, atendemos criancas a partir de 6 anos! Temos ortodontistas especializados em ortodontia infantil. A avaliacao inicial e super importante nessa idade para verificar o desenvolvimento da arcada dentaria. Gostaria de agendar?', 'outbound', NOW() - INTERVAL '1 hour 30 minutes' + INTERVAL '4 seconds'),
    ('01DEMO_MSG_00000000035', '01DEMO_CHANNEL_0000000', '01DEMO_CONV_0000000010', 'tg_5511977776666', 'Camila Rodrigues', 'text', 'Sim! Tem horario na proxima semana?', 'inbound', NOW() - INTERVAL '1 hour');

  END IF;
END $$;
