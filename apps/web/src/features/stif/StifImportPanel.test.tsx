import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { StifDiagnosticSeverity } from '@/gen/proto/synthify/app/v1/tree_import_pb';

const { previewStifImport, commitStifImport } = vi.hoisted(() => ({
  previewStifImport: vi.fn(),
  commitStifImport: vi.fn(),
}));

// The api module is replaced wholesale rather than partially: importing the
// real one pulls in the Connect transport, which validates the browser env at
// module load and is not available under vitest.
vi.mock('./api', async () => {
  const { StifDiagnosticSeverity: severity } = await import('@/gen/proto/synthify/app/v1/tree_import_pb');
  type Diagnostic = { severity: number };
  return {
    StifDiagnosticSeverity: severity,
    previewStifImport,
    commitStifImport,
    isErrorDiagnostic: (d: Diagnostic) => d.severity === severity.ERROR,
    hasBlockingErrors: (ds: Diagnostic[]) => ds.some((d) => d.severity === severity.ERROR),
  };
});

const { StifImportPanel } = await import('./StifImportPanel');

const DOCUMENT = '#STIF/1\n::item /a\ntitle: A\n::end\n';

function parsedItem(localPath: string, title: string, depth = 0) {
  return {
    localPath,
    parentLocalPath: '',
    title,
    description: '',
    content: '',
    overrideCss: '',
    crossDocument: false,
    order: 0,
    sources: [],
    itemId: '',
    depth,
  };
}

function response(items: ReturnType<typeof parsedItem>[], diagnostics: unknown[] = []) {
  return { committed: false, mode: 'append', items, diagnostics, importedCount: 0 };
}

function renderPanel(onImported = vi.fn()) {
  render(<StifImportPanel workspaceId="ws-1" onClose={vi.fn()} onImported={onImported} />);
  return {
    onImported,
    textarea: screen.getByLabelText('STIF ドキュメント'),
    previewButton: screen.getByRole('button', { name: 'プレビュー' }),
    importButton: () => screen.getByRole('button', { name: /インポート/ }),
  };
}

function typeDocument(textarea: HTMLElement, value = DOCUMENT) {
  fireEvent.change(textarea, { target: { value } });
}

describe('StifImportPanel', () => {
  beforeEach(() => {
    previewStifImport.mockReset();
    commitStifImport.mockReset();
  });

  it('previews with a dry run and lists the items that would be created', async () => {
    previewStifImport.mockResolvedValue(response([parsedItem('/a', 'Overview'), parsedItem('/a/b', 'API', 1)]));
    const panel = renderPanel();

    typeDocument(panel.textarea);
    fireEvent.click(panel.previewButton);

    await waitFor(() => expect(screen.getByText('Overview')).toBeDefined());
    expect(screen.getByText('API')).toBeDefined();
    expect(previewStifImport).toHaveBeenCalledWith('ws-1', DOCUMENT, '');
    expect(commitStifImport).not.toHaveBeenCalled();
  });

  it('refuses to import until a preview has been run', () => {
    const panel = renderPanel();

    typeDocument(panel.textarea);

    expect(panel.importButton().hasAttribute('disabled')).toBe(true);
  });

  // Importing from a review the user has since edited is the mistake the
  // preview step exists to prevent, so any edit has to invalidate it.
  it('invalidates the preview when the document is edited', async () => {
    previewStifImport.mockResolvedValue(response([parsedItem('/a', 'Overview')]));
    const panel = renderPanel();

    typeDocument(panel.textarea);
    fireEvent.click(panel.previewButton);
    await waitFor(() => expect(screen.getByText('Overview')).toBeDefined());

    typeDocument(panel.textarea, `${DOCUMENT}::item /b\n`);

    expect(screen.queryByText('Overview')).toBeNull();
    expect(panel.importButton().hasAttribute('disabled')).toBe(true);
  });

  it('blocks the import when the document has error diagnostics', async () => {
    previewStifImport.mockResolvedValue(response([], [
      { severity: StifDiagnosticSeverity.ERROR, line: 3, code: 'missing_title', message: 'missing title', localPath: '/a' },
    ]));
    const panel = renderPanel();

    typeDocument(panel.textarea);
    fireEvent.click(panel.previewButton);

    await waitFor(() => expect(screen.getByText('missing title')).toBeDefined());
    expect(panel.importButton().hasAttribute('disabled')).toBe(true);
  });

  // Sanitizer removals arrive as warnings; they are worth showing but must not
  // stand between the user and the import.
  it('allows the import when the document only has warnings', async () => {
    previewStifImport.mockResolvedValue(response([parsedItem('/a', 'Overview')], [
      { severity: StifDiagnosticSeverity.WARNING, line: 5, code: 'unsafe_html', message: 'removed <script>', localPath: '/a' },
    ]));
    const panel = renderPanel();

    typeDocument(panel.textarea);
    fireEvent.click(panel.previewButton);

    await waitFor(() => expect(screen.getByText('removed <script>')).toBeDefined());
    expect(panel.importButton().hasAttribute('disabled')).toBe(false);
  });

  it('reports the committed count and refreshes the tree', async () => {
    previewStifImport.mockResolvedValue(response([parsedItem('/a', 'Overview')]));
    commitStifImport.mockResolvedValue({
      committed: true,
      mode: 'append',
      items: [{ ...parsedItem('/a', 'Overview'), itemId: 'item-1' }],
      diagnostics: [],
      importedCount: 1,
    });
    const panel = renderPanel();

    typeDocument(panel.textarea);
    fireEvent.click(panel.previewButton);
    await waitFor(() => expect(screen.getByText('Overview')).toBeDefined());
    fireEvent.click(panel.importButton());

    await waitFor(() => expect(panel.onImported).toHaveBeenCalledWith(1));
    expect(screen.getByText('1 件のアイテムをツリーに追加しました。')).toBeDefined();
  });

  // The server re-parses on commit, so it can still refuse after a clean
  // preview; the panel must not claim success in that case.
  it('surfaces a refused commit instead of reporting success', async () => {
    previewStifImport.mockResolvedValue(response([parsedItem('/a', 'Overview')]));
    commitStifImport.mockResolvedValue(response([], [
      { severity: StifDiagnosticSeverity.ERROR, line: 1, code: 'missing_header', message: 'missing header', localPath: '' },
    ]));
    const panel = renderPanel();

    typeDocument(panel.textarea);
    fireEvent.click(panel.previewButton);
    await waitFor(() => expect(screen.getByText('Overview')).toBeDefined());
    fireEvent.click(panel.importButton());

    await waitFor(() => expect(screen.getByText('インポートできませんでした。エラーを確認してください。')).toBeDefined());
    expect(panel.onImported).not.toHaveBeenCalled();
  });
});
