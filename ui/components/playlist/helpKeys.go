package playlist

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/wellbou/wellya/config"
)

type helpKeyMap struct {
	CursorUp      key.Binding
	CursorDown    key.Binding
	Rename        key.Binding
	HidePlaylists key.Binding
	Radio         key.Binding
	Renamable     bool
}

func newHelpMap() *helpKeyMap {
	controls := config.Current.Controls
	return &helpKeyMap{
		CursorUp:      key.NewBinding(controls.PlaylistsUp.Binding(), controls.PlaylistsUp.Help("up")),
		CursorDown:    key.NewBinding(controls.PlaylistsDown.Binding(), controls.PlaylistsDown.Help("down")),
		Rename:        key.NewBinding(controls.PlaylistsRename.Binding(), controls.PlaylistsRename.Help("rename")),
		HidePlaylists: key.NewBinding(controls.PlaylistsHide.Binding(), controls.PlaylistsHide.Help("hide")),
		Radio:         key.NewBinding(controls.PlaylistsRadio.Binding(), controls.PlaylistsRadio.Help("radio")),
	}
}

func (k helpKeyMap) ShortHelp() []key.Binding {
	controls := config.Current.Controls
	return []key.Binding{key.NewBinding(controls.KeysHelp.Binding(), controls.KeysHelp.Help("all hotkeys"))}
}

func (k helpKeyMap) FullHelp() [][]key.Binding {
	bindings := [][]key.Binding{
		k.ShortHelp(),
	}

	if k.Renamable {
		bindings = append(bindings, []key.Binding{k.Rename})
	}

	bindings = append(bindings, []key.Binding{k.HidePlaylists, k.Radio})

	return bindings
}
