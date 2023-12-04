package flags

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func Add[T Integer](f T, bit T) T {
	return f | bit
}

func Remove[T Integer](f T, bit T) T {
	return f &^ bit
}

func Has[T Integer](f T, bit T) bool {
	return (f & bit) == bit
}

func Misses[T Integer](f T, bit T) bool {
	return (f & bit) != bit
}
