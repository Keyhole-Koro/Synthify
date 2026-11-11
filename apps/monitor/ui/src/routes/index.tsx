/**
 * Home: the operations BI dashboard behind the admin sign-in.
 *
 * @module
 */
import { createFileRoute } from '@tanstack/react-router';
import { AuthGate } from '@/components/AuthGate';
import { Dashboard } from '@/components/Dashboard';

export const Route = createFileRoute('/')({
  component: () => (
    <AuthGate>
      <Dashboard />
    </AuthGate>
  ),
});
