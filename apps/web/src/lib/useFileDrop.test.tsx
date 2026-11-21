import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useFileDrop } from './useFileDrop';

function Area({ onFile, disabled }: { onFile: (file: File) => void; disabled?: boolean }) {
  const drop = useFileDrop({ onFile, disabled });
  return (
    <div>
      <input {...drop.inputProps} data-testid="input" />
      <div data-testid="area" data-dragging={drop.isDragging} {...drop.dropHandlers}>
        <span data-testid="child">drop here</span>
      </div>
      <button type="button" onClick={drop.openPicker}>pick</button>
    </div>
  );
}

const file = (name: string) => new File(['x'], name, { type: 'text/markdown' });

/** A drag event whose relatedTarget is `to` (jsdom has no DragEvent to carry it). */
function dragLeave(from: Element, to: Element | null) {
  const event = new Event('dragleave', { bubbles: true, cancelable: true });
  Object.defineProperty(event, 'relatedTarget', { value: to });
  act(() => {
    from.dispatchEvent(event);
  });
}

afterEach(cleanup);

describe('useFileDrop', () => {
  it('highlights while a file is dragged over the area, also over its children', () => {
    render(<Area onFile={vi.fn()} />);
    const area = screen.getByTestId('area');
    fireEvent.dragOver(area);
    expect(area.dataset.dragging).toBe('true');
    dragLeave(area, screen.getByTestId('child'));
    expect(area.dataset.dragging).toBe('true');
    dragLeave(area, document.body);
    expect(area.dataset.dragging).toBe('false');
  });

  it('hands over the first dropped file and ends the highlight', () => {
    const onFile = vi.fn();
    render(<Area onFile={onFile} />);
    const area = screen.getByTestId('area');
    fireEvent.dragOver(area);
    const first = file('a.md');
    fireEvent.drop(area, { dataTransfer: { files: [first, file('b.md')] } });
    expect(onFile).toHaveBeenCalledExactlyOnceWith(first);
    expect(area.dataset.dragging).toBe('false');
    fireEvent.drop(area, { dataTransfer: { files: [] } });
    expect(onFile).toHaveBeenCalledTimes(1);
  });

  it('hands over a picked file and clears the input, so the same file can be picked again', () => {
    const onFile = vi.fn();
    render(<Area onFile={onFile} />);
    const input = screen.getByTestId('input') as HTMLInputElement;
    const click = vi.spyOn(input, 'click');
    fireEvent.click(screen.getByText('pick'));
    expect(click).toHaveBeenCalledTimes(1);
    const picked = file('c.md');
    fireEvent.change(input, { target: { files: [picked] } });
    expect(onFile).toHaveBeenCalledExactlyOnceWith(picked);
    expect(input.value).toBe('');
  });

  it('does nothing while disabled', () => {
    const onFile = vi.fn();
    render(<Area onFile={onFile} disabled />);
    const area = screen.getByTestId('area');
    const click = vi.spyOn(screen.getByTestId('input') as HTMLInputElement, 'click');
    fireEvent.dragOver(area);
    expect(area.dataset.dragging).toBe('false');
    fireEvent.drop(area, { dataTransfer: { files: [file('a.md')] } });
    fireEvent.click(screen.getByText('pick'));
    expect(onFile).not.toHaveBeenCalled();
    expect(click).not.toHaveBeenCalled();
  });
});
