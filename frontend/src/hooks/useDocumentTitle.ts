import { useEffect } from 'react';

/**
 * Sets the document title. Appends "| Confabulous" suffix unless title is just "Confabulous".
 */
export function useDocumentTitle(title: string) {
  useEffect(() => {
    const fullTitle = title === 'Confabulous' ? title : `${title} | Confabulous`;
    document.title = fullTitle;

    return () => {
      document.title = 'Confabulous';
    };
  }, [title]);
}
