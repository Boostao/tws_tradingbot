package bot

// Emitter defines the interface for event emission.
type Emitter interface {
	Emit(name string, data any)
}
