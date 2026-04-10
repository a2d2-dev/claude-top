package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MessageDetail holds the full content of one assistant turn and its preceding
// user turn, loaded on-demand from the source JSONL file.
// None of these fields are stored in the cache; they are read from disk each time.
type MessageDetail struct {
	// AssistantText is the plain text response.
	AssistantText string
	// ThinkingText is the extended thinking block content, if present.
	ThinkingText string
	// ToolCalls lists tools invoked by the assistant in this turn.
	ToolCalls []ToolCall
	// UserText is the human's message text that triggered this response.
	UserText string
	// ToolResults lists the tool outputs that were part of the user turn.
	ToolResults []ToolResult
	// LoadErr is set if on-demand loading failed (e.g. file not found).
	LoadErr error
}

// ToolCall represents a single tool invocation by the assistant.
type ToolCall struct {
	// ID is the tool_use ID returned by the API.
	ID string
	// Name is the tool function name (e.g. "Read", "Bash").
	Name string
	// Input is the pretty-printed JSON input object.
	Input string
}

// ToolResult represents the output of a tool that was returned in the user turn.
type ToolResult struct {
	// ToolUseID matches the ToolCall.ID from the previous assistant turn.
	ToolUseID string
	// Content is the (possibly truncated) text output.
	Content string
	// IsError indicates the tool returned an error result.
	IsError bool
}

// maxToolContent is the maximum characters to show for a single tool result.
const maxToolContent = 2000

// contentBlock is used to unmarshal individual items in a message content array.
type contentBlock struct {
	Type     string `json:"type"`
	// text / thinking
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
	// tool_use
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	// tool_result
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// FindClaudeSessionFile returns the path of the JSONL file that contains the
// given session ID. Claude stores sessions as <dataPath>/*/<sessionID>.jsonl.
// Returns "" if not found.
func FindClaudeSessionFile(dataPath, sessionID string) string {
	if sessionID == "" {
		return ""
	}
	if dataPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataPath = filepath.Join(home, ".claude", "projects")
	}
	matches, _ := filepath.Glob(filepath.Join(dataPath, "*", sessionID+".jsonl"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// ReadMessageDetail opens sourceFile and extracts the full content of the
// assistant message identified by messageID, along with its preceding user turn.
func ReadMessageDetail(sourceFile, messageID string) (*MessageDetail, error) {
	f, err := os.Open(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", sourceFile, err)
	}
	defer f.Close()

	// Read all lines, then build uuid-keyed index.
	// NOTE: the index is built AFTER collecting all lines so that append-growth
	// never invalidates the pointers stored in byUUID.
	type rawLine struct {
		entryType  string
		uuid       string
		parentUUID string
		raw        []byte
	}

	var lines []rawLine
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		b := scanner.Bytes()
		if len(b) == 0 {
			continue
		}
		var hdr struct {
			Type       string `json:"type"`
			UUID       string `json:"uuid"`
			ParentUUID string `json:"parentUuid"`
		}
		if err := json.Unmarshal(b, &hdr); err != nil {
			continue
		}
		cp := make([]byte, len(b))
		copy(cp, b)
		lines = append(lines, rawLine{
			entryType:  hdr.Type,
			uuid:       hdr.UUID,
			parentUUID: hdr.ParentUUID,
			raw:        cp,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Build uuid → line index after slice is fully populated (no more appends).
	byUUID := make(map[string]*rawLine, len(lines))
	for i := range lines {
		if lines[i].uuid != "" {
			byUUID[lines[i].uuid] = &lines[i]
		}
	}

	// Find the assistant line whose message.id matches messageID.
	var aLine *rawLine
	for i := range lines {
		l := &lines[i]
		if l.entryType != "assistant" {
			continue
		}
		var raw rawEntry
		if err := json.Unmarshal(l.raw, &raw); err != nil {
			continue
		}
		if raw.Message != nil && raw.Message.ID == messageID {
			aLine = l
			break
		}
	}
	if aLine == nil {
		return nil, fmt.Errorf("message %s not found in %s", messageID, sourceFile)
	}

	detail := &MessageDetail{}

	// Parse assistant content.
	var assistantRaw rawEntry
	if err := json.Unmarshal(aLine.raw, &assistantRaw); err == nil && assistantRaw.Message != nil {
		blocks := parseContentBlocks(assistantRaw.Message.Content)
		for _, b := range blocks {
			switch b.Type {
			case "text":
				detail.AssistantText += b.Text
			case "thinking":
				detail.ThinkingText += b.Thinking
			case "tool_use":
				detail.ToolCalls = append(detail.ToolCalls, ToolCall{
					ID:    b.ID,
					Name:  b.Name,
					Input: prettyJSON(b.Input),
				})
			}
		}
	}

	// Find and parse the preceding user turn.
	userLine := byUUID[aLine.parentUUID]
	if userLine != nil && userLine.entryType == "user" {
		var userRaw rawEntry
		if err := json.Unmarshal(userLine.raw, &userRaw); err == nil && userRaw.Message != nil {
			blocks := parseContentBlocks(userRaw.Message.Content)
			for _, b := range blocks {
				switch b.Type {
				case "text":
					detail.UserText += b.Text
				case "tool_result":
					detail.ToolResults = append(detail.ToolResults, ToolResult{
						ToolUseID: b.ToolUseID,
						Content:   extractToolResultText(b.Content),
						IsError:   b.IsError,
					})
				}
			}
		}
	}

	return detail, nil
}

// parseContentBlocks unmarshals a message content field into a slice of
// contentBlock. Content can be a JSON string or a JSON array.
func parseContentBlocks(raw json.RawMessage) []contentBlock {
	if len(raw) == 0 {
		return nil
	}
	// Try as array first (most common for structured messages).
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}
	// Fall back to plain string (older Claude format).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return []contentBlock{{Type: "text", Text: s}}
	}
	return nil
}

// extractToolResultText extracts human-readable text from a tool_result content field.
// Content can be a string, an array of content blocks, or arbitrary JSON.
func extractToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return truncateContent(s)
	}
	// Try array of content blocks.
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return truncateContent(strings.Join(parts, "\n"))
	}
	// Fallback: raw JSON.
	return truncateContent(string(raw))
}

// prettyJSON returns a human-readable JSON representation of raw.
func prettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.MarshalIndent(v, "    ", "  ")
	if err != nil {
		return string(raw)
	}
	return "    " + string(b)
}

// truncateContent returns at most maxToolContent characters of s, appending "…" if longer.
func truncateContent(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxToolContent {
		return s
	}
	return string(runes[:maxToolContent-1]) + "…"
}
