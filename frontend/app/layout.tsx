import type { Metadata } from 'next';
import '../src/index.css';

export const metadata: Metadata = {
  title: 'Synthify',
  description: 'ドキュメントから知識グラフを生成・探索するシステム',
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ja">
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased">
        {children}
      </body>
    </html>
  );
}
