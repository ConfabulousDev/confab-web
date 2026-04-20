-- Admin-triggered card invalidations. Doubles as audit log and smart-recap
-- quota-bypass signal. One row per (session, invalidation) so FindStaleSmartRecapSessions
-- can LEFT JOIN per session.
--
-- admin_user_id intentionally has no FK: preserves the audit trail independent
-- of user-table lifecycle, so admin deletion cannot corrupt or block history.
--
-- card_types stores the admin's requested selection, identical for every row
-- inserted in a single run (same correlation_id). Sufficient for both the audit
-- view and the smart-recap bypass (which filters on 'session_card_smart_recap' = ANY(card_types)).

CREATE TABLE admin_card_invalidations (
    id              BIGSERIAL PRIMARY KEY,
    session_id      UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    admin_user_id   BIGINT NOT NULL,
    invalidated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    card_types      TEXT[] NOT NULL,
    correlation_id  UUID NOT NULL,
    reason          TEXT NOT NULL CHECK (length(reason) > 0)
);

CREATE INDEX idx_admin_card_invalidations_session_invalidated_at
    ON admin_card_invalidations(session_id, invalidated_at DESC);
CREATE INDEX idx_admin_card_invalidations_correlation
    ON admin_card_invalidations(correlation_id);
CREATE INDEX idx_admin_card_invalidations_invalidated_at
    ON admin_card_invalidations(invalidated_at DESC);

COMMENT ON TABLE admin_card_invalidations IS 'Audit log and smart-recap quota-bypass signal for admin-triggered card invalidations';
COMMENT ON COLUMN admin_card_invalidations.admin_user_id IS 'No FK to users(id): audit trail survives admin deletion';
COMMENT ON COLUMN admin_card_invalidations.card_types IS 'Admin requested card types (same for every row in one correlation_id)';
COMMENT ON COLUMN admin_card_invalidations.correlation_id IS 'Groups all rows from a single admin invalidation action';
