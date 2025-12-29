-- Add message breakdown columns to session_card_session
-- These columns track detailed message counts for accurate turn counting

ALTER TABLE session_card_session
ADD COLUMN IF NOT EXISTS total_messages INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS user_messages INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS assistant_messages INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS human_prompts INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS tool_results INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS text_responses INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS tool_calls INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS thinking_blocks INTEGER NOT NULL DEFAULT 0;

-- Note: user_turns and assistant_turns columns already exist but their meaning changes:
-- Previously: counted every user/assistant message
-- Now: user_turns = human_prompts, assistant_turns = text_responses (actual conversational turns)
