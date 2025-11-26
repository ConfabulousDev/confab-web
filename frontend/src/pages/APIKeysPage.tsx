import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { keysAPI } from '@/services/api';
import { useDocumentTitle, useCopyToClipboard } from '@/hooks';
import { formatRelativeTime } from '@/utils';
import { createAPIKeySchema, validateForm, getFieldError } from '@/schemas/validation';
import type { CreateAPIKeyData } from '@/schemas/validation';
import FormField from '@/components/FormField';
import LoadingSkeleton from '@/components/LoadingSkeleton';
import ErrorDisplay from '@/components/ErrorDisplay';
import Button from '@/components/Button';
import Alert from '@/components/Alert';
import styles from './APIKeysPage.module.css';

function APIKeysPage() {
  useDocumentTitle('API Keys');
  const queryClient = useQueryClient();
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<{ key: string; name: string } | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [validationErrors, setValidationErrors] = useState<Record<string, string[]>>();
  const { copy, copied } = useCopyToClipboard();

  // Fetch API keys
  const { data: keys = [], isLoading, error, refetch } = useQuery({
    queryKey: ['apiKeys'],
    queryFn: keysAPI.list,
  });

  // Create API key mutation
  const createMutation = useMutation({
    mutationFn: (name: string) => keysAPI.create(name),
    onSuccess: (result) => {
      setCreatedKey({ key: result.key, name: result.api_key.name });
      setNewKeyName('');
      setShowCreateForm(false);
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
    },
  });

  // Delete API key mutation
  const deleteMutation = useMutation({
    mutationFn: (id: number) => keysAPI.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
    },
  });

  const createKey = async () => {
    setValidationErrors(undefined);

    // Validate form data with Zod
    const formData: CreateAPIKeyData = { name: newKeyName };
    const validation = validateForm(createAPIKeySchema, formData);

    if (!validation.success) {
      setValidationErrors(validation.errors);
      return;
    }

    createMutation.mutate(validation.data.name);
  };

  const deleteKey = async (id: number, name: string) => {
    if (!confirm(`Are you sure you want to delete "${name}"? This cannot be undone.`)) {
      return;
    }
    deleteMutation.mutate(id);
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>API Keys</h1>
      </div>

      {createdKey && (
        <Alert variant="success" onClose={() => setCreatedKey(null)}>
          <h3>✓ API Key Created Successfully!</h3>
          <p>
            <strong>Name:</strong> {createdKey.name}
          </p>
          <div className={styles.keyDisplay}>
            <code>{createdKey.key}</code>
            <Button size="sm" onClick={() => copy(createdKey.key)}>
              {copied ? 'Copied!' : 'Copy'}
            </Button>
          </div>
          <Alert variant="warning">
            ⚠️ This is the only time you'll see this key. Save it securely!
          </Alert>
        </Alert>
      )}

      {error && <ErrorDisplay message={error instanceof Error ? error.message : 'Failed to load API keys'} retry={refetch} />}
      {createMutation.error && <Alert variant="error">{createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create key'}</Alert>}
      {deleteMutation.error && <Alert variant="error">{deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete key'}</Alert>}

      <div className={styles.card}>
        <div className={styles.cardHeader}>
          {!showCreateForm && (
            <Button variant="primary" onClick={() => setShowCreateForm(true)}>
              + Create New Key
            </Button>
          )}
        </div>

        {showCreateForm && (
          <div className={styles.createForm}>
            <h3>Create New API Key</h3>
            <FormField
              label="Key name"
              required
              error={getFieldError(validationErrors, 'name')}
            >
              <input
                type="text"
                placeholder="e.g., Production Server, My Laptop"
                value={newKeyName}
                onChange={(e) => {
                  setNewKeyName(e.target.value);
                  setValidationErrors(undefined);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    createKey();
                  }
                }}
                className={styles.input}
                disabled={createMutation.isPending}
              />
            </FormField>
            <div className={styles.formActions}>
              <Button
                variant="primary"
                onClick={createKey}
                disabled={createMutation.isPending}
              >
                {createMutation.isPending ? 'Creating...' : 'Create Key'}
              </Button>
              <Button
                variant="secondary"
                onClick={() => {
                  setShowCreateForm(false);
                  setValidationErrors(undefined);
                  setNewKeyName('');
                }}
                disabled={createMutation.isPending}
              >
                Cancel
              </Button>
            </div>
          </div>
        )}

        {isLoading ? (
          <LoadingSkeleton variant="list" count={3} />
        ) : keys.length === 0 ? (
          <p className={styles.empty}>No API keys yet. Create one to get started!</p>
        ) : (
          <div className={styles.keysList}>
            {keys.map((key) => (
              <div key={key.id} className={styles.keyItem}>
                <div className={styles.keyInfo}>
                  <h3>{key.name}</h3>
                  <p className={styles.keyMeta}>Created {formatRelativeTime(key.created_at)}</p>
                </div>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => deleteKey(key.id, key.name)}
                >
                  Delete
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

    </div>
  );
}

export default APIKeysPage;
