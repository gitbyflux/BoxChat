package utils

import (
	"boxchat/internal/config"
	"net"
	"testing"
)

// setupTestExtensions initializes extension maps with test data
func setupTestExtensions() {
	cfg := &config.Config{
		AllowedExtensions: []string{"jpg", "png", "gif", "pdf", "txt", "doc", "mp3", "wav", "mp4", "avi"},
		ImageExtensions:   []string{"jpg", "jpeg", "png", "gif", "webp", "bmp"},
		MusicExtensions:   []string{"mp3", "wav", "flac", "aac", "ogg"},
		VideoExtensions:   []string{"mp4", "avi", "mkv", "mov", "webm"},
	}
	InitExtensions(cfg)
}

// ============================================================================
// IsImageFile Tests
// ============================================================================

func TestIsImageFile(t *testing.T) {
	setupTestExtensions()

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Valid JPG", "photo.jpg", true},
		{"Valid JPEG", "image.jpeg", true},
		{"Valid PNG", "screenshot.png", true},
		{"Valid GIF", "animation.gif", true},
		{"Valid WebP", "picture.webp", true},
		{"Valid BMP", "bitmap.bmp", true},
		{"Invalid PDF", "document.pdf", false},
		{"Invalid MP3", "song.mp3", false},
		{"Invalid MP4", "video.mp4", false},
		{"Uppercase extension", "PHOTO.JPG", true},
		{"Mixed case extension", "Photo.JpG", true},
		{"No extension", "noextension", false},
		{"Empty string", "", false},
		{"Dotfile", ".hidden", false},
		{"Multiple dots", "archive.tar.gz", false},
		{"Path with filename", "/uploads/images/photo.png", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsImageFile(tt.filename)
			if result != tt.expected {
				t.Errorf("IsImageFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsMusicFile Tests
// ============================================================================

func TestIsMusicFile(t *testing.T) {
	setupTestExtensions()

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Valid MP3", "song.mp3", true},
		{"Valid WAV", "audio.wav", true},
		{"Valid FLAC", "track.flac", true},
		{"Valid AAC", "music.aac", true},
		{"Valid OGG", "sound.ogg", true},
		{"Invalid JPG", "photo.jpg", false},
		{"Invalid MP4", "video.mp4", false},
		{"Invalid PDF", "document.pdf", false},
		{"Uppercase extension", "SONG.MP3", true},
		{"Mixed case extension", "Song.Mp3", true},
		{"No extension", "noextension", false},
		{"Empty string", "", false},
		{"Path with filename", "/uploads/music/track.wav", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMusicFile(tt.filename)
			if result != tt.expected {
				t.Errorf("IsMusicFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsVideoFile Tests
// ============================================================================

func TestIsVideoFile(t *testing.T) {
	setupTestExtensions()

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Valid MP4", "movie.mp4", true},
		{"Valid AVI", "clip.avi", true},
		{"Valid MKV", "film.mkv", true},
		{"Valid MOV", "video.mov", true},
		{"Valid WebM", "animation.webm", true},
		{"Invalid MP3", "song.mp3", false},
		{"Invalid JPG", "photo.jpg", false},
		{"Invalid PDF", "document.pdf", false},
		{"Uppercase extension", "MOVIE.MP4", true},
		{"Mixed case extension", "Movie.Mp4", true},
		{"No extension", "noextension", false},
		{"Empty string", "", false},
		{"Path with filename", "/uploads/videos/clip.avi", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVideoFile(tt.filename)
			if result != tt.expected {
				t.Errorf("IsVideoFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// AllowedFile Tests
// ============================================================================

func TestAllowedFile(t *testing.T) {
	setupTestExtensions()

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Valid JPG", "photo.jpg", true},
		{"Valid PNG", "image.png", true},
		{"Valid GIF", "animation.gif", true},
		{"Valid PDF", "document.pdf", true},
		{"Valid TXT", "notes.txt", true},
		{"Valid DOC", "report.doc", true},
		{"Valid MP3", "song.mp3", true},
		{"Valid WAV", "audio.wav", true},
		{"Valid MP4", "video.mp4", true},
		{"Valid AVI", "clip.avi", true},
		{"Invalid EXE", "program.exe", false},
		{"Invalid SH", "script.sh", false},
		{"Invalid BAT", "batch.bat", false},
		{"Invalid ZIP", "archive.zip", false},
		{"Uppercase extension", "PHOTO.JPG", true},
		{"Mixed case extension", "Photo.JpG", true},
		{"No extension", "noextension", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllowedFile(tt.filename)
			if result != tt.expected {
				t.Errorf("AllowedFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// InitExtensions Tests
// ============================================================================

func TestInitExtensions(t *testing.T) {
	// Clear maps before test
	AllowedExtensionsMap = nil
	ImageExtensionsMap = nil
	MusicExtensionsMap = nil
	VideoExtensionsMap = nil

	cfg := &config.Config{
		AllowedExtensions: []string{".jpg", ".png", ".pdf"},
		ImageExtensions:   []string{".jpg", ".png"},
		MusicExtensions:   []string{".mp3"},
		VideoExtensions:   []string{".mp4"},
	}

	InitExtensions(cfg)

	// Verify maps are initialized
	if AllowedExtensionsMap == nil {
		t.Error("AllowedExtensionsMap should not be nil")
	}
	if ImageExtensionsMap == nil {
		t.Error("ImageExtensionsMap should not be nil")
	}
	if MusicExtensionsMap == nil {
		t.Error("MusicExtensionsMap should not be nil")
	}
	if VideoExtensionsMap == nil {
		t.Error("VideoExtensionsMap should not be nil")
	}

	// Verify correct extensions are mapped
	tests := []struct {
		name     string
		ext      string
		mapName  string
		expected bool
	}{
		{"Allowed JPG", ".jpg", "allowed", true},
		{"Allowed PNG", ".png", "allowed", true},
		{"Allowed PDF", ".pdf", "allowed", true},
		{"Not allowed MP3", ".mp3", "allowed", false},
		{"Image JPG", ".jpg", "image", true},
		{"Not image MP3", ".mp3", "image", false},
		{"Music MP3", ".mp3", "music", true},
		{"Not music JPG", ".jpg", "music", false},
		{"Video MP4", ".mp4", "video", true},
		{"Not video JPG", ".jpg", "video", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			switch tt.mapName {
			case "allowed":
				result = AllowedExtensionsMap[tt.ext]
			case "image":
				result = ImageExtensionsMap[tt.ext]
			case "music":
				result = MusicExtensionsMap[tt.ext]
			case "video":
				result = VideoExtensionsMap[tt.ext]
			}

			if result != tt.expected {
				t.Errorf("%s: got %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestFileExtensionEdgeCases(t *testing.T) {
	setupTestExtensions()

	tests := []struct {
		name     string
		filename string
		testFunc func(string) bool
	}{
		{"Double extension tar.gz", "archive.tar.gz", AllowedFile},
		{"Triple extension", "file.tar.gz.enc", AllowedFile},
		{"Hidden file with extension", ".config.json", AllowedFile},
		{"Filename starting with dot", ".jpg", IsImageFile},
		{"Very long filename", "very_long_filename_with_extension.jpg", IsImageFile},
		{"Filename with spaces", "my photo.jpg", IsImageFile},
		{"Filename with special chars", "photo (1).jpg", IsImageFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify these don't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function panicked: %v", r)
				}
			}()
			tt.testFunc(tt.filename)
		})
	}
}

// ============================================================================
// Case Insensitivity Tests
// ============================================================================

func TestCaseInsensitivity(t *testing.T) {
	setupTestExtensions()

	// Test that all extension checks are case-insensitive
	testCases := []struct {
		name     string
		filename string
		testFunc func(string) bool
	}{
		{"JPG lowercase", "test.jpg", IsImageFile},
		{"JPG uppercase", "test.JPG", IsImageFile},
		{"JPG mixed case", "test.JpG", IsImageFile},
		{"MP3 lowercase", "test.mp3", IsMusicFile},
		{"MP3 uppercase", "test.MP3", IsMusicFile},
		{"MP4 lowercase", "test.mp4", IsVideoFile},
		{"MP4 uppercase", "test.MP4", IsVideoFile},
	}

	// All should return the same result (true)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.testFunc(tc.filename)
			if !result {
				t.Errorf("%s: expected true, got %v", tc.name, result)
			}
		})
	}
}

// ============================================================================
// SanitizeHTML Tests
// ============================================================================

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Plain text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "Script tag",
			input:    "<script>alert('XSS')</script>",
			expected: "&lt;script&gt;alert(&#x27;XSS&#x27;)&lt;/script&gt;",
		},
		{
			name:     "Image tag",
			input:    "<img src=x onerror=alert(1)>",
			expected: "&lt;img src=x onerror=alert(1)&gt;",
		},
		{
			name:     "Event handler",
			input:    "<div onclick=\"alert('XSS')\">",
			expected: "&lt;div onclick=&quot;alert(&#x27;XSS&#x27;)&quot;&gt;",
		},
		{
			name:     "Ampersand",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "Multiple special chars",
			input:    "<>&\"'",
			expected: "&lt;&gt;&amp;&quot;&#x27;",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Already escaped",
			input:    "&lt;script&gt;",
			expected: "&amp;lt;script&amp;gt;",
		},
		{
			name:     "SVG tag",
			input:    "<svg onload=alert(1)>",
			expected: "&lt;svg onload=alert(1)&gt;",
		},
		{
			name:     "Iframe tag",
			input:    "<iframe src='http://evil.com'></iframe>",
			expected: "&lt;iframe src=&#x27;http://evil.com&#x27;&gt;&lt;/iframe&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IsValidExternalURL Tests
// ============================================================================

func TestIsValidExternalURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid HTTPS URL",
			url:      "https://example.com/image.png",
			expected: true,
		},
		{
			name:     "Valid HTTP URL",
			url:      "http://example.com/file.jpg",
			expected: true,
		},
		{
			name:     "Giphy URL",
			url:      "https://media.giphy.com/media/abc123/giphy.gif",
			expected: true,
		},
		{
			name:     "Localhost URL",
			url:      "http://localhost:8080/file.png",
			expected: false,
		},
		{
			name:     "127.0.0.1 URL",
			url:      "http://127.0.0.1/file.png",
			expected: false,
		},
		{
			name:     "Private IP 192.168.x.x",
			url:      "http://192.168.1.1/file.png",
			expected: false,
		},
		{
			name:     "Private IP 10.x.x.x",
			url:      "http://10.0.0.1/file.png",
			expected: false,
		},
		{
			name:     "Private IP 172.16.x.x",
			url:      "http://172.16.0.1/file.png",
			expected: false,
		},
		{
			name:     "AWS metadata endpoint",
			url:      "http://169.254.169.254/latest/meta-data/",
			expected: false,
		},
		{
			name:     "Link-local address",
			url:      "http://169.254.1.1/file.png",
			expected: false,
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: false,
		},
		{
			name:     "Invalid URL",
			url:      "not-a-url",
			expected: false,
		},
		{
			name:     "FTP URL",
			url:      "ftp://example.com/file.png",
			expected: false,
		},
		{
			name:     "File URL",
			url:      "file:///etc/passwd",
			expected: false,
		},
		{
			name:     "URL without scheme",
			url:      "example.com/file.png",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidExternalURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsValidExternalURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// isPrivateIP Tests
// ============================================================================

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"Public IP 8.8.8.8", "8.8.8.8", false},
		{"Public IP 1.1.1.1", "1.1.1.1", false},
		{"Private 10.0.0.1", "10.0.0.1", true},
		{"Private 10.255.255.255", "10.255.255.255", true},
		{"Private 172.16.0.1", "172.16.0.1", true},
		{"Private 172.31.255.255", "172.31.255.255", true},
		{"Public 172.15.0.1", "172.15.0.1", false},
		{"Public 172.32.0.1", "172.32.0.1", false},
		{"Private 192.168.0.1", "192.168.0.1", true},
		{"Private 192.168.255.255", "192.168.255.255", true},
		{"Public 192.167.0.1", "192.167.0.1", false},
		{"Public 192.169.0.1", "192.169.0.1", false},
		{"Loopback 127.0.0.1", "127.0.0.1", true},
		{"Loopback 127.255.255.255", "127.255.255.255", true},
		{"Link-local 169.254.0.1", "169.254.0.1", true},
		{"Link-local 169.254.255.255", "169.254.255.255", true},
		{"AWS metadata 169.254.169.254", "169.254.169.254", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP %s", tt.ip)
			}
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}
