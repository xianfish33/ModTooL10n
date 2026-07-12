package engine

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type packMeta struct {
	Pack packDescriptor `json:"pack"`
}

type packDescriptor struct {
	PackFormat       int    `json:"pack_format"`
	Description      string `json:"description"`
	SupportedFormats []int  `json:"supported_formats,omitempty"`
}

func CreateResourcePack(packDir, packName string) (string, error) {
	meta := packMeta{
		Pack: packDescriptor{
			PackFormat:       46,
			Description:      "由ModTooL10n自动输出的汉化资源包",
			SupportedFormats: []int{34, 46},
		},
	}

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal pack.mcmeta: %w", err)
	}

	metaPath := filepath.Join(packDir, "pack.mcmeta")
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return "", fmt.Errorf("write pack.mcmeta: %w", err)
	}

	zipName := SanitizePackName(packName)
	if !strings.HasSuffix(strings.ToLower(zipName), ".zip") {
		zipName += ".zip"
	}
	zipPath := filepath.Join(filepath.Dir(packDir), zipName)

	outFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	err = filepath.Walk(packDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(packDir, path)
		if err != nil {
			return err
		}
		zipName := strings.ReplaceAll(rel, "\\", "/")

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipName
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(w, src)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("walk pack dir: %w", err)
	}

	return zipPath, nil
}

func SanitizePackName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
}
