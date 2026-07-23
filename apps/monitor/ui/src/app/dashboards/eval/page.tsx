import { AuthGate } from '@/components/AuthGate';
import { EvalDashboard } from '@/components/EvalDashboard';

export default function EvalDashboardPage() {
  return (
    <AuthGate>
      <EvalDashboard />
    </AuthGate>
  );
}
