import { createClient, type Client } from '@connectrpc/connect';
import type { DescService } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { env } from '@/config/env';
import { getAuthHeaders } from '@/features/auth/session';
import { toAppError } from './error_normalize';

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
        throw toAppError(err);
      }
    },
  ],
});

export function createRPCClient<T extends DescService>(service: T): Client<T> {
  return createClient(service, transport);
}
