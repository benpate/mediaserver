package mediaserver

// round100
func round100(number int) int {

	result := (number / 100)

	if number%100 != 0 {
		result = result + 1
	}

	return result * 100
}

func first[T comparable](values ...T) T {

	var zero T

	for _, value := range values {
		if value != zero {
			return value
		}
	}

	return zero
}
