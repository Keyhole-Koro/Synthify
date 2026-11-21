import type { PointerEvent } from 'react';
import { PRIVACY_PATH, TERMS_PATH } from './legalDocument';

const LINK_CLASS = 'underline-offset-2 transition-colors hover:text-stone-800 hover:underline';

type LegalPage = 'terms' | 'privacy';

/**
 * Links to the terms of service and the privacy policy. Plain <a> elements (full page
 * loads), so they work under any router and from inside the paper canvas.
 *
 * @param current the document being shown, marked with aria-current
 */
export function LegalLinks({ current, className = '' }: { current?: LegalPage; className?: string }) {
  return (
    <nav aria-label="利用規約とプライバシーポリシー" className={`flex flex-wrap items-center justify-center gap-x-4 gap-y-1 ${className}`}>
      <a href={TERMS_PATH} aria-current={current === 'terms' ? 'page' : undefined} className={LINK_CLASS}>
        利用規約
      </a>
      <a href={PRIVACY_PATH} aria-current={current === 'privacy' ? 'page' : undefined} className={LINK_CLASS}>
        プライバシーポリシー
      </a>
    </nav>
  );
}

/** The site footer: the landing page overlays it on the background, the legal pages end with it. */
export function LegalFooter({ current, className = '' }: { current?: LegalPage; className?: string }) {
  return (
    <footer className={`text-[11px] text-stone-500 ${className}`}>
      <LegalLinks current={current} />
    </footer>
  );
}

function stopPointer(event: PointerEvent) {
  event.stopPropagation();
}

/**
 * The consent line under the sign-in button. The documents open in a new tab so the
 * sign-in paper stays where it was. Pointer events stop here, as on the sign-in button,
 * so a click on the line is not taken by the canvas.
 */
export function SignInLegalNotice() {
  return (
    <p
      onPointerDown={stopPointer}
      onPointerUp={stopPointer}
      className="text-center text-[10px] leading-relaxed text-stone-400"
    >
      ログインすると、
      <a href={TERMS_PATH} target="_blank" rel="noopener noreferrer" className={`text-stone-500 underline ${LINK_CLASS}`}>
        利用規約
      </a>
      と
      <a href={PRIVACY_PATH} target="_blank" rel="noopener noreferrer" className={`text-stone-500 underline ${LINK_CLASS}`}>
        プライバシーポリシー
      </a>
      に同意したものとみなします。
    </p>
  );
}
