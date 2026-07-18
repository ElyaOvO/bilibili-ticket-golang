package global

// ── Build info ──────────────────────────────────────────
var (
	GitCommit   = "Development"
	BuildTime   = "-1"
	Version     = "0.0.0"
	LoggerLevel = "6"

	// Cloud-control settings are injected at build time with -ldflags -X.
	// Production startup fails closed when any required value is invalid.
	ReportDSN = ""
	// ReportPublicKey is the Ed25519 public key used to verify capabilities
	// signed by the cloud-control service. It is injected at release build time.
	ReportPublicKey    = ""
	ReportTimeout      = "5s"
	ReportSkipSSLCheck = "false"

	// Debug controls verbose debug output on both frontend and backend.
	// When the `debug` build tag is active, this is set to true in debug_on.go.
	Debug = false
)

// ── Default constants shared across packages ────────────

// DefaultIntervalMs is the default polling interval between submit attempts (ms).
const DefaultIntervalMs = 500

// FrontVersion is the bilibili mall frontend version string sent in API requests.
const FrontVersion = "134"

// DefaultTicketExpireDays is how many days from now a ticket expires if no API expiry is available.
const DefaultTicketExpireDays = 30

// MaxTokenRefreshCount is the number of submit attempts before refreshing the order token.
const MaxTokenRefreshCount = 61
