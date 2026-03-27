-- Admin settings: generic key-value store for admin-configurable settings.
-- Initial use case: custom smart recap system prompt (key: "smart_recap_system_prompt").
-- Also used for: bulk regeneration timestamp (key: "smart_recap_regen_requested_at").

CREATE TABLE admin_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE admin_settings IS 'Generic key-value store for admin-configurable settings';
COMMENT ON COLUMN admin_settings.updated_at IS 'When this setting was last modified (admin identity tracked in audit log)';
