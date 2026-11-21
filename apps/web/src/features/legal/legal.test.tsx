import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import {
  documentTexts,
  formatLegalDate,
  LEGAL_TODO,
  LEGAL_TODO_PATTERN,
  listPlaceholders,
  orTodo,
  PRIVACY_PATH,
  resolveArticleRefs,
  TERMS_PATH,
  todo,
  type LegalDocument,
} from './legalDocument';
import { LegalDocumentPage } from './LegalDocumentPage';
import { LegalFooter, SignInLegalNotice } from './LegalLinks';
import { COURT, OPERATOR } from './operator';
import { PRIVACY_POLICY } from './privacyPolicy';
import { TERMS_OF_SERVICE } from './termsOfService';

afterEach(cleanup);

const DOCUMENTS = { 利用規約: TERMS_OF_SERVICE, プライバシーポリシー: PRIVACY_POLICY };

describe('resolveArticleRefs', () => {
  const source: LegalDocument = {
    title: 'テスト',
    lead: ['{{article:second}}を見てください。'],
    articles: [
      { id: 'first', title: '最初', clauses: [{ text: '{{article:second}}第1項', items: ['{{article:first}}の号'] }] },
      { id: 'second', title: '次', clauses: [{ text: '窓口', rows: [['参照', '{{article:first}}']] }] },
    ],
  };

  it('numbers references by the position of the article', () => {
    const resolved = resolveArticleRefs(source);
    expect(resolved.lead).toEqual(['第2条を見てください。']);
    expect(resolved.articles[0].clauses[0]).toEqual({ text: '第2条第1項', items: ['第1条の号'] });
    expect(resolved.articles[1].clauses[0]).toEqual({ text: '窓口', rows: [['参照', '第1条']] });

    const reordered = resolveArticleRefs({ ...source, articles: [...source.articles].reverse() });
    expect(reordered.lead).toEqual(['第1条を見てください。']);
  });

  it('rejects a reference to an article that does not exist', () => {
    expect(() => resolveArticleRefs({ ...source, lead: ['{{article:missing}}'] })).toThrow(/missing/);
  });

  it('rejects two articles with the same id', () => {
    expect(() => resolveArticleRefs({ ...source, articles: [source.articles[0], source.articles[0]] })).toThrow(/duplicate/);
  });
});

describe('placeholders', () => {
  it('names what is missing', () => {
    expect(todo('運営者の名称')).toBe('【要確認：運営者の名称】');
    expect(orTodo(null, '所在地')).toBe('【要確認：所在地】');
    expect(orTodo('東京都', '所在地')).toBe('東京都');
    expect(formatLegalDate(null)).toBe('【要確認：制定日】');
    expect(formatLegalDate('2026-12-01')).toBe('2026年12月1日');
  });

  it('lists each placeholder once, in order', () => {
    const document: LegalDocument = {
      title: 'テスト',
      lead: [`${todo('A')}と${todo('B')}`],
      articles: [{ id: 'a', title: '条', clauses: [`${todo('A')}${LEGAL_TODO}`] }],
    };
    expect(listPlaceholders(document)).toEqual(['【要確認：A】', '【要確認：B】', LEGAL_TODO]);
  });
});

describe.each(Object.entries(DOCUMENTS))('%s', (_, document) => {
  it('leaves no unresolved article reference and no broken placeholder', () => {
    for (const text of documentTexts(document)) {
      expect(text).not.toMatch(/\{\{|\}\}/);
      // Every 【要確認 opens a complete placeholder, so the page can highlight it.
      const opened = text.split('【要確認').length - 1;
      expect(text.match(LEGAL_TODO_PATTERN) ?? []).toHaveLength(opened);
    }
  });

  it('names the operator, or shows that the name is still missing', () => {
    expect(document.lead[0]).toContain(OPERATOR.name ?? todo('運営者の名称'));
  });

  it('gives every article a title and at least one paragraph', () => {
    for (const article of document.articles) {
      expect(article.title).not.toBe('');
      expect(article.clauses.length).toBeGreaterThan(0);
    }
  });
});

describe('the documents', () => {
  it('tell the user how to reach the operator', () => {
    const contact = PRIVACY_POLICY.articles.at(-1)!;
    expect(contact.title).toBe('お問い合わせ窓口');
    const rows = (contact.clauses[0] as { rows: ReadonlyArray<readonly [string, string]> }).rows;
    expect(rows).toContainEqual(['お問い合わせ', OPERATOR.contactEmail ?? todo('お問い合わせ用のメールアドレス')]);
  });

  it('name the court for disputes, or show that it is still missing', () => {
    const law = TERMS_OF_SERVICE.articles.at(-1)!;
    expect(law.title).toBe('準拠法及び管轄裁判所');
    expect(law.clauses[1]).toContain(COURT ?? todo('管轄裁判所'));
  });

  it('do not promise that the paid plan continues', () => {
    const fees = TERMS_OF_SERVICE.articles.find((article) => article.title === '料金と支払')!;
    expect(fees.clauses.join('')).toContain('有料プランの提供を終了し');
  });
});

describe('LegalDocumentPage', () => {
  const content: LegalDocument = {
    title: 'テストの規約',
    lead: [`${todo('運営者の名称')}が定めます。`],
    articles: [
      { id: 'one', title: '一つ目', clauses: ['ただ一つの段落'] },
      {
        id: 'two',
        title: '二つ目',
        clauses: [
          { text: '次のとおりです。', items: ['最初の号', '次の号'] },
          { text: '窓口', rows: [['メール', 'help@example.com'], ['所在地', todo('所在地')]] },
        ],
      },
    ],
  };

  it('numbers the articles, links them from the contents and highlights the placeholders', () => {
    const { container } = render(<LegalDocumentPage content={content} effectiveDate={null} current="terms" />);

    expect(screen.getByRole('heading', { level: 1, name: 'テストの規約' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 2, name: '第1条（一つ目）' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 2, name: '第2条（二つ目）' })).toBeTruthy();
    expect(container.querySelector('#article-2')).not.toBeNull();

    const toc = screen.getByRole('navigation', { name: '目次' });
    expect(within(toc).getByRole('link', { name: /第2条\s*二つ目/ }).getAttribute('href')).toBe('#article-2');

    const marks = Array.from(container.querySelectorAll('mark'), (mark) => mark.textContent);
    expect(marks).toEqual(['【要確認：制定日】', '【要確認：運営者の名称】', '【要確認：所在地】']);

    expect(screen.getByRole('link', { name: 'help@example.com' }).getAttribute('href')).toBe('mailto:help@example.com');
    expect(screen.getByText('最初の号')).toBeTruthy();
  });

  it('ends with the links to both documents, marking the current one', () => {
    render(<LegalDocumentPage content={content} effectiveDate="2026-12-01" current="privacy" />);

    expect(screen.getByText('制定日：2026年12月1日')).toBeTruthy();
    const footer = screen.getByRole('contentinfo');
    expect(within(footer).getByRole('link', { name: '利用規約' }).getAttribute('aria-current')).toBeNull();
    expect(within(footer).getByRole('link', { name: 'プライバシーポリシー' }).getAttribute('aria-current')).toBe('page');
  });
});

describe('links', () => {
  it('point the footer at the two pages', () => {
    render(<LegalFooter />);
    expect(screen.getByRole('link', { name: '利用規約' }).getAttribute('href')).toBe(TERMS_PATH);
    expect(screen.getByRole('link', { name: 'プライバシーポリシー' }).getAttribute('href')).toBe(PRIVACY_PATH);
  });

  it('open the documents from the sign-in notice in a new tab', () => {
    render(<SignInLegalNotice />);
    for (const name of ['利用規約', 'プライバシーポリシー']) {
      const link = screen.getByRole('link', { name });
      expect(link.getAttribute('target')).toBe('_blank');
      expect(link.getAttribute('rel')).toBe('noopener noreferrer');
    }
    expect(screen.getByText(/に同意したものとみなします/)).toBeTruthy();
  });
});
