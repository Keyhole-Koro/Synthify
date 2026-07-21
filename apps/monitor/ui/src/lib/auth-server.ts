import { NextResponse, type NextRequest } from 'next/server';
import { adminAuth } from './firebase-admin';
import { config } from '../config';

export interface AdminPrincipal {
  uid: string;
  email: string;
}

function adminEmailSet(): Set<string> {
  return new Set(
    config.SYNTHIFY_ADMIN_USER_EMAILS.split(',')
      .map((e) => e.trim().toLowerCase())
      .filter((e) => e.length > 0),
  );
}

function bearerToken(req: NextRequest): string {
  const header = req.headers.get('authorization') ?? '';
  const [scheme, token] = header.split(' ');
  if (!token || scheme?.toLowerCase() !== 'bearer') return '';
  return token.trim();
}

// 認証 (Firebase ID トークン検証) と認可 (admin メール判定) を行う。
// いずれかに失敗したら NextResponse (401/403) を、成功したら AdminPrincipal を返す。
// admin メールが未設定の場合は fail-closed で全リクエストを 403 にする。
export async function authenticateAdmin(
  req: NextRequest,
): Promise<AdminPrincipal | NextResponse> {
  const token = bearerToken(req);
  if (!token) {
    return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  }

  let decoded;
  try {
    decoded = await adminAuth().verifyIdToken(token);
  } catch {
    return NextResponse.json({ error: 'unauthenticated' }, { status: 401 });
  }

  const email = (decoded.email ?? '').toLowerCase();
  const admins = adminEmailSet();
  if (admins.size === 0 || email === '' || !admins.has(email)) {
    return NextResponse.json({ error: 'forbidden' }, { status: 403 });
  }

  return { uid: decoded.uid, email };
}

// ルートハンドラの先頭で使うガード。認可 NG なら NextResponse を返し (そのまま
// return する)、OK なら null を返す。
export async function requireAdmin(req: NextRequest): Promise<NextResponse | null> {
  const result = await authenticateAdmin(req);
  return result instanceof NextResponse ? result : null;
}
