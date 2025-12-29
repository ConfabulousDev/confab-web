// Zod schemas for API response validation
// Validates all data received from backend APIs
import { z } from 'zod';

// ============================================================================
// Common Schemas
// ============================================================================

export const GitInfoSchema = z.object({
  repo_url: z.string().optional(),
  branch: z.string().optional(),
  commit_sha: z.string().optional(),
  commit_message: z.string().optional(),
  author: z.string().optional(),
  is_dirty: z.boolean().optional(),
});

export const SyncFileDetailSchema = z.object({
  file_name: z.string(),
  file_type: z.string(),
  last_synced_line: z.number(),
  updated_at: z.string(),
});

// ============================================================================
// Session Schemas
// ============================================================================

export const SessionSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  first_seen: z.string(),
  file_count: z.number(),
  last_sync_time: z.string().nullable().optional(),
  custom_title: z.string().max(255).nullable().optional(),
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
  hostname: z.string().nullable().optional(), // Client machine hostname (owner-only, null for shared)
  username: z.string().nullable().optional(), // OS username (owner-only, null for shared)
});

export const SessionDetailSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  custom_title: z.string().max(255).nullable().optional(),
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
});

export const SessionShareSchema = z.object({
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
  name: z.string(),
  email: z.string(),
  avatar_url: z.string(),
});

// ============================================================================
// API Key Schemas
// ============================================================================

export const APIKeySchema = z.object({
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

export const GitHubLinkTypeSchema = z.enum(['commit', 'pull_request']);
export const GitHubLinkSourceSchema = z.enum(['cli_hook', 'manual']);

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

export const TokenStatsSchema = z.object({
  input: z.number(),
  output: z.number(),
  cache_creation: z.number(),
  cache_read: z.number(),
});

export const CostStatsSchema = z.object({
  estimated_usd: z.string(), // Decimal serialized as string from backend
});

export const CompactionInfoSchema = z.object({
  auto: z.number(),
  manual: z.number(),
  avg_time_ms: z.number().nullable().optional(),
});

// Card data schemas for the new cards-based format
// Tokens card includes cost info (consolidated from previous separate cost card)
export const TokensCardDataSchema = z.object({
  input: z.number(),
  output: z.number(),
  cache_creation: z.number(),
  cache_read: z.number(),
  estimated_usd: z.string(), // Consolidated from cost card
});

// Session card includes compaction info (consolidated from previous separate compaction card)
// Note: Messages with text+tool_use count as text_responses, not tool_calls.
// Therefore assistant_messages may not equal text_responses + tool_calls + thinking_blocks.
export const SessionCardDataSchema = z.object({
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

  // Actual conversational turns
  user_turns: z.number(), // Same as human_prompts
  assistant_turns: z.number(), // Same as text_responses

  // Session metadata
  duration_ms: z.number().nullable().optional(),
  models_used: z.array(z.string()),

  // Compaction stats (consolidated from previous separate compaction card)
  compaction_auto: z.number(),
  compaction_manual: z.number(),
  compaction_avg_time_ms: z.number().nullable().optional(),
});

export const ToolStatsSchema = z.object({
  success: z.number(),
  errors: z.number(),
});

export const ToolsCardDataSchema = z.object({
  total_calls: z.number(),
  tool_stats: z.record(z.string(), ToolStatsSchema),
  error_count: z.number(),
});

// Cards map schema - extensible for future cards
// All fields optional to handle empty analytics (session with no transcript)
// Note: cost is now part of tokens card, compaction is now part of session card
export const AnalyticsCardsSchema = z.object({
  tokens: TokensCardDataSchema.optional(),
  session: SessionCardDataSchema.optional(),
  tools: ToolsCardDataSchema.optional(),
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
});

// ============================================================================
// Array Response Schemas
// ============================================================================

export const SessionListSchema = z.array(SessionSchema);
export const SessionShareListSchema = z.array(SessionShareSchema);
export const APIKeyListSchema = z.array(APIKeySchema);

// ============================================================================
// Inferred Types
// ============================================================================

export type GitInfo = z.infer<typeof GitInfoSchema>;
export type SyncFileDetail = z.infer<typeof SyncFileDetailSchema>;
export type Session = z.infer<typeof SessionSchema>;
export type SessionDetail = z.infer<typeof SessionDetailSchema>;
export type SessionShare = z.infer<typeof SessionShareSchema>;
export type User = z.infer<typeof UserSchema>;
export type APIKey = z.infer<typeof APIKeySchema>;
export type CreateAPIKeyResponse = z.infer<typeof CreateAPIKeyResponseSchema>;
export type CreateShareResponse = z.infer<typeof CreateShareResponseSchema>;
export type GitHubLinkType = z.infer<typeof GitHubLinkTypeSchema>;
export type GitHubLinkSource = z.infer<typeof GitHubLinkSourceSchema>;
export type GitHubLink = z.infer<typeof GitHubLinkSchema>;
export type GitHubLinksResponse = z.infer<typeof GitHubLinksResponseSchema>;
export type TokenStats = z.infer<typeof TokenStatsSchema>;
export type CostStats = z.infer<typeof CostStatsSchema>;
export type CompactionInfo = z.infer<typeof CompactionInfoSchema>;
export type TokensCardData = z.infer<typeof TokensCardDataSchema>;
export type SessionCardData = z.infer<typeof SessionCardDataSchema>;
export type ToolStats = z.infer<typeof ToolStatsSchema>;
export type ToolsCardData = z.infer<typeof ToolsCardDataSchema>;
export type AnalyticsCards = z.infer<typeof AnalyticsCardsSchema>;
export type SessionAnalytics = z.infer<typeof SessionAnalyticsSchema>;

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
export class APIValidationError extends Error {
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
