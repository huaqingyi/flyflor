package cliui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// RenderCommandHelp builds Ruff-style sectioned, two-column help when
// UseFancyLayout(); otherwise plain Cobra-style text.
func RenderCommandHelp(c *cobra.Command) string {
	if !c.HasParent() {
		return RenderRootHelp(c.CommandPath(), c.UseLine())
	}
	if !UseFancyLayout() {
		return plainCommandHelp(c)
	}
	syncFlags(c)

	var b strings.Builder
	head, sub := helpIntro(c)

	inner := InnerWidth()
	contentW := inner - 6
	if contentW < 36 {
		contentW = 36
	}

	b.WriteString(commandHeaderPanel(c, head, sub, inner))
	b.WriteString("\n")

	usageBody := bodyStyle().MaxWidth(contentW).Render(styleUsageTokens(c.UseLine()))
	b.WriteString(sectionPanel("Usage", usageBody, inner, accentCyan))
	b.WriteString("\n")

	if ex := strings.TrimSpace(c.Example); ex != "" {
		exBody := bodyStyle().Width(contentW).Render(normalizeExampleBlock(ex))
		b.WriteString(sectionPanel("Examples", exBody, inner, accentGold))
		b.WriteString("\n")
	}

	subs := visibleSubcommands(c)
	if len(subs) > 0 {
		if c.HasParent() {
			rows := make([][2]string, 0, len(subs))
			for _, sub := range subs {
				rows = append(rows, commandHelpRow(sub))
			}
			b.WriteString(sectionPanel("Commands", renderTwoColPairs(rows, contentW), inner, accentViolet))
		} else {
			b.WriteString(sectionPanel("Commands", renderCommandGroups(subs, contentW), inner, accentViolet))
		}
		b.WriteString("\n")
	}

	local := c.LocalFlags()
	opts := collectFlagRows(local)
	if len(opts) > 0 {
		title := "Options"
		if !c.HasParent() {
			title = "Flags"
		}
		b.WriteString(sectionPanel(title, renderTwoColPairs(opts, contentW), inner, accentPink))
		b.WriteString("\n")
	}

	if c.HasAvailableInheritedFlags() {
		inh := collectFlagRows(c.InheritedFlags())
		if len(inh) > 0 {
			b.WriteString(sectionPanel("Global options", renderTwoColPairs(inh, contentW), inner, accentBlue))
			b.WriteString("\n")
		}
	}

	b.WriteString(RenderAgentStatusBar("cli:help", "ready"))
	return b.String()
}

// RenderCommandQuickRef prints the same Usage / Flags / Global sections as help,
// for embedding after errors (stderr). outerW is typically InnerStderrWidth().
func RenderCommandQuickRef(c *cobra.Command, outerW int) string {
	if c == nil || outerW < 40 {
		return ""
	}
	syncFlags(c)
	contentW := outerW - 6
	if contentW < 36 {
		contentW = 36
	}
	var b strings.Builder
	usageBody := bodyStyle().MaxWidth(contentW).Render(styleUsageTokens(c.UseLine()))
	b.WriteString(sectionPanel("Usage", usageBody, outerW, accentCyan))
	b.WriteString("\n")
	if len(c.Aliases) > 0 {
		al := "Aliases: " + strings.Join(c.Aliases, ", ")
		alBody := mutedStyle().MaxWidth(contentW).Render(al)
		b.WriteString(sectionPanel("Aliases", alBody, outerW, accentViolet))
		b.WriteString("\n")
	}
	opts := collectFlagRows(c.LocalFlags())
	if len(opts) > 0 {
		title := "Options"
		if !c.HasParent() {
			title = "Flags"
		}
		b.WriteString(sectionPanel(title, renderTwoColPairs(opts, contentW), outerW, accentPink))
		b.WriteString("\n")
	}
	if c.HasAvailableInheritedFlags() {
		inh := collectFlagRows(c.InheritedFlags())
		if len(inh) > 0 {
			b.WriteString(sectionPanel("Global options", renderTwoColPairs(inh, contentW), outerW, accentBlue))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func syncFlags(c *cobra.Command) {
	_ = c.LocalFlags()
	if c.HasAvailableInheritedFlags() {
		_ = c.InheritedFlags()
	}
}

func plainCommandHelp(c *cobra.Command) string {
	desc := c.Long
	if desc == "" {
		desc = c.Short
	}
	desc = strings.TrimRight(desc, " \t\n\r")
	var b strings.Builder
	if desc != "" {
		fmt.Fprintln(&b, desc)
		fmt.Fprintln(&b)
	}
	if c.Runnable() || c.HasSubCommands() {
		b.WriteString(c.UsageString())
	}
	return b.String()
}

func helpIntro(c *cobra.Command) (head, sub string) {
	head = strings.TrimSpace(c.Short)
	long := strings.TrimSpace(c.Long)
	if long == "" || long == head {
		return head, ""
	}
	lines := strings.Split(long, "\n")
	var rest []string
	for i, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if i == 0 && ln == head {
			continue
		}
		rest = append(rest, ln)
	}
	sub = strings.Join(rest, "\n")
	return head, sub
}

func visibleSubcommands(c *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, sub := range c.Commands() {
		if sub.Hidden {
			continue
		}
		out = append(out, sub)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

func commandHeaderPanel(c *cobra.Command, head, sub string, width int) string {
	if head == "" {
		head = c.CommandPath()
	}
	var rows []string
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		panelTitleStyle(accentCyan).Render("Flyflor CLI"),
		mutedStyle().Render("  /  "),
		panelTitleStyle(accentPink).Render(c.CommandPath()),
	))
	rows = append(rows, "")
	rows = append(rows, helpIntroStyle().Render(head))
	if sub != "" {
		rows = append(rows, mutedStyle().Render(sub))
	}
	if len(c.Aliases) > 0 {
		rows = append(rows, kvKeyStyle().Render("Aliases")+"  "+mutedStyle().Render(strings.Join(c.Aliases, ", ")))
	}
	rows = append(rows, kvKeyStyle().Render("Scope")+"    "+commandScope(c))
	return neonPanel(width, strings.Join(rows, "\n"))
}

func commandScope(c *cobra.Command) string {
	if !c.HasParent() {
		return mutedStyle().Render("agent runtime · gateway · memory · integrations")
	}
	if c.Runnable() {
		return mutedStyle().Render("runnable command")
	}
	return mutedStyle().Render("command group")
}

func normalizeExampleBlock(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

func sectionPanel(title, body string, width int, color lipgloss.Color) string {
	head := panelTitleStyle(color).Render(title) + "\n\n"
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1).
		Width(width).
		Render(head + body)
}

// styleUsageTokens highlights Flyflor command tokens and <placeholders>/[groups].
func styleUsageTokens(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		ia := strings.Index(s, "<")
		ib := strings.Index(s, "[")
		next, kind := -1, 0 // 1 = angle, 2 = bracket
		switch {
		case ia >= 0 && (ib < 0 || ia < ib):
			next, kind = ia, 1
		case ib >= 0:
			next, kind = ib, 2
		}
		if next < 0 {
			b.WriteString(helpIdentStyle().Render(s))
			break
		}
		if next > 0 {
			b.WriteString(helpIdentStyle().Render(s[:next]))
		}
		s = s[next:]
		if kind == 1 {
			j := strings.Index(s, ">")
			if j < 0 {
				b.WriteString(helpIdentStyle().Render(s))
				break
			}
			b.WriteString(helpPlaceholderStyle().Render(s[:j+1]))
			s = s[j+1:]
			continue
		}
		j := strings.Index(s, "]")
		if j < 0 {
			b.WriteString(helpIdentStyle().Render(s))
			break
		}
		b.WriteString(helpPlaceholderStyle().Render(s[:j+1]))
		s = s[j+1:]
	}
	return b.String()
}

func collectFlagRows(fs *flag.FlagSet) [][2]string {
	var names []string
	seen := map[string][2]string{}
	fs.VisitAll(func(f *flag.Flag) {
		if f.Hidden {
			return
		}
		left := formatFlagLeft(f)
		right := f.Usage
		if f.Deprecated != "" {
			right += " (deprecated: " + f.Deprecated + ")"
		}
		names = append(names, f.Name)
		seen[f.Name] = [2]string{left, right}
	})
	sort.Strings(names)
	rows := make([][2]string, 0, len(names))
	for _, n := range names {
		rows = append(rows, seen[n])
	}
	return rows
}

func formatFlagLeft(f *flag.Flag) string {
	suffix := ""
	if f.Value != nil {
		typ := f.Value.Type()
		if typ != "" && typ != "bool" {
			suffix = " <" + typ + ">"
		}
	}
	if len(f.Shorthand) > 0 {
		return "-" + f.Shorthand + ", --" + f.Name + suffix
	}
	return "--" + f.Name + suffix
}

func commandHelpRow(sub *cobra.Command) [2]string {
	left := sub.Name()
	if a := sub.Aliases; len(a) > 0 {
		left += " (" + strings.Join(a, ", ") + ")"
	}
	return [2]string{left, sub.Short}
}

func renderCommandGroups(subs []*cobra.Command, contentW int) string {
	groups := []struct {
		title string
		color lipgloss.Color
		names map[string]bool
	}{
		{"Conversation", accentCyan, map[string]bool{"agent": true, "model": true}},
		{"Runtime", accentLime, map[string]bool{"gateway": true, "status": true, "cron": true}},
		{"Integrations", accentViolet, map[string]bool{"auth": true, "mcp": true, "skills": true}},
		{"System", accentGold, map[string]bool{"migrate": true, "update": true, "version": true}},
	}
	var chunks []string
	seen := map[string]bool{}
	for _, group := range groups {
		var rows [][2]string
		for _, sub := range subs {
			if group.names[sub.Name()] {
				rows = append(rows, commandHelpRow(sub))
				seen[sub.Name()] = true
			}
		}
		if len(rows) == 0 {
			continue
		}
		title := panelTitleStyle(group.color).Render(group.title)
		chunks = append(chunks, title+"\n"+renderTwoColPairs(rows, contentW))
	}

	var otherRows [][2]string
	for _, sub := range subs {
		if !seen[sub.Name()] {
			otherRows = append(otherRows, commandHelpRow(sub))
		}
	}
	if len(otherRows) > 0 {
		chunks = append(chunks, panelTitleStyle(accentPink).Render("Other")+"\n"+renderTwoColPairs(otherRows, contentW))
	}
	return strings.Join(chunks, "\n\n")
}

func renderTwoColPairs(rows [][2]string, contentW int) string {
	if len(rows) == 0 {
		return ""
	}
	leftW := 0
	for _, r := range rows {
		if w := lipgloss.Width(r[0]); w > leftW {
			leftW = w
		}
	}
	const minLeft, maxLeft = 16, 34
	if leftW < minLeft {
		leftW = minLeft
	}
	if leftW > maxLeft {
		leftW = maxLeft
	}
	gap := "  "
	rightW := contentW - leftW - lipgloss.Width(gap)
	if rightW < 24 {
		rightW = 24
	}

	var b strings.Builder
	for _, r := range rows {
		left := helpIdentStyle().Width(leftW).Align(lipgloss.Left).Render(r[0])
		right := bodyStyle().Width(rightW).Render(strings.TrimSpace(r[1]))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
