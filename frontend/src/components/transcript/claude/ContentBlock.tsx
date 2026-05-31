import type { ContentBlock as ContentBlockType } from '@/types';
import { isTextBlock, isThinkingBlock, isToolUseBlock, isToolResultBlock, isImageBlock, isToolReferenceBlock, warnIfKnownTypeCaughtByCatchall } from '@/types';
import { stripAnsi, renderMarkdownToHtml, tryParseAsJson } from '@/utils';
import { getHighlightClass, highlightTextInHtml, splitTextByQuery } from '@/utils/highlightSearch';
import CodeBlock from '../CodeBlock';
import BashOutput from '../BashOutput';
import styles from './ContentBlock.module.css';

interface ContentBlockProps {
  block: ContentBlockType;
  toolName?: string;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
}

// Detect if this is Bash-like output
function isBashOutput(content: string, tool: string): boolean {
  if (tool === 'Bash') return true;
  // Heuristic: check for common bash patterns
  return content.includes('$ ') || content.match(/^[\w@-]+:/) !== null || content.includes('\n$ ');
}

function ContentBlock({ block, toolName: initialToolName = '', searchQuery, isCurrentSearchMatch }: ContentBlockProps) {
  // Derive tool name from block if it's a tool_use block, otherwise use the passed-in name
  const toolName = isToolUseBlock(block) ? block.name : initialToolName;
  const highlightClass = getHighlightClass(isCurrentSearchMatch ?? false);

  if (isTextBlock(block)) {
    // Check if text content is JSON - if so, pretty-print it
    const jsonContent = tryParseAsJson(block.text);
    if (jsonContent) {
      return <CodeBlock code={jsonContent} language="json" maxHeight="500px" searchQuery={searchQuery} isCurrentSearchMatch={isCurrentSearchMatch} />;
    }
    let html = renderMarkdownToHtml(block.text);
    if (searchQuery) {
      html = highlightTextInHtml(html, searchQuery, highlightClass);
    }
    return (
      <div
        className={styles.textBlock}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    );
  }

  if (isThinkingBlock(block)) {
    const thinkingText = stripAnsi(block.thinking);
    // Empty thinking blocks (signature-only, no actual content) are protocol artifacts
    if (!thinkingText.trim()) return null;
    return (
      <div className={styles.thinkingBlock}>
        <div className={styles.thinkingHeader}>
          <span className={styles.thinkingIcon}>💭</span>
          <span className={styles.thinkingLabel}>Thinking</span>
        </div>
        <div className={styles.thinkingContent}>
          <pre>
            {searchQuery
              ? splitTextByQuery(thinkingText, searchQuery).map((segment, i) =>
                  typeof segment === 'string'
                    ? segment
                    : <mark key={i} className={highlightClass}>{segment.match}</mark>
                )
              : thinkingText
            }
          </pre>
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
          <CodeBlock code={JSON.stringify(block.input, null, 2)} language="json" searchQuery={searchQuery} isCurrentSearchMatch={isCurrentSearchMatch} />
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
              <BashOutput output={block.content} searchQuery={searchQuery} isCurrentSearchMatch={isCurrentSearchMatch} />
            ) : (
              <CodeBlock code={block.content} language="plain" maxHeight="500px" truncateLines={100} searchQuery={searchQuery} isCurrentSearchMatch={isCurrentSearchMatch} />
            )
          ) : (
            // Recursive rendering for nested content blocks
            block.content.map((nestedBlock, i) => (
              <ContentBlock
                key={i}
                block={nestedBlock}
                toolName={toolName}
                searchQuery={searchQuery}
                isCurrentSearchMatch={isCurrentSearchMatch}
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

  if (isToolReferenceBlock(block)) {
    return (
      <div className={styles.toolReference}>
        <span className={styles.toolReferenceIcon}>🔗</span>
        <span className={styles.toolReferenceName}>{block.tool_name}</span>
      </div>
    );
  }

  // Forward-compatibility: render unknown block types with best-effort content
  warnIfKnownTypeCaughtByCatchall('block', block.type);

  let bestEffortText: string | null = null;
  if ('text' in block && typeof block.text === 'string') {
    bestEffortText = block.text;
  } else if ('content' in block && typeof block.content === 'string') {
    bestEffortText = block.content;
  }

  return (
    <div className={styles.unknownBlock}>
      <em>Unknown content block: {block.type}</em>
      {bestEffortText && (
        <pre className={styles.unknownBlockContent}>{bestEffortText}</pre>
      )}
    </div>
  );
}

export default ContentBlock;
