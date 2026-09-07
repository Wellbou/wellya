package mainpage

import (
	"errors"
	"net/url"
	"sync"

	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/cache"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/ui/components/playlist"
)

type initialLoadDoneMsg struct {
	client               *api.YaMusicClient
	myWaveMenuBlock      menuBlock
	stationsMenuBlock    menuBlock
	localTracksMenuBlock menuBlock
	likedTracksMenuBlock menuBlock
	likedAlbumsMenuBlock menuBlock
	pinnedAlbumsMenuBlock  menuBlock
	userPlaylistsMenuBlock menuBlock
}

type menuBlock struct {
	items []*playlist.Item
	err   error
}

func (m *Model) initialLoad() {
	var client *api.YaMusicClient

	if len(config.Current.Token) == 0 {
		log.Print(log.LVL_ERROR, "missing client token, check the config file at '%s'", config.Path())
		m.Send(errorToastMsg{reason: "missing token"})
	} else {
		c, err := api.NewClient(config.DirName, config.Current.Token)
		client = c
		if err != nil {
			if _, ok := err.(*url.Error); ok {
				log.Print(log.LVL_ERROR, "failed to connect to the Yandex server: %s", err)
				m.Send(errorToastMsg{reason: "unable to connect to the Yandex server"})
			} else {
				log.Print(log.LVL_ERROR, "client init error: %s", err)
				m.Send(errorToastMsg{reason: "unable to login: " + err.Error()})
			}
		}
	}

	var (
		wg                     sync.WaitGroup
		myWaveMenuBlock        menuBlock
		stationsMenuBlock      menuBlock
		localTracksMenuBlock   menuBlock
		likedTracksMenuBlock   menuBlock
		likedAlbumsMenuBlock   menuBlock
		pinnedAlbumsMenuBlock  menuBlock
		userPlaylistsMenuBlock menuBlock
	)

	wg.Add(7)
	go loadMyWave(client, &wg, &myWaveMenuBlock)
	go loadStations(client, &wg, &stationsMenuBlock)
	go m.loadLocalTracks(&wg, &localTracksMenuBlock)
	go loadLikedTracks(client, &wg, &likedTracksMenuBlock)
	go loadLikedAlbums(client, &wg, &likedAlbumsMenuBlock)
	go loadPinnedAlbums(client, &wg, &pinnedAlbumsMenuBlock)
	go loadUserPlaylists(client, &wg, &userPlaylistsMenuBlock)
	wg.Wait()

	m.Send(initialLoadDoneMsg{
		client:                 client,
		myWaveMenuBlock:        myWaveMenuBlock,
		stationsMenuBlock:      stationsMenuBlock,
		localTracksMenuBlock:   localTracksMenuBlock,
		likedTracksMenuBlock:   likedTracksMenuBlock,
		likedAlbumsMenuBlock:   likedAlbumsMenuBlock,
		pinnedAlbumsMenuBlock:  pinnedAlbumsMenuBlock,
		userPlaylistsMenuBlock: userPlaylistsMenuBlock,
	})
}

func (m *Model) applyInitialLoad(d initialLoadDoneMsg) {
	m.client = d.client
	m.tracker.HideError()
	m.offline = d.client == nil
	if m.offline {
		m.playlists.InsertItem(-1, playlist.ItemCategory("offline — check token in ~/.config/wellya"))
	}

	myWaveMenuBlock := d.myWaveMenuBlock
	stationsMenuBlock := d.stationsMenuBlock
	localTracksMenuBlock := d.localTracksMenuBlock
	likedTracksMenuBlock := d.likedTracksMenuBlock
	likedAlbumsMenuBlock := d.likedAlbumsMenuBlock
	pinnedAlbumsMenuBlock := d.pinnedAlbumsMenuBlock
	userPlaylistsMenuBlock := d.userPlaylistsMenuBlock

	if stationsMenuBlock.err == nil {
		m.radioPlaylists.InsertItem(-1, playlist.ItemCategory("stations:"))
		for _, item := range stationsMenuBlock.items {
			m.radioPlaylists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to list stations: %s", stationsMenuBlock.err)
		m.Send(errorToastMsg{reason: "stations list"})
	}

	m.playlists.InsertItem(-1, playlist.ItemCategory("likes:"))
	if likedTracksMenuBlock.err == nil {
		for _, item := range likedTracksMenuBlock.items {
			m.playlists.InsertItem(-1, item)
			for _, tr := range item.Tracks {
				m.likedTracksMap[string(tr.Id)] = true
			}
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain liked tracks: %s", likedTracksMenuBlock.err)
		m.Send(errorToastMsg{reason: "liked tracks"})
	}

	if likedAlbumsMenuBlock.err == nil {
		for _, item := range likedAlbumsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
			for i := range item.Albums {
				m.likedAlbumsMap[uint64(item.Albums[i].Id)] = true
			}
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain liked albums: %s", likedAlbumsMenuBlock.err)
		m.Send(errorToastMsg{reason: "liked albums"})
	}

	if pinnedAlbumsMenuBlock.err == nil {
		for _, item := range pinnedAlbumsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain pinned albums: %s", pinnedAlbumsMenuBlock.err)
		m.Send(errorToastMsg{reason: "pinned albums"})
	}

	if userPlaylistsMenuBlock.err == nil {
		for _, item := range userPlaylistsMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to obtain user playlists: %s", userPlaylistsMenuBlock.err)
		m.Send(errorToastMsg{reason: "playlists"})
	}

	if myWaveMenuBlock.err == nil {
		m.playlists.InsertItem(-1, playlist.ItemEmpty())
		for _, item := range myWaveMenuBlock.items {
			m.playlists.InsertItem(-1, item)
		}
	} else {
		log.Print(log.LVL_ERROR, "unable to init rotor session: %s", myWaveMenuBlock.err)
		m.Send(errorToastMsg{reason: "unable to init rotor session"})
	}

	m.playlists.InsertItem(-1, playlist.ItemEmpty())
	m.playlists.InsertItem(-1, &playlist.Item{Name: "history", Kind: playlist.HISTORY, Active: true, Subitem: false})

	if localTracksMenuBlock.err == nil {
		for _, item := range localTracksMenuBlock.items {
			m.playlists.InsertItem(-1, item)
			for _, tr := range item.Tracks {
				m.cachedTracksMap[string(tr.Id)] = true
			}
		}
	} else {
		log.Print(log.LVL_ERROR, "failed to list cached tracks: %s", localTracksMenuBlock.err)
		m.Send(errorToastMsg{reason: "cache list"})
	}

	m.currentPlaylistIndex = -1
	m.currentIsRadio = false
	m.playlists.Select(0)
	m.radioPlaylists.Select(0)

	active := m.activePlaylists()
	items := active.Items()
	sel := 0
	for i, it := range items {
		if it.Active {
			sel = i
			break
		}
	}
	active.Select(sel)
	if len(items) > 0 {
		selectedPlaylist := items[sel]
		m.displayPlaylist(selectedPlaylist)
		m.indicateCurrentTrackPlaying(m.tracker.IsPlaying())
		m.tracklist.Shufflable = (selectedPlaylist.Kind != playlist.NONE && selectedPlaylist.Kind != playlist.MYWAVE && selectedPlaylist.Kind != playlist.STATION && selectedPlaylist.Kind != playlist.HISTORY && len(selectedPlaylist.Tracks) > 0)
	}
	m.restoreSession()
}

func loadMyWave(client *api.YaMusicClient, wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	if client == nil {
		return
	}

	session, err := client.RotorNewSession(api.MyWaveId)
	if err != nil {
		block.err = err
		return
	}

	st := &playlist.Item{Name: "my wave", Kind: playlist.MYWAVE, Active: true, Subitem: false, Rotor: true}
	st.SessionId = session.RadioSessionId
	st.SessionBatch = session.BatchId
	if len(session.AcceptedSeeds) > 0 {
		st.StationId = session.AcceptedSeeds[0]
	} else {
		st.StationId = session.Id
	}
	if len(session.Sequence) > 0 {
		st.Tracks = []api.Track{session.Sequence[0].Track}
	} else {
		block.err = errors.New("unable to get session tracks")
	}

	block.items = append(block.items, st)
}

func (m *Model) loadLocalTracks(wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	localTracks, err := cache.ListTracks()
	if err != nil {
		block.err = err
		return
	}

	st := &playlist.Item{Name: "local", Kind: playlist.LOCAL, Active: true, Subitem: false, Tracks: localTracks}
	block.items = append(block.items, st)
}

func loadLikedTracks(client *api.YaMusicClient, wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	if client == nil {
		return
	}

	likes, err := client.LikedTracks()
	if err != nil {
		block.err = err
		return
	}

	ids := make([]string, len(likes))
	for i, tr := range likes {
		ids[i] = string(tr.Id)
	}

	tracks, err := client.Tracks(ids)
	if err != nil {
		block.err = err
		return
	}

	st := &playlist.Item{Name: "tracks", Kind: playlist.LIKES, Active: len(tracks) > 0, Subitem: true, Tracks: tracks}
	block.items = append(block.items, st)
}

func loadLikedAlbums(client *api.YaMusicClient, wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	if client == nil {
		return
	}

	likedAlbums, err := client.LikedAlbums()
	if err != nil {
		block.err = err
		return
	}

	albums := make([]api.Album, 0, len(likedAlbums))
	for _, albumInfo := range likedAlbums {
		album, err := client.Album(uint64(albumInfo.Id), true)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain album [%d] info: %s", albumInfo.Id, err)
			continue
		}
		albums = append(albums, album)
	}

	st := &playlist.Item{Name: "albums", Kind: playlist.ALBUMS, Active: len(albums) > 0, Subitem: true, Albums: albums}
	block.items = append(block.items, st)
}

func loadPinnedAlbums(client *api.YaMusicClient, wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	if client == nil {
		return
	}

	pinnedAlbums, err := client.PinnedAlbums()
	if err != nil {
		block.err = err
		return
	}

	albums := make([]api.Album, 0, len(pinnedAlbums))
	for _, albumInfo := range pinnedAlbums {
		album, err := client.Album(uint64(albumInfo.Data.Id), true)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to obtain pinned album [%d] info: %s", albumInfo.Data.Id, err)
			continue
		}
		albums = append(albums, album)
	}

	if len(albums) > 0 {
		block.items = append(block.items, playlist.ItemEmpty())
		block.items = append(block.items, playlist.ItemCategory("pins:"))

		for _, album := range albums {
			var albumTracks []api.Track
			for _, volume := range album.Volumes {
				albumTracks = append(albumTracks, volume...)
			}
			if len(albumTracks) == 0 {
				continue
			}
			block.items = append(block.items, &playlist.Item{
				Name:    album.Title,
				Kind:    playlist.ALBUMS,
				Active:  true,
				Subitem: true,
				Tracks:  albumTracks,
			})
		}
	}
}

func loadUserPlaylists(client *api.YaMusicClient, wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	if client == nil {
		return
	}

	playlists, err := client.ListPlaylists()
	if err != nil {
		block.err = err
		return
	}

	playlistTracks := make([][]api.Track, len(playlists))
	var innerWg sync.WaitGroup
	for i, pl := range playlists {
		innerWg.Add(1)
		go func(i int, pl api.Playlist) {
			defer innerWg.Done()
			tracks, terr := client.PlaylistTracks(uint64(pl.Kind), uint64(pl.Owner.Uid), false)
			if terr != nil {
				log.Print(log.LVL_ERROR, "failed to obtain user playlist [%s] tracks: %s", pl.Title, terr)
				return
			}
			playlistTracks[i] = tracks
		}(i, pl)
	}
	innerWg.Wait()

	if len(playlists) > 0 {
		block.items = append(block.items, playlist.ItemEmpty())
		block.items = append(block.items, playlist.ItemCategory("playlists:"))

		for i, pl := range playlists {
			tracks := playlistTracks[i]
			if len(tracks) == 0 {
				log.Print(log.LVL_WARNING, "empty user playlist [%s], skipped", pl.Title)
				continue
			}
			block.items = append(block.items, &playlist.Item{
				Name:     pl.Title,
				Kind: uint64(pl.Kind),
				Revision: int(pl.Revision),
				Active:   true,
				Subitem:  true,
				Tracks:   tracks,
			})
		}
	}
}

func loadStations(client *api.YaMusicClient, wg *sync.WaitGroup, block *menuBlock) {
	defer wg.Done()

	if client == nil {
		return
	}

	stations, err := client.Stations("ru")
	if err != nil {
		block.err = err
		return
	}

	for _, st := range stations {
		block.items = append(block.items, &playlist.Item{
			Name:      st.Station.Name,
			Kind:      playlist.STATION,
			StationId: st.Station.Id,
			Active:    true,
			Subitem:   true,
		})
	}
}
