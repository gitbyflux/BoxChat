package utils

import (
	"strings"
	"boxchat/internal/config"
)

var (
	AllowedExtensionsMap map[string]bool
	ImageExtensionsMap   map[string]bool
	MusicExtensionsMap   map[string]bool
	VideoExtensionsMap   map[string]bool
)

func InitExtensions(cfg *config.Config) {
	AllowedExtensionsMap = make(map[string]bool)
	for _, ext := range cfg.AllowedExtensions {
		AllowedExtensionsMap[strings.ToLower(ext)] = true
	}
	
	ImageExtensionsMap = make(map[string]bool)
	for _, ext := range cfg.ImageExtensions {
		ImageExtensionsMap[strings.ToLower(ext)] = true
	}
	
	MusicExtensionsMap = make(map[string]bool)
	for _, ext := range cfg.MusicExtensions {
		MusicExtensionsMap[strings.ToLower(ext)] = true
	}
	
	VideoExtensionsMap = make(map[string]bool)
	for _, ext := range cfg.VideoExtensions {
		VideoExtensionsMap[strings.ToLower(ext)] = true
	}
}

func IsImageFile(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filename[strings.LastIndex(filename, "."):], "."))
	return ImageExtensionsMap[ext]
}

func IsMusicFile(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filename[strings.LastIndex(filename, "."):], "."))
	return MusicExtensionsMap[ext]
}

func IsVideoFile(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filename[strings.LastIndex(filename, "."):], "."))
	return VideoExtensionsMap[ext]
}

func AllowedFile(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filename[strings.LastIndex(filename, "."):], "."))
	return AllowedExtensionsMap[ext]
}
