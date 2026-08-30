package mainpage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/ui/components/input"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/components/search"
	"github.com/wellbou/wellya/ui/components/tracker"
	"github.com/wellbou/wellya/ui/components/tracklist"
	"github.com/wellbou/wellya/ui/helpers"
	"github.com/wellbou/wellya/ui/model"
	"github.com/wellbou/wellya/ui/style"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dece2183/go-clipboard"
)

const AppVersion = "dev-search-tab"

type Model struct {
	program       *tea.Program
	client        *api.YaMusicClient
	clipboard     *clipboard.Clipboard
	mediaHandler  handler.MediaHandler
	width, height int

	spinner        spinner.Model
	playlists      *playlist.Model
	radioPlaylists *playlist.Model
	tracklist      *tracklist.Model
	tracker        *tracker.Model
	isRadioTab     bool
	isSearchTab    bool

	searchDialog           *search.Model
	inputDialog            *input.Model
	isLoading              bool
	isSearchActive         bool
	isAddPlaylistActive    bool
	isRenamePlaylistActive bool
	isUploadActive         bool
	isPlaylistHideOverride bool
	isConfirmActive        bool
	isTrackInfoActive      bool
	showQueue              bool
	confirmAction          tea.Cmd
	confirmMessage         string

	currentPlaylistIndex int
	currentIsRadio       bool
	playGeneration       int
	likedTracksMap       map[string]bool
	cachedTracksMap      map[string]bool
	historyTracks        []api.Track
	sortMode             int

	toastMessage string
	toastTimer   int

	lastSearchResult api.SearchResult
	hasSearchResult  bool
}

type toastTickMsg struct{}

func (m *Model) ShowToast(msg string) tea.Cmd {
	m.toastMessage = msg
	m.toastTimer = 150
	return toastTickCmd
}

func toastTickCmd() tea.Msg {
	return toastTickMsg{}
}

func New(mediaHandler handler.MediaHandler) *Model {
	m := &Model{}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	m.clipboard = clipboard.New()
	m.mediaHandler = mediaHandler
	m.likedTracksMap = make(map[string]bool)
	m.cachedTracksMap = make(map[string]bool)
	m.historyTracks = make([]api.Track, 0, 100)
	m.spinner = spinner.New(spinner.WithSpinner(spinner.Points))
	m.playlists = playlist.New(m.program, "YaMusic")
	m.radioPlaylists = playlist.New(m.program, "Radio")
	m.tracklist = tracklist.New(m.program, &m.likedTracksMap, &m.cachedTracksMap)
	m.tracker = tracker.New(m.program, &m.likedTracksMap)
	m.searchDialog = search.New()
	m.inputDialog = input.New()

	return m
}

func (m *Model) activePlaylists() *playlist.Model {
	if m.isRadioTab {
		return m.radioPlaylists
	}
	return m.playlists
}

func (m *Model) currentPlaylists() *playlist.Model {
	if m.currentIsRadio {
		return m.radioPlaylists
	}
	return m.playlists
}

func (m *Model) toggleRadioTab() {
	m.isRadioTab = !m.isRadioTab
	if m.isRadioTab {
		m.isSearchTab = false
		m.radioPlaylists.Select(0)
		if len(m.radioPlaylists.Items()) > 0 {
			m.displayPlaylist(m.radioPlaylists.SelectedItem())
		}
	} else {
		m.playlists.Select(0)
		if len(m.playlists.Items()) > 0 {
			m.displayPlaylist(m.playlists.SelectedItem())
		}
	}
}

func (m *Model) toggleSearchTab() {
	m.isSearchTab = !m.isSearchTab
	m.isRadioTab = false
	if m.isSearchTab {
		m.searchDialog.SetSize(m.width-style.SidePanelWidth-4, m.height-6)
	} else {
		m.searchDialog.Reset()
	}
}

func (m *Model) Run() error {
	go m.mediaHandle()
	_, err := m.program.Run()
	m.tracker.Stop()
	return err
}

func (m *Model) Send(msg tea.Msg) {
	go m.program.Send(msg)
}

func (m *Model) Init() tea.Cmd {
	m.isLoading = true
	go m.initialLoad()
	return m.spinner.Tick
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := message.(type) {
	case LoadingMsg:
		m.isLoading = false
		return m, model.Cmd(playlist.CURSOR_UP)

	case toastTickMsg:
		if m.toastTimer > 0 {
			m.toastTimer--
			if m.toastTimer == 0 {
				m.toastMessage = ""
			} else {
				cmds = append(cmds, toastTickCmd)
			}
		}

	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, tea.ClearScreen

	case tea.KeyMsg:
		controls := config.Current.Controls
		keypress := msg.String()

		if m.isConfirmActive {
			switch keypress {
			case "y", "enter":
				action := m.confirmAction
				m.isConfirmActive = false
				m.confirmAction = nil
				m.confirmMessage = ""
				return m, action
			case "n", "esc":
				m.isConfirmActive = false
				m.confirmAction = nil
				m.confirmMessage = ""
				return m, nil
			}
			return m, nil
		}

		if m.isTrackInfoActive {
			switch keypress {
			case "esc", "enter":
				m.isTrackInfoActive = false
				return m, nil
			}
			return m, nil
		}

		switch {
		case controls.Quit.Contains(keypress):
			return m, tea.Quit
		case m.isSearchActive || m.isAddPlaylistActive:
			m.searchDialog, cmd = m.searchDialog.Update(message)
			cmds = append(cmds, cmd)
		case m.isRenamePlaylistActive || m.isUploadActive:
			m.inputDialog, cmd = m.inputDialog.Update(message)
			cmds = append(cmds, cmd)
		case controls.PlaylistsRadio.Contains(keypress):
			m.toggleRadioTab()
		case controls.TracksSearchTab.Contains(keypress):
			m.toggleSearchTab()
		case controls.Reload.Contains(keypress):
			config.InitialLoad()
			m.isLoading = true
			cmd = m.playlists.Reset()
			cmds = append(cmds, cmd)
			cmd = m.radioPlaylists.Reset()
			cmds = append(cmds, cmd)
			cmds = append(cmds, m.spinner.Tick)
			go m.initialLoad()
		default:
			if m.isLoading {
				m.spinner, cmd = m.spinner.Update(message)
				cmds = append(cmds, cmd)
			} else {
				if m.isRadioTab {
					m.radioPlaylists, cmd = m.radioPlaylists.Update(message)
					cmds = append(cmds, cmd)
				} else {
					m.playlists, cmd = m.playlists.Update(message)
					cmds = append(cmds, cmd)
				}
				m.tracklist, cmd = m.tracklist.Update(message)
				cmds = append(cmds, cmd)
				m.tracker, cmd = m.tracker.Update(message)
				cmds = append(cmds, cmd)
			}
		}

	// playlist control update
	case playlist.Control:
		switch msg {
		case playlist.CURSOR_UP, playlist.CURSOR_DOWN:
			m.showQueue = false
			active := m.activePlaylists()
			selectedPlaylist := active.SelectedItem()

			if !selectedPlaylist.Active && !selectedPlaylist.Subitem {
				return m, nil
			}

			if selectedPlaylist.Kind == playlist.HISTORY {
				selectedPlaylist.Tracks = make([]api.Track, len(m.historyTracks))
				copy(selectedPlaylist.Tracks, m.historyTracks)
				active.SetItem(active.Index(), selectedPlaylist)
			}

			if selectedPlaylist.Kind == playlist.ALBUMS && len(selectedPlaylist.Albums) > 0 {
				selectedPlaylist.SelectedAlbum = -1
			}

			if selectedPlaylist.Kind == playlist.STATION && len(selectedPlaylist.Tracks) == 0 && m.client != nil {
				m.loadStationTracks(selectedPlaylist)
				active.SetItem(active.Index(), selectedPlaylist)
			}

			if m.currentPlaylistIndex >= 0 {
				curPls := m.currentPlaylists()
				if m.currentPlaylistIndex < len(curPls.Items()) {
					currentPlaylist := curPls.Items()[m.currentPlaylistIndex]
					if selectedPlaylist.IsSame(currentPlaylist) && len(selectedPlaylist.Tracks) > 0 {
						selectedPlaylist.SelectedTrack = selectedPlaylist.CurrentTrack
						active.SetItem(active.Index(), selectedPlaylist)
					}
				}
			}

			m.displayPlaylist(selectedPlaylist)
			m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())

			m.tracklist.Shufflable = (selectedPlaylist.Kind != playlist.NONE && selectedPlaylist.Kind != playlist.MYWAVE && selectedPlaylist.Kind != playlist.STATION && selectedPlaylist.Kind != playlist.HISTORY && len(selectedPlaylist.Tracks) > 0)
		case playlist.RENAME:
			active := m.activePlaylists()
			selectedPlaylist := active.SelectedItem()
			if selectedPlaylist.Kind < playlist.USER {
				break
			}
			m.inputDialog.Title = "Rename playlist " + selectedPlaylist.Name
			m.inputDialog.SetValue(selectedPlaylist.Name)
			m.isRenamePlaylistActive = true
		case playlist.TOGGLE_VIEW:
			m.isPlaylistHideOverride = !m.isPlaylistHideOverride
		}

	// tracklist control update
	case tracklist.Control:
		switch msg {
		case tracklist.PLAY:
			playlistItem := m.activePlaylists().SelectedItem()
			if !playlistItem.Active {
				break
			}
			if m.albumListActive() {
				cmd = m.openAlbum(m.tracklist.Index())
				cmds = append(cmds, cmd)
			} else {
				m.playSelectedPlaylist(m.tracklist.Index())
			}
		case tracklist.SHOW_QUEUE:
			m.toggleQueue()
		case tracklist.CURSOR_UP, tracklist.CURSOR_DOWN:
			active := m.activePlaylists()
			currentPlaylist := active.SelectedItem()
			cursorIndex := m.tracklist.Index()
			currentPlaylist.SelectedTrack = cursorIndex
			cmd = active.SetItem(active.Index(), currentPlaylist)
			cmds = append(cmds, cmd)
		case tracklist.LIKE:
			if !m.albumListActive() {
				cmd = m.likeSelectedTrack()
				cmds = append(cmds, cmd)
			}
		case tracklist.ADD_TO_PLAYLIST:
			if m.albumListActive() {
				break
			}
			selectedTrack := m.tracklist.SelectedItem()
			m.searchDialog.Title = "Add " + selectedTrack.Track.Title + " to"
			m.searchDialog.Action = "add"
			m.isAddPlaylistActive = true
			m.Send(search.UPDATE_SUGGESTIONS)
		case tracklist.REMOVE_FROM_PLAYLIST:
			selectedPlaylist := m.activePlaylists().SelectedItem()
			cmd = m.confirmRemoveFromPlaylist(selectedPlaylist, m.tracklist.Index())
			cmds = append(cmds, cmd)
		case tracklist.SEARCH:
			m.searchDialog.Title = "Search"
			m.searchDialog.Action = "search"
			m.isSearchActive = true
			m.Send(search.UPDATE_SUGGESTIONS)
		case tracklist.SHUFFLE:
			cmd = m.shufflePlaylist(m.activePlaylists().SelectedItem())
			cmds = append(cmds, cmd)
		case tracklist.SHARE:
			if m.albumListActive() {
				break
			}
			link := api.ShareTrackLink(m.tracklist.SelectedItem().Track)
			if link != "" {
				m.clipboard.CopyText(link)
			}
		case tracklist.BACK:
			active := m.activePlaylists()
			selectedPlaylist := active.SelectedItem()
			if selectedPlaylist.Kind == playlist.ALBUMS && len(selectedPlaylist.Albums) > 0 && selectedPlaylist.SelectedAlbum >= 0 {
				selectedPlaylist.SelectedAlbum = -1
				m.displayPlaylist(selectedPlaylist)
				cmd = active.SetItem(active.Index(), selectedPlaylist)
				cmds = append(cmds, cmd)
			}
		case tracklist.MOVE_UP:
			cmd = m.moveTrack(-1)
			cmds = append(cmds, cmd)
		case tracklist.MOVE_DOWN:
			cmd = m.moveTrack(1)
			cmds = append(cmds, cmd)
		case tracklist.JUMP_TO_PLAYING:
			cmd = m.jumpToPlayingTrack()
			cmds = append(cmds, cmd)
		case tracklist.ARTIST_BROWSE:
			cmd = m.browseSelectedTrackArtist()
			cmds = append(cmds, cmd)
		case tracklist.TRACK_INFO:
			m.showTrackInfo()
		case tracklist.GO_TO_ALBUM:
			cmd = m.goToAlbum()
			cmds = append(cmds, cmd)
		case tracklist.DISLIKE:
			cmd = m.dislikeSelectedTrack()
			cmds = append(cmds, cmd)
		case tracklist.SORT:
			cmd = m.sortPlaylist()
			cmds = append(cmds, cmd)
		case tracklist.REMOVE_FROM_QUEUE:
			cmd = m.removeFromQueue()
			cmds = append(cmds, cmd)
		case tracklist.EXPORT:
			cmd = m.exportPlaylist()
			cmds = append(cmds, cmd)
		case tracklist.UPLOAD:
			m.inputDialog.Title = "Upload MP3 (file or dir):"
			m.inputDialog.Action = "upload"
			m.inputDialog.SetValue("")
			m.isUploadActive = true
		case tracklist.STATS:
			cmd = m.showStats()
			cmds = append(cmds, cmd)
		}

	// player control update
	case tracker.Control:
		switch msg {
		case tracker.NEXT:
			m.nextTrack()
		case tracker.PREV:
			m.prevTrack()
		case tracker.LIKE:
			cmd = m.likePlayingTrack()
			cmds = append(cmds, cmd)
		case tracker.PLAY, tracker.PAUSE:
			m.mediaHandler.OnPlayPause()
		case tracker.STOP:
			m.mediaHandler.OnEnded()
		case tracker.REWIND:
			m.mediaHandler.OnSeek(m.tracker.Position())
		case tracker.VOLUME:
			m.mediaHandler.OnVolume()
		case tracker.CACHE_TRACK:
			cmd = m.cacheCurrentTrack()
			cmds = append(cmds, cmd)
		case tracker.CACHE_ALL_LIKED:
			go m.cacheAllLikedTracks()
		case tracker.DOWNLOAD_TRACK:
			cmd = m.downloadCurrentTrack()
			cmds = append(cmds, cmd)
		case tracker.TOGGLE_MUTE:
			m.tracker.ToggleMute()
		case tracker.BUFFERING_COMPLETE:
			cacheMode := config.Current.CacheTracks
			if cacheMode == config.CACHE_ALL || (cacheMode == config.CACHE_LIKED_ONLY && m.likedTracksMap[string(m.tracker.CurrentTrack().Id)]) {
				cmd = m.cacheCurrentTrack()
				cmds = append(cmds, cmd)
			}
		case tracker.DISLIKE:
			cmd = m.dislikePlayingTrack()
			cmds = append(cmds, cmd)
		}

		m.tracker, cmd = m.tracker.Update(message)
		cmds = append(cmds, cmd)

	// search control update
	case search.Control:
		if m.isSearchTab {
			cmd = m.searchTabControl(msg)
			cmds = append(cmds, cmd)
		} else if m.isSearchActive {
			cmd = m.searchControl(msg)
			cmds = append(cmds, cmd)
		} else if m.isAddPlaylistActive {
			cmd = m.addPlaylistControl(msg)
			cmds = append(cmds, cmd)
		}

	// input dialog control update
	case input.Control:
		if m.isUploadActive {
			m.isUploadActive = false
			cmd = m.uploadControl(msg)
			cmds = append(cmds, cmd)
		} else {
			m.isRenamePlaylistActive = false
			cmd = m.renamePlaylistControl(msg)
			cmds = append(cmds, cmd)
		}

	default:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(message)
			cmds = append(cmds, cmd)
		} else if m.isSearchTab {
			m.searchDialog, cmd = m.searchDialog.Update(message)
			cmds = append(cmds, cmd)
		} else if m.isSearchActive || m.isAddPlaylistActive {
			m.searchDialog, cmd = m.searchDialog.Update(message)
			cmds = append(cmds, cmd)
		} else if m.isRenamePlaylistActive || m.isUploadActive {
			m.inputDialog, cmd = m.inputDialog.Update(message)
			cmds = append(cmds, cmd)
		} else {
			if m.isRadioTab {
				m.radioPlaylists, cmd = m.radioPlaylists.Update(message)
				cmds = append(cmds, cmd)
			} else {
				m.playlists, cmd = m.playlists.Update(message)
				cmds = append(cmds, cmd)
			}
			m.tracklist, cmd = m.tracklist.Update(message)
			cmds = append(cmds, cmd)
			m.tracker, cmd = m.tracker.Update(message)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	if m.isLoading {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.spinner.View())
	}

	if m.isSearchActive || m.isAddPlaylistActive {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.searchDialog.View())
	} else if m.isRenamePlaylistActive || m.isUploadActive {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.inputDialog.View())
	} else if m.isConfirmActive {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.confirmView())
	} else if m.isTrackInfoActive {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.trackInfoView())
	}

	activePls := m.activePlaylists()
	tabActive := style.ActiveButtonStyle.Padding(0, 1).Render
	tabInactive := style.ButtonStyle.Padding(0, 1).Render
	var tabBar string
	if m.isSearchTab {
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top, tabInactive(" Playlists "), tabInactive(" Radio "), tabActive(" Search "))
	} else if m.isRadioTab {
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top, tabInactive(" Playlists "), tabActive(" Radio "), tabInactive(" Search "))
	} else {
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top, tabActive(" Playlists "), tabInactive(" Radio "), tabInactive(" Search "))
	}
	tabBar = lipgloss.NewStyle().Width(style.SidePanelWidth).Align(lipgloss.Center).Render(tabBar + " " + style.TrackVersionStyle.Render("S/R:switch"))

	playlistView := activePls.View()
	playlistWithTabs := lipgloss.JoinVertical(lipgloss.Left, tabBar, playlistView)
	playlistWidth := lipgloss.Width(playlistWithTabs)

	m.tracker.SetWidth(m.width - playlistWidth - 2)
	m.tracklist.SetWidth(m.width - playlistWidth - 2)
	m.searchDialog.SetSize(m.width-playlistWidth-2, m.height-6)

	trackerView := m.tracker.View()
	trackerHeight := lipgloss.Height(trackerView)
	m.tracklist.SetHeight(m.height - trackerHeight - 2)

	var midPanel string
	if m.isSearchTab {
		midPanel = m.searchDialog.View()
	} else {
		tracklistView := m.tracklist.View()
		if m.tracklist.Hidden {
			midPanel = trackerView
		} else if m.tracker.Hidden {
			midPanel = tracklistView
		} else {
			midPanel = lipgloss.JoinVertical(lipgloss.Left, tracklistView, trackerView)
		}
	}

	mainView := lipgloss.JoinHorizontal(lipgloss.Bottom, playlistWithTabs, midPanel)

	versionLabel := style.TrackVersionStyle.Render(" " + AppVersion + " ")
	mainView = lipgloss.JoinVertical(lipgloss.Left, mainView, versionLabel)

	if m.toastMessage != "" {
		toast := style.ToastBoxStyle.Render(style.ToastTextStyle.Render(m.toastMessage))
		toastOverlay := lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Bottom, toast)
		mainView = lipgloss.JoinVertical(lipgloss.Left, mainView, toastOverlay)
	}

	return mainView
}

func (m *Model) resize(width, height int) {
	m.width, m.height = width, height

	m.playlists.SetSize(style.SidePanelWidth, height-4)
	m.radioPlaylists.SetSize(style.SidePanelWidth, height-4)
	if !m.isPlaylistHideOverride {
		hide := m.width < style.SidePanelAutohide
		m.playlists.Hidden = hide
		m.radioPlaylists.Hidden = hide
	}

	searchWidth := style.SearchModalWidth
	if searchWidth > m.width {
		searchWidth = m.width - 2
	}

	m.searchDialog.SetSize(searchWidth, m.height-4)
	m.inputDialog.SetWidth(searchWidth)
}

func (m *Model) mediaHandle() {
	for msg := range m.mediaHandler.Message() {
		switch msg.Type {
		case handler.MSG_NEXT:
			m.Send(tracker.NEXT)
		case handler.MSG_PREVIOUS:
			m.Send(tracker.PREV)
		case handler.MSG_PLAY:
			m.tracker.Play()
			m.Send(tracker.PLAY)
		case handler.MSG_PAUSE:
			m.tracker.Pause()
			m.Send(tracker.PAUSE)
		case handler.MSG_PLAYPAUSE:
			if m.tracker.IsPlaying() {
				m.tracker.Pause()
				m.Send(tracker.PAUSE)
			} else {
				m.tracker.Play()
				m.Send(tracker.PLAY)
			}
		case handler.MSG_STOP:
			m.Send(tracker.STOP)
		case handler.MSG_SEEK:
			offset, ok := msg.Arg.(time.Duration)
			if ok {
				m.tracker.Rewind(offset)
			}
		case handler.MSG_SETPOS:
			pos, ok := msg.Arg.(time.Duration)
			if ok {
				m.tracker.SetPos(pos)
			}

		case handler.MSG_SET_SHUFFLE:
			val, ok := msg.Arg.(bool)
			if !ok || !val {
				break
			}
			if m.currentPlaylistIndex < 0 || m.currentPlaylistIndex >= len(m.currentPlaylists().Items()) {
				break
			}
			currentPlaylist := m.currentPlaylists().Items()[m.currentPlaylistIndex]
			if len(currentPlaylist.Tracks) == 0 {
				break
			}
			if currentPlaylist.Kind >= playlist.LIKES {
				cmd := m.shufflePlaylist(currentPlaylist)
				m.Send(func() tea.Cmd {
					return cmd
				})
			}
		case handler.MSG_SET_VOLUME:
			vol, ok := msg.Arg.(float64)
			if ok {
				m.tracker.SetVolume(vol)
			}

		case handler.MSG_GET_PLAYBACKSTATUS:
			var state handler.PlaybackState
			if m.tracker.IsPlaying() {
				state = handler.STATE_PLAYING
			} else {
				if m.tracker.IsStoped() {
					state = handler.STATE_STOPPED
				} else {
					state = handler.STATE_PAUSED
				}
			}
			m.mediaHandler.SendAnswer(state)
		case handler.MSG_GET_SHUFFLE:
			m.mediaHandler.SendAnswer(false)
		case handler.MSG_GET_METADATA:
			if m.tracker.IsStoped() {
				m.mediaHandler.SendAnswer(handler.TrackMetadata{})
				break
			}
			track := m.tracker.CurrentTrack()
			artists := make([]string, 0, len(track.Artists))
			for i := range track.Artists {
				artists = append(artists, track.Artists[i].Name)
			}
			albumArtists := make([]string, 0)
			var albumName string
			genre := make([]string, 0)
			if len(track.Albums) != 0 {
				for i := range track.Albums[0].Artists {
					albumArtists = append(albumArtists, track.Albums[0].Artists[i].Name)
				}
				albumName = track.Albums[0].Title
				genre = append(genre, track.Albums[0].Genre)
			}

			md := handler.TrackMetadata{
				TrackId: string(track.Id),
				Length:       time.Duration(track.DurationMs) * time.Millisecond,
				CoverUrl:     m.coverFilePath(track),
				AlbumName:    albumName,
				AlbumArtists: albumArtists,
				Artists:      artists,
				Genre:        genre,
				Title:        track.Title,
				Url:          api.ShareTrackLink(track),
			}
			m.mediaHandler.SendAnswer(md)
		case handler.MSG_GET_VOLUME:
			m.mediaHandler.SendAnswer(m.tracker.Volume())
		case handler.MSG_GET_POSITION:
			m.mediaHandler.SendAnswer(m.tracker.Position())
		}
	}
}

func (m *Model) coverFilePath(track *api.Track) string {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if os.MkdirAll(tempDir, 0755) != nil {
		return ""
	}
	return filepath.Join(tempDir, string(track.Id)+".jpg")
}

func (m *Model) metadataFilePath() string {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if os.MkdirAll(tempDir, 0755) != nil {
		return ""
	}
	return filepath.Join(tempDir, "metadata.mp3")
}

func (m *Model) confirmView() string {
	title := style.DialogTitleStyle.Render(m.confirmMessage)
	body := style.DialogBoxStyle.Render("(y)es  (n)o")
	return lipgloss.JoinVertical(lipgloss.Left, title, body)
}

func (m *Model) trackInfoView() string {
	if m.tracklist.SelectedItem().Track == nil {
		return ""
	}
	track := m.tracklist.SelectedItem().Track

	valueStyle := lipgloss.NewStyle().Foreground(style.NormalTextColor)

	lines := make([]string, 0)
	lines = append(lines, style.DialogTitleStyle.Render("Track Info"))
	lines = append(lines, "")

	lines = append(lines, style.AccentTextStyle.Render("Title: ")+valueStyle.Render(track.Title))
	if track.Version != "" {
		lines = append(lines, style.AccentTextStyle.Render("Version: ")+valueStyle.Render(track.Version))
	}
	lines = append(lines, style.AccentTextStyle.Render("Artists: ")+valueStyle.Render(helpers.ArtistList(track.Artists)))
	if len(track.Albums) > 0 {
		albumNames := make([]string, 0)
		for _, a := range track.Albums {
			albumNames = append(albumNames, a.Title)
		}
		lines = append(lines, style.AccentTextStyle.Render("Album: ")+valueStyle.Render(strings.Join(albumNames, ", ")))
		lines = append(lines, style.AccentTextStyle.Render("Year: ")+valueStyle.Render(fmt.Sprintf("%d", track.Albums[0].Year)))
		if track.Albums[0].Genre != "" {
			lines = append(lines, style.AccentTextStyle.Render("Genre: ")+valueStyle.Render(track.Albums[0].Genre))
		}
	}
	dur := time.Duration(track.DurationMs) * time.Millisecond
	lines = append(lines, style.AccentTextStyle.Render("Duration: ")+valueStyle.Render(fmt.Sprintf("%d:%02d", int(dur.Minutes()), int(dur.Seconds())%60)))

	liked := "No"
	if m.likedTracksMap[string(track.Id)] {
		liked = "Yes"
	}
	lines = append(lines, style.AccentTextStyle.Render("Liked: ")+valueStyle.Render(liked))
	cached := "No"
	if m.cachedTracksMap[string(track.Id)] {
		cached = "Yes"
	}
	lines = append(lines, style.AccentTextStyle.Render("Cached: ")+valueStyle.Render(cached))

	body := style.DialogBoxStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, body)
}

func (m *Model) showTrackInfo() {
	if m.tracklist.SelectedItem().Track != nil {
		m.isTrackInfoActive = true
	}
}

func (m *Model) searchTabControl(msg search.Control) tea.Cmd {
	var cmd tea.Cmd
	switch msg {
	case search.SELECT:
		if t := m.searchDialog.SelectedTrack(); t != nil {
			cmd = m.playNowTrack(t)
		}
	case search.PLAY_NEXT:
		if t := m.searchDialog.SelectedTrack(); t != nil {
			cmd = m.enqueueNextTrack(t)
		}
	case search.CANCEL:
		m.isSearchTab = false
		m.searchDialog.Reset()
	case search.TYPING:
		req := m.searchDialog.InputValue()
		if req == "" {
			return nil
		}
		res, err := m.client.Search(req, api.SEARCH_ALL)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to search [%s]: %s", req, err)
			m.tracker.ShowError("search")
			return nil
		}
		m.searchDialog.SetResults(res.Tracks.Results)
	}
	return cmd
}
