package tracklist

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/wellbou/wellya/config"
)

type helpKeyMap struct {
	CursorUp           key.Binding
	CursorDown         key.Binding
	PageUp             key.Binding
	PageDown           key.Binding
	Play               key.Binding
	LikeUnlike         key.Binding
	AddToPlaylist      key.Binding
	RemoveFromPlaylist key.Binding
	Search             key.Binding
	Share              key.Binding
	Shuffle            key.Binding
	Back               key.Binding
	Reload             key.Binding
	ShowHelp           key.Binding
	CloseHelp          key.Binding
	HideTracklist      key.Binding
	MoveUp             key.Binding
	MoveDown           key.Binding
	JumpToPlaying      key.Binding
	ShowQueue          key.Binding
	ArtistBrowse       key.Binding
	TrackInfo          key.Binding
	GoToAlbum          key.Binding
	Dislike            key.Binding
	Sort               key.Binding
	Filter             key.Binding
	RemoveFromQueue    key.Binding
	Export             key.Binding
	Upload             key.Binding
	Stats              key.Binding

	Shufflable bool
}

func newHelpMap() *helpKeyMap {
	controls := config.Current.Controls
	return &helpKeyMap{
		CursorUp:           key.NewBinding(controls.CursorUp.Binding(), controls.CursorUp.Help("up")),
		CursorDown:         key.NewBinding(controls.CursorDown.Binding(), controls.CursorDown.Help("down")),
		PageUp:             key.NewBinding(controls.TracksNextPage.Binding(), controls.TracksNextPage.Help("page up")),
		PageDown:           key.NewBinding(controls.TracksPrevPage.Binding(), controls.TracksPrevPage.Help("page down")),
		Play:               key.NewBinding(controls.Apply.Binding(), controls.Apply.Help("play")),
		LikeUnlike:         key.NewBinding(controls.TracksLike.Binding(), controls.TracksLike.Help("like/unlike")),
		AddToPlaylist:      key.NewBinding(controls.TracksAddToPlaylist.Binding(), controls.TracksAddToPlaylist.Help("add to")),
		RemoveFromPlaylist: key.NewBinding(controls.TracksRemoveFromPlaylist.Binding(), controls.TracksRemoveFromPlaylist.Help("remove")),
		Search:             key.NewBinding(controls.TracksSearch.Binding(), controls.TracksSearch.Help("search")),
		Share:              key.NewBinding(controls.TracksShare.Binding(), controls.TracksShare.Help("share")),
		Shuffle:            key.NewBinding(controls.TracksShuffle.Binding(), controls.TracksShuffle.Help("shuffle")),
		Back:               key.NewBinding(controls.TracksBack.Binding(), controls.TracksBack.Help("back")),
		Reload:             key.NewBinding(controls.Reload.Binding(), controls.Reload.Help("reload")),
		HideTracklist:      key.NewBinding(controls.TracksHide.Binding(), controls.TracksHide.Help("hide")),
		MoveUp:             key.NewBinding(controls.TracksMoveUp.Binding(), controls.TracksMoveUp.Help("move up")),
		MoveDown:           key.NewBinding(controls.TracksMoveDown.Binding(), controls.TracksMoveDown.Help("move down")),
		JumpToPlaying:      key.NewBinding(controls.TracksJumpToPlaying.Binding(), controls.TracksJumpToPlaying.Help("jump to playing")),
		ShowQueue:          key.NewBinding(controls.TracksShowQueue.Binding(), controls.TracksShowQueue.Help("up next")),
		ArtistBrowse:       key.NewBinding(controls.TracksArtistBrowse.Binding(), controls.TracksArtistBrowse.Help("artist")),
		TrackInfo:          key.NewBinding(controls.TracksInfo.Binding(), controls.TracksInfo.Help("track info")),
		GoToAlbum:          key.NewBinding(controls.TracksGoToAlbum.Binding(), controls.TracksGoToAlbum.Help("go to album")),
		Dislike:            key.NewBinding(controls.TracksDislike.Binding(), controls.TracksDislike.Help("dislike")),
		Sort:               key.NewBinding(controls.TracksSort.Binding(), controls.TracksSort.Help("sort")),
		Filter:             key.NewBinding(controls.TracksFilter.Binding(), controls.TracksFilter.Help("filter")),
		RemoveFromQueue:    key.NewBinding(controls.TracksRemoveFromQueue.Binding(), controls.TracksRemoveFromQueue.Help("remove from queue")),
		Export:             key.NewBinding(controls.TracksExport.Binding(), controls.TracksExport.Help("export m3u")),
		Upload:             key.NewBinding(controls.TracksUpload.Binding(), controls.TracksUpload.Help("upload mp3")),
		Stats:              key.NewBinding(controls.TracksStats.Binding(), controls.TracksStats.Help("stats")),
		ShowHelp:           key.NewBinding(controls.ShowAllKeys.Binding(), controls.ShowAllKeys.Help("show keys")),
		CloseHelp:          key.NewBinding(controls.ShowAllKeys.Binding(), controls.ShowAllKeys.Help("hide keys")),
	}
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.CursorUp, k.CursorDown, k.Play, k.LikeUnlike, k.JumpToPlaying, k.ArtistBrowse, k.TrackInfo, k.Dislike, k.RemoveFromQueue, k.Export, k.Upload, k.Stats, k.Sort, k.Filter, k.ShowHelp}
}

func (k helpKeyMap) FullHelp() [][]key.Binding {
	bindings := [][]key.Binding{
		{k.CursorUp, k.CursorDown, k.PageUp, k.PageDown},
		{k.Play, k.LikeUnlike, k.AddToPlaylist, k.RemoveFromPlaylist},
		{k.Search, k.Share, k.MoveUp, k.MoveDown},
		{k.JumpToPlaying, k.ShowQueue, k.ArtistBrowse, k.TrackInfo, k.GoToAlbum, k.Dislike, k.RemoveFromQueue, k.Export, k.Upload, k.Stats, k.Sort, k.Filter},
	}

	if k.Shufflable {
		bindings[2] = append(bindings[2], k.Shuffle)
	}

	return append(bindings, []key.Binding{k.Reload, k.HideTracklist, k.CloseHelp, k.Back})
}

func (k helpKeyMap) HelpHeight() int {
	maxLines := 0
	keys := k.FullHelp()
	for i := range keys {
		if len(keys[i]) > maxLines {
			maxLines = len(keys[i])
		}
	}
	return maxLines
}
