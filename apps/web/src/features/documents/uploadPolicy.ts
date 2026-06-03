import { createAppError } from '@/lib/errors';

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

export function validateDocumentUploadFile(file: File): void {
  const ext = extensionOf(file.name);
  const mimeType = normalizeMimeType(file.type);

  if (EXECUTABLE_EXTENSIONS.has(ext)) {
    throw unsupportedDocumentTypeError();
  }
  if (EXECUTABLE_MIME_TYPES.has(mimeType)) {
    throw unsupportedDocumentTypeError();
  }
}

function extensionOf(filename: string): string {
  const trimmed = filename.trim().toLowerCase();
  const index = trimmed.lastIndexOf('.');
  return index >= 0 ? trimmed.slice(index) : '';
}

function normalizeMimeType(mimeType: string): string {
  return mimeType.split(';', 1)[0].trim().toLowerCase();
}

function unsupportedDocumentTypeError() {
  return createAppError({
    kind: 'validation',
    message: '実行ファイル形式はアップロードできません。',
    retryable: false,
    code: 'UNSUPPORTED_DOCUMENT_TYPE',
  });
}
