package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	matchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	arrowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	conflictStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	countStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

const (
	fieldPattern = iota
	fieldReplacement
)

type tuiModel struct {
	inputs    [2]textinput.Model
	focused   int
	dir       string
	recursive bool
	ops       []renameOp
	conflicts []conflictError
	parseErr  error
	height    int
	executed  bool
	quit      bool
}

func newTUIModel(dir string, recursive bool) tuiModel {
	pattern := textinput.New()
	pattern.Placeholder = "RE2 pattern"
	pattern.Focus()
	pattern.Width = 40
	pattern.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	pattern.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	replacement := textinput.New()
	replacement.Placeholder = "replacement ($1, $2, ...)"
	replacement.Width = 40
	replacement.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	replacement.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	return tuiModel{
		inputs:    [2]textinput.Model{pattern, replacement},
		focused:   fieldPattern,
		dir:       dir,
		recursive: recursive,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) refreshPreview() tuiModel {
	pat := m.inputs[fieldPattern].Value()
	if pat == "" {
		m.ops = nil
		m.conflicts = nil
		m.parseErr = nil
		return m
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		m.ops = nil
		m.conflicts = nil
		m.parseErr = err
		return m
	}
	repl := normalizeReplacement(m.inputs[fieldReplacement].Value())
	ops, err := collectRenames(m.dir, re, repl, m.recursive)
	if err != nil {
		m.ops = nil
		m.conflicts = nil
		m.parseErr = err
		return m
	}
	m.ops = ops
	m.conflicts = validateRenames(ops)
	m.parseErr = nil
	return m
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quit = true
			return m, tea.Quit

		case tea.KeyTab, tea.KeyShiftTab:
			m.inputs[m.focused].Blur()
			m.inputs[m.focused].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			m.focused = (m.focused + 1) % 2
			m.inputs[m.focused].Focus()
			m.inputs[m.focused].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))

		case tea.KeyEnter:
			if len(m.ops) > 0 && len(m.conflicts) == 0 {
				executeRenames(m.ops)
				m.executed = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	m = m.refreshPreview()
	return m, cmd
}

func (m tuiModel) View() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s%s\n", labelStyle.Render("Pattern:     "), m.inputs[fieldPattern].View())
	fmt.Fprintf(&sb, "%s%s\n", labelStyle.Render("Replacement: "), m.inputs[fieldReplacement].View())
	sb.WriteString("\n")

	switch {
	case m.parseErr != nil:
		fmt.Fprintf(&sb, "%s\n", errorStyle.Render("Error: "+m.parseErr.Error()))

	case m.inputs[fieldPattern].Value() == "":
		fmt.Fprintf(&sb, "%s\n", hintStyle.Render("Enter a pattern"))

	case len(m.ops) == 0:
		fmt.Fprintf(&sb, "%s\n", hintStyle.Render("No files matched"))

	default:
		maxRows := 20
		if m.height > 0 {
			maxRows = m.height - 9
			if maxRows < 3 {
				maxRows = 3
			}
		}

		conflictPaths := make(map[string]string, len(m.conflicts))
		for _, c := range m.conflicts {
			conflictPaths[c.op.oldPath] = c.reason
		}

		header := fmt.Sprintf("Preview (%d files):", len(m.ops))
		if len(m.conflicts) > 0 {
			header += conflictStyle.Render(fmt.Sprintf("  ⚠ %d conflict(s)", len(m.conflicts)))
		}
		fmt.Fprintf(&sb, "%s\n", countStyle.Render(header))

		arrow := arrowStyle.Render(" → ")
		for i, op := range m.ops {
			if i >= maxRows {
				fmt.Fprintf(&sb, "%s\n", hintStyle.Render(fmt.Sprintf("  ... and %d more", len(m.ops)-maxRows)))
				break
			}
			if reason, bad := conflictPaths[op.oldPath]; bad {
				fmt.Fprintf(&sb, "  %s%s%s\n", op.oldPath+arrow, conflictStyle.Render(op.newPath), conflictStyle.Render("  ("+reason+")"))
			} else {
				fmt.Fprintf(&sb, "  %s%s%s\n", op.oldPath, arrow, matchStyle.Render(op.newPath))
			}
		}
	}

	sb.WriteString("\n")
	hints := []string{"[Tab] Switch field", "[Ctrl+C/Esc] Quit"}
	if len(m.ops) > 0 && len(m.conflicts) == 0 {
		hints = append([]string{"[Enter] Execute"}, hints...)
	}
	sb.WriteString(hintStyle.Render(strings.Join(hints, "  ")))

	return sb.String()
}

func runInteractive(dir string, recursive bool) {
	p := tea.NewProgram(newTUIModel(dir, recursive), tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m, ok := result.(tuiModel)
	if !ok || !m.executed {
		fmt.Println("Aborted.")
		return
	}
	for _, op := range m.ops {
		fmt.Printf("Renamed: %s → %s\n", op.oldPath, op.newPath)
	}
}
