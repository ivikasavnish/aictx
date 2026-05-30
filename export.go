package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func exportMarkdown(d *DB) error {
	sessions, _ := d.Sessions(5)
	todos, _ := d.Todos("")
	decisions, _ := d.Decisions()
	project := d.GetMeta("project")
	url := d.GetMeta("url")

	var b strings.Builder
	b.WriteString("# AI Context\n\n")
	b.WriteString(fmt.Sprintf("**Project:** %s  \n", project))
	if url != "" {
		b.WriteString(fmt.Sprintf("**URL:** %s  \n", url))
	}
	b.WriteString(fmt.Sprintf("**Updated:** %s\n\n", time.Now().Format("2006-01-02 15:04")))

	b.WriteString("## In Progress\n\n")
	hasActive := false
	for _, t := range todos {
		if t.Status == "in_progress" {
			b.WriteString(fmt.Sprintf("- [ ] %s\n", t.Text))
			hasActive = true
		}
	}
	if !hasActive {
		b.WriteString("_none_\n")
	}

	b.WriteString("\n## Open Tasks\n\n")
	hasOpen := false
	for _, t := range todos {
		if t.Status == "open" {
			b.WriteString(fmt.Sprintf("- [ ] [#%d] %s\n", t.ID, t.Text))
			hasOpen = true
		}
	}
	if !hasOpen {
		b.WriteString("_none_\n")
	}

	b.WriteString("\n## Blocked\n\n")
	for _, t := range todos {
		if t.Status == "blocked" {
			b.WriteString(fmt.Sprintf("- ⚠ [#%d] %s\n", t.ID, t.Text))
		}
	}

	b.WriteString("\n## Key Decisions\n\n")
	for _, dec := range decisions {
		b.WriteString(fmt.Sprintf("- **%s**", dec.Text))
		if dec.Context != "" {
			b.WriteString(fmt.Sprintf(" — %s", dec.Context))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n## Recent Sessions\n\n")
	for _, s := range sessions {
		t := time.Unix(s.CreatedAt, 0).Format("2006-01-02 15:04")
		b.WriteString(fmt.Sprintf("**[%s] %s:** %s\n\n", t, s.AITool, s.Summary))
	}

	out := filepath.Join(d.dir, "current.md")
	return os.WriteFile(out, []byte(b.String()), 0644)
}

func printResume(d *DB) {
	sessions, _ := d.Sessions(3)
	todos, _ := d.Todos("")
	decisions, _ := d.Decisions()
	project := d.GetMeta("project")

	fmt.Printf("=== AI CONTEXT: %s ===\n\n", project)

	fmt.Println("LAST SESSIONS:")
	for _, s := range sessions {
		t := time.Unix(s.CreatedAt, 0).Format("Jan 02 15:04")
		fmt.Printf("  [%s][%s] %s\n", t, s.AITool, s.Summary)
	}

	fmt.Println("\nIN PROGRESS:")
	for _, t := range todos {
		if t.Status == "in_progress" {
			fmt.Printf("  >> %s\n", t.Text)
		}
	}

	fmt.Println("\nOPEN TASKS:")
	for _, t := range todos {
		if t.Status == "open" {
			fmt.Printf("  [#%d] %s\n", t.ID, t.Text)
		}
	}

	blocked := false
	for _, t := range todos {
		if t.Status == "blocked" {
			if !blocked {
				fmt.Println("\nBLOCKED:")
				blocked = true
			}
			fmt.Printf("  !! [#%d] %s\n", t.ID, t.Text)
		}
	}

	if len(decisions) > 0 {
		fmt.Println("\nKEY DECISIONS:")
		for _, dec := range decisions[:min(3, len(decisions))] {
			fmt.Printf("  * %s\n", dec.Text)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
