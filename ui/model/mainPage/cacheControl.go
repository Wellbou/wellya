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
	if m.tracker.IsStoped() || m.cachedTracksMap[currentTrack.Id] {
		return nil
	}

	metadataFile, err := os.OpenFile(m.metadataFilePath(), os.O_RDONLY, 0755)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to open cache file: %s", err)
		m.tracker.ShowError("cache open")
		return nil
	}

	defer metadataFile.Close()

	cacheFile, err := cache.Write(currentTrack.Id)
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
	trackBuffer.Seek(0, 0)
	trackBuffer.WriteTo(cacheFile)

	m.cachedTracksMap[currentTrack.Id] = true
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

	err := cache.Remove(track.Id)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to remove cached file: %s", err)
		m.tracker.ShowError("cache remove")
		return nil
	}

	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	cachePlaylist.RemoveTrack(track.Id)

	delete(m.cachedTracksMap, track.Id)
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
		m.tracker.ShowError("no liked tracks to cache")
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	var mu sync.Mutex
	var cachedTracks []api.Track

	tracksCopy := make([]api.Track, len(likedPlaylist.Tracks))
	copy(tracksCopy, likedPlaylist.Tracks)

	for _, track := range tracksCopy {
		if m.cachedTracksMap[track.Id] {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(t api.Track) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := m.downloadAndCacheTrack(&t); err != nil {
				log.Print(log.LVL_ERROR, "failed to cache track [%s]: %s", t.Id, err)
				return
			}

			mu.Lock()
			cachedTracks = append(cachedTracks, t)
			mu.Unlock()
		}(track)
	}

	wg.Wait()

	for _, t := range cachedTracks {
		m.cachedTracksMap[t.Id] = true
	}
	cachePlaylist, index := m.playlists.GetFirst(playlist.LOCAL)
	if cachePlaylist != nil {
		for i := range cachedTracks {
			cachePlaylist.AddTrack(&cachedTracks[i])
		}
		m.playlists.SetItem(index, cachePlaylist)
	}

	log.Print(log.LVL_INFO, "batch cache complete: %d tracks cached", len(cachedTracks))
}

func (m *Model) downloadAndCacheTrack(track *api.Track) error {
	trackInfos, err := m.client.TrackDownloadInfo(track.Id)
	if err != nil {
		return err
	}

	bestTrackInfo := selectBestDownloadInfo(trackInfos, config.Current.AudioQuality)

	trackReader, _, err := m.client.DownloadTrack(bestTrackInfo)
	if err != nil {
		return err
	}
	defer trackReader.Close()

	cacheFile, err := cache.Write(track.Id)
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

func (m *Model) downloadCurrentTrack() tea.Cmd {
	currentTrack := m.tracker.CurrentTrack()
	if m.tracker.IsStoped() {
		return nil
	}

	downloadDir := config.Current.DownloadDir
	if downloadDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to get home dir: %s", err)
			m.tracker.ShowError("download: home dir")
			return nil
		}
		downloadDir = filepath.Join(home, "Music", "wellya")
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

	trackInfos, err := m.client.TrackDownloadInfo(currentTrack.Id)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to get download info: %s", err)
		m.tracker.ShowError("download: info")
		return nil
	}

	bestTrackInfo := selectBestDownloadInfo(trackInfos, config.Current.AudioQuality)

	trackReader, _, err := m.client.DownloadTrack(bestTrackInfo)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to download track: %s", err)
		m.tracker.ShowError("download: stream")
		return nil
	}
	defer trackReader.Close()

	file, err := os.Create(filePath)
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to create file: %s", err)
		m.tracker.ShowError("download: create file")
		return nil
	}
	defer file.Close()

	writeTrackID3Tag(file, currentTrack, nil, "")
	io.Copy(file, trackReader)

	log.Print(log.LVL_INFO, "track downloaded: %s", filePath)
	return m.ShowToast("Saved: " + filename)
}
