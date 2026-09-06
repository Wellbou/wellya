package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
)

func (m *Model) dislikePlayingTrack() tea.Cmd {
	var currentPlaylist *playlist.Item
	currentPlaylist = m.currentPlaylist()

	track := m.tracker.CurrentTrack()
	return m.dislikeTrack(track, currentPlaylist)
}

func (m *Model) dislikeSelectedTrack() tea.Cmd {
	if m.currentPlaylistIndex < 0 {
		return nil
	}

	selectedPlaylist := m.activePlaylists().SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		return nil
	}

	track := m.tracklist.SelectedItem().Track
	if track == nil {
		return nil
	}
	return m.dislikeTrack(track, selectedPlaylist)
}

func (m *Model) dislikeTrack(track *api.Track, pl *playlist.Item) tea.Cmd {
	if pl != nil && pl.Rotor {
		ev := api.NewTrackFeedbackEvent(api.EV_TRACK_DISLIKED, track, 0)
		go m.client.RotorSessionFeedback(pl.SessionId, api.NewFeedback(pl.SessionBatch, ev))
		log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+track.Title)
	}
	return nil
}
