import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { fetchWithCSRF } from '@/services/csrf';
import type { APIKey } from '@/types';
import { formatDate } from '@/utils/utils';
import styles from './APIKeysPage.module.css';

function APIKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<{ key: string; name: string } | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);

  useEffect(() => {
    fetchKeys();
  }, []);

  const fetchKeys = async () => {
    setLoading(true);
    setError('');
    try {
      const response = await fetch('/api/v1/keys', {
        credentials: 'include',
      });

      if (response.status === 401) {
        window.location.href = '/';
        return;
      }

      if (!response.ok) {
        throw new Error('Failed to fetch keys');
      }

      const data = await response.json();
      setKeys(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load API keys');
    } finally {
      setLoading(false);
    }
  };

  const createKey = async () => {
    if (!newKeyName.trim()) {
      setError('Please enter a key name');
      return;
    }

    setError('');
    try {
      const response = await fetchWithCSRF('/api/v1/keys', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: newKeyName }),
      });

      if (!response.ok) {
        throw new Error('Failed to create key');
      }

      const result = await response.json();
      setCreatedKey({ key: result.key, name: result.name });
      setNewKeyName('');
      setShowCreateForm(false);
      await fetchKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create key');
    }
  };

  const deleteKey = async (id: number, name: string) => {
    if (!confirm(`Are you sure you want to delete "${name}"? This cannot be undone.`)) {
      return;
    }

    setError('');
    try {
      const response = await fetchWithCSRF(`/api/v1/keys/${id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to delete key');
      }

      await fetchKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete key');
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    alert('API key copied to clipboard!');
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>API Keys</h1>
        <Link to="/" className={styles.btnLink}>
          ← Back to Home
        </Link>
      </div>

      {createdKey && (
        <div className={styles.alertSuccess}>
          <h3>✓ API Key Created Successfully!</h3>
          <p>
            <strong>Name:</strong> {createdKey.name}
          </p>
          <div className={styles.keyDisplay}>
            <code>{createdKey.key}</code>
            <button className={`${styles.btn} ${styles.btnSm}`} onClick={() => copyToClipboard(createdKey.key)}>
              Copy
            </button>
          </div>
          <p className={styles.warning}>
            ⚠️ This is the only time you'll see this key. Save it securely!
          </p>
          <button className={styles.btn} onClick={() => setCreatedKey(null)}>
            Close
          </button>
        </div>
      )}

      {error && <div className={styles.alertError}>{error}</div>}

      <div className={styles.card}>
        <div className={styles.cardHeader}>
          <h2>Your API Keys</h2>
          {!showCreateForm && (
            <button className={`${styles.btn} ${styles.btnPrimary}`} onClick={() => setShowCreateForm(true)}>
              + Create New Key
            </button>
          )}
        </div>

        {showCreateForm && (
          <div className={styles.createForm}>
            <h3>Create New API Key</h3>
            <input
              type="text"
              placeholder="Key name (e.g., Production Server, My Laptop)"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              className={styles.input}
            />
            <div className={styles.formActions}>
              <button className={`${styles.btn} ${styles.btnPrimary}`} onClick={createKey}>
                Create Key
              </button>
              <button className={`${styles.btn} ${styles.btnSecondary}`} onClick={() => setShowCreateForm(false)}>
                Cancel
              </button>
            </div>
          </div>
        )}

        {loading ? (
          <p className={styles.loading}>Loading...</p>
        ) : keys.length === 0 ? (
          <p className={styles.empty}>No API keys yet. Create one to get started!</p>
        ) : (
          <div className={styles.keysList}>
            {keys.map((key) => (
              <div key={key.id} className={styles.keyItem}>
                <div className={styles.keyInfo}>
                  <h3>{key.name}</h3>
                  <p className={styles.keyMeta}>Created: {formatDate(key.created_at)}</p>
                </div>
                <button
                  className={`${styles.btn} ${styles.btnDanger} ${styles.btnSm}`}
                  onClick={() => deleteKey(key.id, key.name)}
                >
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className={styles.help}>
        <h3>Using API Keys</h3>
        <p>Use your API key with the Confab CLI:</p>
        <pre>
          <code>confab configure --api-key &lt;your-key&gt;</code>
        </pre>
        <p>
          Or use <code>confab login</code> for interactive authentication.
        </p>
      </div>
    </div>
  );
}

export default APIKeysPage;
