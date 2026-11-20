package extract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const (
	maxPDFPages   = 4
	maxDOCXRunes  = 20000
)

type Result struct {
	Text   string
	Reason string // empty if ok or empty text with reason
}

func FromFile(path string) Result {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return fromPDF(path)
	case ".docx":
		return fromDOCX(path)
	default:
		return Result{Reason: "unsupported extension"}
	}
}

func CheckMagic(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 8)
	n, _ := f.Read(buf)
	buf = buf[:n]
	switch ext {
	case ".pdf":
		if !bytes.HasPrefix(buf, []byte("%PDF")) {
			return fmt.Errorf("magic: not PDF")
		}
	case ".docx":
		if !bytes.HasPrefix(buf, []byte("PK")) {
			return fmt.Errorf("magic: not ZIP/DOCX")
		}
		zr, err := zip.OpenReader(path)
		if err != nil {
			return fmt.Errorf("magic: zip open: %w", err)
		}
		defer zr.Close()
		found := false
		for _, zf := range zr.File {
			if zf.Name == "word/document.xml" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("magic: missing word/document.xml")
		}
	default:
		return fmt.Errorf("unsupported")
	}
	return nil
}

func fromPDF(path string) (res Result) {
	defer func() {
		if r := recover(); r != nil {
			res = Result{Reason: fmt.Sprintf("pdf panic: %v", r)}
		}
	}()
	f, r, err := pdf.Open(path)
	if err != nil {
		return Result{Reason: err.Error()}
	}
	defer f.Close()
	n := r.NumPage()
	if n > maxPDFPages {
		n = maxPDFPages
	}
	var b strings.Builder
	for i := 1; i <= n; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return Result{Reason: "pdf empty text"}
	}
	return Result{Text: text}
}

func fromDOCX(path string) Result {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Result{Reason: err.Error()}
	}
	defer zr.Close()
	var xmlData []byte
	for _, zf := range zr.File {
		if zf.Name == "word/document.xml" {
			rc, err := zf.Open()
			if err != nil {
				return Result{Reason: err.Error()}
			}
			xmlData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return Result{Reason: err.Error()}
			}
			break
		}
	}
	if xmlData == nil {
		return Result{Reason: "missing document.xml"}
	}
	text := stripXMLText(string(xmlData))
	text = truncateRunes(text, maxDOCXRunes)
	if strings.TrimSpace(text) == "" {
		return Result{Reason: "docx empty text"}
	}
	return Result{Text: text}
}

func stripXMLText(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return collapseSpace(b.String())
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n])
}

// FrontText returns the leading portion used for DOI hunting.
func FrontText(text string, maxRunes int) string {
	return truncateRunes(text, maxRunes)
}
