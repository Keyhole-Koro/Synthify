import { describe, it, expect } from 'vitest';
import { validateWorkspaceName } from './workspaceValidation';

describe('validateWorkspaceName', () => {
  it('accepts valid workspace names and trims them', () => {
    const res = validateWorkspaceName('  My Project  ');
    expect(res.ok).toBe(true);
    if (res.ok) {
      expect(res.value).toBe('My Project');
    }
  });

  it('rejects empty or whitespace-only names', () => {
    const empty = validateWorkspaceName('');
    expect(empty.ok).toBe(false);
    if (!empty.ok) {
      expect(empty.error.code).toBe('EMPTY_NAME');
      expect(empty.error.message).toBe('ワークスペース名を入力してください。');
    }

    const spaces = validateWorkspaceName('   ');
    expect(spaces.ok).toBe(false);
    if (!spaces.ok) {
      expect(spaces.error.code).toBe('EMPTY_NAME');
    }
  });

  it('rejects names exceeding 64 characters', () => {
    const longName = 'a'.repeat(65);
    const res = validateWorkspaceName(longName);
    expect(res.ok).toBe(false);
    if (!res.ok) {
      expect(res.error.code).toBe('NAME_TOO_LONG');
      expect(res.error.message).toBe('ワークスペース名は64文字以内で入力してください。');
    }
  });
});
