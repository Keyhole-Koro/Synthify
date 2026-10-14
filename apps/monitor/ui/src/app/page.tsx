/**
 * Root home page of the Synthify Monitor, protected behind an authentication gate.
 *
 * @module
 */
import { AuthGate } from '@/components/AuthGate';
import { Dashboard } from '@/components/Dashboard';

/**
 * Default home view displaying the Operations BI Dashboard inside AuthGate.
 */
export default function Home() {
  return (
    <AuthGate>
      <Dashboard />
    </AuthGate>
  );
}
