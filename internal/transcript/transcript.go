// Package transcript reads Claude Code session transcript files (JSONL) and
// resolves the on-disk location of a project's transcripts from a working
// directory.
package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Info is the distilled view of a session transcript used to build a recap.
type Info struct {
	SessionID         string
	AITitle           string // AI-generated session title (best one-line recap)
	LastUserPrompt    string // last genuine user prompt (typed text, not tool results)
	LastAssistantText string // last assistant text reply
	GitBranch         string
	LastActivity      time.Time
	NumUserPrompts    int
}

// EncodeProjectDir converts a working directory into the directory name Claude
// Code uses under ~/.claude/projects. Every non-alphanumeric character becomes
// a dash, e.g. /Users/kento_ogi/dotfiles -> -Users-kento-ogi-dotfiles.
func EncodeProjectDir(cwd string) string {
	var b strings.Builder
	b.Grow(len(cwd))
	for _, r := range cwd {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// ProjectPath returns the absolute path to a project's transcript directory.
func ProjectPath(home, cwd string) string {
	return filepath.Join(home, ".claude", "projects", EncodeProjectDir(cwd))
}

type rawEntry struct {
	Type        string      `json:"type"`
	SessionID   string      `json:"sessionId"`
	GitBranch   string      `json:"gitBranch"`
	Timestamp   string      `json:"timestamp"`
	IsSidechain bool        `json:"isSidechain"`
	AITitle     string      `json:"aiTitle"`
	Message     *rawMessage `json:"message"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Parse reads a JSONL transcript and extracts an Info. Malformed lines are
// skipped so a partially-written transcript (the common live case) still parses.
func Parse(r io.Reader) (Info, error) {
	var info Info
	sc := bufio.NewScanner(r)
	// transcripts can contain very long lines (large tool outputs); grow the buffer.
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed line
		}

		if e.SessionID != "" {
			info.SessionID = e.SessionID
		}
		if e.GitBranch != "" {
			info.GitBranch = e.GitBranch
		}
		if ts := parseTime(e.Timestamp); !ts.IsZero() && ts.After(info.LastActivity) {
			info.LastActivity = ts
		}

		switch e.Type {
		case "ai-title":
			if e.AITitle != "" {
				info.AITitle = e.AITitle
			}
		case "user":
			if e.IsSidechain {
				continue
			}
			if text, ok := userPromptText(e.Message); ok {
				info.LastUserPrompt = text
				info.NumUserPrompts++
			}
		case "assistant":
			if e.IsSidechain {
				continue
			}
			if text := assistantText(e.Message); text != "" {
				info.LastAssistantText = text
			}
		}
	}
	if err := sc.Err(); err != nil {
		return info, err
	}
	return info, nil
}

// userPromptText returns the text of a genuine typed user prompt. Tool-result
// messages (content is an array of tool_result blocks) are not prompts.
func userPromptText(m *rawMessage) (string, bool) {
	if m == nil || len(m.Content) == 0 {
		return "", false
	}
	// content may be a plain string (the typical typed prompt)
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return "", false
		}
		return s, true
	}
	// or an array of content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return "", false
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			texts = append(texts, strings.TrimSpace(b.Text))
		}
	}
	if len(texts) == 0 {
		return "", false // e.g. only tool_result blocks
	}
	return strings.Join(texts, " "), true
}

// assistantText returns the concatenated text blocks of an assistant message.
func assistantText(m *rawMessage) string {
	if m == nil || len(m.Content) == 0 {
		return ""
	}
	var blocks []contentBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		// assistant content might also be a bare string
		var s string
		if json.Unmarshal(m.Content, &s) == nil {
			return strings.TrimSpace(s)
		}
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			texts = append(texts, strings.TrimSpace(b.Text))
		}
	}
	return strings.Join(texts, " ")
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}
