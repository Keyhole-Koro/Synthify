import React from 'react';
import type { AuthUser } from '@/features/auth/session';
import { LoggedInProfile } from './components/LoggedInProfile';
import { AuthForm } from './components/AuthForm';
import { SocialLogin } from './components/SocialLogin';

type AuthMode = 'login' | 'register';

type AuthPaperProps = {
  user: AuthUser | null;
  mode: AuthMode;
  loading: boolean;
  onModeChange: (mode: AuthMode) => void;
  onEmailSubmit: () => void;
  onGoogleSubmit: () => void;
  onLogout: () => void;
};

export function AuthPaper({
  user,
  mode,
  loading,
  onModeChange,
  onEmailSubmit,
  onGoogleSubmit,
  onLogout,
}: AuthPaperProps) {
  if (user) {
    return <LoggedInProfile user={user} onLogout={onLogout} />;
  }

  return (
    <>
      <AuthForm
        mode={mode}
        loading={loading}
        onModeChange={onModeChange}
        onEmailSubmit={onEmailSubmit}
      />
      <SocialLogin
        loading={loading}
        onGoogleSubmit={onGoogleSubmit}
      />
    </>
  );
}

export type { AuthMode };
