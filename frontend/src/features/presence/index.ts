import {
  collection,
  doc,
  query,
  setDoc,
  onSnapshot,
  serverTimestamp,
  type Unsubscribe,
} from 'firebase/firestore';
import { db } from '@/lib/firebase';

export interface PresenceData {
  user_id: string;
  node_id?: string | null;
  updated_at: unknown; // serverTimestamp()
}

/**
 * ワークスペース内のユーザーの presence 情報を Firestore に書き込む。
 * パス: workspaces/{workspaceId}/presence/{userId}
 */
export async function updatePresence(
  workspaceId: string,
  userId: string,
  nodeId?: string,
): Promise<void> {
  const ref = doc(db, 'workspaces', workspaceId, 'presence', userId);
  await setDoc(ref, {
    user_id: userId,
    node_id: nodeId ?? null,
    updated_at: serverTimestamp(),
  });
}

/**
 * ワークスペースの presence 情報をリアルタイムで購読する。
 * コールバックには全ユーザーの最新 presence データが渡される。
 */
export function subscribePresence(
  workspaceId: string,
  cb: (entries: PresenceData[]) => void,
): Unsubscribe {
  const presenceRef = collection(db, 'workspaces', workspaceId, 'presence');
  const q = query(presenceRef);
  return onSnapshot(q, (snapshot) => {
    const entries: PresenceData[] = snapshot.docs.map((d) => d.data() as PresenceData);
    cb(entries);
  });
}
