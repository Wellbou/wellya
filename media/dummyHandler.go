//go:build nomedia

package media

import (
	"github.com/wellbou/wellya/media/handler"
	"github.com/wellbou/wellya/media/handler/dummy"
)

func NewHandler(name, description string) handler.MediaHandler {
	return dummy.NewHandler(name, description)
}
