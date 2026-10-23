import { createRPCClient } from '@/lib/connect';
import { TreeImportService } from '@/gen/proto/synthify/app/v1/tree_import_pb';
import {
  StifDiagnosticSeverity,
  type ImportStifResponse,
  type StifDiagnostic,
  type StifParsedItem,
} from '@/gen/proto/synthify/app/v1/tree_import_pb';

export { StifDiagnosticSeverity };
export type { ImportStifResponse, StifDiagnostic, StifParsedItem };

const client = createRPCClient(TreeImportService);

// previewStifImport parses and validates without writing. The parser lives on
// the server, so what the preview shows is exactly what an import would store —
// there is no second implementation here that could drift from it.
export async function previewStifImport(
  workspaceId: string,
  source: string,
  filename = '',
): Promise<ImportStifResponse> {
  return client.importStif({ workspaceId, source, filename, dryRun: true });
}

// commitStifImport appends the document's items to the workspace tree. It still
// returns diagnostics: the server re-parses, so a document that has gone stale
// or that the preview was never run on comes back uncommitted rather than
// half-applied.
export async function commitStifImport(
  workspaceId: string,
  source: string,
  filename = '',
): Promise<ImportStifResponse> {
  return client.importStif({ workspaceId, source, filename, dryRun: false });
}

export function isErrorDiagnostic(diagnostic: StifDiagnostic): boolean {
  return diagnostic.severity === StifDiagnosticSeverity.ERROR;
}

export function hasBlockingErrors(diagnostics: StifDiagnostic[]): boolean {
  return diagnostics.some(isErrorDiagnostic);
}
