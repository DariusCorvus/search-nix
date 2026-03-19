package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiState int

const (
	stateInput   tuiState = iota
	stateResults
)

type searchDoneMsg struct {
	results []ESHit
	total   int
	elapsed time.Duration
	err     error
}

type model struct {
	channel   string
	size      int
	state     tuiState
	textInput textinput.Model
	results   []ESHit
	total     int
	elapsed   time.Duration
	cursor   int
	expanded int // index of expanded result, -1 if none
	scroll   int
	searching bool
	err       error
	width     int
	height    int
}

// Styles
var (
	statusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	pkgNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true)

	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	numStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6"))

	programsLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("5"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

func initialModel(channel string, size int, initialQuery string) model {
	ti := textinput.New()
	ti.Placeholder = "search nixos packages..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60
	ti.SetValue(initialQuery)

	return model{
		channel:   channel,
		size:      size,
		state:     stateInput,
		expanded:  -1,
		textInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.textInput.Value() != "" {
		cmds = append(cmds, m.doSearch())
	}
	return tea.Batch(cmds...)
}

func (m model) doSearch() tea.Cmd {
	query := m.textInput.Value()
	channel := m.channel
	size := m.size
	return func() tea.Msg {
		resp, elapsed, err := search(query, channel, size)
		if err != nil {
			return searchDoneMsg{err: err}
		}
		return searchDoneMsg{
			results: resp.Hits.Hits,
			total:   resp.Hits.Total.Value,
			elapsed: elapsed,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = min(msg.Width-4, 80)
		return m, nil

	case searchDoneMsg:
		m.searching = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.results = msg.results
		m.total = msg.total
		m.elapsed = msg.elapsed
		m.cursor = 0
		m.scroll = 0
		if len(m.results) > 0 {
			m.state = stateResults
			m.expanded = 0
			m.textInput.Blur()
		} else {
			m.expanded = -1
		}
		return m, nil

	case tea.KeyMsg:
		return m.updateMain(msg)
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.expanded >= 0 {
			m.expanded = -1
			return m, nil
		}
		if m.textInput.Value() == "" {
			return m, tea.Quit
		}
		// Clear input and go back to input state
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.results = nil
		m.total = 0
		m.expanded = -1
		m.state = stateInput
		m.err = nil
		return m, nil
	case "q":
		if m.state == stateResults {
			return m, tea.Quit
		}
	case "enter":
		if m.state == stateResults && len(m.results) > 0 {
			// Toggle expanded detail inline
			if m.expanded == m.cursor {
				m.expanded = -1
			} else {
				m.expanded = m.cursor
			}
			m.ensureVisible()
			return m, nil
		}
		if m.textInput.Value() != "" {
			m.searching = true
			m.err = nil
			return m, m.doSearch()
		}
		return m, nil
	case "up", "k":
		if m.state == stateResults && len(m.results) > 0 {
			if m.cursor > 0 {
				m.cursor--
			}
			m.expanded = m.cursor
			m.ensureVisible()
		}
		return m, nil
	case "down", "j":
		if m.state == stateResults && len(m.results) > 0 {
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
			m.expanded = m.cursor
			m.ensureVisible()
		}
		return m, nil
	case "/", "tab":
		m.textInput.Focus()
		m.state = stateInput
		return m, textinput.Blink
	}

	// Only pass keys to text input when in input state
	if m.state == stateInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) ensureVisible() {
	rh := m.resultsHeight()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	// Scroll down until cursor item (plus its detail if expanded) fits
	for {
		used := m.linesFromTo(m.scroll, m.cursor)
		if used <= rh {
			break
		}
		m.scroll++
		if m.scroll > m.cursor {
			m.scroll = m.cursor
			break
		}
	}
}

// linesFromTo returns total lines used to render results from index `from` through `to` (inclusive).
func (m model) linesFromTo(from, to int) int {
	lines := 0
	for i := from; i <= to && i < len(m.results); i++ {
		if i > from {
			lines++ // separator
		}
		lines += 2 // summary: name+version, description
		if i == m.expanded {
			lines += m.detailLineCount(i)
		}
	}
	return lines
}

func (m model) resultsHeight() int {
	// total height minus: status bar (1) + input (1) + separator (1) + footer (1) + padding (1)
	h := m.height - 5
	if h < 3 {
		h = 3
	}
	return h
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Status bar
	left := fmt.Sprintf(" Channel: %s", m.channel)
	right := "q: quit  /: search  enter: expand"
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	statusLine := left + strings.Repeat(" ", gap) + right
	b.WriteString(statusStyle.Width(m.width).Render(statusLine))
	b.WriteString("\n")

	// Input
	b.WriteString(" > ")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	// Separator
	b.WriteString(dimStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	b.WriteString(m.viewResults())

	return b.String()
}

func (m model) viewResults() string {
	var b strings.Builder
	rh := m.resultsHeight()

	if m.searching {
		b.WriteString(dimStyle.Render("  Searching..."))
		b.WriteString("\n")
	} else if m.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n", m.err))
	} else if len(m.results) == 0 {
		if m.textInput.Value() != "" {
			b.WriteString(dimStyle.Render("  No results. Press Enter to search."))
			b.WriteString("\n")
		} else {
			b.WriteString(dimStyle.Render("  Type a query and press Enter to search."))
			b.WriteString("\n")
		}
	} else {
		linesUsed := 0
		for i := m.scroll; i < len(m.results); i++ {
			needed := 2
			isExpanded := i == m.expanded
			if isExpanded {
				needed += m.detailLineCount(i)
			}
			// Add 1 for separator between items (not before first visible)
			if i > m.scroll {
				needed++
			}
			if linesUsed+needed > rh {
				break
			}

			// Separator between items
			if i > m.scroll {
				b.WriteString(dimStyle.Render(strings.Repeat("─", m.width)))
				b.WriteString("\n")
				linesUsed++
			}

			p := m.results[i].Source
			num := i + 1
			selected := i == m.cursor

			numStr := numStyle.Render(fmt.Sprintf("[%d]", num))
			nameStr := pkgNameStyle.Render(p.PackageAttrName)
			verStr := versionStyle.Render(nvl(p.PackageVersion, "?"))
			descStr := nvl(p.PackageDescription, "-")

			exactMatch := hasExactProgramMatch(p, m.textInput.Value())
			matchTag := ""
			if exactMatch {
				matchTag = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("*")
			}

			line1 := fmt.Sprintf(" %s %s  %s%s", numStr, nameStr, verStr, matchTag)
			line2 := fmt.Sprintf("      %s", truncate(descStr, m.width-7))

			if selected {
				bg := "\033[48;5;236m"
				pad1 := m.width - lipgloss.Width(line1)
				if pad1 < 0 {
					pad1 = 0
				}
				pad2 := m.width - lipgloss.Width(line2)
				if pad2 < 0 {
					pad2 = 0
				}
				line1 = bg + line1 + strings.Repeat(" ", pad1) + reset
				line2 = bg + line2 + strings.Repeat(" ", pad2) + reset
			}

			b.WriteString(line1)
			b.WriteString("\n")
			b.WriteString(line2)
			b.WriteString("\n")
			linesUsed += 2

			if isExpanded {
				detail := m.renderInlineDetail(i)
				b.WriteString(detail)
				linesUsed += m.detailLineCount(i)
			}
		}
	}

	// Pad remaining space
	lines := strings.Count(b.String(), "\n")
	for i := lines; i < rh; i++ {
		b.WriteString("\n")
	}

	// Footer
	if len(m.results) > 0 {
		footer := fmt.Sprintf("Showing %d of %d results (%dms)",
			len(m.results), m.total, m.elapsed.Milliseconds())
		b.WriteString(footerStyle.Render(footer))
	}

	return b.String()
}

func (m model) renderInlineDetail(idx int) string {
	if idx < 0 || idx >= len(m.results) {
		return ""
	}
	p := m.results[idx].Source
	var b strings.Builder
	indent := "       "

	if len(p.PackagePrograms) > 0 {
		query := strings.ToLower(m.textInput.Value())
		var progs []string
		for _, prog := range p.PackagePrograms {
			if strings.EqualFold(prog, query) {
				progs = append(progs, lipgloss.NewStyle().Bold(true).Underline(true).Render(prog))
			} else {
				progs = append(progs, prog)
			}
		}
		b.WriteString(fmt.Sprintf("%s%s  %s\n",
			indent,
			programsLabelStyle.Render("programs"),
			strings.Join(progs, "  "),
		))
	}
	if hp := homepage(p); hp != "" {
		b.WriteString(fmt.Sprintf("%s%s      %s\n", indent, labelStyle.Render("home"), hp))
	}
	if lic := licenses(p); lic != "" {
		b.WriteString(fmt.Sprintf("%s%s   %s\n", indent, labelStyle.Render("license"), lic))
	}
	b.WriteString(fmt.Sprintf("%s%s\n",
		indent, dimStyle.Render("nix-env -iA nixpkgs."+p.PackageAttrName)))
	return b.String()
}

func (m model) detailLineCount(idx int) int {
	if idx < 0 || idx >= len(m.results) {
		return 0
	}
	p := m.results[idx].Source
	lines := 1 // install command always shown
	if len(p.PackagePrograms) > 0 {
		lines++
	}
	if homepage(p) != "" {
		lines++
	}
	if licenses(p) != "" {
		lines++
	}
	return lines
}

func hasExactProgramMatch(p SearchResult, query string) bool {
	if query == "" {
		return false
	}
	q := strings.ToLower(query)
	for _, prog := range p.PackagePrograms {
		if strings.ToLower(prog) == q {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func runTUI(channel string, size int, initialQuery string) int {
	m := initialModel(channel, size, initialQuery)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
