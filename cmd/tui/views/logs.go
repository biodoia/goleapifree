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

// LogsView displays live logs
type LogsView struct {
	db     *gorm.DB
	width  int
	height int

	// Data
	logs          []models.RequestLog
	filtered      []models.RequestLog
	maxLogs       int
	autoScroll    bool
	scrollOffset  int
	selectedIdx   int

	// Filters
	filterLevel    string // "all", "success", "error", "warn"
	filterProvider string
	searchText     string

	// Styles
	boxStyle      lipgloss.Style
	titleStyle    lipgloss.Style
	timestampStyle lipgloss.Style
	successStyle  lipgloss.Style
	errorStyle    lipgloss.Style
	warnStyle     lipgloss.Style
	infoStyle     lipgloss.Style
	labelStyle    lipgloss.Style
}

// NewLogsView creates a new logs view
func NewLogsView(db *gorm.DB) *LogsView {
	return &LogsView{
		db:             db,
		maxLogs:        100,
		autoScroll:     true,
		filterLevel:    "all",
		filterProvider: "all",
		boxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF10F0")).
			Padding(1, 2),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true).
			MarginBottom(1),
		timestampStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")),
		successStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")),
		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true),
		warnStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")),
		infoStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")),
		labelStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")),
	}
}

// Init initializes the view
func (l *LogsView) Init() tea.Cmd {
	l.loadLogs()
	return l.tickCmd()
}

// Update handles messages
func (l *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if l.scrollOffset > 0 {
				l.scrollOffset--
				l.autoScroll = false
			}
		case "down", "j":
			maxScroll := len(l.filtered) - (l.height - 10)
			if maxScroll < 0 {
				maxScroll = 0
			}
			if l.scrollOffset < maxScroll {
				l.scrollOffset++
			}
		case "home":
			l.scrollOffset = 0
			l.autoScroll = false
		case "end":
			l.scrollOffset = len(l.filtered)
			l.autoScroll = true
		case "pgup":
			l.scrollOffset -= l.height - 10
			if l.scrollOffset < 0 {
				l.scrollOffset = 0
			}
			l.autoScroll = false
		case "pgdown":
			l.scrollOffset += l.height - 10
			maxScroll := len(l.filtered) - (l.height - 10)
			if l.scrollOffset > maxScroll {
				l.scrollOffset = maxScroll
			}
		case "a":
			l.autoScroll = !l.autoScroll
		case "f":
			l.cycleFilter()
			l.applyFilters()
		case "c":
			// Clear logs
			l.logs = nil
			l.filtered = nil
		case "r":
			l.loadLogs()
		}

	case TickMsg:
		// Auto-refresh logs
		l.loadLogs()
		cmds = append(cmds, l.tickCmd())
	}

	return l, tea.Batch(cmds...)
}

// View renders the view
func (l *LogsView) View() string {
	if l.width == 0 {
		return "Loading..."
	}

	return l.renderLogs()
}

// SetSize sets the view size
func (l *LogsView) SetSize(width, height int) {
	l.width = width
	l.height = height
}

func (l *LogsView) tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (l *LogsView) loadLogs() {
	// Load recent logs
	l.db.Order("timestamp desc").
		Limit(l.maxLogs).
		Find(&l.logs)

	l.applyFilters()

	// Auto-scroll to bottom if enabled
	if l.autoScroll {
		l.scrollOffset = len(l.filtered) - (l.height - 10)
		if l.scrollOffset < 0 {
			l.scrollOffset = 0
		}
	}
}

func (l *LogsView) applyFilters() {
	l.filtered = make([]models.RequestLog, 0)

	for _, log := range l.logs {
		// Filter by level
		if l.filterLevel != "all" {
			if l.filterLevel == "success" && !log.Success {
				continue
			}
			if l.filterLevel == "error" && log.Success {
				continue
			}
		}

		// Filter by provider
		if l.filterProvider != "all" && log.ProviderID.String() != l.filterProvider {
			// For now, skip provider filtering as we need to join
			// In a real implementation, we'd preload the provider
		}

		l.filtered = append(l.filtered, log)
	}
}

func (l *LogsView) cycleFilter() {
	switch l.filterLevel {
	case "all":
		l.filterLevel = "success"
	case "success":
		l.filterLevel = "error"
	case "error":
		l.filterLevel = "all"
	}
}

func (l *LogsView) renderLogs() string {
	title := l.titleStyle.Render(fmt.Sprintf(
		"LIVE LOGS (Last %d, showing %d) - Filter: %s %s",
		len(l.logs),
		len(l.filtered),
		l.filterLevel,
		l.getAutoScrollIndicator(),
	))

	// Calculate visible range
	visibleHeight := l.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := l.scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(l.filtered) {
		end = len(l.filtered)
	}

	// Render log lines
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, l.formatLog(l.filtered[i]))
	}

	if len(lines) == 0 {
		lines = append(lines, l.labelStyle.Render("No logs to display"))
	}

	// Scroll indicator
	scrollInfo := ""
	if len(l.filtered) > visibleHeight {
		scrollInfo = l.labelStyle.Render(fmt.Sprintf(
			"[Showing %d-%d of %d]",
			start+1,
			end,
			len(l.filtered),
		))
	}

	help := l.labelStyle.Render(
		"[↑↓] Scroll  [Home/End] Top/Bottom  [a] Auto-scroll  [f] Filter  [c] Clear  [r] Refresh",
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		strings.Join(lines, "\n"),
		"",
		scrollInfo,
		help,
	)

	return l.boxStyle.
		Width(l.width - 4).
		Height(l.height - 2).
		Render(content)
}

func (l *LogsView) formatLog(log models.RequestLog) string {
	timestamp := l.timestampStyle.Render(log.Timestamp.Format("15:04:05.000"))

	// Status icon and style
	var statusIcon string
	var statusStyle lipgloss.Style
	if log.Success {
		statusIcon = "✓"
		statusStyle = l.successStyle
	} else {
		statusIcon = "✗"
		statusStyle = l.errorStyle
	}

	// Status code styling
	statusCode := fmt.Sprintf("%d", log.StatusCode)
	if log.StatusCode >= 200 && log.StatusCode < 300 {
		statusCode = l.successStyle.Render(statusCode)
	} else if log.StatusCode >= 400 {
		statusCode = l.errorStyle.Render(statusCode)
	} else {
		statusCode = l.warnStyle.Render(statusCode)
	}

	// Latency with color coding
	latency := fmt.Sprintf("%3dms", log.LatencyMs)
	if log.LatencyMs < 100 {
		latency = l.successStyle.Render(latency)
	} else if log.LatencyMs < 500 {
		latency = l.warnStyle.Render(latency)
	} else {
		latency = l.errorStyle.Render(latency)
	}

	// Method
	method := log.Method
	if method == "" {
		method = "POST"
	}

	// Endpoint
	endpoint := log.Endpoint
	if len(endpoint) > 30 {
		endpoint = endpoint[:27] + "..."
	}

	// Tokens
	tokens := ""
	if log.InputTokens > 0 || log.OutputTokens > 0 {
		tokens = l.labelStyle.Render(fmt.Sprintf("in:%d out:%d", log.InputTokens, log.OutputTokens))
	}

	// Error message
	errorMsg := ""
	if !log.Success && log.ErrorMessage != "" {
		errText := log.ErrorMessage
		if len(errText) > 50 {
			errText = errText[:47] + "..."
		}
		errorMsg = l.errorStyle.Render(" [" + errText + "]")
	}

	return fmt.Sprintf("%s %s %s %s %-6s %-30s %s %s%s",
		timestamp,
		statusStyle.Render(statusIcon),
		statusCode,
		latency,
		method,
		endpoint,
		tokens,
		l.labelStyle.Render(log.ProviderID.String()[:8]),
		errorMsg,
	)
}

func (l *LogsView) getAutoScrollIndicator() string {
	if l.autoScroll {
		return l.successStyle.Render("[AUTO]")
	}
	return l.labelStyle.Render("[PAUSED]")
}

// LogLevel represents log severity
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// ColorizeLogLevel returns a styled log level
func (l *LogsView) ColorizeLogLevel(level LogLevel, text string) string {
	switch level {
	case LogLevelDebug:
		return l.labelStyle.Render(text)
	case LogLevelInfo:
		return l.infoStyle.Render(text)
	case LogLevelWarn:
		return l.warnStyle.Render(text)
	case LogLevelError:
		return l.errorStyle.Render(text)
	default:
		return text
	}
}

// FormatBytes formats bytes with units
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a duration in human readable format
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
