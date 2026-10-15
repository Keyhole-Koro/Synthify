/**
 * LLM Eval dashboard: runs, execution traces and prompt comparisons.
 *
 * @module
 */
import { createFileRoute } from '@tanstack/react-router';
import { AuthGate } from '@/components/AuthGate';
import { EvalDashboard } from '@/components/EvalDashboard';

export const Route = createFileRoute('/dashboards/eval')({
  component: () => (
    <AuthGate>
      <EvalDashboard />
    </AuthGate>
  ),
});
