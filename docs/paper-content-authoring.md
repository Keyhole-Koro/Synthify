# Paper Content Authoring

このドキュメントは、このリポジトリにおける paper-in-paper コンテンツの現行の authoring モデルを定義する。

## Scope

このドキュメントは、paper-in-paper コンテンツの `content` フィールドに渡す値に適用する。

現時点で確認されている使用箇所:

- `frontend/src/features/landing/landingPaperMap.tsx`

今後 paper-in-paper 用に authoring されるコンテンツも、このドキュメントでルールが改訂されない限り、同じルールに従うこと。

## Goal

Paper コンテンツは、まず読みやすさを優先して authoring するべきである。

作者がコンテンツ定義を、低レベルな構文木ではなく、コンテンツそのものとして読める必要がある。

これは次の両方にとって重要である:

- 人間による編集
- LLM 支援による生成

## Current Model

コンテンツは **JSX** で authoring する。

`Paper.content` の型は `string | ReactNode | ContentNode[]` であり、現在は `ReactNode`（JSX）を使う。

### CSS 変数

`PaperContentReact` レンダラーがコンテナに以下の CSS 変数を注入する。コンテンツ内のスタイルはこれを使うこと。

| 変数 | 用途 |
|---|---|
| `--text` | 本文テキスト色 |
| `--muted` | 補助テキスト・セル色 |
| `--accent` | リンク・強調色 |
| `--link-bg` | リンク背景色 |
| `--link-border` | リンク枠色 |
| `--surface` | ベース背景色 |
| `--surface-alt` | 交互行・callout 背景 |
| `--surface-raised` | テーブルヘッダー背景 |
| `--line` | 区切り線・callout ボーダー |

コンテナ自体に `color: theme.text` が設定されるため、テキスト要素への明示的な `color` 指定は不要。`--muted` など異なる色が必要な場合のみ指定する。

### paper-in-paper 固有の操作

クリックすると対象 paper を inline 展開する操作は、`data-paper-id` 属性で表現する。

```tsx
<a data-paper-id="graph">知識グラフ</a>
```

クリックハンドラーはレンダラー側が自動的に処理する。authoring 側は `data-paper-id` を書くだけでよい。

## Primitives

### PaperLink (`PL`)

paper 展開リンク。各 authoring ファイルにローカル定義する。
`usePaperStore` を使って `paperMap` から title・description を自動取得する。

```tsx
import { usePaperStore } from '@keyhole-koro/paper-in-paper';

function PL({ id, children, variant }: { id: string; children?: React.ReactNode; variant?: 'card' }) {
  const { state } = usePaperStore();
  const paper = state.paperMap.get(id);

  if (variant === 'card') {
    return (
      <a data-paper-id={id} tabIndex={0} style={{ display: 'block', border: '1px solid var(--link-border)', borderRadius: 8, padding: '10px 12px', background: 'var(--link-bg)', cursor: 'pointer', textDecoration: 'none' }}>
        <p style={{ margin: '0 0 4px', fontSize: '0.7rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em', color: 'var(--accent)' }}>
          {paper?.title ?? id}
        </p>
        <p style={{ margin: 0, fontSize: '0.78rem', lineHeight: 1.55 }}>
          {paper?.description}
        </p>
      </a>
    );
  }

  return (
    <a data-paper-id={id} tabIndex={0} style={{ color: 'var(--accent)', background: 'var(--link-bg)', border: '1px solid var(--link-border)', borderRadius: 4, padding: '1px 5px', cursor: 'pointer', textDecoration: 'none', display: 'inline', fontSize: 'inherit' }}>
      {children ?? paper?.title ?? id}
    </a>
  );
}
```

**3つのバリアント:**

```tsx
// chip — paper.title を自動表示。ラベルが paper title と同じ場合に使う
<PL id="graph" />

// inline — children をラベルとして文中に埋め込む。ラベルが paper title と異なる場合
<PL id="extraction">AIが概念・主張・根拠を抽出</PL>

// card — paper.title + paper.description をブロック表示
<PL id="auth" variant="card" />
```

### Section

```tsx
<section>
  <h2 style={{ margin: '0 0 8px', fontSize: '1rem' }}>タイトル</h2>
  <div style={{ display: 'grid', gap: 8 }}>
    {/* children */}
  </div>
</section>
```

### Paragraph

```tsx
<p style={{ margin: 0, lineHeight: 1.65, fontSize: '0.85rem' }}>
  テキスト。<PL id="other">リンク</PL>も inline で埋め込める。
</p>
```

### List

```tsx
<ul style={{ margin: 0, paddingLeft: 16, lineHeight: 1.8, fontSize: '0.85rem' }}>
  <li>プレーンテキスト項目</li>
  <li><strong>bold</strong> - 説明</li>
  <li><PL id="canonicalization">リンク項目</PL></li>
</ul>
```

### Table

```tsx
<table style={{ width: '100%', fontSize: '0.75rem', borderCollapse: 'collapse' }}>
  <thead>
    <tr style={{ background: 'var(--surface-raised)' }}>
      <th style={{ padding: '6px 8px', textAlign: 'left' }}>列A</th>
      <th style={{ padding: '6px 8px', textAlign: 'left' }}>列B</th>
    </tr>
  </thead>
  <tbody>
    {rows.map(([a, b], i) => (
      <tr key={a} style={{ background: i % 2 === 1 ? 'var(--surface-alt)' : 'transparent' }}>
        <td style={{ padding: '5px 8px' }}>{a}</td>
        <td style={{ padding: '5px 8px', color: 'var(--muted)' }}>{b}</td>
      </tr>
    ))}
  </tbody>
</table>
```

### Callout

```tsx
<div style={{
  borderLeft: '3px solid var(--line)',
  background: 'var(--surface-alt)',
  borderRadius: 4,
  padding: '8px 12px',
  fontSize: '0.8rem',
  color: 'var(--muted)',
  lineHeight: 1.6,
}}>
  callout テキスト
</div>
```


## Authoring Principles

- コンテンツの読みやすさを、構造上の巧妙さより優先する
- paper 間リンクは `data-paper-id` 属性で表現する（`PL` コンポーネント、またはインライン `<a data-paper-id="...">` 直書き）
- スタイルは CSS 変数（`--text`, `--accent` 等）で参照し、ハードコードしない
- 新しいヘルパーは、実際に authoring の friction を減らす場合にのみ導入する

## LLM による生成

LLM にコンテンツを生成させる場合、以下だけ伝えれば十分:

- コンテンツは JSX で書く
- paper 間リンクは `PL` コンポーネントで表現する（クリックで展開）
  - `<PL id="graph" />` — paper title を自動表示（chip）
  - `<PL id="graph">文脈に合ったラベル</PL>` — カスタムラベル（inline）
  - `<PL id="auth" variant="card" />` — ブロックカード
- スタイルは `var(--text)`, `var(--accent)`, `var(--muted)` 等の CSS 変数を使う
- それ以外は普通の HTML/JSX

## Non-Goals

- 汎用 markdown parser
- schema compiler
- アプリ全体向けの汎用コンテンツモデル
