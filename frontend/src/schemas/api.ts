// Zod schemas for API response validation
// Validates all data received from backend APIs
import { z } from 'zod';

// ============================================================================
// Common Schemas
// ============================================================================

const GitInfoSchema = z.object({
  repo_url: z.string().optional(),
  branch: z.string().optional(),
  commit_sha: z.string().optional(),
  commit_message: z.string().optional(),
  author: z.string().optional(),
  is_dirty: z.boolean().optional(),
});

const SyncFileDetailSchema = z.object({
  file_name: z.string(),
  file_type: z.string(),
  last_synced_line: z.number(),
  updated_at: z.string(),
});

// ============================================================================
// Session Schemas
// ============================================================================

const SessionSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  first_seen: z.string(),
  file_count: z.number(),
  last_sync_time: z.string().nullable().optional(),
  custom_title: z.string().max(255).nullable().optional(),
  suggested_session_title: z.string().max(100).nullable().optional(),
  summary: z.string().nullable().optional(),
  first_user_message: z.string().nullable().optional(),
  session_type: z.string(),
  total_lines: z.number(),
  git_repo: z.string().nullable().optional(),
  git_repo_url: z.string().nullable().optional(), // Full git repository URL
  git_branch: z.string().nullable().optional(),
  github_prs: z.array(z.string()).nullable().optional(), // Linked GitHub PR refs (e.g., ["123", "456"])
  github_commits: z.array(z.string()).nullable().optional(), // Linked GitHub commit SHAs (latest first)
  is_owner: z.boolean(),
  access_type: z.enum(['owner', 'private_share', 'public_share', 'system_share']),
  shared_by_email: z.string().nullable().optional(),
  owner_email: z.string(),
});

const SessionFilterOptionsSchema = z.object({
  repos: z.array(z.string()),
  branches: z.array(z.string()),
  owners: z.array(z.string()),
});

export const SessionListResponseSchema = z.object({
  sessions: z.array(SessionSchema),
  has_more: z.boolean(),
  next_cursor: z.string().optional().default(''),
  page_size: z.number(),
  filter_options: SessionFilterOptionsSchema,
});

export const SessionDetailSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  custom_title: z.string().max(255).nullable().optional(),
  suggested_session_title: z.string().max(100).nullable().optional(),
  summary: z.string().nullable().optional(),
  first_user_message: z.string().nullable().optional(),
  first_seen: z.string(),
  cwd: z.string().nullable().optional(),
  transcript_path: z.string().nullable().optional(),
  git_info: GitInfoSchema.nullable().optional(),
  last_sync_at: z.string().nullable().optional(),
  files: z.array(SyncFileDetailSchema),
  hostname: z.string().nullable().optional(), // Client machine hostname (owner-only, null for shared)
  username: z.string().nullable().optional(), // OS username (owner-only, null for shared)
  is_owner: z.boolean().optional(), // True if viewer is session owner (shared sessions only)
  shared_by_email: z.string().nullable().optional(), // Email of session owner (non-owner access only)
  owner_email: z.string(), // Email of session owner (always populated)
});

const SessionShareSchema = z.object({
  id: z.number(),
  session_id: z.string(),
  external_id: z.string(),
  session_summary: z.string().nullable().optional(),
  session_first_user_message: z.string().nullable().optional(),
  is_public: z.boolean(),
  recipients: z.array(z.string()).nullable().optional(),
  expires_at: z.string().nullable().optional(),
  created_at: z.string(),
  last_accessed_at: z.string().nullable().optional(),
});

// ============================================================================
// Auth Schemas
// ============================================================================

export const UserSchema = z.object({
  name: z.string().optional(),
  email: z.string(),
  avatar_url: z.string().optional(),
});

// ============================================================================
// API Key Schemas
// ============================================================================

const APIKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  created_at: z.string(),
  last_used_at: z.string().nullable().optional(),
});

export const CreateAPIKeyResponseSchema = z.object({
  id: z.number(),
  key: z.string(),
  name: z.string(),
  created_at: z.string(),
});

// ============================================================================
// Share Schemas
// ============================================================================

export const CreateShareResponseSchema = z.object({
  share_url: z.string(),
});

// ============================================================================
// GitHub Link Schemas
// ============================================================================

const GitHubLinkTypeSchema = z.enum(['commit', 'pull_request']);
const GitHubLinkSourceSchema = z.enum(['cli_hook', 'manual']);

export const GitHubLinkSchema = z.object({
  id: z.number(),
  session_id: z.string(),
  link_type: GitHubLinkTypeSchema,
  url: z.string(),
  owner: z.string(),
  repo: z.string(),
  ref: z.string(),
  title: z.string().nullable().optional(),
  source: GitHubLinkSourceSchema,
  created_at: z.string(),
});

export const GitHubLinksResponseSchema = z.object({
  links: z.array(GitHubLinkSchema),
});

// ============================================================================
// Analytics Schemas
// ============================================================================

const TokenStatsSchema = z.object({
  input: z.number(),
  output: z.number(),
  cache_creation: z.number(),
  cache_read: z.number(),
});

const CostStatsSchema = z.object({
  estimated_usd: z.string(), // Decimal serialized as string from backend
});

const CompactionInfoSchema = z.object({
  auto: z.number(),
  manual: z.number(),
  avg_time_ms: z.number().nullable().optional(),
});

// Card data schemas for the new cards-based format
// Tokens card includes cost info (consolidated from previous separate cost card)
const TokensCardDataSchema = z.object({
  input: z.number(),
  output: z.number(),
  cache_creation: z.number(),
  cache_read: z.number(),
  estimated_usd: z.string(), // Consolidated from cost card
});

// Session card includes compaction info (consolidated from previous separate compaction card)
// Note: Messages with text+tool_use count as text_responses, not tool_calls.
// Therefore assistant_messages may not equal text_responses + tool_calls + thinking_blocks.
// Note: Turn counts are in the Conversation card.
const SessionCardDataSchema = z.object({
  // Message counts (raw line counts)
  total_messages: z.number(),
  user_messages: z.number(),
  assistant_messages: z.number(),

  // Message type breakdown
  human_prompts: z.number(), // User messages with string content
  tool_results: z.number(), // User messages with tool_result arrays
  text_responses: z.number(), // Assistant messages containing text (counts as turn)
  tool_calls: z.number(), // Assistant messages with ONLY tool_use (no text)
  thinking_blocks: z.number(), // Assistant messages with ONLY thinking (no text)

  // Session metadata
  duration_ms: z.number().nullable().optional(),
  models_used: z.array(z.string()),

  // Compaction stats (consolidated from previous separate compaction card)
  compaction_auto: z.number(),
  compaction_manual: z.number(),
  compaction_avg_time_ms: z.number().nullable().optional(),
});

const ToolStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

const ToolsCardDataSchema = z.object({
  total_calls: z.number(),
  tool_stats: z.record(z.string(), ToolStatsSchema),
  error_count: z.number(),
});

const CodeActivityCardDataSchema = z.object({
  files_read: z.number(),
  files_modified: z.number(),
  lines_added: z.number(),
  lines_removed: z.number(),
  search_count: z.number(),
  language_breakdown: z.record(z.string(), z.number()),
});

// Conversation card: tracks timing metrics for conversational turns
const ConversationCardDataSchema = z.object({
  user_turns: z.number(),
  assistant_turns: z.number(),
  avg_assistant_turn_ms: z.number().nullable().optional(),
  avg_user_thinking_ms: z.number().nullable().optional(),
  total_assistant_duration_ms: z.number().nullable().optional(),
  total_user_duration_ms: z.number().nullable().optional(),
  assistant_utilization_pct: z.number().nullable().optional(),
});

// Agent stats: per-agent-type success/error counts (same structure as ToolStats)
const AgentStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

// Skill stats: per-skill success/error counts (same structure as AgentStats)
const SkillStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

// Combined Agents and Skills card: unified view of both agent and skill invocations
const AgentsAndSkillsCardDataSchema = z.object({
  agent_invocations: z.number(),
  skill_invocations: z.number(),
  agent_stats: z.record(z.string(), AgentStatsSchema),
  skill_stats: z.record(z.string(), SkillStatsSchema),
});

// Redactions card: tracks [REDACTED:TYPE] markers in transcript
const RedactionsCardDataSchema = z.object({
  total_redactions: z.number(),
  redaction_counts: z.record(z.string(), z.number()), // Type -> count
});

// Smart Recap card: AI-generated session analysis
const SmartRecapCardDataSchema = z.object({
  recap: z.string(),
  went_well: z.array(z.string()),
  went_bad: z.array(z.string()),
  human_suggestions: z.array(z.string()),
  environment_suggestions: z.array(z.string()),
  default_context_suggestions: z.array(z.string()),
  computed_at: z.string(),
  model_used: z.string(),
});

// Quota information for smart recap generation
const SmartRecapQuotaInfoSchema = z.object({
  used: z.number(),
  limit: z.number(),
  exceeded: z.boolean(),
});

// Cards map schema - extensible for future cards
// All fields optional to handle empty analytics (session with no transcript)
// Note: cost is now part of tokens card, compaction is now part of session card
const AnalyticsCardsSchema = z.object({
  tokens: TokensCardDataSchema.optional(),
  session: SessionCardDataSchema.optional(),
  tools: ToolsCardDataSchema.optional(),
  code_activity: CodeActivityCardDataSchema.optional(),
  conversation: ConversationCardDataSchema.optional(),
  agents_and_skills: AgentsAndSkillsCardDataSchema.optional(),
  redactions: RedactionsCardDataSchema.optional(),
  smart_recap: SmartRecapCardDataSchema.optional(),
});

export const SessionAnalyticsSchema = z.object({
  computed_at: z.string(), // ISO timestamp when analytics were computed
  computed_lines: z.number(), // Line count when analytics were computed
  // Legacy flat format (deprecated - use cards instead)
  tokens: TokenStatsSchema,
  cost: CostStatsSchema,
  compaction: CompactionInfoSchema,
  // New cards-based format (optional for empty analytics)
  cards: AnalyticsCardsSchema.optional().nullable(),
  // Per-card computation errors (graceful degradation)
  // Maps card key (e.g., "tokens", "session") to error message
  card_errors: z.record(z.string(), z.string()).optional().nullable(),
  // Smart recap quota (only present if feature is enabled)
  smart_recap_quota: SmartRecapQuotaInfoSchema.optional().nullable(),
  // Suggested session title from Smart Recap (if generated)
  suggested_session_title: z.string().nullable().optional(),
});

// ============================================================================
// Trends Schemas
// ============================================================================

const DateRangeSchema = z.object({
  start_date: z.string(), // YYYY-MM-DD
  end_date: z.string(),   // YYYY-MM-DD
});

const TrendsOverviewCardSchema = z.object({
  session_count: z.number(),
  total_duration_ms: z.number(),
  avg_duration_ms: z.number().nullable().optional(),
  days_covered: z.number(),
  total_assistant_duration_ms: z.number(),
  assistant_utilization_pct: z.number().nullable().optional(),
});

const DailyCostPointSchema = z.object({
  date: z.string(),     // YYYY-MM-DD
  cost_usd: z.string(), // Decimal as string
});

const TrendsTokensCardSchema = z.object({
  total_input_tokens: z.number(),
  total_output_tokens: z.number(),
  total_cache_creation_tokens: z.number(),
  total_cache_read_tokens: z.number(),
  total_cost_usd: z.string(),
  daily_costs: z.array(DailyCostPointSchema),
});

const DailySessionCountSchema = z.object({
  date: z.string(),         // YYYY-MM-DD
  session_count: z.number(),
});

const TrendsActivityCardSchema = z.object({
  total_files_read: z.number(),
  total_files_modified: z.number(),
  total_lines_added: z.number(),
  total_lines_removed: z.number(),
  daily_session_counts: z.array(DailySessionCountSchema),
});

const TrendsToolStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

const TrendsToolsCardSchema = z.object({
  total_calls: z.number(),
  total_errors: z.number(),
  tool_stats: z.record(z.string(), TrendsToolStatsSchema),
});

const DailyUtilizationPointSchema = z.object({
  date: z.string(),
  utilization_pct: z.number().nullable(),
});

const TrendsUtilizationCardSchema = z.object({
  daily_utilization: z.array(DailyUtilizationPointSchema),
});

const TrendsAgentStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

const TrendsSkillStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

const TrendsAgentsAndSkillsCardSchema = z.object({
  total_agent_invocations: z.number(),
  total_skill_invocations: z.number(),
  agent_stats: z.record(z.string(), TrendsAgentStatsSchema),
  skill_stats: z.record(z.string(), TrendsSkillStatsSchema),
});

const TrendsCardsSchema = z.object({
  overview: TrendsOverviewCardSchema.nullable(),
  tokens: TrendsTokensCardSchema.nullable(),
  activity: TrendsActivityCardSchema.nullable(),
  tools: TrendsToolsCardSchema.nullable(),
  utilization: TrendsUtilizationCardSchema.nullable(),
  agents_and_skills: TrendsAgentsAndSkillsCardSchema.nullable(),
});

export const TrendsResponseSchema = z.object({
  computed_at: z.string(),
  date_range: DateRangeSchema,
  session_count: z.number(),
  repos_included: z.array(z.string()),
  include_no_repo: z.boolean(),
  cards: TrendsCardsSchema,
});

// ============================================================================
// Array Response Schemas
// ============================================================================

export const SessionShareListSchema = z.array(SessionShareSchema);
export const APIKeyListSchema = z.array(APIKeySchema);

// ============================================================================
// Inferred Types
// ============================================================================

export type GitInfo = z.infer<typeof GitInfoSchema>;
export type Session = z.infer<typeof SessionSchema>;
export type SessionDetail = z.infer<typeof SessionDetailSchema>;
export type SessionShare = z.infer<typeof SessionShareSchema>;
export type User = z.infer<typeof UserSchema>;
export type APIKey = z.infer<typeof APIKeySchema>;
export type CreateAPIKeyResponse = z.infer<typeof CreateAPIKeyResponseSchema>;
export type CreateShareResponse = z.infer<typeof CreateShareResponseSchema>;
export type GitHubLink = z.infer<typeof GitHubLinkSchema>;
export type GitHubLinksResponse = z.infer<typeof GitHubLinksResponseSchema>;
export type TokensCardData = z.infer<typeof TokensCardDataSchema>;
export type SessionCardData = z.infer<typeof SessionCardDataSchema>;
export type ToolsCardData = z.infer<typeof ToolsCardDataSchema>;
export type CodeActivityCardData = z.infer<typeof CodeActivityCardDataSchema>;
export type ConversationCardData = z.infer<typeof ConversationCardDataSchema>;
export type AgentsAndSkillsCardData = z.infer<typeof AgentsAndSkillsCardDataSchema>;
export type RedactionsCardData = z.infer<typeof RedactionsCardDataSchema>;
export type SmartRecapCardData = z.infer<typeof SmartRecapCardDataSchema>;
export type SmartRecapQuotaInfo = z.infer<typeof SmartRecapQuotaInfoSchema>;
export type AnalyticsCards = z.infer<typeof AnalyticsCardsSchema>;
export type SessionAnalytics = z.infer<typeof SessionAnalyticsSchema>;
export type TrendsResponse = z.infer<typeof TrendsResponseSchema>;
export type TrendsOverviewCard = z.infer<typeof TrendsOverviewCardSchema>;
export type TrendsTokensCard = z.infer<typeof TrendsTokensCardSchema>;
export type TrendsActivityCard = z.infer<typeof TrendsActivityCardSchema>;
export type TrendsToolsCard = z.infer<typeof TrendsToolsCardSchema>;
export type TrendsUtilizationCard = z.infer<typeof TrendsUtilizationCardSchema>;
export type TrendsAgentsAndSkillsCard = z.infer<typeof TrendsAgentsAndSkillsCardSchema>;
export type SessionFilterOptions = z.infer<typeof SessionFilterOptionsSchema>;
export type SessionListResponse = z.infer<typeof SessionListResponseSchema>;

// ============================================================================
// Validation Functions
// ============================================================================

/**
 * Validate API response data against a schema.
 * Throws ZodError with detailed messages on failure.
 */
export function validateResponse<T>(schema: z.ZodType<T>, data: unknown, endpoint: string): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    console.error(`API validation failed for ${endpoint}:`, result.error.issues);
    throw new APIValidationError(endpoint, result.error);
  }
  return result.data;
}

/**
 * Custom error class for API validation failures
 */
class APIValidationError extends Error {
  endpoint: string;
  zodError: z.ZodError;

  constructor(endpoint: string, zodError: z.ZodError) {
    const issues = zodError.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join('; ');
    super(`Invalid API response from ${endpoint}: ${issues}`);
    this.name = 'APIValidationError';
    this.endpoint = endpoint;
    this.zodError = zodError;
  }
}
