import { env } from '@/config/env';
import { getAuthHeaders } from '@/features/auth/session';

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

async function getRequestAuthHeaders(): Promise<Record<string, string>> {
  try {
    return await getAuthHeaders();
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
  const authHeaders = await getRequestAuthHeaders();
  const url = `${env.apiBaseUrl}/synthify.tree.v1.${service}/${method}`;
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...authHeaders,
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
