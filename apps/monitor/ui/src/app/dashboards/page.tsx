import { AuthGate } from '@/components/AuthGate';
import { Dashboard } from '@/components/Dashboard';

export default function DashboardsPage() {
  return (
    <AuthGate>
      <Dashboard />
    </AuthGate>
  );
}
