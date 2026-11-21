import type { ReactNode } from 'react';
import {
  formatLegalDate,
  LEGAL_TODO_PATTERN,
  type LegalArticle,
  type LegalClause,
  type LegalDocument,
} from './legalDocument';
import { LegalFooter } from './LegalLinks';

// A server-renderable page: no hooks and no event handlers, so a static export writes the
// whole document into the HTML.

/** The text with every 【要確認…】 placeholder highlighted. */
function withPlaceholders(text: string): ReactNode[] {
  const parts: ReactNode[] = [];
  let last = 0;
  for (const match of text.matchAll(LEGAL_TODO_PATTERN)) {
    const start = match.index ?? 0;
    if (start > last) parts.push(text.slice(last, start));
    parts.push(
      <mark key={start} className="rounded-sm bg-amber-200/80 px-0.5 text-stone-900">
        {match[0]}
      </mark>,
    );
    last = start + match[0].length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

const EMAIL = /^[^\s@]+@[^\s@]+$/;

function RowValue({ value }: { value: string }) {
  if (EMAIL.test(value)) {
    return (
      <a href={`mailto:${value}`} className="text-indigo-600 underline-offset-2 hover:underline">
        {value}
      </a>
    );
  }
  return <>{withPlaceholders(value)}</>;
}

function ClauseBody({ clause }: { clause: LegalClause }) {
  if (typeof clause === 'string') return <>{withPlaceholders(clause)}</>;
  return (
    <>
      {withPlaceholders(clause.text)}
      {'items' in clause ? (
        <ol className="mt-2 space-y-1.5">
          {clause.items.map((item, index) => (
            <li key={index} className="flex gap-1">
              <span className="shrink-0 tabular-nums text-stone-500">（{index + 1}）</span>
              <span className="min-w-0">{withPlaceholders(item)}</span>
            </li>
          ))}
        </ol>
      ) : (
        <dl className="mt-3 divide-y divide-stone-200 rounded-lg border border-stone-200 bg-white/70 text-sm leading-7">
          {clause.rows.map(([label, value]) => (
            <div key={label} className="grid gap-x-4 px-4 py-2.5 sm:grid-cols-[9rem_1fr]">
              <dt className="font-medium text-stone-500">{label}</dt>
              <dd className="min-w-0 break-words">
                <RowValue value={value} />
              </dd>
            </div>
          ))}
        </dl>
      )}
    </>
  );
}

function Article({ article, number }: { article: LegalArticle; number: number }) {
  const headingId = `article-${number}-title`;
  return (
    <section id={`article-${number}`} aria-labelledby={headingId} className="scroll-mt-6">
      <h2 id={headingId} className="text-base font-semibold text-stone-900 sm:text-lg">
        第{number}条（{article.title}）
      </h2>
      {article.clauses.length === 1 ? (
        <div className="mt-2">
          <ClauseBody clause={article.clauses[0]} />
        </div>
      ) : (
        <ol className="mt-2 space-y-2">
          {article.clauses.map((clause, index) => (
            <li key={index} className="flex gap-2">
              <span className="shrink-0 tabular-nums text-stone-500">{index + 1}</span>
              <div className="min-w-0">
                <ClauseBody clause={clause} />
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

function TableOfContents({ articles }: { articles: LegalArticle[] }) {
  return (
    <nav aria-labelledby="legal-toc-title" className="mt-8 rounded-lg border border-stone-200 bg-white/60 px-4 py-3 text-sm">
      <p id="legal-toc-title" className="font-semibold text-stone-700">
        目次
      </p>
      <ol className="mt-2 grid gap-x-6 gap-y-1 sm:grid-cols-2">
        {articles.map((article, index) => (
          <li key={article.id}>
            <a href={`#article-${index + 1}`} className="text-stone-600 underline-offset-2 hover:text-stone-900 hover:underline">
              <span className="mr-2 tabular-nums">第{index + 1}条</span>
              {article.title}
            </a>
          </li>
        ))}
      </ol>
    </nav>
  );
}

/**
 * The terms of service or the privacy policy as a page of its own.
 *
 * @param content the text, with article references already resolved
 * @param effectiveDate YYYY-MM-DD, or null while undecided (shown as a placeholder)
 * @param current which of the two documents this is, for the footer links
 */
export function LegalDocumentPage({
  content,
  effectiveDate,
  current,
}: {
  content: LegalDocument;
  effectiveDate: string | null;
  current: 'terms' | 'privacy';
}) {
  return (
    <div className="min-h-screen bg-[#faf6ef] text-stone-800">
      <header className="border-b border-stone-200/80 bg-white/60">
        <div className="mx-auto max-w-3xl px-4 py-3 sm:px-6">
          <a href="/" className="text-sm font-semibold tracking-tight text-stone-800 hover:text-indigo-600">
            Synthify
          </a>
        </div>
      </header>

      <main className="mx-auto max-w-3xl px-4 pb-16 pt-10 sm:px-6">
        <h1 className="text-2xl font-bold tracking-tight text-stone-900 sm:text-3xl">{content.title}</h1>
        <p className="mt-2 text-sm text-stone-500">制定日：{withPlaceholders(formatLegalDate(effectiveDate))}</p>

        <div className="mt-6 space-y-4 leading-8">
          {content.lead.map((paragraph, index) => (
            <p key={index}>{withPlaceholders(paragraph)}</p>
          ))}
        </div>

        <TableOfContents articles={content.articles} />

        <div className="mt-10 space-y-10 leading-8">
          {content.articles.map((article, index) => (
            <Article key={article.id} article={article} number={index + 1} />
          ))}
        </div>

        <p className="mt-12 text-right text-sm text-stone-500">以上</p>
      </main>

      <LegalFooter current={current} className="border-t border-stone-200/80 px-4 py-6" />
    </div>
  );
}
