package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// stripJSONQuotedContent removes all quoted strings (including the quotes)
// from a JSON string, leaving only structural characters and whitespace.
// Handles escaped quotes inside strings correctly.
func stripJSONQuotedContent(s string) string {
	var result strings.Builder
	inString := false
	escaped := false
	for _, r := range s {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// collapseWhitespace replaces all whitespace sequences with a single space
// and trims leading/trailing whitespace.
func collapseWhitespace(s string) string {
	var result strings.Builder
	lastWasSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				result.WriteRune(' ')
				lastWasSpace = true
			}
		} else {
			result.WriteRune(r)
			lastWasSpace = false
		}
	}
	return strings.TrimSpace(result.String())
}

// ValidateChunkStructure compares the structural skeleton of the original
// chunk JSON and the model output by stripping all quoted strings and
// comparing the remaining characters (ignoring whitespace).
// This accepts fragments without {} wrapping as well as complete JSON objects.
func ValidateChunkStructure(originalJSON, output string) error {
	origStripped := stripJSONQuotedContent(originalJSON)
	outStripped := stripJSONQuotedContent(output)

	origStripped = collapseWhitespace(origStripped)
	outStripped = collapseWhitespace(outStripped)

	if origStripped != outStripped {
		return fmt.Errorf("structure mismatch")
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
