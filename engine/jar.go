package engine

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func ScanJARs(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("access path %q: %w", path, err)
	}

	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(path), ".jar") {
			return []string{path}, nil
		}
		return nil, fmt.Errorf("not a jar file: %s", path)
	}

	var jars []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(p), ".jar") {
			jars = append(jars, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir %q: %w", path, err)
	}
	return jars, nil
}

func ExtractJAR(jarPath string, cacheRoot string) (*JarResult, error) {
	modID := modIDFromPath(jarPath)
	cacheDir := filepath.Join(cacheRoot, modID)

	if err := os.RemoveAll(cacheDir); err != nil {
		return nil, fmt.Errorf("clean cache %q: %w", cacheDir, err)
	}

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, fmt.Errorf("open jar: %w", err)
	}
	defer r.Close()

	result := &JarResult{
		Path:     jarPath,
		ModID:    modID,
		CacheDir: cacheDir,
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		targetPath := filepath.Join(cacheDir, f.Name)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("mkdir %q: %w", filepath.Dir(targetPath), err)
		}

		if err := extractFile(f, targetPath); err != nil {
			return nil, fmt.Errorf("extract %q: %w", f.Name, err)
		}

		lower := strings.ToLower(f.Name)

		if isLangFile(lower) {
			parts := strings.Split(f.Name, "/")
			langCode := strings.TrimSuffix(parts[len(parts)-1], ".json")
			result.LangFiles = append(result.LangFiles, LangFile{
				Path:     f.Name,
				LangCode: langCode,
				TempPath: targetPath,
			})
		}

		if loader := isMetadataFile(lower); loader != "" && result.ModMeta == nil {
			data, err := os.ReadFile(targetPath)
			if err != nil {
				continue
			}
			var meta *ModMetadata
			switch loader {
			case "fabric":
				meta = readFabricMetadata(data)
			case "neoforge", "forge":
				meta = readForgeMetadata(data, loader)
			}
			if meta != nil {
				result.ModMeta = meta
			}
		}
	}

	return result, nil
}

func RepackJAR(srcDir, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create jar: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
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

		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(writer, src)
		return err
	})

	if err != nil {
		return fmt.Errorf("walk source dir: %w", err)
	}

	return nil
}

func modIDFromPath(jarPath string) string {
	name := filepath.Base(jarPath)
	name = strings.TrimSuffix(name, ".jar")
	name = strings.TrimSuffix(name, ".JAR")
	return name
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func isLangFile(lower string) bool {
	return strings.Contains(lower, "/lang/") && strings.HasSuffix(lower, ".json")
}

const (
	fabricModJSON      = "fabric.mod.json"
	neoforgeModsTOML   = "meta-inf/neoforge.mods.toml"
	forgeModsTOML      = "meta-inf/mods.toml"
)

func isMetadataFile(lower string) string {
	if lower == fabricModJSON {
		return "fabric"
	}
	if lower == neoforgeModsTOML {
		return "neoforge"
	}
	if lower == forgeModsTOML {
		return "forge"
	}
	return ""
}

type fabricMod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func readFabricMetadata(data []byte) *ModMetadata {
	var fm fabricMod
	if err := json.Unmarshal(data, &fm); err != nil {
		return nil
	}
	if fm.ID == "" {
		return nil
	}
	return &ModMetadata{
		ID:          fm.ID,
		Name:        fm.Name,
		Description: fm.Description,
		Loader:      "fabric",
	}
}

type tomlModFile struct {
	Mods []tomlMod `toml:"mods"`
}

type tomlMod struct {
	ModID       string `toml:"modId"`
	DisplayName string `toml:"displayName"`
	Description string `toml:"description"`
}

func readForgeMetadata(data []byte, loader string) *ModMetadata {
	var mf tomlModFile
	if err := toml.Unmarshal(data, &mf); err != nil {
		return nil
	}
	if len(mf.Mods) == 0 {
		return nil
	}
	m := mf.Mods[0]
	if m.ModID == "" {
		return nil
	}
	return &ModMetadata{
		ID:          m.ModID,
		Name:        m.DisplayName,
		Description: m.Description,
		Loader:      loader,
	}
}
