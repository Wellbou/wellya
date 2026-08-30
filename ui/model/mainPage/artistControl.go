package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/helpers"
)

func (m *Model) browseSelectedTrackArtist() tea.Cmd {
	if m.client == nil {
		return nil
	}

	selectedPlaylist := m.playlists.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	selectedTrack := m.tracklist.SelectedItem().Track
	if len(selectedTrack.Artists) == 0 {
		return nil
	}

	artist := selectedTrack.Artists[0]
	artistTracks, err := m.client.ArtistPopularTracks(artist.Id)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks: %s", artist.Name, err)
		m.tracker.ShowError("artist tracks")
		return nil
	}

	tracks, err := m.client.Tracks(artistTracks.Tracks)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks full info: %s", artist.Name, err)
		m.tracker.ShowError("artist tracks info")
		return nil
	}

	if len(tracks) == 0 {
		m.tracker.ShowError("artist has no tracks")
		return nil
	}

	artistName := helpers.ArtistList([]api.Artist{artist})

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

	artistItem := &playlist.Item{
		Name:    artistName,
		Kind:    playlist.ARTIST,
		Active:  true,
		Subitem: true,
		Tracks:  tracks,
	}

	m.playlists.InsertItem(insertIndex, artistItem)

	if insertIndex <= m.playlists.Index() {
		m.playlists.Select(m.playlists.Index() + 1)
	}

	m.displayPlaylist(artistItem)

	return nil
}
