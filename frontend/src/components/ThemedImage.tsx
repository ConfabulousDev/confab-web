import { useTheme } from '@/hooks/useTheme';

interface ThemedImageProps extends React.ImgHTMLAttributes<HTMLImageElement> {
  src: string;
}

/**
 * Image component that reduces opacity in dark mode for better visual integration.
 */
function ThemedImage({ src, alt, style, ...props }: ThemedImageProps) {
  const { theme } = useTheme();

  const themedStyle: React.CSSProperties = {
    ...style,
    opacity: theme === 'dark' ? 0.8 : 1,
    transition: 'opacity 0.2s ease',
  };

  return <img src={src} alt={alt} style={themedStyle} {...props} />;
}

export default ThemedImage;
