// Type of the TanStack Start server bundle (dist/server/server.js) that server.ts serves.
declare module '*/dist/server/server.js' {
  const handler: { fetch(request: Request): Response | Promise<Response> };
  export default handler;
}
