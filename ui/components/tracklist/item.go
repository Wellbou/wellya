package tracklist

import (
	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/ui/helpers"
)

type Item struct {
	Track        *api.Track
	Album        *api.Album
	Artists      string
	IsPlaying    bool
	IsSuggestion bool
	PlayCount    int
}

func NewItem(track *api.Track) Item {
	return Item{
		Track:   track,
		Artists: helpers.ArtistList(track.Artists),
	}
}

func NewAlbumItem(album *api.Album) Item {
	return Item{
		Album:   album,
		Artists: helpers.ArtistList(album.Artists),
	}
}

func (i Item) FilterValue() string {
	if i.Track != nil {
		return i.Track.Title + " " + i.Artists
	}
	if i.Album != nil {
		return i.Album.Title
	}
	return ""
}
