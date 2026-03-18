package utils

import (
	"boxchat/internal/config"
	"net"
	"net/url"
	"strings"
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
	idx := strings.LastIndex(filename, ".")
	if idx == -1 || idx == len(filename)-1 {
		return false
	}
	ext := strings.ToLower(filename[idx+1:])
	return ImageExtensionsMap[ext]
}

func IsMusicFile(filename string) bool {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 || idx == len(filename)-1 {
		return false
	}
	ext := strings.ToLower(filename[idx+1:])
	return MusicExtensionsMap[ext]
}

func IsVideoFile(filename string) bool {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 || idx == len(filename)-1 {
		return false
	}
	ext := strings.ToLower(filename[idx+1:])
	return VideoExtensionsMap[ext]
}

func AllowedFile(filename string) bool {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 || idx == len(filename)-1 {
		return false
	}
	ext := strings.ToLower(filename[idx+1:])
	return AllowedExtensionsMap[ext]
}

// SanitizeHTML escapes HTML special characters to prevent XSS attacks
func SanitizeHTML(input string) string {
	if input == "" {
		return ""
	}
	
	// Escape HTML special characters
	input = strings.ReplaceAll(input, "&", "&amp;")
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")
	
	return input
}

// IsValidExternalURL checks if a URL is safe to use (not pointing to internal resources)
// Returns true if the URL is valid and doesn't point to internal/private networks
func IsValidExternalURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	
	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}
	
	// Get the hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return false
	}
	
	// Resolve the hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return false
	}
	
	// Check if any of the resolved IPs are in private/internal ranges
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return false
		}
	}
	
	// Check for localhost hostnames
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return false
	}
	
	return true
}

// isPrivateIP checks if an IP address is in a private/internal range
func isPrivateIP(ip net.IP) bool {
	// Check for private IP ranges
	privateRanges := []struct {
		*net.IPNet
	}{
		// RFC 1918: 10.0.0.0/8
		{&net.IPNet{
			IP:   net.IPv4(10, 0, 0, 0),
			Mask: net.CIDRMask(8, 32),
		}},
		// RFC 1918: 172.16.0.0/12
		{&net.IPNet{
			IP:   net.IPv4(172, 16, 0, 0),
			Mask: net.CIDRMask(12, 32),
		}},
		// RFC 1918: 192.168.0.0/16
		{&net.IPNet{
			IP:   net.IPv4(192, 168, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}},
		// Loopback: 127.0.0.0/8
		{&net.IPNet{
			IP:   net.IPv4(127, 0, 0, 0),
			Mask: net.CIDRMask(8, 32),
		}},
		// Link-local: 169.254.0.0/16 (including AWS metadata)
		{&net.IPNet{
			IP:   net.IPv4(169, 254, 0, 0),
			Mask: net.CIDRMask(16, 32),
		}},
		// AWS metadata endpoint (explicit check)
		{&net.IPNet{
			IP:   net.IPv4(169, 254, 169, 254),
			Mask: net.CIDRMask(32, 32),
		}},
	}
	
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	
	return false
}
