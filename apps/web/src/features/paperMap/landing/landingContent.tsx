import type { CSSProperties, ReactNode } from 'react';

/** Building blocks of the landing canvas's papers. They read the paper's CSS variables. */

const sectionStyle: CSSProperties = {
  display: 'grid',
  gap: 10,
  fontSize: '0.84rem',
  lineHeight: 1.65,
};

const eyebrowStyle: CSSProperties = {
  margin: 0,
  color: 'var(--accent)',
  fontSize: '0.66rem',
  fontWeight: 700,
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
};

const titleStyle: CSSProperties = {
  margin: 0,
  color: 'var(--text)',
  fontSize: '1rem',
  fontWeight: 700,
  letterSpacing: 0,
};

const panelStyle: CSSProperties = {
  border: '1px solid var(--soft-line)',
  borderRadius: 8,
  background:
    'linear-gradient(135deg, color-mix(in srgb, var(--panel) 92%, white), color-mix(in srgb, var(--link-bg) 56%, white))',
  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.72)',
};

/**
 * Renders a titled section with an uppercase category eyebrow within a landing paper card.
 */
export function PaperSection({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string;
  title: string;
  children: ReactNode;
}) {
  return (
    <section style={sectionStyle}>
      <div>
        <p style={eyebrowStyle}>{eyebrow}</p>
        <h2 style={titleStyle}>{title}</h2>
      </div>
      {children}
    </section>
  );
}

/**
 * Clickable card element that navigates to a child or related paper node on the canvas.
 */
export function PaperLinkCard({
  id,
  title,
  body,
  hue,
}: {
  id: string;
  title: string;
  body: string;
  hue?: number;
}) {
  const accent = hue === undefined ? 'var(--accent)' : `hsl(${hue}, 68%, 45%)`;
  const border = hue === undefined ? 'var(--link-border)' : `hsl(${hue}, 42%, 82%)`;
  const background = hue === undefined
    ? 'linear-gradient(135deg, color-mix(in srgb, var(--link-bg) 78%, white), color-mix(in srgb, var(--surface) 82%, white))'
    : `linear-gradient(135deg, hsl(${hue}, 72%, 96%), hsl(${hue}, 58%, 99%))`;

  return (
    <a
      data-paper-id={id}
      tabIndex={0}
      style={{
        display: 'grid',
        gap: 5,
        padding: '10px 11px',
        border: `1px solid ${border}`,
        borderRadius: 8,
        background,
        color: 'var(--text)',
        cursor: 'pointer',
        textDecoration: 'none',
        boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.74)',
      }}
    >
      <span style={{ display: 'flex', alignItems: 'center', gap: 6, color: accent, fontSize: '0.76rem', fontWeight: 700 }}>
        <span style={{ width: 6, height: 6, borderRadius: 99, background: 'currentColor', flexShrink: 0 }} />
        {title}
      </span>
      <span style={{ color: 'var(--muted)', fontSize: '0.76rem', lineHeight: 1.45 }}>{body}</span>
    </a>
  );
}

/**
 * Renders a plain unordered list of summary or feature bullet points.
 */
export function PlainList({ items }: { items: string[] }) {
  return (
    <ul style={{ margin: 0, paddingLeft: 17, display: 'grid', gap: 5 }}>
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ul>
  );
}

/**
 * Renders a compact key-value metric line with soft borders.
 */
export function MetricRow({ label, value }: { label: string; value: string }) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '86px minmax(0, 1fr)',
        gap: 8,
        padding: '6px 0',
        borderBottom: '1px solid var(--soft-line)',
      }}
    >
      <span style={{ color: 'var(--muted)', fontSize: '0.75rem' }}>{label}</span>
      <span style={{ fontSize: '0.8rem' }}>{value}</span>
    </div>
  );
}

/**
 * Renders a highlighted overview card with high-level description and three key metrics.
 */
export function HeroPanel({
  title,
  body,
  metrics,
}: {
  title: string;
  body: string;
  metrics: { label: string; value: string }[];
}) {
  return (
    <div
      style={{
        ...panelStyle,
        display: 'grid',
        gap: 12,
        padding: 14,
        background:
          'radial-gradient(circle at 8% 0%, color-mix(in srgb, var(--accent) 24%, transparent), transparent 38%), linear-gradient(135deg, color-mix(in srgb, var(--surface-raised) 92%, white), var(--surface))',
      }}
    >
      <div style={{ display: 'grid', gap: 5 }}>
        <h3 style={{ margin: 0, fontSize: '0.98rem', lineHeight: 1.35 }}>{title}</h3>
        <p style={{ margin: 0, color: 'var(--muted)', fontSize: '0.8rem', lineHeight: 1.6 }}>{body}</p>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 7 }}>
        {metrics.map((metric) => (
          <div key={metric.label} style={{ borderLeft: '2px solid var(--accent)', paddingLeft: 8, minWidth: 0 }}>
            <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.68rem', fontWeight: 700 }}>{metric.label}</p>
            <p style={{ margin: '2px 0 0', color: 'var(--text)', fontSize: '0.76rem', lineHeight: 1.35 }}>{metric.value}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Renders an ordered sequential step-by-step pipeline or workflow rail.
 */
export function StageRail({ stages }: { stages: { label: string; title: string; body: string }[] }) {
  return (
    <div style={{ display: 'grid', gap: 7 }}>
      {stages.map((stage, index) => (
        <div
          key={stage.label}
          style={{
            ...panelStyle,
            display: 'grid',
            gridTemplateColumns: '34px minmax(0, 1fr)',
            gap: 9,
            padding: '9px 10px',
            alignItems: 'start',
          }}
        >
          <div
            style={{
              display: 'grid',
              placeItems: 'center',
              width: 28,
              height: 28,
              borderRadius: 7,
              background: 'color-mix(in srgb, var(--accent) 15%, var(--surface))',
              color: 'var(--accent)',
              fontSize: '0.68rem',
              fontWeight: 800,
            }}
          >
            {String(index + 1).padStart(2, '0')}
          </div>
          <div>
            <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.68rem', fontWeight: 800, letterSpacing: '0.06em', textTransform: 'uppercase' }}>{stage.label}</p>
            <p style={{ margin: '1px 0 0', fontSize: '0.82rem', fontWeight: 700 }}>{stage.title}</p>
            <p style={{ margin: '2px 0 0', color: 'var(--muted)', fontSize: '0.75rem', lineHeight: 1.45 }}>{stage.body}</p>
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * Renders a responsive grid of indicator or value signals.
 */
export function SignalGrid({ items }: { items: { label: string; value: string }[] }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(118px, 1fr))', gap: 7 }}>
      {items.map((item) => (
        <div key={item.label} style={{ ...panelStyle, padding: '9px 10px' }}>
          <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.68rem', fontWeight: 800 }}>{item.label}</p>
          <p style={{ margin: '4px 0 0', color: 'var(--text)', fontSize: '0.78rem', lineHeight: 1.45 }}>{item.value}</p>
        </div>
      ))}
    </div>
  );
}

/**
 * Illustrative mockup panel displaying the anatomy of a paper node (assertion, evidence, counter, link).
 */
export function NodePreview() {
  return (
    <div style={{ ...panelStyle, padding: 12, display: 'grid', gap: 9 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <span style={{ color: 'var(--accent)', fontSize: '0.72rem', fontWeight: 800 }}>paper node</span>
        <span style={{ color: 'var(--muted)', fontSize: '0.68rem' }}>見出し / 根拠 / つながり</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 7 }}>
        {[
          ['主張', '市場変化の仮説'],
          ['根拠', '資料の該当箇所'],
          ['反論', '前提条件の制約'],
          ['関連', 'supports / measured_by'],
        ].map(([label, value]) => (
          <div key={label} style={{ border: '1px solid var(--soft-line)', borderRadius: 6, padding: '7px 8px', background: 'color-mix(in srgb, var(--surface) 78%, white)' }}>
            <p style={{ margin: 0, color: 'var(--accent)', fontSize: '0.66rem', fontWeight: 800 }}>{label}</p>
            <p style={{ margin: '2px 0 0', fontSize: '0.74rem', color: 'var(--muted)', lineHeight: 1.35 }}>{value}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
