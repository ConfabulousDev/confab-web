-- Remove total duration and utilization columns from conversation card
ALTER TABLE session_card_conversation
    DROP COLUMN IF EXISTS total_assistant_duration_ms,
    DROP COLUMN IF EXISTS total_user_duration_ms,
    DROP COLUMN IF EXISTS assistant_utilization;
