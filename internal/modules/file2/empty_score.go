package file2

// EmptyPlayerScore is an experimental short6917 envelope: player number,
// followed by a zero short, with NO score words. Original-client acceptance
// must be measured. Never treat this reply as a decoded ranking or result.
func EmptyPlayerScore(number byte) []byte {
	if number < 1 || number > 4 {
		return nil
	}
	return []byte{number, 0, 0}
}
