package engine

import (
	"sync"
)

const (
	CacheRoot  = "cache"
	OutputRoot = "output"
)

const MaxConcurrentMods = 3

type OutputMode string

const (
	OutputModeMod  OutputMode = "mod"
	OutputModePack OutputMode = "pack"
)

type RetryMode string

const (
	RetryModeNewContext RetryMode = "new_context"
	RetryModeCorrect    RetryMode = "correct"
	RetryModeThreshold  RetryMode = "threshold"
)

type TranslationMap map[string]string

type LangFile struct {
	Path     string
	LangCode string
	TempPath string
}

type ModMetadata struct {
	ID          string
	Name        string
	Description string
	Loader      string
}

type JarResult struct {
	Path      string
	ModID     string
	CacheDir  string
	LangFiles []LangFile
	ModMeta   *ModMetadata
}

type ChunkState struct {
	Status     string
	KeysCount  int
	ParsedKeys int
	Retry      int
	MaxRetries int
}

type Result struct {
	ModID       string
	OutputPath  string
	PackPath    string
	TotalKeys   int
	ChunksTotal int
	ChunksOK    int
	ChunksFail  int
	Skipped     bool
	SkipMsg     string
	Errors      []string
}

type ModProgress struct {
	Mu          sync.Mutex
	ModID       string
	ModName     string
	Chunks      []ChunkState
	TotalKeys   int
	TotalChunks int
	DoneKeys    int
	Phase       string
	LangExists  bool
	LangMsg     string
	Err         error
	Result      *Result
}

type SingleProgress struct {
	Mu          sync.Mutex
	Phase       string
	Chunks      []ChunkState
	TotalKeys   int
	TotalChunks int
	DoneKeys    int
	Err         error
	Result      *Result
	AllDone     bool
}

type BatchProgress struct {
	Mu           sync.Mutex
	Mods         []*ModProgress
	TotalMods    int
	DoneMods     int
	CurrentMods  int
	Phase        string
	AllDone      bool
	Err          error
	Results      []*Result
	ScrollOffset int
	PackPath     string
}

type ProviderConfig struct {
	BaseURL        string
	APIKey         string
	ModelName      string
	MaxKeys        int
	MaxRetries     int
	RetryMode      string
	RetryThreshold int
}

type ValidationResult struct {
	IsValidJSON bool
	MissingKeys []string
	ExtraKeys   []string
	HasChanges  bool
}
