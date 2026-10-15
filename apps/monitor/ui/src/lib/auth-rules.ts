/**
 * Pure authorization and token parsing helper functions independent of runtime frameworks.
 *
 * @module
 */

/**
 * Extracts the bearer token string from an Authorization header value.
 *
 * @param header Raw Authorization header string (e.g. "Bearer <token>").
 * @returns The extracted token string, or an empty string if malformed or missing.
 */
export function parseBearer(header: string | null | undefined): string {
  const [scheme, token] = (header ?? '').split(' ');
  if (!token || scheme?.toLowerCase() !== 'bearer') return '';
  return token.trim();
}
/**
 * Parses a comma-separated list of admin email addresses into a normalized lowercase set.
 *
 * @param csv Comma-separated admin email list.
 * @returns Set of normalized, trimmed lowercase email strings.
 */
export function parseAdminEmails(csv: string): Set<string> {
  return new Set(
    csv
      .split(',')
      .map((e) => e.trim().toLowerCase())
      .filter((e) => e.length > 0),
  );
}

/**
 * Evaluates whether an email address belongs to the authorized admin set.
 * Fails closed by returning false if the admin set is empty or the email is omitted.
 *
 * @param email User email address to evaluate.
 * @param admins Normalized set of allowed admin emails.
 * @returns True if the email is present in the admin set.
 */
export function isAdmin(email: string | null | undefined, admins: Set<string>): boolean {
  const normalized = (email ?? '').trim().toLowerCase();
  return admins.size > 0 && normalized !== '' && admins.has(normalized);
}
