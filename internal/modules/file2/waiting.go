package file2

// WaitingRules uses the same measured host property as launch, retaining only
// the choices actually exposed by the Japanese area. Hidden flags remain in6914.
func WaitingRules(area int, stat uint32, profile []byte) ([]byte, error) {
	native, err := LaunchRules(area, stat, profile)
	if err != nil {
		return nil, err
	}
	values := []byte{profile[233], profile[234], profile[235]}
	for _, name := range JapaneseExtra[area] {
		switch name {
		case "friendly_fire":
			values = append(values, native[4])
		case "nightmare":
			values = append(values, native[7])
		case "infinity":
			values = append(values, native[8])
		}
	}
	return values, nil
}
