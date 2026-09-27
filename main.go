package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wellbou/wellya/api"
	"github.com/wellbou/wellya/config"
	"github.com/wellbou/wellya/log"
	"github.com/wellbou/wellya/media"
	"github.com/wellbou/wellya/ui/model"
	loginpage "github.com/wellbou/wellya/ui/model/loginPage"
	mainpage "github.com/wellbou/wellya/ui/model/mainPage"
	"github.com/wellbou/wellya/ui/style"
)

func main() {
	if handleCliCommand() {
		return
	}

	log.Start()
	defer log.Stop()

	go cleanTempDir()

	err := config.InitialLoad()
	if err != nil {
		log.Print(log.LVL_WARNING, "config load error: %s", err.Error())
	}

	if w := config.CollisionWarnings(); w != "" {
		log.Print(log.LVL_WARNING, w)
	}

	style.Apply(config.Current.Style)
	api.SetupClient(config.Current.Proxy)

	if config.Current.Token == "" {
		err = loginpage.New().Run()
		if err != nil {
			if errors.Is(err, loginpage.ErrQuitLogin) {
				return
			}
			log.Print(log.LVL_PANIC, err.Error())
			model.PrettyExit(err, 4)
		}
	}

	mediaHandler := media.NewHandler(config.DirName, config.AppName)
	page := mainpage.New(mediaHandler)
	err = mediaHandler.Start(page.Run)
	if err != nil {
		log.Print(log.LVL_PANIC, err.Error())
		model.PrettyExit(err, 6)
	}
}

// cleanTempDir drops stale per-track scratch files (ID3 headers for the
// cache feature and MPRIS cover art) so /tmp/wellya doesn't grow forever.
// Only files untouched for a day are removed; the current track is safe.
func cleanTempDir() {
	dir := filepath.Join(os.TempDir(), config.DirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasPrefix(name, "metadata-") || strings.HasSuffix(name, ".jpg")) {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, name))
		}
	}
}
