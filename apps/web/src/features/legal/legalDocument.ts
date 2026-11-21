/**
 * The shape of the terms of service and the privacy policy, and the helpers that
 * number their articles. The text itself lives in termsOfService.ts and privacyPolicy.ts.
 *
 * Kept free of Next.js and React imports so the documents and their tests survive the
 * move to another router unchanged.
 */

/** Where the two documents are served. Plain paths, linked with <a>, so any router works. */
export const TERMS_PATH = '/terms';
export const PRIVACY_PATH = '/privacy';

/**
 * One paragraph (項) of an article.
 *
 * - a string is a plain paragraph
 * - `items` adds numbered items (号) under the paragraph's text
 * - `rows` adds a label/value table under the paragraph's text (the contact details)
 */
export type LegalClause =
  | string
  | { text: string; items: string[] }
  | { text: string; rows: ReadonlyArray<readonly [label: string, value: string]> };

/**
 * One article (条). Articles are numbered by their position, so `id` is what the text
 * refers to: write `{{article:<id>}}` and {@link resolveArticleRefs} turns it into 「第n条」.
 */
export type LegalArticle = { id: string; title: string; clauses: LegalClause[] };

export type LegalDocument = {
  title: string;
  /** Paragraphs before the first article. */
  lead: string[];
  articles: LegalArticle[];
};

/** The mark for anything the operator still has to decide or confirm. */
export const LEGAL_TODO = '【要確認】';

/** Matches every placeholder: 【要確認】 and 【要確認：what to fill in】. */
export const LEGAL_TODO_PATTERN = /【要確認(?:：[^】]*)?】/g;

/** A placeholder naming what is missing, e.g. todo('運営者の名称') → 【要確認：運営者の名称】. */
export function todo(what: string): string {
  return `【要確認：${what}】`;
}

/** The value, or a placeholder naming it when it is not decided yet (null). */
export function orTodo(value: string | null, what: string): string {
  return value ?? todo(what);
}

/** "2026-12-01" → 「2026年12月1日」; null → a placeholder. */
export function formatLegalDate(date: string | null): string {
  if (!date) return todo('制定日');
  const [year, month, day] = date.split('-').map(Number);
  return `${year}年${month}月${day}日`;
}

const ARTICLE_REF = /\{\{article:([\w-]+)\}\}/g;

/**
 * Replaces every `{{article:<id>}}` with the article's number (「第n条」).
 *
 * Throws on a duplicate id or a reference to an id that does not exist, so a reordered or
 * renamed article cannot leave a wrong number in the text: the build and the tests fail.
 */
export function resolveArticleRefs(document: LegalDocument): LegalDocument {
  const numbers = new Map<string, number>();
  document.articles.forEach((article, index) => {
    if (numbers.has(article.id)) throw new Error(`duplicate legal article id: ${article.id}`);
    numbers.set(article.id, index + 1);
  });

  const resolve = (text: string) =>
    text.replace(ARTICLE_REF, (_, id: string) => {
      const number = numbers.get(id);
      if (number === undefined) throw new Error(`unknown legal article reference: ${id}`);
      return `第${number}条`;
    });

  const resolveClause = (clause: LegalClause): LegalClause => {
    if (typeof clause === 'string') return resolve(clause);
    if ('items' in clause) return { text: resolve(clause.text), items: clause.items.map(resolve) };
    return { text: resolve(clause.text), rows: clause.rows.map(([label, value]) => [label, resolve(value)] as const) };
  };

  return {
    title: document.title,
    lead: document.lead.map(resolve),
    articles: document.articles.map((article) => ({
      id: article.id,
      title: resolve(article.title),
      clauses: article.clauses.map(resolveClause),
    })),
  };
}

/** Every string a reader sees in the document, in order. */
export function documentTexts(document: LegalDocument): string[] {
  return [
    document.title,
    ...document.lead,
    ...document.articles.flatMap((article) => [
      article.title,
      ...article.clauses.flatMap((clause) => {
        if (typeof clause === 'string') return [clause];
        if ('items' in clause) return [clause.text, ...clause.items];
        return [clause.text, ...clause.rows.flat()];
      }),
    ]),
  ];
}

/** The distinct placeholders left in the document, in order of appearance. */
export function listPlaceholders(document: LegalDocument): string[] {
  const found = documentTexts(document).flatMap((text) => text.match(LEGAL_TODO_PATTERN) ?? []);
  return [...new Set(found)];
}
