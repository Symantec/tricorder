package types

func round(f float64) float64 {
	if f < 0 {
		return f - 0.5
	}
	return f + 0.5
}
