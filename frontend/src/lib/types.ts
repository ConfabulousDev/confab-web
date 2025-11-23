// Shared TypeScript types for Confab frontend

// Re-export transcript types
export * from './types/transcript';

// File detail within a run
export type FileDetail = {
	id: number;
	file_path: string;
	file_type: string;
	size_bytes: number;
	s3_key?: string;
	s3_uploaded_at?: string;
};

// Git repository information
export type GitInfo = {
	repo_url?: string;
	branch?: string;
	commit_sha?: string;
	commit_message?: string;
	author?: string;
	is_dirty?: boolean;
};

// Run detail with files
export type RunDetail = {
	id: number;
	end_timestamp: string;
	cwd: string;
	reason: string;
	transcript_path: string;
	s3_uploaded: boolean;
	git_info?: GitInfo;
	files: FileDetail[];
};

// Full session detail with runs
export type SessionDetail = {
	session_id: string;
	first_seen: string;
	runs: RunDetail[];
};

// Session list item (summary)
export type Session = {
	session_id: string;
	first_seen: string;
	run_count: number;
	last_run_time: string;
	title?: string;
	session_type: string;
	max_transcript_size: number; // Max transcript size across all runs (0 = empty session)
	git_repo?: string; // Git repository from latest run (e.g., "org/repo")
	git_branch?: string; // Git branch from latest run
};

// Share configuration
export type SessionShare = {
	id: number;
	share_token: string;
	visibility: string;
	invited_emails?: string[];
	expires_at?: string;
	created_at: string;
	last_accessed_at?: string;
};

// API key
export type APIKey = {
	id: number;
	name: string;
	created_at: string;
};

// Todo item from Claude Code todo list
export type TodoItem = {
	content: string;
	status: 'pending' | 'in_progress' | 'completed';
	activeForm: string;
};
