package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiState int

const (
	stateInput   tuiState = iota
	stateResults
	stateHelp
)

type clearFlashMsg struct{}

type shellDoneMsg struct{ err error }

type searchDoneMsg struct {
	results []ESHit
	total   int
	elapsed time.Duration
	err     error
}

type pageLoadedMsg struct {
	results []ESHit
	err     error
}

type model struct {
	channel    string
	altChannel string
	size       int
	state     tuiState
	prevState tuiState
	textInput textinput.Model
	results   []ESHit
	total     int
	elapsed   time.Duration
	cursor   int
	expanded int // index of expanded result, -1 if none
	scroll   int
	searching   bool
	loadingMore bool
	err         error
	flashMsg    string
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

func initialModel(channel, altChannel string, size int, initialQuery string) model {
	ti := textinput.New()
	ti.Placeholder = "search nixos packages..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60
	ti.SetValue(initialQuery)

	return model{
		channel:    channel,
		altChannel: altChannel,
		size:       size,
		state:      stateInput,
		expanded:   -1,
		textInput:  ti,
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

func (m model) loadNextPage() tea.Cmd {
	query := m.textInput.Value()
	channel := m.channel
	size := m.size
	from := len(m.results)
	return func() tea.Msg {
		resp, _, err := searchFrom(query, channel, size, from)
		if err != nil {
			return pageLoadedMsg{err: err}
		}
		return pageLoadedMsg{results: resp.Hits.Hits}
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
		m.loadingMore = false
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

	case pageLoadedMsg:
		m.loadingMore = false
		if msg.err == nil && len(msg.results) > 0 {
			m.results = append(m.results, msg.results...)
		}
		return m, nil

	case clearFlashMsg:
		m.flashMsg = ""
		return m, nil

	case shellDoneMsg:
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
	case "?":
		if m.state == stateHelp {
			m.state = m.prevState
			return m, nil
		}
		if m.state == stateResults {
			m.prevState = m.state
			m.state = stateHelp
			return m, nil
		}
	case "esc":
		if m.state == stateHelp {
			m.state = m.prevState
			return m, nil
		}
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
			// Load next page when near the end
			if m.cursor >= len(m.results)-3 && len(m.results) < m.total && !m.loadingMore {
				m.loadingMore = true
				return m, m.loadNextPage()
			}
		}
		return m, nil
	case "y":
		if m.state == stateResults && len(m.results) > 0 {
			name := m.results[m.cursor].Source.PackageAttrName
			clipboard.WriteAll(name)
			return m, m.flash("Copied: " + name)
		}
	case "e":
		if m.state == stateResults && len(m.results) > 0 {
			cmd := "nix-env -iA nixpkgs." + m.results[m.cursor].Source.PackageAttrName
			clipboard.WriteAll(cmd)
			return m, m.flash("Copied: " + cmd)
		}
	case "p":
		if m.state == stateResults && len(m.results) > 0 {
			cmd := "nix profile install nixpkgs#" + m.results[m.cursor].Source.PackageAttrName
			clipboard.WriteAll(cmd)
			return m, m.flash("Copied: " + cmd)
		}
	case "r":
		if m.state == stateResults && len(m.results) > 0 {
			name := m.results[m.cursor].Source.PackageAttrName
			c := exec.Command("nix-shell", "-p", name)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return shellDoneMsg{err}
			})
		}
	case "o":
		if m.state == stateResults && len(m.results) > 0 {
			hp := homepage(m.results[m.cursor].Source)
			if hp != "" {
				exec.Command("xdg-open", hp).Start()
				return m, m.flash("Opened: " + hp)
			}
		}
	case "c":
		if m.state == stateResults && m.altChannel != "" && !m.searching {
			m.channel, m.altChannel = m.altChannel, m.channel
			m.results = nil
			m.total = 0
			m.cursor = 0
			m.scroll = 0
			m.expanded = -1
			if m.textInput.Value() != "" {
				m.searching = true
				return m, tea.Batch(m.doSearch(), m.flash("Channel: "+m.channel))
			}
			return m, m.flash("Channel: " + m.channel)
		}
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

func (m *model) flash(msg string) tea.Cmd {
	m.flashMsg = msg
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return clearFlashMsg{}
	})
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
	// total height minus: status bar (1) + input (1) + separator (1) + info line (1) + bottom bar (1) + padding (1)
	h := m.height - 6
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
	if m.altChannel != "" {
		left += "  " + dimStyle.Render(m.altChannel+" [c]")
	}
	b.WriteString(statusStyle.Width(m.width).Render(left))
	b.WriteString("\n")

	if m.state == stateHelp {
		b.WriteString(m.viewHelp())
	} else {
		// Input
		b.WriteString(" > ")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")

		// Separator
		b.WriteString(dimStyle.Render(strings.Repeat("─", m.width)))
		b.WriteString("\n")

		b.WriteString(m.viewResults())
	}

	// Bottom bar
	hints := "?:help  c:ch  y:copy  r:shell  o:open  q:quit"
	b.WriteString(statusStyle.Width(m.width).Render(" " + hints))

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
	if m.flashMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render(m.flashMsg))
	} else if len(m.results) > 0 {
		footer := fmt.Sprintf("Showing %d of %d results (%dms)",
			len(m.results), m.total, m.elapsed.Milliseconds())
		if m.loadingMore {
			footer += "  loading more..."
		}
		b.WriteString(footerStyle.Render(footer))
	}

	return b.String()
}

func (m model) viewHelp() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	key := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))

	b.WriteString("\n")
	b.WriteString(title.Render("  Keybindings"))
	b.WriteString("\n\n")

	bindings := []struct{ k, desc string }{
		{"Enter", "Search (in input) / Toggle detail (in results)"},
		{"j / Down", "Move cursor down"},
		{"k / Up", "Move cursor up"},
		{"/ / Tab", "Focus search input"},
		{"c", "Switch channel"},
		{"y", "Copy package name"},
		{"e", "Copy nix-env install command"},
		{"p", "Copy nix profile install command"},
		{"r", "Open nix-shell with package"},
		{"o", "Open homepage in browser"},
		{"Esc", "Collapse detail / Clear input / Quit"},
		{"q", "Quit"},
		{"?", "Toggle this help"},
		{"Ctrl+C", "Quit immediately"},
	}

	for _, bind := range bindings {
		k := key.Render(fmt.Sprintf("  %-12s", bind.k))
		b.WriteString(fmt.Sprintf("%s %s\n", k, bind.desc))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Press ? or Esc to close"))
	b.WriteString("\n")

	// Pad remaining height: status(1) + bottom bar(1) already accounted for outside
	used := strings.Count(b.String(), "\n")
	total := m.height - 2
	for i := used; i < total; i++ {
		b.WriteString("\n")
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

func runTUI(channel, altChannel string, size int, initialQuery string) int {
	m := initialModel(channel, altChannel, size, initialQuery)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
