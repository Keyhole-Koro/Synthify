import { useRef, useState, type ChangeEvent, type DragEvent, type MouseEvent, type RefObject } from 'react';

interface UseFileDropOptions {
  /** Called with the dropped or picked file (the first one, when there are several). */
  onFile: (file: File) => void | Promise<void>;
  /** While true, drags do not highlight, drops are ignored and the picker does not open. */
  disabled?: boolean;
}

export interface FileDrop {
  /** A file is being dragged over the drop area. */
  isDragging: boolean;
  /** Handlers for the drop area. */
  dropHandlers: {
    onDragOver: (event: DragEvent) => void;
    onDragLeave: (event: DragEvent) => void;
    onDrop: (event: DragEvent) => void;
  };
  /** Props for a hidden `<input type="file">` that `openPicker` clicks. */
  inputProps: {
    ref: RefObject<HTMLInputElement | null>;
    type: 'file';
    onChange: (event: ChangeEvent<HTMLInputElement>) => void;
    onClick: (event: MouseEvent<HTMLInputElement>) => void;
  };
  /** Opens the file picker. */
  openPicker: () => void;
}

/**
 * One file, dropped on an area or picked with the file picker: the drag highlight, the drop and
 * the hidden input that the picker goes through.
 */
export function useFileDrop({ onFile, disabled = false }: UseFileDropOptions): FileDrop {
  const inputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);

  return {
    isDragging: isDragging && !disabled,
    dropHandlers: {
      onDragOver: (event) => {
        // Without this the browser would open the file instead of dropping it here.
        event.preventDefault();
        if (!disabled) setIsDragging(true);
      },
      onDragLeave: (event) => {
        // Moving onto a child of the area is not leaving it.
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setIsDragging(false);
      },
      onDrop: (event) => {
        event.preventDefault();
        setIsDragging(false);
        const file = event.dataTransfer.files[0];
        if (file && !disabled) void onFile(file);
      },
    },
    inputProps: {
      ref: inputRef,
      type: 'file',
      onChange: (event) => {
        const file = event.target.files?.[0];
        // Cleared so picking the same file again is a change too.
        event.target.value = '';
        if (file) void onFile(file);
      },
      onClick: (event) => {
        event.currentTarget.value = '';
      },
    },
    openPicker: () => {
      if (!disabled) inputRef.current?.click();
    },
  };
}
