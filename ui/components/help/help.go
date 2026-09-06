package help

import (
	"strings"

	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/ui/style"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	width, height int
	visible       bool
}

func New() *Model {
	return &Model{}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Show() { m.visible = true }
func (m *Model) Hide() { m.visible = false }
func (m *Model) Visible() bool { return m.visible }
func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.visible = false
	}
	return m, nil
}

type entry struct {
	key  string
	desc string
}

func keyName(c *config.Key, fallback string) string {
	if c == nil || c.IsEmpty() {
		return fallback
	}
	return c.Display()
}

func (m *Model) View() string {
	if !m.visible {
		return ""
	}

	c := config.Current.Controls
	rewind := int(config.Current.RewindDuration)

	groups := []struct {
		title string
		items []entry
	}{
		{"Global", []entry{
			{keyName(c.Quit, "?"), "Quit"},
			{keyName(c.Cancel, "esc"), "Cancel / close dialog"},
			{keyName(c.ShowAllKeys, "?"), "Toggle bottom help bar"},
			{keyName(c.Reload, "ctrl+\\"), "Reload config + reinit"},
			{keyName(c.KeysHelp, "f1"), "Open this help modal"},
		}},
		{"Tabs", []entry{
			{keyName(c.PlaylistsRadio, "R"), "Toggle Playlists / Radio tab"},
			{keyName(c.TracksSearchTab, "S"), "Toggle Search tab"},
			{keyName(c.PlaylistsHide, "ctrl+b"), "Hide sidebar"},
			{keyName(c.TracksHide, "ctrl+t"), "Hide tracklist"},
		}},
		{"Sidebar", []entry{
			{keyName(c.PlaylistsUp, "ctrl+↑"), "Move up in sidebar"},
			{keyName(c.PlaylistsDown, "ctrl+↓"), "Move down in sidebar"},
			{keyName(c.PlaylistsRename, "ctrl+r"), "Rename selected playlist"},
		}},
		{"Player", []entry{
			{keyName(c.PlayerPause, "space"), "Play / pause"},
			{keyName(c.PlayerNext, "→"), "Next track"},
			{keyName(c.PlayerPrevious, "←"), "Previous track"},
			{keyName(c.PlayerRewindForward, "ctrl+→"), "Forward " + itoa(rewind) + "s"},
			{keyName(c.PlayerRewindBackward, "ctrl+←"), "Backward " + itoa(rewind) + "s"},
			{keyName(c.PlayerVolUp, "+/="), "Volume up"},
			{keyName(c.PlayerVolDown, "-"), "Volume down"},
			{keyName(c.PlayerMute, "m"), "Mute / unmute"},
			{keyName(c.PlayerLike, "L"), "Like / unlike current"},
			{keyName(c.PlayerDislike, "D"), "Dislike current"},
			{keyName(c.PlayerCache, "S"), "Cache current track"},
			{keyName(c.PlayerCacheAllLiked, "C"), "Cache all liked tracks"},
			{keyName(c.PlayerDownload, "ctrl+w"), "Download current track"},
			{keyName(c.PlayerQualityCycle, "q"), "Cycle audio quality"},
			{keyName(c.PlayerRepeatMode, "r"), "Repeat mode (off / all / 1)"},
			{keyName(c.PlayerSleepTimer, "n"), "Sleep timer"},
			{keyName(c.PlayerToggleLyrics, "t"), "Toggle synced lyrics"},
			{keyName(c.PlayerHide, "ctrl+p"), "Hide player"},
		}},
		{"Tracklist", []entry{
			{keyName(c.CursorUp, "↑"), "Move cursor up"},
			{keyName(c.CursorDown, "↓"), "Move cursor down"},
			{keyName(c.TracksNextPage, "pgup"), "Page up"},
			{keyName(c.TracksPrevPage, "pgdn"), "Page down"},
			{keyName(c.Apply, "enter"), "Play / pause current"},
			{keyName(c.TracksLike, "l"), "Like selected track"},
			{keyName(c.TracksDislike, "d"), "Dislike selected"},
			{keyName(c.TracksAddToPlaylist, "a"), "Add to playlist"},
			{keyName(c.TracksRemoveFromPlaylist, "ctrl+a"), "Remove from playlist (y/n)"},
			{keyName(c.TracksRemoveFromQueue, "X"), "Remove from queue (local)"},
			{keyName(c.TracksMoveUp, "ctrl+u"), "Move selected up"},
			{keyName(c.TracksMoveDown, "ctrl+d"), "Move selected down"},
			{keyName(c.TracksShuffle, "ctrl+x"), "Shuffle playlist"},
			{keyName(c.TracksSort, "s"), "Sort (default → title → artist → dur)"},
			{keyName(c.TracksFilter, "/"), "Focus local filter (esc clears)"},
			{keyName(c.TracksExport, "E"), "Export playlist to M3U"},
			{keyName(c.TracksUpload, "U"), "Import local MP3/M3U/folder"},
			{keyName(c.TracksStats, "ctrl+g"), "Stats toast"},
			{keyName(c.TracksJumpToPlaying, "N"), "Jump to currently playing"},
			{keyName(c.TracksShowQueue, "tab"), "Show up next"},
			{keyName(c.TracksArtistBrowse, "i"), "Browse track artist"},
			{keyName(c.TracksGoToAlbum, "A"), "Open track album"},
			{keyName(c.TracksInfo, "I"), "Track info modal"},
			{keyName(c.TracksShare, "ctrl+s"), "Copy share link"},
			{keyName(c.TracksSearch, "ctrl+f"), "Open search modal (legacy)"},
			{keyName(c.TracksBack, "backspace"), "Back / out of album"},
		}},
		{"Search tab", []entry{
			{keyName(c.Apply, "enter"), "Play selected track now"},
			{keyName(c.TracksPlayNext, "n"), "Enqueue selected as next"},
			{keyName(c.CursorUp, "↑"), "Up in results"},
			{keyName(c.CursorDown, "↓"), "Down in results"},
			{keyName(c.Cancel, "esc"), "Exit search tab"},
		}},
	}

	keyCol := lipgloss.NewStyle().Foreground(style.AccentColor).Bold(true).Width(18).Align(lipgloss.Right)
	descCol := lipgloss.NewStyle().PaddingLeft(2)
	row := func(e entry) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, keyCol.Render(e.key), descCol.Render(e.desc))
	}

	groupTitle := lipgloss.NewStyle().Foreground(style.AccentColor).Bold(true).MarginTop(1)
	hr := lipgloss.NewStyle().Foreground(style.BorderColor).Render(strings.Repeat("─", 60))

	var lines []string
	lines = append(lines,
		style.DialogTitleStyle.Render(" WellYaMusic CLI — Hotkeys "),
		hr,
		style.TrackVersionStyle.Render(" Press any key to close "),
	)
	for _, g := range groups {
		lines = append(lines, groupTitle.Render(" "+g.title))
		for _, e := range g.items {
			lines = append(lines, row(e))
		}
	}
	lines = append(lines, "")
	lines = append(lines, style.TrackVersionStyle.Render(" config: ~/.config/wellya/config.yaml  (yaml tags shown in README) "))

	body := strings.Join(lines, "\n")
	dialog := style.DialogBoxStyle.Width(68).Render(body)
	ph := lipgloss.Height(dialog)
	pw := lipgloss.Width(dialog)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Height(ph).Width(pw).Render(dialog))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	const d = "0123456789"
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{d[n%10]}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
