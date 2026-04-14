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

export async function callRPC<Req, Res>(
  service: string,
  method: string,
  body: Req,
): Promise<Res> {
  const token = await auth.currentUser?.getIdToken();
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
