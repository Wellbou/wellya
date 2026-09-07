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
	"github.com/wellbou/wellya/cache"
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
		if m.client == nil {
			m.tracker.ShowError("not logged in")
			return nil
		}

		var foundPlaylist *playlist.Item
		for i := range playlists {
			if playlists[i].Active && playlists[i].Kind >= playlist.USER {
				if strings.EqualFold(playlists[i].Name, inputVal) {
					foundPlaylist = playlists[i]
					break
				}
			}
		}

		if len(m.tracklist.Items()) == 0 || m.tracklist.SelectedItem().Track == nil {
			return nil
		}
		trackCopy := *m.tracklist.SelectedItem().Track

		if foundPlaylist == nil {
			go m.createPlaylistAndAdd(m.client, inputVal, trackCopy)
			m.isAddPlaylistActive = false
			return nil
		}

		if selectedPlaylist.Kind == foundPlaylist.Kind {
			return nil
		}

		kind := foundPlaylist.Kind
		rev := foundPlaylist.Revision
		pos := len(foundPlaylist.Tracks)
		go m.addTrackToPlaylist(m.client, kind, rev, pos, trackCopy)
		m.isAddPlaylistActive = false
		return nil
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

type playlistCreatedMsg struct {
	item  *playlist.Item
	track api.Track
}

type playlistTrackAddedMsg struct {
	kind  uint64
	rev   int
	track api.Track
}

func (m *Model) createPlaylistAndAdd(client *api.YaMusicClient, name string, track api.Track) {
	pl, err := client.CreatePlaylist(name, false)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to create playlist [%s]: %s", name, err)
		m.Send(errorToastMsg{reason: "playlist create"})
		return
	}
	m.Send(playlistCreatedMsg{
		item: &playlist.Item{
			Name:     pl.Title,
			Kind:     uint64(pl.Kind),
			Revision: int(pl.Revision),
			Active:   true,
			Subitem:  true,
		},
		track: track,
	})
}

func (m *Model) addTrackToPlaylist(client *api.YaMusicClient, kind uint64, rev, pos int, track api.Track) {
	pl, err := client.AddToPlaylist(kind, rev, pos, string(track.Id))
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to add track [%s]: %s", track.Id, err)
		m.Send(errorToastMsg{reason: "playlist add"})
		return
	}
	m.Send(playlistTrackAddedMsg{kind: kind, rev: int(pl.Revision), track: track})
}

func (m *Model) applyPlaylistCreated(item *playlist.Item, track api.Track) {
	playlists := m.playlists.Items()
	at := len(playlists)
	for i := range playlists {
		if playlists[i].Active && playlists[i].Kind >= playlist.USER {
			at = i
			break
		}
	}
	m.playlists.InsertItem(at, item)
	if at <= m.playlists.Index() {
		m.playlists.Select(m.playlists.Index() + 1)
	}
	if !m.currentIsRadio && m.currentPlaylistIndex >= m.playlists.Index() {
		m.currentPlaylistIndex++
	}
	go m.addTrackToPlaylist(m.client, item.Kind, item.Revision, 0, track)
}

func (m *Model) quickAddSelectedTrack() tea.Cmd {
	if m.client == nil {
		m.tracker.ShowError("not logged in")
		return nil
	}
	if m.lastAddPlaylistKind == 0 {
		return m.ShowToast("no recent playlist — press a first")
	}
	pl := m.activePlaylists().SelectedItem()
	idx := m.realTrackIndex(pl)
	if idx < 0 || idx >= len(pl.Tracks) {
		return nil
	}
	trackCopy := pl.Tracks[idx]
	var target *playlist.Item
	for _, it := range m.playlists.Items() {
		if it.Kind == m.lastAddPlaylistKind {
			target = it
			break
		}
	}
	if target == nil {
		m.lastAddPlaylistKind = 0
		return m.ShowToast("recent playlist is gone — press a first")
	}
	go m.addTrackToPlaylist(m.client, target.Kind, target.Revision, len(target.Tracks), trackCopy)
	if next := m.tracklist.Index() + 1; next < len(m.tracklist.Items()) {
		m.tracklist.Select(next)
	}
	return m.ShowToast("added to " + target.Name)
}

func (m *Model) applyPlaylistTrackAdded(kind uint64, rev int, track api.Track) {
	m.lastAddPlaylistKind = kind
	playlists := m.playlists.Items()
	for i := range playlists {
		if playlists[i].Kind != kind {
			continue
		}
		playlists[i].Revision = rev
		playlists[i].Tracks = append(playlists[i].Tracks, track)
		m.playlists.SetItem(i, playlists[i])
		return
	}
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
	if m.client == nil {
		m.tracker.ShowError("not logged in")
		return nil
	}

	kind := m.playlists.SelectedItem().Kind
	name := m.playlists.SelectedItem().Name
	go m.renamePlaylist(m.client, kind, name, newName)

	return cmd
}

type renameDoneMsg struct {
	kind  uint64
	title string
	rev   int
}

func (m *Model) renamePlaylist(client *api.YaMusicClient, kind uint64, oldName, newName string) {
	pl, err := client.RenamePlaylist(kind, newName)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to rename playlist [%s] to '%s': %s", oldName, newName, err)
		m.Send(errorToastMsg{reason: "playlist rename"})
		return
	}
	m.Send(renameDoneMsg{kind: kind, title: pl.Title, rev: int(pl.Revision)})
}

func (m *Model) applyRename(kind uint64, title string, rev int) {
	playlists := m.playlists.Items()
	for i := range playlists {
		if playlists[i].Kind != kind {
			continue
		}
		playlists[i].Name = title
		playlists[i].Revision = rev
		m.playlists.SetItem(i, playlists[i])
		return
	}
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
	if len(pl.Tracks) < 2 {
		switch pl.Kind {
		case playlist.NONE, playlist.MYWAVE, playlist.STATION, playlist.ALBUMS, playlist.HISTORY, playlist.LIKES, playlist.LOCAL:
		default:
			msg = "Last track! The whole playlist will be deleted. Continue? (y/n)"
		}
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
		if m.client == nil {
			m.tracker.ShowError("not logged in")
			return nil
		}
		kind := pl.Kind
		rev := pl.Revision
		name := pl.Name
		trackId := ""
		if index < len(pl.Tracks) {
			trackId = string(pl.Tracks[index].Id)
		}
		isRadio := m.activePlaylists() == m.radioPlaylists
		client := m.client

		if len(pl.Tracks) < 2 {
			go func() {
				if err := client.RemovePlaylist(kind); err != nil {
					log.Print(log.LVL_ERROR, "failed to remove playlist [%s]: %s", name, err)
					m.Send(errorToastMsg{reason: "playlist remove"})
					return
				}
				m.Send(playlistRemovedMsg{kind: kind, isRadio: isRadio})
			}()
			return nil
		}

		go func() {
			newpl, err := client.RemoveFromPlaylist(kind, rev, index)
			if err != nil {
				log.Print(log.LVL_ERROR, "failed to remove track [%s] from playlist [%s]: %s", trackId, name, err)
				m.Send(errorToastMsg{reason: "playlist remove track"})
				return
			}
			m.Send(playlistTrackRemovedMsg{kind: kind, rev: int(newpl.Revision), index: index, isRadio: isRadio})
		}()
		return nil
	}
}

type playlistRemovedMsg struct {
	kind    uint64
	isRadio bool
}

type playlistTrackRemovedMsg struct {
	kind    uint64
	rev     int
	index   int
	isRadio bool
}

func (m *Model) targetPlaylists(isRadio bool) *playlist.Model {
	if isRadio {
		return m.radioPlaylists
	}
	return m.playlists
}

func (m *Model) applyPlaylistRemoved(kind uint64, isRadio bool) {
	lists := m.targetPlaylists(isRadio)
	items := lists.Items()
	at := -1
	for i := range items {
		if items[i].Kind == kind {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	lists.RemoveItem(at)
	if !isRadio == !m.currentIsRadio {
		if m.currentPlaylistIndex == at {
			m.currentPlaylistIndex = -1
		} else if m.currentPlaylistIndex > at {
			m.currentPlaylistIndex--
		}
	}
	if len(lists.Items()) <= lists.Index() {
		lists.Select(0)
	}
	m.displayPlaylist(lists.SelectedItem())
}

func (m *Model) applyPlaylistTrackRemoved(kind uint64, rev, index int, isRadio bool) {
	lists := m.targetPlaylists(isRadio)
	var pl *playlist.Item
	at := -1
	for i, it := range lists.Items() {
		if it.Kind == kind {
			pl = it
			at = i
			break
		}
	}
	if pl == nil || at < 0 || index < 0 || index >= len(pl.Tracks) {
		return
	}

	pl.Revision = rev
	pl.Tracks = slices.Delete(pl.Tracks, index, index+1)
	if index >= len(pl.Tracks) {
		pl.SelectedTrack = len(pl.Tracks) - 1
	} else {
		pl.SelectedTrack = index
	}
	deleteCurrentTrack := index == pl.CurrentTrack
	if deleteCurrentTrack {
		pl.CurrentTrack = -1
	} else if pl.CurrentTrack > index {
		pl.CurrentTrack--
	}
	lists.SetItem(at, pl)
	if lists.SelectedItem().IsSame(pl) {
		m.displayPlaylist(pl)
	}

	if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
		if pl.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(!deleteCurrentTrack)
		}
	}
}

func (m *Model) removeFromQueue() tea.Cmd {
	selectedPlaylist := m.activePlaylists().SelectedItem()
	index := m.realTrackIndex(selectedPlaylist)

	if index < 0 || index >= len(selectedPlaylist.Tracks) {
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

	active := m.activePlaylists()
	cmd := active.SetItem(active.Index(), selectedPlaylist)
	m.displayPlaylist(selectedPlaylist)

	if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
		if selectedPlaylist.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}
	m.refreshQueueView()

	return tea.Batch(cmd, m.ShowToast("hidden locally until reload (not synced)"))
}

func (m *Model) shufflePlaylist(pl *playlist.Item) tea.Cmd {
	var cmds []tea.Cmd
	if pl.Kind == playlist.NONE || pl.Kind == playlist.MYWAVE || pl.Kind == playlist.STATION || pl.Kind == playlist.HISTORY || len(pl.Tracks) == 0 {
		return nil
	}

	currentTrackIndex := pl.CurrentTrack
	selectedTrackIndex := pl.SelectedTrack
	if currentTrackIndex < 0 || currentTrackIndex >= len(pl.Tracks) {
		currentTrackIndex = 0
	}
	if selectedTrackIndex < 0 || selectedTrackIndex >= len(pl.Tracks) {
		selectedTrackIndex = 0
	}
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
	active := m.activePlaylists()
	cmds = append(cmds, active.SetItem(active.Index(), pl))
	cmds = append(cmds, m.tracklist.SetItems(trackList))
	m.tracklist.Select(selectedTrackIndex)

	if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
		if pl.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}

	return tea.Batch(cmds...)
}

func (m *Model) albumListActive() bool {
	pl := m.activePlaylists().SelectedItem()
	return pl.Kind == playlist.ALBUMS && len(pl.Albums) > 0 && pl.SelectedAlbum < 0
}

func (m *Model) openAlbum(index int) tea.Cmd {
	pl := m.activePlaylists().SelectedItem()
	if m.tracklist.FilterValue() != "" && len(m.tracklist.Items()) > 0 {
		if sel := m.tracklist.SelectedItem(); sel.Album != nil {
			for i := range pl.Albums {
				if &pl.Albums[i] == sel.Album {
					index = i
					break
				}
			}
		}
	}
	if index < 0 || index >= len(pl.Albums) {
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
	return m.activePlaylists().SetItem(m.activePlaylists().Index(), pl)
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
		item.PlayCount = int(pl.Tracks[i].PlayCount)
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
			totalMs += int(t.DurationMs)
		}
		total := time.Duration(totalMs) * time.Millisecond
		m.tracklist.Title += fmt.Sprintf("  [%d:%02d:%02d]", int(total.Hours()), int(total.Minutes())%60, int(total.Seconds())%60)
	}
}

func (m *Model) indicateCurrentTrackPlaying(playing bool) {
	currentPlaylist := m.currentPlaylist()
	if currentPlaylist == nil {
		return
	}
	if currentPlaylist.Kind == playlist.ALBUMS && len(currentPlaylist.Albums) > 0 && currentPlaylist.SelectedAlbum < 0 {
		return
	}
	if !currentPlaylist.IsSame(m.activePlaylists().SelectedItem()) {
		return
	}
	visibleIdx := -1
	items := m.tracklist.Items()
	if currentPlaylist.CurrentTrack >= 0 && currentPlaylist.CurrentTrack < len(currentPlaylist.Tracks) {
		want := currentPlaylist.Tracks[currentPlaylist.CurrentTrack].Id
		for i := range items {
			if items[i].Track != nil && items[i].Track.Id == want {
				visibleIdx = i
				break
			}
		}
		if visibleIdx < 0 && m.tracklist.FilterValue() == "" && !m.showQueue {
			visibleIdx = currentPlaylist.CurrentTrack
		}
	}
	if visibleIdx < 0 || visibleIdx >= len(items) {
		return
	}
	track := items[visibleIdx]
	track.IsPlaying = playing
	m.tracklist.SetItem(visibleIdx, track)

	if playing {
		m.tracklist.Select(visibleIdx)
	}
}

func (m *Model) refreshQueueView() {
	if !m.showQueue {
		return
	}
	m.showQueue = false
	m.toggleQueue()
}

func (m *Model) jumpToPlayingTrack() tea.Cmd {
	currentPlaylist := m.currentPlaylist()
	if currentPlaylist == nil {
		return nil
	}

	if m.currentIsRadio != m.isRadioTab {
		m.toggleRadioTab()
	}
	m.activePlaylists().Select(m.currentPlaylistIndex)
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
		selectedPlaylist := m.activePlaylists().SelectedItem()
		m.displayPlaylist(selectedPlaylist)
		if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
			if selectedPlaylist.IsSame(currentPlaylist) {
				m.tracklist.Select(currentPlaylist.SelectedTrack)
				if m.tracker.IsPlaying() {
					m.indicateCurrentTrackPlaying(true)
				}
			}
		}
		return
	}

	currentPlaylist := m.currentPlaylist()
	if currentPlaylist == nil {
		return
	}
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
	selectedPlaylist := m.activePlaylists().SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 || selectedPlaylist.Kind == playlist.MYWAVE || selectedPlaylist.Kind == playlist.STATION || selectedPlaylist.Kind == playlist.HISTORY || selectedPlaylist.Kind == playlist.NONE {
		return nil
	}

	m.sortMode = (m.sortMode + 1) % 4

	currentTrackId := ""
	if selectedPlaylist.CurrentTrack >= 0 && selectedPlaylist.CurrentTrack < len(selectedPlaylist.Tracks) {
		currentTrackId = string(selectedPlaylist.Tracks[selectedPlaylist.CurrentTrack].Id)
	}
	selectedTrackId := ""
	if idx := m.realTrackIndex(selectedPlaylist); idx >= 0 {
		selectedTrackId = string(selectedPlaylist.Tracks[idx].Id)
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
			return int(a.DurationMs - b.DurationMs)
		})
	default:
		return nil
	}

	for i, t := range selectedPlaylist.Tracks {
		if string(t.Id) == currentTrackId {
			selectedPlaylist.CurrentTrack = i
		}
		if string(t.Id) == selectedTrackId {
			selectedPlaylist.SelectedTrack = i
		}
	}

	active := m.activePlaylists()
	cmd := active.SetItem(active.Index(), selectedPlaylist)
	m.displayPlaylist(selectedPlaylist)
	m.tracklist.Select(selectedPlaylist.SelectedTrack)
	m.refreshQueueView()

	if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
		if selectedPlaylist.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}

	return cmd
}

func (m *Model) exportPlaylist() tea.Cmd {
	selectedPlaylist := m.activePlaylists().SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return m.ShowToast("nothing to export")
	}

	downloadDir := config.Current.DownloadDir
	if downloadDir == "" {
		downloadDir = config.MusicDir()
		if downloadDir == "" {
			return m.ShowToast("export: no home dir")
		}
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
		durationSec := int(t.DurationMs) / 1000
		fmt.Fprintf(file, "#EXTINF:%d,%s - %s\n", durationSec, artist, t.Title)
		link := ""
		if id := string(t.Id); m.cachedTracksMap[id] {
			if p := cache.Path(id); p != "" {
				if _, err := os.Stat(p); err == nil {
					link = p
				}
			}
		}
		if link == "" {
			link = api.ShareTrackLink(&t)
		}
		if link == "" {
			link = string(t.Id)
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
			totalMs += int(t.DurationMs)
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

func (m *Model) enqueueNextTrack(track *api.Track) tea.Cmd {
	if track == nil || m.currentPlaylistIndex < 0 {
		return nil
	}
	curPls := m.currentPlaylists()
	if m.currentPlaylistIndex < 0 || m.currentPlaylistIndex >= len(curPls.Items()) {
		return nil
	}
	pl := curPls.Items()[m.currentPlaylistIndex]
	if pl.Kind == playlist.NONE || pl.Kind == playlist.HISTORY {
		return nil
	}
	insertAt := pl.CurrentTrack + 1
	if insertAt < 0 || insertAt > len(pl.Tracks) {
		insertAt = len(pl.Tracks)
	}
	tCopy := *track
	pl.Tracks = append(pl.Tracks[:insertAt], append([]api.Track{tCopy}, pl.Tracks[insertAt:]...)...)
	curPls.SetItem(m.currentPlaylistIndex, pl)
	if curPls == m.activePlaylists() {
		m.displayPlaylist(pl)
	}
	return m.ShowToast("queued: " + track.Title)
}

func (m *Model) playNowTrack(track *api.Track) tea.Cmd {
	if track == nil {
		return nil
	}
	curPls := m.currentPlaylists()
	if m.currentPlaylistIndex < 0 || m.currentPlaylistIndex >= len(curPls.Items()) {
		return nil
	}
	pl := curPls.Items()[m.currentPlaylistIndex]
	if pl.Kind == playlist.NONE || pl.Kind == playlist.HISTORY {
		return nil
	}
	insertAt := pl.CurrentTrack + 1
	if insertAt < 0 || insertAt > len(pl.Tracks) {
		insertAt = len(pl.Tracks)
	}
	tCopy := *track
	pl.Tracks = append(pl.Tracks[:insertAt], append([]api.Track{tCopy}, pl.Tracks[insertAt:]...)...)
	pl.CurrentTrack = insertAt
	pl.SelectedTrack = insertAt
	curPls.SetItem(m.currentPlaylistIndex, pl)
	m.playTrack(&tCopy)
	if curPls == m.activePlaylists() {
		m.displayPlaylist(pl)
	}
	return nil
}

type regionSkipMsg struct {
	count int
}

func (m *Model) moveTrack(direction int) tea.Cmd {
	selectedPlaylist := m.activePlaylists().SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 || selectedPlaylist.Kind == playlist.MYWAVE || selectedPlaylist.Kind == playlist.STATION || selectedPlaylist.Kind == playlist.HISTORY || selectedPlaylist.Kind == playlist.NONE {
		return nil
	}

	idx := m.realTrackIndex(selectedPlaylist)
	newIdx := idx + direction
	if idx < 0 || newIdx < 0 || newIdx >= len(selectedPlaylist.Tracks) {
		return nil
	}

	selectedPlaylist.Tracks[idx], selectedPlaylist.Tracks[newIdx] = selectedPlaylist.Tracks[newIdx], selectedPlaylist.Tracks[idx]

	if selectedPlaylist.CurrentTrack == idx {
		selectedPlaylist.CurrentTrack = newIdx
	} else if selectedPlaylist.CurrentTrack == newIdx {
		selectedPlaylist.CurrentTrack = idx
	}

	selectedPlaylist.SelectedTrack = newIdx

	cmd := m.activePlaylists().SetItem(m.activePlaylists().Index(), selectedPlaylist)

	m.displayPlaylist(selectedPlaylist)
	movedId := selectedPlaylist.Tracks[newIdx].Id
	for i, it := range m.tracklist.Items() {
		if it.Track != nil && it.Track.Id == movedId {
			m.tracklist.Select(i)
			break
		}
	}

	if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
		if selectedPlaylist.IsSame(currentPlaylist) && m.tracker.IsPlaying() {
			m.indicateCurrentTrackPlaying(true)
		}
	}
	m.refreshQueueView()

	return cmd
}