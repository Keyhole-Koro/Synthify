import assert from 'node:assert/strict';
import { test } from 'node:test';
import { pkceChallenge, authorizationUrl, hasPlanScope, sseEvents } from './siwc.mjs';

test('PKCE challenge matches RFC 7636 test vector', () => {
  assert.equal(pkceChallenge('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk'),
    'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM');
});

test('new registration requires local loopback and correct scopes', () => {
  const u = authorizationUrl({ clientId: 'dynamic_agent_client', hostId: 'urn:uuid:test',
    redirectUri: 'http://127.0.0.1:9000/auth/callback', state: 's', nonce: 'n', challenge: 'c' });
  assert.equal(u.searchParams.get('agent_name_hint'), 'Synthify');
  assert.equal(u.searchParams.get('resource'), 'https://api.openai.com/v1');
  assert.equal(u.searchParams.get('code_challenge_method'), 'S256');
  assert.ok(hasPlanScope(u.searchParams.get('scope')));
  const old = authorizationUrl({ clientId: 'oaiapp_test', hostId: 'urn:uuid:test',
    redirectUri: 'http://127.0.0.1:9000/auth/callback', state: 's', nonce: 'n', challenge: 'c' });
  assert.equal(old.searchParams.get('agent_name_hint'), null);
});

test('scope validation requires exact permission', () => {
  assert.equal(hasPlanScope('openid chatgpt.tokens.use.directly'), false);
  assert.equal(hasPlanScope('openid chatgpt.tokens.use.direct'), true);
});

test('SSE handles split frames and CRLF', async () => {
  const chunks = ['data: {"type":"response.output_text.delta","delta":"A"}\r',
    '\n\r\ndata: {"type":"response.completed"}\n\n'];
  const frames = [];
  for await (const e of sseEvents((async function* () { for (const s of chunks) yield Buffer.from(s); })())) frames.push(e);
  assert.deepEqual(frames.map(f => f.type), ['response.output_text.delta', 'response.completed']);
});

test('SSE refuses truncated event stream', async () => {
  await assert.rejects(async () => {
    for await (const _ of sseEvents((async function* () { yield Buffer.from('data: {"type":"response.completed"}'); })())) {}
  }, /Truncated SSE/);
});
