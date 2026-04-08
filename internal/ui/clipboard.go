package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// copyToClipboard writes text to the system clipboard.
// Returns an error if no clipboard tool is available.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip, xsel, or wl-copy)")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// formatMsgForCopy returns a plain-text summary of a usage entry for clipboard.
func formatMsgForCopy(e data.UsageEntry, sessionCost float64) string {
	total := e.InputTokens + e.OutputTokens + e.CacheCreationTokens + e.CacheReadTokens
	pct := func(n int) float64 {
		if total == 0 {
			return 0
		}
		return float64(n) / float64(total) * 100
	}
	costPct := 0.0
	if sessionCost > 0 {
		costPct = e.CostUSD / sessionCost * 100
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Time:          %s\n", e.Timestamp.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "Model:         %s\n", e.Model)
	fmt.Fprintf(&sb, "Cost:          $%.6f  (%.1f%% of session)\n", e.CostUSD, costPct)
	fmt.Fprintf(&sb, "\nTOKEN BREAKDOWN\n")
	fmt.Fprintf(&sb, "Input:         %s  (%.1f%%)\n", formatInt(e.InputTokens), pct(e.InputTokens))
	fmt.Fprintf(&sb, "Output:        %s  (%.1f%%)\n", formatInt(e.OutputTokens), pct(e.OutputTokens))
	fmt.Fprintf(&sb, "Cache Read:    %s  (%.1f%%)\n", formatInt(e.CacheReadTokens), pct(e.CacheReadTokens))
	fmt.Fprintf(&sb, "Cache Create:  %s  (%.1f%%)  [3.75× cost weight]\n", formatInt(e.CacheCreationTokens), pct(e.CacheCreationTokens))
	fmt.Fprintf(&sb, "Total:         %s\n", formatInt(total))
	if e.UserPrompt != "" {
		fmt.Fprintf(&sb, "\nPROMPT\n%s\n", e.UserPrompt)
	}
	return sb.String()
}
