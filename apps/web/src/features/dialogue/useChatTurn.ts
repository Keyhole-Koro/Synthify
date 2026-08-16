'use client';

import { useMemo } from 'react';
import { doc } from 'firebase/firestore';
import type { ContentNode } from '@keyhole-koro/paper-in-paper';
import { db } from '@/lib/firebase';
import { useAuthedDoc } from '@/lib/firestore';
import { auth } from '@/lib/firebase';
import type { FirestoreChatTurn } from './firestore/firestoreChatTurn.generated';

export type { FirestoreChatTurn } from './firestore/firestoreChatTurn.generated';

export interface ChatTurnView {
  status: FirestoreChatTurn['status'] | null;
  /** Parsed content. Empty until the first section lands. */
  nodes: ContentNode[];
  sectionCount: number;
  selectedContextIds: string[];
  errorMessage: string;
  isRunning: boolean;
}

const EMPTY: ChatTurnView = {
  status: null,
  nodes: [],
  sectionCount: 0,
  selectedContextIds: [],
  errorMessage: '',
  isRunning: false,
};

/**
 * Subscribes to one turn document.
 *
 * The path is user-scoped rather than workspace-scoped, which is what lets the
 * security rules authorize on `request.auth.uid` alone. The uid therefore has
 * to come from the signed-in user here, not from a prop.
 */
export function useChatTurn(chatId: string | null, turnId: string | null): ChatTurnView {
  const uid = auth.currentUser?.uid ?? null;

  const ref = useMemo(
    () => (uid && chatId && turnId ? doc(db, 'users', uid, 'chats', chatId, 'turns', turnId) : null),
    [uid, chatId, turnId],
  );
  const { data } = useAuthedDoc<FirestoreChatTurn>(ref, 'chat_turn');

  return useMemo(() => {
    if (!data) return EMPTY;
    return {
      status: data.status,
      nodes: parseContent(data.content),
      sectionCount: data.sectionCount ?? 0,
      selectedContextIds: data.selectedContextIds ?? [],
      errorMessage: data.errorMessage ?? '',
      isRunning: data.status === 'running',
    };
  }, [data]);
}

/**
 * content is an opaque JSON string on the wire: ContentNode is a recursive
 * union the Firestore type generator cannot express, so the shape is checked
 * here instead. A malformed payload renders as nothing rather than throwing —
 * the turn document is the only place an error could otherwise surface, and it
 * already carries its own status.
 */
function parseContent(raw: string | undefined): ContentNode[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((n): n is ContentNode => typeof n === 'object' && n !== null && 'type' in n);
  } catch {
    return [];
  }
}
