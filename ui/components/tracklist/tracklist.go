package tracklist

import (
	"strings"

	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/ui/model"
	"github.com/wellbou/wellya/ui/style"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Control uint

const (
	PLAY Control = iota
	CURSOR_UP
	CURSOR_DOWN
	PAGE_UP
	PAGE_DOWN
	SEARCH
	SHUFFLE
	SHARE
	LIKE
	ADD_TO_PLAYLIST
	REMOVE_FROM_PLAYLIST
	BACK
	TOGGLE_VIEW
	MOVE_UP
	MOVE_DOWN
	JUMP_TO_PLAYING
	ARTIST_BROWSE
	SHOW_QUEUE
	TRACK_INFO
	GO_TO_ALBUM
	DISLIKE
	SORT
	FILTER
	REMOVE_FROM_QUEUE
	EXPORT
	UPLOAD
	STATS
	PLAY_NEXT
)

type Model struct {
	program       *tea.Program
	list          list.Model
	help          help.Model
	helpMap       *helpKeyMap
	filterInput   textinput.Model
	allItems      []Item
	width, height int
	Hidden        bool
	Title         string
	Shufflable    bool
}

func New(p *tea.Program, likesMap *map[string]bool, cacheMap *map[string]bool) *Model {
	m := &Model{
		program: p,
		help:    help.New(),
		helpMap: newHelpMap(),
		Title:   "Tracks",
	}

	controls := config.Current.Controls

	m.list = list.New([]list.Item{}, ItemDelegate{likesMap: likesMap, cacheMap: cacheMap}, 512, 512)
	m.list.Styles.Title = style.TrackListTitleStyle
	m.list.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(controls.CursorUp.Binding(), controls.CursorUp.Help("up")),
		CursorDown: key.NewBinding(controls.CursorDown.Binding(), controls.CursorDown.Help("down")),
		NextPage:   key.NewBinding(controls.TracksPrevPage.Binding(), controls.TracksPrevPage.Help("next page")),
		PrevPage:   key.NewBinding(controls.TracksNextPage.Binding(), controls.TracksNextPage.Help("prev page")),
	}
	m.list.Paginator.KeyMap.NextPage.SetEnabled(false)
	m.list.Paginator.KeyMap.PrevPage.SetEnabled(false)
	m.list.SetShowHelp(false)

	m.filterInput = textinput.New()
	m.filterInput.Placeholder = "Filter playlist... (esc to clear)"
	m.filterInput.CharLimit = 64
	m.filterInput.Width = 32

	m.help.Ellipsis = "…"
	m.help.Styles.FullDesc = m.help.Styles.FullDesc.PaddingRight(1)

	return m
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) View() string {
	if m.Hidden {
		return ""
	}

	titleLen := lipgloss.Width(m.Title)
	if titleLen > m.width-8 {
		m.list.Title = lipgloss.NewStyle().MaxWidth(m.width-9).Render(m.Title) + "…"
	} else {
		m.list.Title = m.Title
	}

	m.helpMap.Shufflable = m.Shufflable
	helpView := m.help.View(m.helpMap)
	filterView := m.filterInput.View()
	filterHeight := 1
	if lipgloss.Width(filterView) > 0 {
		filterHeight = 2
	}
	m.list.SetHeight(m.height - lipgloss.Height(helpView) - filterHeight - 4)

	listView := m.list.View()
	if lipgloss.Height(listView) <= m.list.Height() {
		lastLine := strings.LastIndex(listView[:len(listView)-1], "\n")
		listView = listView[:lastLine] + "\n" + listView[lastLine:]
	}

	filterBox := style.TrackBoxStyle.Width(m.width - 4).Render(filterView)
	return style.TrackBoxStyle.Width(m.width).Render(lipgloss.JoinVertical(lipgloss.Left, filterBox, listView, "", helpView))
}

func (m *Model) FilterValue() string {
	return m.filterInput.Value()
}

func (m *Model) SetFilterValue(v string) {
	m.filterInput.SetValue(v)
	m.applyFilter()
}

func (m *Model) applyFilter() {
	val := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if val == "" {
		newItems := make([]list.Item, len(m.allItems))
		for i := range m.allItems {
			newItems[i] = m.allItems[i]
		}
		m.list.SetItems(newItems)
		return
	}
	filtered := []list.Item{}
	for _, it := range m.allItems {
		if strings.Contains(strings.ToLower(it.FilterValue()), val) {
			filtered = append(filtered, it)
		}
	}
	m.list.SetItems(filtered)
}

func (m *Model) Update(message tea.Msg) (*Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := message.(type) {
	case tea.KeyMsg:
		controls := config.Current.Controls
		keypress := msg.String()

		if m.filterInput.Focused() {
			switch keypress {
			case "esc":
				m.filterInput.SetValue("")
				m.filterInput.Blur()
				m.applyFilter()
				return m, nil
			case "enter":
				m.filterInput.Blur()
				return m, nil
			}
			var c tea.Cmd
			m.filterInput, c = m.filterInput.Update(msg)
			cmds = append(cmds, c)
			m.applyFilter()
			return m, tea.Batch(cmds...)
		}

		if controls.TracksFilter.Contains(keypress) {
			m.filterInput.Focus()
			return m, textinput.Blink
		}
		if len(keypress) == 1 && keypress[0] >= 32 && keypress[0] <= 126 && !controls.ShowAllKeys.Contains(keypress) {
			m.filterInput.Focus()
			m.filterInput.SetValue(m.filterInput.Value() + keypress)
			m.applyFilter()
			return m, textinput.Blink
		}

		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		switch {
		case controls.ShowAllKeys.Contains(keypress):
			m.help.ShowAll = !m.help.ShowAll
		case controls.Apply.Contains(keypress):
			cmds = append(cmds, model.Cmd(PLAY))
		case controls.CursorUp.Contains(keypress):
			cmds = append(cmds, model.Cmd(CURSOR_UP))
		case controls.CursorDown.Contains(keypress):
			cmds = append(cmds, model.Cmd(CURSOR_DOWN))
		case controls.TracksNextPage.Contains(keypress):
			pageSize := m.list.Height() / 3
			if pageSize < 1 {
				pageSize = 1
			}
			newIdx := m.list.Index() - pageSize
			if newIdx < 0 {
				newIdx = 0
			}
			m.list.Select(newIdx)
			cmds = append(cmds, model.Cmd(PAGE_UP))
		case controls.TracksPrevPage.Contains(keypress):
			pageSize := m.list.Height() / 3
			if pageSize < 1 {
				pageSize = 1
			}
			newIdx := m.list.Index() + pageSize
			if newIdx >= len(m.list.Items()) {
				newIdx = len(m.list.Items()) - 1
			}
			m.list.Select(newIdx)
			cmds = append(cmds, model.Cmd(PAGE_DOWN))
		case controls.TracksSearch.Contains(keypress):
			cmds = append(cmds, model.Cmd(SEARCH))
		case controls.TracksShuffle.Contains(keypress):
			cmds = append(cmds, model.Cmd(SHUFFLE))
		case controls.TracksShare.Contains(keypress):
			cmds = append(cmds, model.Cmd(SHARE))
		case controls.TracksLike.Contains(keypress):
			cmds = append(cmds, model.Cmd(LIKE))
		case controls.TracksAddToPlaylist.Contains(keypress):
			cmds = append(cmds, model.Cmd(ADD_TO_PLAYLIST))
		case controls.TracksRemoveFromPlaylist.Contains(keypress):
			cmds = append(cmds, model.Cmd(REMOVE_FROM_PLAYLIST))
		case controls.TracksBack.Contains(keypress):
			cmds = append(cmds, model.Cmd(BACK))
		case controls.TracksHide.Contains(keypress):
			m.Hidden = !m.Hidden
			cmds = append(cmds, model.Cmd(TOGGLE_VIEW))
		case controls.TracksMoveUp.Contains(keypress):
			cmds = append(cmds, model.Cmd(MOVE_UP))
		case controls.TracksMoveDown.Contains(keypress):
			cmds = append(cmds, model.Cmd(MOVE_DOWN))
		case controls.TracksJumpToPlaying.Contains(keypress):
			cmds = append(cmds, model.Cmd(JUMP_TO_PLAYING))
		case controls.TracksArtistBrowse.Contains(keypress):
			cmds = append(cmds, model.Cmd(ARTIST_BROWSE))
		case controls.TracksShowQueue.Contains(keypress):
			cmds = append(cmds, model.Cmd(SHOW_QUEUE))
		case controls.TracksInfo.Contains(keypress):
			cmds = append(cmds, model.Cmd(TRACK_INFO))
		case controls.TracksGoToAlbum.Contains(keypress):
			cmds = append(cmds, model.Cmd(GO_TO_ALBUM))
		case controls.TracksDislike.Contains(keypress):
			cmds = append(cmds, model.Cmd(DISLIKE))
		case controls.TracksSort.Contains(keypress):
			cmds = append(cmds, model.Cmd(SORT))
		case controls.TracksRemoveFromQueue.Contains(keypress):
			cmds = append(cmds, model.Cmd(REMOVE_FROM_QUEUE))
		case controls.TracksExport.Contains(keypress):
			cmds = append(cmds, model.Cmd(EXPORT))
		case controls.TracksUpload.Contains(keypress):
			cmds = append(cmds, model.Cmd(UPLOAD))
		case controls.TracksStats.Contains(keypress):
			cmds = append(cmds, model.Cmd(STATS))
		case controls.TracksPlayNext.Contains(keypress):
			cmds = append(cmds, model.Cmd(PLAY_NEXT))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) Items() []Item {
	litems := m.list.Items()
	items := make([]Item, len(litems))
	for i := range litems {
		items[i] = litems[i].(Item)
	}
	return items
}

func (m *Model) SetItems(items []Item) tea.Cmd {
	m.allItems = make([]Item, len(items))
	copy(m.allItems, items)
	m.applyFilter()
	return nil
}

func (m *Model) InsertItem(index int, item Item) tea.Cmd {
	if index < 0 {
		index = len(m.list.Items()) + 1
	}
	return m.list.InsertItem(index, item)
}

func (m *Model) RemoveItem(index int) {
	m.list.RemoveItem(index)
}

func (m *Model) RemoveItemAfter(index int) tea.Cmd {
	items := m.list.Items()[:index+1]
	return m.list.SetItems(items)
}

func (m *Model) SetItem(index int, item Item) tea.Cmd {
	return m.list.SetItem(index, item)
}

func (m *Model) SelectedItem() Item {
	return m.list.SelectedItem().(Item)
}

func (m *Model) Index() int {
	return m.list.Index()
}

func (m *Model) Select(index int) {
	m.list.Select(index)
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.help.Width = m.width - 4
	m.list.SetWidth(m.width - 6)
	m.filterInput.Width = m.width - 8
	m.SetHeight(h)
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.help.Width = m.width - 4
	m.list.SetWidth(m.width - 6)
}

func (m *Model) Width() int {
	return m.width
}

func (m *Model) SetHeight(h int) {
	m.height = h
}

func (m *Model) Height() int {
	return m.height
}
