package source

import "os"

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func isNotExist(err error) bool { return os.IsNotExist(err) }
