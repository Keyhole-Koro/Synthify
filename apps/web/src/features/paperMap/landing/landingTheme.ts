import type { CSSProperties } from 'react';

/** Hues of the landing canvas's paper groups. */
export const CATEGORY_HUES = {
  auth: 285,
  about: 212,
  overview: 220,
  documents: 175,
  worker: 292,
  tree: 132,
  operations: 24,
  workspaces: 202,
  billing: 48,
} as const;

/** A responsive grid of link cards. */
export const cardGridStyle: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(148px, 1fr))',
  gap: 8,
};
