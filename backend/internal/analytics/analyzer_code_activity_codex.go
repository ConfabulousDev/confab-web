package analytics

import (
	"bufio"
	"path/filepath"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// computeCodexCodeActivity fills the Code Activity card from whichever stream
// the rollout's CLI era used. Codex reshaped both signals at 0.149.1, so both
// shapes are read per-event — a session that spans a CLI upgrade carries both
// in one file.
//
//	<=0.130.0  File edits arrive as apply_patch tool calls carrying a
//	           `*** Begin Patch` envelope. FilesRead and SearchCount stay at
//	           zero: exec_command records the shell line but nothing about what
//	           it did, and reconstructing that from command text would be a
//	           guess. Genuinely unavailable, not an omission.
//	>=0.149.1  File edits arrive as event_msg item_completed FileChange items
//	           carrying a per-file diff, and CommandExecution items carry
//	           Codex's own parsed_cmd classification — which does populate
//	           FilesRead and SearchCount.
//
// The asymmetry is deliberate and user-visible: the modern format states
// something the old one never recorded, and backfilling the old era would mean
// inventing a classifier for commands Codex itself never classified.
func computeCodexCodeActivity(out *ComputeResult, r *codex.ParsedRollout) {
	for _, turn := range r.Turns {
		for _, tc := range turn.ToolCalls {
			if tc.Name != "apply_patch" {
				continue
			}
			files, added, removed := parseApplyPatch(tc.Arguments, out.LanguageBreakdown)
			out.FilesModified += files
			out.LinesAdded += added
			out.LinesRemoved += removed
		}

		for _, edit := range turn.FileEdits {
			out.FilesModified++
			if lang := languageFromPath(edit.Path); lang != "" {
				out.LanguageBreakdown[lang]++
			}
			added, removed := countFileEditLines(edit)
			out.LinesAdded += added
			out.LinesRemoved += removed
		}

		for _, kind := range turn.ParsedCommandKinds {
			// Bucket explicitly. "unknown" is the majority of real entries
			// (159 of 214 across the sampled rollouts), so a default bucket
			// would inflate the card wildly; "list_files" is neither a read
			// nor a search.
			switch kind {
			case "read":
				out.FilesRead++
			case "search":
				out.SearchCount++
			}
		}
	}
}

// countFileEditLines returns the +/- line counts for one >=0.149.1 file change.
// An `update` states them as a unified diff; an `add` carries the whole new
// file, every line of which is added. This mirrors what parseApplyPatch derives
// from the <=0.130.0 envelope, where an `*** Add File` block lists its content
// as `+` lines — so the two eras count the same edit the same way.
//
// Edit status is deliberately ignored, matching the old-era path: both count a
// reported change whether or not it ultimately applied.
func countFileEditLines(edit codex.FileEdit) (added, removed int) {
	if edit.UnifiedDiff == "" {
		return countLines(edit.Content), 0
	}
	// Same +/- accounting as parseApplyPatch below, headers excluded.
	for _, line := range strings.Split(edit.UnifiedDiff, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			removed++
		}
	}
	return added, removed
}

// parseApplyPatch parses a Codex apply_patch envelope, returning the number
// of files touched (any of Add/Update/Delete) and the cumulative +/- line
// counts. If langs is non-nil it's updated with file-extension language counts.
func parseApplyPatch(envelope string, langs map[string]int) (files, added, removed int) {
	scanner := bufio.NewScanner(strings.NewReader(envelope))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	inFile := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "*** Add File: "),
			strings.HasPrefix(line, "*** Update File: "),
			strings.HasPrefix(line, "*** Delete File: "):
			files++
			inFile = true
			if langs != nil {
				path := line[strings.Index(line, ": ")+2:]
				if lang := languageFromPath(path); lang != "" {
					langs[lang]++
				}
			}
		case strings.HasPrefix(line, "*** End Patch"),
			strings.HasPrefix(line, "*** Begin Patch"):
			inFile = false
		case inFile && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added++
		case inFile && strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			removed++
		}
	}
	return files, added, removed
}

// languageFromPath returns a language label from a file extension, mirroring
// the conventions used elsewhere in analytics (e.g. analyzer_code_activity).
// Returns "" for unrecognized extensions.
func languageFromPath(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "go":
		return "go"
	case "py":
		return "python"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "rb":
		return "ruby"
	case "cs":
		return "csharp"
	case "cpp", "cc", "cxx", "hpp", "h":
		return "cpp"
	case "c":
		return "c"
	case "sh", "bash", "zsh":
		return "shell"
	case "md", "markdown":
		return "markdown"
	case "yml", "yaml":
		return "yaml"
	case "json":
		return "json"
	case "sql":
		return "sql"
	case "html":
		return "html"
	case "css", "scss":
		return "css"
	}
	return ""
}
