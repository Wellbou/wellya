package mainpage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/bogem/id3v2/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/cache"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/helpers"
)

func (m *Model) cacheCurrentTrack() tea.Cmd {
	currentTrack := m.tracker.CurrentTrack()
	if m.tracker.IsStoped() || m.cachedTracksMap[string(currentTrack.Id)] {
		return nil
	}

	metadataFile, err := os.OpenFile(m.metadataFilePath(), os.O_RDONLY, 0755)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to open cache file: %s", err)
		m.tracker.ShowError("cache open")
		return nil
	}

	defer metadataFile.Close()

	cacheFile, err := cache.Write(string(currentTrack.Id))
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to write cache file: %s", err)
		m.tracker.ShowError("cache write")
		return nil
	}

	defer cacheFile.Close()

	tag := id3v2.NewEmptyTag()
	tag.Reset(metadataFile, id3v2.Options{Parse: true})
	tag.WriteTo(cacheFile)

	trackBuffer := m.tracker.TrackBuffer()
	if trackBuffer == nil || trackBuffer.BufferingProgress() < 1 {
		cacheFile.Close()
		_ = os.Remove(cacheFile.Name())
		return nil
	}
	trackBuffer.WriteTo(cacheFile)

	m.cachedTracksMap[string(currentTrack.Id)] = true
	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	cachePlaylist.AddTrack(currentTrack)
	cmd := m.playlists.SetItem(index, cachePlaylist)

	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(cachePlaylist)
	}

	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	return cmd
}

func (m *Model) removeCache(track *api.Track) tea.Cmd {
	if m.tracker.CurrentTrack().Id == track.Id && len(m.tracker.CurrentTrack().RealId) == 0 {
		m.tracker.ShowError("can't remove currently playing track")
		return nil
	}

	err := cache.Remove(string(track.Id))
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to remove cached file: %s", err)
		m.tracker.ShowError("cache remove")
		return nil
	}

	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	cachePlaylist.RemoveTrack(string(track.Id))

	delete(m.cachedTracksMap, string(track.Id))
	cmd := m.playlists.SetItem(index, cachePlaylist)

	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(cachePlaylist)
	}

	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
	return cmd
}

func (m *Model) cacheAllLikedTracks() {
	likedPlaylist, _ := m.playlists.GetFirst(playlist.LIKES)
	if likedPlaylist == nil || len(likedPlaylist.Tracks) == 0 {
		m.Send(trackFailedMsg{generation: m.playGeneration, reason: "no liked tracks to cache"})
		return
	}

	tracksCopy := make([]api.Track, len(likedPlaylist.Tracks))
	copy(tracksCopy, likedPlaylist.Tracks)
	skip := make(map[string]bool, len(m.cachedTracksMap))
	for id := range m.cachedTracksMap {
		skip[id] = true
	}
	go m.cacheTracksBatch(tracksCopy, skip, m.client)
}

func (m *Model) cacheTracksBatch(tracksCopy []api.Track, skip map[string]bool, client *api.YaMusicClient) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	var mu sync.Mutex
	var cachedTracks []api.Track

	for _, track := range tracksCopy {
		if skip[string(track.Id)] {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(t api.Track) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := downloadAndCacheTrack(client, &t); err != nil {
				log.Print(log.LVL_ERROR, "failed to cache track [%s]: %s", t.Id, err)
				return
			}

			mu.Lock()
			cachedTracks = append(cachedTracks, t)
			mu.Unlock()
		}(track)
	}

	wg.Wait()

	log.Print(log.LVL_INFO, "batch cache complete: %d tracks cached", len(cachedTracks))
	m.Send(cacheAllDoneMsg{tracks: cachedTracks})
}

type cacheAllDoneMsg struct {
	tracks []api.Track
}

func (m *Model) applyCachedTracks(tracks []api.Track) {
	for _, t := range tracks {
		m.cachedTracksMap[string(t.Id)] = true
	}
	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	if cachePlaylist != nil {
		for i := range tracks {
			cachePlaylist.AddTrack(&tracks[i])
		}
		m.playlists.SetItem(index, cachePlaylist)
	}
	m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
}

func downloadAndCacheTrack(client *api.YaMusicClient, track *api.Track) error {
	trackInfos, err := client.TrackDownloadInfo(string(track.Id))
	if err != nil {
		return err
	}

	bestTrackInfo := selectBestDownloadInfo(trackInfos, config.Current.AudioQuality)

	trackReader, _, err := client.DownloadTrack(bestTrackInfo)
	if err != nil {
		return err
	}
	defer trackReader.Close()

	cacheFile, err := cache.Write(string(track.Id))
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	writeTrackID3Tag(cacheFile, track, nil, "")
	io.Copy(cacheFile, trackReader)

	return nil
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else if r != '/' && r != '\\' {
			b.WriteRune('_')
		}
	}
	result := b.String()
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
}

type downloadDoneMsg struct {
	filename string
	err      string
}

func (m *Model) downloadCurrentTrack() tea.Cmd {
	currentTrack := m.tracker.CurrentTrack()
	if m.tracker.IsStoped() {
		return nil
	}

	downloadDir := config.Current.DownloadDir
	if downloadDir == "" {
		downloadDir = config.MusicDir()
		if downloadDir == "" {
			log.Print(log.LVL_ERROR, "failed to get home dir")
			m.tracker.ShowError("download: home dir")
			return nil
		}
	}

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Print(log.LVL_ERROR, "failed to create download dir: %s", err)
		m.tracker.ShowError("download: mkdir")
		return nil
	}

	filename := sanitizeFilename(fmt.Sprintf("%s - %s", helpers.ArtistList(currentTrack.Artists), currentTrack.Title)) + ".mp3"
	filePath := filepath.Join(downloadDir, filename)

	if _, err := os.Stat(filePath); err == nil {
		m.tracker.ShowError("already downloaded")
		return nil
	}

	trackCopy := *currentTrack
	go m.downloadTrackFile(m.client, &trackCopy, filePath, filename)
	return m.ShowToast("downloading: " + filename)
}

func (m *Model) downloadTrackFile(client *api.YaMusicClient, track *api.Track, filePath, filename string) {
	fail := func(reason, logMsg string) {
		log.Print(log.LVL_ERROR, logMsg)
		m.Send(downloadDoneMsg{err: reason})
	}

	trackInfos, err := client.TrackDownloadInfo(string(track.Id))
	if err != nil {
		fail("download: info", fmt.Sprintf("failed to get download info: %s", err))
		return
	}

	bestTrackInfo := selectBestDownloadInfo(trackInfos, config.Current.AudioQuality)

	trackReader, _, err := client.DownloadTrack(bestTrackInfo)
	if err != nil {
		fail("download: stream", fmt.Sprintf("failed to download track: %s", err))
		return
	}
	defer trackReader.Close()

	file, err := os.Create(filePath)
	if err != nil {
		fail("download: create file", fmt.Sprintf("failed to create file: %s", err))
		return
	}
	defer file.Close()

	writeTrackID3Tag(file, track, nil, "")
	if _, err := io.Copy(file, trackReader); err != nil {
		_ = os.Remove(filePath)
		fail("download: stream", fmt.Sprintf("failed to write track: %s", err))
		return
	}

	log.Print(log.LVL_INFO, "track downloaded: %s", filePath)
	m.Send(downloadDoneMsg{filename: filename})
}
