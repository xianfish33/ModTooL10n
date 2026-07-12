package engine

import (
	"fmt"
	"path/filepath"
	"sync"
)

type Engine struct {
	provider ProviderConfig
}

func NewEngine(provider ProviderConfig) *Engine {
	return &Engine{provider: provider}
}

func (e *Engine) ValidateProvider() error {
	if e.provider.APIKey == "" {
		return fmt.Errorf("当前选中的提供商 APIKey 为空")
	}
	if e.provider.ModelName == "" {
		return fmt.Errorf("当前提供商没有激活的模型")
	}
	return nil
}

func (e *Engine) ResolveLangCode(input string) string {
	client := NewLLMClient(e.provider.BaseURL, e.provider.APIKey, e.provider.ModelName)
	return ResolveLangCode(client, input)
}

func (e *Engine) StartSingle(path string, targetLang string, targetCode string, outputMode OutputMode, packName string, progress *SingleProgress) {
	jars, err := ScanJARs(path)
	if err != nil {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("扫描jar失败: %v", err)
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}
	if len(jars) == 0 {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("未找到任何jar文件: %s", path)
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}

	jarPath := jars[0]
	jr, err := ExtractJAR(jarPath, CacheRoot)
	if err != nil {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("解压失败: %v", err)
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}
	if jr.ModMeta == nil {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("未找到 mod 元数据")
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}
	modName := jr.ModMeta.Name
	if modName == "" {
		modName = jr.ModMeta.ID
	}
	var enFile *LangFile
	for i := range jr.LangFiles {
		if jr.LangFiles[i].LangCode == "en_us" {
			enFile = &jr.LangFiles[i]
			break
		}
	}
	if enFile == nil {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("未找到 en_us.json")
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}
	data, err := LoadLang(enFile.TempPath)
	if err != nil {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("读取语言文件失败: %v", err)
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}

	totalKeys := len(data)
	code := targetCode
	if code == "" {
		code = targetLang
	}

	totalChunks := (totalKeys + e.provider.MaxKeys - 1) / e.provider.MaxKeys
	chunks := make([]ChunkState, totalChunks)
	for i := range chunks {
		chunks[i] = ChunkState{Status: "pending", KeysCount: 0}
	}

	progress.Mu.Lock()
	progress.Phase = "translating"
	progress.TotalKeys = totalKeys
	progress.TotalChunks = totalChunks
	progress.Chunks = chunks
	progress.Mu.Unlock()

	client := NewLLMClient(e.provider.BaseURL, e.provider.APIKey, e.provider.ModelName)

		go func() {
		tr := NewTranslator(client, TranslatorConfig{
			MaxChunkKeys:   e.provider.MaxKeys,
			MaxRetries:     e.provider.MaxRetries,
			TargetLang:     targetLang,
			TargetCode:     code,
			OutputDir:      OutputRoot,
			OutputMode:     outputMode,
			PackName:       packName,
			RetryMode:      RetryMode(e.provider.RetryMode),
			RetryThreshold: e.provider.RetryThreshold,
			OnProgress: func(info ProgressInfo) {
				progress.Mu.Lock()
				defer progress.Mu.Unlock()
				switch info.Status {
				case "translating":
					if info.ChunkIdx < len(progress.Chunks) {
						cs := progress.Chunks[info.ChunkIdx]
						cs.Status = "translating"
						cs.KeysCount = info.ChunkKeys
						if info.ParsedKeys > cs.ParsedKeys {
							cs.ParsedKeys = info.ParsedKeys
						}
						progress.Chunks[info.ChunkIdx] = cs
					}
				case "retrying":
					if info.ChunkIdx < len(progress.Chunks) {
						cs := progress.Chunks[info.ChunkIdx]
						cs.Status = "retrying"
						cs.KeysCount = info.ChunkKeys
						cs.Retry = info.Retry
						cs.MaxRetries = info.MaxRetries
						progress.Chunks[info.ChunkIdx] = cs
					}
				case "done":
					if info.ChunkIdx < len(progress.Chunks) {
						progress.Chunks[info.ChunkIdx] = ChunkState{Status: "done", KeysCount: info.ChunkKeys, ParsedKeys: info.ChunkKeys}
						progress.DoneKeys += info.ChunkKeys
					}
				case "failed":
					if info.ChunkIdx < len(progress.Chunks) {
						progress.Chunks[info.ChunkIdx] = ChunkState{Status: "failed", KeysCount: info.ChunkKeys}
					}
				case "merging":
					progress.Phase = "merging"
				case "saving":
					progress.Phase = "saving"
				case "completed":
					progress.Phase = "completed"
				}
			},
		})

		result, err := tr.TranslateWithJar(jr.ModMeta.ID, data, modName, jr.ModMeta.Description, enFile.Path, jr.CacheDir)
		if err == nil && result != nil && outputMode == OutputModePack && result.OutputPath != "" {
			packName := SanitizePackName(packName)
			if packName == "" {
				packName = "resourcepack"
			}
			packDir := filepath.Join(OutputRoot, packName)
			zipPath, zipErr := CreateResourcePack(packDir, packName)
			if zipErr == nil {
				result.PackPath = zipPath
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("创建资源包失败: %v", zipErr))
			}
		}
		CleanCache()
		progress.Mu.Lock()
		progress.AllDone = true
		progress.Err = err
		progress.Result = result
		progress.Mu.Unlock()
	}()
}

func (e *Engine) StartBatch(path string, targetLang string, targetCode string, outputMode OutputMode, packName string, progress *BatchProgress) {
	jars, err := ScanJARs(path)
	if err != nil {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("扫描jar失败: %v", err)
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}
	if len(jars) == 0 {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("未找到任何jar文件: %s", path)
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}

	type extractedMod struct {
		jarPath string
		jr      *JarResult
		enFile  *LangFile
		data    TranslationMap
	}

	var extractedMods []extractedMod
	for _, jp := range jars {
		jr, err := ExtractJAR(jp, CacheRoot)
		if err != nil {
			continue
		}
		if jr.ModMeta == nil {
			continue
		}
		var enFile *LangFile
		for i := range jr.LangFiles {
			if jr.LangFiles[i].LangCode == "en_us" {
				enFile = &jr.LangFiles[i]
				break
			}
		}
		if enFile == nil {
			continue
		}
		data, err := LoadLang(enFile.TempPath)
		if err != nil {
			continue
		}
		extractedMods = append(extractedMods, extractedMod{
			jarPath: jp,
			jr:      jr,
			enFile:  enFile,
			data:    data,
		})
	}

	if len(extractedMods) == 0 {
		progress.Mu.Lock()
		progress.Err = fmt.Errorf("未找到任何可翻译的mod (需要en_us.json)")
		progress.AllDone = true
		progress.Mu.Unlock()
		return
	}

	code := targetCode
	if code == "" {
		code = targetLang
	}

	progress.Mu.Lock()
	progress.Mods = make([]*ModProgress, len(extractedMods))
	progress.TotalMods = len(extractedMods)
	progress.Phase = "preparing"
	for i, em := range extractedMods {
		totalKeys := len(em.data)
		totalChunks := (totalKeys + e.provider.MaxKeys - 1) / e.provider.MaxKeys
		chunks := make([]ChunkState, totalChunks)
		for j := range chunks {
			chunks[j] = ChunkState{Status: "pending", KeysCount: 0}
		}
		progress.Mods[i] = &ModProgress{
			ModID:       em.jr.ModMeta.ID,
			ModName:     em.jr.ModMeta.Name,
			Chunks:      chunks,
			TotalKeys:   totalKeys,
			TotalChunks: totalChunks,
			Phase:       "pending",
		}
	}
	progress.Mu.Unlock()

	go func() {
		client := NewLLMClient(e.provider.BaseURL, e.provider.APIKey, e.provider.ModelName)

		progress.Mu.Lock()
		progress.Phase = "translating"
		progress.Mu.Unlock()

		var wg sync.WaitGroup
		sem := make(chan struct{}, MaxConcurrentMods)
		var results []*Result
		var firstErr error

		for idx, em := range extractedMods {
			wg.Add(1)
			sem <- struct{}{}

			go func(modIdx int, em extractedMod) {
				defer wg.Done()
				defer func() { <-sem }()

				progress.Mu.Lock()
				progress.Mods[modIdx].Phase = "translating"
				progress.CurrentMods++
				progress.Mu.Unlock()

				modName := em.jr.ModMeta.Name
				if modName == "" {
					modName = em.jr.ModMeta.ID
				}

				tr := NewTranslator(client, TranslatorConfig{
					MaxChunkKeys:   e.provider.MaxKeys,
					MaxRetries:     e.provider.MaxRetries,
					TargetLang:     targetLang,
					TargetCode:     code,
					OutputDir:      OutputRoot,
					OutputMode:     outputMode,
					PackName:       packName,
					RetryMode:      RetryMode(e.provider.RetryMode),
					RetryThreshold: e.provider.RetryThreshold,
					OnProgress: func(info ProgressInfo) {
						progress.Mu.Lock()
						defer progress.Mu.Unlock()
						mp := progress.Mods[modIdx]
						switch info.Status {
						case "translating":
							if info.ChunkIdx < len(mp.Chunks) {
								cs := mp.Chunks[info.ChunkIdx]
								cs.Status = "translating"
								cs.KeysCount = info.ChunkKeys
								if info.ParsedKeys > cs.ParsedKeys {
									cs.ParsedKeys = info.ParsedKeys
								}
								mp.Chunks[info.ChunkIdx] = cs
							}
						case "retrying":
							if info.ChunkIdx < len(mp.Chunks) {
								cs := mp.Chunks[info.ChunkIdx]
								cs.Status = "retrying"
								cs.KeysCount = info.ChunkKeys
								cs.Retry = info.Retry
								cs.MaxRetries = info.MaxRetries
								mp.Chunks[info.ChunkIdx] = cs
							}
						case "done":
							if info.ChunkIdx < len(mp.Chunks) {
								mp.Chunks[info.ChunkIdx] = ChunkState{Status: "done", KeysCount: info.ChunkKeys, ParsedKeys: info.ChunkKeys}
								mp.DoneKeys += info.ChunkKeys
							}
						case "failed":
							if info.ChunkIdx < len(mp.Chunks) {
								mp.Chunks[info.ChunkIdx] = ChunkState{Status: "failed", KeysCount: info.ChunkKeys}
							}
						case "merging":
							mp.Phase = "merging"
						case "saving":
							mp.Phase = "saving"
						case "completed":
							mp.Phase = "completed"
						}
					},
				})

				result, err := tr.TranslateWithJar(
					em.jr.ModMeta.ID,
					em.data,
					modName,
					em.jr.ModMeta.Description,
					em.enFile.Path,
					em.jr.CacheDir,
				)

				progress.Mu.Lock()
				mp := progress.Mods[modIdx]
				if err != nil {
					mp.Phase = "failed"
					mp.Err = err
					if firstErr == nil {
						firstErr = err
					}
					if result != nil {
						mp.Result = result
						results = append(results, result)
					}
				} else if result != nil && result.Skipped {
					mp.Phase = "completed"
					mp.LangExists = true
					mp.LangMsg = result.SkipMsg
					mp.Result = result
					results = append(results, result)
				} else {
					mp.Phase = "completed"
					mp.Result = result
					results = append(results, result)
				}
				progress.CurrentMods--
				progress.Mu.Unlock()
			}(idx, em)
		}

		wg.Wait()

		if outputMode == OutputModePack && len(results) > 0 {
			packName := SanitizePackName(packName)
			if packName == "" {
				packName = "resourcepack"
			}
			packDir := filepath.Join(OutputRoot, packName)
			zipPath, zipErr := CreateResourcePack(packDir, packName)
			progress.Mu.Lock()
			if zipErr == nil {
				progress.PackPath = zipPath
			} else if len(results) > 0 && results[0] != nil {
				results[0].Errors = append(results[0].Errors, fmt.Sprintf("创建资源包失败: %v", zipErr))
			}
			progress.Mu.Unlock()
		}

		CleanCache()

		progress.Mu.Lock()
		if firstErr != nil {
			progress.Phase = "failed"
		} else {
			progress.Phase = "completed"
		}
		progress.AllDone = true
		progress.Err = firstErr
		progress.Results = results
		progress.Mu.Unlock()
	}()
}
