package cache

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/wellbou/wellya/api"
)

func ListTracks() ([]api.Track, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	tracks := make([]api.Track, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if entry.IsDir() || ext != ".mp3" {
			continue
		}

		tag, err := id3v2.Open(filepath.Join(dir, name), id3v2.Options{Parse: true})
		if err != nil {
			continue
		}

		artistNames := strings.Split(tag.Artist(), ",")
		artists := make([]api.Artist, len(artistNames))
		for i := range artistNames {
			artists[i].Name = strings.TrimSpace(artistNames[i])
		}

		title, albumTitle, genre, yearStr := tag.Title(), tag.Album(), tag.Genre(), tag.Year()
		durationMs, _ := strconv.Atoi(tag.GetTextFrame("TLEN").Text)
		tag.Close()

		stat, statErr := entry.Info()
		if statErr != nil || stat == nil {
			continue
		}
		year, _ := strconv.Atoi(yearStr)
		if durationMs <= 0 && stat.Size() > 0 {
			durationMs = int(stat.Size() / 16)
		}
		if title == "" {
			title = strings.TrimSuffix(name, ext)
		}

		tracks = append(tracks, api.Track{
			Id:         api.FlexString(name[:len(name)-len(ext)]),
			Title:      title,
			Available:  true,
			FileSize:   api.FlexInt(stat.Size()),
			DurationMs: api.FlexInt(durationMs),
			Artists:    artists,
			Albums: []api.Album{
				{
					Title: albumTitle,
					Genre: genre,
					Year:  api.FlexInt(year),
				},
			},
		})
	}

	return tracks, nil
}
