package assets

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed guide/*.md
var guideFS embed.FS

const (
	GuideZhName = "README.zh-CN.md"
	GuideEnName = "README.en.md"
)

// EnsureGuides writes embedded guides into dir (usually %APPDATA%\pdf-watcher).
func EnsureGuides(dir string) (zhPath, enPath string, err error) {
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	zhPath = filepath.Join(dir, GuideZhName)
	enPath = filepath.Join(dir, GuideEnName)
	if err = writeGuide("guide/"+GuideZhName, zhPath); err != nil {
		return "", "", err
	}
	if err = writeGuide("guide/"+GuideEnName, enPath); err != nil {
		return "", "", err
	}
	return zhPath, enPath, nil
}

func writeGuide(embedName, dest string) error {
	data, err := guideFS.ReadFile(embedName)
	if err != nil {
		return err
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
