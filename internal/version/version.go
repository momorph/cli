package version

var (
	// Version information (set via ldflags during build)
	Version   = "dev"
	CommitSHA = "unknown"
	BuildDate = "unknown"
	GoVersion = "unknown"
)
