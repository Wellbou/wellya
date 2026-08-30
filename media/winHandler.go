//go:build windows && !nomedia

package media

import (
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/media/handler/win"
)

func NewHandler(name, description string) handler.MediaHandler {
	return win.NewHandler(name, description)
}
