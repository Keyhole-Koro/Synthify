import { auth } from '@/lib/firebase';
import { env } from '@/config/env';

export interface UserPresence {
  uid: string;
  displayName?: string;
  photoURL?: string;
  lastActive: string;
  status: 'online' | 'away' | 'offline';
  workspace_id?: string | null;
  item_id?: string | null;
}

export async function updateUserPresence(
  status: 'online' | 'away' | 'offline',
  workspaceId?: string,
  itemId?: string,
) {
  if (!auth.currentUser) return;
  const token = await auth.currentUser.getIdToken();
  const url = `${env.apiBaseUrl}/presence/update`;
  await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      status,
      workspace_id: workspaceId ?? null,
      item_id: itemId ?? null,
    }),
  });
}
