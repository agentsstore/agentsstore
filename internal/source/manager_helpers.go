package source

import "github.com/wu/agentsstore/internal/config"

func loadIfExists(path string) (*config.Config, error) {
	if _, err := readFile(path); err != nil {
		if isNotExist(err) {
			return &config.Config{
				Server:  config.ServerConfig{Listen: ":8080", DataDir: "./data"},
				Sources: []config.Source{},
			}, nil
		}
		return nil, err
	}
	return config.Load(path)
}
