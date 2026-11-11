import { createClient, type Client } from '@connectrpc/connect';
import type { DescService } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { env } from '@/config/env';
import { getAuthHeaders } from '@/features/auth/session';

const baseUrl = env.apiBaseUrl;

const transport = createConnectTransport({
  baseUrl,
  interceptors: [
    (next) => async (req) => {
      const authHeaders = await getAuthHeaders();
      for (const [key, value] of Object.entries(authHeaders)) {
        req.header.set(key, value);
      }
      return await next(req);
    },
  ],
});

/**
 * Creates an authenticated ConnectRPC client injecting user Firebase ID tokens into request headers.
 *
 * @typeParam T - DescService definition type.
 * @param service - The protobuf service descriptor.
 * @returns Configured ConnectRPC client.
 */
export function createRPCClient<T extends DescService>(service: T): Client<T> {
  return createClient(service, transport);
}

// publicTransport does not append auth headers (for public links, share tokens, etc.)
const publicTransport = createConnectTransport({
  baseUrl,
});

/**
 * Creates an unauthenticated ConnectRPC client for public endpoints (such as shared workspace links).
 *
 * @typeParam T - DescService definition type.
 * @param service - The protobuf service descriptor.
 * @returns Configured public ConnectRPC client.
 */
export function createPublicRPCClient<T extends DescService>(service: T): Client<T> {
  return createClient(service, publicTransport);
}
