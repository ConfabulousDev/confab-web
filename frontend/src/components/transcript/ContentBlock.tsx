import { useState, useEffect } from 'react';
import type { ContentBlock as ContentBlockType } from '@/types';
import { isTextBlock, isThinkingBlock, isToolUseBlock, isToolResultBlock } from '@/types/transcript';
import CodeBlock from './CodeBlock';
import BashOutput from './BashOutput';
import styles from './ContentBlock.module.css';

interface ContentBlockProps {
  block: ContentBlockType;
  index?: number;
  toolName?: string;
  showThinking?: boolean;
  expandAllTools?: boolean;
  expandAllResults?: boolean;
}

function ContentBlock({
  block,
  // index = 0, // Reserved for future use
  toolName: initialToolName = '',
  showThinking = true,
  expandAllTools = false,
  expandAllResults = true,
}: ContentBlockProps) {
  const [toolExpanded, setToolExpanded] = useState(expandAllTools);
  const [toolResultExpanded, setToolResultExpanded] = useState(expandAllResults);
  const [thinkingExpanded, setThinkingExpanded] = useState(false);
  const [toolName, setToolName] = useState(initialToolName);

  // React to changes in expand all controls
  useEffect(() => {
    setToolExpanded(expandAllTools);
  }, [expandAllTools]);

  useEffect(() => {
    setToolResultExpanded(expandAllResults);
  }, [expandAllResults]);

  // Track tool name from tool_use blocks
  useEffect(() => {
    if (isToolUseBlock(block)) {
      setToolName(block.name);
    }
  }, [block]);

  // Auto-link URLs in text
  function linkify(text: string): string {
    const urlRegex = /(https?:\/\/[^\s]+)/g;
    return text.replace(urlRegex, '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>');
  }

  // Detect if this is Bash-like output
  function isBashOutput(content: string, tool: string): boolean {
    if (tool === 'Bash') return true;
    // Heuristic: check for common bash patterns
    return content.includes('$ ') || content.match(/^[\w@-]+:/) !== null || content.includes('\n$ ');
  }

  if (isTextBlock(block)) {
    return (
      <div className={styles.textBlock}>
        <pre dangerouslySetInnerHTML={{ __html: linkify(block.text) }} />
      </div>
    );
  }

  if (isThinkingBlock(block)) {
    if (!showThinking) return null;

    return (
      <div className={styles.thinkingBlock}>
        <div className={styles.thinkingHeader} onClick={() => setThinkingExpanded(!thinkingExpanded)}>
          <span className={styles.thinkingIcon}>💭</span>
          <span className={styles.thinkingLabel}>Thinking</span>
          <button className={styles.expandBtn}>{thinkingExpanded ? '▼' : '▶'}</button>
        </div>
        {thinkingExpanded && (
          <div className={styles.thinkingContent}>
            <pre>{block.thinking}</pre>
          </div>
        )}
      </div>
    );
  }

  if (isToolUseBlock(block)) {
    return (
      <div className={styles.toolUseBlock}>
        <div className={styles.toolHeader} onClick={() => setToolExpanded(!toolExpanded)}>
          <span className={styles.toolIcon}>🛠️</span>
          <span className={styles.toolName}>{block.name}</span>
          <button className={styles.expandBtn}>{toolExpanded ? '▼' : '▶'}</button>
        </div>
        {toolExpanded && (
          <div className={styles.toolInput}>
            <CodeBlock code={JSON.stringify(block.input, null, 2)} language="json" />
          </div>
        )}
      </div>
    );
  }

  if (isToolResultBlock(block)) {
    return (
      <div className={`${styles.toolResultBlock} ${block.is_error ? styles.error : ''}`}>
        <div className={styles.toolResultHeader} onClick={() => setToolResultExpanded(!toolResultExpanded)}>
          <span className={styles.resultIcon}>{block.is_error ? '❌' : '✅'}</span>
          <span>Tool Result</span>
          <button className={styles.expandBtn}>{toolResultExpanded ? '▼' : '▶'}</button>
        </div>
        {toolResultExpanded && (
          <div className={styles.toolResultContent}>
            {typeof block.content === 'string' ? (
              isBashOutput(block.content, toolName) ? (
                <BashOutput output={block.content} />
              ) : (
                <CodeBlock code={block.content} language="plain" maxHeight="500px" truncateLines={100} />
              )
            ) : (
              // Recursive rendering for nested content blocks
              (block.content as any[]).map((nestedBlock: ContentBlockType, i: number) => (
                <ContentBlock
                  key={i}
                  block={nestedBlock}
                  toolName={toolName}
                  showThinking={showThinking}
                  expandAllTools={expandAllTools}
                  expandAllResults={expandAllResults}
                />
              ))
            )}
          </div>
        )}
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
