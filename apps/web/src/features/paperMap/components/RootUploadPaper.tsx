import { useState } from 'react';
import { WorkspaceDropzone } from '@/features/workspaces/paper/components/WorkspaceDropzone';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';
import { log } from '@/lib/observability/log';
import { useFileDrop } from '@/lib/useFileDrop';

/**
 * Props for the {@link RootUploadPaper} component.
 */
interface RootUploadPaperProps {
  /** Whether interactions and file drops are currently disabled. */
  disabled: boolean;
  /** Asynchronous callback triggered to process the dropped or selected file. */
  onUpload: (file: File) => Promise<void>;
}

/**
 * Dropzone component rendered on the root canvas landing paper,
 * allowing users to upload documents directly into the active or initial workspace.
 *
 * @param props - Component properties.
 * @returns The rendered root upload dropzone area.
 */
export function RootUploadPaper({
  disabled,
  onUpload,
}: RootUploadPaperProps) {
  const [uploading, setUploading] = useState(false);
  const [uploadMessage, setUploadMessage] = useState<string | null>(null);

  async function handleUpload(file: File) {
    if (disabled || uploading) return;
    setUploading(true);
    setUploadMessage(null);
    try {
      await onUpload(file);
    } catch (err) {
      log.error('Root upload failed', { source: 'root_upload_paper' }, err);
      let message = '通信環境またはファイル形式を確認してください。';
      if (err instanceof UserFacingError) {
        message = err.message;
      } else if (isExpectedConnectError(err)) {
        message = formatConnectErrorMessage(err, message);
      }
      setUploadMessage(
        `アップロードに失敗しました。${message}`,
      );
    } finally {
      setUploading(false);
    }
  }

  const drop = useFileDrop({ onFile: handleUpload, disabled });

  return (
    <div className="flex h-full flex-col justify-center gap-3 p-5">
      <input {...drop.inputProps} className="sr-only" />
      <WorkspaceDropzone
        isTreeMissing
        hasChildItems={false}
        uploading={uploading}
        activeJobId={null}
        isDragging={drop.isDragging}
        {...drop.dropHandlers}
        onClick={drop.openPicker}
      />
      {uploadMessage && (
        <p
          className={[
            'text-center text-[11px]',
            uploadMessage.includes('失敗') ? 'text-red-400' : 'text-indigo-500',
          ].join(' ')}
        >
          {uploadMessage}
        </p>
      )}
    </div>
  );
}
