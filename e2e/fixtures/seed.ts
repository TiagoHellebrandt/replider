/**
 * Seed script for e2e tests.
 *
 * Seeds the database with demo data for the dental clinic scenario.
 * Can be run via: npx tsx fixtures/seed.ts
 *
 * Prerequisites:
 *   - API must be running at API_URL (default: http://localhost:8080/v1)
 *   - The replider_db Docker container must be accessible (for email verification)
 */

import { execSync } from 'child_process';
import { ApiClient } from '../helpers/api-client';

const DEMO_EMAIL = 'demo@replider.com';
const DEMO_PASSWORD = 'demo1234';
const DEMO_ACCOUNT_NAME = 'Clinica Saude+ Odontologia';
const DEMO_ADMIN_NAME = 'Dr. Maria Silva';

const DB_CONTAINER = process.env.DB_CONTAINER || 'replider_db';
const DB_USER = process.env.DB_USER || 'replider';
const DB_NAME = process.env.DB_NAME || 'replider_db';

const CLINIC_SYSTEM_PROMPT = `Voce e a assistente virtual da Clinica Saude+ Odontologia. Seu nome e Sofia.

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
5. Responda sempre em portugues`;

const FAQ_CHUNKS = [
  'Quanto custa uma limpeza dental? O valor da limpeza na Clinica Saude+ e de R$ 180,00. Para pacientes com plano odontologico, verificamos a cobertura. A limpeza inclui raspagem, polimento e aplicacao de fluor. Recomendamos fazer a cada 6 meses.',
  'Como funciona o clareamento dental? Oferecemos duas opcoes: clareamento a laser (1 sessao de 1h, R$ 800) e clareamento com moldeira (kit para uso em casa por 2 semanas, R$ 500). Ambos os metodos sao seguros e eficazes. E necessaria uma avaliacao previa.',
  'Quais formas de pagamento sao aceitas? Aceitamos dinheiro, PIX, cartao de debito e credito (parcelamos em ate 12x sem juros para valores acima de R$ 500). Tambem trabalhamos com os principais convenios odontologicos.',
  'Como agendar uma consulta? Voce pode agendar pelo WhatsApp, telefone (11) 3456-7890, ou diretamente comigo aqui no chat. Preciso do seu nome completo, telefone para contato e o procedimento desejado. Temos horarios disponiveis de segunda a sexta das 8h as 18h.',
  'Qual a idade minima para colocar aparelho ortodontico? A avaliacao ortodontica e recomendada a partir dos 7 anos. O tratamento com aparelho fixo geralmente inicia entre 11 e 14 anos, mas pode ser indicado em qualquer idade. Agende uma avaliacao com nosso ortodontista Dr. Carlos.',
];

async function waitForApi(api: ApiClient, retries = 15, delayMs = 3000): Promise<void> {
  for (let i = 1; i <= retries; i++) {
    const healthy = await api.healthCheck();
    if (healthy) return;
    console.log(`   Attempt ${i}/${retries}: API not ready, retrying in ${delayMs / 1000}s...`);
    await new Promise((r) => setTimeout(r, delayMs));
  }
  console.error('   API is not reachable after multiple attempts.');
  console.error('   Make sure the API is running: cd api && make docker-dev');
  process.exit(1);
}

function verifyEmailInDb(email: string): void {
  const sql = `UPDATE users SET is_email_verified = true WHERE email = '${email}';`;
  try {
    execSync(
      `docker exec ${DB_CONTAINER} psql -U ${DB_USER} -d ${DB_NAME} -c "${sql}"`,
      { stdio: 'pipe' }
    );
    console.log('   Email verified via DB.');
  } catch (err: any) {
    console.warn('   Could not auto-verify email via docker exec. Run manually:');
    console.warn(`   docker exec ${DB_CONTAINER} psql -U ${DB_USER} -d ${DB_NAME} -c "${sql}"`);
  }
}

async function seed() {
  console.log('=== Replider E2E Seed Script ===\n');

  const api = new ApiClient();

  // Step 1: Wait for API
  console.log('1. Waiting for API to be ready...');
  await waitForApi(api);
  console.log('   API is healthy.\n');

  // Step 2: Register demo account
  console.log('2. Registering demo account...');
  try {
    await api.register(DEMO_ACCOUNT_NAME, DEMO_ADMIN_NAME, DEMO_EMAIL, DEMO_PASSWORD);
    console.log('   Account registered successfully.');
  } catch (err: any) {
    if (err.message.includes('409') || err.message.includes('already exists') || err.message.includes('DUPLICATE')) {
      console.log('   Account already exists, skipping registration.');
    } else {
      console.error('   Registration failed:', err.message);
      process.exit(1);
    }
  }

  // Step 3: Verify email via docker exec (automatic)
  console.log('\n3. Verifying email...');
  verifyEmailInDb(DEMO_EMAIL);

  // Step 4: Login
  console.log('\n4. Logging in...');
  try {
    const token = await api.login(DEMO_EMAIL, DEMO_PASSWORD);
    console.log('   Logged in successfully. Token:', token.substring(0, 20) + '...\n');
  } catch (err: any) {
    console.error('   Login failed:', err.message);
    process.exit(1);
  }

  // Step 5: Create agent
  console.log('5. Creating AI agent...');
  try {
    const agent = await api.createAgent('Assistente Saude+', CLINIC_SYSTEM_PROMPT, 'google');
    console.log('   Agent created:', agent.id || agent.name);

    // Step 6: Create knowledge source + chunks
    console.log('\n6. Creating knowledge base...');
    try {
      const source = await api.createKnowledgeSource('FAQ Clinica', 'Perguntas frequentes da clinica odontologica');
      console.log('   Knowledge source created:', source.id || source.name);

      for (let i = 0; i < FAQ_CHUNKS.length; i++) {
        await api.addKnowledgeChunk(source.id, FAQ_CHUNKS[i]);
        console.log(`   Chunk ${i + 1}/${FAQ_CHUNKS.length} added.`);
      }
    } catch (err: any) {
      console.warn('   Knowledge base creation failed (may already exist):', err.message);
    }
  } catch (err: any) {
    if (err.message.includes('409') || err.message.includes('LIMIT')) {
      console.log('   Agent may already exist or limit reached, skipping.');
    } else {
      console.error('   Agent creation failed:', err.message);
    }
  }

  console.log('\n=== Seed complete ===');
  console.log(`\nDemo credentials:\n  Email: ${DEMO_EMAIL}\n  Password: ${DEMO_PASSWORD}`);
}

seed().catch((err) => {
  console.error('Seed failed:', err);
  process.exit(1);
});
