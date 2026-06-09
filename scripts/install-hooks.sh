#!/usr/bin/env bash
# install-hooks.sh — add ccux's status hooks to a Claude Code settings.json.
#
# ccux reports each session's status from authoritative Claude Code events
# (instead of scraping the rendered pane). This script registers the hooks that
# write that state. It is idempotent and preserves any existing hooks.
#
# Usage:
#   scripts/install-hooks.sh [SETTINGS_JSON]   # default: ~/.claude/settings.json
#   CCUX=/path/to/ccux scripts/install-hooks.sh
set -euo pipefail

SETTINGS="${1:-$HOME/.claude/settings.json}"
CCUX="${CCUX:-$HOME/go/bin/ccux}"

command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }
[ -f "$SETTINGS" ] || { echo "no settings file: $SETTINGS" >&2; exit 1; }

jq \
  --arg working "$CCUX hook working" \
  --arg idle "$CCUX hook idle" \
  --arg waiting "$CCUX hook waiting" \
  --arg end "$CCUX hook end" '
  def addcmd(c): if any(.hooks[]?; .command == c) then . else .hooks += [{"type":"command","command":c}] end;
  .hooks = (.hooks // {})
  | .hooks.UserPromptSubmit = [{"hooks":[{"type":"command","command":$working}]}]
  | .hooks.PostToolUse      = [{"matcher":"*","hooks":[{"type":"command","command":$working}]}]
  | .hooks.Stop             = [{"hooks":[{"type":"command","command":$idle}]}]
  | .hooks.SessionEnd       = [{"hooks":[{"type":"command","command":$end}]}]
  | .hooks.Notification     = ((.hooks.Notification // []) | map(
      if .matcher == "idle_prompt" then addcmd($idle)
      elif (.matcher == "permission_prompt" or .matcher == "tool_permission_prompt" or .matcher == "user_question_prompt") then addcmd($waiting)
      else . end))
' "$SETTINGS" > "$SETTINGS.ccux.tmp"

command mv -f "$SETTINGS.ccux.tmp" "$SETTINGS"
echo "ccux hooks installed in: $SETTINGS"
