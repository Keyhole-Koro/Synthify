/**
 * Root HTML layout shell for the Synthify Monitor application.
 *
 * @module
 */
import type { Metadata } from 'next';
import './globals.css';

/** Page metadata definition for the monitor application. */
export const metadata: Metadata = {
  title: 'Synthify Log Viewer',
};

/**
 * Root layout component wrapping all monitor UI pages in an HTML document shell.
 */
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="bg-stone-50 text-stone-900 antialiased">{children}</body>
    </html>
  );
}
