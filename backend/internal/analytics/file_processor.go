package analytics

// FileProcessor processes transcript files incrementally.
// ProcessFile is called once per file (main first, then each agent).
//
// The mainFile passed to ProcessFile(file, true) must remain valid
// until after Finalize completes, as some analyzers store a reference
// to it for fallback logic (e.g., scanning toolUseResult blocks).
//
// Finalize is called after all files have been processed. The hasAgentFile
// callback reports whether a given agent ID was processed from an actual
// agent file (as opposed to only being referenced in toolUseResult).
type FileProcessor interface {
	ProcessFile(file *TranscriptFile, isMain bool)
	Finalize(hasAgentFile func(string) bool)
}
