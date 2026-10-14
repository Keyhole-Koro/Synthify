/**
 * Admin authorization status API endpoint.
 *
 * @module
 */
import { NextResponse, type NextRequest } from 'next/server';
import { authenticateAdmin } from '@/lib/auth-server';

/**
 * Handles GET requests to verify whether the requesting user's Bearer token
 * is valid and grants administrator privileges.
 *
 * @param req - Incoming HTTP request with Bearer authorization token.
 * @returns JSON response with admin user claims, or 401/403 status.
 */
export async function GET(req: NextRequest) {
  const result = await authenticateAdmin(req);
  if (result instanceof NextResponse) return result;
  return NextResponse.json({ uid: result.uid, email: result.email, admin: true });
}
