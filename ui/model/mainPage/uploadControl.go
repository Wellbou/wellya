package mainpage

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/cache"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/input"
	"github.com/wellbou/wellya/ui/components/playlist"
)

type uploadDoneMsg struct {
	tracks []*api.Track
	err    string
}

func (m *Model) uploadControl(msg input.Control) tea.Cmd {
	if msg != input.APPLY {
		return nil
	}
	if m.isUploading {
		return m.ShowToast("upload already in progress")
	}
	path := strings.TrimSpace(m.inputDialog.Value())
	if path == "" {
		return m.ShowToast("upload: empty path")
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	} else if path == "~" {
		home, _ := os.UserHomeDir()
		path = home
	}
	path = os.ExpandEnv(path)
	info, err := os.Stat(path)
	if err != nil {
		return m.ShowToast("upload: not found")
	}
	if !info.IsDir() && strings.ToLower(filepath.Ext(path)) != ".m3u" && !isAudioFile(path) {
		return m.ShowToast("upload: not an audio file")
	}
	m.isUploading = true
	go m.importPaths(path, info.IsDir())
	return m.ShowToast("uploading...")
}

func (m *Model) importPaths(path string, isDir bool) {
	files := collectAudioFiles(path, isDir)
	if len(files) == 0 {
		m.Send(uploadDoneMsg{err: "upload: no audio files"})
		return
	}
	var tracks []*api.Track
	seen := make(map[string]bool)
	for _, f := range files {
		track, err := buildLocalTrack(f)
		if err != nil {
			log.Print(log.LVL_WARNING, "upload failed %s: %s", f, err)
			continue
		}
		id := string(track.Id)
		if seen[id] {
			continue
		}
		seen[id] = true
		tracks = append(tracks, track)
	}
	m.Send(uploadDoneMsg{tracks: tracks})
}

func (m *Model) applyUploadedTracks(tracks []*api.Track) tea.Cmd {
	count := 0
	localPl, idx := m.playlists.GetFirst(playlist.LOCAL)
	if localPl == nil {
		return m.ShowToast("upload: no local playlist")
	}
	for _, track := range tracks {
		id := string(track.Id)
		if m.cachedTracksMap[id] {
			continue
		}
		m.cachedTracksMap[id] = true
		localPl.AddTrack(track)
		count++
	}
	m.playlists.SetItem(idx, localPl)
	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(localPl)
	} else {
		m.displayPlaylist(m.playlists.SelectedItem())
	}
	if count == 0 {
		return m.ShowToast("upload: nothing imported")
	}
	return m.ShowToast(fmt.Sprintf("uploaded %d track(s)", count))
}

func isAudioFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".flac", ".ogg", ".m4a", ".wav":
		return true
	}
	return false
}

func collectAudioFiles(path string, isDir bool) []string {
	if !isDir {
		if strings.ToLower(filepath.Ext(path)) == ".m3u" {
			return collectM3UFiles(path)
		}
		return []string{path}
	}
	var files []string
	_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(p)) == ".m3u" {
			files = append(files, collectM3UFiles(p)...)
			return nil
		}
		if isAudioFile(p) {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func collectM3UFiles(m3uPath string) []string {
	data, err := os.ReadFile(m3uPath)
	if err != nil {
		log.Print(log.LVL_WARNING, "m3u read failed %s: %s", m3uPath, err)
		return nil
	}
	baseDir := filepath.Dir(m3uPath)
	var files []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "https://") || strings.HasPrefix(line, "http://") {
			continue
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(baseDir, line)
		}
		if strings.HasPrefix(line, "~/") {
			home, _ := os.UserHomeDir()
			line = filepath.Join(home, line[2:])
		}
		line = os.ExpandEnv(line)
		if _, err := os.Stat(line); err != nil {
			continue
		}
		if isAudioFile(line) {
			files = append(files, line)
		}
	}
	return files
}

func buildLocalTrack(filePath string) (*api.Track, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if fi.Size() == 0 {
		return nil, fmt.Errorf("empty file")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	artist := "Local"
	album := "Local Files"
	genre := ""
	year := ""
	var durationMs int

	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err == nil {
		if tag != nil {
			defer tag.Close()
			if v := tag.Title(); v != "" {
				title = v
			}
			if v := tag.Artist(); v != "" {
				artist = v
			}
			if v := tag.Album(); v != "" {
				album = v
			}
			if v := tag.Genre(); v != "" {
				genre = v
			}
			if v := tag.Year(); v != "" {
				year = v
			}
			if frames := tag.GetFrames(tag.CommonID("Length")); len(frames) > 0 {
				if tf, ok := frames[0].(id3v2.TextFrame); ok {
					var ms int
					_, _ = fmt.Sscan(tf.Text, &ms)
					if ms > 0 {
						durationMs = ms
					}
				}
			}
		}
	}
	if durationMs == 0 {
		durationMs = 180000
		if fi.Size() > 0 {
			durationMs = int(fi.Size() / 16000 * 1000)
			if durationMs < 1000 {
				durationMs = 180000
			}
		}
	}

	h := sha1.New()
	_, _ = io.WriteString(h, filePath)
	_, _ = io.WriteString(h, fi.ModTime().String())
	_, _ = io.WriteString(h, fmt.Sprint(fi.Size()))
	sum := h.Sum(nil)
	id := fmt.Sprintf("local_%x", sum[:8])

	_, _ = f.Seek(0, io.SeekStart)
	cacheFile, err := cache.Write(id)
	if err != nil {
		return nil, err
	}
	defer cacheFile.Close()

	track := &api.Track{
		Id: api.FlexString(id),
		RealId: api.FlexString(id),
		Title:     title,
		Available: true,
		Artists: []api.Artist{
			{Name: artist},
		},
		Albums: []api.Album{
			{Title: album, Genre: genre},
		},
		DurationMs: api.FlexInt(durationMs),
		FileSize:   api.FlexInt(fi.Size()),
	}
	if year != "" {
		var y int
		_, _ = fmt.Sscan(year, &y)
		if y > 0 && len(track.Albums) > 0 {
			track.Albums[0].Year = api.FlexInt(y)
		}
	}

	writeTrackID3Tag(cacheFile, track, nil, "")
	_, _ = f.Seek(0, io.SeekStart)
	_, err = io.Copy(cacheFile, f)
	if err != nil {
		_ = cache.Remove(id)
		return nil, err
	}

	return track, nil
}
