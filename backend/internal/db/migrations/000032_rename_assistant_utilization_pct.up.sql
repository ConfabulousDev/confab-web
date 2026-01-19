-- Rename assistant_utilization to assistant_utilization_pct for clarity
-- The value is already stored as a percentage (0-100), name should reflect that
ALTER TABLE session_card_conversation
    RENAME COLUMN assistant_utilization TO assistant_utilization_pct;
