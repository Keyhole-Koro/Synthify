import { type User } from 'firebase/auth';
import { type Workspace } from '@/features/workspaces/api';

type AuthMode = 'login' | 'register';

type AuthPaperProps = {
  user: User | null;
  workspaces: Workspace[];
  mode: AuthMode;
  loading: boolean;
  onModeChange: (mode: AuthMode) => void;
  onEmailSubmit: () => void;
  onGoogleSubmit: () => void;
  onLogout: () => void;
  onEnterWorkspace: () => void;
};

export function AuthPaper({
  user,
  workspaces,
  mode,
  loading,
  onModeChange,
  onEmailSubmit,
  onGoogleSubmit,
  onLogout,
  onEnterWorkspace,
}: AuthPaperProps) {
  const isLogin = mode === 'login';

  if (user) {
    return (
      <div style={{ paddingTop: 4 }}>
        <div style={{ marginBottom: 16 }}>
          <h3 style={{ fontSize: '1rem', fontWeight: 700, color: '#1e293b', margin: '0 0 4px' }}>
            ようこそ、{user.displayName || user.email} さん
          </h3>
          <p style={{ fontSize: '0.8rem', color: '#64748b', margin: 0 }}>
            ログインしました。ワークスペースに入って探索を始めましょう。
          </p>
        </div>

        {workspaces.length > 0 && (
          <div style={{ marginBottom: 16, padding: 12, background: '#f8fafc', borderRadius: 8, border: '1px solid #e2e8f0' }}>
            <p style={{ fontSize: '0.7rem', fontWeight: 600, color: '#64748b', marginBottom: 8, textTransform: 'uppercase' }}>
              あなたのワークスペース
            </p>
            {workspaces.map(ws => (
              <div key={ws.workspace_id} style={{ fontSize: '0.85rem', fontWeight: 500, color: '#1e293b' }}>
                {ws.name}
              </div>
            ))}
          </div>
        )}

        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <button
            type="button"
            onClick={onEnterWorkspace}
            disabled={loading}
            style={{
              width: '100%',
              border: 'none',
              background: '#4f46e5',
              borderRadius: 6,
              padding: '10px 0',
              fontSize: '0.85rem',
              fontWeight: 600,
              color: 'white',
              cursor: loading ? 'wait' : 'pointer',
              opacity: loading ? 0.7 : 1,
            }}
          >
            {workspaces.length > 0 ? 'ワークスペースに入る' : 'ワークスペースを作成'}
          </button>

          <button
            type="button"
            onClick={onLogout}
            style={{
              width: '100%',
              border: '1px solid #e2e8f0',
              background: 'white',
              borderRadius: 6,
              padding: '8px 0',
              fontSize: '0.8rem',
              fontWeight: 500,
              color: '#64748b',
              cursor: 'pointer',
            }}
          >
            ログアウト
          </button>
        </div>
      </div>
    );
  }

  return (
    <div style={{ paddingTop: 4 }}>
      <div style={{ marginBottom: 16 }}>
        <h3 style={{ fontSize: '1rem', fontWeight: 700, color: '#1e293b', margin: '0 0 4px' }}>
          {isLogin ? 'おかえりなさい' : 'はじめよう'}
        </h3>
        <p style={{ fontSize: '0.8rem', color: '#64748b', margin: 0 }}>
          {isLogin ? 'ドキュメントの知識グラフを探索する' : '無料でワークスペースを作成する'}
        </p>
      </div>

      <div style={{ display: 'flex', background: '#f1f5f9', borderRadius: 8, padding: 4, marginBottom: 16 }}>
        <button
          type="button"
          onClick={() => onModeChange('login')}
          style={{
            flex: 1,
            border: 'none',
            background: isLogin ? 'white' : 'transparent',
            borderRadius: 6,
            padding: '6px 0',
            fontSize: '0.75rem',
            fontWeight: 600,
            color: isLogin ? '#1e293b' : '#64748b',
            cursor: 'pointer',
            boxShadow: isLogin ? '0 1px 2px rgba(0,0,0,0.05)' : 'none',
          }}
        >
          ログイン
        </button>
        <button
          type="button"
          onClick={() => onModeChange('register')}
          style={{
            flex: 1,
            border: 'none',
            background: isLogin ? 'transparent' : 'white',
            borderRadius: 6,
            padding: '6px 0',
            fontSize: '0.75rem',
            fontWeight: 600,
            color: isLogin ? '#64748b' : '#1e293b',
            cursor: 'pointer',
            boxShadow: isLogin ? 'none' : '0 1px 2px rgba(0,0,0,0.05)',
          }}
        >
          新規登録
        </button>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 16 }}>
        {!isLogin && (
          <div>
            <label style={{ display: 'block', fontSize: '0.7rem', fontWeight: 600, color: '#64748b', marginBottom: 4 }}>
              お名前
            </label>
            <input
              type="text"
              placeholder="山田 太郎"
              style={{ width: '100%', border: '1px solid #e2e8f0', background: 'white', borderRadius: 6, padding: '8px 12px', fontSize: '0.85rem', outline: 'none' }}
            />
          </div>
        )}
        <div>
          <label style={{ display: 'block', fontSize: '0.7rem', fontWeight: 600, color: '#64748b', marginBottom: 4 }}>
            メールアドレス
          </label>
          <input
            type="email"
            placeholder="you@example.com"
            style={{ width: '100%', border: '1px solid #e2e8f0', background: 'white', borderRadius: 6, padding: '8px 12px', fontSize: '0.85rem', outline: 'none' }}
          />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '0.7rem', fontWeight: 600, color: '#64748b', marginBottom: 4 }}>
            パスワード
          </label>
          <input
            type="password"
            placeholder="••••••••"
            style={{ width: '100%', border: '1px solid #e2e8f0', background: 'white', borderRadius: 6, padding: '8px 12px', fontSize: '0.85rem', outline: 'none' }}
          />
        </div>
      </div>

      <button
        type="button"
        onClick={onEmailSubmit}
        disabled={loading}
        style={{
          width: '100%',
          border: 'none',
          background: '#4f46e5',
          borderRadius: 6,
          padding: '10px 0',
          fontSize: '0.85rem',
          fontWeight: 600,
          color: 'white',
          cursor: loading ? 'wait' : 'pointer',
          opacity: loading ? 0.7 : 1,
        }}
      >
        {isLogin ? 'ログイン' : 'アカウントを作成'}
      </button>

      <div style={{ display: 'flex', alignItems: 'center', gap: 12, margin: '16px 0' }}>
        <div style={{ flex: 1, height: 1, background: '#e2e8f0' }} />
        <span style={{ fontSize: '0.7rem', color: '#94a3b8' }}>または</span>
        <div style={{ flex: 1, height: 1, background: '#e2e8f0' }} />
      </div>

      <button
        type="button"
        onClick={onGoogleSubmit}
        disabled={loading}
        style={{
          width: '100%',
          border: '1px solid #e2e8f0',
          background: 'white',
          borderRadius: 6,
          padding: '8px 0',
          fontSize: '0.8rem',
          fontWeight: 500,
          color: '#475569',
          cursor: loading ? 'wait' : 'pointer',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          opacity: loading ? 0.7 : 1,
        }}
      >
        <svg style={{ width: 14, height: 14 }} viewBox="0 0 24 24"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/></svg>
        Google でログイン
      </button>
    </div>
  );
}

export type { AuthMode };
