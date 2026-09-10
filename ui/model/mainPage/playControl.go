package mainpage

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	_ "image/jpeg"
	_ "image/png"

	tea "github.com/charmbracelet/bubbletea"
	mp3 "github.com/dece2183/go-stream-mp3"
	"github.com/bogem/id3v2/v2"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/cache"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/stream"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/components/tracker"
	"github.com/wellbou/wellya/ui/components/tracklist"
	"github.com/wellbou/wellya/ui/helpers"
)

const (
	_TRACK_DOWNLOAD_TRIES     = 3
	_TRACK_FINISHED_THRESHOLD = 0.8
)

var errNoClient = errors.New("not logged in")

const (
	_READY_BYTES   = 256 * 1024
	_READY_TIMEOUT = 30 * time.Second
)

func waitBuffered(buf *stream.BufferedStream, need int64) bool {
	deadline := time.Now().Add(_READY_TIMEOUT)
	for {
		if buf.BufferedBytes() >= need || buf.IsBuffered() || buf.IsDone() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (m *Model) feedbackOnTrack(batch string) *api.RotorFeedback {
	currTrack := m.tracker.CurrentTrack()
	if currTrack == nil {
		return nil
	}
	var evType api.TrackEventType
	if m.tracker.Progress() > _TRACK_FINISHED_THRESHOLD {
		evType = api.EV_TRACK_FINISHED
	} else {
		evType = api.EV_TRACK_SKIPED
	}
	ev := api.NewTrackFeedbackEvent(evType, currTrack, m.tracker.Playtime().Seconds())
	if ev == nil {
		return nil
	}
	fb := api.NewFeedback(batch, ev)
	log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type+" track: "+currTrack.Title)
	return fb
}

func (m *Model) rotateTracks(currentPlaylist *playlist.Item) {
	if !currentPlaylist.Rotor || m.client == nil {
		return
	}

	suggestedTracks, err := m.client.RotorSessionTracks(currentPlaylist.SessionId, []*api.RotorFeedback{}, currentPlaylist.Tracks)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain more rotor tracks: %s", err)
		m.tracker.ShowError("next track obtain failure")
		m.Send(tracker.STOP)
		return
	}

	currentPlaylist.SessionBatch = suggestedTracks.BatchId
	if len(suggestedTracks.Sequence) == 0 {
		return
	}
	currentPlaylist.Tracks = append(currentPlaylist.Tracks, suggestedTracks.Sequence[0].Track)

	if m.activePlaylists().SelectedItem().IsSame(currentPlaylist) {
		tackItems := m.tracklist.Items()
		if len(tackItems) == 0 {
			return
		}
		lastTrack := tackItems[len(tackItems)-1]
		lastTrack.IsSuggestion = false
		m.tracklist.SetItem(len(tackItems)-1, lastTrack)
		m.tracklist.InsertItem(-1, tracklist.Item{
			Track:        &currentPlaylist.Tracks[len(currentPlaylist.Tracks)-1],
			Artists:      helpers.ArtistList(suggestedTracks.Sequence[0].Track.Artists),
			IsSuggestion: true,
		})
	}
}

func (m *Model) loadStationTracks(pl *playlist.Item) {
	if m.client == nil {
		return
	}

	session, err := m.client.RotorNewSession(pl.StationId)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to init station session [%s]: %s", pl.Name, err)
		m.tracker.ShowError("station session")
		return
	}

	pl.Rotor = true
	pl.SessionId = session.RadioSessionId
	pl.SessionBatch = session.BatchId
	if len(session.AcceptedSeeds) > 0 {
		pl.StationId = session.AcceptedSeeds[0]
	}

	if len(session.Sequence) > 0 {
		pl.Tracks = make([]api.Track, 0, len(session.Sequence))
		for _, seq := range session.Sequence {
			pl.Tracks = append(pl.Tracks, seq.Track)
		}
	}
}

func (m *Model) prevTrack() tea.Cmd {
	currentPlaylist := m.currentPlaylist()
	if currentPlaylist == nil {
		return nil
	}

	if currentPlaylist.Rotor && m.tracker.IsPlaying() {
		go m.client.RotorSessionFeedback(currentPlaylist.SessionId, m.feedbackOnTrack(currentPlaylist.SessionBatch))
	}

	if len(currentPlaylist.Tracks) == 0 || currentPlaylist.CurrentTrack <= 0 {
		m.Send(tracker.STOP)
		return nil
	}

	selectedPlaylist := m.activePlaylists().SelectedItem()
	shouldFollow := currentPlaylist.IsSame(selectedPlaylist) && m.tracklist.Index() == currentPlaylist.CurrentTrack

	m.indicateCurrentTrackPlaying(false)

	currentPlaylist.CurrentTrack--
	skipped := 0
	for currentPlaylist.CurrentTrack > 0 && !currentPlaylist.Tracks[currentPlaylist.CurrentTrack].Available {
		currentPlaylist.CurrentTrack--
		skipped++
	}
	if skipped > 0 {
		defer func() { m.Send(regionSkipMsg{count: skipped}) }()
	}

	m.currentPlaylists().SetItem(m.currentPlaylistIndex, currentPlaylist)
	track := &currentPlaylist.Tracks[currentPlaylist.CurrentTrack]
	if !track.Available {
		m.Send(tracker.STOP)
		return nil
	}

	m.playTrack(track)
	if shouldFollow {
		m.tracklist.Select(currentPlaylist.CurrentTrack)
		currentPlaylist.SelectedTrack = currentPlaylist.CurrentTrack
		m.currentPlaylists().SetItem(m.currentPlaylistIndex, currentPlaylist)
	}
	return nil
}

func (m *Model) nextTrack() tea.Cmd {
	currentPlaylist := m.currentPlaylist()
	if currentPlaylist == nil {
		return nil
	}

	if currentPlaylist.Rotor && m.tracker.IsPlaying() {
		go m.client.RotorSessionFeedback(currentPlaylist.SessionId, m.feedbackOnTrack(currentPlaylist.SessionBatch))
	}

	if len(currentPlaylist.Tracks) == 0 {
		m.Send(tracker.STOP)
		return nil
	}

	m.indicateCurrentTrackPlaying(false)

	if currentPlaylist.CurrentTrack+1 >= len(currentPlaylist.Tracks) {
		m.currentPlaylists().SetItem(m.currentPlaylistIndex, currentPlaylist)
		repeatMode := m.tracker.RepeatMode()
		switch repeatMode {
		case 1: // repeat playlist
			currentPlaylist.CurrentTrack = 0
			m.currentPlaylists().SetItem(m.currentPlaylistIndex, currentPlaylist)
			track := &currentPlaylist.Tracks[0]
			if track.Available {
				m.playSelectedPlaylist(0)
			} else {
				m.Send(tracker.STOP)
			}
		case 2: // repeat track
			currentPlaylist.CurrentTrack = len(currentPlaylist.Tracks) - 1
			m.playSelectedPlaylist(currentPlaylist.CurrentTrack)
		default: // no repeat
			m.Send(tracker.STOP)
		}
		return nil
	}

	selectedPlaylist := m.activePlaylists().SelectedItem()
	shouldFollow := currentPlaylist.IsSame(selectedPlaylist) && m.tracklist.Index() == currentPlaylist.CurrentTrack

	currentPlaylist.CurrentTrack++
	skipped := 0
	for currentPlaylist.CurrentTrack < len(currentPlaylist.Tracks)-1 && !currentPlaylist.Tracks[currentPlaylist.CurrentTrack].Available {
		currentPlaylist.CurrentTrack++
		skipped++
	}
	if skipped > 0 {
		defer func() { m.Send(regionSkipMsg{count: skipped}) }()
	}

	m.currentPlaylists().SetItem(m.currentPlaylistIndex, currentPlaylist)
	track := &currentPlaylist.Tracks[currentPlaylist.CurrentTrack]
	if !track.Available {
		m.Send(tracker.STOP)
		return nil
	}

	if currentPlaylist.CurrentTrack == len(currentPlaylist.Tracks)-1 {
		m.rotateTracks(currentPlaylist)
	}

	m.playTrack(track)
	if shouldFollow {
		m.tracklist.Select(currentPlaylist.CurrentTrack)
		currentPlaylist.SelectedTrack = currentPlaylist.CurrentTrack
		m.currentPlaylists().SetItem(m.currentPlaylistIndex, currentPlaylist)
	}
	return nil
}

type trackReadyMsg struct {
	generation int
	track      *api.Track
	buffer     *stream.BufferedStream
	decoder    *mp3.Decoder
	lyrics     []api.LyricPair
	bitrate    int
	fromCache  bool
}

type trackFailedMsg struct {
	generation int
	reason     string
}

type errorToastMsg struct {
	reason string
}

func (m *Model) playTrack(track *api.Track) {
	m.tracker.Stop()
	generation := int(m.playGeneration.Add(1))
	go m.loadTrack(m.client, track, generation)
}

func (m *Model) loadTrack(client *api.YaMusicClient, track *api.Track, generation int) {
	var (
		wg sync.WaitGroup

		coverType  string
		coverBytes []byte

	lyrics   []api.LyricPair
	lyricErr error
	bitrate  int

		trackReader    io.ReadCloser
		trackSize      int64
		trackFromCache bool
		downloadErr    error
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		coverPath := m.coverFilePath(track)
		stat, serr := os.Stat(coverPath)
		if serr == nil && stat.Size() > 0 {
			coverFile, ferr := os.Open(coverPath)
			if ferr != nil {
				log.Print(log.LVL_WARNING, "unable to open cover file [%s]: %s", coverPath, ferr)
				return
			}
			defer coverFile.Close()
			s, _ := coverFile.Stat()
			buf := make([]byte, s.Size())
			coverFile.Seek(0, io.SeekStart)
			coverFile.Read(buf)
			coverBytes = buf
			return
		}
		coverFile, ferr := os.OpenFile(coverPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
		if ferr != nil {
			log.Print(log.LVL_WARNING, "unable to open cover file [%s]: %s", coverPath, ferr)
			return
		}
		defer coverFile.Close()
		if serr != nil || stat.Size() == 0 {
			cType, derr := api.DownloadTrackCover(coverFile, track, 200)
			if derr != nil {
				log.Print(log.LVL_WARNING, "unable to download track [%s] cover: %s", track.Id, derr)
				return
			}
			coverType = cType
			coverFile.Sync()
			s, _ := coverFile.Stat()
			buf := make([]byte, s.Size())
			coverFile.Seek(0, io.SeekStart)
			coverFile.Read(buf)
			coverBytes = buf
		}
	}()

	lyricsFormat := ""
	if track.LyricsInfo.HasAvailableSyncLyrics {
		lyricsFormat = "LRC"
	} else if track.LyricsInfo.HasAvailableTextLyrics {
		lyricsFormat = "TEXT"
	}
	if lyricsFormat != "" && client != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lyr, lerr := client.TrackLyricsRequest(string(track.Id), lyricsFormat)
			if lerr != nil {
				log.Print(log.LVL_WARNING, "failed to obtain track [%s] lyrics: %s", track.Id, lerr)
				lyricErr = lerr
				return
			}
			lyrics = lyr
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		tr, ts, cerr := cache.Read(string(track.Id))
		if cerr == nil {
			trackReader = tr
			trackSize = ts
			trackFromCache = true
			return
		}
		if client == nil {
			downloadErr = errNoClient
			return
		}
		var lastErr error
		for i := 0; i < _TRACK_DOWNLOAD_TRIES; i++ {
			trackInfos, ierr := client.TrackDownloadInfo(string(track.Id))
			if ierr != nil {
				log.Print(log.LVL_ERROR, "failed to obtain track [%s] info: %s", track.Id, ierr)
				lastErr = ierr
				continue
			}
		bestTrackInfo := selectBestDownloadInfo(trackInfos, config.Current.AudioQuality)
		bitrate = int(bestTrackInfo.BbitrateInKbps)
		tr2, ts2, derr := client.DownloadTrack(bestTrackInfo)
			if derr != nil {
				log.Print(log.LVL_ERROR, "failed to download track [%s]: %s", track.Id, derr)
				lastErr = derr
				continue
			}
			trackReader = tr2
			trackSize = ts2
			lastErr = nil
			break
		}
		downloadErr = lastErr
	}()

	wg.Wait()

	if int64(generation) != m.playGeneration.Load() {
		if trackReader != nil {
			trackReader.Close()
		}
		return
	}

	if downloadErr != nil && trackReader == nil {
		m.Send(trackFailedMsg{generation: generation, reason: "track download"})
		return
	}

	if lyricErr != nil {
		m.Send(errorToastMsg{reason: "track lyrics"})
	}

	trackBuffer := stream.NewBufferedStream(trackReader, trackSize)
	metadataFile, err := os.OpenFile(m.metadataFilePath(string(track.Id)), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err == nil {
		writeTrackID3Tag(metadataFile, track, coverBytes, coverType)
		io.CopyN(metadataFile, trackBuffer, 32*1024)
		trackBuffer.Seek(0, io.SeekStart)
		metadataFile.Close()
	} else {
		log.Print(log.LVL_WARNING, "failed to create metadata file: %s", err)
	}

	decoder, err := mp3.NewDecoder(trackBuffer)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to create mp3 decoder: %s", err)
		trackBuffer.Close()
		if int64(generation) != m.playGeneration.Load() {
			return
		}
		m.Send(trackFailedMsg{generation: generation, reason: "track decode"})
		return
	}

	if seekMs := m.pendingResumePos; seekMs > 0 && track.DurationMs > 0 {
		posMs := int64(seekMs)
		if posMs < 0 || posMs >= int64(track.DurationMs)-1000 {
			posMs = 0
		}
		if posMs > 0 {
			byteOffset := int64(math.Round((float64(trackBuffer.Length()) / float64(track.DurationMs)) * float64(posMs)))
			byteOffset -= byteOffset % 4
			if _, serr := decoder.Seek(byteOffset, io.SeekStart); serr != nil {
				log.Print(log.LVL_WARNING, "resume seek failed: %s", serr)
			}
		}
	}

	if !waitBuffered(trackBuffer, _READY_BYTES) {
		trackBuffer.Close()
		if int64(generation) != m.playGeneration.Load() {
			return
		}
		m.Send(trackFailedMsg{generation: generation, reason: "track stalled"})
		return
	}

	m.Send(trackReadyMsg{
		generation: generation,
		track:      track,
		buffer:     trackBuffer,
		decoder:    decoder,
		lyrics:     lyrics,
		bitrate:    bitrate,
		fromCache:  trackFromCache,
	})
}

func (m *Model) playSelectedPlaylist(trackIndex int) {
	active := m.activePlaylists()
	selectedPlaylist := active.SelectedItem()
	if len(selectedPlaylist.Tracks) == 0 {
		if selectedPlaylist.Kind == playlist.STATION {
			m.loadStationTracks(selectedPlaylist)
			if len(selectedPlaylist.Tracks) == 0 {
				m.tracker.ShowError("station tracks")
				m.Send(tracker.STOP)
				return
			}
			selectedPlaylist.SelectedTrack = 0
			trackIndex = 0
			active.SetItem(active.Index(), selectedPlaylist)
			m.displayPlaylist(selectedPlaylist)
		} else {
			m.Send(tracker.STOP)
			return
		}
	}

	if trackIndex < 0 || trackIndex >= len(selectedPlaylist.Tracks) {
		m.Send(tracker.STOP)
		return
	}
	selectedPlaylist.SelectedTrack = trackIndex
	trackToPlay := &selectedPlaylist.Tracks[trackIndex]

	if currentPlaylist := m.currentPlaylist(); currentPlaylist != nil {
		if currentPlaylist.IsSame(selectedPlaylist) && selectedPlaylist.CurrentTrack == trackIndex && string(m.tracker.CurrentTrack().Id) == string(trackToPlay.Id) {
			if m.tracker.IsPlaying() {
				m.tracker.Pause()
				return
			} else {
				m.tracker.Play()
				return
			}
		}
		if currentPlaylist.Rotor {
			if m.tracker.IsPlaying() {
				go m.client.RotorSessionFeedback(currentPlaylist.SessionId, m.feedbackOnTrack(currentPlaylist.SessionBatch))
			}
			if !currentPlaylist.IsSame(selectedPlaylist) {
				ev := api.NewRadioFeedbackEvent(api.EV_RADIO_FINISHED)
				go m.client.RotorSessionFeedback(currentPlaylist.SessionId, api.NewFeedback(currentPlaylist.SessionBatch, ev))
				log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type)
			}
		}
	}

	m.indicateCurrentTrackPlaying(false)

	if selectedPlaylist.Rotor {
		if trackIndex == len(selectedPlaylist.Tracks)-1 {
			m.rotateTracks(selectedPlaylist)
		}
		if m.currentPlaylistIndex != active.Index() || m.currentIsRadio != m.isRadioTab {
			ev := api.NewRadioFeedbackEvent(api.EV_RADIO_STARTED)
			go m.client.RotorSessionFeedback(selectedPlaylist.SessionId, api.NewFeedback("", ev))
			log.Print(log.LVL_INFO, "feedback event sended: "+ev.Type)
		}
	}

	selectedPlaylist.CurrentTrack = trackIndex
	m.currentPlaylistIndex = active.Index()
	m.currentIsRadio = m.isRadioTab
	active.SetItem(m.currentPlaylistIndex, selectedPlaylist)
	m.playTrack(trackToPlay)

	trackCopy := *trackToPlay
	for i, t := range m.historyTracks {
		if string(t.Id) == string(trackCopy.Id) {
			m.historyTracks = append(m.historyTracks[:i], m.historyTracks[i+1:]...)
			break
		}
	}
	m.historyTracks = append([]api.Track{trackCopy}, m.historyTracks...)
	if len(m.historyTracks) > 100 {
		m.historyTracks = m.historyTracks[:100]
	}
}

func selectBestDownloadInfo(infos []api.TrackDownloadInfo, quality config.AudioQuality) api.TrackDownloadInfo {
	if len(infos) == 0 {
		return api.TrackDownloadInfo{}
	}

	sorted := make([]api.TrackDownloadInfo, len(infos))
	copy(sorted, infos)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].BbitrateInKbps > sorted[j].BbitrateInKbps
	})

	switch quality {
	case config.QUALITY_HIGH:
		for _, info := range sorted {
			if info.BbitrateInKbps <= 320 && info.BbitrateInKbps >= 256 {
				return info
			}
		}
		return sorted[0]
	case config.QUALITY_MEDIUM:
		for _, info := range sorted {
			if info.BbitrateInKbps <= 256 && info.BbitrateInKbps >= 192 {
				return info
			}
		}
		return sorted[len(sorted)-1]
	case config.QUALITY_LOW:
		return sorted[len(sorted)-1]
	default:
		return sorted[0]
	}
}

func id3v2Size(header []byte) int {
	if len(header) < 10 || string(header[:3]) != "ID3" {
		return 0
	}
	for i := 6; i < 10; i++ {
		if header[i]&0x80 != 0 {
			return 0
		}
	}
	total := 10 + (int(header[6])<<21 | int(header[7])<<14 | int(header[8])<<7 | int(header[9]))
	if header[5]&0x10 != 0 {
		total += 10
	}
	return total
}

func copyAudioWithoutID3(dst io.Writer, src io.Reader) (int64, error) {
	header := make([]byte, 10)
	n, err := io.ReadFull(src, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 0, err
	}
	header = header[:n]
	skip := id3v2Size(header)
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, src, int64(skip-10)); err != nil {
			return 0, err
		}
	} else if _, err := dst.Write(header); err != nil {
		return 0, err
	}
	return io.Copy(dst, src)
}

func writeTrackID3Tag(file *os.File, track *api.Track, coverBytes []byte, coverType string) {
	tag := id3v2.NewEmptyTag()
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)
	tag.SetTitle(track.Title)
	if len(track.Albums) != 0 {
		tag.SetAlbum(track.Albums[0].Title)
		tag.SetGenre(track.Albums[0].Genre)
		tag.SetYear(fmt.Sprint(track.Albums[0].Year))
	}
	tag.SetArtist(helpers.ArtistList(track.Artists))
	if len(coverBytes) > 0 {
		tag.AddAttachedPicture(id3v2.PictureFrame{
			MimeType:    coverType,
			PictureType: id3v2.PTFrontCover,
			Encoding:    id3v2.EncodingUTF16BE,
			Picture:     coverBytes,
		})
	}
	tag.AddFrame("TLEN", id3v2.TextFrame{
		Encoding: id3v2.EncodingUTF8,
		Text:     fmt.Sprint(track.DurationMs),
	})
	tag.WriteTo(file)
}
