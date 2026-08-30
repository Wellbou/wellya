//go:build linux && !nomedia

package media

import (
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/media/handler/mpris"
)

func NewHandler(name, description string) handler.MediaHandler {
	return mpris.NewHandler(name, description)
}
