package mainpage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
)

const (
	maxSessionBytes  = 1 << 20
	maxSessionTracks = 2000
)

type sessionState struct {
	PlaylistKind uint64   `json:"playlist_kind"`
	IsRadio      bool     `json:"is_radio"`
	PlaylistName string   `json:"playlist_name"`
	TrackIDs     []string    `json:"track_ids"`
	Current      int         `json:"current"`
	PositionMs   int64       `json:"position_ms"`
	History      []api.Track `json:"history,omitempty"`
}

func sessionPath() string {
	return filepath.Join(filepath.Dir(config.Path()), "session.json")
}

func (m *Model) saveSession() {
	state := sessionState{History: m.historyTracks}

	cur := m.tracker.CurrentTrack()
	if cur != nil && cur.Id != "" {
		if pl := m.currentPlaylist(); pl != nil && len(pl.Tracks) > 0 && !pl.Rotor {
			ids := make([]string, 0, len(pl.Tracks))
			for i := range pl.Tracks {
				id := string(pl.Tracks[i].Id)
				if id == "" || len(id) > 128 {
					continue
				}
				ids = append(ids, id)
				if len(ids) >= maxSessionTracks {
					break
				}
			}
			curIdx := pl.CurrentTrack
			if curIdx < 0 || curIdx >= len(pl.Tracks) {
				curIdx = 0
			}
			state.PlaylistKind = pl.Kind
			state.IsRadio = m.currentIsRadio
			state.PlaylistName = pl.Name
			state.TrackIDs = ids
			state.Current = curIdx
			state.PositionMs = m.tracker.Position().Milliseconds()
		}
	}

	if len(state.TrackIDs) == 0 && len(state.History) == 0 {
		_ = os.Remove(sessionPath())
		return
	}

	if len(state.History) > 100 {
		state.History = state.History[:100]
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(sessionPath()), "session-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, sessionPath()); err != nil {
		_ = os.Remove(tmpName)
		log.Print(log.LVL_WARNING, "failed to save session: %s", err)
	}
}

func validSessionID(id string) bool {
	return id != "" && len(id) <= 128
}

func (m *Model) restoreSession() {
	st, err := os.Stat(sessionPath())
	if err != nil || st.Size() > maxSessionBytes {
		return
	}
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return
	}
	var s sessionState
	if err := json.Unmarshal(data, &s); err != nil {
		log.Print(log.LVL_WARNING, "ignoring corrupt session file: %s", err)
		_ = os.Remove(sessionPath())
		return
	}
	if len(s.TrackIDs) > maxSessionTracks {
		return
	}
	for _, id := range s.TrackIDs {
		if !validSessionID(id) {
			return
		}
	}
	if s.Current < 0 || s.PositionMs < 0 {
		return
	}
	if len(s.History) > 0 {
		m.historyTracks = append([]api.Track(nil), s.History...)
		if len(m.historyTracks) > 100 {
			m.historyTracks = m.historyTracks[:100]
		}
	}
	if len(s.TrackIDs) == 0 {
		return
	}

	lists := m.playlists
	if s.IsRadio {
		lists = m.radioPlaylists
	}
	items := lists.Items()
	at := -1
	for i := range items {
		if items[i].Kind == s.PlaylistKind && items[i].Active {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	pl := items[at]
	if pl.Rotor || len(pl.Tracks) == 0 {
		return
	}

	byID := make(map[string]int, len(pl.Tracks))
	for i := range pl.Tracks {
		byID[string(pl.Tracks[i].Id)] = i
	}
	ordered := make([]api.Track, 0, len(s.TrackIDs))
	for _, id := range s.TrackIDs {
		if i, ok := byID[id]; ok {
			ordered = append(ordered, pl.Tracks[i])
		}
	}
	if len(ordered) == 0 {
		return
	}
	pl.Tracks = ordered
	lists.SetItem(at, pl)

	idx := 0
	if s.Current >= 0 && s.Current < len(s.TrackIDs) {
		if want := s.TrackIDs[s.Current]; want != "" {
			for j := range ordered {
				if string(ordered[j].Id) == want {
					idx = j
					break
				}
			}
		}
	}
	pl.CurrentTrack = idx
	pl.SelectedTrack = idx
	lists.SetItem(at, pl)

	m.currentPlaylistIndex = at
	m.currentIsRadio = s.IsRadio
	if s.IsRadio != m.isRadioTab {
		m.toggleRadioTab()
	}
	lists.Select(at)
	m.displayPlaylist(pl)
	m.tracklist.Select(idx)

	if !config.Current.ResumeOnStart {
		return
	}
	pos := s.PositionMs
	if dur := int64(pl.Tracks[idx].DurationMs); pos < 0 || pos >= dur-1000 {
		pos = 0
	}
	m.pendingResumePos = pos
	m.playTrack(&pl.Tracks[idx])
}
