import { useSyncExternalStore } from 'react';

// Whether Synthify still takes new documents and paid plans. When the API's
// SYNTHIFY_INTAKE_CLOSED is on, SignInUser says so (intakeClosed), and every
// upload, processing or checkout call is refused with the INTAKE_CLOSED
// reason. A page loaded before the switch learns it from the first refusal
// (lib/connect.ts). While it is closed the page hides upload and upgrade
// entry points; reading, sharing and deleting stay as they are.

export const INTAKE_CLOSED_NOTICE =
  'Synthify は新しい文書の受け付けと有料プランを終了しました。これまでに作ったツリーは、引き続き見る・共有する・削除することができます。';
// The API answers refused calls with these same messages.
export const INTAKE_CLOSED_DOCUMENTS_MESSAGE = '新しい文書の受け付けは終了しました。';
export const INTAKE_CLOSED_PAID_PLANS_MESSAGE = '有料プランの受け付けは終了しました。';

let intakeClosed = false;
const listeners = new Set<() => void>();

export function setIntakeClosed(closed: boolean): void {
  if (intakeClosed === closed) return;
  intakeClosed = closed;
  for (const listener of listeners) listener();
}

export function isIntakeClosed(): boolean {
  return intakeClosed;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useIntakeClosed(): boolean {
  return useSyncExternalStore(subscribe, isIntakeClosed, () => false);
}
