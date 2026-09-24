package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type page int

const (
	pageToday page = iota
	pageUpcoming
	pageUnplanned
	pageCalendar
	pageTags
	pageAll
)

const (
	sideAllFilters = iota
	sideToday
	sideUpcoming
	sideUnplanned
	sideCalendar
	sideTags
	sideAllProjects
	sideFirstProject
)

var labels = struct {
	Today, Upcoming, Unplanned, Calendar, Tags, Projects, Help, Empty string
}{"Today", "Upcoming", "Unplanned", "Calendar", "Tags", "Projects", "? help", "Nothing here"}

type loadedMsg struct {
	tasks    []task
	projects []project
	tags     []tag
	err      error
}
type actionMsg struct{ err error }
type stateSavedMsg struct{ err error }
type editorKind int

const (
	editTask editorKind = iota
	editProject
	editTag
)

type editor struct {
	kind        editorKind
	create      bool
	fields      []textinput.Model
	note        textarea.Model
	focus       int
	task        task
	project     project
	tag         tag
	calendar    bool
	colorPicker bool
	colorIndex  int
	choice      int
	choiceIndex int
	date        time.Time
}

var colorPalette = []string{
	"#fecaca", "#fca5a5", "#f87171", "#ef4444", "#dc2626", "#991b1b",
	"#fed7aa", "#fdba74", "#fb923c", "#f97316", "#ea580c", "#9a3412",
	"#fef08a", "#fde047", "#facc15", "#eab308", "#ca8a04", "#854d0e",
	"#bbf7d0", "#86efac", "#4ade80", "#22c55e", "#16a34a", "#166534",
	"#99f6e4", "#5eead4", "#2dd4bf", "#14b8a6", "#0d9488", "#115e59",
	"#bae6fd", "#7dd3fc", "#38bdf8", "#0ea5e9", "#0284c7", "#075985",
	"#c7d2fe", "#a5b4fc", "#818cf8", "#6366f1", "#4f46e5", "#3730a3",
	"#e9d5ff", "#d8b4fe", "#c084fc", "#a855f7", "#9333ea", "#6b21a8",
	"#fbcfe8", "#f9a8d4", "#f472b6", "#ec4899", "#db2777", "#9d174d",
	"#e5e7eb", "#9ca3af", "#6b7280", "#374151", "#111827", "#000000",
}

type model struct {
	api                *client
	tasks              []task
	projects           []project
	tags               []tag
	page               page
	selectedProjectUID string
	lastFilter         string
	side, cursor       int
	focusSide          bool
	showCompleted      bool
	week               bool
	workingWeek        bool
	calendarTasks      bool
	calDate            time.Time
	width, height      int
	loading            bool
	err                string
	status             string
	editor             *editor
	confirm            bool
	help               bool
}

var (
	blue       = lipgloss.Color("#3b82f6")
	muted      = lipgloss.Color("#8b949e")
	panel      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(0, 1)
	selected   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#1f6feb")).Bold(true)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "#f0f6fc"})
	dim        = lipgloss.NewStyle().Foreground(muted)
)

func newModel(api *client, state appState) model {
	now := time.Now()
	p := pageFromFilter(state.Filter)
	side := sideAllFilters
	if p <= pageUnplanned {
		side = int(p) + sideToday
	}
	return model{api: api, page: p, side: side, selectedProjectUID: state.ProjectUID, lastFilter: state.Filter, focusSide: true, loading: true, calDate: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())}
}

func pageFromFilter(filter string) page {
	switch filter {
	case "upcoming":
		return pageUpcoming
	case "unplanned":
		return pageUnplanned
	case "all":
		return pageAll
	default:
		return pageToday
	}
}

func (m model) persistedState() appState {
	filter := m.lastFilter
	if filter == "" && m.page <= pageUnplanned {
		filter = []string{"today", "upcoming", "unplanned"}[m.page]
	}
	if filter == "" {
		filter = "today"
	}
	return appState{ProjectUID: m.selectedProjectUID, Filter: filter}
}

func (m model) saveStateCmd() tea.Cmd {
	state := m.persistedState()
	return func() tea.Msg { return stateSavedMsg{saveState(state)} }
}

func (m model) Init() tea.Cmd { return m.loadCmd() }
func (m model) loadCmd() tea.Cmd {
	return func() tea.Msg { t, p, g, err := m.api.load(); return loadedMsg{t, p, g, err} }
}
func action(fn func() error) tea.Cmd { return func() tea.Msg { return actionMsg{fn()} } }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.tasks, m.projects, m.tags, m.err = msg.tasks, msg.projects, msg.tags, ""
		}
		m.clamp()
		if m.selectedProjectUID != "" && m.selectedProject() == nil {
			m.selectedProjectUID = ""
			return m, m.saveStateCmd()
		}
		return m, nil
	case actionMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.editor, m.confirm, m.status = nil, false, "Saved"
		return m, m.loadCmd()
	case stateSavedMsg:
		if msg.err != nil {
			m.err = "save state: " + msg.err.Error()
		}
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.editor != nil {
		return m.updateEditor(key)
	}
	if m.confirm {
		return m.updateConfirm(key)
	}
	if m.help {
		if key.String() == "?" || key.String() == "esc" {
			m.help = false
		}
		return m, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.help = true
	case "tab":
		m.focusSide = !m.focusSide
	case "h":
		m.showCompleted = !m.showCompleted
		m.cursor = 0
	case "r":
		m.loading = true
		return m, m.loadCmd()
	case "v":
		if m.page == pageCalendar {
			if !m.week {
				m.week = true
			} else if !m.workingWeek {
				m.workingWeek = true
			} else {
				m.week, m.workingWeek = false, false
			}
		}
	case "left":
		if m.page == pageCalendar && !m.focusSide {
			if m.calendarTasks {
				m.calendarTasks = false
			} else {
				m.moveCalendarDays(-1)
			}
		} else if !m.focusSide {
			m.focusSide = true
		}
	case "right":
		if m.page == pageCalendar && !m.focusSide && !m.calendarTasks {
			m.moveCalendarDays(1)
		} else if m.focusSide {
			m.focusSide = false
		}
	case "[":
		if m.page == pageCalendar && !m.focusSide {
			m.moveCalendar(-1)
		}
	case "]":
		if m.page == pageCalendar && !m.focusSide {
			m.moveCalendar(1)
		}
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter":
		if m.focusSide {
			cmd := m.chooseSide()
			return m, cmd
		}
		if m.page == pageTags {
			m.openEdit()
		} else if m.page == pageCalendar && len(m.tasksDueOn(m.calDate)) > 0 {
			m.calendarTasks = true
			m.cursor = 0
		}
	case "esc":
		m.calendarTasks = false
	case "n":
		m.openNew()
	case "e":
		m.openEdit()
	case "d":
		if m.selectedExists() {
			m.confirm = true
		}
	case " ":
		if t, ok := m.selectedTask(); ok {
			m.loading = true
			return m, action(func() error { return m.api.toggleTask(t) })
		}
	}
	return m, nil
}

func (m *model) move(n int) {
	if m.focusSide {
		m.side += n
		if m.side < 0 {
			m.side = m.sideCount() - 1
		}
		if m.side >= m.sideCount() {
			m.side = 0
		}
		return
	}
	if m.page == pageCalendar {
		if m.calendarTasks {
			m.cursor += n
			m.cursor = max(0, min(m.cursor, len(m.tasksDueOn(m.calDate))-1))
		} else if m.week {
			m.moveCalendarDays(2 * n)
		} else {
			m.moveCalendarDays(7 * n)
		}
		return
	}
	m.cursor += n
	m.clamp()
}
func (m *model) moveCalendarDays(n int) {
	m.calendarTasks = false
	m.cursor = 0
	step := 1
	if n < 0 {
		step = -1
	}
	distance := n
	if distance < 0 {
		distance = -distance
	}
	for moved := 0; moved < distance; {
		m.calDate = m.calDate.AddDate(0, 0, step)
		if !m.workingWeek || m.calDate.Weekday() != time.Saturday && m.calDate.Weekday() != time.Sunday {
			moved++
		}
	}
}
func (m *model) moveCalendar(n int) {
	m.calendarTasks = false
	m.cursor = 0
	if m.week {
		m.calDate = m.calDate.AddDate(0, 0, 7*n)
	} else {
		m.calDate = m.calDate.AddDate(0, n, 0)
	}
}
func (m *model) clamp() {
	l := m.currentLen()
	if l == 0 {
		m.cursor = 0
	} else if m.cursor >= l {
		m.cursor = l - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
func (m model) sideCount() int { return sideFirstProject + len(m.projects) }
func (m *model) chooseSide() tea.Cmd {
	m.cursor = 0
	if m.side == sideAllFilters {
		m.page, m.lastFilter = pageAll, "all"
		return m.saveStateCmd()
	}
	if m.side >= sideToday && m.side <= sideUnplanned {
		m.page = page(m.side - sideToday)
		m.lastFilter = []string{"today", "upcoming", "unplanned"}[m.side-sideToday]
		return m.saveStateCmd()
	}
	if m.side == sideCalendar || m.side == sideTags {
		m.page = page(m.side - sideToday)
		m.focusSide = false
		return nil
	}
	if m.side == sideAllProjects {
		m.selectedProjectUID = ""
		return m.saveStateCmd()
	}
	p := m.projects[m.side-sideFirstProject]
	m.selectedProjectUID = p.UID
	return m.saveStateCmd()
}

func (m model) filteredTasks() []task {
	now := time.Now()
	y, mo, d := now.Date()
	start := time.Date(y, mo, d, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var out []task
	for _, t := range m.tasks {
		if t.completed() != m.showCompleted {
			continue
		}
		if !m.matchesSelectedProject(t) {
			continue
		}
		due, hasDue := t.due()
		match := false
		switch m.page {
		case pageToday:
			match = hasDue && due.Before(end)
		case pageUpcoming:
			match = hasDue && !due.Before(end)
		case pageUnplanned:
			match = !hasDue
		default:
			match = true
		}
		if match {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, aDue := out[i].due()
		b, bDue := out[j].due()
		if aDue != bDue {
			return aDue
		}
		return aDue && a.Before(b)
	})
	return out
}

func (m model) currentLen() int {
	if m.page == pageTags {
		return len(m.tags)
	}
	if m.page == pageCalendar {
		return 0
	}
	return len(m.filteredTasks())
}
func (m model) selectedTask() (task, bool) {
	ts := m.filteredTasks()
	if m.page == pageTags || m.page == pageCalendar || m.cursor >= len(ts) {
		return task{}, false
	}
	return ts[m.cursor], true
}
func (m model) selectedExists() bool {
	if _, ok := m.focusedProject(); ok {
		return true
	}
	if m.page == pageTags {
		return m.cursor < len(m.tags)
	}
	_, ok := m.selectedTask()
	return ok
}

func (m model) selectedProject() *project {
	for i := range m.projects {
		if m.projects[i].UID == m.selectedProjectUID {
			return &m.projects[i]
		}
	}
	return nil
}

func (m model) focusedProject() (project, bool) {
	if m.focusSide && m.side >= sideFirstProject && m.side-sideFirstProject < len(m.projects) {
		return m.projects[m.side-sideFirstProject], true
	}
	return project{}, false
}

func (m model) matchesSelectedProject(t task) bool {
	p := m.selectedProject()
	if p == nil {
		return true
	}
	return t.ProjectID == p.ID || t.ProjectUID == p.UID || t.Project != nil && t.Project.UID == p.UID
}

func input(value, placeholder string) textinput.Model {
	i := textinput.New()
	i.SetValue(value)
	i.Placeholder = placeholder
	i.CharLimit = 200
	i.Width = 48
	return i
}
func note(value, placeholder string) textarea.Model {
	t := textarea.New()
	t.SetValue(value)
	t.Placeholder = placeholder
	t.SetWidth(58)
	t.SetHeight(10)
	return t
}

func (m *model) openNew() {
	if _, ok := m.focusedProject(); ok {
		m.editor = &editor{kind: editProject, create: true, fields: []textinput.Model{input("", "Name"), input("#3b82f6", "#RRGGBB"), input("not_started", "Status")}, note: note("", "Description")}
		m.editor.fields[0].Focus()
		return
	}
	switch m.page {
	case pageTags:
		m.editor = &editor{kind: editTag, create: true, fields: []textinput.Model{input("", "Name"), input("#3b82f6", "#RRGGBB")}}
	default:
		m.editor = &editor{kind: editTask, create: true, fields: []textinput.Model{input("", "Name"), input("", "Due date"), input("", "Project"), input("", "Tags, comma separated")}, note: note("", "Markdown description")}
	}
	m.editor.fields[0].Focus()
}

func (m *model) openEdit() {
	if p, ok := m.focusedProject(); ok {
		m.editor = &editor{kind: editProject, project: p, fields: []textinput.Model{input(p.Name, "Name"), input(p.Color, "#RRGGBB"), input(p.Status, "Status")}, note: note(p.Description, "Description")}
	} else if m.page == pageTags && m.cursor < len(m.tags) {
		t := m.tags[m.cursor]
		m.editor = &editor{kind: editTag, tag: t, fields: []textinput.Model{input(t.Name, "Name"), input(t.Color, "#RRGGBB")}}
	} else if t, ok := m.selectedTask(); ok {
		var due, projectName string
		if d, ok := t.due(); ok {
			due = d.Format("2006-01-02")
		}
		if t.Project != nil {
			projectName = t.Project.Name
		} else {
			for _, p := range m.projects {
				if p.ID == t.ProjectID {
					projectName = p.Name
				}
			}
		}
		names := make([]string, len(t.Tags))
		for i, v := range t.Tags {
			names[i] = v.Name
		}
		m.editor = &editor{kind: editTask, task: t, fields: []textinput.Model{input(t.Name, "Name"), input(due, "Due date"), input(projectName, "Project"), input(strings.Join(names, ", "), "Tags")}, note: note(t.Note, "Markdown description")}
	}
	if m.editor != nil {
		m.editor.fields[0].Focus()
	}
}

func (m model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "n":
		m.confirm = false
	case "y", "enter":
		m.loading = true
		m.confirm = false
		if p, ok := m.focusedProject(); ok {
			return m, action(func() error { return m.api.deleteProject(p) })
		}
		if m.page == pageTags && m.cursor < len(m.tags) {
			t := m.tags[m.cursor]
			return m, action(func() error { return m.api.deleteTag(t) })
		}
		if t, ok := m.selectedTask(); ok {
			return m, action(func() error { return m.api.deleteTask(t) })
		}
	}
	return m, nil
}

func (m model) updateEditor(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	if e.calendar {
		return m.updateMiniCalendar(key)
	}
	if e.colorPicker {
		return m.updateColorPicker(key)
	}
	if e.choice != 0 {
		return m.updateTaskChoice(key)
	}
	switch key.String() {
	case "esc":
		m.editor = nil
		return m, nil
	case "ctrl+s":
		return m.saveEditor()
	case "ctrl+d":
		if e.kind == editTask {
			d, err := time.Parse("2006-01-02", e.fields[1].Value())
			if err != nil {
				d = time.Now()
			}
			e.date, e.calendar = d, true
		}
		return m, nil
	case "enter", "ctrl+p":
		if e.kind == editTask {
			switch e.focus {
			case 1:
				d, err := time.Parse("2006-01-02", e.fields[1].Value())
				if err != nil {
					d = time.Now()
				}
				e.date, e.calendar = d, true
			case 2:
				e.choice, e.choiceIndex = 1, 0
				for i, p := range m.projects {
					if strings.EqualFold(p.Name, e.fields[2].Value()) {
						e.choiceIndex = i + 1
					}
				}
			case 3:
				e.choice, e.choiceIndex = 2, 0
			}
			return m, nil
		}
		if e.kind != editTask && e.focus == 1 {
			e.colorIndex = 0
			for i, color := range colorPalette {
				if strings.EqualFold(color, e.fields[1].Value()) {
					e.colorIndex = i + 1
				}
			}
			e.colorPicker = true
			return m, nil
		}
	case "tab", "shift+tab", "up", "down":
		if e.focus == len(e.fields) && (key.String() == "up" || key.String() == "down") {
			break
		}
		if e.focus == len(e.fields) {
			e.note.Blur()
		} else {
			e.fields[e.focus].Blur()
		}
		if key.String() == "tab" || key.String() == "down" {
			e.focus++
		} else {
			e.focus--
		}
		max := len(e.fields)
		if e.kind == editTag {
			max--
		}
		if e.focus > max {
			e.focus = 0
		}
		if e.focus < 0 {
			e.focus = max
		}
		if e.focus == len(e.fields) {
			e.note.Focus()
		} else {
			e.fields[e.focus].Focus()
		}
		return m, nil
	}
	var cmd tea.Cmd
	if e.focus == len(e.fields) {
		if e.kind == editTask {
			editorW := min(64, max(38, m.width/3))
			e.note.SetWidth(max(10, editorW-6))
			e.note.SetHeight(max(3, m.height-25))
		}
		e.note, cmd = e.note.Update(key)
	} else if e.kind == editTask && (e.focus == 2 || e.focus == 3) {
		return m, nil
	} else {
		e.fields[e.focus], cmd = e.fields[e.focus].Update(key)
	}
	return m, cmd
}

func (m model) updateTaskChoice(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	count := len(m.projects) + 1
	if e.choice == 2 {
		count = len(m.tags)
	}
	switch key.String() {
	case "esc", "tab", "shift+tab":
		e.choice = 0
	case "up", "k":
		if count > 0 {
			e.choiceIndex = (e.choiceIndex - 1 + count) % count
		}
	case "down", "j":
		if count > 0 {
			e.choiceIndex = (e.choiceIndex + 1) % count
		}
	case "enter", " ":
		if e.choice == 1 {
			if e.choiceIndex == 0 {
				e.fields[2].SetValue("")
			} else {
				e.fields[2].SetValue(m.projects[e.choiceIndex-1].Name)
			}
			e.choice = 0
		} else if count > 0 {
			name := m.tags[e.choiceIndex].Name
			selected := selectedNames(e.fields[3].Value())
			if selected[strings.ToLower(name)] {
				delete(selected, strings.ToLower(name))
			} else {
				selected[strings.ToLower(name)] = true
			}
			var names []string
			for _, tag := range m.tags {
				if selected[strings.ToLower(tag.Name)] {
					names = append(names, tag.Name)
				}
			}
			e.fields[3].SetValue(strings.Join(names, ", "))
		}
	}
	return m, nil
}

func selectedNames(value string) map[string]bool {
	selected := map[string]bool{}
	for _, name := range strings.Split(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			selected[strings.ToLower(name)] = true
		}
	}
	return selected
}

func (m model) updateColorPicker(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	if e.colorIndex == 0 {
		switch key.String() {
		case "esc":
			e.colorPicker = false
			return m, nil
		case "down", "tab":
			e.colorIndex = 1
			return m, nil
		case "enter":
			if validHexColor(e.fields[1].Value()) {
				e.colorPicker = false
			} else {
				m.err = "Color must use #RRGGBB"
			}
			return m, nil
		}
		var cmd tea.Cmd
		e.fields[1], cmd = e.fields[1].Update(key)
		return m, cmd
	}
	const columns = 6
	switch key.String() {
	case "esc":
		e.colorPicker = false
	case "left", "h":
		if e.colorIndex > 1 {
			e.colorIndex--
		}
	case "right", "l":
		if e.colorIndex < len(colorPalette) {
			e.colorIndex++
		}
	case "up", "k":
		e.colorIndex -= columns
		if e.colorIndex < 1 {
			e.colorIndex = 0
		}
	case "down", "j":
		e.colorIndex = min(len(colorPalette), e.colorIndex+columns)
	case "enter":
		e.fields[1].SetValue(colorPalette[e.colorIndex-1])
		e.colorPicker = false
	}
	return m, nil
}

func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(value[1:], 16, 24)
	return err == nil
}

func (m model) updateMiniCalendar(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.editor
	switch key.String() {
	case "esc":
		e.calendar = false
	case "left", "h":
		e.date = e.date.AddDate(0, 0, -1)
	case "right", "l":
		e.date = e.date.AddDate(0, 0, 1)
	case "up", "k":
		e.date = e.date.AddDate(0, 0, -7)
	case "down", "j":
		e.date = e.date.AddDate(0, 0, 7)
	case "[":
		e.date = e.date.AddDate(0, -1, 0)
	case "]":
		e.date = e.date.AddDate(0, 1, 0)
	case "backspace", "delete":
		e.fields[1].SetValue("")
		e.calendar = false
	case "enter":
		e.fields[1].SetValue(e.date.Format("2006-01-02"))
		e.calendar = false
	}
	return m, nil
}

func (m model) saveEditor() (tea.Model, tea.Cmd) {
	e := m.editor
	if strings.TrimSpace(e.fields[0].Value()) == "" {
		m.err = "Name is required"
		return m, nil
	}
	m.loading = true
	m.err = ""
	switch e.kind {
	case editTag:
		t := e.tag
		t.Name, t.Color = strings.TrimSpace(e.fields[0].Value()), strings.TrimSpace(e.fields[1].Value())
		return m, action(func() error { return m.api.saveTag(t, e.create) })
	case editProject:
		p := e.project
		p.Name, p.Color, p.Status, p.Description = strings.TrimSpace(e.fields[0].Value()), strings.TrimSpace(e.fields[1].Value()), strings.TrimSpace(e.fields[2].Value()), e.note.Value()
		if p.Status == "" {
			p.Status = "not_started"
		}
		return m, action(func() error { return m.api.saveProject(p, e.create) })
	default:
		t := e.task
		t.Name, t.Note, t.DueDate = strings.TrimSpace(e.fields[0].Value()), e.note.Value(), ""
		if d := strings.TrimSpace(e.fields[1].Value()); d != "" {
			parsed, err := time.Parse("2006-01-02", d)
			if err != nil {
				m.loading = false
				m.err = "Date must be YYYY-MM-DD"
				return m, nil
			}
			t.DueDate = parsed.Format(time.RFC3339)
		}
		t.ProjectID = 0
		pn := strings.TrimSpace(e.fields[2].Value())
		for _, p := range m.projects {
			if strings.EqualFold(p.Name, pn) {
				t.ProjectID = p.ID
				break
			}
		}
		if pn != "" && t.ProjectID == 0 {
			m.loading = false
			m.err = "Unknown project"
			return m, nil
		}
		t.Tags = nil
		for _, name := range strings.Split(e.fields[3].Value(), ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				t.Tags = append(t.Tags, tag{Name: name})
			}
		}
		return m, action(func() error { return m.api.saveTask(t, e.create) })
	}
}

func (m model) View() string {
	if m.width > 0 && (m.width < 80 || m.height < 20) {
		return panel.Render("Terminal too small\nMinimum: 80x20")
	}
	if m.editor != nil && (m.editor.kind != editTask || m.editor.calendar || m.editor.colorPicker) {
		return m.editorView()
	}
	if m.help {
		return m.helpView()
	}
	if m.confirm {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Width(42).Render("Delete selected item?\n\n[y/enter] delete   [n/esc] cancel"))
	}
	sideW := 24
	if m.width < 100 {
		sideW = 20
	}
	mainW := m.width - sideW - 1
	editorW := 0
	if m.editor != nil && m.editor.kind == editTask {
		editorW = min(64, max(38, m.width/3))
		mainW -= editorW
	}
	sidebar := panel.Width(sideW - 2).Height(m.height - 3).Render(m.sidebarView())
	content := panel.Width(mainW - 2).Height(m.height - 3).Render(m.contentView(mainW - 4))
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	if editorW > 0 {
		editor := panel.Width(editorW - 2).Height(m.height - 3).Render(m.editorContent(editorW-4, m.height-5))
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, editor)
	}
	footer := m.footerView()
	if m.err != "" {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Render(m.err)
	}
	if m.loading {
		footer = dim.Render("Loading…")
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m model) footerView() string {
	completionLabel := "completed"
	if m.showCompleted {
		completionLabel = "active"
	}
	items := [][2]string{{"tab", "focus"}, {"n", "new"}, {"e", "edit"}, {"d", "delete"}, {"space", "complete"}, {"h", completionLabel}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	if m.width < 150 {
		items = [][2]string{{"tab", "focus"}, {"n", "new"}, {"e", "edit"}, {"d", "delete"}, {"space", "done"}, {"h", completionLabel}, {"?", "help"}, {"q", "quit"}}
	}
	if m.width < 105 {
		items = [][2]string{{"tab", "focus"}, {"n", "new"}, {"e", "edit"}, {"h", completionLabel}, {"?", "help"}, {"q", "quit"}}
	}
	capStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), false, true, false, true).BorderForeground(lipgloss.Color("#1f6feb")).Background(lipgloss.Color("#1f6feb")).Foreground(lipgloss.Color("#ffffff")).Bold(true).Padding(0, 1)
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = capStyle.Render(item[0]) + " " + dim.Render(item[1])
	}
	return strings.Join(parts, "  ")
}

func (m model) sidebarView() string {
	items := []string{"All", "◷  " + labels.Today, "→  " + labels.Upcoming, "○  " + labels.Unplanned, "▦  " + labels.Calendar, "#  " + labels.Tags}
	var b strings.Builder
	for i, v := range items {
		if i <= sideUnplanned {
			active := i == sideAllFilters && m.page == pageAll || i >= sideToday && m.page == page(i-sideToday)
			if active {
				v = "> " + v
			} else {
				v = "  " + v
			}
		}
		b.WriteString(m.sideLine(i, v))
		b.WriteByte('\n')
	}
	b.WriteString("\n" + dim.Render(strings.ToUpper(labels.Projects)) + "\n")
	allProjects := "  All"
	if m.selectedProjectUID == "" {
		allProjects = "> All"
	}
	b.WriteString(m.sideLine(sideAllProjects, allProjects) + "\n")
	for i, p := range m.projects {
		prefix := "  "
		if p.UID == m.selectedProjectUID {
			prefix = "> "
		}
		name := prefix + "● " + p.Name
		if p.Color != "" && !(m.focusSide && m.side == i+sideFirstProject) {
			name = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Color)).Render(name)
		}
		b.WriteString(m.sideLine(i+sideFirstProject, name))
		b.WriteByte('\n')
	}
	return b.String()
}
func (m model) sideLine(i int, s string) string {
	if m.focusSide && m.side == i {
		return selected.Width(20).Render(s)
	}
	return lipgloss.NewStyle().Width(20).Render(s)
}

func (m model) contentView(width int) string {
	if m.page == pageCalendar {
		return m.calendarView(width)
	}
	if m.page == pageTags {
		return m.tagsView()
	}
	ts := m.filteredTasks()
	heading := "All tasks"
	if m.page <= pageUnplanned {
		heading = []string{labels.Today, labels.Upcoming, labels.Unplanned}[m.page]
	}
	heading = fmt.Sprintf("%s (%d)", heading, len(ts))
	var b strings.Builder
	b.WriteString(titleStyle.Render(heading))
	if p := m.selectedProject(); p != nil {
		style := titleStyle
		if p.Color != "" {
			style = style.Foreground(lipgloss.Color(p.Color))
		}
		b.WriteString(dim.Render("  ·  ") + style.Render(p.Name))
	}
	if m.showCompleted {
		b.WriteString(dim.Render("  · completed only"))
	}
	b.WriteString("\n\n")
	if len(ts) == 0 {
		b.WriteString(dim.Render(labels.Empty))
		return b.String()
	}
	listW := width
	details := width >= 92
	if details {
		listW = width * 55 / 100
	}
	visibleCount := max(1, (m.height-8)/3)
	start := 0
	if m.cursor >= visibleCount {
		start = m.cursor - visibleCount + 1
	}
	end := min(len(ts), start+visibleCount)
	visible := ts[start:end]
	var list strings.Builder
	for i, t := range visible {
		list.WriteString(m.taskListItem(t, listW, !m.focusSide && start+i == m.cursor))
		if i < len(visible)-1 {
			list.WriteString("\n" + dim.Render(strings.Repeat("─", max(1, listW-2))) + "\n")
		}
	}
	if !details {
		return b.String() + list.String()
	}
	detail := ""
	if m.cursor < len(ts) {
		detail = m.taskDetail(ts[m.cursor], width-listW-3)
	}
	detailHeight := max(3, m.height-9)
	detailWidth := width - listW
	detailPanel := panel.Width(detailWidth - 2).MaxWidth(detailWidth).Height(detailHeight).MaxHeight(detailHeight).Render(detail)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(listW).Render(list.String()), detailPanel))
	return b.String()
}

func (m model) taskListItem(t task, width int, active bool) string {
	mark := "○"
	if t.completed() {
		mark = "✓"
	}
	title := mark + " " + truncate(t.Name, width-4)
	var metadata []string
	late := ""
	for _, tag := range t.Tags {
		if active {
			metadata = append(metadata, "#"+tag.Name)
			continue
		}
		style := dim
		if tag.Color != "" {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
		}
		metadata = append(metadata, style.Render("#"+tag.Name))
	}
	if due, ok := t.due(); ok {
		now := time.Now()
		y, month, day := now.Date()
		if !t.completed() && due.Before(time.Date(y, month, day, 0, 0, 0, 0, now.Location())) {
			late = "⚠  late"
			if !active {
				late = lipgloss.NewStyle().Foreground(lipgloss.Color("#dc2626")).Bold(true).Render(late)
			}
		}
		date := "◷ " + due.Format("Jan 02")
		if active {
			metadata = append(metadata, date)
		} else {
			metadata = append(metadata, dim.Render(date))
		}
	}
	second := "  " + strings.Join(metadata, "  ")
	if len(metadata) == 0 {
		second = " "
	}
	if late != "" {
		second += strings.Repeat(" ", max(1, width-2-lipgloss.Width(second)-lipgloss.Width(late))) + late
	}
	item := title + "\n" + second
	if active {
		return selected.Width(width - 2).Render(item)
	}
	return item
}

func (m model) taskDetail(t task, width int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(t.Name) + "\n")
	if d, ok := t.due(); ok {
		b.WriteString(dim.Render("Due "+d.Format("Mon, 02 Jan 2006")) + "\n")
	}
	if len(t.Tags) > 0 {
		for _, tag := range t.Tags {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color)).Render("#"+tag.Name) + " ")
		}
		b.WriteByte('\n')
	}
	if t.Note != "" {
		rendered, err := glamour.Render(t.Note, "dark")
		if err == nil {
			b.WriteString("\n" + rendered)
		} else {
			b.WriteString("\n" + t.Note)
		}
	}
	return b.String()
}

func (m model) tagsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(labels.Tags) + "\n\n")
	if len(m.tags) == 0 {
		return b.String() + dim.Render(labels.Empty)
	}
	for i, t := range m.tags {
		active := !m.focusSide && i == m.cursor
		dot, color := "●", t.Color
		if !active {
			if t.Color != "" {
				dot = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Color)).Render(dot)
			}
			color = dim.Render(color)
		}
		line := fmt.Sprintf("%s  %-28s %s", dot, t.Name, color)
		if active {
			line = selected.Width(46).Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m model) calendarView(width int) string {
	if m.week {
		return m.weekView(width)
	}
	first := time.Date(m.calDate.Year(), m.calDate.Month(), 1, 0, 0, 0, 0, m.calDate.Location())
	start := first.AddDate(0, 0, -(int(first.Weekday())+6)%7)
	cellW := max(7, width/7)
	detail := m.calendarSelection(width)
	cellH := max(3, (m.height-7-lipgloss.Height(detail))/6)
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.calDate.Format("January 2006")) + dim.Render("  arrows day  enter tasks  [/] month  v view") + "\n")
	if detail != "" {
		b.WriteString(detail + "\n")
	}
	for _, d := range []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"} {
		b.WriteString(dim.Width(cellW).Align(lipgloss.Center).Render(d))
	}
	b.WriteByte('\n')
	for row := 0; row < 6; row++ {
		cells := make([]string, 0, 7)
		for col := 0; col < 7; col++ {
			day := start.AddDate(0, 0, row*7+col)
			dayLabel := fmt.Sprintf("%2d", day.Day())
			if sameDay(day, m.calDate) {
				dayLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(blue).Bold(true).Render(dayLabel)
			}
			text := dayLabel
			tasks := m.tasksDueOn(day)
			if len(tasks) > 0 {
				text += " " + truncate(tasks[0].Name, cellW-4)
				if len(tasks) > 1 {
					text += fmt.Sprintf(" +%d", len(tasks)-1)
				}
			}
			st := lipgloss.NewStyle().Width(cellW - 2).MaxWidth(cellW).Height(cellH - 2).MaxHeight(cellH).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#30363d"))
			if day.Month() != m.calDate.Month() {
				st = st.Foreground(muted)
			}
			if sameDay(day, m.calDate) {
				st = st.BorderForeground(blue)
			}
			cells = append(cells, st.Render(text))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		if row < 5 {
			b.WriteByte('\n')
		}
	}
	view := b.String()
	if m.calendarTasks {
		return overlayCentered(view, m.calendarTasksPopup(width))
	}
	return view
}
func (m model) weekView(width int) string {
	start := m.calDate.AddDate(0, 0, -(int(m.calDate.Weekday())+6)%7)
	days, title := 7, "Week of "
	if m.workingWeek {
		days, title = 5, "Working week of "
	}
	rows := (days + 1) / 2
	cellW := max(16, width/2)
	detail := m.calendarSelection(width)
	cellH := max(5, (m.height-5-lipgloss.Height(detail))/rows)
	var b strings.Builder
	b.WriteString(titleStyle.Render(title+start.Format("02 January 2006")) + dim.Render("  arrows day  enter tasks  [/] week  v view") + "\n")
	if detail != "" {
		b.WriteString(detail + "\n")
	}
	for row := 0; row < rows; row++ {
		var cells []string
		for col := 0; col < 2; col++ {
			i := row*2 + col
			if i >= days {
				cells = append(cells, lipgloss.NewStyle().Width(cellW).Render(""))
				continue
			}
			d := start.AddDate(0, 0, i)
			lines := []string{titleStyle.Render(d.Format("Monday 02"))}
			for j, task := range m.tasksDueOn(d) {
				active := m.calendarTasks && sameDay(d, m.calDate) && j == m.cursor
				lines = append(lines, m.calendarTaskLines(task, cellW-4, active)...)
			}
			st := lipgloss.NewStyle().Width(cellW - 2).MaxWidth(cellW).Height(cellH - 2).MaxHeight(cellH).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#30363d"))
			if sameDay(d, m.calDate) {
				st = st.BorderForeground(blue)
			}
			cells = append(cells, st.Render(strings.Join(lines, "\n")))
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
		if row < rows-1 {
			b.WriteByte('\n')
		}
	}
	view := b.String()
	if m.calendarTasks {
		return overlayCentered(view, m.calendarTasksPopup(width))
	}
	return view
}

func (m model) tasksDueOn(day time.Time) []task {
	var out []task
	for _, t := range m.tasks {
		if t.completed() != m.showCompleted {
			continue
		}
		if !m.matchesSelectedProject(t) {
			continue
		}
		if d, ok := t.due(); ok && sameDay(d, day) {
			out = append(out, t)
		}
	}
	return out
}

func (m model) calendarSelection(width int) string {
	return dim.Render(m.calDate.Format("Monday, 02 January 2006"))
}

func (m model) calendarTasksPopup(width int) string {
	tasks := m.tasksDueOn(m.calDate)
	popupW := min(64, max(32, width*2/3))
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.calDate.Format("Monday, 02 January")) + "\n\n")
	for i, task := range tasks {
		b.WriteString(strings.Join(m.calendarTaskLines(task, popupW-4, i == m.cursor), "\n"))
		if i < len(tasks)-1 {
			b.WriteString("\n\n")
		}
	}
	b.WriteString("\n\n" + dim.Render("↑/↓ task  esc close"))
	return panel.Width(popupW - 2).Render(b.String())
}

func overlayCentered(base, popup string) string {
	baseLines, popupLines := strings.Split(base, "\n"), strings.Split(popup, "\n")
	baseW, popupW := lipgloss.Width(base), lipgloss.Width(popup)
	x, y := max(0, (baseW-popupW)/2), max(0, (len(baseLines)-len(popupLines))/2)
	for i, line := range popupLines {
		row := y + i
		if row >= len(baseLines) {
			break
		}
		left := ansi.Cut(baseLines[row], 0, x)
		right := ansi.Cut(baseLines[row], x+popupW, baseW)
		baseLines[row] = left + lipgloss.NewStyle().Width(popupW).Render(line) + right
	}
	return strings.Join(baseLines, "\n")
}

func (m model) calendarTaskLines(t task, width int, active bool) []string {
	title := truncate(t.Name, width)
	var tags []string
	for _, tag := range t.Tags {
		name := "#" + tag.Name
		if !active && tag.Color != "" {
			name = lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color)).Render(name)
		}
		if lipgloss.Width(strings.Join(append(tags, name), " ")) > width {
			break
		}
		tags = append(tags, name)
	}
	metadata := strings.Join(tags, " ")
	if active {
		style := selected.Width(width)
		return []string{style.Render(title), style.Render(metadata)}
	}
	return []string{title, metadata}
}

func (m model) editorView() string {
	e := m.editor
	if e.calendar {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Width(64).Render(m.miniCalendarView()))
	}
	if e.colorPicker {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Width(64).Render(m.colorPickerView()))
	}
	height := 24
	if e.kind == editTag {
		height = 18
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Width(64).Render(m.editorContent(60, height)))
}

func (m model) editorContent(width, height int) string {
	e := m.editor
	names := map[editorKind][]string{editTask: {"Name", "Due date", "Project", "Tags"}, editProject: {"Name", "Color", "Status"}, editTag: {"Name", "Color"}}[e.kind]
	var b strings.Builder
	actionName := "Edit"
	if e.create {
		actionName = "New"
	}
	kinds := []string{"task", "project", "tag"}
	b.WriteString(titleStyle.Render(actionName+" "+kinds[e.kind]) + "\n")
	for i, f := range e.fields {
		f.Width = max(10, width-3)
		label, field := dim.Render(names[i]), f.View()
		if i == e.focus {
			label = lipgloss.NewStyle().Foreground(blue).Bold(true).Underline(true).Render(names[i])
		}
		b.WriteString("\n" + label + "\n" + field + "\n")
		if e.kind == editTask && e.choice != 0 && i == e.focus {
			b.WriteString(m.taskChoiceView(width) + "\n")
		}
		if e.kind == editTask && i == 3 {
			b.WriteString(m.tagPreview(f.Value()) + "\n")
		}
	}
	if e.kind != editTag {
		label := "Description"
		if e.kind == editTask {
			label = "Markdown description"
		}
		note := e.note
		note.SetWidth(max(10, width-2))
		note.SetHeight(max(3, height-20))
		labelView, noteView := dim.Render(label), note.View()
		if e.focus == len(e.fields) {
			labelView = lipgloss.NewStyle().Foreground(blue).Bold(true).Underline(true).Render(label)
		}
		b.WriteString("\n" + labelView + "\n" + noteView + "\n")
	}
	button := lipgloss.NewStyle().Background(lipgloss.Color("#1f6feb")).Foreground(lipgloss.Color("#ffffff")).Bold(true).Padding(0, 1)
	buttons := button.Render("Save  ctrl+s") + "  " + button.Render("Cancel  esc")
	body := lipgloss.NewStyle().Height(max(1, height-1)).MaxHeight(max(1, height-1)).Render(b.String())
	return body + "\n" + buttons
}

func (m model) taskChoiceView(width int) string {
	e := m.editor
	var options []string
	selected := map[string]bool{}
	if e.choice == 1 {
		options = append(options, "No project")
		for _, p := range m.projects {
			options = append(options, p.Name)
		}
	} else {
		selected = selectedNames(e.fields[3].Value())
		for _, tag := range m.tags {
			options = append(options, tag.Name)
		}
	}
	start := max(0, e.choiceIndex-4)
	end := min(len(options), start+6)
	var lines []string
	for i := start; i < end; i++ {
		mark := "  "
		if i == e.choiceIndex {
			mark = "> "
		}
		checked := ""
		if e.choice == 2 {
			checked = "[ ] "
			if selected[strings.ToLower(options[i])] {
				checked = "[x] "
			}
		}
		name := truncate(options[i], width-lipgloss.Width(mark+checked))
		color := ""
		if e.choice == 1 && i > 0 {
			color = m.projects[i-1].Color
		} else if e.choice == 2 {
			color = m.tags[i].Color
		}
		if color != "" {
			name = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(name)
		}
		lines = append(lines, mark+checked+name)
	}
	return panel.Width(width - 2).Render(strings.Join(lines, "\n"))
}

func (m model) tagPreview(value string) string {
	var parts []string
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		style := dim
		for _, tag := range m.tags {
			if strings.EqualFold(tag.Name, name) && tag.Color != "" {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
				break
			}
		}
		parts = append(parts, style.Render("#"+name))
	}
	return strings.Join(parts, "  ")
}

func (m model) colorPickerView() string {
	e := m.editor
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose a color") + "\n\n")
	custom := " Custom  " + e.fields[1].Value() + " "
	if validHexColor(e.fields[1].Value()) {
		custom = lipgloss.NewStyle().Background(lipgloss.Color(e.fields[1].Value())).Foreground(lipgloss.Color("#ffffff")).Bold(true).Render(custom)
	} else {
		custom = lipgloss.NewStyle().Foreground(muted).Render(custom)
	}
	if e.colorIndex == 0 {
		custom = "› " + custom
	} else {
		custom = "  " + custom
	}
	b.WriteString(custom + "\n\n")
	const columns = 6
	for i, color := range colorPalette {
		swatch := lipgloss.NewStyle().Background(lipgloss.Color(color)).Render("      ")
		if i+1 == e.colorIndex {
			swatch = "[" + swatch + "]"
		} else {
			swatch = " " + swatch + " "
		}
		b.WriteString(swatch + " ")
		if i%columns == columns-1 {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n" + dim.Render("type custom  ↑/↓ palette  arrows choose  enter apply  esc cancel"))
	return b.String()
}
func (m model) miniCalendarView() string {
	e := m.editor
	first := time.Date(e.date.Year(), e.date.Month(), 1, 0, 0, 0, 0, e.date.Location())
	start := first.AddDate(0, 0, -(int(first.Weekday())+6)%7)
	var b strings.Builder
	b.WriteString(titleStyle.Render(e.date.Format("January 2006")) + "\n Mo Tu We Th Fr Sa Su\n")
	for i := 0; i < 42; i++ {
		d := start.AddDate(0, 0, i)
		s := fmt.Sprintf("%2d", d.Day())
		if sameDay(d, e.date) {
			s = selected.Render(s)
		} else if d.Month() != e.date.Month() {
			s = dim.Render(s)
		}
		b.WriteString(" " + s)
		if i%7 == 6 {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\n←↑↓→ day  [ ] month  enter choose  delete clear")
	return b.String()
}
func (m model) helpView() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel.Width(60).Render(titleStyle.Render("Keyboard")+"\n\n↑/↓ or j/k  move\n←/→ or tab  switch sidebar/content\ncalendar arrows  select day/task\ncalendar enter   browse day's tasks\n[/]              previous/next calendar period\nenter            open sidebar item\nn/e/d            new/edit/delete\nspace            toggle task completion\nh                active/completed tasks\nv                calendar view\nr                refresh\n?                close help\nq                quit"))
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
func truncate(s string, n int) string {
	if n < 2 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
