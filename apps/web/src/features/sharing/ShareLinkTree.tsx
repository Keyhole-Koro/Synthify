
import { useEffect, useState } from 'react';
import { getTreeViaShareToken, type ApiItem } from '@/features/tree/api';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';

/**
 * Props for displaying a knowledge tree fetched via public share token.
 */
interface ShareLinkTreeProps {
  /** Target workspace identifier. */
  workspaceId: string;
  /** Share authorization token. */
  token: string;
}

interface TreeNode {
  item: ApiItem;
  children: TreeNode[];
}

/**
 * Reconstructs a hierarchical forest of TreeNodes from a flat array of items using parentId pointers.
 */
function buildForest(items: ApiItem[]): TreeNode[] {
  const byId = new Map<string, TreeNode>();
  for (const item of items) {
    byId.set(item.id, { item, children: [] });
  }
  const roots: TreeNode[] = [];
  for (const node of byId.values()) {
    const parent = node.item.parentId ? byId.get(node.item.parentId) : undefined;
    if (parent) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

/**
 * Recursive tree node item rendering title, description, and children nested with indentation.
 */
function TreeNodeView({ node, depth }: { node: TreeNode; depth: number }) {
  return (
    <li>
      <div className="py-1" style={{ paddingLeft: depth * 16 }}>
        <p className="text-sm font-medium text-slate-800">{node.item.title}</p>
        {node.item.description && (
          <p className="text-xs text-slate-500">{node.item.description}</p>
        )}
      </div>
      {node.children.length > 0 && (
        <ul>
          {node.children.map((child) => (
            <TreeNodeView key={child.item.id} node={child} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  );
}

type TreeState =
  | { status: 'loading' }
  | { status: 'ready'; forest: TreeNode[] }
  | { status: 'error'; message: string };

/**
 * Component rendering the knowledge tree view for unauthenticated or public viewers holding a share token.
 *
 * @param props - Workspace identifier and public share token used to fetch the tree.
 * @returns Loading, error, empty, or rendered shared-tree UI.
 */
export function ShareLinkTree({ workspaceId, token }: ShareLinkTreeProps) {
  const [state, setState] = useState<TreeState>({ status: 'loading' });

  useEffect(() => {
    let cancelled = false;
    getTreeViaShareToken(workspaceId, token)
      .then((tree) => {
        if (cancelled) return;
        // workspace_root は表示の最上位なので、その子から見せる。
        const forest = buildForest(tree.items ?? []);
        setState({ status: 'ready', forest });
      })
      .catch((err) => {
        if (cancelled) return;
        let message = 'ツリーを取得できませんでした。';
        if (err instanceof UserFacingError) {
          message = err.message;
        } else if (isExpectedConnectError(err)) {
          message = formatConnectErrorMessage(err, message);
        } else {
          console.error('Unexpected UI error', err);
        }
        setState({
          status: 'error',
          message,
        });
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, token]);

  if (state.status === 'loading') {
    return <p className="mt-6 text-sm text-slate-400">ツリーを読み込んでいます…</p>;
  }
  if (state.status === 'error') {
    return <p className="mt-6 text-sm text-slate-500">{state.message}</p>;
  }
  if (state.forest.length === 0) {
    return <p className="mt-6 text-sm text-slate-400">まだ知識ツリーがありません。</p>;
  }

  return (
    <ul className="mt-6 border-t border-slate-100 pt-4">
      {state.forest.map((node) => (
        <TreeNodeView key={node.item.id} node={node} depth={0} />
      ))}
    </ul>
  );
}
