package anthropic

// ptrFloat64 restituisce un puntatore a float64
func ptrFloat64(f float64) *float64 {
	return &f
}

// ptrInt restituisce un puntatore a int
func ptrInt(i int) *int {
	return &i
}

// ptrString restituisce un puntatore a string
func ptrString(s string) *string {
	return &s
}

// ptrBool restituisce un puntatore a bool
func ptrBool(b bool) *bool {
	return &b
}
