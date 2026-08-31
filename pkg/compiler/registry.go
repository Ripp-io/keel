package compiler

import "fmt"

// Registry maps for compilers, writers, and readers.
// All registration happens during init() which is single-threaded.
// Read access during compilation is safe because init() completes
// before any compilation begins. Not safe for concurrent registration
// after program startup — if Keel is used as a library with dynamic
// agent registration, these maps need mutex protection.

var writers = make(map[string]func() Writer)

// RegisterWriter registers a writer for the named agent format.
func RegisterWriter(name string, ctor func() Writer) {
	writers[name] = ctor
}

// GetWriter returns a Writer for the given agent format.
func GetWriter(agent string) (Writer, error) {
	ctor, ok := writers[agent]
	if !ok {
		return nil, fmt.Errorf("no writer registered for agent %q", agent)
	}
	return ctor(), nil
}

var readers = make(map[string]func() NativeReader)

// RegisterReader registers a native reader for the named agent format.
// Call this from init() in each reader file.
func RegisterReader(name string, ctor func() NativeReader) {
	readers[name] = ctor
}

// GetReader returns a NativeReader for the given agent format.
func GetReader(agent string) (NativeReader, error) {
	ctor, ok := readers[agent]
	if !ok {
		return nil, fmt.Errorf("no native reader registered for agent %q", agent)
	}
	return ctor(), nil
}

var profileFuncs = make(map[string]func(Scope) AgentProfile)

// RegisterProfileFunc registers a function that returns the AgentProfile for a
// given scope. Called from each adapter's init().
func RegisterProfileFunc(name string, fn func(Scope) AgentProfile) {
	profileFuncs[name] = fn
}

// GetProfile returns the AgentProfile for an agent at a given scope.
func GetProfile(agent string, scope Scope) (AgentProfile, bool) {
	fn, ok := profileFuncs[agent]
	if !ok {
		return AgentProfile{}, false
	}
	return fn(scope), true
}
