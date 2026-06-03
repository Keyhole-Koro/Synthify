import { describe, expect, it } from 'vitest';
import { validateDocumentUploadFile } from './uploadPolicy';

function file(name: string, type: string) {
  return new File(['content'], name, { type });
}

describe('validateDocumentUploadFile', () => {
  it('allows non-executable files', () => {
    expect(() => validateDocumentUploadFile(file('paper.pdf', 'application/pdf'))).not.toThrow();
    expect(() => validateDocumentUploadFile(file('notes.md', 'application/octet-stream'))).not.toThrow();
    expect(() => validateDocumentUploadFile(file('main.go', ''))).not.toThrow();
    expect(() => validateDocumentUploadFile(file('payload.dat', 'application/octet-stream'))).not.toThrow();
    expect(() => validateDocumentUploadFile(file('diagram.png', 'image/png'))).not.toThrow();
  });

  it('rejects executable files', () => {
    expect(() => validateDocumentUploadFile(file('installer.exe', 'application/x-msdownload'))).toThrow();
    expect(() => validateDocumentUploadFile(file('payload.dat', 'application/x-executable'))).toThrow();
  });
});
