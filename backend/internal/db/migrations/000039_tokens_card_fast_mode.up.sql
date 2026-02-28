-- Add fast mode breakdown fields to the tokens card.
ALTER TABLE session_card_tokens ADD COLUMN fast_turns INT NOT NULL DEFAULT 0;
ALTER TABLE session_card_tokens ADD COLUMN fast_cost_usd DECIMAL(10,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN session_card_tokens.fast_turns IS 'Number of assistant turns using fast mode';
COMMENT ON COLUMN session_card_tokens.fast_cost_usd IS 'Estimated cost from fast mode turns';
