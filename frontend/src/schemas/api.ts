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
  // Canonical agent identifier: 'claude-code' or 'codex'. Future providers
  // may be added; treat as a free string at the schema layer so older
  // backends shipping new values do not fail validation.
  provider: z.string(),
  total_lines: z.number(),
  git_repo: z.string().nullable().optional(),
  git_repo_url: z.string().nullable().optional(), // Full git repository URL
  git_branch: z.string().nullable().optional(),
  github_prs: z.array(z.string()).nullable().optional(), // Linked GitHub PR URLs (e.g., ["https://github.com/org/repo/pull/123"])
  github_commits: z.array(z.string()).nullable().optional(), // Linked GitHub commit SHAs (latest first)
  estimated_cost_usd: z.string().nullable().optional(), // Estimated API cost from analytics
  is_owner: z.boolean(),
  access_type: z.enum(['owner', 'private_share', 'public_share', 'system_share']),
  shared_by_email: z.string().nullable().optional(),
  owner_email: z.string(),
});

const SessionFilterOptionsSchema = z.object({
  repos: z.array(z.string()),
  branches: z.array(z.string()),
  owners: z.array(z.string()),
  providers: z.array(z.string()),
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
  // Canonical agent identifier: 'claude-code' or 'codex'.
  provider: z.string(),
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
  has_own_sessions: z.boolean().optional(),
  has_api_keys: z.boolean().optional(),
  is_admin: z.boolean().optional(),
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
const GitHubLinkSourceSchema = z.string(); // forward-compatible: backend may add new sources

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
  estimated_usd: z.string().refine((s) => /^-?\d+(\.\d+)?$/.test(s), { message: 'Invalid decimal' }), // Decimal serialized as string from backend
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
  estimated_usd: z.string().refine((s) => /^-?\d+(\.\d+)?$/.test(s), { message: 'Invalid decimal' }), // Consolidated from cost card
  // Fast mode breakdown (only present when fast mode was used)
  fast_turns: z.number().optional(),
  fast_cost_usd: z.string().optional(),
});

// tokens_v2: hierarchical per-provider per-model breakdown. Built for all
// providers (7eje): Claude/Codex key by_provider on the canonical agent id with
// getModelFamily families (fast under "<family> · fast"); OpenCode keys by vendor.
// Supersedes the flat tokens card on the session page when present.
const TokensV2ModelSchema = z.object({
  input: z.number(),
  output: z.number(),
  cache_read: z.number(),
  cache_write: z.number(),
  reasoning: z.number(),
  cost_usd: z.string().refine((s) => /^-?\d+(\.\d+)?$/.test(s), { message: 'Invalid decimal' }),
});

const TokensV2ProviderSchema = z.object({
  cost_usd: z.string().refine((s) => /^-?\d+(\.\d+)?$/.test(s), { message: 'Invalid decimal' }),
  models: z.record(z.string(), TokensV2ModelSchema),
});

const TokensV2CardDataSchema = z.object({
  total_cost_usd: z.string().refine((s) => /^-?\d+(\.\d+)?$/.test(s), { message: 'Invalid decimal' }),
  total_input: z.number(),
  total_output: z.number(),
  by_provider: z.record(z.string(), TokensV2ProviderSchema),
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

// Workflows card: per-run aggregates for Claude Code workflow subagent runs (CF-534).
// Derived server-side from subagents/workflows/<runId>/agent-<id>.jsonl + journal.jsonl.
const WorkflowRunSchema = z.object({
  run_id: z.string(),
  agent_count: z.number(),
  input_tokens: z.number(),
  output_tokens: z.number(),
  cache_creation: z.number(),
  cache_read: z.number(),
  estimated_usd: z.string(), // Decimal as string for precision
  succeeded_agents: z.number(), // From journal result-line presence
  has_journal: z.boolean(), // False when no journal.jsonl → status omitted
  duration_ms: z.number(), // Activity span across the run's agent files
});

const WorkflowsCardDataSchema = z.object({
  runs: z.array(WorkflowRunSchema),
});

// AnnotatedItem: a list item with optional message reference.
// Backwards-compatible: accepts plain strings (legacy) or objects (new).
const AnnotatedItemObjectSchema = z.object({ text: z.string(), message_id: z.string().optional() });
const AnnotatedItemSchema = z
  .union([
    z.string().transform((s) => ({ text: s })),
    AnnotatedItemObjectSchema,
  ])
  .pipe(AnnotatedItemObjectSchema);

// Smart Recap card: AI-generated session analysis
const SmartRecapCardDataSchema = z.object({
  recap: z.string(),
  went_well: z.array(AnnotatedItemSchema),
  went_bad: z.array(AnnotatedItemSchema),
  human_suggestions: z.array(AnnotatedItemSchema),
  environment_suggestions: z.array(AnnotatedItemSchema),
  default_context_suggestions: z.array(AnnotatedItemSchema),
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
  tokens_v2: TokensV2CardDataSchema.optional(),
  session: SessionCardDataSchema.optional(),
  tools: ToolsCardDataSchema.optional(),
  code_activity: CodeActivityCardDataSchema.optional(),
  conversation: ConversationCardDataSchema.optional(),
  agents_and_skills: AgentsAndSkillsCardDataSchema.optional(),
  redactions: RedactionsCardDataSchema.optional(),
  workflows: WorkflowsCardDataSchema.optional(),
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
  // Smart recap quota (only present if feature is enabled, owner only)
  smart_recap_quota: SmartRecapQuotaInfoSchema.optional().nullable(),
  // Why smart recap data is missing: "quota_exceeded" (owner) or "unavailable" (non-owner)
  smart_recap_missing_reason: z.enum(['quota_exceeded', 'unavailable']).optional().nullable(),
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
  cost_usd: z.string(), // Decimal as string — cross-provider total for the day
  // Per-provider cost breakdown for the day, keyed by canonical provider id.
  // `.default({})` keeps older wire payloads (single-series chart era) parseable.
  per_provider: z.record(z.string(), z.string()).default({}),
});

// CF-435: per-provider tokens/cost breakdown. Backend always populates this
// map (empty when the range has no sessions). Frontend renders a per-provider
// table when `Object.keys(per_provider).length >= 2`; otherwise the existing
// single-series StatRow layout is used.
const TrendsTokensPerProviderSchema = z.object({
  total_input_tokens: z.number(),
  total_output_tokens: z.number(),
  total_cache_creation_tokens: z.number(),
  total_cache_read_tokens: z.number(),
  total_cost_usd: z.string(),
});

const TrendsTokensCardSchema = z.object({
  total_input_tokens: z.number(),
  total_output_tokens: z.number(),
  total_cache_creation_tokens: z.number(),
  total_cache_read_tokens: z.number(),
  total_cost_usd: z.string(),
  daily_costs: z.array(DailyCostPointSchema),
  // `.default({})` keeps older backends (no per_provider field) parseable.
  per_provider: z.record(z.string(), TrendsTokensPerProviderSchema).default({}),
});

const DailySessionCountSchema = z.object({
  date: z.string(),         // YYYY-MM-DD
  session_count: z.number(),
  // CF-444: per-provider session-count breakdown for the stacked-bar chart.
  // `.default({})` keeps older backends parseable; canonical provider ids
  // come pre-normalized server-side.
  per_provider: z.record(z.string(), z.number()).default({}),
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

const TopSessionItemSchema = z.object({
  id: z.string(),
  title: z.string(),
  provider: z.string(),
  estimated_cost_usd: z.string(),
  duration_ms: z.number().nullable().optional(),
  git_repo: z.string().nullable().optional(),
});

const TrendsTopSessionsCardSchema = z.object({
  sessions: z.array(TopSessionItemSchema),
});

// 2hh1: per-(provider, model-family) cost/token breakdown. `model` is the
// normalized family key ('' → rendered "Unknown"; '<family> · fast' kept as its
// own row); `pct_of_total` is the row's share of the v2 model-attributed total
// (0–100), NOT a share of the grand-total headline (different scopes — see card).
const TrendsCostByModelRowSchema = z.object({
  model: z.string(),
  provider: z.string(),
  cost_usd: z.string(),
  pct_of_total: z.number(),
  input: z.number(),
  output: z.number(),
  cache_read: z.number(),
  cache_write: z.number(),
  session_count: z.number(),
});

const TrendsCostByModelCardSchema = z.object({
  rows: z.array(TrendsCostByModelRowSchema),
  covered_session_count: z.number(),
  total_session_count: z.number(),
  // True when the aggregation exceeded its budget and degraded to empty; the
  // card renders a "narrow your range" notice instead of an empty state.
  timed_out: z.boolean(),
});

// y1w5: fixed log-scale histogram of per-session cost. `label` is the display
// string the card renders verbatim ('$0.01 – $1'); `lo`/`hi` are the band's
// dollar edges (half-open [lo, hi); `hi` is null for the unbounded top band).
// `session_count` is the data-point count — sessions by default, or (session,
// model) pairs when a model filter is active.
const TrendsCostDistributionBucketSchema = z.object({
  label: z.string(),
  lo: z.number(),
  hi: z.number().nullable(),
  session_count: z.number(),
  total_usd: z.string(),
});

// Summary stats of the per-data-point cost values (decimal strings): p50/p90/p99
// percentiles plus the arithmetic mean (`avg`). The whole object is null when there
// are no data points.
const TrendsCostDistributionStatsSchema = z.object({
  p50: z.string(),
  p90: z.string(),
  p99: z.string(),
  avg: z.string(),
});

const TrendsCostDistributionCardSchema = z.object({
  buckets: z.array(TrendsCostDistributionBucketSchema),
  stats: TrendsCostDistributionStatsSchema.nullable(),
  // covered_session_count = sessions with per-session cost data contributing data
  // points; total_session_count = all filtered sessions in range. The stats are
  // biased by this partial v2 subset during backfill (surfaced in the caption).
  covered_session_count: z.number(),
  total_session_count: z.number(),
  // True when the aggregation exceeded its budget and degraded to empty; the card
  // renders a "narrow your range" notice instead of a histogram.
  timed_out: z.boolean(),
});

const TrendsCardsSchema = z.object({
  overview: TrendsOverviewCardSchema.nullable(),
  tokens: TrendsTokensCardSchema.nullable(),
  activity: TrendsActivityCardSchema.nullable(),
  tools: TrendsToolsCardSchema.nullable(),
  utilization: TrendsUtilizationCardSchema.nullable(),
  agents_and_skills: TrendsAgentsAndSkillsCardSchema.nullable(),
  top_sessions: TrendsTopSessionsCardSchema.nullable(),
  // `.default(null)` keeps older backends (pre-2hh1, no cost_by_model field)
  // parseable — the card renders nothing when absent.
  cost_by_model: TrendsCostByModelCardSchema.nullable().default(null),
  // y1w5: `.default(null)` keeps older backends (no cost_distribution field)
  // parseable — the card renders nothing when absent.
  cost_distribution: TrendsCostDistributionCardSchema.nullable().default(null),
});

// CF-495: TrendsFilterOptions surfaces the dropdown source for owners + repos.
// Mirrors SessionFilterOptions: lists reflect the caller's visible-session set
// and are STATIC across active filter changes. Always present; empty arrays
// when nothing is visible.
export const TrendsFilterOptionsSchema = z.object({
  owners: z.array(z.string()),
  repos: z.array(z.string()),
  // 2hh1: distinct normalized model-family keys (family + '· fast' variants)
  // across visible sessions, alphabetical, excluding the empty Unknown key.
  // `.default([])` keeps older backends (no models field) parseable.
  models: z.array(z.string()).default([]),
});

export const TrendsResponseSchema = z.object({
  computed_at: z.string(),
  date_range: DateRangeSchema,
  session_count: z.number(),
  repos_included: z.array(z.string()),
  include_no_repo: z.boolean(),
  // CF-424: distinct canonical providers present in the filtered result set,
  // sorted alphabetically. Always present; empty range yields []. Used by the
  // Tokens card to render a multi-provider caveat when len >= 2.
  providers_present: z.array(z.string()),
  cards: TrendsCardsSchema,
  // CF-495: pre-materialized filter dropdown source. Saves a side-call to
  // /api/sessions for the owner + repo lists.
  filter_options: TrendsFilterOptionsSchema,
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
export type WorkflowRun = z.infer<typeof WorkflowRunSchema>;
export type WorkflowsCardData = z.infer<typeof WorkflowsCardDataSchema>;
export type AnnotatedItem = z.infer<typeof AnnotatedItemSchema>;
export type SmartRecapCardData = z.infer<typeof SmartRecapCardDataSchema>;
export type SmartRecapQuotaInfo = z.infer<typeof SmartRecapQuotaInfoSchema>;
export type AnalyticsCards = z.infer<typeof AnalyticsCardsSchema>;
export type SessionAnalytics = z.infer<typeof SessionAnalyticsSchema>;
export type TrendsResponse = z.infer<typeof TrendsResponseSchema>;
export type TrendsFilterOptions = z.infer<typeof TrendsFilterOptionsSchema>;
export type TrendsOverviewCard = z.infer<typeof TrendsOverviewCardSchema>;
export type TrendsTokensCard = z.infer<typeof TrendsTokensCardSchema>;
export type TrendsTokensPerProvider = z.infer<typeof TrendsTokensPerProviderSchema>;
export type TrendsActivityCard = z.infer<typeof TrendsActivityCardSchema>;
export type TrendsToolsCard = z.infer<typeof TrendsToolsCardSchema>;
export type TrendsUtilizationCard = z.infer<typeof TrendsUtilizationCardSchema>;
export type TrendsAgentsAndSkillsCard = z.infer<typeof TrendsAgentsAndSkillsCardSchema>;
export type TrendsTopSessionsCard = z.infer<typeof TrendsTopSessionsCardSchema>;
export type TrendsCostByModelCard = z.infer<typeof TrendsCostByModelCardSchema>;
export type TrendsCostByModelRow = z.infer<typeof TrendsCostByModelRowSchema>;
export type TrendsCostDistributionCard = z.infer<typeof TrendsCostDistributionCardSchema>;
export type TrendsCostDistributionBucket = z.infer<typeof TrendsCostDistributionBucketSchema>;
export type SessionFilterOptions = z.infer<typeof SessionFilterOptionsSchema>;
export type SessionListResponse = z.infer<typeof SessionListResponseSchema>;

// ============================================================================
// Organization Analytics Schemas
// ============================================================================

const OrgUserInfoSchema = z.object({
  id: z.number(),
  email: z.string(),
  name: z.string().nullable().optional(),
});

const OrgUserAnalyticsSchema = z.object({
  user: OrgUserInfoSchema,
  session_count: z.number(),
  total_cost_usd: z.string(),
  total_duration_ms: z.number(),
  total_assistant_time_ms: z.number(),
  total_user_time_ms: z.number(),
  avg_cost_usd: z.string(),
  avg_duration_ms: z.number().nullable().optional(),
  avg_assistant_time_ms: z.number().nullable().optional(),
  avg_user_time_ms: z.number().nullable().optional(),
});

export const OrgAnalyticsResponseSchema = z.object({
  computed_at: z.string(),
  date_range: DateRangeSchema,
  // Canonical providers with any qualifying session in the filtered range.
  // Backend always emits `[]` but we accept null too — a regression to a nil
  // slice on the server should degrade to "no narrowing", not blow up parse.
  providers_present: z.array(z.string()).nullish().transform((v) => v ?? []),
  users: z.array(OrgUserAnalyticsSchema),
});

export const OrgReposResponseSchema = z.object({
  computed_at: z.string(),
  date_range: DateRangeSchema,
  // Same null-tolerance rule as providers_present above.
  repos: z.array(z.string()).nullish().transform((v) => v ?? []),
});

export type OrgUserAnalytics = z.infer<typeof OrgUserAnalyticsSchema>;
export type OrgAnalyticsResponse = z.infer<typeof OrgAnalyticsResponseSchema>;
export type OrgReposResponse = z.infer<typeof OrgReposResponseSchema>;

// ============================================================================
// Admin Schemas
// ============================================================================

const AdminUserSchema = z.object({
  id: z.number(),
  email: z.string(),
  name: z.string().nullable(),
  status: z.string(),
  session_count: z.number(),
  recap_cache_count: z.number(),
  recaps_this_month: z.number(),
  last_api_key_used: z.string().nullable(),
  last_logged_in: z.string().nullable(),
  created_at: z.string(),
  // 5k4v: is_admin is the runtime-toggleable users.is_admin column;
  // is_super_admin is whether the email is in SUPER_ADMIN_EMAILS (env, fixed).
  is_admin: z.boolean(),
  is_super_admin: z.boolean(),
});
const AdminTotalsSchema = z.object({
  total_sessions: z.number(),
  non_empty_sessions: z.number(),
  sessions_with_cache: z.number(),
  computations_this_month: z.number(),
});
export const AdminUserListResponseSchema = z.object({
  users: z.array(AdminUserSchema),
  totals: AdminTotalsSchema,
});
export type AdminUserListResponse = z.infer<typeof AdminUserListResponseSchema>;

export const CreateAdminUserResponseSchema = z.object({
  id: z.number(),
  email: z.string(),
});
export type CreateAdminUserResponse = z.infer<typeof CreateAdminUserResponseSchema>;

export const StatusChangeResponseSchema = z.object({
  id: z.number(),
  status: z.string(),
});
export type StatusChangeResponse = z.infer<typeof StatusChangeResponseSchema>;

// 5k4v: response from the grant/revoke-admin endpoints.
export const AdminChangeResponseSchema = z.object({
  id: z.number(),
  is_admin: z.boolean(),
});
export type AdminChangeResponse = z.infer<typeof AdminChangeResponseSchema>;

const AdminSystemShareSchema = z.object({
  id: z.number(),
  session_id: z.string(),
  external_id: z.string(),
  // Canonical provider value (e.g. 'claude-code' / 'codex'). Permissive z.string()
  // matches the codebase's backend-first rollout pattern; unknown future values
  // pass through providerLabel() as-is. CF-370.
  provider: z.string(),
  share_url: z.string(),
  expires_at: z.string().nullable(),
  created_at: z.string(),
  last_accessed_at: z.string().nullable(),
});
export const AdminSystemSharesResponseSchema = z.object({
  shares: z.array(AdminSystemShareSchema),
});
export type AdminSystemSharesResponse = z.infer<typeof AdminSystemSharesResponseSchema>;

export const CreateSystemShareResponseSchema = z.object({
  share_id: z.number(),
  external_id: z.string(),
  share_url: z.string(),
});
export type CreateSystemShareResponse = z.infer<typeof CreateSystemShareResponseSchema>;

// ============================================================================
// Smart Recap Prompt Schemas
// ============================================================================

export const SmartRecapPromptResponseSchema = z.object({
  instructions: z.string(),
  is_custom: z.boolean(),
  updated_at: z.string().optional(),
  input_format: z.string(),
  output_schema: z.string(),
  example: z.string(),
});
export type SmartRecapPromptResponse = z.infer<typeof SmartRecapPromptResponseSchema>;

export const SmartRecapPromptDefaultResponseSchema = z.object({
  instructions: z.string(),
});
export type SmartRecapPromptDefaultResponse = z.infer<typeof SmartRecapPromptDefaultResponseSchema>;

export const SetSmartRecapPromptResponseSchema = z.object({
  instructions: z.string(),
  is_custom: z.literal(true),
  updated_at: z.string(),
});
export type SetSmartRecapPromptResponse = z.infer<typeof SetSmartRecapPromptResponseSchema>;

export const DeleteSmartRecapPromptResponseSchema = z.object({
  instructions: z.string(),
  is_custom: z.literal(false),
});
export type DeleteSmartRecapPromptResponse = z.infer<typeof DeleteSmartRecapPromptResponseSchema>;

export const RegenerateCountResponseSchema = z.object({
  count: z.number(),
});
export type RegenerateCountResponse = z.infer<typeof RegenerateCountResponseSchema>;

export const RegenerateAllResponseSchema = z.object({
  sessions_queued: z.number(),
});
export type RegenerateAllResponse = z.infer<typeof RegenerateAllResponseSchema>;

// ============================================================================
// Card Invalidations (CF-343)
// ============================================================================

// Card table names are served by the backend (GET /admin/cards/types) from
// `analytics.AllCardTableNames` — the single source of truth (vd31). The frontend
// no longer keeps a copy; `card_types` is a plain string[] and the backend
// authoritatively rejects unknown values on invalidate.
export const CardTypesResponseSchema = z.object({
  card_types: z.array(z.string()),
});
export type CardTypesResponse = z.infer<typeof CardTypesResponseSchema>;

export const InvalidateCardsRequestSchema = z.object({
  start_date: z.string(),
  end_date: z.string().optional(),
  card_types: z.array(z.string()).min(1),
  reason: z.string().min(1).max(500),
  dry_run: z.boolean().optional(),
  // kyrr: on execute (dry_run=false), the admin must echo the affected-session count
  // from the dry-run preview; the server re-counts and rejects (400) on mismatch.
  confirm: z.string().optional(),
});
export type InvalidateCardsRequest = z.infer<typeof InvalidateCardsRequestSchema>;

export const InvalidateCardsResponseSchema = z.object({
  correlation_id: z.string(),
  affected_sessions: z.number(),
  affected_cards: z.record(z.string(), z.number()),
  executed: z.boolean(),
  completed_batches: z.number().nullable().optional(),
  affected_sessions_executed: z.number().nullable().optional(),
  error: z.string().optional(),
});
export type InvalidateCardsResponse = z.infer<typeof InvalidateCardsResponseSchema>;

export const CardInvalidationRowSchema = z.object({
  id: z.number(),
  session_id: z.string(),
  admin_user_id: z.number(),
  admin_email: z.string().optional(),
  invalidated_at: z.string(),
  card_types: z.array(z.string()),
  correlation_id: z.string(),
  reason: z.string(),
});
export type CardInvalidationRow = z.infer<typeof CardInvalidationRowSchema>;

export const CardInvalidationsListResponseSchema = z.object({
  rows: z.array(CardInvalidationRowSchema),
});
export type CardInvalidationsListResponse = z.infer<typeof CardInvalidationsListResponseSchema>;

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
