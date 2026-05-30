package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	cmd := os.Args[1]

	if cmd == "init" {
		cmdInit()
		return
	}

	d, err := openDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	switch cmd {
	case "save":
		cmdSave(d)
	case "resume":
		printResume(d)
	case "export":
		if err := exportMarkdown(d); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		dir, _ := dbDir()
		fmt.Println("exported →", filepath.Join(dir, "current.md"))
	case "todo":
		cmdTodo(d)
	case "decide":
		cmdDecide(d)
	case "log":
		cmdLog(d)
	case "watch":
		runWatch(d)
	case "set":
		cmdSet(d)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		usage()
		os.Exit(1)
	}
}

func cmdInit() {
	cwd, _ := os.Getwd()
	dir := filepath.Join(cwd, ".ai-context")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}
	d, err := openDBAt(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		os.Exit(1)
	}

	// Set project name from dir name
	project := filepath.Base(cwd)
	d.SetMeta("project", project)
	d.SetMeta("init_at", strconv.FormatInt(now(), 10))

	// Write .gitignore entry
	gi := filepath.Join(dir, ".gitignore")
	os.WriteFile(gi, []byte("context.db\n"), 0644)

	fmt.Printf("initialized .ai-context/ in %s\n", cwd)
	fmt.Printf("project: %s\n", project)
	fmt.Println("tip: git add .ai-context/current.md (track exported markdown)")
}

func cmdSave(d *DB) {
	var summary string
	if len(os.Args) >= 3 {
		summary = strings.Join(os.Args[2:], " ")
	} else {
		fmt.Print("session summary: ")
		reader := bufio.NewReader(os.Stdin)
		summary, _ = reader.ReadString('\n')
		summary = strings.TrimSpace(summary)
	}
	if summary == "" {
		fmt.Fprintln(os.Stderr, "summary required")
		os.Exit(1)
	}

	aiTool := os.Getenv("AI_TOOL")
	if aiTool == "" {
		aiTool = "claude"
	}
	project := d.GetMeta("project")
	url := d.GetMeta("url")

	s, err := d.SaveSession(project, url, summary, aiTool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}

	exportMarkdown(d)
	fmt.Printf("session #%d saved\n", s.ID)
}

func cmdTodo(d *DB) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: aictx todo <add|done|start|block|list|rm> [args]")
		os.Exit(1)
	}
	sub := os.Args[2]

	switch sub {
	case "add":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: aictx todo add <text> [priority]")
			os.Exit(1)
		}
		text := os.Args[3]
		priority := 0
		if len(os.Args) >= 5 {
			priority, _ = strconv.Atoi(os.Args[4])
		}
		t, err := d.AddTodo(text, priority)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		exportMarkdown(d)
		fmt.Printf("todo #%d added: %s\n", t.ID, t.Text)

	case "done":
		id := requireID()
		if err := d.SetTodoStatus(id, "done"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		exportMarkdown(d)
		fmt.Printf("todo #%d marked done\n", id)

	case "start":
		id := requireID()
		if err := d.SetTodoStatus(id, "in_progress"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		exportMarkdown(d)
		fmt.Printf("todo #%d in progress\n", id)

	case "block":
		id := requireID()
		if err := d.SetTodoStatus(id, "blocked"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		exportMarkdown(d)
		fmt.Printf("todo #%d blocked\n", id)

	case "open":
		id := requireID()
		if err := d.SetTodoStatus(id, "open"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		exportMarkdown(d)
		fmt.Printf("todo #%d reopened\n", id)

	case "list", "ls":
		filter := ""
		if len(os.Args) >= 4 {
			filter = os.Args[3]
		}
		todos, err := d.Todos(filter)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		statusIcon := map[string]string{
			"open":        "○",
			"in_progress": "●",
			"done":        "✔",
			"blocked":     "✖",
		}
		for _, t := range todos {
			icon := statusIcon[t.Status]
			if icon == "" {
				icon = "?"
			}
			fmt.Printf(" %s [#%d] %-12s %s\n", icon, t.ID, "("+t.Status+")", t.Text)
		}

	default:
		fmt.Fprintln(os.Stderr, "unknown todo subcommand:", sub)
		os.Exit(1)
	}
}

func cmdDecide(d *DB) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: aictx decide <text> [context]")
		os.Exit(1)
	}
	text := os.Args[2]
	ctx := ""
	if len(os.Args) >= 4 {
		ctx = strings.Join(os.Args[3:], " ")
	}
	if err := d.AddDecision(text, ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	exportMarkdown(d)
	fmt.Println("decision recorded:", text)
}

func cmdLog(d *DB) {
	limit := 10
	if len(os.Args) >= 3 {
		limit, _ = strconv.Atoi(os.Args[2])
	}
	sessions, err := d.Sessions(limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("last %d sessions:\n\n", len(sessions))
	for _, s := range sessions {
		t := tsFormat(s.CreatedAt)
		fmt.Printf("[%s] [%s] %s\n", t, s.AITool, s.Summary)
	}
}

func cmdSet(d *DB) {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: aictx set <key> <value>")
		os.Exit(1)
	}
	key, val := os.Args[2], strings.Join(os.Args[3:], " ")
	if err := d.SetMeta(key, val); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("set %s = %s\n", key, val)
}

func requireID() int64 {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "id required")
		os.Exit(1)
	}
	id, err := strconv.ParseInt(os.Args[3], 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid id:", os.Args[3])
		os.Exit(1)
	}
	return id
}

func tsFormat(unix int64) string {
	if unix == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", unix) // will improve with time package
}

func usage() {
	fmt.Print(`aictx — AI context tracker

USAGE:
  aictx init                       init in current directory
  aictx save [summary]             save session snapshot
  aictx resume                     print context for AI to read
  aictx export                     export to .ai-context/current.md

  aictx todo add <text> [priority] add task
  aictx todo start <id>            mark in progress
  aictx todo done <id>             mark done
  aictx todo block <id>            mark blocked
  aictx todo open <id>             reopen task
  aictx todo list [status]         list tasks

  aictx decide <text> [context]    record a key decision
  aictx log [n]                    show last n sessions (default 10)
  aictx set <key> <value>          set metadata (project, url, etc.)
  aictx watch                      live terminal dashboard

ENV:
  AI_TOOL   tag sessions by AI (default: claude)
`)
}
