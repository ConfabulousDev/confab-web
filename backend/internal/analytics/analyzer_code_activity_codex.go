package analytics

import (
	"bufio"
	"path/filepath"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// computeCodexCodeActivity inspects apply_patch tool calls (the Codex
// equivalent of Edit/Write). Codex doesn't have a Read tool, so FilesRead
// stays at zero — intentional, not an omission.
//
// SearchCount is left at zero. Codex's web_search_call is semantically a
// web search rather than the grep/glob "file search" that Claude's
// SearchCount tracks.
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
	}
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
