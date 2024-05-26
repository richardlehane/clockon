// To dos..
// - purge or compress logs
// - Reports: weekly, monthly, yearly? Active vs historical.
// Keep it working through changes!

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/snabb/isoweek"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state uint8

const (
	ready state = iota // textinput
	adding
	removing
	selecting
	working
	resting
	weekly
	yearly
	quitting
)

type model struct {
	log        *logger
	activities []string
	tally      [][2]time.Duration
	cursor     int
	selected   int
	stopwatch  stopwatch.Model
	textInput  textinput.Model
	keymap     keymap
	help       help.Model
	state      state
	statePrev  state
	bank       time.Duration
	week       [2]int
	weekNxt    [2]int
	weekPrev   [2]int
	year       int
	yearNxt    int
	yearPrev   int
	weekTbl    table.Model
	yearTbl    table.Model
}

type keymap struct {
	add    key.Binding
	change key.Binding
	delete key.Binding
	work   key.Binding
	rest   key.Binding
	stop   key.Binding
	week   key.Binding
	year   key.Binding
	next   key.Binding
	prev   key.Binding
	quit   key.Binding
}

func (m model) Init() tea.Cmd {
	return nil
}

var style = lipgloss.NewStyle().Foreground(lipgloss.Color("#3C3C3C"))

var hstyle = lipgloss.NewStyle().Bold(true)

var tableStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func (m model) View() string {
	switch m.state {
	case quitting:
		return "bye!"
	case adding:
		var suffix string
		if len(m.activities) == 0 {
			suffix = `Enter an activity e.g. "Top Secret Project" or (ctl-c) to quit`
		} else {
			suffix = "(esc) to cancel"
		}
		return fmt.Sprintf(
			"Add a new activity:\n%s\n\n%s",
			m.textInput.View(),
			style.Render(suffix),
		)
	case ready:
		return fmt.Sprintf("Hit 'w' to start working on %s\n%s\n%s", m.activities[m.selected], m.statusView(), m.helpView())
	case working:
		return fmt.Sprintf("Working for %s on %s\n%s\n%s", m.stopwatch.Elapsed(), m.activities[m.selected], m.statusView(), m.helpView())
	case resting:
		return fmt.Sprintf("Breaking for %s from %s\n%s\n%s", m.stopwatch.Elapsed(), m.activities[m.selected], m.statusView(), m.helpView())
	case selecting, removing:
		// Iterate over our choices
		var s string
		if m.state == selecting {
			s = "Select activity:\n"
		} else {
			s = "Delete activity:\n"
		}
		for i, activity := range m.activities {
			cursor, checked := " ", " "
			if m.cursor == i {
				cursor = ">"
				checked = "x"
			}
			// Render the row
			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, activity)
		}
		return s + "\n" + m.help.ShortHelpView([]key.Binding{
			m.keymap.add,
			m.keymap.change,
			m.keymap.delete,
			m.keymap.quit,
		})
	case weekly:
		hdr := fmt.Sprintf("Weekly report for %s (%s):", m.activities[m.selected],
			isoweek.StartTime(m.week[0], m.week[1], time.UTC).Format(time.DateOnly),
		)
		return fmt.Sprintf("%s\n%s\n%s",
			hstyle.Render(hdr),
			tableStyle.Render(m.weekTbl.View()),
			m.helpView(),
		)
	case yearly:
		hdr := fmt.Sprintf("Yearly report for %s (%d):", m.activities[m.selected],
			m.year,
		)
		return fmt.Sprintf("%s\n%s\n%s",
			hstyle.Render(hdr),
			tableStyle.Render(m.yearTbl.View()),
			m.helpView(),
		)
	}
	return "" // won't get here
}

func (m model) statusView() string {
	str := fmt.Sprintf("Bank: %s\nDaily tally: %s, %s, %s",
		m.bank.Round(time.Second),
		m.tally[m.selected][0].Round(time.Second),
		m.tally[m.selected][1].Round(time.Second),
		(m.tally[m.selected][0] + m.tally[m.selected][1]).Round(time.Second),
	)
	return style.Render(str)
}

func (m model) helpView() string {
	return "\n" + m.help.ShortHelpView([]key.Binding{
		m.keymap.work,
		m.keymap.rest,
		m.keymap.change,
		m.keymap.week,
		m.keymap.year,
		m.keymap.stop,
		m.keymap.quit,
	})
}

func (m model) switchTo(s state) model {
	m.statePrev = m.state
	if (s == ready || s == selecting || s == removing) && len(m.activities) == 0 {
		m.state = adding
	} else {
		m.state = s
	}
	switch m.state {
	case ready:
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(true)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(true)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(false)
		m.keymap.week.SetEnabled(m.week[0] > 0)
		m.keymap.year.SetEnabled(m.year > 0)
		m.keymap.next.SetEnabled(false)
		m.keymap.prev.SetEnabled(false)
		m.keymap.quit.SetEnabled(true)
	case adding:
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(false)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(false)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(false)
		m.keymap.week.SetEnabled(false)
		m.keymap.year.SetEnabled(false)
		m.keymap.next.SetEnabled(false)
		m.keymap.prev.SetEnabled(false)
		m.keymap.quit.SetEnabled(false)
	case removing:
		m.cursor = m.selected
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(true)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(false)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(false)
		m.keymap.week.SetEnabled(false)
		m.keymap.year.SetEnabled(false)
		m.keymap.next.SetEnabled(false)
		m.keymap.prev.SetEnabled(false)
		m.keymap.quit.SetEnabled(true)
	case selecting:
		m.cursor = m.selected
		m.keymap.add.SetEnabled(true)
		m.keymap.change.SetEnabled(false)
		m.keymap.delete.SetEnabled(true)
		m.keymap.work.SetEnabled(false)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(false)
		m.keymap.week.SetEnabled(false)
		m.keymap.year.SetEnabled(false)
		m.keymap.next.SetEnabled(false)
		m.keymap.prev.SetEnabled(false)
		m.keymap.quit.SetEnabled(true)
	case working:
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(false)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(false)
		m.keymap.rest.SetEnabled(true)
		m.keymap.stop.SetEnabled(true)
		m.keymap.week.SetEnabled(false)
		m.keymap.year.SetEnabled(false)
		m.keymap.next.SetEnabled(false)
		m.keymap.prev.SetEnabled(false)
		m.keymap.quit.SetEnabled(true)
	case resting:
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(false)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(true)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(true)
		m.keymap.week.SetEnabled(false)
		m.keymap.year.SetEnabled(false)
		m.keymap.next.SetEnabled(false)
		m.keymap.prev.SetEnabled(false)
		m.keymap.quit.SetEnabled(true)
	case weekly:
		rows, nxt, prev := m.log.weeks(m.activities[m.selected], m.week)
		m.weekNxt = nxt
		m.weekPrev = prev
		m.weekTbl.SetRows(rows)
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(true)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(true)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(false)
		m.keymap.week.SetEnabled(false)
		m.keymap.year.SetEnabled(true)
		m.keymap.next.SetEnabled(m.weekNxt[0] > 0)
		m.keymap.prev.SetEnabled(m.weekPrev[0] > 0)
		m.keymap.quit.SetEnabled(true)
	case yearly:
		rows, nxt, prev := m.log.years(m.activities[m.selected], m.year)
		m.yearNxt = nxt
		m.yearPrev = prev
		m.yearTbl.SetRows(rows)
		m.keymap.add.SetEnabled(false)
		m.keymap.change.SetEnabled(true)
		m.keymap.delete.SetEnabled(false)
		m.keymap.work.SetEnabled(true)
		m.keymap.rest.SetEnabled(false)
		m.keymap.stop.SetEnabled(false)
		m.keymap.week.SetEnabled(true)
		m.keymap.year.SetEnabled(false)
		m.keymap.next.SetEnabled(m.yearNxt > 0)
		m.keymap.prev.SetEnabled(m.yearPrev > 0)
		m.keymap.quit.SetEnabled(true)
	case quitting:
		m.log.flush()
	}
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case adding:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				return m.switchTo(ready), nil
			case tea.KeyCtrlC:
				m = m.switchTo(quitting)
				return m, tea.Quit
			case tea.KeyEnter:
				if m.textInput.Value() == "" {
					return m, nil
				}
				m.log.send(entry{a: m.textInput.Value(), typ: selecting, t: time.Now()})
				m.activities, m.selected, m.week, m.weekPrev, m.year, m.yearPrev = m.log.refresh()
				m.weekNxt = [2]int{}
				m.yearNxt = 0
				m.tally = m.log.tally(m.activities)
				m.textInput.Reset()
				return m.switchTo(ready), nil
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	case selecting, removing:
		// Is it a key press?
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "a":
				return m.switchTo(adding), nil
			case "ctrl+c", "q":
				return m.switchTo(quitting), tea.Quit
			case "d":
				return m.switchTo(removing), nil
			case "c":
				return m.switchTo(selecting), nil
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.activities)-1 {
					m.cursor++
				}
			case "enter", " ":
				// remove or select
				if m.selected != m.cursor {
					m.selected = m.cursor
				}
				m.log.send(entry{a: m.activities[m.selected], typ: m.state, t: time.Now()})
				if m.state == selecting {
					if m.statePrev == weekly || m.statePrev == yearly {
						return m.switchTo(m.statePrev), nil
					}
					return m.switchTo(ready), nil
				}
				m.activities, m.selected, m.week, m.weekPrev, m.year, m.yearPrev = m.log.refresh()
				m.weekNxt = [2]int{}
				m.yearNxt = 0
				m.tally = m.log.tally(m.activities)
				return m.switchTo(removing), nil
			}
		}
		return m, cmd
	case ready:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, m.keymap.quit):
				return m.switchTo(quitting), tea.Quit
			case key.Matches(msg, m.keymap.change):
				return m.switchTo(selecting), cmd
			case key.Matches(msg, m.keymap.work):
				return m.switchTo(working), m.stopwatch.Start()
			case key.Matches(msg, m.keymap.week):
				return m.switchTo(weekly), cmd
			case key.Matches(msg, m.keymap.year):
				return m.switchTo(yearly), cmd
			}
		}
		return m, nil
	case working, resting:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, m.keymap.quit):
				m.log.send(entry{a: m.activities[m.selected], typ: m.state, t: time.Now(), d: m.stopwatch.Elapsed()})
				return m.switchTo(quitting), tea.Quit
			case key.Matches(msg, m.keymap.stop):
				if m.state == working {
					m.tally[m.selected][0] += m.stopwatch.Elapsed()
				} else {
					m.tally[m.selected][1] += m.stopwatch.Elapsed()
				}
				m.log.send(entry{a: m.activities[m.selected], typ: m.state, t: time.Now(), d: m.stopwatch.Elapsed()})
				m.stopwatch, cmd = m.stopwatch.Update(m.stopwatch.Reset()()) // force a reset - note a tea.Cmd needs to be turned into a tea.Msg
				return m.switchTo(ready), cmd
			case key.Matches(msg, m.keymap.work):
				m.tally[m.selected][1] += m.stopwatch.Elapsed()
				m.log.send(entry{a: m.activities[m.selected], typ: m.state, t: time.Now(), d: m.stopwatch.Elapsed()})
				m.bank -= m.stopwatch.Elapsed()
				return m.switchTo(working), m.stopwatch.Reset()
			case key.Matches(msg, m.keymap.rest):
				m.tally[m.selected][0] += m.stopwatch.Elapsed()
				m.log.send(entry{a: m.activities[m.selected], typ: m.state, t: time.Now(), d: m.stopwatch.Elapsed()})
				m.bank = m.bank + m.stopwatch.Elapsed()/3
				return m.switchTo(resting), m.stopwatch.Reset()
			}
		}
		// handle the tick
		m.stopwatch, cmd = m.stopwatch.Update(msg)
		return m, cmd
	case weekly, yearly:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, m.keymap.quit):
				return m.switchTo(quitting), tea.Quit
			case key.Matches(msg, m.keymap.change):
				return m.switchTo(selecting), cmd
			case key.Matches(msg, m.keymap.work):
				return m.switchTo(working), m.stopwatch.Start()
			case key.Matches(msg, m.keymap.week):
				return m.switchTo(weekly), nil
			case key.Matches(msg, m.keymap.year):
				return m.switchTo(yearly), nil
			case key.Matches(msg, m.keymap.next):
				if m.state == weekly {
					m.week = m.weekNxt
				} else {
					m.year = m.yearNxt
				}
				return m.switchTo(m.state), nil
			case key.Matches(msg, m.keymap.prev):
				if m.state == weekly {
					m.week = m.weekPrev
				} else {
					m.year = m.yearPrev
				}
				return m.switchTo(m.state), nil
			case msg.Type == tea.KeyEnter || msg.Type == tea.KeySpace || msg.Type == tea.KeyEsc:
				return m.switchTo(ready), nil
			}

		}
		if m.state == weekly {
			m.weekTbl, cmd = m.weekTbl.Update(msg)
		} else {
			m.yearTbl, cmd = m.yearTbl.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func main() {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	lg, err := newlogger()
	if err != nil {
		fmt.Printf("something went wrong: %v", err)
		os.Exit(1)
	}
	act, sel, week, weekPrev, year, yearPrev := lg.refresh()

	wcols := []table.Column{
		{Title: "Type", Width: 5},
		{Title: "Mon", Width: 6},
		{Title: "Tues", Width: 6},
		{Title: "Weds", Width: 6},
		{Title: "Thurs", Width: 6},
		{Title: "Fri", Width: 6},
		{Title: "Sat", Width: 6},
		{Title: "Sun", Width: 6},
		{Title: "Total", Width: 6},
	}

	ycols := []table.Column{
		{Title: "Type", Width: 5},
		{Title: "Jan", Width: 7},
		{Title: "Feb", Width: 7},
		{Title: "Mar", Width: 7},
		{Title: "Apr", Width: 7},
		{Title: "May", Width: 7},
		{Title: "June", Width: 7},
		{Title: "July", Width: 7},
		{Title: "Aug", Width: 7},
		{Title: "Sept", Width: 7},
		{Title: "Oct", Width: 7},
		{Title: "Nov", Width: 7},
		{Title: "Dec", Width: 7},
		{Title: "Total", Width: 8},
	}

	wt := table.New(
		table.WithColumns(wcols),
		table.WithFocused(true),
		table.WithHeight(3),
	)

	yt := table.New(
		table.WithColumns(ycols),
		table.WithFocused(true),
		table.WithHeight(3),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	wt.SetStyles(s)
	yt.SetStyles(s)

	m := model{
		log:        lg,
		activities: act,
		tally:      lg.tally(act),
		selected:   sel,
		textInput:  ti,
		stopwatch:  stopwatch.NewWithInterval(time.Second),
		keymap: keymap{
			work: key.NewBinding(
				key.WithKeys("w"),
				key.WithHelp("w", "work"),
			),
			rest: key.NewBinding(
				key.WithKeys("b"),
				key.WithHelp("b", "break"),
			),
			add: key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "add"),
			),
			change: key.NewBinding(
				key.WithKeys("c"),
				key.WithHelp("c", "change activity"),
			),
			delete: key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "delete"),
			),
			stop: key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "stop"),
			),
			week: key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "weekly report"),
			),
			year: key.NewBinding(
				key.WithKeys("y"),
				key.WithHelp("y", "yearly report"),
			),
			next: key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "next"),
			),
			prev: key.NewBinding(
				key.WithKeys("p"),
				key.WithHelp("p", "previous"),
			),
			quit: key.NewBinding(
				key.WithKeys("ctrl+c", "q"),
				key.WithHelp("q", "quit"),
			),
		},
		help:     help.New(),
		week:     week,
		weekPrev: weekPrev,
		year:     year,
		yearPrev: yearPrev,
		weekTbl:  wt,
		yearTbl:  yt,
	}
	m = m.switchTo(ready)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Oh no, it didn't work:", err)
		os.Exit(1)
	}
}
