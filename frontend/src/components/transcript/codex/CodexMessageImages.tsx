// Shared image-gallery block for Codex user + assistant messages (CF-388).
// Renders one `<img>` per entry in `images` (typically inlined base64 data
// URLs), styled to match the dimension caps in `ContentBlock`'s
// `imageBlock` for cross-provider visual parity. Alt-text is parameterized
// because the user-attached vs. assistant-generated distinction is
// load-bearing for screen readers.

import styles from './CodexMessage.module.css';

export interface CodexMessageImagesProps {
  images: string[];
  /** Alt-text prefix; final alt is `${altPrefix} #${i + 1}`. */
  altPrefix: string;
}

export default function CodexMessageImages({
  images,
  altPrefix,
}: CodexMessageImagesProps) {
  if (images.length === 0) return null;
  return (
    <div className={styles.imageList}>
      {images.map((src, i) => (
        <div key={i} className={styles.imageBlock}>
          <img src={src} alt={`${altPrefix} #${i + 1}`} loading="lazy" />
        </div>
      ))}
    </div>
  );
}
