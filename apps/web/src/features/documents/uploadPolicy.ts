import { ok, err, type Result } from '@/shared/result';

const EXECUTABLE_EXTENSIONS = new Set([
  '.apk', '.app', '.bat', '.bin', '.class', '.cmd', '.com', '.dll', '.dmg',
  '.dylib', '.elf', '.exe', '.jar', '.msi', '.scr', '.so',
]);

const EXECUTABLE_MIME_TYPES = new Set([
  'application/x-dosexec',
  'application/x-executable',
  'application/x-mach-binary',
  'application/x-msdownload',
  'application/x-msdos-program',
  'application/x-msi',
  'application/x-sharedlib',
]);

/**
 * Error indicating that a chosen file is prohibited by the upload security policy.
 *
 * @property code Stable machine-readable rejection code.
 * @property message Localized message suitable for displaying near the upload control.
 */
export interface UploadValidationError {
  code: 'UNSUPPORTED_DOCUMENT_TYPE';
  message: string;
}

/**
 * Validates that an upload candidate is not a dangerous binary executable or script format.
 *
 * @param file - File object to inspect.
 * @returns Result with file if valid, or UploadValidationError if rejected.
 */
export function validateDocumentUploadFile(file: File): Result<File, UploadValidationError> {
  const ext = extensionOf(file.name);
  const mimeType = normalizeMimeType(file.type);

  if (EXECUTABLE_EXTENSIONS.has(ext) || EXECUTABLE_MIME_TYPES.has(mimeType)) {
    return err({
      code: 'UNSUPPORTED_DOCUMENT_TYPE',
      message: '実行ファイル形式はアップロードできません。',
    });
  }

  return ok(file);
}

/**
 * Extracts lowercase file extension including dot, or empty string.
 */
function extensionOf(filename: string): string {
  const trimmed = filename.trim().toLowerCase();
  const index = trimmed.lastIndexOf('.');
  return index >= 0 ? trimmed.slice(index) : '';
}

/**
 * Normalizes a MIME type string by stripping parameters and lowercasing.
 */
function normalizeMimeType(mimeType: string): string {
  return mimeType.split(';', 1)[0].trim().toLowerCase();
}
