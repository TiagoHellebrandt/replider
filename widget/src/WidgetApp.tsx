import { useEffect, useRef, useState } from 'preact/hooks';
import type { ApiClient, BootstrapConfig, Message } from './api';
import type { WidgetStorage } from './storage';
import { mergeMessages } from './merge';

interface WidgetProps {
  api: ApiClient;
  storage: WidgetStorage;
  pollIntervalMs?: number;
}

// Top-level widget: floating launcher bubble + expandable chat panel.
export function Widget({ api, storage, pollIntervalMs = 3000 }: WidgetProps) {
  const [open, setOpen] = useState(false);
  const [bootstrap, setBootstrap] = useState<BootstrapConfig | null>(null);
  const [bootstrapError, setBootstrapError] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // CSAT flow. The prompt becomes visible after the visitor has sent at
  // least one message AND received a bot reply — any earlier and the
  // visitor hasn't experienced enough to rate anything. `csatDone` is
  // persisted so refreshing the page doesn't re-prompt.
  const [csatRating, setCsatRating] = useState<number | null>(null);
  const [csatDone, setCsatDone] = useState<boolean>(() => storage.getCsatDone());
  const [csatSubmitting, setCsatSubmitting] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  const visitorId = storage.getVisitorId();

  // One-shot bootstrap (channel config from the server).
  useEffect(() => {
    let cancelled = false;
    api.bootstrap()
      .then((cfg) => !cancelled && setBootstrap(cfg))
      .catch((err: Error) => {
        if (!cancelled) setBootstrapError(err.message);
      });
    return () => {
      cancelled = true;
    };
  }, [api]);

  // Polling loop (only when the panel is open, to avoid waking visitors who never open it).
  useEffect(() => {
    if (!open) return;
    let active = true;

    const tick = async () => {
      if (!active) return;
      try {
        const cursor = storage.getCursor();
        const res = await api.poll(visitorId, cursor);
        if (!active) return;
        if (res.messages.length > 0) {
          setMessages((prev) => mergeMessages(prev, res.messages));
          const latest = res.messages[res.messages.length - 1];
          storage.setCursor(latest.created_at);
        } else {
          // No new messages — still advance cursor to server's "now" to avoid
          // re-scanning the same range forever.
          storage.setCursor(res.now);
        }
      } catch {
        // Silent: polling errors shouldn't spam the UI. Connectivity blips
        // are expected; the next tick retries. We keep the old messages.
      }
    };

    void tick();
    const id = globalThis.setInterval(tick, pollIntervalMs);
    return () => {
      active = false;
      globalThis.clearInterval(id);
    };
  }, [open, api, storage, visitorId, pollIntervalMs]);

  // Auto-scroll to latest message.
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const send = async (e?: Event) => {
    e?.preventDefault();
    const text = input.trim();
    if (!text || sending) return;
    setSending(true);
    setError(null);
    const optimistic: Message = {
      id: `pending-${Date.now()}`,
      direction: 'inbound',
      text,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, optimistic]);
    setInput('');
    try {
      await api.send(visitorId, text);
      // The server-side version of the message (with final id) will arrive
      // on the next poll; we don't need to do anything else here.
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'falha ao enviar';
      setError(msg);
      // Revert optimistic message
      setMessages((prev) => prev.filter((m) => m.id !== optimistic.id));
    } finally {
      setSending(false);
    }
  };

  const primary = bootstrap?.primary_color || '#25D366';
  const title = bootstrap?.title || 'Atendimento';

  if (bootstrapError) {
    // Fail silently — don't pollute the host page with a broken widget.
    // The host will see an empty bubble that, when clicked, reveals the error.
    return (
      <Launcher color={primary} open={open} onClick={() => setOpen((o) => !o)}>
        {open && (
          <div class="rw-panel" data-testid="rw-panel">
            <Header title="Widget indisponível" primary={primary} onClose={() => setOpen(false)} />
            <div class="rw-error-inline">
              Não foi possível carregar o atendimento. Tente novamente mais tarde.
            </div>
          </div>
        )}
      </Launcher>
    );
  }

  return (
    <Launcher color={primary} open={open} onClick={() => setOpen((o) => !o)}>
      {open && (
        <div class="rw-panel" data-testid="rw-panel" role="dialog" aria-label={title}>
          <Header title={title} primary={primary} onClose={() => setOpen(false)} />
          <div class="rw-messages" data-testid="rw-messages">
            {messages.length === 0 && bootstrap?.welcome_message && (
              <Bubble
                direction="outbound"
                text={bootstrap.welcome_message}
                primary={primary}
              />
            )}
            {messages.map((m) => (
              <Bubble key={m.id} direction={m.direction} text={m.text} primary={primary} />
            ))}
            <div ref={messagesEndRef} />
          </div>
          {error && <div class="rw-error-inline">{error}</div>}
          {!csatDone && shouldShowCsatPrompt(messages) && (
            <CsatPrompt
              primary={primary}
              rating={csatRating}
              submitting={csatSubmitting}
              onRate={setCsatRating}
              onSubmit={async (feedback) => {
                if (csatRating == null) return;
                setCsatSubmitting(true);
                try {
                  await api.recordCsat(visitorId, csatRating, feedback);
                  storage.setCsatDone();
                  setCsatDone(true);
                } catch {
                  // Surface as inline error; the visitor can retry by
                  // tapping again. We don't block the conversation on
                  // a failed rating.
                  setError('Não foi possível enviar sua avaliação.');
                } finally {
                  setCsatSubmitting(false);
                }
              }}
              onDismiss={() => {
                // Dismissing counts as "don't ask again" this browser —
                // nagging a visitor costs more than a lower response rate.
                storage.setCsatDone();
                setCsatDone(true);
              }}
            />
          )}
          <form class="rw-input-row" onSubmit={send}>
            <input
              class="rw-input"
              type="text"
              placeholder="Digite sua mensagem..."
              value={input}
              onInput={(e) => setInput((e.currentTarget as HTMLInputElement).value)}
              disabled={sending}
              aria-label="Mensagem"
              maxLength={4000}
            />
            <button
              class="rw-send"
              type="submit"
              style={{ backgroundColor: primary }}
              disabled={sending || input.trim().length === 0}
              aria-label="Enviar"
            >
              {sending ? '...' : '➤'}
            </button>
          </form>
          {/* Free-tier accounts render a discreet attribution badge; paid
              plans turn it off via bootstrap.show_branding (US-172). */}
          {bootstrap?.show_branding && <PoweredBy />}
        </div>
      )}
    </Launcher>
  );
}

// Discreet "Powered by Replider" attribution shown on free-tier widgets.
// Links to the marketing site with a UTM tag so the distribution loop is
// measurable. rel=noopener keeps the host page safe from the new tab.
function PoweredBy() {
  return (
    <a
      class="rw-branding"
      href="https://replider.com?utm_source=embed"
      target="_blank"
      rel="noopener noreferrer"
      data-testid="rw-branding"
    >
      Powered by <strong>Replider</strong>
    </a>
  );
}

// CSAT only makes sense AFTER the visitor experienced the bot replying.
// Require at least one inbound message (server-side "inbound" direction
// is the bot/human operator replying to the visitor) and show the prompt
// at a natural pause, i.e. when the last message wasn't the visitor's
// own typing. Keeps the prompt from interrupting a live question.
function shouldShowCsatPrompt(messages: Message[]): boolean {
  if (messages.length < 2) return false;
  const last = messages[messages.length - 1];
  return last.direction === 'inbound';
}

function CsatPrompt({
  primary,
  rating,
  submitting,
  onRate,
  onSubmit,
  onDismiss,
}: {
  primary: string;
  rating: number | null;
  submitting: boolean;
  onRate: (r: number) => void;
  onSubmit: (feedback?: string) => void;
  onDismiss: () => void;
}) {
  const [feedback, setFeedback] = useState('');
  return (
    <div class="rw-csat" role="region" aria-label="Avalie o atendimento">
      <div class="rw-csat-title">Como foi o atendimento?</div>
      <div class="rw-csat-stars">
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            class="rw-csat-star"
            aria-label={`${n} estrela${n > 1 ? 's' : ''}`}
            onClick={() => onRate(n)}
            style={{ color: rating != null && n <= rating ? primary : '#bbb' }}
            disabled={submitting}
          >
            ★
          </button>
        ))}
      </div>
      {rating != null ? (
        <div>
          <textarea
            class="rw-csat-feedback"
            placeholder="Conte o que achou (opcional)"
            maxLength={500}
            value={feedback}
            onInput={(e) => setFeedback((e.currentTarget as HTMLTextAreaElement).value)}
            disabled={submitting}
          />
          <div class="rw-csat-actions">
            <button type="button" class="rw-csat-skip" onClick={onDismiss} disabled={submitting}>
              Dispensar
            </button>
            <button
              type="button"
              class="rw-csat-submit"
              style={{ backgroundColor: primary }}
              onClick={() => onSubmit(feedback.trim() || undefined)}
              disabled={submitting}
            >
              {submitting ? 'Enviando...' : 'Enviar'}
            </button>
          </div>
        </div>
      ) : (
        <button type="button" class="rw-csat-skip-inline" onClick={onDismiss}>
          Não avaliar agora
        </button>
      )}
    </div>
  );
}

function Launcher({
  color,
  open,
  onClick,
  children,
}: {
  color: string;
  open: boolean;
  onClick: () => void;
  children?: preact.ComponentChildren;
}) {
  return (
    <div class="rw-root">
      {children}
      <button
        class="rw-bubble"
        style={{ backgroundColor: color }}
        onClick={onClick}
        aria-label={open ? 'Fechar atendimento' : 'Abrir atendimento'}
        data-testid="rw-bubble"
      >
        {open ? '×' : '💬'}
      </button>
    </div>
  );
}

function Header({
  title,
  primary,
  onClose,
}: {
  title: string;
  primary: string;
  onClose: () => void;
}) {
  return (
    <div class="rw-header" style={{ backgroundColor: primary }}>
      <span class="rw-title">{title}</span>
      <button class="rw-close" onClick={onClose} aria-label="Fechar">
        ×
      </button>
    </div>
  );
}

function Bubble({
  direction,
  text,
  primary,
}: {
  direction: 'inbound' | 'outbound';
  text: string;
  primary: string;
}) {
  // inbound = sent by the visitor (shows on the right, colored)
  // outbound = sent by the agent (shows on the left, light gray)
  const isMine = direction === 'inbound';
  const style = isMine ? { backgroundColor: primary, color: '#fff' } : undefined;
  return (
    <div class={`rw-bubble-msg ${isMine ? 'rw-mine' : 'rw-theirs'}`} style={style}>
      {text}
    </div>
  );
}

