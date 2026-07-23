// 認可判定の純粋ロジック。Next / Firebase / env に依存しないのでそのまま単体
// テストできる。auth-server.ts (Firebase 検証 + NextResponse) から利用する。

// Authorization ヘッダから Bearer トークンを取り出す。無効なら空文字。
export function parseBearer(header: string | null | undefined): string {
  const [scheme, token] = (header ?? '').split(' ');
  if (!token || scheme?.toLowerCase() !== 'bearer') return '';
  return token.trim();
}

// admin メールの CSV を、trim + lowercase + 空要素除去した Set にする。
export function parseAdminEmails(csv: string): Set<string> {
  return new Set(
    csv
      .split(',')
      .map((e) => e.trim().toLowerCase())
      .filter((e) => e.length > 0),
  );
}

// email が admin 集合に含まれるか。admin が空なら常に false = fail-closed。
export function isAdmin(email: string | null | undefined, admins: Set<string>): boolean {
  const normalized = (email ?? '').trim().toLowerCase();
  return admins.size > 0 && normalized !== '' && admins.has(normalized);
}
