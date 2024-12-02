package socket

import (
	"errors"
	"fmt"
)

func ArgAt[T any](args []any, idx int) (T, error) {
	arg, err := argAt(args, idx)
	if err != nil {
		var v T
		return v, err
	}

	v, ok := arg.(T)
	if !ok {
		var v T
		return v, errors.New("socket: invalid arg type")
	}
	return v, nil
}

func argAt[S []E, E comparable](s S, idx int) (E, error) {
	if idx < 0 {
		idx += len(s)
	}
	if idx < 0 || idx >= len(s) {
		var v E
		return v, fmt.Errorf("socket: index of %d out of range", idx)
	}
	return s[idx], nil
}

func argDeleteLast[T any](s []T) []T {
	if len(s) == 0 {
		return s
	}
	return s[:len(s)-1]
}

func GetAckFunc(args []any) (func(...any), bool) {
	ackFn, err := ArgAt[func(...any)](args, -1)
	return ackFn, err == nil
}
