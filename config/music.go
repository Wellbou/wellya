package config

import (
	"os"
	"path/filepath"
	"strings"
)

func MusicDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(home, ".config", "user-dirs.dirs")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "XDG_MUSIC_DIR=") {
				continue
			}
			v := strings.Trim(strings.SplitN(line, "=", 2)[1], `"`)
			v = strings.ReplaceAll(v, "$HOME", home)
			if st, err := os.Stat(v); err == nil && st.IsDir() {
				return v
			}
		}
	}
	if st, err := os.Stat(filepath.Join(home, "Музыка")); err == nil && st.IsDir() {
		return filepath.Join(home, "Музыка")
	}
	if st, err := os.Stat(filepath.Join(home, "Music")); err == nil && st.IsDir() {
		return filepath.Join(home, "Music")
	}
	if strings.HasPrefix(os.Getenv("LANG"), "ru") {
		return filepath.Join(home, "Музыка")
	}
	return filepath.Join(home, "Music")
}
