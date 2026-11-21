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

// 検証済み ID トークンのうち、admin 判定が読むクレーム。
export interface AdminClaims {
  email?: string | null;
  email_verified?: boolean;
}

// トークンの持ち主が admin か。メールが確認済み (email_verified が true) で、
// admin 集合に含まれるときだけ true。Firebase はメールとパスワードの登録で
// アドレスの持ち主かを確かめないので、未確認のメールでは判定しない
// (apps/api の principalFromClaims と同じ規則)。admin が空なら常に false =
// fail-closed。
export function isAdmin(claims: AdminClaims, admins: Set<string>): boolean {
  if (claims.email_verified !== true) return false;
  const normalized = (claims.email ?? '').trim().toLowerCase();
  return admins.size > 0 && normalized !== '' && admins.has(normalized);
}
