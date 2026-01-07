package types

// Equal compares two Tier instances for equality
// Required by the gogo protobuf generated Equal method in params.pb.go
func (t *Tier) Equal(other *Tier) bool {
	if t == nil && other == nil {
		return true
	}
	if t == nil || other == nil {
		return false
	}
	return t.Name == other.Name && t.Cutoff.Equal(other.Cutoff)
}
