import { useEffect } from 'react';

/**
 * Sets the document title. Appends "| Confab" suffix unless title is just "Confab".
 */
export function useDocumentTitle(title: string) {
  useEffect(() => {
    const fullTitle = title === 'Confab' ? title : `${title} | Confab`;
    document.title = fullTitle;

    return () => {
      document.title = 'Confab';
    };
  }, [title]);
}
