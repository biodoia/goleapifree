package main

import (
	"fmt"
	"os"
	"time"

	"github.com/biodoia/framegotui/pkg/components"
	"github.com/biodoia/framegotui/pkg/theme"
	"github.com/biodoia/goleapifree/cmd/tui/views"
	"github.com/biodoia/goleapifree/pkg/config"
	"github.com/biodoia/goleapifree/pkg/database"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	configPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "goleapai-tui",
		Short: "GoLeapAI TUI Dashboard",
		Long:  `Cyberpunk terminal interface per monitorare e gestire GoLeapAI Gateway`,
		RunE:  runTUI,
	}

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load config, using defaults")
		cfg = &config.Config{}
	}

	// Initialize database
	dbWrapper, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create main model
	m := NewMainModel(dbWrapper.DB)

	// Create Bubble Tea program
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)

	// Run program
	if _, err := p.Run(); err != nil {
		log.Fatal().Err(err).Msg("TUI failed")
		return err
	}

	return nil
}

// TickMsg is sent periodically for updates
type TickMsg time.Time

// MainModel è il modello principale della TUI
type MainModel struct {
	db         *gorm.DB
	theme      theme.Theme
	activeView int
	width      int
	height     int
	ready      bool

	// Views
	tabs           *components.Tabs
	dashboardView  *views.DashboardView
	providersView  *views.ProvidersView
	statsView      *views.StatsView
	logsView       *views.LogsView

	// Modal
	modal *components.Modal

	// Styles
	headerStyle lipgloss.Style
	footerStyle lipgloss.Style
}

// NewMainModel creates a new main model
func NewMainModel(db *gorm.DB) *MainModel {
	m := &MainModel{
		db:          db,
		theme:       theme.Cyberpunk(),
		activeView:  0,
		headerStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("#1A1A2E")).
			Foreground(lipgloss.Color("#FF10F0")).
			Bold(true).
			Padding(0, 2),
		footerStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("#1A1A2E")).
			Foreground(lipgloss.Color("#00FFFF")).
			Padding(0, 1),
	}

	// Initialize views
	m.dashboardView = views.NewDashboardView(db)
	m.providersView = views.NewProvidersView(db)
	m.statsView = views.NewStatsView(db)
	m.logsView = views.NewLogsView(db)

	// Initialize modal
	m.modal = components.NewModal("", "", components.ModalTypeInfo)

	return m
}

// Init inizializza il modello
func (m *MainModel) Init() tea.Cmd {
	return tea.Batch(
		m.tickCmd(),
		m.dashboardView.Init(),
		m.providersView.Init(),
		m.statsView.Init(),
		m.logsView.Init(),
	)
}

func (m *MainModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update gestisce i messaggi
func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle modal first if visible
	if m.modal != nil && m.modal.IsVisible() {
		_, cmd := m.modal.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		// Check for modal dismiss
		if dismissMsg, ok := msg.(components.ModalDismissMsg); ok {
			m.modal.Hide()
			// Handle modal result
			if dismissMsg.Result == components.ModalResultYes {
				// Handle confirmation actions
			}
		}

		// Don't process other messages when modal is visible
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update view sizes
		contentHeight := m.height - 4 // header + footer
		m.dashboardView.SetSize(m.width, contentHeight)
		m.providersView.SetSize(m.width, contentHeight)
		m.statsView.SetSize(m.width, contentHeight)
		m.logsView.SetSize(m.width, contentHeight)

		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "1":
			m.activeView = 0
		case "2":
			m.activeView = 1
		case "3":
			m.activeView = 2
		case "4":
			m.activeView = 3
		case "?", "h":
			// Show help modal
			m.showHelpModal()
			return m, nil
		}

	case TickMsg:
		// Periodic update
		cmds = append(cmds, m.tickCmd())

		// Update active view
		switch m.activeView {
		case 0:
			_, cmd := m.dashboardView.Update(msg)
			cmds = append(cmds, cmd)
		case 1:
			_, cmd := m.providersView.Update(msg)
			cmds = append(cmds, cmd)
		case 2:
			_, cmd := m.statsView.Update(msg)
			cmds = append(cmds, cmd)
		case 3:
			_, cmd := m.logsView.Update(msg)
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)
	}

	// Update active view
	switch m.activeView {
	case 0:
		_, cmd := m.dashboardView.Update(msg)
		cmds = append(cmds, cmd)
	case 1:
		_, cmd := m.providersView.Update(msg)
		cmds = append(cmds, cmd)
	case 2:
		_, cmd := m.statsView.Update(msg)
		cmds = append(cmds, cmd)
	case 3:
		_, cmd := m.logsView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renderizza la vista
func (m *MainModel) View() string {
	if !m.ready {
		spinner := components.NewSpinner(
			components.WithSpinnerFrames(components.SpinnerNeon),
			components.WithSpinnerMessage("Initializing GoLeapAI TUI..."),
		)
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			spinner.View(),
		)
	}

	// Render header
	header := m.renderHeader()

	// Render content based on active view
	var content string
	switch m.activeView {
	case 0:
		content = m.dashboardView.View()
	case 1:
		content = m.providersView.View()
	case 2:
		content = m.statsView.View()
	case 3:
		content = m.logsView.View()
	default:
		content = m.dashboardView.View()
	}

	// Render footer
	footer := m.renderFooter()

	// Combine all
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)

	// Overlay modal if visible
	if m.modal != nil && m.modal.IsVisible() {
		modalView := m.modal.View()
		// Center modal
		view = lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			modalView,
		)
	}

	return view
}

// renderHeader renders the header
func (m *MainModel) renderHeader() string {
	title := "▓▓▓ GoLeapAI Gateway - Cyberpunk Dashboard ▓▓▓"

	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1A1A2E")).
		Foreground(lipgloss.Color("#FF10F0")).
		Bold(true).
		Padding(0, 2).
		Width(m.width)

	return titleStyle.Render(title)
}

// renderFooter renders the footer with shortcuts
func (m *MainModel) renderFooter() string {
	shortcuts := []string{
		"[1] Dashboard",
		"[2] Providers",
		"[3] Stats",
		"[4] Logs",
		"[?] Help",
		"[q] Quit",
	}

	var parts []string
	for i, shortcut := range shortcuts {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#808080")).
			Padding(0, 1)

		if i == m.activeView {
			style = style.
				Foreground(lipgloss.Color("#00FFFF")).
				Bold(true)
		}

		parts = append(parts, style.Render(shortcut))
	}

	footer := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1A1A2E")).
		Width(m.width).
		Render(footer)
}

func (m *MainModel) showHelpModal() {
	helpText := `Navigation:
  1 - Dashboard view
  2 - Providers view
  3 - Statistics view
  4 - Logs view

  ? - This help
  q - Quit

  Arrow keys - Navigate
  Enter - Select
  ESC - Cancel/Back`

	m.modal.SetTitle("Help")
	m.modal.SetMessage(helpText)
	m.modal.Show()
}
