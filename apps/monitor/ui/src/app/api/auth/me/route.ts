import { NextResponse, type NextRequest } from 'next/server';
import { authenticateAdmin } from '@/lib/auth-server';

// 現在の Bearer トークンが admin かどうかを返す。AuthGate が認可状態の判定に使う。
export async function GET(req: NextRequest) {
  const result = await authenticateAdmin(req);
  if (result instanceof NextResponse) return result;
  return NextResponse.json({ uid: result.uid, email: result.email, admin: true });
}
