-- Restore turn count columns to session card.
ALTER TABLE session_card_session ADD COLUMN IF NOT EXISTS user_turns INT NOT NULL DEFAULT 0;
ALTER TABLE session_card_session ADD COLUMN IF NOT EXISTS assistant_turns INT NOT NULL DEFAULT 0;
