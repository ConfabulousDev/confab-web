import { useEffect, useMemo, type MouseEvent } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useDocumentTitle } from '@/hooks/useDocumentTitle';
import { LEGAL_DOCS, type LegalSlug } from '@/legal/docs';
import { LEGAL_MARKDOWN } from '@/legal/content';
import { renderLegalMarkdown } from '@/utils';
import { formatCalendarDate } from '@/utils/formatting';
import styles from './LegalDocPage.module.css';

interface LegalDocPageProps {
  slug: LegalSlug;
}

/** Renders one in-repo legal document (frontend/src/legal/) by slug. */
function LegalDocPage({ slug }: LegalDocPageProps) {
  const doc = LEGAL_DOCS.find((d) => d.slug === slug)!;
  const html = useMemo(() => renderLegalMarkdown(LEGAL_MARKDOWN[slug]), [slug]);
  const { hash } = useLocation();
  const navigate = useNavigate();
  useDocumentTitle(doc.title);

  // The body is injected after mount, so the browser can't jump to a deep
  // link (/privacy#section) on its own.
  useEffect(() => {
    if (!hash) return;
    let id: string;
    try {
      id = decodeURIComponent(hash.slice(1));
    } catch {
      return; // malformed escape in a hand-typed URL: nothing to scroll to
    }
    document.getElementById(id)?.scrollIntoView();
  }, [hash, html]);

  // Links between policies (/terms, /cookies, …) are plain <a> tags in the
  // Markdown; route them client-side instead of reloading the app.
  function handleBodyClick(e: MouseEvent<HTMLDivElement>) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const href = e.target instanceof Element ? e.target.closest('a')?.getAttribute('href') : null;
    if (href?.startsWith('/') && !href.startsWith('//')) {
      e.preventDefault();
      navigate(href);
    }
  }

  return (
    <div className={styles.container}>
      <article className={styles.content}>
        <h1 className={styles.title}>{doc.title}</h1>
        <p className={styles.updated}>Last updated {formatCalendarDate(doc.lastUpdated)}</p>
        <div
          className={styles.body}
          onClick={handleBodyClick}
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </article>
    </div>
  );
}

export default LegalDocPage;
