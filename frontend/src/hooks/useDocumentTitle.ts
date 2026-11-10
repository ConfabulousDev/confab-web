import { useEffect } from 'react';

const PRODUCT_NAME = 'Confabulous Code';

/**
 * Sets the document title as "<title> | Confabulous Code", or just
 * "Confabulous Code" when the title is the product name itself (home page).
 */
export function useDocumentTitle(title: string) {
  useEffect(() => {
    document.title = title === PRODUCT_NAME ? title : `${title} | ${PRODUCT_NAME}`;

    return () => {
      document.title = PRODUCT_NAME;
    };
  }, [title]);
}
