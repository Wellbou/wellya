package main

import (
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
	log.Start()
	defer log.Stop()

	err := config.InitialLoad()
	if err != nil {
		log.Print(log.LVL_WARNING, "config load error: %s", err.Error())
	}

	style.Apply(config.Current.Style)
	api.SetupClient(config.Current.Proxy)

	if config.Current.Token == "" {
		err = loginpage.New().Run()
		if err != nil {
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
