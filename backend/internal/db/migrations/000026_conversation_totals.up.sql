-- Add total duration and utilization columns to conversation card
ALTER TABLE session_card_conversation
    ADD COLUMN total_assistant_duration_ms BIGINT,
    ADD COLUMN total_user_duration_ms BIGINT,
    ADD COLUMN assistant_utilization DOUBLE PRECISION;
