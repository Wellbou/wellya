package search

import "github.com/wellbou/wellya/api"

type Item struct {
	Label string
	Track *api.Track
}

func (i Item) FilterValue() string {
	return i.Label
}
