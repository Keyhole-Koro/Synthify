import { describe, expect, test } from 'bun:test';
import { isAdmin, parseAdminEmails, parseBearer } from './auth-rules';

describe('parseBearer', () => {
  test('extracts the token from a Bearer header', () => {
    expect(parseBearer('Bearer abc.def.ghi')).toBe('abc.def.ghi');
  });

  test('is case-insensitive on the scheme', () => {
    expect(parseBearer('bearer tok')).toBe('tok');
    expect(parseBearer('BEARER tok')).toBe('tok');
  });

  test('returns empty for missing / malformed / wrong scheme', () => {
    expect(parseBearer(null)).toBe('');
    expect(parseBearer('')).toBe('');
    expect(parseBearer('abc.def.ghi')).toBe(''); // no scheme
    expect(parseBearer('Basic abc')).toBe(''); // wrong scheme
    expect(parseBearer('Bearer')).toBe(''); // no token
  });
});

describe('parseAdminEmails', () => {
  test('splits, trims and lowercases', () => {
    const set = parseAdminEmails(' Alice@Example.com , bob@example.com ');
    expect(set.has('alice@example.com')).toBe(true);
    expect(set.has('bob@example.com')).toBe(true);
    expect(set.size).toBe(2);
  });

  test('drops empty entries', () => {
    expect(parseAdminEmails('').size).toBe(0);
    expect(parseAdminEmails(' , , ').size).toBe(0);
  });
});

describe('isAdmin (fail-closed)', () => {
  const admins = parseAdminEmails('admin@example.com');
  const verified = (email: string | null | undefined) => ({ email, email_verified: true });

  test('allows a listed, verified email (case-insensitive)', () => {
    expect(isAdmin(verified('admin@example.com'), admins)).toBe(true);
    expect(isAdmin(verified('ADMIN@example.com'), admins)).toBe(true);
    expect(isAdmin(verified('  admin@example.com '), admins)).toBe(true);
  });

  // Firebase does not check that whoever signs up with an email and password
  // owns the address, so a listed address proves nothing until it is verified.
  test('denies a listed email that is not verified', () => {
    expect(isAdmin({ email: 'admin@example.com', email_verified: false }, admins)).toBe(false);
    expect(isAdmin({ email: 'admin@example.com' }, admins)).toBe(false);
  });

  test('denies a non-listed email', () => {
    expect(isAdmin(verified('someone@example.com'), admins)).toBe(false);
  });

  test('denies empty / missing email', () => {
    expect(isAdmin(verified(''), admins)).toBe(false);
    expect(isAdmin(verified(null), admins)).toBe(false);
    expect(isAdmin(verified(undefined), admins)).toBe(false);
  });

  test('fail-closed: an empty admin set denies everyone', () => {
    const none = parseAdminEmails('');
    expect(isAdmin(verified('admin@example.com'), none)).toBe(false);
    expect(isAdmin(verified(''), none)).toBe(false);
  });
});
