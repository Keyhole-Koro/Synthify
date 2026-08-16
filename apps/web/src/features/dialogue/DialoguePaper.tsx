'use client';

import { useCallback, useMemo, useState } from 'react';
import {
  usePaperDispatch,
  usePaperStoreSelector,
  type ContentNode,
  type PaperId,
} from '@keyhole-koro/paper-in-paper';
import { postChatTurn, type ChatHistoryEntry } from './api';
import { collectCandidates } from './contextText';
import { useChatTurn } from './useChatTurn';

interface Props {
  /** The paper this dialogue is anchored to. */
  paperId: PaperId;
  workspaceId: string;
}

/**
 * A dialogue rendered inside the canvas.
 *
 * The answer is not returned by the trigger RPC — it arrives through the
 * Firestore turn document, one section at a time, so the paper fills in as it
 * is written rather than after the whole turn completes.
 */
export function DialoguePaper({ paperId, workspaceId }: Props) {
  const dispatch = usePaperDispatch();
  const paperMap = usePaperStoreSelector((s) => s.state.paperMap);

  const [history, setHistory] = useState<ChatHistoryEntry[]>([]);
  const [input, setInput] = useState('');
  const [chatId, setChatId] = useState<string | null>(null);
  const [turnId, setTurnId] = useState<string | null>(null);
  const [sendError, setSendError] = useState('');
  const [sending, setSending] = useState(false);

  const turn = useChatTurn(chatId, turnId);
  const candidates = useMemo(() => collectCandidates(paperMap, paperId), [paperMap, paperId]);

  const send = useCallback(async () => {
    const text = input.trim();
    if (!text || sending || turn.isRunning) return;

    const nextHistory: ChatHistoryEntry[] = [...history, { role: 'user', text }];
    setHistory(nextHistory);
    setInput('');
    setSendError('');
    setSending(true);
    try {
      const res = await postChatTurn({
        workspaceId,
        paperId,
        chatId: chatId ?? undefined,
        history: nextHistory,
        candidates,
        autoSelectContext: true,
      });
      setChatId(res.chatId);
      setTurnId(res.turnId);
    } catch (err) {
      setSendError(err instanceof Error ? err.message : String(err));
      // Put the question back so it is not lost to a failed dispatch.
      setHistory(history);
      setInput(text);
    } finally {
      setSending(false);
    }
  }, [candidates, chatId, history, input, paperId, sending, turn.isRunning, workspaceId]);

  const openPaper = useCallback(
    (target: PaperId) => {
      const node = paperMap.get(target);
      if (!node?.parentId) return;
      dispatch({ type: 'OPEN_NODE', parentId: node.parentId, childId: target });
      dispatch({ type: 'FOCUS_NODE', nodeId: target });
    },
    [dispatch, paperMap],
  );

  const busy = sending || turn.isRunning;

  return (
    <div style={styles.root}>
      <div style={styles.log}>
        {history.map((m, i) => (
          <div key={i} style={m.role === 'user' ? styles.userMsg : styles.assistantMsg}>
            {m.text}
          </div>
        ))}

        {turn.nodes.length > 0 && (
          <div style={styles.answer}>
            <Nodes nodes={turn.nodes} onOpen={openPaper} />
          </div>
        )}

        {turn.isRunning && (
          <p style={styles.hint}>
            {turn.sectionCount > 0
              ? `続きを書いています… (${turn.sectionCount})`
              : '考えています…'}
          </p>
        )}

        {turn.status === 'failed' && (
          <p style={styles.error}>生成に失敗しました: {turn.errorMessage || '不明なエラー'}</p>
        )}
        {sendError && <p style={styles.error}>送信に失敗しました: {sendError}</p>}

        {turn.selectedContextIds.length > 0 && (
          <div style={styles.chips}>
            {turn.selectedContextIds.map((id) => (
              <button key={id} type="button" style={styles.chip} onClick={() => openPaper(id)}>
                {paperMap.get(id)?.title ?? id}
              </button>
            ))}
          </div>
        )}
      </div>

      <form
        style={styles.form}
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        <input
          style={styles.input}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="この paper について質問する"
          disabled={busy}
          aria-label="質問"
        />
        <button type="submit" style={styles.send} disabled={busy || !input.trim()}>
          {busy ? '…' : '送信'}
        </button>
      </form>
    </div>
  );
}

/**
 * Renders the answer.
 *
 * paper-in-paper renders ContentNode[] natively when it is a paper's content,
 * but this dialogue is itself a React paper, so the nodes are rendered here.
 * Only the subset the worker's sanitizer can emit is handled; anything else is
 * skipped rather than rendered as a fallback, matching the worker's rule that
 * an unrenderable node is dropped.
 */
function Nodes({ nodes, onOpen }: { nodes: ContentNode[]; onOpen: (id: PaperId) => void }) {
  return (
    <>
      {nodes.map((node, i) => (
        <Node key={i} node={node} onOpen={onOpen} />
      ))}
    </>
  );
}

function Node({ node, onOpen }: { node: ContentNode; onOpen: (id: PaperId) => void }) {
  switch (node.type) {
    case 'text':
      return <span>{node.value}</span>;
    case 'paragraph':
      return (
        <p style={styles.paragraph}>
          <Nodes nodes={node.children} onOpen={onOpen} />
        </p>
      );
    case 'bold':
      return (
        <strong>
          <Nodes nodes={node.children} onOpen={onOpen} />
        </strong>
      );
    case 'section':
      return (
        <section style={styles.section}>
          {node.title && <h4 style={styles.sectionTitle}>{node.title}</h4>}
          <Nodes nodes={node.children} onOpen={onOpen} />
        </section>
      );
    case 'callout':
      return (
        <div style={styles.callout}>
          <Nodes nodes={node.children} onOpen={onOpen} />
        </div>
      );
    case 'list':
      return (
        <ul style={styles.list}>
          {node.items.map((item, i) => (
            <li key={i}>
              <Nodes nodes={item} onOpen={onOpen} />
            </li>
          ))}
        </ul>
      );
    case 'table':
      return (
        <div style={styles.tableWrap}>
          <table style={styles.table}>
            <thead>
              <tr>
                {node.headers.map((h, i) => (
                  <th key={i} style={styles.th}>
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {node.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => (
                    <td key={j} style={styles.td}>
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    case 'paper-link':
      return (
        <button type="button" style={styles.link} onClick={() => onOpen(node.paperId)}>
          {node.label}
        </button>
      );
    case 'card':
      return (
        <button type="button" style={styles.card} onClick={() => onOpen(node.paperId)}>
          <span style={styles.cardTitle}>{node.title}</span>
          <span style={styles.cardBody}>{node.description}</span>
        </button>
      );
    default:
      return null;
  }
}

const styles: Record<string, React.CSSProperties> = {
  root: { display: 'flex', flexDirection: 'column', height: '100%', gap: 8, fontSize: '0.85rem' },
  log: { flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 8, minHeight: 0 },
  userMsg: {
    alignSelf: 'flex-end',
    background: 'rgba(0,0,0,0.06)',
    borderRadius: 10,
    padding: '6px 10px',
    maxWidth: '85%',
  },
  assistantMsg: { alignSelf: 'flex-start', maxWidth: '85%' },
  answer: { lineHeight: 1.6 },
  paragraph: { margin: '0 0 8px' },
  section: { margin: '0 0 10px' },
  sectionTitle: { margin: '0 0 4px', fontSize: '0.9rem', fontWeight: 700 },
  callout: {
    borderLeft: '3px solid rgba(0,0,0,0.2)',
    padding: '4px 10px',
    margin: '0 0 8px',
    background: 'rgba(0,0,0,0.03)',
  },
  list: { margin: '0 0 8px', paddingLeft: 18 },
  tableWrap: { overflowX: 'auto', margin: '0 0 8px' },
  table: { borderCollapse: 'collapse', fontSize: '0.8rem' },
  th: { border: '1px solid rgba(0,0,0,0.15)', padding: '3px 6px', textAlign: 'left', fontWeight: 700 },
  td: { border: '1px solid rgba(0,0,0,0.15)', padding: '3px 6px' },
  link: {
    display: 'inline',
    border: '1px solid rgba(0,0,0,0.2)',
    borderRadius: 4,
    background: 'rgba(255,255,255,0.6)',
    padding: '1px 5px',
    cursor: 'pointer',
    font: 'inherit',
  },
  card: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    alignItems: 'flex-start',
    textAlign: 'left',
    border: '1px solid rgba(0,0,0,0.15)',
    borderRadius: 8,
    background: 'rgba(255,255,255,0.6)',
    padding: '8px 10px',
    margin: '0 0 8px',
    cursor: 'pointer',
    font: 'inherit',
    width: '100%',
  },
  cardTitle: { fontWeight: 700, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.06em' },
  cardBody: { fontSize: '0.78rem', lineHeight: 1.5 },
  chips: { display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 4 },
  chip: {
    border: '1px solid rgba(0,0,0,0.15)',
    borderRadius: 999,
    background: 'rgba(255,255,255,0.7)',
    padding: '2px 8px',
    fontSize: '0.72rem',
    cursor: 'pointer',
    font: 'inherit',
  },
  hint: { margin: 0, opacity: 0.65, fontStyle: 'italic' },
  error: { margin: 0, color: '#b3261e' },
  form: { display: 'flex', gap: 6 },
  input: {
    flex: 1,
    border: '1px solid rgba(0,0,0,0.2)',
    borderRadius: 8,
    padding: '6px 10px',
    font: 'inherit',
    minWidth: 0,
  },
  send: {
    border: '1px solid rgba(0,0,0,0.2)',
    borderRadius: 8,
    background: 'rgba(255,255,255,0.8)',
    padding: '6px 12px',
    cursor: 'pointer',
    font: 'inherit',
  },
};
