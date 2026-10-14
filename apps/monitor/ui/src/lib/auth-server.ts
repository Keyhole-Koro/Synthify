/**
 * Server-side Firebase ID token verification and admin authorization guard middleware.
 *
 * @module
 */
import { NextResponse, type NextRequest } from 'next/server';
import { adminAuth } from './firebase-admin';
import { isAdmin, parseAdminEmails, parseBearer } from './auth-rules';
import { config } from '../config';

/**
 * Authenticated admin user identity principal.
 */
export interface AdminPrincipal {
  /** Unique Firebase user identifier. */
  uid: string;
  /** Verified email address in lowercase. */
  email: string;
}
/**
 * Verifies the Firebase ID token in the Authorization header and confirms admin membership.
 * Fails closed with 401 Unauthorized or 403 Forbidden responses on validation errors.
 *
 * @param req Incoming Next.js API request.
 * @returns The resolved AdminPrincipal, or a NextResponse error on failure.
 */
export async function authenticateAdmin(
  req: NextRequest,
): Promise<AdminPrincipal | NextResponse> {
  const token = parseBearer(req.headers.get('authorization'));
  if (!token) {
    return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  }

  let decoded;
  try {
    decoded = await adminAuth().verifyIdToken(token);
  } catch {
    return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  }

  if (!isAdmin(decoded.email, parseAdminEmails(config.SYNTHIFY_ADMIN_USER_EMAILS))) {
    return NextResponse.json({ error: 'forbidden' }, { status: 403 });
  }

  return { uid: decoded.uid, email: (decoded.email ?? '').toLowerCase() };
}

/**
 * Guard utility for Route Handlers. Returns an error NextResponse if unauthenticated or forbidden,
 * or null if authorization succeeds.
 *
 * @param req Incoming Next.js API request.
 * @returns Error response to terminate the request, or null to continue.
 */
export async function requireAdmin(req: NextRequest): Promise<NextResponse | null> {
  const result = await authenticateAdmin(req);
  return result instanceof NextResponse ? result : null;
}
