package utils

func InArray[T comparable](needle T, haystack []T) (exist bool) {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
