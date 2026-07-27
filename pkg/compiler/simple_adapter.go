package compiler

// RegisterSimpleAdapter registers both a SimpleReader and a SimpleWriter for
// a lightweight agent adapter. This eliminates the boilerplate of separate
// reader.go and writer.go files in adapters that use SimpleReader/SimpleWriter
// without customization.
func RegisterSimpleAdapter(name string, profileFn func() AgentProfile) {
	RegisterReader(name, func() NativeReader {
		return &SimpleReader{
			Agent:      name,
			Candidates: profileFn().InstructionCandidates,
		}
	})
	RegisterWriter(name, func() Writer {
		return &SimpleWriter{}
	})
}
