// ABOUTME: Learnings page with list, status filter tabs, and review queue actions
// ABOUTME: Displays learning artifacts with confirm/archive/delete actions
import { useState, useMemo } from "react";
import { useDocumentTitle } from "@/hooks";
import {
  useLearnings,
  useUpdateLearning,
  useDeleteLearning,
} from "@/hooks/useLearnings";
import { RelativeTime } from "@/components/RelativeTime";
import Alert from "@/components/Alert";
import type { Learning } from "@/schemas/api";
import styles from "./LearningsPage.module.css";

type StatusFilter = "all" | "draft" | "confirmed" | "exported" | "archived";

const STATUS_TABS: { value: StatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "draft", label: "Draft" },
  { value: "confirmed", label: "Confirmed" },
  { value: "exported", label: "Exported" },
  { value: "archived", label: "Archived" },
];

/** Format source enum values for display: manual_session -> "manual session" */
function formatSource(source: string): string {
  return source.replace(/_/g, " ");
}

/** Map status to CSS class name */
function statusClass(status: string): string {
  switch (status) {
    case "draft":
      return styles.statusDraft ?? "";
    case "confirmed":
      return styles.statusConfirmed ?? "";
    case "exported":
      return styles.statusExported ?? "";
    case "archived":
      return styles.statusArchived ?? "";
    default:
      return "";
  }
}

function LearningsPage() {
  useDocumentTitle("Learnings");

  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");

  // Build API filters from local state
  const apiFilters = useMemo(() => {
    const filters: Record<string, string> = {};
    if (statusFilter !== "all") {
      filters.status = statusFilter;
    }
    if (search.trim()) {
      filters.search = search.trim();
    }
    return Object.keys(filters).length > 0 ? filters : undefined;
  }, [statusFilter, search]);

  const { data, isLoading, error } = useLearnings(apiFilters);
  const updateLearning = useUpdateLearning();
  const deleteLearning = useDeleteLearning();

  const learnings = data?.learnings ?? [];
  const counts = data?.counts ?? {};

  const handleConfirm = (learning: Learning) => {
    updateLearning.mutate({ id: learning.id, status: "confirmed" });
  };

  const handleArchive = (learning: Learning) => {
    updateLearning.mutate({ id: learning.id, status: "archived" });
  };

  const handleDelete = (learning: Learning) => {
    deleteLearning.mutate(learning.id);
  };

  return (
    <div className={styles.pageWrapper}>
      <div className={styles.mainContent}>
        <header className={styles.toolbar}>
          <div className={styles.toolbarTop}>
            <span className={styles.pageTitle}>Learnings</span>
            <div className={styles.stats}>
              {Object.entries(counts).map(([status, count]) => (
                <span key={status} className={styles.statItem}>
                  <span className={styles.statCount}>{count}</span> {status}
                </span>
              ))}
            </div>
          </div>

          <div className={styles.filters}>
            <input
              type="text"
              className={styles.searchInput}
              placeholder="Search learnings..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <div className={styles.statusTabs}>
              {STATUS_TABS.map((tab) => (
                <button
                  key={tab.value}
                  className={`${styles.tab} ${statusFilter === tab.value ? styles.active : ""}`}
                  onClick={() => setStatusFilter(tab.value)}
                >
                  {tab.label}
                </button>
              ))}
            </div>
          </div>
        </header>

        <div className={styles.container}>
          {error && <Alert variant="error">{(error as Error).message}</Alert>}

          <div className={styles.card}>
            {isLoading && learnings.length === 0 && (
              <p className={styles.loading}>Loading learnings...</p>
            )}

            {!isLoading && learnings.length === 0 && (
              <div className={styles.empty}>
                <p>No learnings found.</p>
                <p>
                  Use <code>confab learn</code> from the CLI to capture
                  learnings from your Claude Code sessions, or they will be
                  extracted automatically.
                </p>
              </div>
            )}

            {learnings.length > 0 && (
              <div className={styles.list}>
                {learnings.map((learning) => (
                  <div key={learning.id} className={styles.learningCard}>
                    <div className={styles.cardHeader}>
                      <h3 className={styles.cardTitle}>{learning.title}</h3>
                      <span
                        className={`${styles.statusChip} ${statusClass(learning.status)}`}
                      >
                        {learning.status}
                      </span>
                      <div className={styles.actions}>
                        {learning.status === "draft" && (
                          <button
                            className={`${styles.actionBtn} ${styles.confirmBtn}`}
                            onClick={() => handleConfirm(learning)}
                            disabled={updateLearning.isPending}
                          >
                            Confirm
                          </button>
                        )}
                        {learning.status !== "archived" && (
                          <button
                            className={`${styles.actionBtn} ${styles.archiveBtn}`}
                            onClick={() => handleArchive(learning)}
                            disabled={updateLearning.isPending}
                          >
                            Archive
                          </button>
                        )}
                        <button
                          className={`${styles.actionBtn} ${styles.deleteBtn}`}
                          onClick={() => handleDelete(learning)}
                          disabled={deleteLearning.isPending}
                        >
                          Delete
                        </button>
                      </div>
                    </div>

                    {learning.body && learning.body !== learning.title && (
                      <p className={styles.body}>{learning.body}</p>
                    )}

                    <div className={styles.meta}>
                      <span className={styles.source}>
                        {formatSource(learning.source)}
                      </span>
                      {learning.tags.length > 0 && (
                        <div className={styles.tags}>
                          {learning.tags.map((tag) => (
                            <span key={tag} className={styles.tag}>
                              {tag}
                            </span>
                          ))}
                        </div>
                      )}
                      <span className={styles.date}>
                        <RelativeTime date={learning.created_at} />
                      </span>
                    </div>
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

export default LearningsPage;
