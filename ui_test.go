package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestNumericTaskPriorityFromAPI(t *testing.T) {
	var got struct {
		Tasks []task `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(`{"tasks":[{"name":"test","priority":2,"status":0}]}`), &got); err != nil {
		t.Fatal(err)
	}
	if name := priorityName(got.Tasks[0].Priority); name != "high" {
		t.Fatalf("priority = %q, want high", name)
	}
}

func TestTaskViewsAndCompletedToggle(t *testing.T) {
	now := time.Now()
	today := now.Add(-time.Hour).Format(time.RFC3339)
	tomorrow := now.Add(24 * time.Hour).Format(time.RFC3339)
	m := model{page: pageToday, tasks: []task{
		{Name: "overdue", DueDate: today, Status: "not_started"},
		{Name: "later", DueDate: tomorrow, Status: "not_started"},
		{Name: "done", DueDate: today, Status: "done"},
		{Name: "floating", Status: "not_started"},
	}}
	if got := len(m.filteredTasks()); got != 1 {
		t.Fatalf("Today returned %d tasks, want 1", got)
	}
	m.showCompleted = true
	if got := m.filteredTasks(); len(got) != 1 || got[0].Name != "done" {
		t.Fatalf("completed Today tasks = %#v", got)
	}
	m.showCompleted = false
	m.page = pageUpcoming
	if got := len(m.filteredTasks()); got != 1 {
		t.Fatalf("Upcoming returned %d tasks, want 1", got)
	}
	m.page = pageUnplanned
	if got := len(m.filteredTasks()); got != 1 {
		t.Fatalf("Unplanned returned %d tasks, want 1", got)
	}
}

func TestDashboardStatsUseSelectedProject(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	m := model{
		selectedProjectUID: "one",
		projects:           []project{{ID: 1, UID: "one"}},
		tasks: []task{
			{ProjectID: 1, Status: "in_progress", Priority: 2, DueDate: "2026-09-24", CreatedAt: "2026-09-20T12:00:00Z", Tags: []tag{{Name: "work"}}},
			{ProjectID: 1, Status: "not_started", DueDate: "2026-09-23"},
			{ProjectID: 1, Status: "done", DueDate: "2026-09-24"},
			{ProjectID: 2, Status: "not_started"},
		},
	}
	got := m.dashboardStats(now)
	if got.total != 3 || got.inProgress != 1 || got.active != 2 || got.dueToday != 1 || got.overdue != 1 || got.completed != 1 {
		t.Fatalf("unexpected totals: %+v", got)
	}
	if got.priorities != [3]int{0, 1, 1} || got.ageDays != 4 || got.tags["work"] != 1 {
		t.Fatalf("dashboard breakdowns = %+v", got)
	}
}

func TestDashboardStatsCharts(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	m := model{tasks: []task{
		{Status: "not_started"},
		{Status: "not_started", DueDate: "2026-09-25", Tags: []tag{{Name: "work"}}},
		{Status: "done"},
	}}
	stats := m.dashboardStats(now)
	if stats.unplanned != 1 || stats.nextWeek[0] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	view := m.dashboardCharts(stats, 100)
	for _, want := range []string{"Completion & planning", "Active by priority", "Unplanned", "Fri", "#work"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard charts missing %q: %q", want, view)
		}
	}
}

func TestDashboardFitsCommonTerminal(t *testing.T) {
	m := model{page: pageDashboard, width: 160, height: 30}
	for i := 0; i < 8; i++ {
		m.tasks = append(m.tasks, task{Status: "not_started", Priority: i % 3, DueDate: time.Now().AddDate(0, 0, i).Format("2006-01-02"), Tags: []tag{{Name: fmt.Sprintf("tag-%d", i)}}})
	}
	view := m.View()
	if lipgloss.Width(view) > m.width || lipgloss.Height(view) > m.height {
		t.Fatalf("dashboard is %dx%d, terminal is %dx%d", lipgloss.Width(view), lipgloss.Height(view), m.width, m.height)
	}
}

func TestShiftTabFromMarkdownEditor(t *testing.T) {
	m := model{page: pageToday}
	m.openNew()
	m.editor.focus = len(m.editor.fields)
	m.editor.note.Focus()

	updated, _ := m.updateEditor(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := updated.(model)
	if want := len(got.editor.fields) - 1; got.editor.focus != want {
		t.Fatalf("focus = %d, want %d", got.editor.focus, want)
	}
}

func TestTaskListItemHasTitleMetadataAndDueDate(t *testing.T) {
	task := task{
		Name:    "Ship terminal client",
		DueDate: "2026-09-24T12:00:00Z",
		Tags:    []tag{{Name: "release", Color: "#00ff00"}},
	}
	got := (model{}).taskListItem(task, 60, false)
	if !strings.Contains(got, "Ship terminal client\n") || !strings.Contains(got, "#release") || !strings.Contains(got, "Sep 24") {
		t.Fatalf("unexpected task item: %q", got)
	}
}

func TestTaskPrioritySortAndFilters(t *testing.T) {
	m := model{page: pageAll, sortBy: "priority", sortDesc: true, priorityFilter: "high", tagFilters: map[string]bool{"work": true}, tasks: []task{
		{Name: "high work", Priority: "high", Tags: []tag{{Name: "work"}}},
		{Name: "high home", Priority: "high", Tags: []tag{{Name: "home"}}},
		{Name: "low work", Priority: "low", Tags: []tag{{Name: "work"}}},
	}}
	got := m.filteredTasks()
	if len(got) != 1 || got[0].Name != "high work" {
		t.Fatalf("filtered tasks = %#v", got)
	}
	if item := m.taskListItem(got[0], 60, false); !strings.HasSuffix(ansi.Strip(strings.Split(item, "\n")[1]), "H") {
		t.Fatalf("priority is not right aligned: %q", item)
	}
}

func TestLateTagIsShownOnlyForOverdueActiveTasks(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	m := model{}
	if got := m.taskListItem(task{Name: "late", DueDate: yesterday}, 60, false); !strings.Contains(got, "⚠  late") || lipgloss.Width(strings.Split(got, "\n")[1]) != 58 {
		t.Fatalf("late tag missing: %q", got)
	}
	if got := m.taskListItem(task{Name: "done", DueDate: yesterday, Status: "done"}, 60, false); strings.Contains(got, "⚠  late") {
		t.Fatalf("completed task marked late: %q", got)
	}
}

func TestCompletedTaskTitleIsStruckAndDimmed(t *testing.T) {
	got := (model{}).taskListItem(task{Name: "finished", Status: "done"}, 60, false)
	want := lipgloss.NewStyle().Foreground(muted).Faint(true).Strikethrough(true).Render("✓ finished")
	if !strings.Contains(got, want) {
		t.Fatalf("completed task is not dimmed and struck: %q", got)
	}
}

func TestSelectedCompletedTaskKeepsFullHighlight(t *testing.T) {
	got := (model{}).taskListItem(task{Name: "finished", Status: "done"}, 60, true)
	for i, line := range strings.Split(got, "\n") {
		if lipgloss.Width(line) != 58 {
			t.Fatalf("line %d width = %d, want 58: %q", i, lipgloss.Width(line), line)
		}
	}
}

func TestTaskHeadingCountsCombinedFilters(t *testing.T) {
	m := model{
		page:               pageUnplanned,
		selectedProjectUID: "one",
		projects:           []project{{ID: 1, UID: "one", Name: "One"}},
		tasks:              []task{{Name: "matching", ProjectID: 1}, {Name: "other", ProjectID: 2}},
	}
	if got := m.contentView(80); !strings.Contains(got, "Unplanned (1)") {
		t.Fatalf("filtered task count missing from heading: %q", got)
	}
}

func TestMarkdownDetailDoesNotChangeContentHeight(t *testing.T) {
	m := model{
		page:   pageUnplanned,
		width:  160,
		height: 30,
		tasks: []task{
			{Name: "short"},
			{Name: "markdown", Note: strings.Repeat("## Heading\n\nParagraph\n\n", 20)},
		},
	}
	m.cursor = 0
	shortHeight := lipgloss.Height(m.contentView(130))
	lines := strings.Split(ansi.Strip(m.contentView(130)), "\n")
	if bottom := lines[len(lines)-1]; !strings.Contains(bottom, "╰") || !strings.Contains(bottom, "╯") {
		t.Fatalf("task detail bottom border is not closed: %q", bottom)
	}
	m.cursor = 1
	markdownHeight := lipgloss.Height(m.contentView(130))
	if shortHeight != markdownHeight {
		t.Fatalf("content height changed from %d to %d", shortHeight, markdownHeight)
	}
	if got := lipgloss.Height(m.View()); got > m.height {
		sidebar := panel.Width(22).Height(m.height - 3).Render(m.sidebarView())
		content := panel.Width(133).Height(m.height - 3).Render(m.contentView(131))
		t.Fatalf("view height = %d, content view = %dx%d, content panel = %dx%d, sidebar = %d, terminal = %d", got, lipgloss.Width(m.contentView(130)), lipgloss.Height(m.contentView(130)), lipgloss.Width(content), lipgloss.Height(content), lipgloss.Height(sidebar), m.height)
	}
}

func TestLongTaskListKeepsCursorVisibleWithoutOverflow(t *testing.T) {
	m := model{page: pageUnplanned, width: 160, height: 30}
	for i := 0; i < 30; i++ {
		m.tasks = append(m.tasks, task{Name: fmt.Sprintf("task %02d", i)})
	}
	m.cursor = len(m.tasks) - 1
	view := m.View()
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("view height = %d, terminal height = %d", got, m.height)
	}
	if !strings.Contains(view, "task 29") {
		t.Fatal("selected last task is not visible")
	}
}

func TestProjectSelectionCombinesWithTaskFilterAndAllClears(t *testing.T) {
	now := time.Now().Add(-time.Hour).Format(time.RFC3339)
	m := model{
		page:               pageToday,
		focusSide:          true,
		side:               sideFirstProject,
		selectedProjectUID: "one",
		projects:           []project{{ID: 1, UID: "one"}, {ID: 2, UID: "two"}},
		tasks:              []task{{Name: "one", ProjectID: 1, DueDate: now}, {Name: "two", ProjectID: 2, DueDate: now}},
	}
	if got := m.filteredTasks(); len(got) != 1 || got[0].Name != "one" {
		t.Fatalf("filtered tasks = %#v", got)
	}
	m.chooseSide()
	if m.selectedProjectUID != "one" || !m.focusSide {
		t.Fatal("selecting a project again should keep it selected and focused")
	}
	m.side = sideAllProjects
	m.chooseSide()
	if m.selectedProjectUID != "" || len(m.filteredTasks()) != 2 || !m.focusSide {
		t.Fatal("All projects should clear the project filter and keep sidebar focus")
	}
}

func TestFilterSelectionAndAll(t *testing.T) {
	m := model{
		page:      pageToday,
		focusSide: true,
		side:      sideToday,
		tasks: []task{
			{Name: "dated", DueDate: time.Now().Format("2006-01-02")},
			{Name: "unplanned"},
		},
	}
	if len(m.filteredTasks()) != 1 {
		t.Fatal("today filter should initially be active")
	}
	m.chooseSide()
	if m.page != pageToday || len(m.filteredTasks()) != 1 || !m.focusSide {
		t.Fatal("selecting Today again should keep the filter and sidebar focus")
	}
	m.side = sideAllFilters
	m.chooseSide()
	if m.page != pageAll || len(m.filteredTasks()) != 2 {
		t.Fatal("All filters should show every task")
	}
	if m.persistedState().Filter != "all" {
		t.Fatal("cleared filter should be persisted")
	}
}

func TestDashboardSelectionIsPersisted(t *testing.T) {
	m := model{side: sideDashboard}
	if m.chooseSide() == nil || m.persistedState().Filter != "dashboard" {
		t.Fatal("dashboard selection should be persisted")
	}
	if got := newModel(nil, m.persistedState()); got.page != pageDashboard || got.side != sideDashboard {
		t.Fatalf("dashboard state restored as page=%v side=%v", got.page, got.side)
	}
}

func TestCalendarViewsStayInsideTerminal(t *testing.T) {
	for _, mode := range []struct{ week, working bool }{{}, {week: true}, {week: true, working: true}} {
		m := model{page: pageCalendar, width: 160, height: 30, week: mode.week, workingWeek: mode.working, calDate: time.Now()}
		view := m.View()
		if got := lipgloss.Height(view); got > m.height {
			t.Fatalf("mode=%+v: height = %d, terminal = %d", mode, got, m.height)
		}
		if got := lipgloss.Width(view); got > m.width {
			t.Fatalf("mode=%+v: width = %d, terminal = %d", mode, got, m.width)
		}
	}
}

func TestCalendarNavigationAndTaskBrowsing(t *testing.T) {
	day := time.Now()
	due := day.Format(time.RFC3339)
	m := model{page: pageCalendar, width: 160, height: 30, calDate: day, tasks: []task{
		{Name: "first task", DueDate: due, Tags: []tag{{Name: "home", Color: "#00ff00"}}},
		{Name: "the complete second task name", DueDate: due, Tags: []tag{{Name: "work", Color: "#ff0000"}}},
	}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.calendarTasks {
		t.Fatal("enter should focus the selected day's tasks")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	popup := m.calendarView(130)
	if m.cursor != 1 || !strings.Contains(popup, "first task") || !strings.Contains(popup, "the complete second task name") || !strings.Contains(popup, "#home") || !strings.Contains(popup, "#work") || !strings.Contains(popup, day.Format("January 2006")) {
		t.Fatal("calendar task navigation did not expose the full task name")
	}

	m.calendarTasks = false
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := updated.(model).calDate; !sameDay(got, day.AddDate(0, 0, 1)) {
		t.Fatal("right should select the next calendar day")
	}
}

func TestOpeningCalendarFocusesDayNavigation(t *testing.T) {
	m := model{focusSide: true, side: sideCalendar, calDate: time.Now()}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.page != pageCalendar || m.focusSide {
		t.Fatal("opening Calendar should focus its day grid")
	}
	day := m.calDate
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := updated.(model).calDate; !sameDay(got, day.AddDate(0, 0, 1)) {
		t.Fatal("right should move to the next day immediately after opening Calendar")
	}
}

func TestCalendarViewCycleIncludesWorkingWeek(t *testing.T) {
	m := model{page: pageCalendar}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(model)
	if !m.week || m.workingWeek {
		t.Fatal("first v should select week view")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(model)
	if !m.week || !m.workingWeek {
		t.Fatal("second v should select working-week view")
	}
}

func TestCalendarArrowsMoveBetweenVisibleBlocks(t *testing.T) {
	monday := time.Date(2026, 9, 21, 0, 0, 0, 0, time.Local)
	m := model{page: pageCalendar, calDate: monday}
	m.move(1)
	if want := monday.AddDate(0, 0, 7); !sameDay(m.calDate, want) {
		t.Fatalf("month down = %v, want %v", m.calDate, want)
	}

	m.week, m.calDate = true, monday
	m.move(1)
	if want := monday.AddDate(0, 0, 2); !sameDay(m.calDate, want) {
		t.Fatalf("week down = %v, want %v", m.calDate, want)
	}

	m.workingWeek = true
	m.calDate = time.Date(2026, 9, 25, 0, 0, 0, 0, time.Local)
	m.moveCalendarDays(1)
	if m.calDate.Weekday() != time.Monday || m.calDate.Day() != 28 {
		t.Fatalf("working-week right did not cross to Monday: %v", m.calDate)
	}
}

func TestMonthViewShowsSelectedDayClearly(t *testing.T) {
	day := time.Date(2026, 9, 23, 0, 0, 0, 0, time.Local)
	m := model{page: pageCalendar, width: 160, height: 30, calDate: day}
	view := m.calendarView(130)
	if !strings.Contains(view, "Wednesday, 23 September 2026") {
		t.Fatal("month view does not show the selected date")
	}
	want := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(blue).Bold(true).Render("23")
	if !strings.Contains(view, want) {
		t.Fatal("month view does not visibly highlight the selected day")
	}
}

func TestColorPickerAppliesSelectedColor(t *testing.T) {
	m := model{editor: &editor{kind: editTag, focus: 1, fields: []textinput.Model{input("tag", "Name"), input("#3b82f6", "Color")}}}
	updated, _ := m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.editor.colorPicker {
		t.Fatal("enter on the color field should open the picker")
	}
	m.editor.colorIndex = 6
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got := m.editor.fields[1].Value(); got != colorPalette[5] {
		t.Fatalf("color = %q, want %q", got, colorPalette[5])
	}
}

func TestColorPickerCustomInputUpdatesLive(t *testing.T) {
	color := input("", "Color")
	color.Focus()
	m := model{editor: &editor{kind: editTag, focus: 1, colorPicker: true, fields: []textinput.Model{input("tag", "Name"), color}}}
	for _, r := range "#123abc" {
		updated, _ := m.updateColorPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	if got := m.editor.fields[1].Value(); got != "#123abc" || !validHexColor(got) {
		t.Fatalf("custom color = %q", got)
	}
	if !strings.Contains(m.colorPickerView(), "#123abc") {
		t.Fatal("custom color preview was not updated")
	}
}

func TestSelectedTagUsesOneContinuousHighlight(t *testing.T) {
	tg := tag{Name: "Documentation", Color: "#15803d"}
	m := model{page: pageTags, tags: []tag{tg}, cursor: 0}
	want := selected.Width(46).Render(fmt.Sprintf("●  %-28s %s", tg.Name, tg.Color))
	if got := m.tagsView(); !strings.Contains(got, want) {
		t.Fatalf("selected tag is not rendered as one highlighted block: %q", got)
	}
}

func TestFooterKeyCapsStayOnOneLine(t *testing.T) {
	footer := (model{width: 80}).footerView()
	if lipgloss.Height(footer) != 1 || lipgloss.Width(footer) > 80 {
		t.Fatalf("footer size = %dx%d", lipgloss.Width(footer), lipgloss.Height(footer))
	}
	if !strings.Contains(footer, "│") {
		t.Fatal("footer shortcuts are not framed")
	}
}

func TestEnterEditsSelectedTag(t *testing.T) {
	m := model{page: pageTags, tags: []tag{{UID: "tag-1", Name: "Docs"}}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if got.editor == nil || got.editor.kind != editTag || got.editor.tag.UID != "tag-1" {
		t.Fatal("enter should edit the selected tag")
	}
}

func TestEditorArrowKeysMoveFocus(t *testing.T) {
	m := model{page: pageToday}
	m.openNew()

	updated, _ := m.updateEditor(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	if m.editor.focus != 1 {
		t.Fatalf("down focus = %d, want 1", m.editor.focus)
	}

	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(model)
	if m.editor.focus != 0 {
		t.Fatalf("up focus = %d, want 0", m.editor.focus)
	}

	m.editor.focus = len(m.editor.fields)
	m.editor.note.Focus()
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyUp})
	if got := updated.(model).editor.focus; got != len(m.editor.fields) {
		t.Fatalf("up in markdown moved focus to %d", got)
	}
}

func TestTaskEditorPickers(t *testing.T) {
	m := model{projects: []project{{Name: "Work", Color: "#ff0000"}}, tags: []tag{{Name: "urgent", Color: "#00ff00"}}}
	m.openNew()

	m.editor.focus = 2
	updated, _ := m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.editor.calendar {
		t.Fatal("enter on due date should open the calendar")
	}
	m.editor.calendar = false

	m.editor.focus = 3
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if want := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("Work"); !strings.Contains(m.taskChoiceView(40), want) {
		t.Fatal("project picker did not use the project color")
	}
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got := m.editor.fields[3].Value(); got != "Work" {
		t.Fatalf("project = %q", got)
	}

	m.editor.focus = 4
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if want := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render("urgent"); !strings.Contains(m.taskChoiceView(40), want) {
		t.Fatal("tag picker did not use the tag color")
	}
	updated, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got := m.editor.fields[4].Value(); got != "urgent" {
		t.Fatalf("tags = %q", got)
	}
}

func TestLongTaskDescriptionKeepsEditorButtonsVisible(t *testing.T) {
	m := model{page: pageAll, width: 160, height: 30}
	m.openNew()
	m.editor.note.SetValue(strings.Repeat("long description\n", 100))
	view := m.View()
	if !strings.Contains(view, "Save  ctrl+s") || !strings.Contains(view, "Cancel  esc") {
		t.Fatal("fixed editor buttons are not visible")
	}
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("view height = %d, terminal = %d", got, m.height)
	}
}

func TestHorizontalArrowsSwitchMainFocus(t *testing.T) {
	m := model{focusSide: true}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(model)
	if m.focusSide {
		t.Fatal("right should focus the main content")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if !updated.(model).focusSide {
		t.Fatal("left should focus the sidebar")
	}
}

func TestTaskEditorRendersInRightSidebar(t *testing.T) {
	m := model{page: pageToday, width: 160, height: 40, tasks: []task{{Name: "Existing task"}}}
	m.openNew()
	view := m.View()
	if !strings.Contains(view, "Today") || !strings.Contains(view, "New task") {
		t.Fatalf("task editor did not render beside the main view: %q", view)
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("view width = %d, terminal = %d", got, m.width)
	}
}
