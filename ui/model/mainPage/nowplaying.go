package mainpage

import (
	"fmt"
	"strings"
	"time"

	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/ui/helpers"
	"github.com/wellbou/wellya/ui/style"
)

func fmtDur(d time.Duration) string {
	total := int(d.Seconds())
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func (m *Model) nowPlayingView() string {
	track := m.tracker.CurrentTrack()
	title := "—"
	artist := ""
	album := ""
	var total time.Duration
	if track != nil && track.Id != "" {
		title = track.Title
		artist = helpers.ArtistList(track.Artists)
		if len(track.Albums) > 0 {
			album = track.Albums[0].Title
			if track.Albums[0].Year > 0 {
				album = fmt.Sprintf("%s (%d)", album, track.Albums[0].Year)
			}
		}
		total = time.Duration(track.DurationMs) * time.Millisecond
	}
	pos := m.tracker.Position()
	if pos < 0 {
		pos = 0
	}
	if total > 0 && pos > total {
		pos = total
	}

	barW := m.width - 14
	if barW < 10 {
		barW = 10
	}
	frac := 0.0
	if total > 0 {
		frac = float64(pos) / float64(total)
	}
	filled := int(frac * float64(barW))
	bar := style.AccentTextStyle.Render(strings.Repeat("█", filled)) +
		style.TrackVersionStyle.Render(strings.Repeat("─", barW-filled))

	var status []string
	if m.tracker.IsPlaying() {
		status = append(status, "▶ playing")
	} else {
		status = append(status, "❚❚ paused")
	}
	if track != nil && track.Id != "" && m.likedTracksMap[string(track.Id)] {
		status = append(status, "💛 liked")
	}
	if b := m.tracker.Bitrate(); b > 0 {
		status = append(status, fmt.Sprintf("%dk", b))
	}
	status = append(status, config.Current.AudioQuality.Label())
	switch m.tracker.RepeatMode() {
	case 1:
		status = append(status, "repeat all")
	case 2:
		status = append(status, "repeat one")
	}
	if label := m.tracker.SleepTimerLabel(); label != "" {
		status = append(status, strings.TrimSpace(label))
	}
	if m.tracker.IsMuted() {
		status = append(status, "muted")
	}

	lines := []string{
		style.DialogTitleStyle.Render(" Now playing "),
		"",
		style.TrackTitleStyle.Render(title),
		style.TrackArtistStyle.Render(artist),
	}
	if album != "" {
		lines = append(lines, style.TrackVersionStyle.Render(album))
	}
	lines = append(lines,
		"",
		fmt.Sprintf("%s  %s", bar, style.TrackVersionStyle.Render(fmtDur(pos)+" / "+fmtDur(total))),
		"",
		style.TrackVersionStyle.Render(strings.Join(status, " · ")),
	)

	if pl := m.currentPlaylist(); pl != nil && pl.CurrentTrack >= 0 {
		up := []string{}
		for i := pl.CurrentTrack + 1; i < len(pl.Tracks) && len(up) < 5; i++ {
			t := pl.Tracks[i]
			name := t.Title
			if a := helpers.ArtistList(t.Artists); a != "" {
				name += " — " + a
			}
			up = append(up, style.TrackVersionStyle.Render(fmt.Sprintf("  %d. %s", i+1, name)))
		}
		if len(up) > 0 {
			lines = append(lines, "", style.AccentTextStyle.Render(" Up next:"))
			lines = append(lines, up...)
		}
	}

	lines = append(lines, "", style.TrackVersionStyle.Render(" esc / f — back "))
	return style.DialogBoxStyle.Render(strings.Join(lines, "\n"))
}
