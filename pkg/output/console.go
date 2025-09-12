package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	isatty "github.com/mattn/go-isatty"
)

var (
	styleArrow    = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)  // cyan/blue
	styleSection  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)  // bright white
	styleDesc     = lipgloss.NewStyle().Faint(true)                                  // dim
	styleWarnLbl  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true) // yellow
	styleWarnTxt  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))            // yellow
	styleNote     = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Faint(true) // teal dim
	styleEmitting = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)  // green
	styleZero     = lipgloss.NewStyle().Faint(true)
	colorEnabled  = true
)

// InitConsole configures color output based on noColor flag and TTY detection
func InitConsole(noColor bool) {
	tty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	colorEnabled = tty && !noColor
}

func r(st lipgloss.Style, s string) string {
	if !colorEnabled {
		return s
	}
	return st.Render(s)
}

// SectionHeader returns a colored header for a processed section and its optional description.
func SectionHeader(name, description string) string {
	var b strings.Builder
	arrow := r(styleArrow, "→")
	b.WriteString(fmt.Sprintf("%s %s\n", arrow, r(styleSection, name)))
	if strings.TrimSpace(description) != "" {
		b.WriteString(r(styleDesc, "  "+description))
		b.WriteByte('\n')
	}
	return b.String()
}

// Warnf returns a single-line colored warning string with a standard prefix.
func Warnf(format string, a ...interface{}) string {
	msg := fmt.Sprintf(format, a...)
	return r(styleWarnLbl, "Warning:") + " " + r(styleWarnTxt, msg)
}

// Notef returns a faint informational line (used for fetched counts, etc.)
func Notef(format string, a ...interface{}) string {
	return r(styleNote, fmt.Sprintf(format, a...))
}

// EmittingCount returns a colored summary for how many variables are emitted.
func EmittingCount(n int) string {
	if n <= 0 {
		return r(styleZero, "  Emitting 0 variable(s)")
	}
	return r(styleEmitting, fmt.Sprintf("  Emitting %d variable(s)", n))
}

// ListNames returns a bullet list of names, faint.
func ListNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	for _, n := range names {
		b.WriteString(r(styleDesc, "    - "))
		b.WriteString(r(styleDesc, n))
		b.WriteByte('\n')
	}
	return b.String()
}

// ShortError attempts to condense a verbose multi-line error into a short reason.
func ShortError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	lines := strings.Split(s, "\n")
	var candidate string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		// Skip known verbose prefixes from Vault HTTP layer
		if strings.HasPrefix(t, "URL:") || strings.HasPrefix(t, "Code:") || strings.HasPrefix(t, "Errors:") || t == "* 1 error occurred:" {
			continue
		}
		candidate = t
	}
	if strings.Contains(strings.ToLower(candidate), "permission denied") {
		return "permission denied"
	}
	if candidate == "" && len(lines) > 0 {
		candidate = strings.TrimSpace(lines[0])
	}
	return candidate
}
