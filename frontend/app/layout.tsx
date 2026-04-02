import type { Metadata } from "next";
import "../src/index.css";

export const metadata: Metadata = {
  title: "Synthify Frontend",
  description: "Next.js frontend starter for Synthify"
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ja">
      <body>{children}</body>
    </html>
  );
}
