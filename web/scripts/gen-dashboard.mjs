// Generates a high-fidelity Replider dashboard image for the Solution section.
// Renders an on-brand SVG (colors mirror app/lib/design_system/app_theme.dart:
// bg #0A0A0A / #0F0F0F, blue accent #2563EB / #3B82F6, WhatsApp green #25D366)
// and rasterizes it to webp via sharp. Run from inside web/: `node scripts/gen-dashboard.mjs`.
import sharp from "sharp";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const OUT = join(__dirname, "..", "src", "assets", "dashboard.webp");

const W = 1280;
const H = 960;

const esc = (s) => String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

// A single conversation row in the inbox list.
function convRow(y, name, preview, time, badge, active, unread) {
  const rowBg = active
    ? `<rect x="232" y="${y}" width="392" height="76" rx="10" fill="#2563EB" opacity="0.14"/>` +
      `<rect x="232" y="${y}" width="3" height="76" rx="2" fill="#3B82F6"/>`
    : "";
  const badgeColor = badge === "wa" ? "#25D366" : "#229ED9";
  const badgeGlyph =
    badge === "wa"
      ? `<path d="M0,-6 a6,6 0 1,0 0.01,0 M-2.4,2.6 l-0.9,2.8 2.9,-0.85" fill="none" stroke="#0A0A0A" stroke-width="1.4"/>`
      : `<path d="M-5,0 L5,-4 L3.2,5 L0.6,1.8 Z" fill="#0A0A0A"/>`;
  const dot = unread
    ? `<circle cx="610" cy="${y + 52}" r="9" fill="#2563EB"/><text x="610" y="${y + 55}" font-size="11" fill="#fff" text-anchor="middle" font-weight="700">${unread}</text>`
    : "";
  return `
    ${rowBg}
    <circle cx="262" cy="${y + 38}" r="20" fill="url(#avatar${y})"/>
    <text x="262" y="${y + 43}" font-size="15" fill="#fff" text-anchor="middle" font-weight="700">${esc(
      name[0],
    )}</text>
    <g transform="translate(278,${y + 53})"><circle r="9" fill="${badgeColor}"/>${badgeGlyph}</g>
    <text x="298" y="${y + 32}" font-size="15" fill="#F5F5F5" font-weight="600">${esc(name)}</text>
    <text x="298" y="${y + 54}" font-size="13" fill="#8A8A8A">${esc(preview)}</text>
    <text x="610" y="${y + 32}" font-size="11" fill="#6A6A6A" text-anchor="end">${esc(time)}</text>
    ${dot}
  `;
}

// A chat bubble in the conversation detail pane.
function bubble(y, lines, fromAI) {
  const x = fromAI ? 980 : 700;
  const fill = fromAI ? "#2563EB" : "#1A1A1A";
  const anchor = fromAI ? "end" : "start";
  const tx = fromAI ? 948 : 718;
  const w = 268;
  const bx = fromAI ? 696 : 700;
  const h = 22 + lines.length * 20;
  const text = lines
    .map(
      (l, i) =>
        `<text x="${tx}" y="${y + 26 + i * 20}" font-size="13.5" fill="${
          fromAI ? "#EAF1FF" : "#D8D8D8"
        }" text-anchor="${anchor}">${esc(l)}</text>`,
    )
    .join("");
  return `<rect x="${bx}" y="${y}" width="${w}" height="${h}" rx="14" fill="${fill}"/>${text}`;
}

const avatarGradients = [340, 416, 492, 568]
  .map(
    (y, i) =>
      `<linearGradient id="avatar${y}" x1="0" y1="0" x2="1" y2="1">
         <stop offset="0" stop-color="${["#4F46E5", "#2563EB", "#06B6D4", "#F97316"][i]}"/>
         <stop offset="1" stop-color="${["#7C3AED", "#3B82F6", "#0EA5E9", "#FB923C"][i]}"/>
       </linearGradient>`,
  )
  .join("");

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" font-family="Inter, 'Segoe UI', system-ui, sans-serif">
  <defs>
    ${avatarGradients}
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#0B0B0D"/><stop offset="1" stop-color="#08080A"/>
    </linearGradient>
    <linearGradient id="logo" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#3B82F6"/><stop offset="1" stop-color="#2563EB"/>
    </linearGradient>
  </defs>

  <rect width="${W}" height="${H}" fill="url(#bg)"/>

  <!-- Left nav rail -->
  <rect x="0" y="0" width="72" height="${H}" fill="#0F0F0F"/>
  <rect x="0" y="0" width="72" height="${H}" fill="none" stroke="#1C1C1C"/>
  <rect x="20" y="24" width="32" height="32" rx="9" fill="url(#logo)"/>
  <text x="36" y="46" font-size="18" fill="#fff" text-anchor="middle" font-weight="800">R</text>
  <rect x="14" y="96" width="44" height="44" rx="12" fill="#2563EB" opacity="0.18"/>
  <rect x="22" y="108" width="20" height="20" rx="5" fill="none" stroke="#3B82F6" stroke-width="2"/>
  ${[160, 216, 272]
    .map(
      (y) =>
        `<rect x="22" y="${y}" width="28" height="20" rx="5" fill="none" stroke="#4A4A4A" stroke-width="2"/>`,
    )
    .join("")}
  <circle cx="36" cy="${H - 40}" r="16" fill="#1F2937"/>
  <text x="36" y="${H - 35}" font-size="12" fill="#9CA3AF" text-anchor="middle" font-weight="700">TS</text>

  <!-- Inbox column header -->
  <rect x="72" y="0" width="560" height="${H}" fill="#0C0C0E"/>
  <line x1="632" y1="0" x2="632" y2="${H}" stroke="#1C1C1C"/>
  <text x="104" y="56" font-size="22" fill="#FFFFFF" font-weight="800">Conversas</text>
  <rect x="556" y="36" width="48" height="26" rx="13" fill="#16331F"/>
  <circle cx="572" cy="49" r="4" fill="#25D366"/>
  <text x="586" y="53" font-size="11" fill="#25D366" text-anchor="middle" font-weight="700">12</text>

  <!-- Search -->
  <rect x="104" y="80" width="500" height="40" rx="10" fill="#141416" stroke="#222"/>
  <circle cx="128" cy="100" r="6" fill="none" stroke="#666" stroke-width="2"/>
  <line x1="132" y1="104" x2="137" y2="109" stroke="#666" stroke-width="2"/>
  <text x="150" y="105" font-size="13.5" fill="#666">Buscar conversa...</text>

  <!-- Filter pills -->
  <rect x="104" y="136" width="78" height="30" rx="15" fill="#2563EB"/>
  <text x="143" y="156" font-size="12.5" fill="#fff" text-anchor="middle" font-weight="600">Todas</text>
  <rect x="190" y="136" width="118" height="30" rx="15" fill="#141416" stroke="#222"/>
  <text x="249" y="156" font-size="12.5" fill="#A8A8A8" text-anchor="middle">Aguardando</text>
  <rect x="316" y="136" width="96" height="30" rx="15" fill="#141416" stroke="#222"/>
  <text x="364" y="156" font-size="12.5" fill="#A8A8A8" text-anchor="middle">Resolvidas</text>

  ${convRow(192, "Mariana Costa", "Perfeito, qual o horário?", "09:42", "wa", false, "2")}
  ${convRow(272, "João Pedro", "Bot: Agendei sua consulta ✓", "09:38", "wa", true, "")}
  ${convRow(352, "Loja Aurora", "Quero saber sobre o plano...", "09:31", "tg", false, "")}
  ${convRow(432, "Carlos Mendes", "Obrigado pelo atendimento!", "08:57", "wa", false, "")}
  ${convRow(512, "Ana Beatriz", "Bot: Posso ajudar em algo?", "08:40", "tg", false, "")}

  <!-- Conversation detail header -->
  <rect x="632" y="0" width="${W - 632}" height="84" fill="#0C0C0E"/>
  <line x1="632" y1="84" x2="${W}" y2="84" stroke="#1C1C1C"/>
  <circle cx="676" cy="42" r="20" fill="url(#avatar340)"/>
  <text x="676" y="47" font-size="15" fill="#fff" text-anchor="middle" font-weight="700">J</text>
  <text x="708" y="36" font-size="16" fill="#FFFFFF" font-weight="700">João Pedro</text>
  <text x="708" y="56" font-size="12.5" fill="#25D366">● WhatsApp · online</text>
  <rect x="1090" y="26" width="150" height="34" rx="17" fill="#2563EB" opacity="0.16" stroke="#2563EB"/>
  <circle cx="1112" cy="43" r="6" fill="#3B82F6"/>
  <text x="1126" y="47" font-size="12.5" fill="#93B4FF" font-weight="600">IA ativa</text>

  <!-- Conversation thread -->
  ${bubble(120, ["Oi! Gostaria de marcar", "uma consulta para amanhã"], false)}
  <text x="700" y="186" font-size="10.5" fill="#5A5A5A">09:35</text>

  ${bubble(210, ["Claro! Temos horários às", "14h, 15h e 16h. Qual prefere?"], true)}
  <g transform="translate(1224,212)"><circle r="9" fill="#2563EB"/><text y="3.5" font-size="9" fill="#fff" text-anchor="middle" font-weight="800">AI</text></g>
  <text x="960" y="262" font-size="10.5" fill="#5A5A5A" text-anchor="end">09:36 · IA</text>

  ${bubble(296, ["Pode ser às 15h"], false)}
  <text x="700" y="342" font-size="10.5" fill="#5A5A5A">09:37</text>

  ${bubble(372, ["Agendado para amanhã às 15h ✓", "Enviei a confirmação no seu", "e-mail. Até lá!"], true)}
  <g transform="translate(1224,374)"><circle r="9" fill="#2563EB"/><text y="3.5" font-size="9" fill="#fff" text-anchor="middle" font-weight="800">AI</text></g>
  <text x="960" y="446" font-size="10.5" fill="#5A5A5A" text-anchor="end">09:38 · IA</text>

  <!-- AI suggestion / takeover bar -->
  <rect x="700" y="${H - 188}" width="540" height="64" rx="14" fill="#0F1A2E" stroke="#1E3A5F"/>
  <circle cx="730" cy="${H - 156}" r="11" fill="#2563EB"/>
  <text x="730" y="${H - 152}" font-size="10" fill="#fff" text-anchor="middle" font-weight="800">AI</text>
  <text x="752" y="${H - 162}" font-size="12.5" fill="#93B4FF" font-weight="600">Sugestão do agente</text>
  <text x="752" y="${H - 143}" font-size="12.5" fill="#B8C4D8">Confirmar lembrete 1h antes da consulta?</text>
  <rect x="1150" y="${H - 172}" width="74" height="32" rx="16" fill="#2563EB"/>
  <text x="1187" y="${H - 151}" font-size="12" fill="#fff" text-anchor="middle" font-weight="600">Enviar</text>

  <!-- Composer -->
  <rect x="700" y="${H - 104}" width="540" height="48" rx="24" fill="#141416" stroke="#222"/>
  <text x="724" y="${H - 74}" font-size="13.5" fill="#666">Assumir conversa e responder...</text>
  <circle cx="1212" cy="${H - 80}" r="18" fill="#2563EB"/>
  <path d="M1205,${H - 80} L1219,${H - 86} L1216,${H - 74} Z" fill="#fff"/>
</svg>`;

await sharp(Buffer.from(svg)).webp({ quality: 88 }).toFile(OUT);
console.log("wrote", OUT);
