import { createClient, type Client } from '@connectrpc/connect';
import type { DescService } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { env } from '@/config/env';
import { getAuthHeaders } from '@/features/auth/session';
import { setIntakeClosed } from '@/features/intake/intakeStatus';
import { toAppError } from './error_normalize';
import { isIntakeClosedError } from './errors';

const baseUrl = env.apiBaseUrl;

const transport = createConnectTransport({
  baseUrl,
  interceptors: [
    (next) => async (req) => {
      try {
        const authHeaders = await getAuthHeaders();
        for (const [key, value] of Object.entries(authHeaders)) {
          req.header.set(key, value);
        }
      } catch (err) {
        // If auth headers fail (e.g. token refresh failed), normalize and rethrow
        throw toAppError(err);
      }
      
      try {
        return await next(req);
      } catch (err) {
        // Normalize RPC errors
        const appError = toAppError(err);
        // A page loaded before intake closed learns it from the first
        // refusal, and hides the rest of the upload and upgrade entry points.
        if (isIntakeClosedError(appError)) {
          setIntakeClosed(true);
        }
        throw appError;
      }
    },
  ],
});

export function createRPCClient<T extends DescService>(service: T): Client<T> {
  return createClient(service, transport);
}

// publicTransport は認証ヘッダを付けない。公開リンク (ResolveShareLink) のように
// ログイン不要で叩く無認証経路のためのもの。エラー正規化だけは共有する。
const publicTransport = createConnectTransport({
  baseUrl,
  interceptors: [
    (next) => async (req) => {
      try {
        return await next(req);
      } catch (err) {
        throw toAppError(err);
      }
    },
  ],
});

export function createPublicRPCClient<T extends DescService>(service: T): Client<T> {
  return createClient(service, publicTransport);
}
