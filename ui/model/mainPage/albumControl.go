package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
)

func (m *Model) goToAlbum() tea.Cmd {
	if m.client == nil {
		return nil
	}

	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	selectedTrack := m.tracklist.SelectedItem().Track
	if len(selectedTrack.Albums) == 0 {
		return nil
	}

	albumId := uint64(selectedTrack.Albums[0].Id)
	album, err := m.client.Album(albumId, true)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain album [%d] tracks: %s", albumId, err)
		m.tracker.ShowError("album tracks")
		return nil
	}

	var albumTracks []api.Track
	for _, volume := range album.Volumes {
		albumTracks = append(albumTracks, volume...)
	}

	if len(albumTracks) == 0 {
		return nil
	}

	playlists := m.playlists.Items()
	insertIndex := m.playlists.Index() + 1
	for i := insertIndex; i < len(playlists); i++ {
		if playlists[i].Kind >= playlist.USER {
			insertIndex = i
			break
		}
		if i == len(playlists)-1 {
			insertIndex = len(playlists)
		}
	}

	albumItem := &playlist.Item{
		Name:    album.Title,
		Kind:    playlist.ALBUMS,
		Active:  true,
		Subitem: true,
		Tracks:  albumTracks,
	}

	m.playlists.InsertItem(insertIndex, albumItem)

	if insertIndex <= m.playlists.Index() {
		m.playlists.Select(m.playlists.Index() + 1)
	}

	m.displayPlaylist(albumItem)

	return nil
}
