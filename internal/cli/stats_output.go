package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/trevorashby/llamasitter/internal/analytics"
	"github.com/trevorashby/llamasitter/internal/model"
)

const (
	statsDefaultBreakdownRows = 3
	statsDefaultSessionRows   = 5
	statsDefaultRecentRows    = 5
	statsDefaultRenderWidth   = 79
	statsMinRenderWidth       = 72
	statsMaxRenderWidth       = 100
)

type statsStore interface {
	UsageSummary(context.Context, model.RequestFilter) (*model.UsageSummary, error)
	UsageTimeseries(context.Context, model.RequestFilter, string, bool) ([]model.TimeBucket, error)
	UsageHeatmap(context.Context, model.RequestFilter, int, bool) ([]model.HeatmapCell, error)
	ListSessions(context.Context, model.RequestFilter) ([]model.SessionSummary, error)
	ListRequests(context.Context, model.RequestFilter) ([]model.RequestEvent, error)
}

type statsSnapshot struct {
	ConfigPath     string
	GeneratedAt    time.Time
	Summary        *model.UsageSummary
	Last24Hours    *model.UsageSummary
	Last7Days      *model.UsageSummary
	Trend7Days     []model.TimeBucket
	PeakHour       *model.HeatmapCell
	TopSessions    []model.SessionSummary
	RecentRequests []model.RequestEvent
}

func loadStatsSnapshot(ctx context.Context, store statsStore, configPath string, now time.Time) (*statsSnapshot, error) {
	summary, err := store.UsageSummary(ctx, model.RequestFilter{})
	if err != nil {
		return nil, err
	}

	dayStart, dayEnd := analytics.DefaultWindow("day", now)
	last24Hours, err := store.UsageSummary(ctx, model.RequestFilter{
		StartedAfter:  dayStart,
		StartedBefore: dayEnd,
	})
	if err != nil {
		return nil, err
	}

	weekStart, weekEnd := analytics.DefaultWindow("week", now)
	last7Days, err := store.UsageSummary(ctx, model.RequestFilter{
		StartedAfter:  weekStart,
		StartedBefore: weekEnd,
	})
	if err != nil {
		return nil, err
	}

	trend7Days, err := store.UsageTimeseries(ctx, model.RequestFilter{
		StartedAfter:  weekStart,
		StartedBefore: weekEnd,
	}, "week", false)
	if err != nil {
		return nil, err
	}

	_, offsetSeconds := now.Zone()
	heatmap, err := store.UsageHeatmap(ctx, model.RequestFilter{
		StartedAfter:  weekStart,
		StartedBefore: weekEnd,
	}, -offsetSeconds/60, false)
	if err != nil {
		return nil, err
	}

	topSessions, err := store.ListSessions(ctx, model.RequestFilter{Limit: statsDefaultSessionRows})
	if err != nil {
		return nil, err
	}

	recentRequests, err := store.ListRequests(ctx, model.RequestFilter{Limit: statsDefaultRecentRows})
	if err != nil {
		return nil, err
	}

	return &statsSnapshot{
		ConfigPath:     configPath,
		GeneratedAt:    now,
		Summary:        summary,
		Last24Hours:    last24Hours,
		Last7Days:      last7Days,
		Trend7Days:     trend7Days,
		PeakHour:       busiestHeatmapCell(heatmap),
		TopSessions:    topSessions,
		RecentRequests: recentRequests,
	}, nil
}

func renderStatsReport(w io.Writer, snapshot *statsSnapshot) error {
	if snapshot == nil {
		return nil
	}

	width := statsRenderWidth()
	if err := writeStatsBox(w, width, "LlamaSitter Stats", statsHeaderLines(snapshot, width-4)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if err := writeStatsBox(w, width, "Overview", statsOverviewLines(snapshot, width-4)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if err := writeStatsBox(w, width, "Recent Windows", statsWindowLines(snapshot)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if err := writeStatsBox(w, width, "Daily Trend (Last 7d)", statsTrendLines(snapshot, width-4)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if err := writeStatsBox(w, width, "Top Breakdown", statsBreakdownLines(snapshot, width-4)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if err := writeStatsBox(w, width, "Top Sessions", statsSessionLines(snapshot)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return writeStatsBox(w, width, "Recent Requests", statsRecentRequestLines(snapshot))
}

func statsHeaderLines(snapshot *statsSnapshot, innerWidth int) []string {
	lines := []string{}
	lines = append(lines, wrapPrefixedLine("Config     ", snapshot.ConfigPath, innerWidth)...)
	lines = append(lines, wrapPrefixedLine("Generated  ", snapshot.GeneratedAt.Format("2006-01-02 15:04:05 MST"), innerWidth)...)

	if len(snapshot.RecentRequests) == 0 {
		lines = append(lines, "Last seen   No requests captured yet.")
		return lines
	}

	last := snapshot.RecentRequests[0]
	details := []string{
		relativeTime(last.StartedAt, snapshot.GeneratedAt),
		compactRequestSummary(last),
	}
	lines = append(lines, wrapPrefixedLine("Last seen   ", strings.Join(details, " | "), innerWidth)...)
	return lines
}

func statsOverviewLines(snapshot *statsSnapshot, innerWidth int) []string {
	if snapshot.Summary == nil {
		return []string{"No usage data is available yet."}
	}

	summary := snapshot.Summary
	rows := []statsMetricRow{
		{
			LeftLabel:  "Requests",
			LeftValue:  formatCount(summary.RequestCount),
			RightLabel: "Success rate",
			RightValue: summarySuccessRate(summary),
		},
		{
			LeftLabel:  "Unsuccessful",
			LeftValue:  formatCount(unsuccessfulRequests(summary)),
			RightLabel: "Aborted",
			RightValue: formatCount(summary.AbortedCount),
		},
		{
			LeftLabel:  "Active sessions",
			LeftValue:  formatCount(summary.ActiveSessionCount),
			RightLabel: "Avg latency",
			RightValue: formatDurationMs(summary.AvgRequestDurationMs),
		},
		{
			LeftLabel:  "Total tokens",
			LeftValue:  formatCompactCount(summary.TotalTokens),
			RightLabel: "Avg / request",
			RightValue: formatCompactCount(avgTokensPerRequest(summary)),
		},
		{
			LeftLabel:  "Prompt / Output",
			LeftValue:  fmt.Sprintf("%s / %s", formatCompactCount(summary.PromptTokens), formatCompactCount(summary.OutputTokens)),
			RightLabel: "Output / Prompt",
			RightValue: formatOutputPromptRatio(summary),
		},
	}
	return renderMetricRows(rows, innerWidth)
}

func statsWindowLines(snapshot *statsSnapshot) []string {
	lines := []string{
		formatFixedColumns([]string{"WINDOW", "REQUESTS", "TOKENS", "SUCCESS", "AVG LAT"}, []int{14, 10, 10, 10, 10}),
		formatFixedColumns([]string{"------------", "--------", "--------", "--------", "--------"}, []int{14, 10, 10, 10, 10}),
	}

	lines = append(lines, formatFixedColumns([]string{
		"Last 24h",
		formatCount(summaryCount(snapshot.Last24Hours)),
		formatCompactCount(summaryTokens(snapshot.Last24Hours)),
		summarySuccessRate(snapshot.Last24Hours),
		formatDurationMs(summaryLatency(snapshot.Last24Hours)),
	}, []int{14, 10, 10, 10, 10}))

	lines = append(lines, formatFixedColumns([]string{
		"Last 7d",
		formatCount(summaryCount(snapshot.Last7Days)),
		formatCompactCount(summaryTokens(snapshot.Last7Days)),
		summarySuccessRate(snapshot.Last7Days),
		formatDurationMs(summaryLatency(snapshot.Last7Days)),
	}, []int{14, 10, 10, 10, 10}))

	if snapshot.PeakHour != nil {
		lines = append(lines, fmt.Sprintf("Peak hour (7d)  %s | %s req | %s tok",
			peakHourLabel(snapshot.PeakHour),
			formatCount(snapshot.PeakHour.RequestCount),
			formatCompactCount(snapshot.PeakHour.TotalTokens),
		))
	}

	return lines
}

func statsTrendLines(snapshot *statsSnapshot, innerWidth int) []string {
	if len(snapshot.Trend7Days) == 0 || totalTrendRequests(snapshot.Trend7Days) == 0 {
		return []string{"No activity recorded in the last 7 days."}
	}

	maxRequests := int64(0)
	for _, bucket := range snapshot.Trend7Days {
		if bucket.RequestCount > maxRequests {
			maxRequests = bucket.RequestCount
		}
	}

	barWidth := innerWidth - 35
	if barWidth < 8 {
		barWidth = 8
	}

	lines := make([]string, 0, len(snapshot.Trend7Days))
	for _, bucket := range snapshot.Trend7Days {
		bar := scaledBar(bucket.RequestCount, maxRequests, barWidth)
		line := fmt.Sprintf("%-9s %6s req %8s tok  %s",
			bucket.Label,
			formatCount(bucket.RequestCount),
			formatCompactCount(bucket.TotalTokens),
			bar,
		)
		lines = append(lines, line)
	}
	return lines
}

func statsBreakdownLines(snapshot *statsSnapshot, innerWidth int) []string {
	if snapshot.Summary == nil || snapshot.Summary.RequestCount == 0 {
		return []string{"No breakdown data is available yet."}
	}

	lines := []string{}
	lines = append(lines, wrapPrefixedLine("Models     ", summarizeBreakdown(snapshot.Summary.ByModel, statsDefaultBreakdownRows), innerWidth)...)
	if len(snapshot.Summary.ByListenerName) > 0 {
		lines = append(lines, wrapPrefixedLine("Listeners  ", summarizeBreakdown(snapshot.Summary.ByListenerName, statsDefaultBreakdownRows), innerWidth)...)
	}
	lines = append(lines, wrapPrefixedLine("Clients    ", summarizeBreakdown(snapshot.Summary.ByClientType, statsDefaultBreakdownRows), innerWidth)...)
	lines = append(lines, wrapPrefixedLine("Instances  ", summarizeBreakdown(snapshot.Summary.ByClientInstance, statsDefaultBreakdownRows), innerWidth)...)
	lines = append(lines, wrapPrefixedLine("Agents     ", summarizeBreakdown(snapshot.Summary.ByAgentName, statsDefaultBreakdownRows), innerWidth)...)
	return lines
}

func statsSessionLines(snapshot *statsSnapshot) []string {
	lines := []string{
		formatFixedColumns([]string{"SESSION", "REQ", "TOKENS", "LAST", "CALLER", "AGENT"}, []int{12, 5, 8, 9, 12, 12}),
		formatFixedColumns([]string{"-------", "---", "------", "----", "------", "-----"}, []int{12, 5, 8, 9, 12, 12}),
	}

	if len(snapshot.TopSessions) == 0 {
		return append(lines, "No sessions have been attributed yet.")
	}

	for _, session := range snapshot.TopSessions {
		lines = append(lines, formatFixedColumns([]string{
			shorten(session.SessionID, 12),
			formatCount(session.RequestCount),
			formatCompactCount(session.TotalTokens),
			relativeTime(session.LastSeenAt, snapshot.GeneratedAt),
			shorten(callerLabel(session.ClientType, session.ClientInstance), 12),
			shorten(emptyFallback(session.AgentName, "-"), 12),
		}, []int{12, 5, 8, 9, 12, 12}))
	}
	return lines
}

func statsRecentRequestLines(snapshot *statsSnapshot) []string {
	lines := []string{
		formatFixedColumns([]string{"WHEN", "ST", "MODEL", "TOKENS", "LAT", "CALLER", "AGENT"}, []int{7, 4, 10, 7, 7, 13, 8}),
		formatFixedColumns([]string{"----", "--", "-----", "------", "---", "------", "-----"}, []int{7, 4, 10, 7, 7, 13, 8}),
	}

	if len(snapshot.RecentRequests) == 0 {
		return append(lines, "No requests have been captured yet.")
	}

	for _, item := range snapshot.RecentRequests {
		lines = append(lines, formatFixedColumns([]string{
			relativeTime(item.StartedAt, snapshot.GeneratedAt),
			formatStatus(item.HTTPStatus),
			shorten(emptyFallback(item.Model, "-"), 10),
			formatCompactCount(item.TotalTokens),
			formatDurationMs(float64(item.RequestDurationMs)),
			shorten(callerLabel(item.ClientType, item.ClientInstance), 13),
			shorten(emptyFallback(item.AgentName, "-"), 8),
		}, []int{7, 4, 10, 7, 7, 13, 8}))
	}
	return lines
}

type statsMetricRow struct {
	LeftLabel  string
	LeftValue  string
	RightLabel string
	RightValue string
}

func renderMetricRows(rows []statsMetricRow, innerWidth int) []string {
	columnWidth := (innerWidth - 3) / 2
	if columnWidth < 20 {
		columnWidth = 20
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		left := formatMetricCell(row.LeftLabel, row.LeftValue, columnWidth)
		right := formatMetricCell(row.RightLabel, row.RightValue, columnWidth)
		lines = append(lines, left+" | "+right)
	}
	return lines
}

func formatMetricCell(label, value string, width int) string {
	text := fmt.Sprintf("%-14s %s", label, value)
	return padRight(shorten(text, width), width)
}

func writeStatsBox(w io.Writer, width int, title string, lines []string) error {
	if width < statsMinRenderWidth {
		width = statsMinRenderWidth
	}

	innerWidth := width - 4
	border := "+" + strings.Repeat("-", width-2) + "+\n"

	if _, err := io.WriteString(w, border); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| %-*s |\n", innerWidth, shorten(title, innerWidth)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, border); err != nil {
		return err
	}

	for _, line := range lines {
		for _, wrapped := range wrapText(line, innerWidth) {
			if _, err := fmt.Fprintf(w, "| %-*s |\n", innerWidth, wrapped); err != nil {
				return err
			}
		}
	}

	_, err := io.WriteString(w, border)
	return err
}

func wrapText(text string, width int) []string {
	text = strings.TrimRight(text, " ")
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}

	var lines []string
	for len(text) > width {
		cut := strings.LastIndex(text[:width+1], " ")
		if cut <= 0 {
			cut = width
		}
		lines = append(lines, strings.TrimSpace(text[:cut]))
		text = strings.TrimSpace(text[cut:])
	}
	lines = append(lines, text)
	return lines
}

func wrapPrefixedLine(prefix, body string, width int) []string {
	if width <= len(prefix)+4 {
		return []string{prefix + body}
	}
	if strings.TrimSpace(body) == "" {
		return []string{prefix}
	}

	contentWidth := width - len(prefix)
	prefixPadding := strings.Repeat(" ", len(prefix))
	parts := wrapText(body, contentWidth)
	lines := make([]string, 0, len(parts))
	for index, part := range parts {
		if index == 0 {
			lines = append(lines, prefix+part)
			continue
		}
		lines = append(lines, prefixPadding+part)
	}
	return lines
}

func summarizeBreakdown(rows []model.BreakdownRow, limit int) string {
	if len(rows) == 0 {
		return "No data yet."
	}

	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}

	parts := make([]string, 0, limit)
	for _, row := range rows[:limit] {
		parts = append(parts, fmt.Sprintf("%s (%s req, %s tok)", row.Key, formatCount(row.RequestCount), formatCompactCount(row.TotalTokens)))
	}

	if limit < len(rows) {
		parts = append(parts, fmt.Sprintf("+%d more", len(rows)-limit))
	}
	return strings.Join(parts, ", ")
}

func busiestHeatmapCell(cells []model.HeatmapCell) *model.HeatmapCell {
	var best *model.HeatmapCell
	for i := range cells {
		cell := &cells[i]
		if cell.RequestCount == 0 {
			continue
		}
		if best == nil ||
			cell.RequestCount > best.RequestCount ||
			(cell.RequestCount == best.RequestCount && cell.TotalTokens > best.TotalTokens) ||
			(cell.RequestCount == best.RequestCount && cell.TotalTokens == best.TotalTokens && (cell.Weekday < best.Weekday || (cell.Weekday == best.Weekday && cell.Hour < best.Hour))) {
			best = cell
		}
	}
	return best
}

func peakHourLabel(cell *model.HeatmapCell) string {
	if cell == nil {
		return "n/a"
	}

	weekdays := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	weekday := "Day"
	if cell.Weekday >= 0 && cell.Weekday < len(weekdays) {
		weekday = weekdays[cell.Weekday]
	}
	return fmt.Sprintf("%s %s", weekday, formatHour(cell.Hour))
}

func formatHour(hour int) string {
	if hour < 0 || hour > 23 {
		return "--"
	}
	meridiem := "AM"
	display := hour
	switch {
	case hour == 0:
		display = 12
	case hour == 12:
		meridiem = "PM"
	case hour > 12:
		display = hour - 12
		meridiem = "PM"
	}
	if hour >= 12 && hour != 12 {
		meridiem = "PM"
	}
	return fmt.Sprintf("%d %s", display, meridiem)
}

func totalTrendRequests(items []model.TimeBucket) int64 {
	var total int64
	for _, item := range items {
		total += item.RequestCount
	}
	return total
}

func scaledBar(value, max int64, width int) string {
	if value <= 0 || max <= 0 || width <= 0 {
		return ""
	}

	filled := int((value * int64(width)) / max)
	if filled <= 0 {
		filled = 1
	}
	return strings.Repeat("#", filled)
}

func summaryCount(summary *model.UsageSummary) int64 {
	if summary == nil {
		return 0
	}
	return summary.RequestCount
}

func summaryTokens(summary *model.UsageSummary) int64 {
	if summary == nil {
		return 0
	}
	return summary.TotalTokens
}

func summaryLatency(summary *model.UsageSummary) float64 {
	if summary == nil {
		return 0
	}
	return summary.AvgRequestDurationMs
}

func unsuccessfulRequests(summary *model.UsageSummary) int64 {
	if summary == nil {
		return 0
	}
	if summary.RequestCount < summary.SuccessCount {
		return 0
	}
	return summary.RequestCount - summary.SuccessCount
}

func avgTokensPerRequest(summary *model.UsageSummary) int64 {
	if summary == nil || summary.RequestCount <= 0 {
		return 0
	}
	return summary.TotalTokens / summary.RequestCount
}

func formatOutputPromptRatio(summary *model.UsageSummary) string {
	if summary == nil || summary.PromptTokens <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2fx", float64(summary.OutputTokens)/float64(summary.PromptTokens))
}

func summarySuccessRate(summary *model.UsageSummary) string {
	if summary == nil || summary.RequestCount <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", (float64(summary.SuccessCount)/float64(summary.RequestCount))*100)
}

func compactRequestSummary(item model.RequestEvent) string {
	parts := []string{
		emptyFallback(item.Model, "unknown"),
		fmt.Sprintf("%s tok", formatCompactCount(item.TotalTokens)),
		formatDurationMs(float64(item.RequestDurationMs)),
		callerLabel(item.ClientType, item.ClientInstance),
	}
	if item.AgentName != "" {
		parts = append(parts, item.AgentName)
	}
	return strings.Join(parts, " | ")
}

func callerLabel(clientType, clientInstance string) string {
	switch {
	case clientType != "" && clientInstance != "":
		return clientType + "/" + clientInstance
	case clientType != "":
		return clientType
	case clientInstance != "":
		return clientInstance
	default:
		return "unknown"
	}
}

func relativeTime(value, now time.Time) string {
	if value.IsZero() {
		return "n/a"
	}

	diff := now.Sub(value)
	if diff < 0 {
		diff = -diff
	}

	switch {
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}

func formatDurationMs(value float64) string {
	if value <= 0 {
		return "0ms"
	}
	switch {
	case value < 1000:
		return fmt.Sprintf("%.0fms", value)
	case value < 10000:
		return fmt.Sprintf("%.2fs", value/1000)
	case value < 60000:
		return fmt.Sprintf("%.1fs", value/1000)
	default:
		return fmt.Sprintf("%.1fm", value/60000)
	}
}

func formatStatus(status int) string {
	if status <= 0 {
		return "-"
	}
	return strconv.Itoa(status)
}

func formatCount(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		return sign + raw
	}

	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return sign + strings.Join(parts, ",")
}

func formatCompactCount(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	switch {
	case value >= 1_000_000_000:
		return sign + fmt.Sprintf("%.1fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return sign + fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case value >= 1_000:
		return sign + fmt.Sprintf("%.1fk", float64(value)/1_000)
	default:
		return sign + strconv.FormatInt(value, 10)
	}
}

func formatFixedColumns(values []string, widths []int) string {
	parts := make([]string, 0, len(values))
	for index, value := range values {
		width := widths[index]
		parts = append(parts, padRight(shorten(value, width), width))
	}
	return strings.Join(parts, "  ")
}

func padRight(value string, width int) string {
	if width <= len(value) {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func shorten(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func statsRenderWidth() int {
	width := statsDefaultRenderWidth
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			width = parsed
		}
	}
	if width < statsMinRenderWidth {
		return statsMinRenderWidth
	}
	if width > statsMaxRenderWidth {
		return statsMaxRenderWidth
	}
	return width
}
