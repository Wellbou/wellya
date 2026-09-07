package mainpage

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/cache"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/style"
)

func fmtSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (m *Model) cacheManagerView() string {
	lines := []string{
		style.DialogTitleStyle.Render(" Cached tracks "),
		"",
	}

	files, err := cache.Files()
	if err != nil {
		lines = append(lines, style.ErrorTextStyle.Render("cache error: "+err.Error()))
	} else if len(files) == 0 {
		lines = append(lines, style.TrackVersionStyle.Render(" cache is empty "))
	} else {
		var total int64
		shown := files
		extra := 0
		if len(shown) > 15 {
			extra = len(shown) - 15
			shown = shown[:15]
		}
		for _, f := range files {
			total += f.Size
		}
		for _, f := range shown {
			name := f.Name
			if len(name) > 44 {
				name = name[:43] + "…"
			}
			lines = append(lines, fmt.Sprintf(" %-44s %8s", name, style.TrackVersionStyle.Render(fmtSize(f.Size))))
		}
		if extra > 0 {
			lines = append(lines, style.TrackVersionStyle.Render(fmt.Sprintf(" …and %d more", extra)))
		}
		lines = append(lines, "", style.AccentTextStyle.Render(fmt.Sprintf(" Total: %d files, %s", len(files), fmtSize(total))))
	}

	lines = append(lines, "", style.TrackVersionStyle.Render(" d — delete all · esc — close "))
	return style.DialogBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m *Model) clearCache() tea.Cmd {
	if err := cache.Clear(); err != nil {
		log.Print(log.LVL_ERROR, "failed to clear cache: %s", err)
		return m.ShowToast("cache clear failed")
	}
	m.cachedTracksMap = make(map[string]bool)
	if localPl, idx := m.playlists.GetFirst(playlist.LOCAL); localPl != nil {
		localPl.Tracks = nil
		localPl.SelectedTrack = 0
		localPl.CurrentTrack = -1
		m.playlists.SetItem(idx, localPl)
		if m.playlists.SelectedItem().Kind == playlist.LOCAL {
			m.displayPlaylist(localPl)
		}
	}
	return m.ShowToast("cache cleared")
}
