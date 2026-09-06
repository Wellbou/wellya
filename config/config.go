package config

import (
	"os"
	"path/filepath"
	"reflect"

	"gopkg.in/yaml.v3"
)

var Current Config

const OldDirName = "yamusic-tui"

func migrateOldConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldDir := filepath.Join(home, ".config", OldDirName)
	newDir := filepath.Join(home, ".config", DirName)
	if _, err := os.Stat(newDir); err == nil {
		return
	}
	if _, err := os.Stat(oldDir); err != nil {
		return
	}
	_ = os.MkdirAll(newDir, 0755)
	if data, err := os.ReadFile(filepath.Join(oldDir, "config.yaml")); err == nil {
		_ = os.WriteFile(filepath.Join(newDir, "config.yaml"), data, 0644)
	}
	if data, err := os.ReadFile(filepath.Join(oldDir, "token")); err == nil {
		_ = os.WriteFile(filepath.Join(newDir, "token"), data, 0600)
	}
}

func InitialLoad() error {
	migrateOldConfig()

	conf, err := load()
	if err != nil {
		if os.IsNotExist(err) {
			conf = defaultConfig
			configDir, derr := getDir()
			if derr != nil {
				return derr
			}

			if oldToken, rerr := os.ReadFile(filepath.Join(configDir, "token")); rerr == nil {
				conf.Token = string(oldToken)
			}

			Current = conf
			return save(Current)
		}
		Current = conf
		return err
	}

	Current = conf
	return nil
}

func getDir() (string, error) {
	userDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(userDir, ".config", DirName)
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return "", err
	}

	return configDir, nil
}

func load() (Config, error) {
	configDir, err := getDir()
	if err != nil {
		return defaultConfig, err
	}

	configContent, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		return defaultConfig, err
	}

	var newConfig Config
	err = yaml.Unmarshal(configContent, &newConfig)
	if err != nil {
		return defaultConfig, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(configContent, &raw); err == nil {
		if _, ok := raw["buffer-size-ms"]; !ok {
			newConfig.BufferSize = defaultConfig.BufferSize
		}
		if _, ok := raw["rewind-duration-s"]; !ok {
			newConfig.RewindDuration = defaultConfig.RewindDuration
		}
		if _, ok := raw["volume"]; !ok {
			newConfig.Volume = defaultConfig.Volume
		}
		if _, ok := raw["volume-step"]; !ok {
			newConfig.VolumeStep = defaultConfig.VolumeStep
		}
		if _, ok := raw["audio-quality"]; !ok {
			newConfig.AudioQuality = defaultConfig.AudioQuality
		}
		if _, ok := raw["cache-tracks"]; !ok {
			newConfig.CacheTracks = defaultConfig.CacheTracks
		}
	}

	if newConfig.VolumeStep == 0 {
		newConfig.VolumeStep = defaultConfig.VolumeStep
	}

	if newConfig.Search == nil {
		search := *defaultConfig.Search
		newConfig.Search = &search
	}

	if newConfig.Controls == nil {
		controls := *defaultConfig.Controls
		newConfig.Controls = &controls
	} else {
		fillDefault(newConfig.Controls, defaultConfig.Controls)
		if newConfig.Controls.Quit.IsEmpty() {
			newConfig.Controls.Quit = defaultConfig.Controls.Quit
		}
	}

	if newConfig.Style == nil {
		style := *defaultConfig.Style
		newConfig.Style = &style
	} else {
		if newConfig.Style.Icons == nil {
			icons := *defaultConfig.Style.Icons
			newConfig.Style.Icons = &icons
		} else {
			fillDefault(newConfig.Style.Icons, defaultConfig.Style.Icons)
		}
		if newConfig.Style.Colors == nil {
			colors := *defaultConfig.Style.Colors
			newConfig.Style.Colors = &colors
		} else {
			fillDefault(newConfig.Style.Colors, defaultConfig.Style.Colors)
		}
		if newConfig.Style.SidePanelWidth == 0 {
			newConfig.Style.SidePanelWidth = defaultConfig.Style.SidePanelWidth
		}
	}

	return newConfig, nil
}

func fillDefault(target, values any) {
	targetStruct := reflect.ValueOf(target).Elem()
	defaultStruct := reflect.ValueOf(values).Elem()
	for i := 0; i < targetStruct.NumField(); i++ {
		field := targetStruct.Field(i)
		if field.IsZero() || field.String() == "" {
			field.Set(defaultStruct.Field(i))
		}
	}
}

func save(conf Config) error {
	configDir, err := getDir()
	if err != nil {
		return err
	}

	tmp, err := os.OpenFile(filepath.Join(configDir, "config.yaml.tmp"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	enc := yaml.NewEncoder(tmp)
	enc.SetIndent(4)
	err = enc.Encode(conf)
	cerr := enc.Close()
	if err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if cerr != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return cerr
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}

	return os.Rename(tmp.Name(), filepath.Join(configDir, "config.yaml"))
}

func Path() string {
	configDir, err := getDir()
	if err != nil {
		return ""
	}

	return filepath.Join(configDir, "config.yaml")
}

func Save() error {
	return save(Current)
}

func Reset() error {
	var err error
	Current, err = load()
	return err
}
