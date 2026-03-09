import { useState, useEffect, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { sharesAPI, sessionsAPI, AuthenticationError } from '@/services/api';
import { useAppConfig, useDocumentTitle, useCopyToClipboard, useSuccessMessage } from '@/hooks';
import type { SessionShare } from '@/types';
import { formatRelativeTime, sortData, type SortDirection } from '@/utils';
import PageHeader from '@/components/PageHeader';
import PageSidebar, { SidebarItem } from '@/components/PageSidebar';
import SortableHeader from '@/components/SortableHeader';
import Alert from '@/components/Alert';
import Button from '@/components/Button';
import { UploadIcon, GlobeIcon, LockIcon, ActiveIcon, AlertCircleIcon, CopyIcon } from '@/components/icons';
import styles from './ShareLinksPage.module.css';

type SortColumn = 'session_summary' | 'is_public' | 'created_at' | 'expires_at';
type FilterType = 'all' | 'public' | 'private' | 'expired' | 'active';

function ShareLinksPage() {
  useDocumentTitle('Shares');
  const navigate = useNavigate();
  const { sharesEnabled } = useAppConfig();
  const [shares, setShares] = useState<SessionShare[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const { message: successMessage, setMessage: setSuccessMessage } = useSuccessMessage({
    skipUrlParams: true,
  });
  const { copy, message: copyMessage } = useCopyToClipboard({
    successMessage: 'Link copied to clipboard!',
  });
  const [sortColumn, setSortColumn] = useState<SortColumn>('created_at');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [filter, setFilter] = useState<FilterType>('all');

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortDirection(column === 'created_at' || column === 'expires_at' ? 'desc' : 'asc');
    }
  };

  // Filter counts
  const counts = useMemo(() => {
    const publicCount = shares.filter((s) => s.is_public).length;
    const privateCount = shares.filter((s) => !s.is_public).length;
    const expiredCount = shares.filter((s) => s.expires_at && new Date(s.expires_at) < new Date()).length;
    const activeCount = shares.length - expiredCount;
    return { all: shares.length, public: publicCount, private: privateCount, expired: expiredCount, active: activeCount };
  }, [shares]);

  // Filtered and sorted shares
  const sortedShares = useMemo(() => {
    let filtered = shares;
    switch (filter) {
      case 'public':
        filtered = shares.filter((s) => s.is_public);
        break;
      case 'private':
        filtered = shares.filter((s) => !s.is_public);
        break;
      case 'expired':
        filtered = shares.filter((s) => s.expires_at && new Date(s.expires_at) < new Date());
        break;
      case 'active':
        filtered = shares.filter((s) => !s.expires_at || new Date(s.expires_at) >= new Date());
        break;
    }
    return sortData({
      data: filtered,
      sortBy: sortColumn,
      direction: sortDirection,
    });
  }, [shares, filter, sortColumn, sortDirection]);

  useEffect(() => {
    if (!sharesEnabled) {
      navigate('/sessions', { replace: true });
    }
  }, [sharesEnabled, navigate]);

  useEffect(() => {
    fetchShares();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fetchShares is intentionally omitted; we only want to fetch on mount
  }, []);

  async function fetchShares() {
    setLoading(true);
    setError('');
    try {
      const data = await sharesAPI.list();
      setShares(data);
    } catch (err) {
      if (err instanceof AuthenticationError) {
        navigate('/');
        return;
      }
      setError(err instanceof Error ? err.message : 'Failed to load shares');
    } finally {
      setLoading(false);
    }
  }

  async function handleRevoke(shareId: number) {
    if (!confirm('Are you sure you want to revoke this share?')) {
      return;
    }

    setError('');
    try {
      await sessionsAPI.revokeShare(shareId);
      setSuccessMessage('Share revoked successfully');
      await fetchShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke share');
    }
  }

  function getShareURL(sessionId: string): string {
    // CF-132: Use canonical URL format (no token in URL)
    return `${window.location.origin}/sessions/${sessionId}`;
  }

  const displayMessage = copyMessage || successMessage;

  return (
    <div className={styles.pageWrapper}>
      <PageSidebar
        title="Shares"
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
      >
        <SidebarItem
          icon={UploadIcon}
          label="All Shares"
          count={counts.all}
          active={filter === 'all'}
          onClick={() => setFilter('all')}
          collapsed={sidebarCollapsed}
        />
        {!sidebarCollapsed && <div className={styles.sidebarDivider} />}
        <SidebarItem
          icon={GlobeIcon}
          label="Public"
          count={counts.public}
          active={filter === 'public'}
          onClick={() => setFilter('public')}
          collapsed={sidebarCollapsed}
          disabled={counts.public === 0}
        />
        <SidebarItem
          icon={LockIcon}
          label="Private"
          count={counts.private}
          active={filter === 'private'}
          onClick={() => setFilter('private')}
          collapsed={sidebarCollapsed}
          disabled={counts.private === 0}
        />
        {!sidebarCollapsed && <div className={styles.sidebarDivider} />}
        <SidebarItem
          icon={ActiveIcon}
          label="Active"
          count={counts.active}
          active={filter === 'active'}
          onClick={() => setFilter('active')}
          collapsed={sidebarCollapsed}
          disabled={counts.active === 0}
        />
        <SidebarItem
          icon={<AlertCircleIcon size={16} />}
          label="Expired"
          count={counts.expired}
          active={filter === 'expired'}
          onClick={() => setFilter('expired')}
          collapsed={sidebarCollapsed}
          disabled={counts.expired === 0}
        />
      </PageSidebar>

      <div className={`${styles.mainContent} ${sidebarCollapsed ? styles.sidebarCollapsed : ''}`}>
        <PageHeader
          title="Shares"
          subtitle={`${sortedShares.length} share${sortedShares.length !== 1 ? 's' : ''}`}
        />

        <div className={styles.container}>
          {displayMessage && <Alert variant="success">{displayMessage}</Alert>}
          {error && <Alert variant="error">{error}</Alert>}

          <div className={styles.card}>
            {loading && (
              <p className={styles.loading}>Loading shares...</p>
            )}
            {!loading && shares.length === 0 && (
              <p className={styles.empty}>
                No shares yet. Share a session to see shares here.
              </p>
            )}
            {!loading && shares.length > 0 && sortedShares.length === 0 && (
              <p className={styles.empty}>
                No shares match the selected filter.
              </p>
            )}
            {!loading && sortedShares.length > 0 && (
              <div className={styles.sharesTable}>
                <table>
                  <thead>
                    <tr>
                      <SortableHeader
                        column="session_summary"
                        label="Session"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                      <SortableHeader
                        column="is_public"
                        label="Visibility"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                      <th>Recipients</th>
                      <SortableHeader
                        column="created_at"
                        label="Created"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                      <SortableHeader
                        column="expires_at"
                        label="Expires"
                        currentColumn={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                      />
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedShares.map((share) => {
                      const shareURL = getShareURL(share.session_id);
                      const isExpired = share.expires_at && new Date(share.expires_at) < new Date();

                      return (
                        <tr key={share.id} className={isExpired ? styles.expiredRow : ''}>
                          <td>
                            <Link to={`/sessions/${share.session_id}`} className={styles.sessionLink}>
                              {share.session_summary || share.session_first_user_message || 'Untitled Session'}
                            </Link>
                            <div className={styles.sessionId}>
                              <code>{share.external_id.substring(0, 8)}</code>
                            </div>
                          </td>
                          <td>
                            <span className={`${styles.badge} ${share.is_public ? styles.public : styles.private}`}>
                              {share.is_public ? 'public' : 'private'}
                            </span>
                          </td>
                          <td className={styles.emails}>
                            {!share.is_public && share.recipients && share.recipients.length > 0
                              ? share.recipients.join(', ')
                              : '—'}
                          </td>
                          <td className={styles.timestamp}>{formatRelativeTime(share.created_at)}</td>
                          <td>
                            {share.expires_at ? (
                              <span className={isExpired ? styles.expired : styles.timestamp}>
                                {formatRelativeTime(share.expires_at)}
                                {isExpired && ' (Expired)'}
                              </span>
                            ) : (
                              <span className={styles.neverExpires}>Never</span>
                            )}
                          </td>
                          <td>
                            <div className={styles.actions}>
                              <Button
                                size="sm"
                                onClick={() => copy(shareURL)}
                                title="Copy link"
                              >
                                {CopyIcon}
                                <span>Copy</span>
                              </Button>
                              <Button
                                variant="danger"
                                size="sm"
                                onClick={() => handleRevoke(share.id)}
                              >
                                Revoke
                              </Button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default ShareLinksPage;
