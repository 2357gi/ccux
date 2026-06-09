package transcript

import (
	"strings"
	"testing"
)

func TestEncodeProjectDir(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "simple home path with underscore",
			cwd:  "/Users/kento_ogi/dotfiles",
			want: "-Users-kento-ogi-dotfiles",
		},
		{
			name: "path with dots",
			cwd:  "/Users/kento_ogi/src/github.com/nealle/sre-toolbox",
			want: "-Users-kento-ogi-src-github-com-nealle-sre-toolbox",
		},
		{
			name: "trailing slash",
			cwd:  "/tmp/foo/",
			want: "-tmp-foo-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncodeProjectDir(tt.cwd); got != tt.want {
				t.Errorf("EncodeProjectDir(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}

func TestProjectPath(t *testing.T) {
	got := ProjectPath("/home/me", "/Users/kento_ogi/dotfiles")
	want := "/home/me/.claude/projects/-Users-kento-ogi-dotfiles"
	if got != want {
		t.Errorf("ProjectPath = %q, want %q", got, want)
	}
}

const sampleJSONL = `{"type":"user","sessionId":"sess-1","cwd":"/Users/kento_ogi/dotfiles","gitBranch":"master","timestamp":"2026-06-08T13:00:00.000Z","message":{"role":"user","content":"first prompt"}}
{"type":"assistant","sessionId":"sess-1","gitBranch":"master","timestamp":"2026-06-08T13:00:10.000Z","message":{"role":"assistant","content":[{"type":"text","text":"working on it"}]}}
{"type":"user","sessionId":"sess-1","gitBranch":"master","timestamp":"2026-06-08T13:01:00.000Z","message":{"role":"user","content":[{"type":"tool_result","content":"ignored tool output"}]}}
{"type":"user","sessionId":"sess-1","gitBranch":"feature/x","timestamp":"2026-06-08T13:02:00.000Z","message":{"role":"user","content":"second prompt with more detail"}}
{"type":"assistant","sessionId":"sess-1","gitBranch":"feature/x","timestamp":"2026-06-08T13:02:30.000Z","message":{"role":"assistant","content":[{"type":"text","text":"final answer here"},{"type":"tool_use","name":"Bash"}]}}
{"type":"ai-title","aiTitle":"Refactor the build pipeline","sessionId":"sess-1"}
{"type":"last-prompt","leafUuid":"abc","sessionId":"sess-1"}`

func TestParse(t *testing.T) {
	info, err := Parse(strings.NewReader(sampleJSONL))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if info.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", info.SessionID)
	}
	if info.AITitle != "Refactor the build pipeline" {
		t.Errorf("AITitle = %q, want 'Refactor the build pipeline'", info.AITitle)
	}
	if info.LastUserPrompt != "second prompt with more detail" {
		t.Errorf("LastUserPrompt = %q, want 'second prompt with more detail'", info.LastUserPrompt)
	}
	if info.LastAssistantText != "final answer here" {
		t.Errorf("LastAssistantText = %q, want 'final answer here'", info.LastAssistantText)
	}
	if info.GitBranch != "feature/x" {
		t.Errorf("GitBranch = %q, want feature/x", info.GitBranch)
	}
	if info.NumUserPrompts != 2 {
		t.Errorf("NumUserPrompts = %d, want 2 (tool_result is not a prompt)", info.NumUserPrompts)
	}
	if info.LastActivity.IsZero() {
		t.Error("LastActivity should be set")
	}
	// last activity should be the assistant's 13:02:30 entry
	if got := info.LastActivity.UTC().Format("15:04:05"); got != "13:02:30" {
		t.Errorf("LastActivity = %s, want 13:02:30", got)
	}
}

func TestParseExtractsPendingQuestion(t *testing.T) {
	jsonl := `{"type":"user","sessionId":"s","timestamp":"2026-06-09T01:00:00.000Z","message":{"role":"user","content":"set up the database"}}
{"type":"assistant","sessionId":"s","timestamp":"2026-06-09T01:00:05.000Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"AskUserQuestion","input":{"questions":[{"question":"Which database?","header":"DB","options":[{"label":"Postgres"},{"label":"MySQL"}]}]}}]}}`

	info, err := Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !strings.Contains(info.Question, "Which database?") {
		t.Errorf("Question = %q, want it to contain the question text", info.Question)
	}
	if !strings.Contains(info.Question, "Postgres") || !strings.Contains(info.Question, "MySQL") {
		t.Errorf("Question = %q, want it to include the option labels", info.Question)
	}
}

func TestParseClearsAnsweredQuestion(t *testing.T) {
	// A question followed by a plain text assistant turn means the round is over;
	// it must no longer be reported as a pending question.
	jsonl := `{"type":"assistant","sessionId":"s","timestamp":"2026-06-09T01:00:05.000Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"AskUserQuestion","input":{"questions":[{"question":"Which database?","options":[{"label":"Postgres"}]}]}}]}}
{"type":"assistant","sessionId":"s","timestamp":"2026-06-09T01:01:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":"Going with Postgres."}]}}`

	info, err := Parse(strings.NewReader(jsonl))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if info.Question != "" {
		t.Errorf("Question = %q, want empty (a later text reply ends the question)", info.Question)
	}
	if info.LastAssistantText != "Going with Postgres." {
		t.Errorf("LastAssistantText = %q", info.LastAssistantText)
	}
}

func TestParseEmpty(t *testing.T) {
	info, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse(empty) error: %v", err)
	}
	if info.SessionID != "" || info.AITitle != "" || info.LastUserPrompt != "" {
		t.Errorf("empty transcript should yield empty Info, got %+v", info)
	}
}

func TestParseSkipsMalformedLines(t *testing.T) {
	in := "not json\n" + `{"type":"ai-title","aiTitle":"ok","sessionId":"s"}` + "\n{bad\n"
	info, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse should skip malformed lines, got error: %v", err)
	}
	if info.AITitle != "ok" {
		t.Errorf("AITitle = %q, want ok", info.AITitle)
	}
}
