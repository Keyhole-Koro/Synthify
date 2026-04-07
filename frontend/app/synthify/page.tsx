import { SynthifyCanvas } from '@/shared/components/SynthifyCanvas';

export const metadata = {
  title: 'Synthify - Canvas Prototype',
  description: 'Interactive Paper-in-Paper UI prototype.',
};

export default function SynthifyPage() {
  return (
    <main className="min-h-screen">
      <SynthifyCanvas />
    </main>
  );
}
