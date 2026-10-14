/**
 * LLM Evaluation Trace Dashboard page.
 *
 * @module
 */
import { AuthGate } from '@/components/AuthGate';
import { EvalDashboard } from '@/components/EvalDashboard';

/**
 * LLM Evaluation page protected by AuthGate, rendering test runs, execution trees, and prompt analytics.
 */
export default function EvalDashboardPage() {
  return (
    <AuthGate>
      <EvalDashboard />
    </AuthGate>
  );
}
