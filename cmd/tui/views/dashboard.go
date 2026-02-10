package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gorm.io/gorm"
)

// DashboardView displays real-time stats and system health
type DashboardView struct {
	db     *gorm.DB
	width  int
	height int

	// Stats
	totalProviders  int
	activeProviders int
	totalRequests   int64
	successRate     float64
	avgLatency      int
	costSaved       float64
	uptime          time.Duration

	// Top providers
	topProviders []ProviderHealth

	// Recent logs
	recentLogs []LogEntry

	// Styles
	boxStyle       lipgloss.Style
	titleStyle     lipgloss.Style
	statLabelStyle lipgloss.Style
	statValueStyle lipgloss.Style
	healthyStyle   lipgloss.Style
	warningStyle   lipgloss.Style
	errorStyle     lipgloss.Style
}

// ProviderHealth represents provider health info
type ProviderHealth struct {
	Name        string
	HealthScore float64
	Latency     int
	Status      string
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Provider  string
}

// NewDashboardView creates a new dashboard view
func NewDashboardView(db *gorm.DB) *DashboardView {
	return &DashboardView{
		db: db,
		boxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF10F0")).
			Padding(1, 2).
			MarginBottom(1),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true).
			MarginBottom(1),
		statLabelStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")),
		statValueStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true),
		healthyStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")),
		warningStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")),
		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")),
	}
}

// Init initializes the dashboard
func (d *DashboardView) Init() tea.Cmd {
	return d.fetchData()
}

// Update handles messages
func (d *DashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		// Handle key events if needed
	}
	return d, nil
}

// View renders the dashboard
func (d *DashboardView) View() string {
	if d.width == 0 {
		return "Loading..."
	}

	// Fetch latest data
	d.fetchDataSync()

	// Create layout
	statsBox := d.renderStats()
	providersBox := d.renderTopProviders()
	logsBox := d.renderRecentLogs()
	chartBox := d.renderActivityChart()

	// Two column layout
	leftCol := lipgloss.JoinVertical(
		lipgloss.Left,
		statsBox,
		chartBox,
	)

	rightCol := lipgloss.JoinVertical(
		lipgloss.Left,
		providersBox,
		logsBox,
	)

	colWidth := (d.width - 4) / 2
	leftColStyle := lipgloss.NewStyle().Width(colWidth)
	rightColStyle := lipgloss.NewStyle().Width(colWidth)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColStyle.Render(leftCol),
		rightColStyle.Render(rightCol),
	)
}

// SetSize sets the view size
func (d *DashboardView) SetSize(width, height int) {
	d.width = width
	d.height = height
}

func (d *DashboardView) fetchData() tea.Cmd {
	return func() tea.Msg {
		// This would fetch data asynchronously
		return nil
	}
}

func (d *DashboardView) fetchDataSync() {
	// Count providers
	var providers []models.Provider
	d.db.Find(&providers)
	d.totalProviders = len(providers)

	activeCount := 0
	for _, p := range providers {
		if p.IsAvailable() {
			activeCount++
		}
	}
	d.activeProviders = activeCount

	// Get stats
	var stats []models.ProviderStats
	d.db.Order("timestamp desc").Limit(100).Find(&stats)

	if len(stats) > 0 {
		var totalReq int64
		var successCount int64
		var totalLatency int64
		var totalCost float64

		for _, s := range stats {
			totalReq += s.TotalRequests
			successCount += int64(float64(s.TotalRequests) * s.SuccessRate)
			totalLatency += int64(s.AvgLatencyMs * int(s.TotalRequests))
			totalCost += s.CostSaved
		}

		d.totalRequests = totalReq
		if totalReq > 0 {
			d.successRate = float64(successCount) / float64(totalReq)
			d.avgLatency = int(totalLatency / totalReq)
		}
		d.costSaved = totalCost
	}

	// Get top providers
	d.topProviders = make([]ProviderHealth, 0, 5)
	for i := 0; i < len(providers) && i < 5; i++ {
		p := providers[i]
		status := "healthy"
		if p.HealthScore < 0.7 {
			status = "degraded"
		}
		if p.HealthScore < 0.5 {
			status = "down"
		}

		d.topProviders = append(d.topProviders, ProviderHealth{
			Name:        p.Name,
			HealthScore: p.HealthScore,
			Latency:     p.AvgLatencyMs,
			Status:      status,
		})
	}

	// Get recent logs (mock for now)
	d.recentLogs = []LogEntry{
		{
			Timestamp: time.Now().Add(-time.Second * 5),
			Level:     "INFO",
			Message:   "Request routed successfully",
			Provider:  "Groq",
		},
		{
			Timestamp: time.Now().Add(-time.Second * 12),
			Level:     "INFO",
			Message:   "Request routed successfully",
			Provider:  "OpenRouter",
		},
		{
			Timestamp: time.Now().Add(-time.Second * 23),
			Level:     "WARN",
			Message:   "Fallback: quota exhausted",
			Provider:  "Cerebras",
		},
		{
			Timestamp: time.Now().Add(-time.Second * 45),
			Level:     "INFO",
			Message:   "Request routed successfully",
			Provider:  "Google AI Studio",
		},
	}

	d.uptime = time.Hour*2 + time.Minute*34
}

func (d *DashboardView) renderStats() string {
	title := d.titleStyle.Render("SYSTEM STATISTICS")

	stats := []string{
		d.formatStat("Total Providers", fmt.Sprintf("%d", d.totalProviders)),
		d.formatStat("Active Providers", fmt.Sprintf("%d", d.activeProviders)),
		d.formatStat("Total Requests", fmt.Sprintf("%d", d.totalRequests)),
		d.formatStat("Success Rate", fmt.Sprintf("%.1f%%", d.successRate*100)),
		d.formatStat("Avg Latency", fmt.Sprintf("%dms", d.avgLatency)),
		d.formatStat("Cost Saved", fmt.Sprintf("$%.2f", d.costSaved)),
		d.formatStat("Uptime", d.formatDuration(d.uptime)),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(stats, "\n"))

	return d.boxStyle.Render(content)
}

func (d *DashboardView) renderTopProviders() string {
	title := d.titleStyle.Render("TOP PROVIDERS")

	var lines []string
	for _, p := range d.topProviders {
		// Progress bar
		barWidth := 20
		filled := int(p.HealthScore * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		// Status indicator
		statusIcon := "●"
		statusStyle := d.healthyStyle
		if p.Status == "degraded" {
			statusStyle = d.warningStyle
		} else if p.Status == "down" {
			statusStyle = d.errorStyle
		}

		// Format line
		line := fmt.Sprintf("%s %-20s %s %3.0f%% %4dms %s",
			statusStyle.Render(statusIcon),
			p.Name,
			bar,
			p.HealthScore*100,
			p.Latency,
			statusStyle.Render(p.Status),
		)
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(lines, "\n"))

	return d.boxStyle.Render(content)
}

func (d *DashboardView) renderRecentLogs() string {
	title := d.titleStyle.Render("RECENT ACTIVITY")

	var lines []string
	for _, log := range d.recentLogs {
		timestamp := log.Timestamp.Format("15:04:05")

		levelStyle := d.healthyStyle
		levelIcon := "✓"
		if log.Level == "WARN" {
			levelStyle = d.warningStyle
			levelIcon = "⚠"
		} else if log.Level == "ERROR" {
			levelStyle = d.errorStyle
			levelIcon = "✗"
		}

		line := fmt.Sprintf("[%s] %s %s - %s",
			d.statLabelStyle.Render(timestamp),
			levelStyle.Render(levelIcon),
			log.Provider,
			log.Message,
		)
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(lines, "\n"))

	return d.boxStyle.Render(content)
}

func (d *DashboardView) renderActivityChart() string {
	title := d.titleStyle.Render("REQUEST ACTIVITY (ASCII)")

	// Simple ASCII chart
	data := []int{45, 52, 48, 60, 55, 58, 62, 59, 65, 70, 68, 72}
	maxVal := 80
	chartHeight := 8

	var lines []string
	for i := chartHeight; i > 0; i-- {
		threshold := (maxVal * i) / chartHeight
		var line strings.Builder
		for _, val := range data {
			if val >= threshold {
				line.WriteString(d.healthyStyle.Render("█"))
			} else {
				line.WriteString(d.statLabelStyle.Render("░"))
			}
			line.WriteString(" ")
		}
		lines = append(lines, line.String())
	}

	// X-axis labels
	labels := "0h  2h  4h  6h  8h 10h 12h 14h 16h 18h 20h 22h"
	lines = append(lines, d.statLabelStyle.Render(labels))

	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(lines, "\n"))

	return d.boxStyle.Render(content)
}

func (d *DashboardView) formatStat(label, value string) string {
	return fmt.Sprintf("%s: %s",
		d.statLabelStyle.Render(label),
		d.statValueStyle.Render(value),
	)
}

func (d *DashboardView) formatDuration(dur time.Duration) string {
	hours := int(dur.Hours())
	minutes := int(dur.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
