package signal_test

// floatNear returns true if |a-b| <= tol.
func floatNear(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}
