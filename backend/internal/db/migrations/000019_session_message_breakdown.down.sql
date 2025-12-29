-- Remove message breakdown columns from session_card_session

ALTER TABLE session_card_session
DROP COLUMN IF EXISTS total_messages,
DROP COLUMN IF EXISTS user_messages,
DROP COLUMN IF EXISTS assistant_messages,
DROP COLUMN IF EXISTS human_prompts,
DROP COLUMN IF EXISTS tool_results,
DROP COLUMN IF EXISTS text_responses,
DROP COLUMN IF EXISTS tool_calls,
DROP COLUMN IF EXISTS thinking_blocks;
