'use client';

import { useEffect, useRef, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { listDocuments, createDocument, startProcessing, type Document, type DocumentStatus } from '@/features/documents/api';
import { getWorkspace, type Workspace } from '@/features/workspaces/api';

const STATUS_LABEL: Record<DocumentStatus, string> = {
  uploaded: 'アップロード済',
  pending_normalization: '正規化待ち',
  processing: '処理中',
  completed: '完了',
  failed: 'エラー',
};

const STATUS_COLOR: Record<DocumentStatus, string> = {
  uploaded: 'bg-slate-100 text-slate-600',
  pending_normalization: 'bg-yellow-100 text-yellow-700',
  processing: 'bg-blue-100 text-blue-700',
  completed: 'bg-emerald-100 text-emerald-700',
  failed: 'bg-red-100 text-red-700',
};

function StatusBadge({ status }: { status: DocumentStatus }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${STATUS_COLOR[status]}`}>
      {status === 'processing' && (
        <span className="mr-1.5 h-1.5 w-1.5 rounded-full bg-blue-500 animate-pulse" />
      )}
      {STATUS_LABEL[status]}
    </span>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('ja-JP', { year: 'numeric', month: 'short', day: 'numeric' });
}

type ExtractionDepth = 'full' | 'summary';

interface UploadState {
  file: File;
  depth: ExtractionDepth;
  phase: 'idle' | 'creating' | 'uploading' | 'starting' | 'done' | 'error';
  progress: number;
  error?: string;
}

export default function WorkspacePage() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const router = useRouter();

  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [dragging, setDragging] = useState(false);
  const [upload, setUpload] = useState<UploadState | null>(null);
  const [defaultDepth, setDefaultDepth] = useState<ExtractionDepth>('full');
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    Promise.all([
      getWorkspace(workspaceId).then((r) => setWorkspace(r.workspace)),
      listDocuments(workspaceId).then(setDocuments),
    ])
      .catch((err) => {
        console.error(err);
        router.replace('/');
      })
      .finally(() => setLoading(false));
  }, [workspaceId, router]);

  // Poll processing documents
  useEffect(() => {
    const processing = documents.filter((d) => d.status === 'processing' || d.status === 'pending_normalization');
    if (processing.length === 0) return;
    const id = setInterval(() => {
      listDocuments(workspaceId).then(setDocuments).catch(console.error);
    }, 4000);
    return () => clearInterval(id);
  }, [documents, workspaceId]);

  async function handleFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    const file = files[0];
    setUpload({ file, depth: defaultDepth, phase: 'creating', progress: 0 });

    try {
      // 1. Create document record
      setUpload((u) => u && { ...u, phase: 'creating' });
      const { document, upload_url } = await createDocument(
        workspaceId,
        file.name,
        file.type || 'application/octet-stream',
        file.size,
      );

      // 2. Upload file
      setUpload((u) => u && { ...u, phase: 'uploading', progress: 0 });
      await uploadWithProgress(upload_url, file, (pct) =>
        setUpload((u) => u && { ...u, progress: pct }),
      );

      // 3. Start processing
      setUpload((u) => u && { ...u, phase: 'starting', progress: 100 });
      await startProcessing(document.document_id, upload?.depth ?? defaultDepth);

      setUpload((u) => u && { ...u, phase: 'done' });
      setDocuments((prev) => [{ ...document, status: 'processing' }, ...prev]);
      setTimeout(() => setUpload(null), 1500);
    } catch (err) {
      setUpload((u) => u && { ...u, phase: 'error', error: String(err) });
    }
  }

  function onDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragging(false);
    handleFiles(e.dataTransfer.files);
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-indigo-600 border-t-transparent" />
      </div>
    );
  }

  const completedDocs = documents.filter((d) => d.status === 'completed');

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Header */}
      <header className="border-b border-slate-200 bg-white px-6 py-4">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <div className="flex items-center gap-3">
            <button onClick={() => router.push('/')} className="text-slate-400 hover:text-slate-600 text-sm">
              ← ホーム
            </button>
            <span className="text-slate-300">/</span>
            <div>
              <h1 className="text-base font-semibold text-slate-900">{workspace?.name ?? workspaceId}</h1>
              {workspace && (
                <p className="text-xs text-slate-400">{workspace.plan === 'pro' ? '✦ Pro' : 'Free'}</p>
              )}
            </div>
          </div>
          {completedDocs.length > 0 && (
            <button
              onClick={() => router.push(`/w/${workspaceId}/explore`)}
              className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 transition-colors"
            >
              グラフを探索 →
            </button>
          )}
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-10">
        {/* Upload area */}
        <div
          onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          onClick={() => !upload && fileInputRef.current?.click()}
          className={`mb-8 rounded-2xl border-2 border-dashed px-8 py-10 text-center transition-colors cursor-pointer
            ${dragging ? 'border-indigo-400 bg-indigo-50' : 'border-slate-300 bg-white hover:border-indigo-300 hover:bg-slate-50'}
            ${upload ? 'cursor-default' : ''}
          `}
        >
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            accept=".pdf,.txt,.md,.docx"
            onChange={(e) => handleFiles(e.target.files)}
          />
          {upload ? (
            <UploadProgress state={upload} />
          ) : (
            <>
              <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-indigo-50">
                <svg className="h-6 w-6 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
                </svg>
              </div>
              <p className="font-medium text-slate-700">ドキュメントをドロップ、またはクリックして選択</p>
              <p className="mt-1 text-sm text-slate-400">PDF · TXT · Markdown · DOCX</p>

              {/* Extraction depth toggle */}
              <div className="mt-5 inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-50 p-1">
                {(['full', 'summary'] as ExtractionDepth[]).map((d) => (
                  <button
                    key={d}
                    onClick={(e) => { e.stopPropagation(); setDefaultDepth(d); }}
                    className={`rounded-md px-4 py-1.5 text-xs font-medium transition-colors
                      ${defaultDepth === d ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-700'}
                    `}
                  >
                    {d === 'full' ? '詳細抽出' : '要約のみ'}
                  </button>
                ))}
              </div>
              <p className="mt-1.5 text-xs text-slate-400">
                {defaultDepth === 'full' ? '全チャンクを抽出・グラフ化します' : '要約のみをグラフ化します（高速）'}
              </p>
            </>
          )}
        </div>

        {/* Documents list */}
        <section>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-slate-500 uppercase tracking-wider">ドキュメント</h2>
            <span className="text-xs text-slate-400">{documents.length} 件</span>
          </div>

          {documents.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 bg-white p-10 text-center">
              <p className="text-slate-400 text-sm">まだドキュメントがありません</p>
            </div>
          ) : (
            <ul className="space-y-3">
              {documents.map((doc) => (
                <DocumentCard
                  key={doc.document_id}
                  doc={doc}
                  onExplore={() => router.push(`/w/${workspaceId}/explore?doc=${doc.document_id}`)}
                />
              ))}
            </ul>
          )}
        </section>
      </main>
    </div>
  );
}

function DocumentCard({ doc, onExplore }: { doc: Document; onExplore: () => void }) {
  return (
    <li className="rounded-xl border border-slate-200 bg-white px-5 py-4 shadow-sm flex items-center gap-4 hover:border-slate-300 transition-colors">
      {/* Icon */}
      <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-slate-100">
        <svg className="h-5 w-5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
            d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
        </svg>
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <p className="font-medium text-slate-800 truncate">{doc.filename}</p>
          <StatusBadge status={doc.status} />
        </div>
        <div className="mt-0.5 flex items-center gap-3 text-xs text-slate-400">
          <span>{formatBytes(doc.file_size)}</span>
          <span>·</span>
          <span>{formatDate(doc.created_at)}</span>
          {doc.node_count != null && doc.node_count > 0 && (
            <>
              <span>·</span>
              <span>{doc.node_count} ノード</span>
            </>
          )}
          {doc.current_stage && doc.status === 'processing' && (
            <>
              <span>·</span>
              <span className="text-blue-500">{doc.current_stage}</span>
            </>
          )}
          {doc.error_message && (
            <>
              <span>·</span>
              <span className="text-red-500 truncate max-w-xs">{doc.error_message}</span>
            </>
          )}
        </div>
      </div>

      {/* Action */}
      {doc.status === 'completed' && (
        <button
          onClick={onExplore}
          className="flex-shrink-0 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 hover:border-indigo-400 hover:text-indigo-600 transition-colors"
        >
          探索する
        </button>
      )}
    </li>
  );
}

function UploadProgress({ state }: { state: UploadState }) {
  const phaseLabel: Record<UploadState['phase'], string> = {
    idle: '',
    creating: 'ドキュメントを登録中…',
    uploading: 'アップロード中…',
    starting: '処理を開始中…',
    done: '完了しました',
    error: 'エラーが発生しました',
  };

  return (
    <div className="space-y-3">
      <p className="font-medium text-slate-700">{state.file.name}</p>
      <p className="text-sm text-slate-500">{phaseLabel[state.phase]}</p>
      {state.phase === 'uploading' && (
        <div className="mx-auto max-w-xs">
          <div className="h-1.5 rounded-full bg-slate-200">
            <div
              className="h-1.5 rounded-full bg-indigo-500 transition-all duration-300"
              style={{ width: `${state.progress}%` }}
            />
          </div>
        </div>
      )}
      {state.phase === 'error' && (
        <p className="text-sm text-red-500">{state.error}</p>
      )}
    </div>
  );
}

async function uploadWithProgress(url: string, file: File, onProgress: (pct: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.upload.addEventListener('progress', (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
    });
    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`Upload failed: ${xhr.status}`));
    });
    xhr.addEventListener('error', () => reject(new Error('Network error')));
    xhr.open('PUT', url);
    xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream');
    xhr.send(file);
  });
}
