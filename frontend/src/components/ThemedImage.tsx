import { useState } from 'react';
import { useTheme } from '@/hooks/useTheme';

interface ThemedImageProps extends React.ImgHTMLAttributes<HTMLImageElement> {
  src: string;
  darkSrc?: string;
}

/**
 * Image component that automatically switches between light and dark versions.
 *
 * If darkSrc is not provided, it will be derived from src by inserting '-dark'
 * before the file extension (e.g., '/image.png' -> '/image-dark.png').
 *
 * Falls back to light image if dark version fails to load.
 */
function ThemedImage({ src, darkSrc, alt, onError, ...props }: ThemedImageProps) {
  const { resolvedTheme } = useTheme();
  const [darkFailed, setDarkFailed] = useState(false);

  const derivedDarkSrc = darkSrc ?? src.replace(/(\.[^.]+)$/, '-dark$1');
  const shouldUseDark = resolvedTheme === 'dark' && !darkFailed;
  const imageSrc = shouldUseDark ? derivedDarkSrc : src;

  const handleError = (e: React.SyntheticEvent<HTMLImageElement>) => {
    if (shouldUseDark) {
      setDarkFailed(true);
    }
    onError?.(e);
  };

  return <img src={imageSrc} alt={alt} onError={handleError} {...props} />;
}

export default ThemedImage;
