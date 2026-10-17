#!/usr/bin/env node
// User-controlled, CLI-only proof of concept. Never expose this as a network proxy.
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { homedir } from 'node:os';
import { dirname, join } from 'node:path';
import { mkdir, readFile, rename, writeFile, chmod, unlink } from 'node:fs/promises';

const AUTH = 'https://auth.openai.com';
const RESOURCE = 'https://api.openai.com/v1';
const STORE = join(homedir(), '.config', 'synthify', 'siwc-poc.json');
const SCOPES = 'openid profile email offline_access resource.invoke chatgpt.tokens.use.direct';

export const pkceChallenge = (verifier) => createHash('sha256').update(verifier).digest('base64url');
export const randomSecret = () => randomBytes(32).toString('base64url');
export const hasPlanScope = (scopes) => String(scopes ?? '').split(/\s+/).includes('chatgpt.tokens.use.direct');

export function authorizationUrl({ clientId, redirectUri, hostId, state, nonce, challenge }) {
  const url = new URL('/api/accounts/authorize', AUTH);
  Object.entries({ client_id: clientId, redirect_uri: redirectUri, response_type: 'code',
    scope: SCOPES, resource: RESOURCE, state, nonce, code_challenge: challenge,
    code_challenge_method: 'S256', ext_agent_host_id: hostId }).forEach(([k, v]) => url.searchParams.set(k, v));
  if (clientId === 'dynamic_agent_client') url.searchParams.set('agent_name_hint', 'Synthify');
  return url;
}

// Handles SSE frames split at arbitrary byte boundaries, including CRLF.
export async function* sseEvents(body) {
  let pending = '';
  const decoder = new TextDecoder();
  for await (const chunk of body) {
    pending += decoder.decode(chunk, { stream: true });
    pending = pending.replace(/\r\n/g, '\n');
    let boundary;
    while ((boundary = pending.indexOf('\n\n')) !== -1) {
      const frame = pending.slice(0, boundary);
      pending = pending.slice(boundary + 2);
      const data = frame.split('\n').filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n');
      if (data && data !== '[DONE]') yield JSON.parse(data);
    }
  }
  pending += decoder.decode();
  if (pending.trim()) throw new Error('Truncated SSE stream');
}

async function load() {
  try { return JSON.parse(await readFile(STORE, 'utf8')); }
  catch (err) { if (err.code === 'ENOENT') return null; throw err; }
}

async function save(data) {
  if (process.platform === 'win32') throw new Error('Windows credential storage requires OS keychain integration; unsupported in this PoC.');
  await mkdir(dirname(STORE), { recursive: true, mode: 0o700 });
  const tmp = STORE + '.' + randomUUID() + '.tmp';
  try {
    await writeFile(tmp, JSON.stringify(data, null, 2), { mode: 0o600, flag: 'wx' });
    await chmod(tmp, 0o600);
    await rename(tmp, STORE);
  } finally { await unlink(tmp).catch(err => { if (err.code !== 'ENOENT') throw err; }); }
}

async function postToken(params) {
  const response = await fetch(AUTH + '/api/accounts/oauth/token', {
    method: 'POST', headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams(params), signal: AbortSignal.timeout(15000),
  });
  if (!response.ok) throw new Error(`Token exchange failed (HTTP ${response.status}); re-run login.`);
  return response.json();
}

async function validateIdToken(token, clientId, nonce) {
  const { createRemoteJWKSet, jwtVerify } = await import('jose');
  const jwks = createRemoteJWKSet(new URL(AUTH + '/.well-known/jwks.json'));
  const { payload } = await jwtVerify(token, jwks, { issuer: AUTH, audience: clientId });
  if (!payload.sub || payload.nonce !== nonce) throw new Error('Invalid ID-token subject or nonce');
  return payload;
}

function callbackListener() {
  let resolve, reject;
  const completed = new Promise((ok, fail) => { resolve = ok; reject = fail; });
  const server = createServer((req, res) => {
    const port = server.address()?.port;
    if (req.headers.host !== `127.0.0.1:${port}` || !req.url?.startsWith('/auth/callback?')) {
      res.writeHead(404).end('Not found'); return;
    }
    const params = new URL(req.url, `http://127.0.0.1:${port}`).searchParams;
    res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8', 'cache-control': 'no-store' })
      .end('Synthify sign-in callback received. You may close this tab.');
    resolve(params);
  });
  server.on('error', reject);
  return { server, completed };
}

function openBrowser(url) {
  const cmd = process.platform === 'darwin' ? 'open' : process.platform === 'linux' ? 'xdg-open' : null;
  if (!cmd) throw new Error('This PoC supports macOS/Linux only.');
  const child = spawn(cmd, [url], { stdio: 'ignore' });
  child.on('error', () => {}); // Copy/paste URL below if no browser opener is installed.
}

async function login() {
  const existing = await load();
  const hostId = existing?.hostId ?? `urn:uuid:${randomUUID()}`;
  const clientId = existing?.clientId ?? 'dynamic_agent_client';
  const verifier = randomSecret(), nonce = randomSecret(), state = randomSecret();
  const { server, completed } = callbackListener();
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const redirectUri = `http://127.0.0.1:${server.address().port}/auth/callback`;
  const url = authorizationUrl({ clientId, hostId, redirectUri, state, nonce, challenge: pkceChallenge(verifier) });
  console.log('Opening Continue with ChatGPT in your browser. If it does not open, paste this URL:');
  console.log(url.href); // This flow deliberately omits id_token_hint, which must never be logged.
  openBrowser(url.href);
  try {
    const params = await Promise.race([
      completed,
      new Promise((_, reject) => setTimeout(() => reject(new Error('OAuth callback timed out')), 180000)),
    ]);
    if (params.get('state') !== state) throw new Error('OAuth state mismatch');
    if (params.has('error')) throw new Error('Authorization denied or failed');
    const code = params.get('code');
    if (!code) throw new Error('Missing authorization code');
    const issuedId = params.get('client_id');
    if (clientId === 'dynamic_agent_client' && !issuedId?.startsWith('oaiapp_')) throw new Error('Dynamic client registration failed');
    if (clientId !== 'dynamic_agent_client' && issuedId && issuedId !== clientId) throw new Error('Client ID changed');
    const selectedClient = clientId === 'dynamic_agent_client' ? issuedId : clientId;
    const tokens = await postToken({ grant_type: 'authorization_code', code, client_id: selectedClient,
      code_verifier: verifier, redirect_uri: redirectUri, resource: RESOURCE });
    if (!tokens.id_token || !tokens.access_token || !hasPlanScope(tokens.scope)) throw new Error('Plan permission not granted');
    const identity = await validateIdToken(tokens.id_token, selectedClient, nonce);
    if (existing?.sub && existing.sub !== identity.sub) throw new Error('Account identity changed; do not replace credentials');
    await save({ hostId, clientId: selectedClient, sub: identity.sub, email: identity.email ?? '',
      accessToken: tokens.access_token, refreshToken: tokens.refresh_token ?? null,
      expiresAt: Date.now() + (Number(tokens.expires_in ?? 0) * 1000), scope: tokens.scope });
    console.log('Signed in as', identity.email || identity.sub);
  } finally { server.close(); }
}

async function session() {
  const saved = await load();
  if (!saved?.accessToken || !hasPlanScope(saved.scope)) throw new Error('Run: node siwc.mjs login');
  // Keep this PoC simple: do not silently run rotating refreshes across concurrent processes.
  if (Date.now() >= saved.expiresAt - 30000) throw new Error('Session expired; run: node siwc.mjs login');
  return saved;
}

async function models() {
  const saved = await session();
  const response = await fetch(RESOURCE + '/models', { headers: { authorization: `Bearer ${saved.accessToken}` }, signal: AbortSignal.timeout(20000) });
  if (!response.ok) throw new Error(`Model list failed: HTTP ${response.status}`);
  const json = await response.json();
  for (const model of (json.models ?? []).filter(x => x.visibility === 'list'))
    console.log(`${model.slug}\t${model.display_name}`);
}

async function ask(model, prompt) {
  if (!model || !prompt) throw new Error('Usage: node siwc.mjs ask <model-slug> <prompt>');
  const saved = await session();
  const response = await fetch(RESOURCE + '/responses', { method: 'POST',
    headers: { authorization: `Bearer ${saved.accessToken}`, 'content-type': 'application/json' },
    body: JSON.stringify({ model, input: [{ role: 'user', content: prompt }], store: false, stream: true }),
    signal: AbortSignal.timeout(120000) });
  if (!response.ok || !response.body) throw new Error(`Inference request failed: HTTP ${response.status}`);
  let completed = false;
  for await (const event of sseEvents(response.body)) {
    if (event.type === 'response.output_text.delta') process.stdout.write(event.delta ?? '');
    if (event.type === 'response.completed') completed = true;
    if (['response.failed', 'response.incomplete', 'error'].includes(event.type))
      throw new Error(`Inference failed: ${event.response?.error?.code ?? event.error?.code ?? event.type}`);
  }
  if (!completed) throw new Error('Stream ended without response.completed');
  process.stdout.write('\n');
}

async function logout() {
  const saved = await load();
  if (!saved) return console.log('No saved session.');
  let revoked = false;
  if (saved.refreshToken) {
    try {
      const discovery = await (await fetch(AUTH + '/.well-known/openid-configuration', { signal: AbortSignal.timeout(15000) })).json();
      const res = await fetch(discovery.revocation_endpoint, { method: 'POST',
        headers: { 'content-type': 'application/x-www-form-urlencoded' }, signal: AbortSignal.timeout(15000),
        body: new URLSearchParams({ token: saved.refreshToken, token_type_hint: 'refresh_token', client_id: saved.clientId }) });
      revoked = res.ok;
    } catch { /* Keep local logout possible even when offline. */ }
  }
  await save({ hostId: saved.hostId, clientId: saved.clientId, sub: saved.sub, email: saved.email });
  console.log(revoked ? 'Revoked renewable session and cleared local tokens.' :
    'Local tokens cleared. Remote revocation unconfirmed: disconnect in ChatGPT Settings.');
}

if (process.argv[1] && import.meta.url === new URL(`file://${process.argv[1]}`).href) {
  const [action, ...args] = process.argv.slice(2);
  try {
    if (action === 'login') await login();
    else if (action === 'models') await models();
    else if (action === 'ask') await ask(args[0], args.slice(1).join(' '));
    else if (action === 'logout') await logout();
    else throw new Error('Usage: node siwc.mjs login | models | ask <model-slug> <prompt> | logout');
  } catch (err) { console.error(err.message); process.exitCode = 1; }
}
