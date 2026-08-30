package mainpage

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/input"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/components/search"
	"github.com/wellbou/wellya/ui/components/tracklist"
	"github.com/wellbou/wellya/ui/helpers"
)

func (m *Model) addPlaylistControl(msg search.Control) tea.Cmd {
	var cmd tea.Cmd

	switch msg {
	case search.SELECT:
		m.isAddPlaylistActive = false

		selectedPlaylist := m.playlists.SelectedItem()
		if len(selectedPlaylist.Tracks) == 0 || selectedPlaylist.Kind == playlist.HISTORY {
			return nil
		}

		playlists := m.playlists.Items()
		inputVal, ok := m.searchDialog.SuggestionValue()
		if !ok {
			return nil
		}

		foundPlaylistIndex := -1
		var foundPlaylist *playlist.Item
		for i := range playlists {
			if playlists[i].Active && playlists[i].Kind >= playlist.USER {
				if strings.EqualFold(playlists[i].Name, inputVal) {
					foundPlaylist = playlists[i]
					foundPlaylistIndex = i
					break
				} else if foundPlaylistIndex < 0 {
					foundPlaylistIndex = i
				}
			}
		}

		if foundPlaylist == nil {
			pl, err := m.client.CreatePlaylist(inputVal, true)
			if err != nil {
				log.Print(log.LVL_ERROR, "failed to create playlist [%s]: %s", inputVal, err)
				m.tracker.ShowError("playlist create")
				return nil
			}

			foundPlaylist = &playlist.Item{
				Name:     pl.Title,
				Kind: uint64(pl.Kind),
				Revision: pl.Revision,
				Active:   true,
				Subitem:  true,
			}

			m.playlists.InsertItem(foundPlaylistIndex, foundPlaylist)
			if foundPlaylistIndex < m.playlists.Index() {
				m.playlists.Select(m.playlists.Index() + 1)
			}
			if m.currentPlaylistIndex >= m.playlists.Index() && m.tracker.IsPlaying() {
				m.currentPlaylistIndex += 1
			}
		}

		if selectedPlaylist.Kind == foundPlaylist.Kind {
			return nil
		}

		selectedTrack := &selectedPlaylist.Tracks[m.tracklist.Index()]
		pl, err := m.client.AddToPlaylist(foundPlaylist.Kind, foundPlaylist.Revision, len(foundPlaylist.Tracks), selectedTrack.Id)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to add track [%s] to playlist [%s]: %s", selectedTrack.Id, foundPlaylist.Name, err)
			m.tracker.ShowError("playlist add")
			return nil
		}

		foundPlaylist.Revision = pl.Revision
		foundPlaylist.Tracks = append(foundPlaylist.Tracks, *selectedTrack)
		cmd = m.playlists.SetItem(foundPlaylistIndex, foundPlaylist)

		m.isAddPlaylistActive = false
	case search.CANCEL:
		m.isAddPlaylistActive = false
	case search.UPDATE_SUGGESTIONS:
		inputVal := strings.ToLower(m.searchDialog.InputValue())
		playlists := m.playlists.Items()
		suggestions := make([]string, 0, len(playlists))
		for _, pl := range playlists {
			if !pl.Active || pl.Kind < playlist.USER || (len(inputVal) > 0 && !strings.Contains(strings.ToLower(pl.Name), inputVal)) {
				continue
			}
			suggestions = append(suggestions, pl.Name)
		}
		m.searchDialog.SetSuggestions(suggestions)
	}

	return cmd
}

func (m *Model) renamePlaylistControl(msg input.Control) tea.Cmd {
	var cmd tea.Cmd

	if msg != input.APPLY {
		return nil
	}

	newName := m.inputDialog.Value()
	if len(strings.ReplaceAll(newName, " ", "")) == 0 {
		return nil
	}

	selectedPlaylist := m.playlists.SelectedItem()
	pl, err := m.client.RenamePlaylist(selectedPlaylist.Kind, newName)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to rename playlist [%s] to '%s': %s", selectedPlaylist.Name, newName, err)
		m.tracker.ShowError("playlist rename")
		return nil
	}

	selectedPlaylist.Name = pl.Title
	selectedPlaylist.Revision = pl.Revision
	m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)

	return cmd
}

func (m *Model) confirmRemoveFromPlaylist(pl *playlist.Item, index int) tea.Cmd {
	if index >= len(pl.Tracks) {
		return nil
	}

	var msg string
	switch pl.Kind {
	case playlist.LOCAL:
		msg = "Remove cached track? (y/n)"
	default:
		msg = "Remove track from playlist? (y/n)"
	}

	m.confirmAction = func() tea.Msg {
		return m.removeFromPlaylist(pl, index)()
	}
	m.confirmMessage = msg
	m.isConfirmActive = true
	return nil
}

func (m *Model) removeFromPlaylist(pl *playlist.Item, index int) tea.Cmd {
	if index >= len(pl.Tracks) {
		return nil
	}

	switch pl.Kind {
	case playlist.NONE, playlist.MYWAVE, playlist.STATION, playlist.ALBUMS, playlist.HISTORY:
		return nil
	case playlist.LIKES:
		selectedTrack := pl.Tracks[index]
		return m.likeTrack(&selectedTrack, pl)
	case playlist.LOCAL:
		selectedTrack := pl.Tracks[index]
		return m.removeCache(&selectedTrack)
	default:
		var cmd tea.Cmd

		if len(pl.Tracks) < 2 {
			err := m.client.RemovePlaylist(pl.Kind)
			if err != nil {
				log.Print(log.LVL_ERROR, "failed to remove playlist [%s]: %s", pl.Name, err)
				m.tracker.ShowError("playlist remove")
				return nil
			}
			if m.currentPlaylistIndex >= m.playlists.Index() && m.tracker.IsPlaying() {
				m.currentPlaylistIndex -= 1
			}
			m.playlists.RemoveItem(m.playlists.Index())
			if len(m.playlists.Items()) <= m.playlists.Index() {
				m.playlists.Select(0)
			}
			m.displayPlaylist(m.playlists.SelectedItem())
			return nil
		}

		newpl, err := m.client.RemoveFromPlaylist(pl.Kind, pl.Revision, index)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to remove track [%s] from playlist [%s]: %s", pl.Tracks[index].Id, pl.Name, err)
			m.tracker.ShowError("playlist remove track")
			return nil
		}

		pl.Revision = newpl.Revision
		pl.Tracks = slices.Delete(pl.Tracks, index, index+1)
		if index >= len(pl.Tracks) {
			pl.SelectedTrack = len(pl.Tracks) - 1
		} else {
			pl.SelectedTrack = index
		}
		deleteCurrentTrack := index == pl.CurrentTrack
		if deleteCurrentTrack {
			pl.CurrentTrack = len(pl.Tracks)
		} else if pl.CurrentTrack > index {
			pl.CurrentTrack--
		}
		cmd = m.playlists.SetItem(m.playlists.Index(), pl)
		m.displayPlaylist(pl)

		if m.currentPlaylistIndex >= 0 {
			currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
			if pl.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
				m.indicateCurrentTrackPlaying(!deleteCurrentTrack)
			}
		}

		return cmd
	}
}

func (m *Model) removeFromQueue() tea.Cmd {
	selectedPlaylist := m.playlists.SelectedItem()
	index := m.tracklist.Index()

	if index >= len(selectedPlaylist.Tracks) {
		return nil
	}

	switch selectedPlaylist.Kind {
	case playlist.NONE, playlist.MYWAVE, playlist.STATION, playlist.ALBUMS, playlist.HISTORY, playlist.LOCAL:
		return nil
	}

	deleteCurrentTrack := index == selectedPlaylist.CurrentTrack

	if len(selectedPlaylist.Tracks) < 2 {
		return m.ShowToast("can't remove last track from queue")
	}

	selectedPlaylist.Tracks = slices.Delete(selectedPlaylist.Tracks, index, index+1)

	if deleteCurrentTrack {
		selectedPlaylist.CurrentTrack = -1
	} else if selectedPlaylist.CurrentTrack > index {
		selectedPlaylist.CurrentTrack--
	}

	if index >= len(selectedPlaylist.Tracks) {
		selectedPlaylist.SelectedTrack = len(selectedPlaylist.Tracks) - 1
	} else {
		selectedPlaylist.SelectedTrack = index
	}

	cmd := m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)
	m.displayPlaylist(selectedPlaylist)

	if m.currentPlaylistIndex >= 0 {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if selectedPlaylist.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}

	return tea.Batch(cmd, m.ShowToast("removed from queue"))
}

func (m *Model) shufflePlaylist(pl *playlist.Item) tea.Cmd {
	var cmds []tea.Cmd
	if pl.Kind == playlist.NONE || pl.Kind == playlist.MYWAVE || pl.Kind == playlist.STATION || pl.Kind == playlist.HISTORY || len(pl.Tracks) == 0 {
		return nil
	}

	currentTrackIndex := pl.CurrentTrack
	selectedTrackIndex := pl.SelectedTrack
	currentTrack := pl.Tracks[currentTrackIndex]
	selectedTrack := pl.Tracks[selectedTrackIndex]

	tracks := make([]api.Track, len(pl.Tracks))
	trackList := make([]tracklist.Item, len(pl.Tracks))
	perm := rand.Perm(len(tracks))

	for i, v := range perm {
		tracks[v] = pl.Tracks[i]
		trackList[v] = tracklist.NewItem(&tracks[v])
		if currentTrack.Id == tracks[v].Id {
			currentTrackIndex = v
		}
		if selectedTrackIndex > 0 && selectedTrack.Id == tracks[v].Id {
			selectedTrackIndex = v
		}
	}

	pl.Tracks = tracks
	pl.SelectedTrack = selectedTrackIndex
	pl.CurrentTrack = currentTrackIndex
	cmds = append(cmds, m.playlists.SetItem(m.playlists.Index(), pl))
	cmds = append(cmds, m.tracklist.SetItems(trackList))
	m.tracklist.Select(selectedTrackIndex)

	if m.currentPlaylistIndex >= 0 {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if pl.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}

	return tea.Batch(cmds...)
}

func (m *Model) albumListActive() bool {
	pl := m.playlists.SelectedItem()
	return pl.Kind == playlist.ALBUMS && len(pl.Albums) > 0 && pl.SelectedAlbum < 0
}

func (m *Model) openAlbum(index int) tea.Cmd {
	pl := m.playlists.SelectedItem()
	if index >= len(pl.Albums) {
		return nil
	}

	var albumTracks []api.Track
	for _, volume := range pl.Albums[index].Volumes {
		albumTracks = append(albumTracks, volume...)
	}

	pl.Tracks = albumTracks
	pl.SelectedAlbum = index
	pl.SelectedTrack = 0
	m.displayPlaylist(pl)
	return m.playlists.SetItem(m.playlists.Index(), pl)
}

func (m *Model) displayPlaylist(pl *playlist.Item) {
	if pl.Kind == playlist.ALBUMS && len(pl.Albums) > 0 && pl.SelectedAlbum < 0 {
		albumList := make([]tracklist.Item, len(pl.Albums))
		for i := range pl.Albums {
			albumList[i] = tracklist.NewAlbumItem(&pl.Albums[i])
		}

		m.tracklist.SetItems(albumList)
		m.tracklist.Select(0)
		m.tracklist.Title = "Liked albums"
		return
	}

	trackList := make([]tracklist.Item, len(pl.Tracks))
	for i := range pl.Tracks {
		item := tracklist.NewItem(&pl.Tracks[i])
		item.PlayCount = pl.Tracks[i].PlayCount
		trackList[i] = item
	}
	if pl.Rotor && len(trackList) > 0 {
		trackList[len(trackList)-1].IsSuggestion = true
	}

	m.tracklist.SetItems(trackList)
	m.tracklist.Select(pl.SelectedTrack)

	switch pl.Kind {
	case playlist.MYWAVE:
		m.tracklist.Title = "My wave"
	case playlist.STATION:
		m.tracklist.Title = pl.Name
	case playlist.LIKES:
		m.tracklist.Title = "Liked tracks"
	case playlist.LOCAL:
		m.tracklist.Title = "Cached tracks"
	case playlist.HISTORY:
		m.tracklist.Title = "History"
	case playlist.ARTIST:
		m.tracklist.Title = "Artist: " + pl.Name
	default:
		if pl.Kind == playlist.ALBUMS && len(pl.Albums) > 0 && pl.SelectedAlbum >= 0 {
			m.tracklist.Title = "Tracks from " + pl.Albums[pl.SelectedAlbum].Title
		} else {
			m.tracklist.Title = "Tracks from " + pl.Name
		}
	}

	if len(pl.Tracks) > 0 {
		var totalMs int
		for _, t := range pl.Tracks {
			totalMs += t.DurationMs
		}
		total := time.Duration(totalMs) * time.Millisecond
		m.tracklist.Title += fmt.Sprintf("  [%d:%02d:%02d]", int(total.Hours()), int(total.Minutes())%60, int(total.Seconds())%60)
	}
}

func (m *Model) indicateCurrentTrackPlaying(playing bool) {
	if m.currentPlaylistIndex < 0 {
		return
	}
	currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
	if currentPlaylist.Kind == playlist.ALBUMS && len(currentPlaylist.Albums) > 0 && currentPlaylist.SelectedAlbum < 0 {
		return
	}
	if currentPlaylist.IsSame(m.playlists.SelectedItem()) && currentPlaylist.CurrentTrack < len(m.tracklist.Items()) {
		track := m.tracklist.Items()[currentPlaylist.CurrentTrack]
		track.IsPlaying = playing
		m.tracklist.SetItem(currentPlaylist.CurrentTrack, track)

		if playing {
			m.tracklist.Select(currentPlaylist.CurrentTrack)
		}
	}
}

func (m *Model) jumpToPlayingTrack() tea.Cmd {
	if m.currentPlaylistIndex < 0 {
		return nil
	}

	currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
	m.playlists.Select(m.currentPlaylistIndex)
	m.displayPlaylist(currentPlaylist)

	if currentPlaylist.CurrentTrack >= 0 && currentPlaylist.CurrentTrack < len(currentPlaylist.Tracks) {
		m.tracklist.Select(currentPlaylist.CurrentTrack)
	}

	if m.tracker.IsPlaying() {
		m.indicateCurrentTrackPlaying(true)
	}

	return nil
}

func (m *Model) toggleQueue() {
	if m.showQueue {
		m.showQueue = false
		selectedPlaylist := m.playlists.SelectedItem()
		m.displayPlaylist(selectedPlaylist)
		if m.currentPlaylistIndex >= 0 {
			currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
			if selectedPlaylist.IsSame(currentPlaylist) {
				m.tracklist.Select(currentPlaylist.SelectedTrack)
				if m.tracker.IsPlaying() {
					m.indicateCurrentTrackPlaying(true)
				}
			}
		}
		return
	}

	if m.currentPlaylistIndex < 0 {
		return
	}

	currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
	if currentPlaylist.CurrentTrack < 0 || currentPlaylist.CurrentTrack >= len(currentPlaylist.Tracks)-1 {
		return
	}

	upNext := currentPlaylist.Tracks[currentPlaylist.CurrentTrack+1:]
	if len(upNext) == 0 {
		return
	}

	m.showQueue = true
	trackList := make([]tracklist.Item, len(upNext))
	for i := range upNext {
		trackList[i] = tracklist.NewItem(&upNext[i])
	}

	m.tracklist.SetItems(trackList)
	m.tracklist.Select(0)
	m.tracklist.Title = "Up Next"
}

func (m *Model) sortPlaylist() tea.Cmd {
	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 || selectedPlaylist.Kind == playlist.MYWAVE || selectedPlaylist.Kind == playlist.STATION || selectedPlaylist.Kind == playlist.HISTORY || selectedPlaylist.Kind == playlist.NONE {
		return nil
	}

	m.sortMode = (m.sortMode + 1) % 4

	currentTrackId := ""
	if selectedPlaylist.CurrentTrack >= 0 && selectedPlaylist.CurrentTrack < len(selectedPlaylist.Tracks) {
		currentTrackId = selectedPlaylist.Tracks[selectedPlaylist.CurrentTrack].Id
	}
	selectedTrackId := ""
	if m.tracklist.Index() >= 0 && m.tracklist.Index() < len(selectedPlaylist.Tracks) {
		selectedTrackId = selectedPlaylist.Tracks[m.tracklist.Index()].Id
	}

	switch m.sortMode {
	case 1:
		slices.SortFunc(selectedPlaylist.Tracks, func(a, b api.Track) int {
			return strings.Compare(a.Title, b.Title)
		})
	case 2:
		slices.SortFunc(selectedPlaylist.Tracks, func(a, b api.Track) int {
			aa := ""
			bb := ""
			if len(a.Artists) > 0 {
				aa = a.Artists[0].Name
			}
			if len(b.Artists) > 0 {
				bb = b.Artists[0].Name
			}
			return strings.Compare(aa, bb)
		})
	case 3:
		slices.SortFunc(selectedPlaylist.Tracks, func(a, b api.Track) int {
			return a.DurationMs - b.DurationMs
		})
	default:
		return nil
	}

	for i, t := range selectedPlaylist.Tracks {
		if t.Id == currentTrackId {
			selectedPlaylist.CurrentTrack = i
		}
		if t.Id == selectedTrackId {
			selectedPlaylist.SelectedTrack = i
		}
	}

	cmd := m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)
	m.displayPlaylist(selectedPlaylist)
	m.tracklist.Select(selectedPlaylist.SelectedTrack)

	if m.currentPlaylistIndex >= 0 {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if selectedPlaylist.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}

	return cmd
}

func (m *Model) exportPlaylist() tea.Cmd {
	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return m.ShowToast("nothing to export")
	}

	downloadDir := config.Current.DownloadDir
	if downloadDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return m.ShowToast("export: no home dir")
		}
		downloadDir = filepath.Join(home, "Music", "wellya")
	}

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return m.ShowToast("export: mkdir failed")
	}

	baseName := sanitizeFilename(selectedPlaylist.Name)
	if baseName == "" {
		baseName = "playlist"
	}
	filename := baseName + ".m3u"
	filePath := filepath.Join(downloadDir, filename)

	for i := 1; ; i++ {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		filename = fmt.Sprintf("%s_%d.m3u", baseName, i)
		filePath = filepath.Join(downloadDir, filename)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return m.ShowToast("export: create failed")
	}
	defer file.Close()

	fmt.Fprintln(file, "#EXTM3U")
	for _, t := range selectedPlaylist.Tracks {
		artist := ""
		if len(t.Artists) > 0 {
			artist = t.Artists[0].Name
			if len(t.Artists) > 1 {
				artist = helpers.ArtistList(t.Artists)
			}
		}
		durationSec := t.DurationMs / 1000
		fmt.Fprintf(file, "#EXTINF:%d,%s - %s\n", durationSec, artist, t.Title)
		link := api.ShareTrackLink(&t)
		if link == "" {
			link = t.Id
		}
		fmt.Fprintln(file, link)
	}

	return m.ShowToast("Exported: " + filename)
}

func (m *Model) showStats() tea.Cmd {
	totalPlaylists := len(m.playlists.Items())
	totalTracks := 0
	var totalMs int
	cachedCount := len(m.cachedTracksMap)
	likedCount := 0
	for _, pl := range m.playlists.Items() {
		totalTracks += len(pl.Tracks)
		for _, t := range pl.Tracks {
			totalMs += t.DurationMs
		}
		if pl.Kind == playlist.LIKES {
			likedCount = len(pl.Tracks)
		}
	}
	totalDur := time.Duration(totalMs) * time.Millisecond
	historyCount := len(m.historyTracks)
	msg := fmt.Sprintf("PL:%d TR:%d ♥%d 💿%d hist:%d dur:%d:%02d", totalPlaylists, totalTracks, likedCount, cachedCount, historyCount, int(totalDur.Hours()), int(totalDur.Minutes())%60)
	return m.ShowToast(msg)
}

func (m *Model) moveTrack(direction int) tea.Cmd {
	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 || selectedPlaylist.Kind == playlist.MYWAVE || selectedPlaylist.Kind == playlist.STATION || selectedPlaylist.Kind == playlist.HISTORY || selectedPlaylist.Kind == playlist.NONE {
		return nil
	}

	idx := m.tracklist.Index()
	newIdx := idx + direction
	if newIdx < 0 || newIdx >= len(selectedPlaylist.Tracks) {
		return nil
	}

	selectedPlaylist.Tracks[idx], selectedPlaylist.Tracks[newIdx] = selectedPlaylist.Tracks[newIdx], selectedPlaylist.Tracks[idx]

	if selectedPlaylist.CurrentTrack == idx {
		selectedPlaylist.CurrentTrack = newIdx
	} else if selectedPlaylist.CurrentTrack == newIdx {
		selectedPlaylist.CurrentTrack = idx
	}

	selectedPlaylist.SelectedTrack = newIdx

	cmd := m.playlists.SetItem(m.playlists.Index(), selectedPlaylist)

	m.displayPlaylist(selectedPlaylist)
	m.tracklist.Select(newIdx)

	if m.currentPlaylistIndex >= 0 {
		currentPlaylist := m.playlists.Items()[m.currentPlaylistIndex]
		if selectedPlaylist.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}

	return cmd
}
