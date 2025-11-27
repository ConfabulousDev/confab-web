import { marked } from 'marked';
import DOMPurify from 'dompurify';
import type { ContentBlock as ContentBlockType } from '@/types';
import { isTextBlock, isThinkingBlock, isToolUseBlock, isToolResultBlock, isImageBlock } from '@/types';
import { stripAnsi } from '@/utils';
import CodeBlock from './CodeBlock';
import BashOutput from './BashOutput';
import styles from './ContentBlock.module.css';

// Configure marked for performance
marked.use({
  async: false,
  gfm: true,
  breaks: true,
});

interface ContentBlockProps {
  block: ContentBlockType;
  toolName?: string;
}

function ContentBlock({ block, toolName: initialToolName = '' }: ContentBlockProps) {
  // Derive tool name from block if it's a tool_use block, otherwise use the passed-in name
  const toolName = isToolUseBlock(block) ? block.name : initialToolName;

  // Parse markdown and sanitize HTML
  function renderMarkdown(text: string): string {
    const cleaned = stripAnsi(text);
    // marked.parse returns string when async: false is configured
    const html = marked.parse(cleaned);
    if (typeof html !== 'string') {
      // Should never happen with async: false, but satisfies TypeScript
      return '';
    }
    return DOMPurify.sanitize(html, {
      ADD_ATTR: ['target'], // Allow target="_blank" on links
    });
  }

  // Detect if this is Bash-like output
  function isBashOutput(content: string, tool: string): boolean {
    if (tool === 'Bash') return true;
    // Heuristic: check for common bash patterns
    return content.includes('$ ') || content.match(/^[\w@-]+:/) !== null || content.includes('\n$ ');
  }

  if (isTextBlock(block)) {
    return (
      <div
        className={styles.textBlock}
        dangerouslySetInnerHTML={{ __html: renderMarkdown(block.text) }}
      />
    );
  }

  if (isThinkingBlock(block)) {
    return (
      <div className={styles.thinkingBlock}>
        <div className={styles.thinkingHeader}>
          <span className={styles.thinkingIcon}>💭</span>
          <span className={styles.thinkingLabel}>Thinking</span>
        </div>
        <div className={styles.thinkingContent}>
          <pre>{stripAnsi(block.thinking)}</pre>
        </div>
      </div>
    );
  }

  if (isToolUseBlock(block)) {
    return (
      <div className={styles.toolUseBlock}>
        <div className={styles.toolHeader}>
          <span className={styles.toolIcon}>🛠️</span>
          <span className={styles.toolName}>{block.name}</span>
        </div>
        <div className={styles.toolInput}>
          <CodeBlock code={JSON.stringify(block.input, null, 2)} language="json" />
        </div>
      </div>
    );
  }

  if (isToolResultBlock(block)) {
    return (
      <div className={`${styles.toolResultBlock} ${block.is_error ? styles.error : ''}`}>
        <div className={styles.toolResultHeader}>
          <span className={styles.resultIcon}>{block.is_error ? '❌' : '✅'}</span>
          {toolName && <span className={styles.toolNameLabel}>{toolName}</span>}
        </div>
        <div className={styles.toolResultContent}>
          {typeof block.content === 'string' ? (
            isBashOutput(block.content, toolName) ? (
              <BashOutput output={block.content} />
            ) : (
              <CodeBlock code={block.content} language="plain" maxHeight="500px" truncateLines={100} />
            )
          ) : (
            // Recursive rendering for nested content blocks
            block.content.map((nestedBlock, i) => (
              <ContentBlock
                key={i}
                block={nestedBlock}
                toolName={toolName}
              />
            ))
          )}
        </div>
      </div>
    );
  }

  if (isImageBlock(block)) {
    const src =
      block.source.type === 'base64'
        ? `data:${block.source.media_type};base64,${block.source.data}`
        : block.source.url;

    return (
      <div className={styles.imageBlock}>
        <img src={src} alt="User provided image" loading="lazy" />
      </div>
    );
  }

  return (
    <div className={styles.unknownBlock}>
      <em>Unknown content block type</em>
    </div>
  );
}

export default ContentBlock;
