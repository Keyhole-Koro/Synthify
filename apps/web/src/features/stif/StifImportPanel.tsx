'use client';

import React, { useCallback, useMemo, useRef, useState } from 'react';
import { InlineError } from '@/components/error/InlineError';
import { toAppError } from '@/lib/error_normalize';
import {
  commitStifImport,
  hasBlockingErrors,
  previewStifImport,
  StifDiagnosticSeverity,
  type StifDiagnostic,
  type StifParsedItem,
} from './api';

interface StifImportPanelProps {
  workspaceId: string;
  onClose: () => void;
  // onImported runs after a successful commit so the caller can refresh the
  // tree; the panel does not own the tree cache.
  onImported: (importedCount: number) => Promise<void> | void;
}

// The panel walks one line: paste or drop a document, look at what it would
// create, then commit. The preview is a server dry run rather than a local
// parse, so "what you reviewed" and "what got written" cannot disagree.
export function StifImportPanel({ workspaceId, onClose, onImported }: StifImportPanelProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [source, setSource] = useState('');
  const [filename, setFilename] = useState('');
  const [items, setItems] = useState<StifParsedItem[]>([]);
  const [diagnostics, setDiagnostics] = useState<StifDiagnostic[]>([]);
  const [previewed, setPreviewed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState('');
  const [importedCount, setImportedCount] = useState<number | null>(null);

  const errors = useMemo(() => diagnostics.filter((d) => d.severity === StifDiagnosticSeverity.ERROR), [diagnostics]);
  const warnings = useMemo(() => diagnostics.filter((d) => d.severity === StifDiagnosticSeverity.WARNING), [diagnostics]);
  const blocked = hasBlockingErrors(diagnostics);
  const canPreview = source.trim().length > 0 && !busy;
  const canImport = previewed && !blocked && items.length > 0 && !busy;

  // Any edit invalidates the preview: the items on screen no longer describe
  // the text in the box, and importing from a stale review is the one mistake
  // this panel exists to prevent.
  const resetPreview = useCallback(() => {
    setPreviewed(false);
    setItems([]);
    setDiagnostics([]);
    setImportedCount(null);
  }, []);

  const updateSource = useCallback((next: string, name = '') => {
    setSource(next);
    setFilename(name);
    setError('');
    resetPreview();
  }, [resetPreview]);

  const readFile = useCallback(async (file: File) => {
    try {
      updateSource(await file.text(), file.name);
    } catch (err) {
      setError(toAppError(err).message || 'ファイルを読み込めませんでした。');
    }
  }, [updateSource]);

  const handlePreview = useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      const response = await previewStifImport(workspaceId, source, filename);
      setItems(response.items);
      setDiagnostics(response.diagnostics);
      setPreviewed(true);
    } catch (err) {
      setError(toAppError(err).message || 'プレビューに失敗しました。');
    } finally {
      setBusy(false);
    }
  }, [workspaceId, source, filename]);

  const handleImport = useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      const response = await commitStifImport(workspaceId, source, filename);
      setItems(response.items);
      setDiagnostics(response.diagnostics);
      if (!response.committed) {
        // The server re-parses on commit, so it can still refuse.
        setError('インポートできませんでした。エラーを確認してください。');
        return;
      }
      setImportedCount(response.importedCount);
      await onImported(response.importedCount);
    } catch (err) {
      setError(toAppError(err).message || 'インポートに失敗しました。');
    } finally {
      setBusy(false);
    }
  }, [workspaceId, source, filename, onImported]);

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files[0];
    if (file) await readFile(file);
  }, [readFile]);

  return (
    <div className="border-t border-stone-100 px-5 py-4 text-sm">
      <div className="mb-1 flex items-center justify-between">
        <h3 className="font-semibold text-stone-800">STIF をインポート</h3>
        <button type="button" onClick={onClose} className="text-[11px] text-stone-400 hover:text-stone-600">
          閉じる
        </button>
      </div>
      <p className="mb-3 text-[11px] text-stone-400">
        LLM との会話や設計メモを <code className="rounded bg-stone-100 px-1">.stif</code> に変換して、
        このワークスペースのツリーに追加します。
      </p>

      {error && <InlineError message={error} className="mb-2" />}

      <input
        ref={fileInputRef}
        type="file"
        accept=".stif,text/plain"
        className="sr-only"
        onChange={(e) => {
          const file = e.target.files?.[0];
          e.target.value = '';
          if (file) void readFile(file);
        }}
      />

      <div
        onDragOver={(e) => {
          e.preventDefault();
          setIsDragging(true);
        }}
        onDragLeave={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node)) setIsDragging(false);
        }}
        onDrop={handleDrop}
        className={[
          'rounded-lg border-2 border-dashed transition-colors',
          isDragging ? 'border-indigo-400 bg-indigo-50/60' : 'border-stone-200',
        ].join(' ')}
      >
        <textarea
          aria-label="STIF ドキュメント"
          value={source}
          onChange={(e) => updateSource(e.target.value, filename)}
          disabled={busy}
          spellCheck={false}
          rows={8}
          placeholder={'#STIF/1\n@tree mode="append" scope="canonical"\n\n::item /overview\ntitle: "Overview"\n---\n<p>...</p>\n::end'}
          className="w-full resize-y rounded-lg bg-transparent px-3 py-2 font-mono text-[12px] leading-relaxed text-stone-700 outline-none placeholder:text-stone-300 disabled:opacity-60"
        />
      </div>

      <div className="mt-2 flex items-center gap-2">
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          className="rounded-md border border-stone-200 bg-white px-2.5 py-1 text-[12px] text-stone-500 transition-colors hover:border-indigo-300 hover:text-indigo-500 disabled:opacity-60"
        >
          ファイルを選択
        </button>
        {filename && <span className="min-w-0 truncate text-[11px] text-stone-400">{filename}</span>}
        <div className="flex-1" />
        <button
          type="button"
          onClick={() => void handlePreview()}
          disabled={!canPreview}
          className="rounded-md border border-stone-200 bg-white px-3 py-1 text-[12px] font-medium text-stone-600 transition-colors hover:border-indigo-300 hover:text-indigo-500 disabled:opacity-50"
        >
          {busy && !previewed ? '確認中…' : 'プレビュー'}
        </button>
        <button
          type="button"
          onClick={() => void handleImport()}
          disabled={!canImport}
          className="rounded-md bg-indigo-500 px-3 py-1 text-[12px] font-medium text-white transition-colors hover:bg-indigo-600 disabled:opacity-50"
        >
          {items.length > 0 ? `${items.length} 件をインポート` : 'インポート'}
        </button>
      </div>

      {importedCount !== null && (
        <p className="mt-3 text-[12px] font-medium text-emerald-600">
          {importedCount} 件のアイテムをツリーに追加しました。
        </p>
      )}

      {previewed && importedCount === null && (
        <div className="mt-3">
          {blocked ? (
            <p className="text-[12px] font-medium text-red-600">
              エラーがあるためインポートできません。修正してからもう一度プレビューしてください。
            </p>
          ) : items.length === 0 ? (
            <p className="text-[12px] text-stone-400">追加できるアイテムがありません。</p>
          ) : (
            <StifItemPreview items={items} />
          )}
        </div>
      )}

      {(errors.length > 0 || warnings.length > 0) && (
        <DiagnosticList errors={errors} warnings={warnings} />
      )}
    </div>
  );
}

// StifItemPreview shows the tree the import would build. Indentation comes from
// the item's depth, which the server already resolved, so the preview reflects
// the real parent links rather than the order of blocks in the file.
function StifItemPreview({ items }: { items: StifParsedItem[] }) {
  return (
    <div>
      <h4 className="mb-1.5 text-[11px] font-semibold text-stone-600">
        追加されるアイテム ({items.length})
      </h4>
      <ul
        data-testid="stif-preview-items"
        className="max-h-56 overflow-y-auto rounded-md border border-stone-100 bg-stone-50/60 py-1"
      >
        {items.map((item) => (
          <li
            key={item.localPath}
            className="flex items-baseline gap-2 px-2 py-1 text-[12px]"
            style={{ paddingLeft: `${8 + item.depth * 14}px` }}
          >
            <span className="truncate font-medium text-stone-700">{item.title || '(無題)'}</span>
            <span className="truncate font-mono text-[10px] text-stone-400">{item.localPath}</span>
            {item.sources.length > 0 && (
              <span className="shrink-0 rounded bg-stone-200/70 px-1 text-[10px] text-stone-500">
                出典 {item.sources.length}
              </span>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

function DiagnosticList({ errors, warnings }: { errors: StifDiagnostic[]; warnings: StifDiagnostic[] }) {
  return (
    <div className="mt-3 space-y-1">
      {errors.map((d, i) => <DiagnosticRow key={`e-${i}`} diagnostic={d} isError />)}
      {warnings.map((d, i) => <DiagnosticRow key={`w-${i}`} diagnostic={d} isError={false} />)}
    </div>
  );
}

function DiagnosticRow({ diagnostic, isError }: { diagnostic: StifDiagnostic; isError: boolean }) {
  return (
    <div
      className={[
        'flex items-baseline gap-2 rounded-md px-2 py-1 text-[11px]',
        isError ? 'bg-red-50 text-red-700' : 'bg-amber-50 text-amber-700',
      ].join(' ')}
    >
      {diagnostic.line > 0 && (
        <span className="shrink-0 font-mono opacity-70">L{diagnostic.line}</span>
      )}
      <span className="min-w-0 flex-1">{diagnostic.message}</span>
      {diagnostic.localPath && (
        <span className="shrink-0 truncate font-mono opacity-70">{diagnostic.localPath}</span>
      )}
    </div>
  );
}
