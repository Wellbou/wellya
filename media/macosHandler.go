//go:build darwin && !nomedia

package media

import (
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/media/handler/macos"
)

func NewHandler(name, description string) handler.MediaHandler {
	return macos.NewHandler(name, description)
}
