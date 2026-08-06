package delta

// Patch applies a series of delta operations to reconstruct a target file.
// This is a convenience wrapper around ApplyDelta.
func Patch(base []byte, deltaData []byte) ([]byte, error) {
	ops, err := DecodeDelta(deltaData)
	if err != nil {
		return nil, err
	}
	return ApplyDelta(base, ops)
}

// PatchMultiple applies deltas sequentially to a base.
// Each delta is applied to the result of the previous application.
func PatchMultiple(base []byte, deltas [][]byte) ([]byte, error) {
	current := base
	for _, d := range deltas {
		var err error
		current, err = Patch(current, d)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}
