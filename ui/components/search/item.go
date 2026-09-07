package search

import "github.com/wellbou/wellya/api"

type Item struct {
	Label  string
	Kind   string
	Track  *api.Track
	Artist *api.Artist
	Album  *api.Album
}

func (i Item) FilterValue() string {
	return i.Label
}
