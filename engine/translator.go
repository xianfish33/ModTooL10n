package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type TranslatorConfig struct {
	MaxChunkKeys   int
	MaxRetries     int
	TargetLang     string
	TargetCode     string
	OutputDir      string
	OutputMode     OutputMode
	PackName       string
	RetryMode      RetryMode
	RetryThreshold int
	OnProgress     func(ProgressInfo)
}

type ProgressInfo struct {
	ChunkIdx    int
	TotalChunks int
	Status      string
	KeysTotal   int
	ChunkKeys   int
	ParsedKeys  int
	Retry       int
	MaxRetries  int
	Error       error
}

type Translator struct {
	client *LLMClient
	cfg    TranslatorConfig
}

func NewTranslator(client *LLMClient, cfg TranslatorConfig) *Translator {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.MaxChunkKeys <= 0 {
		cfg.MaxChunkKeys = 50
	}
	if cfg.RetryMode == "" {
		cfg.RetryMode = RetryModeCorrect
	}
	if cfg.RetryThreshold <= 0 {
		cfg.RetryThreshold = 2
	}
	if cfg.RetryThreshold > cfg.MaxRetries {
		cfg.RetryThreshold = cfg.MaxRetries
	}
	return &Translator{client: client, cfg: cfg}
}

func ResolveLangCode(client *LLMClient, input string) string {
	system := "你是一个Minecraft语言代码转换专家。请将用户输入的语言名称转换为对应的Minecraft语言代码（如en_us、zh_cn、ja_jp、ko_kr、de_de等）。只返回代码本身，不要包含任何其他文字。如果输入已经是代码格式则直接返回。"
	user := fmt.Sprintf("语言: %s", input)
	raw, err := client.Chat(system, user)
	if err != nil {
		return input
	}
	code := strings.TrimSpace(raw)
	code = strings.ToLower(code)
	if len(code) > 10 || !strings.Contains(code, "_") {
		return input
	}
	return code
}

func buildSystemPrompt(modID, modName, modDesc, targetLang string) string {
	return fmt.Sprintf(`你是一个Minecraft模组专业翻译专家。请将以下JSON中的英文字符串翻译成%s。

模组ID: %s
模组名称: %s
模组描述: %s
目标语言代码: %s

请根据模组ID、名称和描述自行判断该模组的类型和风格，选择合适的翻译策略：
- 科技/工业类 → 术语准确、简洁直译
- 自然/农业/食物类 → 自然优美、适当诗意
- 魔法/神秘类 → 富有奇幻感、神秘色彩
- 冒险/战斗类 → 直接有力、动作感强
- 实用工具类 → 简洁明了、功能导向

严格规则（必须遵守）:
1. 只返回纯JSON对象, 不要包含任何markdown标记、代码块包裹或额外文字说明
2. 保留所有键(key)完全不变, 只翻译值(value)中的英文文本为目标语言
3. 保留所有占位符 (%%s, %%d, %%, {{...}}, <...> 等) 原封不动
4. 保留 \\n, \\t, \\\" 等转义字符原样
5. 保留 Minecraft 物品ID、命名空间、颜色代码§ 等不翻译
6. 不要修改英文的模组名称、物品名称等专有名词，除非它们有标准中文翻译
7. 注意 Minecraft 游戏内术语的标准中文翻译一致性
8. 每条翻译值必须用双引号包裹
9. 确保JSON语法正确, 末尾不能有多余逗号
10. 【最重要】输出的JSON必须与输入的JSON拥有完全相同的键集合，不允许增加、删除或修改任何键
11. 【最重要】只翻译英文值为目标语言文本，不允许在值中添加任何非翻译内容（如注释、说明、解释等）
12. 【最重要】输出必须是纯JSON，不允许包含任何非JSON内容
13. 【重要】这是分块JSON，原文可能不以{开头或不以}结尾，这是正常的分块结果，请原样返回翻译后的JSON片段，不要补全大括号或添加任何额外字符

输出格式要求 - 必须严格遵守:
不允许输出额外内容，只输出原文相同的json格式，每个键都保留，不出现额外文字说明。
不允许修改键名，不允许增加或删除键，只翻译值中的英文文本。`,
		targetLang, modID, modName, modDesc, targetLang)
}

func (t *Translator) Translate(modID string, data TranslationMap, modName, modDesc string) (*Result, error) {
	return t.TranslateWithJar(modID, data, modName, modDesc, "", "")
}

func (t *Translator) TranslateWithJar(modID string, data TranslationMap, modName, modDesc, enJarPath, cacheDir string) (*Result, error) {
	targetLang := t.cfg.TargetLang
	if targetLang == "" {
		targetLang = "简体中文"
	}
	targetCode := t.cfg.TargetCode
	if targetCode == "" {
		targetCode = strings.ToLower(targetLang)
	}

	// Check if target language file already exists and matches English keys
	if cacheDir != "" && enJarPath != "" {
		targetJarPath := deriveTargetLangPath(enJarPath, targetCode)
		targetFile := filepath.Join(cacheDir, targetJarPath)
		if _, err := os.Stat(targetFile); err == nil {
			existing, err := LoadLang(targetFile)
			if err == nil && len(existing) == len(data) {
				keysMatch := true
				for k := range data {
					if _, ok := existing[k]; !ok {
						keysMatch = false
						break
					}
				}
				if keysMatch {
					// Language file already exists with matching keys
					switch t.cfg.OutputMode {
					case OutputModeMod:
						jarOut := filepath.Join(t.cfg.OutputDir, modID+".jar")
						if err := RepackJAR(cacheDir, jarOut); err == nil {
							return &Result{
								ModID:      modID,
								OutputPath: jarOut,
								TotalKeys:  len(data),
								Skipped:    true,
								SkipMsg:    fmt.Sprintf("%s.json 语言文件已存在，跳过翻译", targetCode),
							}, nil
						}
					case OutputModePack:
						return &Result{
							ModID:      modID,
							OutputPath: "",
							TotalKeys:  len(data),
							Skipped:    true,
							SkipMsg:    fmt.Sprintf("%s.json 语言文件已存在，跳过翻译", targetCode),
						}, nil
					default:
						jarOut := filepath.Join(t.cfg.OutputDir, modID+".jar")
						if err := RepackJAR(cacheDir, jarOut); err == nil {
							return &Result{
								ModID:      modID,
								OutputPath: jarOut,
								TotalKeys:  len(data),
								Skipped:    true,
								SkipMsg:    fmt.Sprintf("%s.json 语言文件已存在，跳过翻译", targetCode),
							}, nil
						}
					}
					return &Result{
						ModID:      modID,
						OutputPath: targetFile,
						TotalKeys:  len(data),
						Skipped:    true,
						SkipMsg:    fmt.Sprintf("%s.json 语言文件已存在，跳过翻译", targetCode),
					}, nil
				}
			}
		}
	}

	chunks := ChunkLang(data, t.cfg.MaxChunkKeys)
	result := &Result{
		ModID:       modID,
		TotalKeys:   len(data),
		ChunksTotal: len(chunks),
	}

	systemPrompt := buildSystemPrompt(modID, modName, modDesc, targetCode)

	type chunkResult struct {
		index int
		trans TranslationMap
		err   error
	}

	results := make([]*chunkResult, len(chunks))
	var wg sync.WaitGroup
	progressFn := t.cfg.OnProgress

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, c TranslationMap) {
			defer wg.Done()

			if progressFn != nil {
				progressFn(ProgressInfo{ChunkIdx: idx, TotalChunks: len(chunks), Status: "translating", KeysTotal: len(data), ChunkKeys: len(c)})
			}

			cr := &chunkResult{index: idx}
			cr.trans, cr.err = t.translateChunk(idx, systemPrompt, c, len(chunks), func(parsed, total int) {
				if progressFn != nil {
					progressFn(ProgressInfo{ChunkIdx: idx, TotalChunks: len(chunks), Status: "translating", KeysTotal: len(data), ChunkKeys: total, ParsedKeys: parsed})
				}
			})

			// If first attempt failed, retry up to MaxRetries
			if cr.err != nil && t.cfg.MaxRetries > 1 {
				retryChunkJSON, _ := ChunkToJSON(c)
				originalOuterPrompt := fmt.Sprintf("请翻译以下JSON内容 (%d/%d):\n\n%s", idx+1, len(chunks), retryChunkJSON)
				correctiveOuterFmt := "你的翻译输出有误，请严格检查并重新翻译。这是分块JSON，可能不以{开头或不以}结尾，这是正常的。必须返回与输入完全相同键集合的纯JSON片段，不要补全大括号或添加额外内容。\n\n原始JSON:\n%s"
				for retry := 2; retry <= t.cfg.MaxRetries; retry++ {
					if progressFn != nil {
						progressFn(ProgressInfo{ChunkIdx: idx, TotalChunks: len(chunks), Status: "retrying", KeysTotal: len(data), ChunkKeys: len(c), Retry: retry, MaxRetries: t.cfg.MaxRetries, Error: cr.err})
					}

					retryPrompt := t.retryPrompt(originalOuterPrompt, retry, retryChunkJSON, correctiveOuterFmt)
					retryRaw, retryErr := t.client.ChatStream(systemPrompt, retryPrompt, func(acc string) {
						if progressFn != nil {
							count := countParsedKeys(acc, c)
							if count > 0 {
								progressFn(ProgressInfo{ChunkIdx: idx, TotalChunks: len(chunks), Status: "retrying", KeysTotal: len(data), ChunkKeys: len(c), ParsedKeys: count, Retry: retry, MaxRetries: t.cfg.MaxRetries})
							}
						}
					})
					if retryErr != nil {
						cr.err = fmt.Errorf("retry %d API: %w", retry, retryErr)
						continue
					}

					cleaned := cleanResponse(retryRaw)
					if err := ValidateChunkStructure(retryChunkJSON, cleaned); err != nil {
						cr.err = fmt.Errorf("retry %d structure: %w", retry, err)
						continue
					}

					retryTranslated := make(TranslationMap)
					if err := json.Unmarshal([]byte(cleaned), &retryTranslated); err != nil {
						cr.err = fmt.Errorf("retry %d unmarshal: %w", retry, err)
						continue
					}

					vRes := ValidateKeys(c, retryTranslated)
					if len(vRes.MissingKeys) > 0 {
						cr.err = fmt.Errorf("retry %d missing %d keys: %v", retry, len(vRes.MissingKeys), vRes.MissingKeys)
						continue
					}

					cr.trans = retryTranslated
					cr.err = nil
					break
				}
			}

			if cr.err != nil {
				if progressFn != nil {
					progressFn(ProgressInfo{ChunkIdx: idx, TotalChunks: len(chunks), Status: "failed", KeysTotal: len(data), ChunkKeys: len(c), Error: cr.err})
				}
			} else {
				if progressFn != nil {
					progressFn(ProgressInfo{ChunkIdx: idx, TotalChunks: len(chunks), Status: "done", KeysTotal: len(data), ChunkKeys: len(c)})
				}
			}
			results[idx] = cr
		}(i, chunk)
	}

	wg.Wait()

	if progressFn != nil {
		progressFn(ProgressInfo{Status: "merging", KeysTotal: len(data)})
	}

	translated := make(TranslationMap)
	var translatedChunks []TranslationMap
	for _, cr := range results {
		if cr.err != nil {
			result.ChunksFail++
			result.Errors = append(result.Errors, fmt.Sprintf("chunk %d: %v", cr.index+1, cr.err))
			continue
		}
		result.ChunksOK++
		translatedChunks = append(translatedChunks, cr.trans)
	}

	if result.ChunksOK == 0 {
		for k, v := range data {
			translated[k] = v
		}
		result.OutputPath = ""
		return result, fmt.Errorf("all chunks failed for mod %s", modID)
	}

	merged := MergeLang(translatedChunks)

	if result.ChunksFail > 0 {
		for k, v := range data {
			if _, ok := merged[k]; !ok {
				merged[k] = v
			}
		}
	}

	if progressFn != nil {
		progressFn(ProgressInfo{Status: "saving", KeysTotal: len(data)})
	}

	switch t.cfg.OutputMode {
	case OutputModeMod:
		if cacheDir != "" && enJarPath != "" {
			targetJarPath := deriveTargetLangPath(enJarPath, targetCode)
			targetFile := filepath.Join(cacheDir, targetJarPath)
			if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
				return nil, fmt.Errorf("create lang dir: %w", err)
			}
			if err := SaveLangPretty(targetFile, merged); err != nil {
				return nil, fmt.Errorf("save lang file: %w", err)
			}

			jarOut := filepath.Join(t.cfg.OutputDir, modID+".jar")
			if err := RepackJAR(cacheDir, jarOut); err != nil {
				return nil, fmt.Errorf("repack jar: %w", err)
			}
			result.OutputPath = jarOut
		} else {
			outputDir := filepath.Join(t.cfg.OutputDir, modID)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return nil, fmt.Errorf("create output dir: %w", err)
			}
			outputPath := filepath.Join(outputDir, targetCode+".json")
			if err := SaveLangPretty(outputPath, merged); err != nil {
				return nil, fmt.Errorf("save output: %w", err)
			}
			result.OutputPath = outputPath
		}

	case OutputModePack:
		packName := SanitizePackName(t.cfg.PackName)
		if packName == "" {
			packName = "resourcepack"
		}
		packDir := filepath.Join(t.cfg.OutputDir, packName)
		assetPath := filepath.Join(packDir, "assets", modID, "lang", targetCode+".json")
		if err := os.MkdirAll(filepath.Dir(assetPath), 0755); err != nil {
			return nil, fmt.Errorf("create asset dir: %w", err)
		}
		if err := SaveLangPretty(assetPath, merged); err != nil {
			return nil, fmt.Errorf("save lang file: %w", err)
		}
		result.OutputPath = assetPath

	default:
		if cacheDir != "" && enJarPath != "" {
			targetJarPath := deriveTargetLangPath(enJarPath, targetCode)
			targetFile := filepath.Join(cacheDir, targetJarPath)
			if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
				return nil, fmt.Errorf("create lang dir: %w", err)
			}
			if err := SaveLangPretty(targetFile, merged); err != nil {
				return nil, fmt.Errorf("save lang file: %w", err)
			}

			jarOut := filepath.Join(t.cfg.OutputDir, modID+".jar")
			if err := RepackJAR(cacheDir, jarOut); err != nil {
				return nil, fmt.Errorf("repack jar: %w", err)
			}
			result.OutputPath = jarOut
		} else {
			outputDir := filepath.Join(t.cfg.OutputDir, modID)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return nil, fmt.Errorf("create output dir: %w", err)
			}
			outputPath := filepath.Join(outputDir, targetCode+".json")
			if err := SaveLangPretty(outputPath, merged); err != nil {
				return nil, fmt.Errorf("save output: %w", err)
			}
			result.OutputPath = outputPath
		}
	}

	if progressFn != nil {
		progressFn(ProgressInfo{Status: "completed", KeysTotal: len(data)})
	}

	return result, nil
}

func deriveTargetLangPath(enJarPath, targetCode string) string {
	dir := filepath.Dir(enJarPath)
	return filepath.ToSlash(filepath.Join(dir, targetCode+".json"))
}

func (t *Translator) translateChunk(idx int, systemPrompt string, chunk TranslationMap, totalChunks int, onParsed func(parsed, total int)) (TranslationMap, error) {
	chunkJSON, err := ChunkToJSON(chunk)
	if err != nil {
		return nil, fmt.Errorf("marshal chunk: %w", err)
	}

	originalPrompt := fmt.Sprintf("请翻译以下JSON内容 (%d/%d):\n\n%s", idx+1, totalChunks, chunkJSON)
	userPrompt := originalPrompt

	var lastErr error
	maxAttempts := t.cfg.MaxRetries
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var (
			raw string
			err error
		)
		if onParsed != nil {
			deltaCount := 0
			lastReported := 0
			raw, err = t.client.ChatStream(systemPrompt, userPrompt, func(acc string) {
				deltaCount++
				if deltaCount%3 != 0 {
					return
				}
				count := countParsedKeys(acc, chunk)
				if count > lastReported {
					lastReported = count
					onParsed(count, len(chunk))
				}
			})
		} else {
			raw, err = t.client.Chat(systemPrompt, userPrompt)
		}
		if err != nil {
			lastErr = fmt.Errorf("API call: %w", err)
			if err != nil && raw != "" {
				lastErr = fmt.Errorf("API call (partial): %w", err)
			}
			continue
		}

		cleaned := cleanResponse(raw)

		// Structural validation — strip quoted content and compare skeleton.
		// This is lenient: accepts fragments without {} and trailing commas.
		if err := ValidateChunkStructure(chunkJSON, cleaned); err != nil {
			lastErr = fmt.Errorf("structure: %w", err)
			if attempt < t.cfg.MaxRetries {
				userPrompt = t.retryPrompt(originalPrompt, attempt, chunkJSON,
					"你的输出结构有误，请确保返回的JSON片段拥有与输入完全相同数量的键值对。这是分块JSON，请原样返回每个键值对。\n原始JSON:\n%s")
			}
			continue
		}

		translated := make(TranslationMap)
		if err := json.Unmarshal([]byte(cleaned), &translated); err != nil {
			if attempt < t.cfg.MaxRetries {
				userPrompt = t.retryPrompt(originalPrompt, attempt, chunkJSON,
					"你的输出包含非法JSON，请只返回纯JSON片段。这是分块JSON，可能不以{开头或不以}结尾，这是正常的，不要补全。\n请严格按照以下格式返回（只返回JSON片段）：\n{\"key1\": \"value1\", \"key2\": \"value2\"}\n\n原始JSON:\n%s")
			}
			continue
		}

		vRes := ValidateKeys(chunk, translated)
		if len(vRes.MissingKeys) > 0 {
			lastErr = fmt.Errorf("missing %d keys: %v", len(vRes.MissingKeys), vRes.MissingKeys)
			userPrompt = t.retryPrompt(originalPrompt, attempt, chunkJSON,
				"以下键缺失了翻译，请补全并返回完整的JSON片段（不要补全大括号，这是分块JSON）：\n缺失键: %v\n\n原始JSON:\n%s")
			continue
		}

		return translated, nil
	}

	return nil, lastErr
}

func (t *Translator) retryPrompt(originalPrompt string, attempt int, chunkJSON string, correctiveFmt string) string {
	switch t.cfg.RetryMode {
	case RetryModeNewContext:
		return originalPrompt
	case RetryModeCorrect:
		return fmt.Sprintf(correctiveFmt, chunkJSON)
	case RetryModeThreshold:
		if attempt <= t.cfg.RetryThreshold {
			return fmt.Sprintf(correctiveFmt, chunkJSON)
		}
		return originalPrompt
	default:
		return fmt.Sprintf(correctiveFmt, chunkJSON)
	}
}

func countParsedKeys(cleaned string, chunk TranslationMap) int {
	count := 0
	for k := range chunk {
		if strings.Contains(cleaned, `"`+k+`":"`) || strings.Contains(cleaned, `"`+k+`": "`) {
			count++
		}
	}
	return count
}

func cleanResponse(raw string) string {
	raw = strings.TrimSpace(raw)

	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var cleaned []string
		inBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inBlock = !inBlock
				continue
			}
			if !inBlock {
				cleaned = append(cleaned, line)
			} else {
				cleaned = append(cleaned, line)
			}
		}
		raw = strings.Join(cleaned, "\n")
	}

	if idx := strings.Index(raw, "{"); idx >= 0 {
		raw = raw[idx:]
	}
	if idx := strings.LastIndex(raw, "}"); idx >= 0 {
		raw = raw[:idx+1]
	}

	// No braces found — this is a chunk fragment (e.g. middle chunk without {})
	// Remove trailing comma and wrap in {}
	if !strings.HasPrefix(raw, "{") {
		raw = strings.TrimRight(raw, ", \t\r\n")
		raw = "{" + raw + "}"
	}

	raw = strings.TrimSpace(raw)
	return raw
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

func CleanCache() error {
	return os.RemoveAll(CacheRoot)
}

// ShortResultPath returns a short display path for the result.
// In pack mode (path contains /assets/ or \assets\), shows "modid -> code.json".
// Otherwise returns the full path.
func ShortResultPath(r *Result) string {
	if r == nil || r.OutputPath == "" {
		return ""
	}
	p := r.OutputPath
	// Detect pack mode: path contains /assets/ or \assets\
	if strings.Contains(p, "/assets/") || strings.Contains(p, "\\assets\\") {
		return fmt.Sprintf("%s -> %s", r.ModID, filepath.Base(p))
	}
	return p
}
