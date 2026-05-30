package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func runWatch(d *DB) {
	app := tview.NewApplication()

	// Header
	header := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Todos panel
	todosView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	todosView.SetBorder(true).SetTitle(" Tasks ")

	// Sessions panel
	sessionsView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	sessionsView.SetBorder(true).SetTitle(" Recent Sessions ")

	// Stats bar
	statsView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	refresh := func() {
		project := d.GetMeta("project")
		header.SetText(fmt.Sprintf("[yellow]aictx[white] — [cyan]%s[white]  [gray]%s", project, time.Now().Format("15:04:05")))

		// todos
		todos, _ := d.Todos("")
		var tb strings.Builder
		statuses := []string{"in_progress", "open", "blocked", "done"}
		labels := map[string]string{
			"in_progress": "[yellow]● IN PROGRESS[white]",
			"open":        "[green]○ OPEN[white]",
			"blocked":     "[red]✖ BLOCKED[white]",
			"done":        "[gray]✔ DONE[white]",
		}
		for _, st := range statuses {
			var items []*Todo
			for _, t := range todos {
				if t.Status == st {
					items = append(items, t)
				}
			}
			if len(items) == 0 {
				continue
			}
			tb.WriteString(fmt.Sprintf("\n %s\n", labels[st]))
			for _, t := range items {
				age := time.Since(time.Unix(t.UpdatedAt, 0)).Round(time.Minute)
				tb.WriteString(fmt.Sprintf("  [white][#%d][white] %s [gray](%v ago)[white]\n", t.ID, t.Text, age))
			}
		}
		todosView.SetText(tb.String())

		// sessions
		sessions, _ := d.Sessions(8)
		var sb strings.Builder
		for _, s := range sessions {
			ts := time.Unix(s.CreatedAt, 0).Format("01/02 15:04")
			sb.WriteString(fmt.Sprintf(" [cyan][%s][white] [[yellow]%s[white]]\n  %s\n\n", ts, s.AITool, s.Summary))
		}
		sessionsView.SetText(sb.String())

		// stats
		st := d.Stats()
		statsView.SetText(fmt.Sprintf(
			"  [green]open:%d[white]  [yellow]active:%d[white]  [red]blocked:%d[white]  [gray]done:%d[white]  |  sessions:%d  decisions:%d  |  [blue]q=quit  r=refresh[white]",
			st.Open, st.InProgress, st.Blocked, st.Done, st.Sessions, st.Decisions,
		))
	}

	refresh()

	// Layout
	mainRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(todosView, 0, 2, false).
		AddItem(sessionsView, 0, 1, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(mainRow, 0, 1, false).
		AddItem(statsView, 1, 0, false)

	// Auto-refresh every 5s
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			app.QueueUpdateDraw(refresh)
		}
	}()

	// Keyboard
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q', 'Q':
			app.Stop()
		case 'r', 'R':
			app.QueueUpdateDraw(refresh)
		}
		return event
	})

	// Graceful exit on SIGINT
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		app.Stop()
	}()

	if err := app.SetRoot(root, true).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
