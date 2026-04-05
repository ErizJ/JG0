package bytesconv

// StringToBytes converts a string to a byte slice without memory allocation.
// Note: the returned slice must not be modified.
func StringToBytes(s string) []byte {
	return []byte(s)
}
