import { useState } from 'react';
import type { TranscriptValidationError } from '@/services/transcriptService';
import { useToggleSet } from '@/hooks';
import styles from './ValidationErrorsPanel.module.css';

interface ValidationErrorsPanelProps {
  errors: TranscriptValidationError[];
}

function ValidationErrorsPanel({ errors }: ValidationErrorsPanelProps) {
  const [expanded, setExpanded] = useState(false);
  const expandedErrors = useToggleSet<number>();

  if (errors.length === 0) return null;

  const displayErrors = expanded ? errors : errors.slice(0, 3);

  return (
    <div className={styles.panel}>
      <div className={styles.header} onClick={() => setExpanded(!expanded)}>
        <span className={styles.warningIcon}>⚠️</span>
        <span className={styles.title}>
          {errors.length} message{errors.length === 1 ? '' : 's'} failed to parse
        </span>
        <span className={styles.toggleIcon}>{expanded ? '▼' : '▶'}</span>
      </div>

      {expanded && (
        <div className={styles.description}>
          Some transcript lines could not be validated. This may be due to schema changes or
          corrupted data. The affected messages are skipped but you can see the details below.
        </div>
      )}

      <div className={styles.errorList}>
        {displayErrors.map((error, index) => (
          <div key={index} className={styles.errorItem}>
            <div className={styles.errorHeader} onClick={() => expandedErrors.toggle(index)}>
              <span className={styles.lineNumber}>Line {error.line}</span>
              {error.messageType && <span className={styles.messageType}>type: {error.messageType}</span>}
              <span className={styles.errorSummary}>
                {error.errors.length} validation error{error.errors.length === 1 ? '' : 's'}
              </span>
              <span className={styles.expandIcon}>{expandedErrors.has(index) ? '−' : '+'}</span>
            </div>

            {expandedErrors.has(index) && (
              <div className={styles.errorDetails}>
                {error.errors.map((e, i) => (
                  <div key={i} className={styles.errorDetail}>
                    <code className={styles.path}>{e.path}</code>
                    <span className={styles.message}>{e.message}</span>
                    {e.expected && e.received && (
                      <span className={styles.expectedReceived}>
                        (expected: <code>{e.expected}</code>, got: <code>{e.received}</code>)
                      </span>
                    )}
                  </div>
                ))}
                <div className={styles.rawJson}>
                  <div className={styles.rawJsonLabel}>Raw JSON:</div>
                  <pre className={styles.rawJsonContent}>
                    {(() => {
                      try {
                        return JSON.stringify(JSON.parse(error.rawJson), null, 2);
                      } catch {
                        return error.rawJson;
                      }
                    })()}
                  </pre>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      {!expanded && errors.length > 3 && (
        <button className={styles.showMore} onClick={() => setExpanded(true)}>
          Show {errors.length - 3} more error{errors.length - 3 === 1 ? '' : 's'}
        </button>
      )}
    </div>
  );
}

export default ValidationErrorsPanel;
