import type { Metadata } from 'next';
import { Noto_Sans_JP } from 'next/font/google';
import '../src/index.css';

const appFont = Noto_Sans_JP({ subsets: ['latin'], variable: '--font-space' });

export const metadata: Metadata = {
  title: 'Synthify',
  description: 'ドキュメントから知識ツリーを生成・探索するシステム',
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ja" className={appFont.variable}>
      <body className={`${appFont.className} min-h-screen bg-slate-50 text-slate-900 antialiased`}>
        {children}
      </body>
    </html>
  );
}
