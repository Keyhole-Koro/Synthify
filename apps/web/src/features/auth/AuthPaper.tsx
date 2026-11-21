import type { AuthUser } from '@/features/auth/session';
import { LoggedInProfile } from './components/LoggedInProfile';
import { AuthForm } from './components/AuthForm';
import { SocialLogin } from './components/SocialLogin';

/**
 * Props for the authentication paper panel.
 */
type AuthPaperProps = {
  /** Currently authenticated user, or null if logged out. */
  user: AuthUser | null;
  /** Whether an authentication request is actively loading. */
  loading: boolean;
  /** Optional error from the previous sign-in attempt. */
  error?: Error | null;
  /** Callback to trigger Google OAuth sign-in. */
  onGoogleSubmit: () => void;
  /** Callback to trigger user sign-out. */
  onLogout: () => void;
};

/**
 * Authentication sheet component displaying either the user's logged-in profile or sign-in options.
 *
 * @param props - Current auth state and callbacks for sign-in/sign-out actions.
 * @returns Authenticated profile content or the login form for the current session state.
 */
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
