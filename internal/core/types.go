package core

import "time"

const (
	stateVersion = 2
	headerGuard  = "X-RcloneGo-Client"
)

type State struct {
	Version           int                  `json:"version"`
	Appearance        string               `json:"appearance"`
	Defaults          TransferSettings     `json:"defaults"`
	QueuePaused       bool                 `json:"queue_paused"`
	Mounts            []MountConfig        `json:"mounts"`
	OwnedMounts       map[string]int       `json:"owned_mounts,omitempty"`
	MountIdentities   map[string]string    `json:"mount_identities,omitempty"`
	Jobs              []Job                `json:"jobs"`
	Workflows         []WorkflowDefinition `json:"workflows"`
	Runs              []Run                `json:"runs"`
	Attempts          []Attempt            `json:"attempts"`
	Automations       []Automation         `json:"automations,omitempty"`
	ExternalTransfers []ExternalTransfer   `json:"external_transfers,omitempty"`
	Handoff           *LegacyHandoff       `json:"handoff,omitempty"`
}

type ExternalTransfer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RCPort      int    `json:"rc_port"`
	OwnerPID    int    `json:"owner_pid,omitempty"`
	Transfers   int    `json:"transfers,omitempty"`
	Checkers    int    `json:"checkers,omitempty"`
	ChunkSize   string `json:"chunk_size,omitempty"`
	BufferSize  string `json:"buffer_size,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	BlockQueue  bool   `json:"block_queue,omitempty"`
}

type ExternalTransferView struct {
	ExternalTransfer
	Available bool            `json:"available"`
	Stats     *RCStats        `json:"stats,omitempty"`
	Tree      []*TransferNode `json:"tree,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type MountConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Remote      string `json:"remote"`
	Drive       string `json:"drive"`
	VolumeName  string `json:"volume_name"`
	CacheDir    string `json:"cache_dir"`
	CacheMax    string `json:"cache_max"`
	CacheMaxAge string `json:"cache_max_age"`
}

type MountView struct {
	MountConfig
	Running        bool   `json:"running"`
	DriveAvailable bool   `json:"drive_available"`
	PID            int    `json:"pid,omitempty"`
	RCPort         int    `json:"rc_port,omitempty"`
	CommandLine    string `json:"-"`
}

type Job struct {
	Origin               string       `json:"origin,omitempty"`
	DryRun               bool         `json:"dry_run"`
	MaxDelete            int          `json:"max_delete"`
	Attempt              int          `json:"attempt"`
	VerificationMode     string       `json:"verification_mode,omitempty"`
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	IdempotencyKey       string       `json:"idempotency_key,omitempty"`
	Operation            string       `json:"operation"`
	Position             int          `json:"position"`
	WorkflowID           string       `json:"workflow_id,omitempty"`
	WorkflowStep         int          `json:"workflow_step,omitempty"`
	WorkflowSize         int          `json:"workflow_size,omitempty"`
	WorkflowTotalBytes   int64        `json:"workflow_total_bytes,omitempty"`
	WorkflowInitialBytes int64        `json:"workflow_initial_bytes,omitempty"`
	ProofPath            string       `json:"proof_path,omitempty"`
	ProofRoot            string       `json:"proof_root,omitempty"`
	Source               string       `json:"source,omitempty"`
	Destination          string       `json:"destination,omitempty"`
	Includes             []string     `json:"includes,omitempty"`
	Files                []string     `json:"files,omitempty"`
	Excludes             []string     `json:"excludes,omitempty"`
	Immutable            bool         `json:"immutable"`
	ChunkSize            string       `json:"chunk_size,omitempty"`
	BufferSize           string       `json:"buffer_size,omitempty"`
	CPUParallelism       int          `json:"cpu_parallelism,omitempty"`
	NotBefore            *time.Time   `json:"not_before,omitempty"`
	Transfers            int          `json:"transfers"`
	Checkers             int          `json:"checkers"`
	Bandwidth            string       `json:"bandwidth,omitempty"`
	Checksum             bool         `json:"checksum"`
	CreateDirs           bool         `json:"create_empty_dirs"`
	AllowDelete          bool         `json:"allow_delete"`
	Status               string       `json:"status"`
	CreatedAt            time.Time    `json:"created_at"`
	StartedAt            *time.Time   `json:"started_at,omitempty"`
	FinishedAt           *time.Time   `json:"finished_at,omitempty"`
	PID                  int          `json:"pid,omitempty"`
	ProcessIdentity      string       `json:"process_identity,omitempty"`
	RCPort               int          `json:"rc_port,omitempty"`
	Bytes                int64        `json:"bytes"`
	TotalBytes           int64        `json:"total_bytes"`
	Speed                float64      `json:"speed"`
	ExitCode             *int         `json:"exit_code,omitempty"`
	Error                string       `json:"error,omitempty"`
	LogPath              string       `json:"log_path,omitempty"`
	LastUpdatedAt        time.Time    `json:"last_updated_at"`
	ActiveFiles          []RCTransfer `json:"active_files,omitempty"`
	FilesDone            int64        `json:"files_done"`
	FilesTotal           int64        `json:"files_total"`
	ChecksDone           int64        `json:"checks_done"`
	ChecksTotal          int64        `json:"checks_total"`
	CheckingFiles        []string     `json:"checking_files,omitempty"`
	EngineErrors         int64        `json:"engine_errors"`
	EngineListed         int64        `json:"engine_listed"`
	EngineElapsed        float64      `json:"engine_elapsed"`
	EngineETA            float64      `json:"engine_eta"`
	EngineFatalError     bool         `json:"engine_fatal_error"`
	EngineRetryError     bool         `json:"engine_retry_error"`
	EngineLastError      string       `json:"engine_last_error,omitempty"`
}

type JobCreateRequest struct {
	DryRun               bool       `json:"dry_run"`
	MaxDelete            int        `json:"max_delete"`
	PreviewID            string     `json:"preview_id,omitempty"`
	VerificationMode     string     `json:"verification_mode"`
	Name                 string     `json:"name"`
	IdempotencyKey       string     `json:"idempotency_key"`
	Operation            string     `json:"operation"`
	WorkflowID           string     `json:"workflow_id"`
	WorkflowTotalBytes   int64      `json:"workflow_total_bytes"`
	WorkflowInitialBytes int64      `json:"workflow_initial_bytes"`
	ProofPath            string     `json:"proof_path"`
	ProofRoot            string     `json:"proof_root"`
	Source               string     `json:"source"`
	Destination          string     `json:"destination"`
	Includes             []string   `json:"includes"`
	Files                []string   `json:"files"`
	Excludes             []string   `json:"excludes"`
	Immutable            bool       `json:"immutable"`
	ChunkSize            string     `json:"chunk_size"`
	BufferSize           string     `json:"buffer_size"`
	CPUParallelism       *int       `json:"cpu_parallelism,omitempty"`
	NotBefore            *time.Time `json:"not_before"`
	Transfers            int        `json:"transfers"`
	Checkers             int        `json:"checkers"`
	Bandwidth            string     `json:"bandwidth"`
	Checksum             bool       `json:"checksum"`
	CreateDirs           bool       `json:"create_empty_dirs"`
	AllowDelete          bool       `json:"allow_delete"`
}

type RemoteInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

type RCStats struct {
	Checks         int64        `json:"checks"`
	TotalChecks    int64        `json:"totalChecks"`
	Bytes          int64        `json:"bytes"`
	TotalBytes     int64        `json:"totalBytes"`
	Speed          float64      `json:"speed"`
	Errors         int64        `json:"errors"`
	Transfers      int64        `json:"transfers"`
	TotalTransfers int64        `json:"totalTransfers"`
	ElapsedTime    float64      `json:"elapsedTime"`
	ETA            float64      `json:"eta"`
	Listed         int64        `json:"listed"`
	FatalError     bool         `json:"fatalError"`
	RetryError     bool         `json:"retryError"`
	LastError      string       `json:"lastError"`
	Transferring   []RCTransfer `json:"transferring"`
	Checking       []string     `json:"checking"`
}

type RCTransfer struct {
	Name       string  `json:"name"`
	Bytes      int64   `json:"bytes"`
	Size       int64   `json:"size"`
	Percentage float64 `json:"percentage"`
	Speed      float64 `json:"speed"`
	SrcFs      string  `json:"srcFs"`
	DstFs      string  `json:"dstFs"`
}

type ActivityStatus struct {
	Available bool     `json:"available"`
	Stats     *RCStats `json:"stats,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func JobStats(job Job) *RCStats {
	return &RCStats{Bytes: job.Bytes, TotalBytes: job.TotalBytes, Speed: job.Speed,
		Transfers: job.FilesDone, TotalTransfers: job.FilesTotal,
		Checks: job.ChecksDone, TotalChecks: job.ChecksTotal,
		Transferring: job.ActiveFiles, Checking: job.CheckingFiles,
		Errors: job.EngineErrors, Listed: job.EngineListed, ElapsedTime: job.EngineElapsed, ETA: job.EngineETA,
		FatalError: job.EngineFatalError, RetryError: job.EngineRetryError, LastError: job.EngineLastError}
}

func CurrentActivity(state State, runtime RuntimeSnapshot) ActivityStatus {
	for _, status := range []string{"running", "handoff"} {
		for _, job := range state.Jobs {
			if job.Status == status {
				return ActivityStatus{Available: true, Stats: JobStats(job)}
			}
		}
	}
	for _, external := range runtime.External {
		if external.Available {
			return ActivityStatus{Available: true, Stats: external.Stats}
		}
	}
	return ActivityStatus{}
}
