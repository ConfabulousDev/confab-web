-- LLM vendor that generated the smart recap ("anthropic" | "openai").
-- Nullable with no DEFAULT/CHECK: the app validates and always writes it, and
-- reads NULL (legacy rows, all Anthropic-generated) as "anthropic".
ALTER TABLE session_card_smart_recap ADD COLUMN llm_provider TEXT;
