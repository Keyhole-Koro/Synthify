import { describe, expect, it } from 'vitest';
import { validateDocumentUploadFile } from './uploadPolicy';

function file(name: string, type: string) {
  return new File(['content'], name, { type });
}

describe('validateDocumentUploadFile', () => {
  it('allows non-executable files', () => {
    expect(validateDocumentUploadFile(file('paper.pdf', 'application/pdf')).ok).toBe(true);
    expect(validateDocumentUploadFile(file('notes.md', 'application/octet-stream')).ok).toBe(true);
    expect(validateDocumentUploadFile(file('main.go', '')).ok).toBe(true);
    expect(validateDocumentUploadFile(file('payload.dat', 'application/octet-stream')).ok).toBe(true);
    expect(validateDocumentUploadFile(file('diagram.png', 'image/png')).ok).toBe(true);
  });

  it('rejects executable files with unsupported type error', () => {
    const exeResult = validateDocumentUploadFile(file('installer.exe', 'application/x-msdownload'));
    expect(exeResult.ok).toBe(false);
    if (!exeResult.ok) {
      expect(exeResult.error.code).toBe('UNSUPPORTED_DOCUMENT_TYPE');
      expect(exeResult.error.message).toBe('実行ファイル形式はアップロードできません。');
    }

    const payloadResult = validateDocumentUploadFile(file('payload.dat', 'application/x-executable'));
    expect(payloadResult.ok).toBe(false);
    if (!payloadResult.ok) {
      expect(payloadResult.error.code).toBe('UNSUPPORTED_DOCUMENT_TYPE');
    }
  });
});
