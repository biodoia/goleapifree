package views

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/biodoia/goleapifree/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gorm.io/gorm"
)

// StatsView displays statistics and analytics
type StatsView struct {
	db     *gorm.DB
	width  int
	height int

	// Data
	stats            []models.ProviderStats
	totalRequests    int64
	successfulReqs   int64
	failedReqs       int64
	avgLatency       float64
	totalCostSaved   float64
	requestsByHour   []int
	successRateByDay []float64

	// Time range
	timeRange string // "1h", "24h", "7d", "30d"

	// Styles
	boxStyle       lipgloss.Style
	titleStyle     lipgloss.Style
	labelStyle     lipgloss.Style
	valueStyle     lipgloss.Style
	positiveStyle  lipgloss.Style
	negativeStyle  lipgloss.Style
	neutralStyle   lipgloss.Style
}

// NewStatsView creates a new stats view
func NewStatsView(db *gorm.DB) *StatsView {
	return &StatsView{
		db:        db,
		timeRange: "24h",
		boxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF10F0")).
			Padding(1, 2).
			MarginBottom(1),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true).
			MarginBottom(1),
		labelStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")),
		valueStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true),
		positiveStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true),
		negativeStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true),
		neutralStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")).
			Bold(true),
	}
}

// Init initializes the view
func (s *StatsView) Init() tea.Cmd {
	s.loadStats()
	return nil
}

// Update handles messages
func (s *StatsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			s.timeRange = "1h"
			s.loadStats()
		case "2":
			s.timeRange = "24h"
			s.loadStats()
		case "3":
			s.timeRange = "7d"
			s.loadStats()
		case "4":
			s.timeRange = "30d"
			s.loadStats()
		case "r":
			s.loadStats()
		}
	}
	return s, nil
}

// View renders the view
func (s *StatsView) View() string {
	if s.width == 0 {
		return "Loading..."
	}

	// Reload data
	s.loadStats()

	overview := s.renderOverview()
	charts := s.renderCharts()
	savings := s.renderCostSavings()
	distribution := s.renderDistribution()

	leftCol := lipgloss.JoinVertical(
		lipgloss.Left,
		overview,
		charts,
	)

	rightCol := lipgloss.JoinVertical(
		lipgloss.Left,
		savings,
		distribution,
	)

	colWidth := (s.width - 4) / 2
	leftColStyle := lipgloss.NewStyle().Width(colWidth)
	rightColStyle := lipgloss.NewStyle().Width(colWidth)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColStyle.Render(leftCol),
		rightColStyle.Render(rightCol),
	)
}

// SetSize sets the view size
func (s *StatsView) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *StatsView) loadStats() {
	// Calculate time range
	var since time.Time
	switch s.timeRange {
	case "1h":
		since = time.Now().Add(-time.Hour)
	case "24h":
		since = time.Now().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().Add(-30 * 24 * time.Hour)
	}

	// Load stats from database
	s.db.Where("timestamp >= ?", since).Order("timestamp asc").Find(&s.stats)

	// Calculate aggregates
	s.totalRequests = 0
	s.successfulReqs = 0
	s.failedReqs = 0
	var totalLatencyWeighted float64
	s.totalCostSaved = 0

	for _, stat := range s.stats {
		s.totalRequests += stat.TotalRequests
		successCount := int64(float64(stat.TotalRequests) * stat.SuccessRate)
		s.successfulReqs += successCount
		s.failedReqs += stat.TotalRequests - successCount
		totalLatencyWeighted += float64(stat.AvgLatencyMs) * float64(stat.TotalRequests)
		s.totalCostSaved += stat.CostSaved
	}

	if s.totalRequests > 0 {
		s.avgLatency = totalLatencyWeighted / float64(s.totalRequests)
	}

	// Generate request distribution by hour (mock data for now)
	s.requestsByHour = make([]int, 24)
	for i := range s.requestsByHour {
		s.requestsByHour[i] = 50 + (i*3 % 30)
	}

	// Generate success rate by day (mock data)
	s.successRateByDay = []float64{0.98, 0.97, 0.99, 0.96, 0.98, 0.99, 0.97}
}

func (s *StatsView) renderOverview() string {
	title := s.titleStyle.Render(fmt.Sprintf("OVERVIEW (Last %s)", s.timeRange))

	successRate := 0.0
	if s.totalRequests > 0 {
		successRate = float64(s.successfulReqs) / float64(s.totalRequests) * 100
	}

	stats := []string{
		s.formatStat("Total Requests", fmt.Sprintf("%d", s.totalRequests)),
		s.formatStat("Successful", fmt.Sprintf("%d (%.1f%%)", s.successfulReqs, successRate)),
		s.formatStat("Failed", fmt.Sprintf("%d", s.failedReqs)),
		s.formatStat("Avg Latency", fmt.Sprintf("%.0fms", s.avgLatency)),
		s.formatStat("Cost Saved", fmt.Sprintf("$%.2f", s.totalCostSaved)),
	}

	help := s.labelStyle.Render("[1] 1h  [2] 24h  [3] 7d  [4] 30d  [r] Refresh")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(stats, "\n"),
		"",
		help,
	)

	return s.boxStyle.Render(content)
}

func (s *StatsView) renderCharts() string {
	title := s.titleStyle.Render("REQUEST VOLUME (24h)")

	// ASCII bar chart
	chart := s.renderBarChart(s.requestsByHour, 10)

	// Success rate trend
	trendTitle := s.titleStyle.Render("SUCCESS RATE TREND (7d)")
	trendChart := s.renderLineChart(s.successRateByDay, 6)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		chart,
		"",
		trendTitle,
		trendChart,
	)

	return s.boxStyle.Render(content)
}

func (s *StatsView) renderCostSavings() string {
	title := s.titleStyle.Render("COST SAVINGS CALCULATOR")

	// Estimate what these requests would cost with OpenAI
	estimatedOpenAICost := float64(s.totalRequests) * 0.015 // $0.015 per request (rough estimate)
	actualCost := 0.0 // Assuming free/freemium providers
	savedAmount := estimatedOpenAICost - actualCost
	savingsPercent := 100.0

	if estimatedOpenAICost > 0 {
		savingsPercent = (savedAmount / estimatedOpenAICost) * 100
	}

	info := []string{
		s.formatStat("Estimated OpenAI Cost", fmt.Sprintf("$%.2f", estimatedOpenAICost)),
		s.formatStat("Actual Cost", fmt.Sprintf("$%.2f", actualCost)),
		s.formatStat("Amount Saved", s.positiveStyle.Render(fmt.Sprintf("$%.2f", savedAmount))),
		s.formatStat("Savings Percent", s.positiveStyle.Render(fmt.Sprintf("%.0f%%", savingsPercent))),
		"",
		s.labelStyle.Render("Monthly Projection:"),
		s.formatStat("  Estimated Cost", fmt.Sprintf("$%.2f", estimatedOpenAICost*30)),
		s.formatStat("  Total Savings", s.positiveStyle.Render(fmt.Sprintf("$%.2f", savedAmount*30))),
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(info, "\n"),
	)

	return s.boxStyle.Render(content)
}

func (s *StatsView) renderDistribution() string {
	title := s.titleStyle.Render("REQUEST DISTRIBUTION BY PROVIDER")

	// Get provider stats
	var providerStats []struct {
		ProviderName string
		RequestCount int64
		SuccessRate  float64
	}

	s.db.Raw(`
		SELECT p.name as provider_name,
		       SUM(ps.total_requests) as request_count,
		       AVG(ps.success_rate) as success_rate
		FROM provider_stats ps
		JOIN providers p ON p.id = ps.provider_id
		WHERE ps.timestamp >= ?
		GROUP BY p.name
		ORDER BY request_count DESC
		LIMIT 10
	`, time.Now().Add(-24*time.Hour)).Scan(&providerStats)

	var lines []string
	totalReqs := int64(0)
	for _, ps := range providerStats {
		totalReqs += ps.RequestCount
	}

	for _, ps := range providerStats {
		percentage := 0.0
		if totalReqs > 0 {
			percentage = float64(ps.RequestCount) / float64(totalReqs) * 100
		}

		// Progress bar
		barWidth := 20
		filled := int(percentage / 5) // Scale to 20 chars
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		line := fmt.Sprintf("%-20s %s %5.1f%% (%d reqs)",
			ps.ProviderName,
			bar,
			percentage,
			ps.RequestCount,
		)
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		lines = append(lines, s.labelStyle.Render("No data available"))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(lines, "\n"),
	)

	return s.boxStyle.Render(content)
}

func (s *StatsView) renderBarChart(data []int, height int) string {
	if len(data) == 0 {
		return "No data"
	}

	// Find max value
	maxVal := 0
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal == 0 {
		maxVal = 1
	}

	var lines []string
	for i := height; i > 0; i-- {
		threshold := (maxVal * i) / height
		var line strings.Builder
		for _, val := range data {
			if val >= threshold {
				line.WriteString(s.positiveStyle.Render("█"))
			} else {
				line.WriteString(s.labelStyle.Render("░"))
			}
		}
		lines = append(lines, line.String())
	}

	// Add time labels
	labels := "00 02 04 06 08 10 12 14 16 18 20 22"
	lines = append(lines, s.labelStyle.Render(labels))

	return strings.Join(lines, "\n")
}

func (s *StatsView) renderLineChart(data []float64, height int) string {
	if len(data) == 0 {
		return "No data"
	}

	width := len(data) * 3
	chart := make([][]rune, height)
	for i := range chart {
		chart[i] = make([]rune, width)
		for j := range chart[i] {
			chart[i][j] = ' '
		}
	}

	// Plot points
	for i, val := range data {
		// Scale value to chart height (assuming 0.0-1.0 range for success rate)
		y := int((1.0 - val) * float64(height-1))
		if y < 0 {
			y = 0
		}
		if y >= height {
			y = height - 1
		}
		x := i * 3

		chart[y][x] = '●'

		// Connect with previous point
		if i > 0 {
			prevY := int((1.0 - data[i-1]) * float64(height-1))
			if prevY < 0 {
				prevY = 0
			}
			if prevY >= height {
				prevY = height - 1
			}

			// Draw line
			minY := int(math.Min(float64(y), float64(prevY)))
			maxY := int(math.Max(float64(y), float64(prevY)))
			for ly := minY; ly <= maxY; ly++ {
				if x-1 >= 0 && x-1 < width {
					chart[ly][x-1] = '─'
				}
			}
		}
	}

	// Convert to strings
	var lines []string
	for _, row := range chart {
		line := string(row)
		lines = append(lines, s.positiveStyle.Render(line))
	}

	// Add labels
	dayLabels := "Mon  Tue  Wed  Thu  Fri  Sat  Sun"
	lines = append(lines, s.labelStyle.Render(dayLabels))

	return strings.Join(lines, "\n")
}

func (s *StatsView) formatStat(label, value string) string {
	return fmt.Sprintf("%s: %s",
		s.labelStyle.Render(label),
		s.valueStyle.Render(value),
	)
}
