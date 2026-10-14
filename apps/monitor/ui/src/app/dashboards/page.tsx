/**
 * Operations BI Dashboard page with quick navigation link to LLM Eval traces.
 *
 * @module
 */
import Link from 'next/link';
import { AuthGate } from '@/components/AuthGate';
import { Dashboard } from '@/components/Dashboard';

/**
 * Operations BI page presenting system health metrics, cost trends, and link to eval trace view.
 */
export default function DashboardsPage() {
  return (
    <AuthGate>
      <div className="relative h-screen">
        <Dashboard />
        <Link
          href="/dashboards/eval"
          className="fixed bottom-3 left-3 z-20 w-[200px] rounded-md border border-stone-200 bg-white px-3 py-2 text-xs font-semibold text-stone-700 shadow-sm transition-colors hover:bg-stone-100"
        >
          LLM Eval →
        </Link>
      </div>
    </AuthGate>
  );
}
