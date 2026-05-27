import React from 'react';
import type { AuthUser } from '@/features/auth/session';
import { LoggedInProfile } from './components/LoggedInProfile';
import { AuthForm } from './components/AuthForm';
import { SocialLogin } from './components/SocialLogin';
import { type AppError } from '@/lib/errors';

type AuthPaperProps = {
  user: AuthUser | null;
  loading: boolean;
  error?: AppError | null;
  onGoogleSubmit: () => void;
  onLogout: () => void;
};

export function AuthPaper({
  user,
  loading,
  error,
  onGoogleSubmit,
  onLogout,
}: AuthPaperProps) {
  if (user) {
    return <LoggedInProfile user={user} onLogout={onLogout} />;
  }

  return (
    <>
      <AuthForm />
      <SocialLogin
        loading={loading}
        error={error}
        onGoogleSubmit={onGoogleSubmit}
      />
    </>
  );
}
