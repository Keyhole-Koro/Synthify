import { NextResponse, type NextRequest } from 'next/server';
import { adminAuth } from './firebase-admin';
import { isAdmin, parseAdminEmails, parseBearer } from './auth-rules';
import { config } from '../config';

export interface AdminPrincipal {
  uid: string;
  email: string;
}

// 認証 (Firebase ID トークン検証) と認可 (admin メール判定) を行う。
// いずれかに失敗したら NextResponse (401/403) を、成功したら AdminPrincipal を返す。
// admin メールが未設定の場合は fail-closed で全リクエストを 403 にする。
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

// ルートハンドラの先頭で使うガード。認可 NG なら NextResponse を返し (そのまま
// return する)、OK なら null を返す。
export async function requireAdmin(req: NextRequest): Promise<NextResponse | null> {
  const result = await authenticateAdmin(req);
  return result instanceof NextResponse ? result : null;
}
