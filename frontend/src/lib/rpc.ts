import { auth } from '@/lib/firebase';
import { env } from '@/config/env';

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

function isFirebaseAuthNetworkError(error: unknown): error is { code: string; message?: string } {
  return !!error && typeof error === 'object' && 'code' in error && error.code === 'auth/network-request-failed';
}

async function getAuthToken(): Promise<string | undefined> {
  if (!auth.currentUser) {
    return undefined;
  }
  try {
    return await auth.currentUser.getIdToken();
  } catch (error) {
    if (isFirebaseAuthNetworkError(error)) {
      const emulatorNote = env.firebase.authEmulatorUrl
        ? ` Firebase Auth emulator is configured at ${env.firebase.authEmulatorUrl}; verify that it is running and reachable from the browser.`
        : '';
      throw new Error(`Failed to refresh the Firebase ID token because the auth service was unreachable.${emulatorNote}`);
    }
    throw error;
  }
}

export async function callRPC<Req, Res>(
  service: string,
  method: string,
  body: Req,
): Promise<Res> {
  const token = await getAuthToken();
  const url = `${env.apiBaseUrl}/synthify.graph.v1.${service}/${method}`;
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      'Connect-Protocol-Version': '1',
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new ApiError(res.status, err.message ?? res.statusText);
  }
  return res.json() as Promise<Res>;
}
