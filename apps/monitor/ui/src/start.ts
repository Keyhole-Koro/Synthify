/**
 * TanStack Start instance options.
 *
 * Pages render in the browser only: every API call needs the admin's
 * Firebase ID token, which exists only client-side, so server rendering
 * would have nothing to show. The /api server routes still run on the server.
 *
 * @module
 */
import { createStart } from '@tanstack/react-start';

export const startInstance = createStart(() => ({ defaultSsr: false }));
