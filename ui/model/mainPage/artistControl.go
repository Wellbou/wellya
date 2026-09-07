package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/helpers"
)

type browsedItemMsg struct {
	item *playlist.Item
	err  string
}

func (m *Model) browseSelectedTrackArtist() tea.Cmd {
	if m.client == nil {
		return nil
	}

	selectedPlaylist := m.activePlaylists().SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	selectedTrack := m.tracklist.SelectedItem().Track
	if selectedTrack == nil || len(selectedTrack.Artists) == 0 {
		return nil
	}

	artist := selectedTrack.Artists[0]
	go m.fetchArtistTracks(m.client, artist)
	return nil
}

func (m *Model) fetchArtistTracks(client *api.YaMusicClient, artist api.Artist) {
	artistTracks, err := client.ArtistPopularTracks(uint64(artist.Id))
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks: %s", artist.Name, err)
		m.Send(errorToastMsg{reason: "artist tracks"})
		return
	}

	tracks, err := client.Tracks(artistTracks.Tracks)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks full info: %s", artist.Name, err)
		m.Send(errorToastMsg{reason: "artist tracks info"})
		return
	}

	if len(tracks) == 0 {
		m.Send(errorToastMsg{reason: "artist has no tracks"})
		return
	}

	artistName := helpers.ArtistList([]api.Artist{artist})
	m.Send(browsedItemMsg{item: &playlist.Item{
		Name:    artistName,
		Kind:    playlist.ARTIST,
		Active:  true,
		Subitem: true,
		Tracks:  tracks,
	}})
}

func (m *Model) applyBrowsedItem(item *playlist.Item) {
	active := m.activePlaylists()
	m.navStack = append(m.navStack, navPos{isRadio: m.isRadioTab, index: active.Index()})
	if len(m.navStack) > 32 {
		m.navStack = m.navStack[len(m.navStack)-32:]
	}
	item.Browsed = true
	playlists := active.Items()
	insertIndex := active.Index() + 1
	for i := insertIndex; i < len(playlists); i++ {
		if playlists[i].Kind >= playlist.USER {
			insertIndex = i
			break
		}
		if i == len(playlists)-1 {
			insertIndex = len(playlists)
		}
	}

	active.InsertItem(insertIndex, item)
	active.Select(insertIndex)
	if active == m.currentPlaylists() && insertIndex <= m.currentPlaylistIndex {
		m.currentPlaylistIndex++
	}

	m.displayPlaylist(item)
}
