package memdb

const (
	// DefaultMemSize the default map size used for storing data.
	DefaultMemSize = 100
)

type config struct {
	memSize int
}

type Option func(*config)

// WithMemSize allows us to specify a custom mem size for store maps
func WithMemSize(memSize int) Option {
	return func(c *config) {
		if memSize >= 0 {
			c.memSize = memSize
		}
	}
}
