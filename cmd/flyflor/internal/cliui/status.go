package cliui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ProviderRow holds one provider's display name and status value.
type ProviderRow struct {
	Name string
	Val  string
}

// StatusReport is a structured status view for PrintStatus.
type StatusReport struct {
	Logo          string
	Version       string
	Build         string
	ConfigPath    string
	ConfigOK      bool
	WorkspacePath string
	WorkspaceOK   bool
	Model         string
	Providers     []ProviderRow
	OAuthLines    []string // each full line "provider (method): state"
}

// PrintStatus renders Flyflor status (plain or fancy).
func PrintStatus(r StatusReport) {
	if !UseFancyLayout() {
		printStatusPlain(r)
		return
	}
	printStatusFancy(r)
}

func printStatusPlain(r StatusReport) {
	fmt.Printf("%s Flyflor Status\n", r.Logo)
	fmt.Printf("Version: %s\n", r.Version)
	if r.Build != "" {
		fmt.Printf("Build: %s\n", r.Build)
	}
	fmt.Println()

	printPathLine("Config", r.ConfigPath, r.ConfigOK)
	printPathLine("Workspace", r.WorkspacePath, r.WorkspaceOK)

	if r.ConfigOK {
		fmt.Printf("Model: %s\n", r.Model)
		for _, p := range r.Providers {
			fmt.Printf("%s: %s\n", p.Name, p.Val)
		}
		if len(r.OAuthLines) > 0 {
			fmt.Println("\nOAuth/Token Auth:")
			for _, line := range r.OAuthLines {
				fmt.Printf("  %s\n", line)
			}
		}
	}
}

func printPathLine(label, path string, ok bool) {
	mark := "✗"
	if ok {
		mark = "✓"
	}
	fmt.Println(label+":", path, mark)
}

func printStatusFancy(r StatusReport) {
	inner := InnerWidth()

	var head strings.Builder
	head.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(accentCyan).Render(r.Logo+" Flyflor"),
		mutedStyle().Render("  /  "),
		panelTitleStyle(accentPink).Render("runtime status"),
	))
	head.WriteString("\n\n")
	head.WriteString(kvKeyStyle().Render("Version") + "  " + kvValStyle().Render(r.Version))
	if r.Build != "" {
		head.WriteString("\n")
		head.WriteString(kvKeyStyle().Render("Build") + "     " + kvValStyle().Render(r.Build))
	}
	if len(r.Providers) > 0 {
		head.WriteString("\n")
		head.WriteString(kvKeyStyle().Render("Providers") + " " + providerCoverage(r.Providers))
	}
	fmt.Println(neonPanel(inner, head.String()))
	fmt.Println()

	if UseColumnLayout() && len(r.Providers) > 0 && r.ConfigOK {
		leftW := (inner - 2) / 2
		rightW := inner - leftW - 2
		pathsNarrow := runtimeStatusPanel(r, leftW)
		prov := providerTablePanel(r, rightW)
		gap := strings.Repeat(" ", 2)
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, pathsNarrow, gap, prov))
	} else {
		fmt.Println(runtimeStatusPanel(r, inner))
		if len(r.Providers) > 0 && r.ConfigOK {
			fmt.Println(providerTablePanel(r, inner))
		}
	}

	if len(r.OAuthLines) > 0 && r.ConfigOK {
		var ob strings.Builder
		ob.WriteString(titleBarStyle().Render("OAuth / token auth") + "\n\n")
		for _, line := range r.OAuthLines {
			ob.WriteString("  • " + line + "\n")
		}
		fmt.Println()
		fmt.Println(borderStyle().Width(inner).Render(ob.String()))
	}
}

func runtimeStatusPanel(r StatusReport, inner int) string {
	cfgMark := statusMark(r.ConfigOK)
	wsMark := statusMark(r.WorkspaceOK)
	var b strings.Builder
	b.WriteString(panelTitleStyle(accentCyan).Render("Runtime") + "\n\n")
	b.WriteString(statusDot(statusColor(r.ConfigOK)) + " " + kvKeyStyle().Render("Config") + "\n")
	b.WriteString(mutedStyle().Render(r.ConfigPath))
	b.WriteString(" " + cfgMark + "\n\n")
	b.WriteString(statusDot(statusColor(r.WorkspaceOK)) + " " + kvKeyStyle().Render("Workspace") + "\n")
	b.WriteString(mutedStyle().Render(r.WorkspacePath))
	b.WriteString(" " + wsMark + "\n")
	if r.ConfigOK {
		b.WriteString("\n")
		b.WriteString(statusDot(accentPink) + " " + kvKeyStyle().Render("Model") + "  " + kvValStyle().Render(r.Model))
		b.WriteString("\n\n")
		b.WriteString(panelTitleStyle(accentGold).Render("Persistence") + "\n")
		b.WriteString(statusDot(accentBlue) + " config     " + mutedStyle().Render("/config") + "\n")
		b.WriteString(statusDot(accentViolet) + " workspace  " + mutedStyle().Render("/workspace") + "\n")
		b.WriteString(statusDot(accentLime) + " memory     " + mutedStyle().Render("sqlite + qdrant ready"))
		b.WriteString("\n\n")
		b.WriteString(panelTitleStyle(accentViolet).Render("Gateway") + "\n")
		b.WriteString(statusDot(accentCyan) + " health     " + mutedStyle().Render("/health + /ready") + "\n")
		b.WriteString(statusDot(accentOrange) + " channels   " + mutedStyle().Render("managed by gateway") + "\n")
		b.WriteString(statusDot(accentPink) + " reload     " + mutedStyle().Render("POST /reload"))
	}
	return borderStyle().Width(inner).Render(b.String())
}

func statusColor(ok bool) lipgloss.Color {
	if ok {
		return colorOK
	}
	return accentRed
}

func statusMark(ok bool) string {
	if ok {
		return lipgloss.NewStyle().Foreground(colorOK).Render("✓")
	}
	return lipgloss.NewStyle().Foreground(accentRed).Render("✗")
}

func providerTablePanel(r StatusReport, colW int) string {
	if len(r.Providers) == 0 {
		return ""
	}
	keyW := min(22, colW/3)
	if keyW < 14 {
		keyW = 14
	}
	valW := colW - keyW - 3
	if valW < 12 {
		valW = 12
	}

	var b strings.Builder
	b.WriteString(panelTitleStyle(accentViolet).Render("Providers & local") + "\n\n")
	for _, p := range r.Providers {
		k := lipgloss.NewStyle().Foreground(accentBlue).Bold(true).Width(keyW).Render(p.Name)
		v := styleProviderVal(p.Val).Width(valW - 2).Render(p.Val)
		mark := statusDot(providerColor(p.Val))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, mark, " ", k, "  ", v))
		b.WriteString("\n")
	}
	return borderStyle().Width(colW).Render(strings.TrimRight(b.String(), "\n"))
}

func styleProviderVal(s string) lipgloss.Style {
	if s == "✓" || strings.HasPrefix(s, "✓ ") {
		return lipgloss.NewStyle().Foreground(colorOK)
	}
	if s == "not set" {
		return mutedStyle()
	}
	return lipgloss.NewStyle()
}

func providerCoverage(rows []ProviderRow) string {
	if len(rows) == 0 {
		return mutedStyle().Render("none")
	}
	enabled := 0
	for _, row := range rows {
		if providerEnabled(row.Val) {
			enabled++
		}
	}
	percent := enabled * 100 / len(rows)
	return fmt.Sprintf("%s  %d/%d configured", progressBar(percent, 12), enabled, len(rows))
}

func providerColor(s string) lipgloss.Color {
	if providerEnabled(s) {
		return colorOK
	}
	return colorMuted
}

func providerEnabled(s string) bool {
	return s == "✓" || strings.HasPrefix(s, "✓ ")
}
