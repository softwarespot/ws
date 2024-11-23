package main

// Config defines the configuration settings for the WebSocket handler.
type Config[T any] struct {
	// Message decoder function, which returns a slice of bytes that will then be converted to a string. Default is json.Unmarshal()
	Decoder func([]byte) (T, error)

	// Message encoder function, which returns a slice of bytes that will then be converted to a string. Default is json.Marshal()
	Encoder func([]T) ([]byte, error)
}

// NewConfig initializes a configuration instance with reasonable defaults.
func NewConfig[T any]() *Config[T] {
	cfg := &Config[T]{}

	// Use the default decoder
	cfg.Decoder = nil

	// Use the default encoder
	cfg.Encoder = nil

	return cfg
}
