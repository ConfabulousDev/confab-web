-- Move turn counts from session card to conversation card.
-- Turn counts now live in session_card_conversation; session card focuses on message breakdown.
ALTER TABLE session_card_session DROP COLUMN IF EXISTS user_turns;
ALTER TABLE session_card_session DROP COLUMN IF EXISTS assistant_turns;
