package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFile(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

func ReadFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return data, nil
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

func DirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	return !os.IsNotExist(err) && info.IsDir()
}

func EnsureDir(dirPath string) error {
	if !DirExists(dirPath) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}
	}
	return nil
}

func RemoveDir(dirPath string) error {
	if DirExists(dirPath) {
		if err := os.RemoveAll(dirPath); err != nil {
			return fmt.Errorf("failed to remove directory %s: %w", dirPath, err)
		}
	}
	return nil
}

func ListFiles(dirPath string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var files []os.FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, info)
	}

	return files, nil
}

func GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}
	return info.Size(), nil
}