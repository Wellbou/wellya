package mainpage

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
	"github.com/wellbou/wellya/ui/components/search"
	"github.com/wellbou/wellya/ui/helpers"
)

type searchReadyMsg struct {
	gen   int
	res   api.SearchResult
	items []*playlist.Item
	index int
}

type searchSuggestMsg struct {
	gen         int
	suggestions []string
	err         error
}

type searchTabResultsMsg struct {
	gen    int
	req    string
	tracks []api.Track
	err    error
}

func (m *Model) searchControl(msg search.Control) tea.Cmd {
	switch msg {
	case search.SELECT:
		m.isSearchActive = false

		req, ok := m.searchDialog.SuggestionValue()
		if !ok {
			return nil
		}

		m.searchGen++
		gen := m.searchGen
		base := append([]*playlist.Item(nil), m.playlists.Items()...)
		filter := m.searchDialog.Filter()
		client := m.client
		go func() {
			res, err := client.Search(req, api.SEARCH_ALL)
			if err != nil {
				log.Print(log.LVL_ERROR, "failed to search [%s]: %s", req, err)
				m.Send(errorToastMsg{reason: "search"})
				return
			}
			items, index := makeSearchItems(client, base, res, filter)
			m.Send(searchReadyMsg{gen: gen, res: res, items: items, index: index})
		}()
		return nil
	case search.CANCEL:
		m.isSearchActive = false
		return nil
	case search.UPDATE_SUGGESTIONS:
		input := m.searchDialog.InputValue()
		m.searchGen++
		gen := m.searchGen
		client := m.client
		go func() {
			suggestions, err := client.SearchSuggest(input)
			if err != nil {
				m.Send(searchSuggestMsg{gen: gen, err: err})
				return
			}
			m.Send(searchSuggestMsg{gen: gen, suggestions: suggestions.Suggestions})
		}()
		return nil
	case search.TOGGLE_FILTER:
		f := m.searchDialog.Filter()
		f = (f + 1) % 5
		m.searchDialog.SetFilter(f)
		if m.hasSearchResult {
			res := m.lastSearchResult
			m.searchGen++
			gen := m.searchGen
			base := append([]*playlist.Item(nil), m.playlists.Items()...)
			filter := m.searchDialog.Filter()
			client := m.client
			go func() {
				items, index := makeSearchItems(client, base, res, filter)
				m.Send(searchReadyMsg{gen: gen, res: res, items: items, index: index})
			}()
		}
		return nil
	}

	return nil
}

func makeSearchItems(client *api.YaMusicClient, playlists []*playlist.Item, res api.SearchResult, filter int) ([]*playlist.Item, int) {
	searchResIndex := len(playlists) + 2
	for i, pl := range playlists {
		if !pl.Active && !pl.Subitem && pl.Name == "search results:" {
			playlists = playlists[:i-1]
			searchResIndex = i + 1
			break
		}
	}

	playlists = append(playlists,
		&playlist.Item{Name: "", Kind: playlist.NONE, Active: false, Subitem: false},
		&playlist.Item{Name: "search results:", Kind: playlist.NONE, Active: false, Subitem: false},
	)

	if filter == 0 || filter == 1 {
		if len(res.Tracks.Results) > 0 {
			playlists = append(playlists, &playlist.Item{
				Name:    "search \"" + res.Text + "\"",
				Active:  true,
				Subitem: true,
				Tracks:  res.Tracks.Results,
			})
		}
	}

	if filter == 0 || filter == 3 {
		if config.Current.Search.Artists && len(res.Artists.Results) > 0 {
			for _, artist := range res.Artists.Results {
				if !strings.Contains(strings.ToLower(artist.Name), strings.ToLower(res.Text)) {
					continue
				}

				artistTracks, err := client.ArtistPopularTracks(uint64(artist.Id))
				if err != nil {
					log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks: %s", artist.Name, err)
					continue
				}

				tracks, err := client.Tracks(artistTracks.Tracks)
				if err != nil {
					log.Print(log.LVL_ERROR, "failed to obtain artist [%s] tracks full info: %s", artist.Name, err)
					continue
				}

				playlists = append(playlists, &playlist.Item{
					Name:    artist.Name,
					Active:  true,
					Subitem: true,
					Tracks:  tracks,
				})
			}
		}
	}

	if filter == 0 || filter == 2 {
		if config.Current.Search.Albums && len(res.Albums.Results) > 0 {
			for _, album := range res.Albums.Results {
				if !strings.Contains(strings.ToLower(album.Title), strings.ToLower(res.Text)) {
					continue
				}

				albumWithTracks, err := client.Album(uint64(album.Id), true)
				if err != nil {
					log.Print(log.LVL_ERROR, "failed to obtain album [%s] tracks: %s", album.Title, err)
					continue
				}

				albumArtists := helpers.ArtistList(albumWithTracks.Artists)
				if len(albumWithTracks.Volumes) > 1 {
					for i := range albumWithTracks.Volumes {
						playlists = append(playlists, &playlist.Item{
							Name:    fmt.Sprintf("%s vol.%d (%s)", albumWithTracks.Title, i, albumArtists),
							Active:  true,
							Subitem: true,
							Tracks:  albumWithTracks.Volumes[i],
						})
					}
				} else {
					playlists = append(playlists, &playlist.Item{
						Name:    fmt.Sprintf("%s (%s)", albumWithTracks.Title, albumArtists),
						Active:  true,
						Subitem: true,
						Tracks:  albumWithTracks.Volumes[0],
					})
				}
			}
		}
	}

	if filter == 0 || filter == 4 {
		if config.Current.Search.Playlists && len(res.Playlists.Results) > 0 {
			for _, pl := range res.Playlists.Results {
				if !strings.Contains(strings.ToLower(pl.Title), strings.ToLower(res.Text)) {
					continue
				}

				playlistTracks, err := client.PlaylistTracks(uint64(pl.Kind), uint64(pl.Owner.Uid), false)
				if err != nil {
					log.Print(log.LVL_ERROR, "failed to obtain playlist [%s] tracks: %s", pl.Title, err)
					continue
				}

				playlists = append(playlists, &playlist.Item{
					Name:    pl.Title + " by " + pl.Owner.Name,
					Active:  true,
					Subitem: true,
					Tracks:  playlistTracks,
				})
			}
		}
	}

	return playlists, searchResIndex
}
