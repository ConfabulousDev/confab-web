import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAPI, APIError } from '@/services/api';
import { formatRelativeTime } from '@/utils';
import Button from '@/components/Button';
import Alert from '@/components/Alert';
import Modal from '@/components/Modal';
import LoadingSkeleton from '@/components/LoadingSkeleton';
import ErrorDisplay from '@/components/ErrorDisplay';
import styles from './AdminSettingsPage.module.css';

// ---------------------------------------------------------------------------
// Collapsible section for fixed prompt parts
// ---------------------------------------------------------------------------
function CollapsibleSection({ title, content }: { title: string; content: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={styles.collapsibleSection}>
      <button
        type="button"
        className={styles.collapsibleToggle}
        onClick={() => setOpen((prev) => !prev)}
      >
        <span className={`${styles.toggleArrow} ${open ? styles.toggleArrowOpen : ''}`}>&#9654;</span>
        {title}
      </button>
      {open && <div className={styles.collapsibleContent}>{content}</div>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Presentational component (exported for Storybook)
// ---------------------------------------------------------------------------
export interface AdminSettingsPageContentProps {
  // Prompt data
  instructions: string;
  isCustom: boolean;
  updatedAt?: string;
  inputFormat: string;
  outputSchema: string;
  example: string;
  defaultInstructions: string;

  // Editor state
  editedInstructions: string;
  onEditedInstructionsChange: (value: string) => void;
  isDirty: boolean;

  // Action handlers
  onSave: () => void;
  isSaving: boolean;
  onResetToDefault: () => void;

  // Regenerate
  onRegenerateClick: () => void;

  // Feedback
  feedback: { type: 'success' | 'error'; message: string } | null;
  onFeedbackClose: () => void;

  // Modals
  showResetModal: boolean;
  onResetModalClose: () => void;
  onResetModalConfirm: () => void;
  isResetting: boolean;

  showRegenerateModal: boolean;
  onRegenerateModalClose: () => void;
  onRegenerateModalConfirm: () => void;
  isRegenerating: boolean;
  regenerateCount: number | null;
  regenerateCountLoading: boolean;
}

export function AdminSettingsPageContent({
  isCustom,
  updatedAt,
  inputFormat,
  outputSchema,
  example,
  defaultInstructions,
  editedInstructions,
  onEditedInstructionsChange,
  isDirty,
  onSave,
  isSaving,
  onResetToDefault,
  onRegenerateClick,
  feedback,
  onFeedbackClose,
  showResetModal,
  onResetModalClose,
  onResetModalConfirm,
  isResetting,
  showRegenerateModal,
  onRegenerateModalClose,
  onRegenerateModalConfirm,
  isRegenerating,
  regenerateCount,
  regenerateCountLoading,
}: AdminSettingsPageContentProps) {
  return (
    <div>
      {feedback && (
        <Alert variant={feedback.type} onClose={onFeedbackClose}>
          {feedback.message}
        </Alert>
      )}

      {/* Card 1: Smart Recap Prompt */}
      <div className={styles.card}>
        <div className={styles.headerRow}>
          <div className={styles.headerLeft}>
            <h3>Smart Recap Instructions</h3>
            <span className={`${styles.statusChip} ${isCustom ? styles.statusCustom : styles.statusDefault}`}>
              {isCustom ? 'Custom' : 'Default'}
            </span>
          </div>
          {isCustom && updatedAt && (
            <span className={styles.metadata}>
              Last modified {formatRelativeTime(updatedAt)}
            </span>
          )}
        </div>

        {/* Collapsible fixed sections */}
        <div className={styles.fixedSections}>
          <CollapsibleSection title="Input Format (read-only)" content={inputFormat} />
          <CollapsibleSection title="Output Schema (read-only)" content={outputSchema} />
          <CollapsibleSection title="Example Output (read-only)" content={example} />
        </div>

        {/* Side-by-side editor */}
        <div className={styles.editorArea}>
          <div className={styles.editorPanel}>
            <label className={styles.editorLabel}>Custom Instructions</label>
            <textarea
              className={styles.textarea}
              value={editedInstructions}
              onChange={(e) => onEditedInstructionsChange(e.target.value)}
              spellCheck={false}
            />
            <div className={styles.charCount}>
              {editedInstructions.length.toLocaleString()} characters
            </div>
          </div>
          <div className={styles.editorPanel}>
            <label className={styles.editorLabel}>Default Instructions</label>
            <textarea
              className={styles.textareaReadOnly}
              value={defaultInstructions}
              readOnly
              tabIndex={-1}
            />
          </div>
        </div>

        {/* Action buttons */}
        <div className={styles.actions}>
          <Button variant="primary" onClick={onSave} disabled={!isDirty || isSaving}>
            {isSaving ? 'Saving...' : 'Save'}
          </Button>
          {isCustom && (
            <Button variant="secondary" onClick={onResetToDefault} disabled={isResetting}>
              Reset to Default
            </Button>
          )}
        </div>
      </div>

      {/* Card 2: Bulk Operations */}
      <div className={styles.bulkCard}>
        <h3>Bulk Operations</h3>
        <p className={styles.bulkDescription}>
          Regenerate all smart recaps using the current prompt. This queues background
          LLM inference for every session that has a cached recap.
        </p>
        <Button variant="secondary" onClick={onRegenerateClick}>
          Regenerate All Recaps
        </Button>
      </div>

      {/* Reset confirmation modal */}
      <Modal
        isOpen={showResetModal}
        onClose={onResetModalClose}
        ariaLabel="Confirm reset to default"
      >
        <div className={styles.confirmModal}>
          <h3>Reset to Default</h3>
          <p>
            This will discard your custom instructions and revert to the built-in default.
            Are you sure?
          </p>
          <div className={styles.modalActions}>
            <Button variant="danger" onClick={onResetModalConfirm} disabled={isResetting}>
              {isResetting ? 'Resetting...' : 'Reset'}
            </Button>
            <Button variant="secondary" onClick={onResetModalClose}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>

      {/* Regenerate confirmation modal */}
      <Modal
        isOpen={showRegenerateModal}
        onClose={onRegenerateModalClose}
        ariaLabel="Confirm regenerate all recaps"
      >
        <div className={styles.confirmModal}>
          <h3>Regenerate All Recaps</h3>
          <p>
            {regenerateCountLoading
              ? 'Loading session count...'
              : `This will re-run LLM inference for up to ${regenerateCount?.toLocaleString() ?? 0} sessions. Are you sure?`}
          </p>
          <div className={styles.modalActions}>
            <Button
              variant="primary"
              onClick={onRegenerateModalConfirm}
              disabled={isRegenerating || regenerateCountLoading}
            >
              {isRegenerating ? 'Regenerating...' : 'Regenerate'}
            </Button>
            <Button variant="secondary" onClick={onRegenerateModalClose}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Connected component (default export)
// ---------------------------------------------------------------------------
function AdminSettingsPage() {
  const queryClient = useQueryClient();
  const [editedInstructions, setEditedInstructions] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [showResetModal, setShowResetModal] = useState(false);
  const [showRegenerateModal, setShowRegenerateModal] = useState(false);

  const promptQueryKey = ['admin', 'smart-recap-prompt'];
  const defaultQueryKey = ['admin', 'smart-recap-prompt-default'];
  const countQueryKey = ['admin', 'smart-recap-regenerate-count'];

  // Fetch current prompt state
  const {
    data: promptData,
    isLoading: promptLoading,
    error: promptError,
    refetch: refetchPrompt,
  } = useQuery({
    queryKey: promptQueryKey,
    queryFn: adminAPI.getSmartRecapPrompt,
  });

  // Fetch default instructions
  const {
    data: defaultData,
    isLoading: defaultLoading,
  } = useQuery({
    queryKey: defaultQueryKey,
    queryFn: adminAPI.getSmartRecapPromptDefault,
  });

  // Fetch regenerate count (only when modal is open)
  const {
    data: countData,
    isLoading: countLoading,
  } = useQuery({
    queryKey: countQueryKey,
    queryFn: adminAPI.getSmartRecapRegenerateCount,
    enabled: showRegenerateModal,
  });

  function formatMutationError(err: unknown, fallback: string): string {
    return err instanceof APIError ? err.message : fallback;
  }

  // Save mutation
  const saveMutation = useMutation({
    mutationFn: adminAPI.setSmartRecapPrompt,
    onSuccess: () => {
      setFeedback({ type: 'success', message: 'Custom instructions saved.' });
      setEditedInstructions(null);
      queryClient.invalidateQueries({ queryKey: promptQueryKey });
    },
    onError: (err) => {
      setFeedback({ type: 'error', message: formatMutationError(err, 'Failed to save instructions.') });
    },
  });

  // Reset mutation
  const resetMutation = useMutation({
    mutationFn: adminAPI.deleteSmartRecapPrompt,
    onSuccess: () => {
      setFeedback({ type: 'success', message: 'Instructions reset to default.' });
      setEditedInstructions(null);
      setShowResetModal(false);
      queryClient.invalidateQueries({ queryKey: promptQueryKey });
    },
    onError: (err) => {
      setFeedback({ type: 'error', message: formatMutationError(err, 'Failed to reset instructions.') });
      setShowResetModal(false);
    },
  });

  // Regenerate mutation
  const regenerateMutation = useMutation({
    mutationFn: adminAPI.regenerateAllSmartRecaps,
    onSuccess: (result) => {
      setFeedback({ type: 'success', message: `Queued ${result.sessions_queued.toLocaleString()} sessions for regeneration.` });
      setShowRegenerateModal(false);
    },
    onError: (err) => {
      setFeedback({ type: 'error', message: formatMutationError(err, 'Failed to trigger regeneration.') });
      setShowRegenerateModal(false);
    },
  });

  const isLoading = promptLoading || defaultLoading;

  if (isLoading) {
    return <LoadingSkeleton variant="card" count={2} />;
  }

  if (promptError) {
    return (
      <ErrorDisplay
        message={promptError instanceof Error ? promptError.message : 'Failed to load smart recap settings'}
        retry={refetchPrompt}
      />
    );
  }

  if (!promptData || !defaultData) {
    return null;
  }

  const currentInstructions = editedInstructions ?? promptData.instructions;
  const isDirty = currentInstructions !== promptData.instructions;

  return (
    <AdminSettingsPageContent
      instructions={promptData.instructions}
      isCustom={promptData.is_custom}
      updatedAt={promptData.updated_at}
      inputFormat={promptData.input_format}
      outputSchema={promptData.output_schema}
      example={promptData.example}
      defaultInstructions={defaultData.instructions}
      editedInstructions={currentInstructions}
      onEditedInstructionsChange={setEditedInstructions}
      isDirty={isDirty}
      onSave={() => saveMutation.mutate({ instructions: currentInstructions })}
      isSaving={saveMutation.isPending}
      onResetToDefault={() => setShowResetModal(true)}
      onRegenerateClick={() => setShowRegenerateModal(true)}
      feedback={feedback}
      onFeedbackClose={() => setFeedback(null)}
      showResetModal={showResetModal}
      onResetModalClose={() => setShowResetModal(false)}
      onResetModalConfirm={() => resetMutation.mutate()}
      isResetting={resetMutation.isPending}
      showRegenerateModal={showRegenerateModal}
      onRegenerateModalClose={() => setShowRegenerateModal(false)}
      onRegenerateModalConfirm={() => regenerateMutation.mutate()}
      isRegenerating={regenerateMutation.isPending}
      regenerateCount={countData?.count ?? null}
      regenerateCountLoading={countLoading}
    />
  );
}

export default AdminSettingsPage;
