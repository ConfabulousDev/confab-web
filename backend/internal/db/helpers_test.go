package db

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"https with .git", "https://github.com/ConfabulousDev/confab-web.git", "ConfabulousDev/confab-web"},
		{"https without .git", "https://github.com/ConfabulousDev/confab-web", "ConfabulousDev/confab-web"},
		{"ssh with .git", "git@github.com:ConfabulousDev/confab.git", "ConfabulousDev/confab"},
		{"ssh without .git", "git@github.com:ConfabulousDev/confab", "ConfabulousDev/confab"},
		{"gitlab https", "https://gitlab.com/group/subgroup/repo.git", "subgroup/repo"},
		{"trailing slash falls through", "just-a-string", "just-a-string"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractRepoName(c.input)
			if got == nil {
				t.Fatalf("ExtractRepoName(%q) returned nil", c.input)
			}
			if *got != c.want {
				t.Errorf("ExtractRepoName(%q) = %q, want %q", c.input, *got, c.want)
			}
		})
	}
}

func TestIsInvalidUUIDError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"matches pg error string", errors.New(`pq: invalid input syntax for type uuid: "bogus"`), true},
		{"unrelated error", errors.New("ECONNREFUSED"), false},
		{"empty-ish error message", errors.New("some other error"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsInvalidUUIDError(c.err); got != c.want {
				t.Errorf("IsInvalidUUIDError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"pg code 23505", errors.New("ERROR: duplicate key value violates SQLSTATE 23505"), true},
		{"phrase match", errors.New("duplicate key value violates unique constraint"), true},
		{"unrelated", errors.New("not found"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsUniqueViolation(c.err); got != c.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestUnmarshalSessionGitInfo_PopulatesField(t *testing.T) {
	session := &SessionDetail{}
	gitInfo := map[string]any{
		"repo_url": "https://github.com/example/repo",
		"branch":   "main",
	}
	raw, err := json.Marshal(gitInfo)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := UnmarshalSessionGitInfo(session, raw); err != nil {
		t.Fatalf("UnmarshalSessionGitInfo returned error: %v", err)
	}
	if session.GitInfo == nil {
		t.Fatal("GitInfo not populated")
	}
	asMap, ok := session.GitInfo.(map[string]any)
	if !ok {
		t.Fatalf("GitInfo is %T, want map[string]any", session.GitInfo)
	}
	if asMap["repo_url"] != "https://github.com/example/repo" {
		t.Errorf("repo_url = %v", asMap["repo_url"])
	}
}

func TestUnmarshalSessionGitInfo_EmptyIsNoop(t *testing.T) {
	session := &SessionDetail{}
	if err := UnmarshalSessionGitInfo(session, nil); err != nil {
		t.Errorf("expected nil error for nil bytes, got %v", err)
	}
	if session.GitInfo != nil {
		t.Error("GitInfo should remain nil for empty input")
	}
}

func TestUnmarshalSessionGitInfo_InvalidJSON(t *testing.T) {
	session := &SessionDetail{}
	err := UnmarshalSessionGitInfo(session, []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal git_info") {
		t.Errorf("error message should mention git_info, got: %v", err)
	}
}
