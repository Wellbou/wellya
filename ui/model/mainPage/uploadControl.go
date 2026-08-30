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

func (m *Model) uploadControl(msg input.Control) tea.Cmd {
	if msg != input.APPLY {
		return nil
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
	var files []string
	if info.IsDir() {
		_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".mp3" || ext == ".flac" || ext == ".ogg" || ext == ".m4a" || ext == ".wav" || ext == ".m3u" {
				files = append(files, p)
			}
			return nil
		})
		if len(files) == 0 {
			return m.ShowToast("upload: no audio files")
		}
	} else {
		if strings.ToLower(filepath.Ext(path)) == ".m3u" {
			n, err := m.importM3U(path)
			if err != nil {
				return m.ShowToast("m3u import: " + err.Error())
			}
			if n == 0 {
				return m.ShowToast("m3u: nothing imported")
			}
			m.displayPlaylist(m.playlists.SelectedItem())
			return m.ShowToast(fmt.Sprintf("m3u imported %d track(s)", n))
		}
		files = []string{path}
	}
	count := 0
	m3uCount := 0
	for _, f := range files {
		if strings.ToLower(filepath.Ext(f)) == ".m3u" {
			n, err := m.importM3U(f)
			if err != nil {
				log.Print(log.LVL_WARNING, "m3u import failed %s: %s", f, err)
				continue
			}
			m3uCount += n
			continue
		}
		if err := m.importLocalFile(f); err != nil {
			log.Print(log.LVL_WARNING, "upload failed %s: %s", f, err)
			continue
		}
		count++
	}
	count += m3uCount
	if count == 0 {
		return m.ShowToast("upload: nothing imported")
	}
	m.displayPlaylist(m.playlists.SelectedItem())
	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(m.playlists.SelectedItem())
	}
	return m.ShowToast(fmt.Sprintf("uploaded %d track(s)", count))
}

func (m *Model) importLocalFile(filePath string) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return fmt.Errorf("empty file")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return err
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

	if m.cachedTracksMap[id] {
		return fmt.Errorf("already imported")
	}

	_, _ = f.Seek(0, io.SeekStart)
	cacheFile, err := cache.Write(id)
	if err != nil {
		return err
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
		DurationMs: durationMs,
		FileSize:   int(fi.Size()),
	}
	if year != "" {
		var y int
		_, _ = fmt.Sscan(year, &y)
		if y > 0 && len(track.Albums) > 0 {
			track.Albums[0].Year = y
		}
	}

	writeTrackID3Tag(cacheFile, track, nil, "")
	_, _ = f.Seek(0, io.SeekStart)
	_, err = io.Copy(cacheFile, f)
	if err != nil {
		_ = cache.Remove(id)
		return err
	}

	m.cachedTracksMap[id] = true
	localPl, idx := m.playlists.GetFirst(playlist.LOCAL)
	if localPl == nil {
		return fmt.Errorf("no local playlist")
	}
	localPl.AddTrack(track)
	m.playlists.SetItem(idx, localPl)
	if m.playlists.SelectedItem().Kind == playlist.LOCAL {
		m.displayPlaylist(localPl)
	}
	return nil
}

func (m *Model) importM3U(m3uPath string) (int, error) {
	data, err := os.ReadFile(m3uPath)
	if err != nil {
		return 0, err
	}
	baseDir := filepath.Dir(m3uPath)
	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
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
		if ext := strings.ToLower(filepath.Ext(line)); ext == ".mp3" || ext == ".flac" || ext == ".ogg" || ext == ".m4a" || ext == ".wav" {
			if err := m.importLocalFile(line); err == nil {
				count++
			}
		}
	}
	return count, nil
}
