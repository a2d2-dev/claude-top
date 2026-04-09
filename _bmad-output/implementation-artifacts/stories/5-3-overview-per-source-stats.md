# Story 5.3: Overview Per-Source Stats

Status: done

## Story

As a multi-source data user,
I want to see Claude and Codex costs broken out in the Overview tab,
So that I can clearly see how much each tool is costing me.

## Acceptance Criteria

1. ALL-TIME TOTALS remains unchanged (merged totals).
2. When both Claude and Codex data exist, two group rows appear below totals.
3. When only one source has data, no group rows are shown.

## Implementation

Updated `internal/ui/tab_overview.go` to compute per-source token/cost/session counts
and conditionally render two extra rows below ALL-TIME TOTALS when both sources present.
