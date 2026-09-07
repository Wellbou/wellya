package mainpage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/ui/components/input"
	"github.com/wellbou/wellya/ui/components/help"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/components/search"
	"github.com/wellbou/wellya/ui/components/tracker"
	"github.com/wellbou/wellya/ui/components/tracklist"
	"github.com/wellbou/wellya/ui/helpers"
	"github.com/wellbou/wellya/ui/style"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dece2183/go-clipboard"
	"github.com/mattn/go-runewidth"
)

var AppVersion = "dev"

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
	helpDialog             *help.Model
	isLoading              bool
	isSearchActive         bool
	isAddPlaylistActive    bool
	isRenamePlaylistActive bool
	isUploadActive         bool
	isUploading            bool
	isPlaylistHideOverride bool
	isConfirmActive        bool
	isTrackInfoActive      bool
	showQueue              bool
	confirmAction          tea.Cmd
	confirmMessage         string

	currentPlaylistIndex int
	currentIsRadio       bool
	playGeneration       atomic.Int64
	pendingResumePos     int64
	pendingCache         bool
	searchGen            int
	lastPlaylistIdx      int
	lastRadioIdx         int
	wasRadioTab          bool
	navStack             []navPos
	offline              bool
	lastAddPlaylistKind  uint64
	likedTracksMap       map[string]bool
	likedAlbumsMap       map[uint64]bool
	cachedTracksMap      map[string]bool
	historyTracks        []api.Track
	sortMode             int

	toastMessage string
	toastGen     int

	lastSearchResult api.SearchResult
	hasSearchResult  bool
}

type toastExpireMsg struct {
	gen int
}

func (m *Model) ShowToast(msg string) tea.Cmd {
	m.toastMessage = msg
	m.toastGen++
	gen := m.toastGen
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return toastExpireMsg{gen: gen}
	})
}

func New(mediaHandler handler.MediaHandler) *Model {
	m := &Model{}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	m.clipboard = clipboard.New()
	m.mediaHandler = mediaHandler
	m.likedTracksMap = make(map[string]bool)
	m.likedAlbumsMap = make(map[uint64]bool)
	m.cachedTracksMap = make(map[string]bool)
	m.historyTracks = make([]api.Track, 0, 100)
	m.spinner = spinner.New(spinner.WithSpinner(spinner.Points))
	m.playlists = playlist.New(m.program, "WellYa")
	m.radioPlaylists = playlist.New(m.program, "Radio")
	m.tracklist = tracklist.New(m.program, &m.likedTracksMap, &m.cachedTracksMap)
	m.tracker = tracker.New(m.program, &m.likedTracksMap)
	m.searchDialog = search.New()
	m.inputDialog = input.New()
	m.helpDialog = help.New()

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

func (m *Model) currentPlaylist() *playlist.Item {
	if m.currentPlaylistIndex < 0 {
		return nil
	}
	items := m.currentPlaylists().Items()
	if m.currentPlaylistIndex >= len(items) {
		return nil
	}
	return items[m.currentPlaylistIndex]
}

func (m *Model) realTrackIndex(pl *playlist.Item) int {
	if len(m.tracklist.Items()) == 0 || pl == nil {
		return -1
	}
	if m.tracklist.FilterValue() == "" && !m.showQueue {
		idx := m.tracklist.Index()
		if idx < 0 || idx >= len(pl.Tracks) {
			return -1
		}
		return idx
	}
	sel := m.tracklist.SelectedItem()
	if sel.Track == nil {
		return -1
	}
	for i := range pl.Tracks {
		if pl.Tracks[i].Id == sel.Track.Id {
			return i
		}
	}
	return -1
}

type navPos struct {
	isRadio bool
	index   int
}

func firstActiveIndex(items []*playlist.Item) int {
	for i := range items {
		if items[i].Active {
			return i
		}
	}
	return 0
}

func (m *Model) toggleRadioTab() {
	if !m.isRadioTab {
		m.lastPlaylistIdx = m.playlists.Index()
		m.wasRadioTab = false
	} else {
		m.lastRadioIdx = m.radioPlaylists.Index()
	}
	m.isRadioTab = !m.isRadioTab
	if m.isRadioTab {
		m.isSearchTab = false
		idx := m.lastRadioIdx
		items := m.radioPlaylists.Items()
		if idx < 0 || idx >= len(items) || !items[idx].Active {
			idx = firstActiveIndex(items)
		}
		m.lastRadioIdx = idx
		m.radioPlaylists.Select(idx)
		if len(items) > 0 {
			sel := m.radioPlaylists.SelectedItem()
			if sel.Kind == playlist.STATION && len(sel.Tracks) == 0 && m.client != nil {
				m.loadStationTracks(sel)
				m.radioPlaylists.SetItem(m.radioPlaylists.Index(), sel)
			}
			m.displayPlaylist(m.radioPlaylists.SelectedItem())
		}
	} else {
		m.playlists.Select(m.lastPlaylistIdx)
		if len(m.playlists.Items()) > 0 && !m.playlists.SelectedItem().Active {
			m.playlists.Select(firstActiveIndex(m.playlists.Items()))
		}
		if len(m.playlists.Items()) > 0 {
			m.displayPlaylist(m.playlists.SelectedItem())
		}
	}
}

func (m *Model) toggleSearchTab() {
	m.isSearchTab = !m.isSearchTab
	if m.isSearchTab {
		m.wasRadioTab = m.isRadioTab
		m.isRadioTab = false
		m.searchDialog.SetSize(m.width-style.SidePanelWidth-4, m.height-6)
	} else {
		m.searchDialog.Reset()
		if m.wasRadioTab {
			m.toggleRadioTab()
		}
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

	if m.answerMediaQueries(message) {
		return m, nil
	}

	switch msg := message.(type) {
	case initialLoadDoneMsg:
		m.isLoading = false
		m.applyInitialLoad(msg)
		return m, nil

	case toastExpireMsg:
		if msg.gen == m.toastGen {
			m.toastMessage = ""
		}

	case trackReadyMsg:
		if int64(msg.generation) != m.playGeneration.Load() {
			if msg.buffer != nil {
				msg.buffer.Close()
			}
			break
		}
		if m.currentPlaylistIndex >= 0 && m.currentPlaylistIndex < len(m.currentPlaylists().Items()) {
			currentPlaylist := m.currentPlaylists().Items()[m.currentPlaylistIndex]
			if currentPlaylist.Rotor {
				if ev := api.NewTrackFeedbackEvent(api.EV_TRACK_STARTED, msg.track, 0); ev != nil {
					go m.client.RotorSessionFeedback(currentPlaylist.SessionId, api.NewFeedback(currentPlaylist.SessionBatch, ev))
					log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+msg.track.Title)
				}
			}
		}
		m.tracker.SetBitrate(msg.bitrate)
		m.pendingCache = false
		if !m.tracker.StartTrack(msg.track, msg.buffer, msg.lyrics) {
			break
		}
		if m.pendingResumePos > 0 {
			pos := m.pendingResumePos
			m.pendingResumePos = 0
			m.tracker.SetPos(time.Duration(pos) * time.Millisecond)
			m.tracker.Pause()
		}
		m.indicateCurrentTrackPlaying(true)
		m.mediaHandler.OnPlayback()
		if m.client != nil {
			go m.client.PlayTrack(msg.track, msg.fromCache)
		}
		m.saveSession()

	case trackFailedMsg:
		if int64(msg.generation) != m.playGeneration.Load() {
			break
		}
		m.tracker.ShowError(msg.reason)

	case cacheAllDoneMsg:
		m.applyCachedTracks(msg.tracks)

	case errorToastMsg:
		m.tracker.ShowError(msg.reason)

	case playlistCreatedMsg:
		m.applyPlaylistCreated(msg.item, msg.track)

	case playlistTrackAddedMsg:
		m.applyPlaylistTrackAdded(msg.kind, msg.rev, msg.track)

	case renameDoneMsg:
		m.applyRename(msg.kind, msg.title, msg.rev)

	case likeDoneMsg:
		cmd = m.applyLike(msg.trackId, msg.unlike, msg.track)
		cmds = append(cmds, cmd)

	case albumLikeDoneMsg:
		cmd = m.applyAlbumLike(msg.albumId, msg.title, msg.unlike)
		cmds = append(cmds, cmd)

	case playlistRemovedMsg:
		m.applyPlaylistRemoved(msg.kind, msg.isRadio)

	case playlistTrackRemovedMsg:
		m.applyPlaylistTrackRemoved(msg.kind, msg.rev, msg.index, msg.isRadio)

	case browsedItemMsg:
		m.applyBrowsedItem(msg.item)

	case uploadDoneMsg:
		m.isUploading = false
		if msg.err != "" {
			cmds = append(cmds, m.ShowToast(msg.err))
			break
		}
		cmds = append(cmds, m.applyUploadedTracks(msg.tracks))

	case downloadDoneMsg:
		if msg.err != "" {
			m.tracker.ShowError(msg.err)
			break
		}
		cmds = append(cmds, m.ShowToast("Saved: "+msg.filename))

	case searchReadyMsg:
		if msg.gen != m.searchGen {
			break
		}
		m.lastSearchResult = msg.res
		m.hasSearchResult = true
		if m.isRadioTab {
			m.toggleRadioTab()
		}
		cmds = append(cmds, m.playlists.SetItems(msg.items))
		m.playlists.Select(msg.index)
		m.Send(playlist.CURSOR_DOWN)

	case searchSuggestMsg:
		if msg.gen != m.searchGen {
			break
		}
		if msg.err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain search suggestions: %s", msg.err)
			m.tracker.ShowError("search suggestion")
			break
		}
		m.searchDialog.SetSuggestions(msg.suggestions)

	case searchTabResultsMsg:
		if msg.gen != m.searchGen {
			break
		}
		if msg.err != nil {
			log.Print(log.LVL_ERROR, "failed to search [%s]: %s", msg.req, msg.err)
			m.tracker.ShowError("search")
			break
		}
		m.searchDialog.SetResults(msg.tracks)

	case tea.QuitMsg:
		m.saveSession()
		return m, nil

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
			m.saveSession()
			return m, tea.Quit
		case m.helpDialog.Visible():
			m.helpDialog, cmd = m.helpDialog.Update(message)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		case m.isSearchTab:
			m.searchDialog, cmd = m.searchDialog.Update(message)
			cmds = append(cmds, cmd)
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
		case controls.KeysHelp.Contains(keypress):
			m.helpDialog.Show()
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
		case playlist.SHARE:
			sel := m.activePlaylists().SelectedItem()
			if sel.Kind >= playlist.USER && m.client != nil {
				m.clipboard.CopyText(api.SharePlaylistLink(m.client.UserID(), sel.Kind))
				cmds = append(cmds, m.ShowToast("playlist link copied"))
			}
		}

	// tracklist control update
	case regionSkipMsg:
		if msg.count == 1 {
			cmds = append(cmds, m.ShowToast("1 track unavailable in your region — skipped"))
		} else {
			cmds = append(cmds, m.ShowToast(fmt.Sprintf("%d tracks unavailable in your region — skipped", msg.count)))
		}

	// tracklist control update
	case tracklist.Control:
		switch msg {
		case tracklist.QUIT:
			m.saveSession()
			return m, tea.Quit
		case tracklist.PLAY:
			playlistItem := m.activePlaylists().SelectedItem()
			if !playlistItem.Active {
				break
			}
			if m.albumListActive() {
				cmd = m.openAlbum(m.tracklist.Index())
				cmds = append(cmds, cmd)
			} else if idx := m.realTrackIndex(playlistItem); idx >= 0 {
				m.playSelectedPlaylist(idx)
			}
		case tracklist.SHOW_QUEUE:
			m.toggleQueue()
		case tracklist.CURSOR_UP, tracklist.CURSOR_DOWN:
			active := m.activePlaylists()
			currentPlaylist := active.SelectedItem()
			if idx := m.realTrackIndex(currentPlaylist); idx >= 0 {
				currentPlaylist.SelectedTrack = idx
				cmd = active.SetItem(active.Index(), currentPlaylist)
				cmds = append(cmds, cmd)
			}
		case tracklist.LIKE:
			if m.albumListActive() {
				cmd = m.likeSelectedAlbum()
				cmds = append(cmds, cmd)
			} else {
				cmd = m.likeSelectedTrack()
				cmds = append(cmds, cmd)
			}
		case tracklist.ADD_TO_PLAYLIST:
			if m.albumListActive() {
				break
			}
			selectedTrack := m.tracklist.SelectedItem()
			if selectedTrack.Track == nil {
				break
			}
			m.searchDialog.Title = "Add " + selectedTrack.Track.Title + " to"
			m.searchDialog.Action = "add"
			m.isAddPlaylistActive = true
			m.Send(search.UPDATE_SUGGESTIONS)
		case tracklist.REMOVE_FROM_PLAYLIST:
			selectedPlaylist := m.activePlaylists().SelectedItem()
			if idx := m.realTrackIndex(selectedPlaylist); idx >= 0 {
				cmd = m.confirmRemoveFromPlaylist(selectedPlaylist, idx)
				cmds = append(cmds, cmd)
			}
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
			if selTrack := m.tracklist.SelectedItem().Track; selTrack != nil {
				if link := api.ShareTrackLink(selTrack); link != "" {
					m.clipboard.CopyText(link)
				}
			}
		case tracklist.BACK:
			active := m.activePlaylists()
			selectedPlaylist := active.SelectedItem()
			if selectedPlaylist.Browsed && len(m.navStack) > 0 {
				prev := m.navStack[len(m.navStack)-1]
				m.navStack = m.navStack[:len(m.navStack)-1]
				if prev.isRadio != m.isRadioTab {
					m.toggleRadioTab()
					active = m.activePlaylists()
				}
				if prev.index >= 0 && prev.index < len(active.Items()) {
					active.Select(prev.index)
					m.displayPlaylist(active.SelectedItem())
				}
			} else if selectedPlaylist.Kind == playlist.ALBUMS && len(selectedPlaylist.Albums) > 0 && selectedPlaylist.SelectedAlbum >= 0 {
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
		case tracklist.PLAY_NEXT:
			if idx := m.realTrackIndex(m.activePlaylists().SelectedItem()); idx >= 0 {
				track := m.activePlaylists().SelectedItem().Tracks[idx]
				cmd = m.enqueueNextTrack(&track)
				cmds = append(cmds, cmd)
			}
		case tracklist.QUICK_ADD:
			cmd = m.quickAddSelectedTrack()
			cmds = append(cmds, cmd)
		}

	// player control update
	case tracker.Control:
		switch msg {
		case tracker.NEXT:
			cmd = m.nextTrack()
			cmds = append(cmds, cmd)
		case tracker.PREV:
			cmd = m.prevTrack()
			cmds = append(cmds, cmd)
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
			if buf := m.tracker.TrackBuffer(); buf != nil && buf.IsBuffered() {
				cmd = m.cacheCurrentTrack()
				cmds = append(cmds, cmd)
			} else {
				m.pendingCache = true
				cmds = append(cmds, m.ShowToast("will cache when buffered"))
			}
		case tracker.CACHE_ALL_LIKED:
			go m.cacheAllLikedTracks()
		case tracker.DOWNLOAD_TRACK:
			cmd = m.downloadCurrentTrack()
			cmds = append(cmds, cmd)
		case tracker.TOGGLE_MUTE:
			m.tracker.ToggleMute()
		case tracker.BUFFERING_COMPLETE:
			if m.pendingCache {
				m.pendingCache = false
				cmd = m.cacheCurrentTrack()
				cmds = append(cmds, cmd)
				break
			}
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
		if msg == search.QUIT {
			m.saveSession()
			return m, tea.Quit
		}
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
		if msg == input.QUIT {
			m.saveSession()
			return m, tea.Quit
		}
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
		} else if m.helpDialog.Visible() {
			m.helpDialog, cmd = m.helpDialog.Update(message)
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

	if m.helpDialog.Visible() {
		boxW := 72
		if m.width-4 < boxW {
			boxW = m.width - 4
		}
		boxH := m.height - 4
		if boxH < 10 {
			boxH = 10
		}
		return modalOverlay(mainView, m.helpDialog.Box(m.helpDialog.Lines(), boxW, boxH), m.width)
	}

	if m.toastMessage != "" {
		toast := style.ToastBoxStyle.Render(style.ToastTextStyle.Render(m.toastMessage))
		mainView = toastOverlay(mainView, toast, m.width)
	}

	return mainView
}

func (m *Model) resize(width, height int) {
	m.width, m.height = width, height

	m.playlists.SetSize(style.SidePanelWidth, height-4)
	m.radioPlaylists.SetSize(style.SidePanelWidth, height-4)
	m.helpDialog.SetSize(width, height)
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



func (m *Model) coverFilePath(track *api.Track) string {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if os.MkdirAll(tempDir, 0755) != nil {
		return ""
	}
	return filepath.Join(tempDir, string(track.Id)+".jpg")
}

func (m *Model) metadataFilePath(trackId string) string {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if os.MkdirAll(tempDir, 0755) != nil {
		return ""
	}
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '_'
		}
		return r
	}, trackId)
	if safe == "" {
		safe = "unknown"
	}
	return filepath.Join(tempDir, "metadata-"+safe+".mp3")
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

var sgrRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func stripSGR(s string) string {
	return sgrRe.ReplaceAllString(s, "")
}

func truncateVisible(s string, w int) string {
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	vw := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < '@' || s[j] > '~') {
				j++
			}
			if j < len(s) {
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runewidth.RuneWidth(r)
		if vw+rw > w {
			break
		}
		b.WriteRune(r)
		vw += rw
		i += size
	}
	return b.String()
}

func modalOverlay(base, box string, width int) string {
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")

	totalW := width
	if totalW <= 0 {
		for _, l := range baseLines {
			if w := lipgloss.Width(l); w > totalW {
				totalW = w
			}
		}
	}

	dim := lipgloss.NewStyle().Faint(true)
	plain := make([]string, len(baseLines))
	for i, l := range baseLines {
		p := stripSGR(l)
		if w := lipgloss.Width(p); w < totalW {
			p += strings.Repeat(" ", totalW-w)
		}
		plain[i] = p
	}

	boxW := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxW {
			boxW = w
		}
	}
	startRow := (len(plain) - len(boxLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (totalW - boxW) / 2
	if startCol < 0 {
		startCol = 0
	}

	paint := func(s string) string { return dim.Render(s) }
	return strings.Join(overlayBox(plain, boxLines, totalW, boxW, startRow, startCol, paint), "\n")
}

func overlayBox(plain, boxLines []string, totalW, boxW, startRow, startCol int, paint func(string) string) []string {
	out := make([]string, len(plain))
	for i, p := range plain {
		out[i] = paint(p)
	}
	for i, bl := range boxLines {
		r := startRow + i
		if r < 0 || r >= len(out) {
			continue
		}
		left := truncateVisible(plain[r], startCol)
		if w := lipgloss.Width(left); w < startCol {
			left += strings.Repeat(" ", startCol-w)
		}
		right := ""
		if rw := totalW - startCol - boxW; rw > 0 {
			right = strings.Repeat(" ", rw)
		}
		out[r] = paint(left) + bl + right
	}
	return out
}

func toastOverlay(base, toast string, width int) string {
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(toast, "\n")

	totalW := width
	if totalW <= 0 {
		for _, l := range baseLines {
			if w := lipgloss.Width(l); w > totalW {
				totalW = w
			}
		}
	}
	plain := make([]string, len(baseLines))
	for i, l := range baseLines {
		p := l
		if w := lipgloss.Width(p); w < totalW {
			p += strings.Repeat(" ", totalW-w)
		}
		plain[i] = p
	}

	boxW := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxW {
			boxW = w
		}
	}
	startRow := len(plain) - len(boxLines)
	if startRow < 0 {
		startRow = 0
	}
	startCol := (totalW - boxW) / 2
	if startCol < 0 {
		startCol = 0
	}

	plainFn := func(s string) string { return s }
	return strings.Join(overlayBox(plain, boxLines, totalW, boxW, startRow, startCol, plainFn), "\n")
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
		if m.client == nil {
			m.tracker.ShowError("not logged in")
			return nil
		}
		m.searchGen++
		gen := m.searchGen
		client := m.client
		go func() {
			res, err := client.Search(req, api.SEARCH_ALL)
			if err != nil {
				m.Send(searchTabResultsMsg{gen: gen, req: req, err: err})
				return
			}
			m.Send(searchTabResultsMsg{gen: gen, req: req, tracks: res.Tracks.Results})
		}()
	}
	return cmd
}
