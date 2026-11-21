import { InlineError } from '@/components/error/InlineError';

/**
 * Props for the {@link WorkspaceEmptyHeader} component.
 */
interface WorkspaceEmptyHeaderProps {
  /** Current display name of the workspace. */
  workspaceName: string;
  /** Whether the inline renaming input field is active. */
  editingName: boolean;
  /** Temporary name string currently being edited. */
  draftName: string;
  /** Whether a rename submission is in flight. */
  savingName: boolean;
  /** Validation error message if the draft name is invalid, or null. */
  nameError: string | null;
  /** Callback fired as the user types in the rename input. */
  onDraftNameChange: (value: string) => void;
  /** Callback to enter renaming mode. */
  onStartRename: () => void;
  /** Callback to commit the renamed draft name. */
  onCommitName: () => void | Promise<void>;
  /** Callback to abort renaming and revert draft changes. */
  onCancelRename: () => void;
  /** Whether the user has permission to edit the workspace name. */
  canEdit?: boolean;
}

/**
 * Renders the top title header and inline rename form for a workspace paper
 * that does not yet possess any tree or document contents.
 *
 * @param props - Component properties.
 * @returns The rendered empty-state header.
 */
export function WorkspaceEmptyHeader({
  workspaceName,
  editingName,
  draftName,
  savingName,
  nameError,
  onDraftNameChange,
  onStartRename,
  onCommitName,
  onCancelRename,
  canEdit = true,
}: WorkspaceEmptyHeaderProps) {
  return (
    <div>
      <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-indigo-400/80">Workspace</p>
      {editingName ? (
        <form
          className="mt-1 flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            void onCommitName();
          }}
        >
          <input
            aria-label="ワークスペース名"
            value={draftName}
            maxLength={64}
            disabled={savingName}
            onChange={(e) => onDraftNameChange(e.target.value)}
            onBlur={() => void onCommitName()}
            autoFocus
            className="min-w-0 flex-1 rounded-md border border-indigo-200 bg-white px-2 py-1 text-lg font-semibold tracking-tight text-stone-800 outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:opacity-60"
          />
          <button
            type="button"
            disabled={savingName}
            onPointerDown={(e) => e.preventDefault()}
            onMouseDown={(e) => e.preventDefault()}
            onClick={onCancelRename}
            className="rounded-md border border-stone-200 px-2 py-1 text-[11px] text-stone-500 disabled:opacity-60"
          >
            取消
          </button>
        </form>
      ) : (
        <button
          type="button"
          onClick={onStartRename}
          disabled={!canEdit}
          className="mt-0.5 block max-w-full truncate text-left text-lg font-semibold tracking-tight text-stone-800 enabled:hover:text-indigo-500"
        >
          {workspaceName}
        </button>
      )}
      {nameError && <InlineError message={nameError} className="mt-1" />}
    </div>
  );
}
