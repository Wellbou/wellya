package cache

import (
	"os"
	"path/filepath"

	"github.com/wellbou/wellya/config"
)

func getCacheDir() (string, error) {
	var (
		cacheDir string
		err      error
	)

	if len(config.Current.CacheDir) == 0 {
		userDir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cacheDir = filepath.Join(userDir, config.DirName)
	} else {
		cacheDir, err = filepath.Abs(config.Current.CacheDir)
		if err != nil {
			return "", err
		}
	}

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", err
	}

	return cacheDir, nil
}

func safeName(trackId string) string {
	safe := filepath.Base(trackId)
	if safe == "" || safe == "." || safe == ".." {
		safe = "invalid"
	}
	return safe + ".mp3"
}

func Path(trackId string) string {
	dir, err := getCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, safeName(trackId))
}

func Read(trackId string) (*os.File, int64, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, 0, err
	}

	file, err := os.Open(filepath.Join(dir, safeName(trackId)))
	if err != nil {
		return nil, 0, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, stat.Size(), nil
}

func Write(trackId string) (*os.File, error) {
	dir, err := getCacheDir()
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filepath.Join(dir, safeName(trackId)), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func Remove(trackId string) error {
	dir, err := getCacheDir()
	if err != nil {
		return err
	}

	return os.Remove(filepath.Join(dir, safeName(trackId)))
}
