package docker

import (
	"docker-cr/pkg/utils"
)

// Compatibility functions for the docker package
func writeFile(filePath string, data []byte) error {
	return utils.WriteFile(filePath, data)
}

func readFile(filePath string) ([]byte, error) {
	return utils.ReadFile(filePath)
}