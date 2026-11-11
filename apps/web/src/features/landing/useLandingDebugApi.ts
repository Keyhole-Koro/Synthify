import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { create } from '@bufbuild/protobuf';
import type { ExpansionMap, Paper, PaperMap } from '@keyhole-koro/paper-in-paper';
import { collection, getDocs, limit, orderBy, query } from 'firebase/firestore';
import { WorkspaceSchema } from '@/gen/proto/synthify/app/v1/workspace_pb';
import type { AuthUser } from '@/features/auth/session';
import type { FirestoreJobStatus } from '@/features/jobs/firestore/useJobStatus';
import type { Workspace } from '@/features/workspaces/api';
import { workspacesQueryKey } from '@/features/workspaces/queries';
import type { useWorkspaceTree } from '@/features/workspaces/useWorkspaceTree';
import { db } from '@/lib/firebase';

declare global {
  interface Window {
    __synthifyDebug?: {
      dumpWorkspace: (workspaceId: string) => Promise<unknown>;
      refreshWorkspaceTree: (workspaceId: string) => Promise<void>;
      injectMockNode: (
        workspaceId: string,
        args?: {
          itemId?: string;
          title?: string;
          description?: string;
          content?: string;
          overrideCss?: string;
          parentId?: string;
        },
      ) => Promise<unknown>;
      createDebugCompletedWorkspace: (
        args?: { workspaceName?: string; paperTitle?: string; paperDescription?: string },
      ) => Promise<unknown>;
      createMockWorkspace: (
        args?: {
          workspaceName?: string;
          documentCount?: number;
          nodesPerDocument?: number;
          documentTitles?: string[];
          rootContent?: string;
          rootOverrideCss?: string;
        },
      ) => unknown;
    };
  }
}

interface LandingDebugApiOptions {
  user: AuthUser | null;
  paperMap: PaperMap;
  expansionMap: ExpansionMap;
  workspacePaperGroups: Map<string, Paper[]>;
  tree: ReturnType<typeof useWorkspaceTree>;
  handleCreateWorkspace: (name: string) => Promise<Workspace>;
  markWorkspaceVerified: (workspaceId: string) => void;
  setPendingRevealNodeId: (nodeId: string | null) => void;
}

/**
 * Development only: exposes `window.__synthifyDebug`, console helpers that dump a workspace's
 * tree and papers, refresh it, and inject mock nodes or whole mock workspaces (without a
 * backend) to preview the canvas. Does nothing in production builds.
 */
export function useLandingDebugApi({
  user,
  paperMap,
  expansionMap,
  workspacePaperGroups,
  tree,
  handleCreateWorkspace,
  markWorkspaceVerified,
  setPendingRevealNodeId,
}: LandingDebugApiOptions) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (import.meta.env.PROD || typeof window === 'undefined') return;

    const debugApi = {
      dumpWorkspace: async (workspaceId: string) => {
        // Firestore is unavailable for frontend-only mock workspaces; don't let
        // it abort the (more useful) paperMap / projection report.
        let latestJobs: FirestoreJobStatus[] = [];
        try {
          const latestJobsSnap = await getDocs(query(
            collection(db, 'workspaces', workspaceId, 'jobs'),
            orderBy('updatedAt', 'desc'),
            limit(5),
          ));
          latestJobs = latestJobsSnap.docs.map((doc) => doc.data() as FirestoreJobStatus);
        } catch (err) {
          console.warn('[synthify-debug] latestJobs unavailable', err);
        }
        const workspacePapers = workspacePaperGroups.get(workspaceId) ?? [];
        const treeSnapshot = tree.getDebugSnapshot(workspaceId);
        const rootNodePaperPresence = treeSnapshot.rootNodeIds.map((id) => ({
          id,
          inPaperMap: paperMap.has(id),
          inWorkspacePaperGroup: workspacePapers.some((paper) => paper.id === id),
        }));

        const dump = {
          workspaceId,
          tree: treeSnapshot,
          workspacePaperGroup: {
            paperIds: workspacePapers.map((paper) => paper.id),
            paperCount: workspacePapers.length,
          },
          paperMap: {
            hasWorkspacePaper: paperMap.has(workspaceId),
            paperMapSize: paperMap.size,
            rootNodePaperPresence,
          },
          expansion: {
            workspaceOpenChildIds: expansionMap.get(workspaceId)?.openChildIds ?? [],
          },
          latestJobs,
        };
        console.info('[synthify-debug] workspace dump', dump);
        return dump;
      },
      refreshWorkspaceTree: async (workspaceId: string) => {
        await tree.refreshWorkspaceTree(workspaceId, { revealNewRootNodes: true });
      },
      injectMockNode: async (
        workspaceId: string,
        args: { itemId?: string; title?: string; description?: string } = {},
      ) => {
        const injected = await tree.injectMockNode(workspaceId, args);
        const dump = await debugApi.dumpWorkspace(workspaceId);
        return { injected, dump };
      },
      createDebugCompletedWorkspace: async (
        args: { workspaceName?: string; paperTitle?: string; paperDescription?: string } = {},
      ) => {
        const workspace = await handleCreateWorkspace(args.workspaceName?.trim() || 'Debug completed workspace');
        await tree.handleOpenWorkspace(workspace.workspaceId, workspace);
        const injected = await tree.injectMockNode(workspace.workspaceId, {
          title: args.paperTitle?.trim() || 'Debug completed paper',
          description: args.paperDescription?.trim() || 'Created by __synthifyDebug.createDebugCompletedWorkspace',
        });
        setPendingRevealNodeId(workspace.workspaceId);
        const dump = await debugApi.dumpWorkspace(workspace.workspaceId);
        return { workspace, injected, dump };
      },
      // createMockWorkspace builds a complete workspace (ws + N documents, each
      // with M nodes) entirely on the frontend — no backend API, no auth, no
      // emulators. Lets you preview WorkspacePaper UI states from the console.
      createMockWorkspace: (
        args: {
          workspaceName?: string;
          documentCount?: number;
          nodesPerDocument?: number;
          documentTitles?: string[];
          rootContent?: string;
          rootOverrideCss?: string;
        } = {},
      ) => {
        const workspaceId = `debug_ws_${Date.now().toString(36)}`;
        const workspace = create(WorkspaceSchema, {
          workspaceId,
          name: args.workspaceName?.trim() || 'Mock workspace',
          ownerId: user?.id ?? 'debug-user',
        });
        queryClient.setQueryData<Workspace[]>(
          workspacesQueryKey(user?.id),
          (prev = []) => [...prev, workspace],
        );
        markWorkspaceVerified(workspaceId);
        const result = tree.injectMockWorkspaceTree(workspace, {
          documentCount: args.documentCount,
          nodesPerDocument: args.nodesPerDocument,
          documentTitles: args.documentTitles,
          rootContent: args.rootContent,
          rootOverrideCss: args.rootOverrideCss,
        });
        setPendingRevealNodeId(workspaceId);
        console.info('[synthify-debug] mock workspace created [SYNC-MARKER-A1]', { workspace, result });
        return { workspace, result };
      },
    };

    window.__synthifyDebug = debugApi;
    return () => {
      if (window.__synthifyDebug === debugApi) {
        delete window.__synthifyDebug;
      }
    };
  }, [handleCreateWorkspace, paperMap, tree, workspacePaperGroups, queryClient, user, expansionMap, markWorkspaceVerified, setPendingRevealNodeId]);
}
