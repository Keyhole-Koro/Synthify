'use client';

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { InlineError } from '@/components/error/InlineError';
import {
  listChatMessages,
  listChatConversations,
  sendChatMessage,
  WorkspaceChatMessageStatus,
  type WorkspaceChatMessage,
} from './api';

// 設計 §4 と揃える。サーバー側が正本で、ここは送信前に気づかせるためだけの複製。
const MAX_MESSAGE_LENGTH = 8000;

interface WorkspaceChatPanelProps {
  workspaceId: string;
}

export function WorkspaceChatPanel({ workspaceId }: WorkspaceChatPanelProps) {
  const [messages, setMessages] = useState<WorkspaceChatMessage[]>([]);
  const [conversationId, setConversationId] = useState('');
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  // 出典になりうるもの (処理済み資料 or ツリーの paper) があるか。
  // 入力を塞ぐためではなく、空のときに一言添えるためだけに使う。
  const [hasAnswerableSources, setHasAnswerableSources] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);

  // 直近の会話を復元する。v1 は会話一覧 UI を持たないので、最新の1件だけ開く。
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const { conversations, hasAnswerableSources: answerable } =
          await listChatConversations(workspaceId);
        if (cancelled) return;
        setHasAnswerableSources(answerable);
        if (conversations.length === 0) return;
        const latest = conversations[0];
        const history = await listChatMessages(workspaceId, latest.conversationId);
        if (cancelled) return;
        setConversationId(latest.conversationId);
        setMessages(history);
      } catch {
        // 履歴の復元失敗で入力まで止めない。送信時のエラーは別途出す。
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [workspaceId]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const text = draft;
      if (!text.trim() || sending) return;

      setSending(true);
      setError(null);
      try {
        const result = await sendChatMessage(workspaceId, conversationId, text);
        setConversationId(result.conversationId);
        setMessages((prev) => [...prev, result.userMessage, result.assistantMessage]);
        setDraft('');
      } catch (err) {
        const message = err instanceof Error ? err.message : '送信に失敗しました。';
        setError(message);
      } finally {
        setSending(false);
      }
    },
    [draft, sending, workspaceId, conversationId],
  );

  const overLimit = draft.length > MAX_MESSAGE_LENGTH;
  const disabled = sending;

  return (
    <section className="border-t border-stone-200 px-5 pt-4 pb-5" data-testid="workspace-chat">
      <h3 className="mb-2 text-[12px] font-medium text-stone-500">このワークスペースに質問</h3>

      {loading ? (
        <p className="text-[12px] text-stone-400">読み込み中…</p>
      ) : (
        <>
          <div
            ref={listRef}
            className="mb-3 flex max-h-72 flex-col gap-3 overflow-y-auto"
            data-testid="workspace-chat-messages"
          >
            {messages.length === 0 && (
              <p className="text-[12px] text-stone-400" data-testid="workspace-chat-hint">
                {hasAnswerableSources
                  ? '例：この資料の結論は？ / ワークスペースの権限について'
                  : 'まだ資料もページもありません。そのまま質問できますが、回答はこのワークスペースの内容に基づきません。'}
              </p>
            )}
            {messages.map((message) => (
              <ChatBubble key={message.messageId} message={message} />
            ))}
            {sending && (
              <p className="text-[12px] text-indigo-500" data-testid="workspace-chat-pending">
                回答を生成中…
              </p>
            )}
          </div>

          {error && <InlineError message={error} className="mb-2" />}

          <form onSubmit={handleSubmit} className="flex items-end gap-2">
            <textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                // Enter で送信、Shift+Enter で改行。
                if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  void handleSubmit(e);
                }
              }}
              rows={2}
              disabled={disabled}
              placeholder="このワークスペースについて質問…"
              aria-label="このワークスペースについて質問"
              data-testid="workspace-chat-input"
              className="min-h-[38px] flex-1 resize-y rounded-md border border-stone-200 px-2.5 py-1.5 text-[12px] text-stone-700 placeholder:text-stone-300 focus:border-indigo-300 focus:outline-none disabled:bg-stone-50"
            />
            <button
              type="submit"
              disabled={disabled || !draft.trim() || overLimit}
              data-testid="workspace-chat-send"
              className="h-[38px] shrink-0 rounded-md border border-indigo-200 bg-indigo-50 px-3 text-[12px] font-medium text-indigo-600 disabled:opacity-50"
            >
              {sending ? '送信中…' : '送信'}
            </button>
          </form>
          {overLimit && (
            <InlineError
              message={`質問が長すぎます (${draft.length} / ${MAX_MESSAGE_LENGTH} 文字)`}
              className="mt-1.5"
            />
          )}
        </>
      )}
    </section>
  );
}

function ChatBubble({ message }: { message: WorkspaceChatMessage }) {
  const isUser = message.role === 'user';
  const isFailed = message.status === WorkspaceChatMessageStatus.FAILED;

  if (isFailed) {
    return (
      <div data-testid="workspace-chat-message-failed">
        <InlineError message="回答の生成に失敗しました。もう一度お試しください。" />
      </div>
    );
  }

  return (
    <div
      className={isUser ? 'self-end max-w-[85%]' : 'self-start max-w-[95%]'}
      data-testid={isUser ? 'workspace-chat-message-user' : 'workspace-chat-message-assistant'}
    >
      <div
        className={[
          'whitespace-pre-wrap rounded-md px-2.5 py-2 text-[12px] leading-relaxed',
          isUser ? 'bg-indigo-50 text-indigo-900' : 'bg-stone-50 text-stone-700',
        ].join(' ')}
      >
        {message.content}
      </div>

      {!isUser && !message.grounded && (
        <p
          className="mt-1.5 text-[11px] text-amber-600"
          data-testid="workspace-chat-ungrounded"
        >
          このワークスペースの資料には基づかない回答です
        </p>
      )}

      {message.sources.length > 0 && (
        <ul className="mt-1.5 flex flex-wrap gap-1.5" data-testid="workspace-chat-sources">
          {message.sources.map((source) => (
            <li
              key={`${source.documentId}:${source.chunkId}:${source.label}`}
              className="rounded border border-stone-200 bg-white px-1.5 py-0.5 text-[11px] text-stone-500"
            >
              {source.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
