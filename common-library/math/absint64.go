package math

func AbsInt64(x int64) int64 {
	if x < 0 {
		return -x
	}

	if x == 0 {
		return 0
	}

	return x
}
