package core

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

// MediaInfo holds metadata for image and video files.
type MediaInfo struct {
	FileType   string // "image", "video", "audio", "unknown"
	MimeType   string
	Width      int
	Height     int
	Duration   string // for video/audio
	FrameCount int    // for animated images (GIF)
	Format     string // e.g., "JPEG", "PNG", "GIF", "MP4"
	Size       int64  // file size in bytes
}

// ReadMediaInfo reads metadata for image and video files.
// Returns MediaInfo with available data; unsupported formats return FileType "unknown".
func ReadMediaInfo(path string) (MediaInfo, error) {
	info := MediaInfo{}

	f, err := os.Open(path)
	if err != nil {
		return info, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return info, err
	}
	info.Size = stat.Size()

	ext := strings.ToLower(filepath.Ext(path))

	// Try image formats first
	switch ext {
	case ".png":
		info.FileType = "image"
		info.Format = "PNG"
		if cfg, err := png.DecodeConfig(f); err == nil {
			info.Width = cfg.Width
			info.Height = cfg.Height
			info.MimeType = "image/png"
		}
	case ".jpg", ".jpeg":
		info.FileType = "image"
		info.Format = "JPEG"
		if cfg, err := jpeg.DecodeConfig(f); err == nil {
			info.Width = cfg.Width
			info.Height = cfg.Height
			info.MimeType = "image/jpeg"
		}
	case ".gif":
		info.FileType = "image"
		info.Format = "GIF"
		if cfg, err := gif.DecodeConfig(f); err == nil {
			info.Width = cfg.Width
			info.Height = cfg.Height
			info.MimeType = "image/gif"
			// Count frames for animated GIF
			f.Seek(0, 0)
			if img, err := gif.DecodeAll(f); err == nil {
				info.FrameCount = len(img.Image)
			}
		}
	case ".bmp":
		info.FileType = "image"
		info.Format = "BMP"
		// BMP support would need additional decoder
		info.MimeType = "image/bmp"
	case ".webp":
		info.FileType = "image"
		info.Format = "WebP"
		// WebP support would need additional decoder
		info.MimeType = "image/webp"
	default:
		// Try to detect by content
		f.Seek(0, 0)
		if img, _, err := image.DecodeConfig(f); err == nil {
			info.FileType = "image"
			info.Width = img.Width
			info.Height = img.Height
		}
	}

	// Video/audio formats (metadata only - no decoding)
	switch ext {
	case ".mp4", ".mkv", ".webm", ".mov", ".avi", ".flv", ".wmv":
		info.FileType = "video"
		info.Format = strings.ToUpper(ext[1:])
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac":
		info.FileType = "audio"
		info.Format = strings.ToUpper(ext[1:])
	}

	// Try to extract text from PDF if not already handled
	if info.FileType == "document" && info.Format == "PDF" {
		// PDF text already extracted above
	}

	// If still unknown, try to detect
	if info.FileType == "" {
		info.FileType = "unknown"
	}

	return info, nil
}

// MediaInfoString returns a human-readable string representation of MediaInfo.
func (m MediaInfo) MediaInfoString() string {
	if m.FileType == "unknown" {
		return "Unknown media format"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Type: %s (%s)", m.FileType, m.Format))
	if m.Width > 0 && m.Height > 0 {
		parts = append(parts, fmt.Sprintf("Dimensions: %dx%d", m.Width, m.Height))
	}
	if m.FrameCount > 1 {
		parts = append(parts, fmt.Sprintf("Frames: %d", m.FrameCount))
	}
	if m.Duration != "" {
		parts = append(parts, fmt.Sprintf("Duration: %s", m.Duration))
	}
	if m.Size > 0 {
		parts = append(parts, fmt.Sprintf("Size: %s", formatSize(m.Size)))
	}

	return strings.Join(parts, "\n")
}

// extractPDFText extracts text content from a PDF file.
func ExtractPDFText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	r, err := pdf.NewReader(f, fi.Size())
	if err != nil {
		return "", err
	}

	var text strings.Builder
	numPages := r.NumPage()
	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
		text.WriteString("\n")
	}
	return text.String(), nil
}

// formatSize is a local copy to avoid import cycle (move to shared util later)
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
