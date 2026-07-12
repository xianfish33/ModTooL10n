package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func LoadLang(path string) (TranslationMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return TranslationMap(raw), nil
}

func SaveLang(path string, data TranslationMap) error {
	out := make(map[string]string, len(data))
	for k, v := range data {
		out[k] = v
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, raw, 0644)
}

func SaveLangPretty(path string, data TranslationMap) error {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString(fmt.Sprintf("  %q: ", k))
		valBytes, err := json.Marshal(data[k])
		if err != nil {
			return fmt.Errorf("marshal value for key %q: %w", k, err)
		}
		b.Write(valBytes)
	}
	b.WriteString("\n}\n")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func ChunkLang(data TranslationMap, maxKeys int) []TranslationMap {
	if len(data) <= maxKeys {
		return []TranslationMap{data}
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var chunks []TranslationMap
	for i := 0; i < len(keys); i += maxKeys {
		end := i + maxKeys
		if end > len(keys) {
			end = len(keys)
		}
		chunk := make(TranslationMap)
		for _, k := range keys[i:end] {
			chunk[k] = data[k]
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}

func MergeLang(chunks []TranslationMap) TranslationMap {
	result := make(TranslationMap)
	for _, chunk := range chunks {
		for k, v := range chunk {
			result[k] = v
		}
	}
	return result
}

func ChunkToJSON(chunk TranslationMap) (string, error) {
	raw, err := json.MarshalIndent(chunk, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func EstimateTokens(data TranslationMap) int {
	total := 0
	for k, v := range data {
		total += len(k)/4 + 1
		total += len(v)/4 + 1
	}
	return total
}
