package flags

import (
	"golang.org/x/exp/constraints"
)

func Add[T constraints.Integer](f T, bit T) T {
	return f | bit
}

func Remove[T constraints.Integer](f T, bit T) T {
	return f &^ bit
}

func Has[T constraints.Integer](f T, bit T) bool {
	return (f & bit) == bit
}

func Misses[T constraints.Integer](f T, bit T) bool {
	return (f & bit) != bit
}
