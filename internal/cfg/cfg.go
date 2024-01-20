package cfg

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/topi314/gobin/v2/internal/env"
)

func Update(f func(map[string]string)) (string, error) {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".gobin")
	}

	cfgFile, err := os.OpenFile(configPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}
	defer cfgFile.Close()

	cfg := make(map[string]string)
	if err = env.NewDecoder(cfgFile).Decode(&cfg); err != nil {
		return "", err
	}

	f(cfg)

	if err = cfgFile.Truncate(0); err != nil {
		return "", err
	}
	if _, err = cfgFile.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return configPath, env.NewEncoder(cfgFile).Encode(cfg)
}

func Get() (string, error) {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".gobin")
	}

	cfgFile, err := os.OpenFile(configPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer cfgFile.Close()

	buff := new(bytes.Buffer)
	if _, err = io.Copy(buff, cfgFile); err != nil {
		return "", err
	}

	return buff.String(), nil
}

func GetKeyValue() (map[string]string, error) {
	r, err := Get()
	if err != nil {
		return nil, err
	}

	cfg := make(map[string]string)
	if err = env.NewDecoder(bytes.NewReader([]byte(r))).Decode(&cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
