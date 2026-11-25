package file2

// The original File2 reply observed after four600e probes has a 33-byte
// opaque version field and its own client packet counter, not the query ID.
// The canonical empty/echo form remains for the existing reference transport.
func versionReply(query, reply uint16, payload []byte) bool {
	return (query == reply && len(payload) == 0) || len(payload) == 33
}
