'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import dynamic from 'next/dynamic';
import { onAuthStateChanged } from 'firebase/auth';
import type { PaperMap, PaperViewState } from '@keyhole-koro/paper-in-paper';
import { buildPaperMapFromGraph, findRootNodeId } from '@/features/graph/buildPaperMap';
import {
  getGraph, getGraphEntityDetail,
  type ApiNode, type ApiEdge, type GraphEntityDetail,
} from '@/features/graph/api';
import { listDocuments, type Document } from '@/features/documents/api';
import { ApiError } from '@/lib/rpc';
import { auth } from '@/lib/firebase';

type ExpansionMap = PaperViewState['expansionMap'];

// PaperCanvas must be rendered client-side only (it uses browser APIs)
const PaperCanvas = dynamic(
  () => import('@keyhole-koro/paper-in-paper').then((mod) => mod.PaperCanvas),
  { ssr: false, loading: () => <CanvasLoader /> },
);

function CanvasLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-indigo-600 border-t-transparent" />
    </div>
  );
}

export default function ExplorePage() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const router = useRouter();
  const searchParams = useSearchParams();

  const [documents, setDocuments] = useState<Document[]>([]);
  const [selectedDocId, setSelectedDocId] = useState<string>('');
  const [paperMap, setPaperMap] = useState<PaperMap>(new Map());
  const [expansionMap, setExpansionMap] = useState<ExpansionMap>(new Map());
  const [rootId, setRootId] = useState<string | undefined>();
  const [focusedNodeId, setFocusedNodeId] = useState<string | null>(null);
  const [detail, setDetail] = useState<GraphEntityDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [graphLoading, setGraphLoading] = useState(false);
  const [authReady, setAuthReady] = useState(false);
  const [nodes, setNodes] = useState<ApiNode[]>([]);
  const [edges, setEdges] = useState<ApiEdge[]>([]);

  useEffect(() => {
    return onAuthStateChanged(auth, (user) => {
      if (!user) {
        router.replace('/');
        return;
      }
      setAuthReady(true);
    });
  }, [router]);

  // Load document list
  useEffect(() => {
    if (!authReady) return;
    if (workspaceId === 'ws_demo') {
      router.replace('/');
      return;
    }

    listDocuments(workspaceId)
      .then((docs: Document[]) => {
        const completed = docs;
        setDocuments(completed);
        const paramDoc = searchParams.get('doc');
        if (paramDoc && completed.find((d) => d.document_id === paramDoc)) {
          setGraphLoading(true);
          setFocusedNodeId(null);
          setDetail(null);
          setSelectedDocId(paramDoc);
        } else if (completed.length > 0) {
          setGraphLoading(true);
          setFocusedNodeId(null);
          setDetail(null);
          setSelectedDocId(completed[0].document_id);
        }
      })
      .catch((err) => {
        console.error(err);
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/');
          return;
        }
        if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
          router.replace('/workspaces');
          return;
        }
        router.replace('/');
      });
  }, [authReady, workspaceId, searchParams, router]);

  // Load graph when document changes
  useEffect(() => {
    if (!authReady || !selectedDocId) return;

    getGraph(workspaceId)
      .then((graph) => {
        setNodes(graph.nodes);
        setEdges(graph.edges);
        const pm = buildPaperMapFromGraph(graph.nodes, graph.edges);
        const rid = findRootNodeId(graph.nodes, graph.edges);
        setPaperMap(pm);
        setRootId(rid);
        setExpansionMap(new Map());
      })
      .catch((err) => {
        console.error(err);
        if (err instanceof ApiError && err.status === 401) {
          router.replace('/');
          return;
        }
        if (err instanceof ApiError && (err.status === 403 || err.status === 404)) {
          router.replace('/workspaces');
        }
      })
      .finally(() => setGraphLoading(false));
  }, [authReady, workspaceId, selectedDocId, router]);

  // Load detail when node focused
  useEffect(() => {
    if (!authReady || !focusedNodeId || !selectedDocId) return;
    getGraphEntityDetail({
      workspace_id: workspaceId,
      scope: 'document',
      id: focusedNodeId,
      document_id: selectedDocId,
    })
      .then(setDetail)
      .catch(console.error)
      .finally(() => setDetailLoading(false));
  }, [authReady, focusedNodeId, workspaceId, selectedDocId]);

  const handleDocChange = useCallback((docId: string) => {
    setGraphLoading(true);
    setFocusedNodeId(null);
    setDetail(null);
    setDetailLoading(false);
    setSelectedDocId(docId);
    router.replace(`/w/${workspaceId}/explore?doc=${docId}`);
  }, [workspaceId, router]);

  const handleFocusedNodeIdChange = useCallback((nodeId: string | null) => {
    setFocusedNodeId(nodeId);
    if (!nodeId) {
      setDetail(null);
      setDetailLoading(false);
      return;
    }
    setDetailLoading(true);
  }, []);

  const focusedNode = nodes.find((n) => n.id === focusedNodeId);

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-slate-900">
      {/* Top bar */}
      <header className="flex flex-shrink-0 items-center gap-4 border-b border-slate-700 bg-slate-800 px-4 py-2.5">
        <button
          onClick={() => router.push(`/w/${workspaceId}`)}
          className="text-slate-400 hover:text-slate-200 text-sm transition-colors"
        >
          ← ワークスペース
        </button>

        <div className="h-4 w-px bg-slate-600" />

        {/* Document selector */}
        <select
          value={selectedDocId}
          onChange={(e) => handleDocChange(e.target.value)}
          className="rounded-md border border-slate-600 bg-slate-700 px-3 py-1.5 text-sm text-slate-200 focus:border-indigo-500 focus:outline-none"
        >
          {documents.map((doc) => (
            <option key={doc.document_id} value={doc.document_id}>
              {doc.filename}
            </option>
          ))}
        </select>

        {graphLoading && (
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-indigo-400 border-t-transparent" />
        )}

        <div className="ml-auto flex items-center gap-2 text-xs text-slate-500">
          <span>{nodes.length} ノード</span>
          <span>·</span>
          <span>{edges.length} エッジ</span>
        </div>
      </header>

      {/* Canvas + side panel */}
      <div className="flex flex-1 overflow-hidden">
        {/* Canvas */}
        <div className="relative flex-1 overflow-hidden">
          {documents.length === 0 ? (
            <div className="flex h-full items-center justify-center text-slate-500">
              完了済みのドキュメントがありません
            </div>
          ) : paperMap.size === 0 && !graphLoading ? (
            <div className="flex h-full items-center justify-center text-slate-500">
              グラフデータがありません
            </div>
          ) : (
            <PaperCanvas
              paperMap={paperMap}
              rootId={rootId}
              expansionMap={expansionMap}
              focusedNodeId={focusedNodeId}
              onPaperMapChange={setPaperMap}
              onExpansionMapChange={setExpansionMap}
              onFocusedNodeIdChange={handleFocusedNodeIdChange}
            />
          )}
        </div>

        {/* Detail panel */}
        {focusedNodeId && (
          <DetailPanel
            node={focusedNode ?? null}
            detail={detail}
            loading={detailLoading}
            onClose={() => {
              setFocusedNodeId(null);
              setDetail(null);
              setDetailLoading(false);
            }}
          />
        )}
      </div>
    </div>
  );
}

interface DetailPanelProps {
  node: ApiNode | null;
  detail: GraphEntityDetail | null;
  loading: boolean;
  onClose: () => void;
}

function DetailPanel({ node, detail, loading, onClose }: DetailPanelProps) {
  const panelRef = useRef<HTMLDivElement>(null);

  return (
    <div
      ref={panelRef}
      className="flex w-80 flex-shrink-0 flex-col border-l border-slate-700 bg-slate-800 overflow-hidden"
    >
      {/* Panel header */}
      <div className="flex items-start justify-between border-b border-slate-700 px-4 py-3">
        <div className="min-w-0 flex-1">
          {node ? (
            <>
              <div className="mb-1 flex items-center gap-2">
                <span className="text-xs text-slate-500">Lv.{node.level}</span>
                {node.entity_type && (
                  <span className="rounded bg-slate-700 px-1.5 py-0.5 text-xs text-slate-400">
                    {node.entity_type}
                  </span>
                )}
              </div>
              <h2 className="font-semibold text-slate-100 leading-snug">{node.label}</h2>
            </>
          ) : (
            <div className="h-5 w-32 animate-pulse rounded bg-slate-700" />
          )}
        </div>
        <button
          onClick={onClose}
          className="ml-2 flex-shrink-0 rounded p-1 text-slate-500 hover:text-slate-300 transition-colors"
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {/* Panel body */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-5 text-sm text-slate-300">
        {loading ? (
          <div className="space-y-3">
            {[60, 90, 45, 75].map((w, i) => (
              <div key={i} className="h-3 animate-pulse rounded bg-slate-700" style={{ width: `${w}%` }} />
            ))}
          </div>
        ) : (
          <>
            {node?.description && (
              <div>
                <p className="mb-1 text-xs font-medium uppercase tracking-wider text-slate-500">説明</p>
                <p className="leading-relaxed text-slate-300">{node.description}</p>
              </div>
            )}

            {node?.summary_html && (
              <div>
                <p className="mb-2 text-xs font-medium uppercase tracking-wider text-slate-500">サマリ</p>
                <div
                  className="prose prose-sm prose-invert max-w-none leading-relaxed
                    [&_a[data-paper-id]]:text-indigo-400 [&_a[data-paper-id]]:underline [&_a[data-paper-id]]:cursor-pointer"
                  dangerouslySetInnerHTML={{ __html: node.summary_html }}
                />
              </div>
            )}

            {detail && detail.evidence.source_chunks.length > 0 && (
              <div>
                <p className="mb-2 text-xs font-medium uppercase tracking-wider text-slate-500">
                  ソースチャンク ({detail.evidence.source_chunks.length})
                </p>
                <ul className="space-y-3">
                  {detail.evidence.source_chunks.map((chunk) => (
                    <li key={chunk.chunk_id} className="rounded-lg border border-slate-700 p-3">
                      {chunk.heading && (
                        <p className="mb-1 text-xs font-medium text-slate-400">{chunk.heading}</p>
                      )}
                      <p className="text-xs leading-relaxed text-slate-400 line-clamp-4">{chunk.text}</p>
                      {chunk.source_page != null && (
                        <p className="mt-1 text-xs text-slate-600">p. {chunk.source_page}</p>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {detail && detail.related_edges.length > 0 && (
              <div>
                <p className="mb-2 text-xs font-medium uppercase tracking-wider text-slate-500">
                  関連エッジ ({detail.related_edges.length})
                </p>
                <ul className="space-y-1.5">
                  {detail.related_edges.map((edge) => (
                    <li key={edge.id} className="flex items-center gap-2 text-xs text-slate-400">
                      <span className="rounded bg-slate-700 px-1.5 py-0.5 font-mono text-slate-500">{edge.type}</span>
                      <span className="truncate">
                        {edge.source === node?.id ? `→ ${edge.target}` : `← ${edge.source}`}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
