// ABOUTME: Modal for extracting a learning artifact from a session transcript
// ABOUTME: Provides a form to capture title, body, and tags, then persists via the learnings API
import { useState, useEffect, useCallback } from 'react';
import Modal from '@/components/Modal';
import Alert from '@/components/Alert';
import { useCreateLearning } from '@/hooks/useLearnings';
import styles from './ExtractLearningModal.module.css';

interface ExtractLearningModalProps {
  isOpen: boolean;
  onClose: () => void;
  sessionId: string;
  selectedText?: string;
}

function ExtractLearningModal({ isOpen, onClose, sessionId, selectedText }: ExtractLearningModalProps) {
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [tagsInput, setTagsInput] = useState('');
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const createLearning = useCreateLearning();

  // Pre-fill body with selected text when modal opens
  useEffect(() => {
    if (isOpen) {
      setBody(selectedText ?? '');
      setTitle('');
      setTagsInput('');
      setSuccessMessage(null);
      createLearning.reset();
    }
    // Only react to isOpen/selectedText changes, not createLearning
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, selectedText]);

  const parseTags = useCallback((input: string): string[] => {
    return input
      .split(',')
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0);
  }, []);

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (!title.trim()) return;

      createLearning.mutate(
        {
          title: title.trim(),
          body: body.trim(),
          tags: parseTags(tagsInput),
          source: 'manual_review',
          session_ids: [sessionId],
        },
        {
          onSuccess: () => {
            setSuccessMessage('Learning saved.');
            setTimeout(() => {
              onClose();
            }, 1200);
          },
        },
      );
    },
    [title, body, tagsInput, sessionId, createLearning, parseTags, onClose],
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} ariaLabel="Extract Learning" className={styles.extractModal}>
      <h2 className={styles.title}>Extract Learning</h2>

      {successMessage && <Alert variant="success">{successMessage}</Alert>}

      {createLearning.isError && (
        <Alert variant="error">
          {createLearning.error instanceof Error
            ? createLearning.error.message
            : 'Failed to save learning'}
        </Alert>
      )}

      <form className={styles.form} onSubmit={handleSubmit}>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="learning-title">
            Title
          </label>
          <input
            id="learning-title"
            className={styles.input}
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="What did you learn?"
            required
            autoFocus
          />
        </div>

        <div className={styles.field}>
          <label className={styles.label} htmlFor="learning-body">
            Body
          </label>
          <textarea
            id="learning-body"
            className={styles.textarea}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Describe the learning in detail..."
          />
        </div>

        <div className={styles.field}>
          <label className={styles.label} htmlFor="learning-tags">
            Tags
          </label>
          <input
            id="learning-tags"
            className={styles.input}
            type="text"
            value={tagsInput}
            onChange={(e) => setTagsInput(e.target.value)}
            placeholder="e.g. debugging, openshift, helm"
          />
          <span className={styles.hint}>Comma-separated</span>
        </div>

        <div className={styles.actions}>
          <button type="button" className={styles.cancelBtn} onClick={onClose}>
            Cancel
          </button>
          <button
            type="submit"
            className={styles.submitBtn}
            disabled={!title.trim() || createLearning.isPending}
          >
            {createLearning.isPending ? 'Saving...' : 'Save Learning'}
          </button>
        </div>
      </form>
    </Modal>
  );
}

export default ExtractLearningModal;
