import { ok, err, type Result } from '@/shared/result';

/**
 * Represents a validation failure when checking a proposed workspace name.
 */
export type WorkspaceNameValidationError = {
  /** Machine-readable error code identifying the validation issue. */
  code: 'EMPTY_NAME' | 'NAME_TOO_LONG';
  /** Localized human-readable error description. */
  message: string;
};

/**
 * Validates a proposed workspace display name against length and emptiness constraints.
 *
 * @param name - The input workspace name candidate.
 * @returns An `ok` Result with trimmed name or an `err` Result containing validation details.
 */
export function validateWorkspaceName(name: string): Result<string, WorkspaceNameValidationError> {
  const trimmed = name.trim();
  if (trimmed.length === 0) {
    return err({
      code: 'EMPTY_NAME',
      message: 'ワークスペース名を入力してください。',
    });
  }
  if (trimmed.length > 64) {
    return err({
      code: 'NAME_TOO_LONG',
      message: 'ワークスペース名は64文字以内で入力してください。',
    });
  }
  return ok(trimmed);
}
