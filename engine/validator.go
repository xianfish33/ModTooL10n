package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ValidateStructure(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return fmt.Errorf("response does not start with '{'")
	}
	if !strings.HasSuffix(trimmed, "}") {
		return fmt.Errorf("response does not end with '}'")
	}
	var v interface{}
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	_, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf("JSON root is not an object")
	}
	return nil
}

func ValidateKeys(original, translated TranslationMap) ValidationResult {
	res := ValidationResult{IsValidJSON: true}

	for k := range original {
		if _, ok := translated[k]; !ok {
			res.MissingKeys = append(res.MissingKeys, k)
		}
	}
	for k := range translated {
		if _, ok := original[k]; !ok {
			res.ExtraKeys = append(res.ExtraKeys, k)
		}
	}

	for k, v := range original {
		if tv, ok := translated[k]; ok && tv != v {
			res.HasChanges = true
			break
		}
	}
	if !res.HasChanges {
		for k := range original {
			if _, ok := translated[k]; ok {
				res.HasChanges = true
				break
			}
		}
	}

	return res
}

func RepairTranslation(raw string, original TranslationMap) (TranslationMap, error) {
	translated := make(TranslationMap)
	if err := json.Unmarshal([]byte(raw), &translated); err != nil {
		return nil, fmt.Errorf("parse repaired JSON: %w", err)
	}

	for k, v := range original {
		if _, ok := translated[k]; !ok {
			translated[k] = v
		}
	}

	for k := range translated {
		if _, ok := original[k]; !ok {
			delete(translated, k)
		}
	}

	return translated, nil
}

func ValidateTranslation(original, translated TranslationMap) []string {
	var warnings []string
	for k, v := range original {
		tv, ok := translated[k]
		if !ok {
			continue
		}
		if tv == v {
			warnings = append(warnings, fmt.Sprintf("key %q: value not translated", k))
		}
	}
	return warnings
}
