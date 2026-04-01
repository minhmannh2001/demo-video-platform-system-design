package esclient

// Config configures the Elasticsearch writer client.
type Config struct {
	// Addresses are cluster URLs, e.g. http://localhost:9200
	Addresses []string
	Username  string
	Password  string
	// Index is the target index name (default: videos).
	Index string
	// MaxRetries is passed to the official client (default: 3).
	MaxRetries int
}
