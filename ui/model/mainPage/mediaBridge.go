package mainpage

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/components/tracker"
)

// All media-key (MPRIS/SMTC) traffic goes through the main thread as
// messages. Touching tracker/playlist models directly from the media
// handler goroutine races with the UI thread.
type mediaPlayMsg struct{}
type mediaPauseMsg struct{}
type mediaPlayPauseMsg struct{}
type mediaSeekMsg struct{ offset time.Duration }
type mediaSetPosMsg struct{ pos time.Duration }
type mediaSetVolumeMsg struct{ vol float64 }
type mediaSetShuffleMsg struct{ on bool }

type mediaStatusQuery struct{ reply chan handler.PlaybackState }
type mediaShuffleQuery struct{ reply chan bool }
type mediaMetadataQuery struct{ reply chan handler.TrackMetadata }
type mediaVolumeQuery struct{ reply chan float64 }
type mediaPositionQuery struct{ reply chan time.Duration }

func (m *Model) mediaHandle() {
	for msg := range m.mediaHandler.Message() {
		switch msg.Type {
		case handler.MSG_NEXT:
			m.Send(tracker.NEXT)
		case handler.MSG_PREVIOUS:
			m.Send(tracker.PREV)
		case handler.MSG_PLAY:
			m.Send(mediaPlayMsg{})
		case handler.MSG_PAUSE:
			m.Send(mediaPauseMsg{})
		case handler.MSG_PLAYPAUSE:
			m.Send(mediaPlayPauseMsg{})
		case handler.MSG_STOP:
			m.Send(tracker.STOP)
		case handler.MSG_SEEK:
			if offset, ok := msg.Arg.(time.Duration); ok {
				m.Send(mediaSeekMsg{offset: offset})
			}
		case handler.MSG_SETPOS:
			if pos, ok := msg.Arg.(time.Duration); ok {
				m.Send(mediaSetPosMsg{pos: pos})
			}
		case handler.MSG_SET_SHUFFLE:
			if val, ok := msg.Arg.(bool); ok {
				m.Send(mediaSetShuffleMsg{on: val})
			}
		case handler.MSG_SET_VOLUME:
			if vol, ok := msg.Arg.(float64); ok {
				m.Send(mediaSetVolumeMsg{vol: vol})
			}
		case handler.MSG_GET_PLAYBACKSTATUS:
			reply := make(chan handler.PlaybackState, 1)
			m.Send(mediaStatusQuery{reply: reply})
			m.mediaHandler.SendAnswer(<-reply)
		case handler.MSG_GET_SHUFFLE:
			reply := make(chan bool, 1)
			m.Send(mediaShuffleQuery{reply: reply})
			m.mediaHandler.SendAnswer(<-reply)
		case handler.MSG_GET_METADATA:
			reply := make(chan handler.TrackMetadata, 1)
			m.Send(mediaMetadataQuery{reply: reply})
			m.mediaHandler.SendAnswer(<-reply)
		case handler.MSG_GET_VOLUME:
			reply := make(chan float64, 1)
			m.Send(mediaVolumeQuery{reply: reply})
			m.mediaHandler.SendAnswer(<-reply)
		case handler.MSG_GET_POSITION:
			reply := make(chan time.Duration, 1)
			m.Send(mediaPositionQuery{reply: reply})
			m.mediaHandler.SendAnswer(<-reply)
		}
	}
}

func (m *Model) answerMediaQueries(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case mediaPlayMsg:
		m.tracker.Play()
		return true
	case mediaPauseMsg:
		m.tracker.Pause()
		return true
	case mediaPlayPauseMsg:
		if m.tracker.IsPlaying() {
			m.tracker.Pause()
		} else {
			m.tracker.Play()
		}
		return true
	case mediaSeekMsg:
		m.tracker.Rewind(msg.offset)
		return true
	case mediaSetPosMsg:
		m.tracker.SetPos(msg.pos)
		return true
	case mediaSetVolumeMsg:
		m.tracker.SetVolume(msg.vol)
		return true
	case mediaSetShuffleMsg:
		if !msg.on {
			return true
		}
		if m.currentPlaylistIndex < 0 || m.currentPlaylistIndex >= len(m.currentPlaylists().Items()) {
			return true
		}
		currentPlaylist := m.currentPlaylists().Items()[m.currentPlaylistIndex]
		if len(currentPlaylist.Tracks) == 0 {
			return true
		}
		if currentPlaylist.Kind >= playlist.LIKES {
			cmd := m.shufflePlaylist(currentPlaylist)
			m.Send(func() tea.Cmd {
				return cmd
			})
		}
		return true
	case mediaStatusQuery:
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
		msg.reply <- state
		return true
	case mediaShuffleQuery:
		msg.reply <- false
		return true
	case mediaMetadataQuery:
		if m.tracker.IsStoped() {
			msg.reply <- handler.TrackMetadata{}
			return true
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

		msg.reply <- handler.TrackMetadata{
			TrackId:      string(track.Id),
			Length:       time.Duration(track.DurationMs) * time.Millisecond,
			CoverUrl:     m.coverFilePath(track),
			AlbumName:    albumName,
			AlbumArtists: albumArtists,
			Artists:      artists,
			Genre:        genre,
			Title:        track.Title,
			Url:          api.ShareTrackLink(track),
		}
		return true
	case mediaVolumeQuery:
		msg.reply <- m.tracker.Volume()
		return true
	case mediaPositionQuery:
		msg.reply <- m.tracker.Position()
		return true
	}
	return false
}
