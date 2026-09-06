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

	selectedPlaylist := m.activePlaylists().SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	selectedTrack := m.tracklist.SelectedItem().Track
	if selectedTrack == nil || len(selectedTrack.Albums) == 0 {
		return nil
	}

	albumId := uint64(selectedTrack.Albums[0].Id)
	go m.fetchAlbumTracks(m.client, albumId)
	return nil
}

func (m *Model) fetchAlbumTracks(client *api.YaMusicClient, albumId uint64) {
	album, err := client.Album(albumId, true)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain album [%d] tracks: %s", albumId, err)
		m.Send(errorToastMsg{reason: "album tracks"})
		return
	}

	var albumTracks []api.Track
	for _, volume := range album.Volumes {
		albumTracks = append(albumTracks, volume...)
	}

	if len(albumTracks) == 0 {
		m.Send(errorToastMsg{reason: "album has no tracks"})
		return
	}

	m.Send(browsedItemMsg{item: &playlist.Item{
		Name:    album.Title,
		Kind:    playlist.ALBUMS,
		Active:  true,
		Subitem: true,
		Tracks:  albumTracks,
	}})
}
