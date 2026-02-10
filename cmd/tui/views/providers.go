package views

import (
	"fmt"
	"strings"

	"github.com/biodoia/framegotui/pkg/components"
	"github.com/biodoia/goleapifree/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gorm.io/gorm"
)

// ProvidersView displays and manages providers
type ProvidersView struct {
	db     *gorm.DB
	width  int
	height int

	// Data
	providers []models.Provider
	filtered  []models.Provider

	// UI Components
	table        *components.Table
	filterStatus string
	filterTier   int
	selectedIdx  int
	showDetails  bool

	// Styles
	boxStyle     lipgloss.Style
	titleStyle   lipgloss.Style
	labelStyle   lipgloss.Style
	valueStyle   lipgloss.Style
	activeStyle  lipgloss.Style
	inactiveStyle lipgloss.Style
}

// NewProvidersView creates a new providers view
func NewProvidersView(db *gorm.DB) *ProvidersView {
	v := &ProvidersView{
		db:           db,
		filterStatus: "all",
		filterTier:   0,
		boxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF10F0")).
			Padding(1, 2),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Bold(true),
		labelStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")),
		valueStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")),
		activeStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")),
		inactiveStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")),
	}

	v.initTable()
	return v
}

// Init initializes the view
func (v *ProvidersView) Init() tea.Cmd {
	v.loadProviders()
	return nil
}

// Update handles messages
func (v *ProvidersView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.selectedIdx > 0 {
				v.selectedIdx--
			}
		case "down", "j":
			if v.selectedIdx < len(v.filtered)-1 {
				v.selectedIdx++
			}
		case "enter":
			v.showDetails = !v.showDetails
		case "f":
			// Cycle filter status
			v.cycleFilter()
			v.applyFilters()
		case "t":
			// Test connection
			if v.selectedIdx < len(v.filtered) {
				// TODO: Implement test connection
			}
		case "r":
			// Refresh
			v.loadProviders()
		}
	}

	// Update table
	if v.table != nil {
		_, cmd := v.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

// View renders the view
func (v *ProvidersView) View() string {
	if v.width == 0 {
		return "Loading..."
	}

	// Reload data
	v.loadProviders()

	if v.showDetails && v.selectedIdx < len(v.filtered) {
		return v.renderDetails()
	}

	return v.renderList()
}

// SetSize sets the view size
func (v *ProvidersView) SetSize(width, height int) {
	v.width = width
	v.height = height
	if v.table != nil {
		v.table.SetSize(width-4, height-10)
	}
}

func (v *ProvidersView) initTable() {
	columns := []components.TableColumn{
		{Title: "Name", Width: 25, Align: lipgloss.Left},
		{Title: "Type", Width: 12, Align: lipgloss.Center},
		{Title: "Status", Width: 12, Align: lipgloss.Center},
		{Title: "Health", Width: 8, Align: lipgloss.Right},
		{Title: "Latency", Width: 10, Align: lipgloss.Right},
		{Title: "Tier", Width: 6, Align: lipgloss.Center},
		{Title: "Models", Width: 8, Align: lipgloss.Right},
	}

	v.table = components.NewTable(
		columns,
		components.WithTableHeight(20),
		components.WithStriped(true),
	)
}

func (v *ProvidersView) loadProviders() {
	v.db.Preload("Models").Find(&v.providers)
	v.applyFilters()
	v.updateTable()
}

func (v *ProvidersView) applyFilters() {
	v.filtered = make([]models.Provider, 0)

	for _, p := range v.providers {
		// Filter by status
		if v.filterStatus != "all" {
			if v.filterStatus == "active" && p.Status != models.ProviderStatusActive {
				continue
			}
			if v.filterStatus == "inactive" && p.Status == models.ProviderStatusActive {
				continue
			}
		}

		// Filter by tier
		if v.filterTier > 0 && p.Tier != v.filterTier {
			continue
		}

		v.filtered = append(v.filtered, p)
	}
}

func (v *ProvidersView) updateTable() {
	rows := make([]components.TableRow, 0, len(v.filtered))

	for _, p := range v.filtered {
		statusIcon := "●"
		if p.Status == models.ProviderStatusActive {
			statusIcon = v.activeStyle.Render("●")
		} else {
			statusIcon = v.inactiveStyle.Render("●")
		}

		health := fmt.Sprintf("%.0f%%", p.HealthScore*100)
		latency := fmt.Sprintf("%dms", p.AvgLatencyMs)
		tier := fmt.Sprintf("T%d", p.Tier)
		modelCount := fmt.Sprintf("%d", len(p.Models))

		row := components.TableRow{
			p.Name,
			string(p.Type),
			statusIcon + " " + string(p.Status),
			health,
			latency,
			tier,
			modelCount,
		}
		rows = append(rows, row)
	}

	v.table.SetRows(rows)
}

func (v *ProvidersView) cycleFilter() {
	switch v.filterStatus {
	case "all":
		v.filterStatus = "active"
	case "active":
		v.filterStatus = "inactive"
	case "inactive":
		v.filterStatus = "all"
	}
}

func (v *ProvidersView) renderList() string {
	title := v.titleStyle.Render(fmt.Sprintf(
		"PROVIDERS (%d total, %d filtered) - Filter: %s",
		len(v.providers),
		len(v.filtered),
		v.filterStatus,
	))

	help := v.labelStyle.Render(
		"[↑↓] Navigate  [Enter] Details  [f] Filter  [t] Test  [r] Refresh  [ESC] Back",
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		v.table.View(),
		"",
		help,
	)

	return v.boxStyle.
		Width(v.width - 4).
		Height(v.height - 2).
		Render(content)
}

func (v *ProvidersView) renderDetails() string {
	if v.selectedIdx >= len(v.filtered) {
		return "No provider selected"
	}

	p := v.filtered[v.selectedIdx]

	title := v.titleStyle.Render(fmt.Sprintf("PROVIDER DETAILS: %s", p.Name))

	// Basic info
	info := []string{
		v.formatField("ID", p.ID.String()),
		v.formatField("Name", p.Name),
		v.formatField("Type", string(p.Type)),
		v.formatField("Status", string(p.Status)),
		v.formatField("Base URL", p.BaseURL),
		v.formatField("Auth Type", string(p.AuthType)),
		v.formatField("Tier", fmt.Sprintf("%d", p.Tier)),
		"",
		v.titleStyle.Render("Health Metrics"),
		v.formatField("Health Score", fmt.Sprintf("%.2f", p.HealthScore)),
		v.formatField("Avg Latency", fmt.Sprintf("%dms", p.AvgLatencyMs)),
		v.formatField("Last Check", p.LastHealthCheck.Format("2006-01-02 15:04:05")),
		"",
		v.titleStyle.Render("Capabilities"),
		v.formatField("Streaming", v.formatBool(p.SupportsStreaming)),
		v.formatField("Tools", v.formatBool(p.SupportsTools)),
		v.formatField("JSON Mode", v.formatBool(p.SupportsJSON)),
		"",
		v.titleStyle.Render("Models"),
		v.formatField("Total Models", fmt.Sprintf("%d", len(p.Models))),
	}

	// Show first few models
	for i, m := range p.Models {
		if i >= 5 {
			info = append(info, v.labelStyle.Render("  ... and more"))
			break
		}
		info = append(info, v.labelStyle.Render(fmt.Sprintf("  • %s", m.Name)))
	}

	help := v.labelStyle.Render("[Enter] Back  [t] Test Connection  [d] Disable  [e] Edit")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		strings.Join(info, "\n"),
		"",
		help,
	)

	return v.boxStyle.
		Width(v.width - 4).
		Height(v.height - 2).
		Render(content)
}

func (v *ProvidersView) formatField(label, value string) string {
	return fmt.Sprintf("%s: %s",
		v.labelStyle.Render(label),
		v.valueStyle.Render(value),
	)
}

func (v *ProvidersView) formatBool(b bool) string {
	if b {
		return v.activeStyle.Render("✓ Yes")
	}
	return v.inactiveStyle.Render("✗ No")
}
