import { useState, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { keysAPI, APIError } from '@/services/api';
import { useDocumentTitle, useCopyToClipboard } from '@/hooks';
import { formatRelativeTime } from '@/utils';
import { createAPIKeySchema, validateForm, getFieldError } from '@/schemas/validation';
import type { CreateAPIKeyData } from '@/schemas/validation';
import PageHeader from '@/components/PageHeader';
import PageSidebar, { SidebarItem } from '@/components/PageSidebar';
import FormField from '@/components/FormField';
import LoadingSkeleton from '@/components/LoadingSkeleton';
import ErrorDisplay from '@/components/ErrorDisplay';
import Button from '@/components/Button';
import Alert from '@/components/Alert';
import styles from './APIKeysPage.module.css';

const MAX_API_KEYS = 100;

// SVG Icons
const AllIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
  </svg>
);

const ActiveIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
    <polyline points="22 4 12 14.01 9 11.01" />
  </svg>
);

const UnusedIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
    <line x1="12" y1="8" x2="12" y2="12" />
    <line x1="12" y1="16" x2="12.01" y2="16" />
  </svg>
);

const KeyIcon = (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
  </svg>
);

const ClockIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <circle cx="12" cy="12" r="10" />
    <polyline points="12 6 12 12 16 14" />
  </svg>
);

type FilterType = 'all' | 'active' | 'unused';

function APIKeysPage() {
  useDocumentTitle('API Keys');
  const queryClient = useQueryClient();
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<{ key: string; name: string } | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [validationErrors, setValidationErrors] = useState<Record<string, string[]>>();
  const { copy, copied } = useCopyToClipboard();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [filter, setFilter] = useState<FilterType>('all');

  // Fetch API keys
  const { data: keys = [], isLoading, error, refetch } = useQuery({
    queryKey: ['apiKeys'],
    queryFn: keysAPI.list,
  });

  // Filter counts
  const counts = useMemo(() => {
    const active = keys.filter((k) => k.last_used_at).length;
    const unused = keys.filter((k) => !k.last_used_at).length;
    return { all: keys.length, active, unused };
  }, [keys]);

  // Filtered keys
  const filteredKeys = useMemo(() => {
    switch (filter) {
      case 'active':
        return keys.filter((k) => k.last_used_at);
      case 'unused':
        return keys.filter((k) => !k.last_used_at);
      default:
        return keys;
    }
  }, [keys, filter]);

  // Create API key mutation
  const createMutation = useMutation({
    mutationFn: (name: string) => keysAPI.create(name),
    onSuccess: (result) => {
      setCreatedKey({ key: result.key, name: result.name });
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
    <div className={styles.pageWrapper}>
      <PageSidebar
        title="API Keys"
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
      >
        <SidebarItem
          icon={AllIcon}
          label="All Keys"
          count={counts.all}
          active={filter === 'all'}
          onClick={() => setFilter('all')}
          collapsed={sidebarCollapsed}
        />
        <SidebarItem
          icon={ActiveIcon}
          label="Recently Used"
          count={counts.active}
          active={filter === 'active'}
          onClick={() => setFilter('active')}
          collapsed={sidebarCollapsed}
          disabled={counts.active === 0}
        />
        <SidebarItem
          icon={UnusedIcon}
          label="Never Used"
          count={counts.unused}
          active={filter === 'unused'}
          onClick={() => setFilter('unused')}
          collapsed={sidebarCollapsed}
          disabled={counts.unused === 0}
        />
      </PageSidebar>

      <div className={`${styles.mainContent} ${sidebarCollapsed ? styles.sidebarCollapsed : ''}`}>
        <PageHeader
          title="API Keys"
          subtitle={`${filteredKeys.length} key${filteredKeys.length !== 1 ? 's' : ''}`}
          actions={
            !showCreateForm && (
              <Button variant="primary" onClick={() => setShowCreateForm(true)}>
                + Create New Key
              </Button>
            )
          }
        />

        <div className={styles.container}>
          {createdKey && (
            <Alert variant="success" onClose={() => setCreatedKey(null)}>
              <div className={styles.successContent}>
                <strong>API Key Created Successfully</strong>
                <p>Name: {createdKey.name}</p>
                <div className={styles.keyDisplay}>
                  <code>{createdKey.key}</code>
                  <Button size="sm" onClick={() => copy(createdKey.key)}>
                    {copied ? 'Copied!' : 'Copy'}
                  </Button>
                </div>
                <Alert variant="warning">
                  This is the only time you'll see this key. Save it securely!
                </Alert>
              </div>
            </Alert>
          )}

          {error && <ErrorDisplay message={error instanceof Error ? error.message : 'Failed to load API keys'} retry={refetch} />}
          {createMutation.error && (
            <Alert variant="error">
              {createMutation.error instanceof APIError && createMutation.error.message.includes('limit') ? (
                <>
                  <strong>API Key Limit Reached</strong>
                  <p>You have reached the maximum of {MAX_API_KEYS} API keys. Please delete some unused keys below before creating new ones.</p>
                </>
              ) : createMutation.error instanceof APIError && createMutation.error.message.includes('already exists') ? (
                <>
                  <strong>Duplicate Key Name</strong>
                  <p>An API key with this name already exists. Please choose a different name.</p>
                </>
              ) : (
                createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create key'
              )}
            </Alert>
          )}
          {deleteMutation.error && <Alert variant="error">{deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete key'}</Alert>}

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

          <div className={styles.card}>
            {isLoading ? (
              <LoadingSkeleton variant="list" count={3} />
            ) : filteredKeys.length === 0 ? (
              <p className={styles.empty}>
                {filter === 'all'
                  ? 'No API keys yet. Create one to get started!'
                  : filter === 'active'
                    ? 'No recently used keys.'
                    : 'No unused keys.'}
              </p>
            ) : (
              <div className={styles.keysList}>
                {filteredKeys.map((key) => (
                  <div key={key.id} className={styles.keyItem}>
                    <div className={styles.keyIcon}>
                      {KeyIcon}
                    </div>
                    <div className={styles.keyInfo}>
                      <h3>{key.name}</h3>
                      <div className={styles.keyMeta}>
                        <span className={styles.metaItem}>
                          {ClockIcon}
                          Created {formatRelativeTime(key.created_at)}
                        </span>
                        <span className={styles.metaItem}>
                          {key.last_used_at ? (
                            <>Last used {formatRelativeTime(key.last_used_at)}</>
                          ) : (
                            <span className={styles.unused}>Never used</span>
                          )}
                        </span>
                      </div>
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
      </div>
    </div>
  );
}

export default APIKeysPage;
