package mainpage

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
)

type albumLikeDoneMsg struct {
	albumId uint64
	title   string
	unlike  bool
}

func (m *Model) likeSelectedAlbum() tea.Cmd {
	if m.client == nil {
		return nil
	}
	pl := m.activePlaylists().SelectedItem()
	if len(pl.Albums) == 0 || len(m.tracklist.Items()) == 0 {
		return nil
	}
	idx := m.tracklist.Index()
	if sel := m.tracklist.SelectedItem(); sel.Album != nil {
		for i := range pl.Albums {
			if &pl.Albums[i] == sel.Album {
				idx = i
				break
			}
		}
	}
	if idx < 0 || idx >= len(pl.Albums) {
		return nil
	}
	albumId := uint64(pl.Albums[idx].Id)
	name := pl.Albums[idx].Title
	unlike := m.likedAlbumsMap[albumId]
	client := m.client
	go func() {
		var err error
		if unlike {
			err = client.UnlikeAlbum(albumId)
		} else {
			err = client.LikeAlbum(albumId)
		}
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to like/unlike album [%d]: %s", albumId, err)
			m.Send(errorToastMsg{reason: "album like failed"})
			return
		}
		m.Send(albumLikeDoneMsg{albumId: albumId, title: name, unlike: unlike})
	}()
	return nil
}

func (m *Model) applyAlbumLike(albumId uint64, title string, unlike bool) tea.Cmd {
	if unlike {
		delete(m.likedAlbumsMap, albumId)
		return m.ShowToast("album unliked: " + title)
	}
	m.likedAlbumsMap[albumId] = true
	return m.ShowToast("album liked: " + title)
}

func (m *Model) likePlayingTrack() tea.Cmd {
	var currentPlaylist *playlist.Item
	currentPlaylist = m.currentPlaylist()

	track := m.tracker.CurrentTrack()
	return m.likeTrack(track, currentPlaylist)
}

func (m *Model) likeSelectedTrack() tea.Cmd {
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
	return m.likeTrack(track, selectedPlaylist)
}

type likeDoneMsg struct {
	trackId string
	unlike  bool
	track   api.Track
}

func (m *Model) likeTrack(track *api.Track, pl *playlist.Item) tea.Cmd {
	if track == nil || m.client == nil {
		return nil
	}
	id := string(track.Id)
	unlike := m.likedTracksMap[id]
	trackCopy := *track
	client := m.client
	var sessionId, sessionBatch string
	var evType api.TrackEventType
	var rotor bool
	if pl != nil && pl.Rotor {
		rotor = true
		sessionId = pl.SessionId
		sessionBatch = pl.SessionBatch
		evType = api.EV_TRACK_LIKED
		if unlike {
			evType = api.EV_TRACK_UNLIKED
		}
	}

	go func() {
		var err error
		if unlike {
			err = client.UnlikeTrack(id)
		} else {
			err = client.LikeTrack(id)
		}
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to like/unlike track [%s]: %s", id, err)
			m.Send(errorToastMsg{reason: "like failed"})
			return
		}
		if rotor {
			ev := api.NewTrackFeedbackEvent(evType, &trackCopy, 0)
			go client.RotorSessionFeedback(sessionId, api.NewFeedback(sessionBatch, ev))
			log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+trackCopy.Title)
		}
		m.Send(likeDoneMsg{trackId: id, unlike: unlike, track: trackCopy})
	}()
	return nil
}

func (m *Model) applyLike(trackId string, unlike bool, track api.Track) tea.Cmd {
	likedPlaylist, index := m.playlists.GetFirst(playlist.LIKES)
	if likedPlaylist == nil {
		return nil
	}
	if unlike {
		delete(m.likedTracksMap, trackId)
		likedPlaylist.RemoveTrack(trackId)
	} else {
		m.likedTracksMap[trackId] = true
		t := track
		likedPlaylist.AddTrack(&t)
	}

	cmd := m.playlists.SetItem(index, likedPlaylist)
	if m.playlists.SelectedItem().Kind == playlist.LIKES {
		m.displayPlaylist(likedPlaylist)
	}

	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	return cmd
}
