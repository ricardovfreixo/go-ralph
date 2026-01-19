package rlm

import "errors"

var (
	ErrMaxDepthExceeded       = errors.New("maximum recursion depth exceeded")
	ErrFeatureNotFound        = errors.New("feature not found")
	ErrInvalidSpawnData       = errors.New("invalid spawn request data")
	ErrContextBudgetExhausted = errors.New("context budget exhausted")
	ErrParentNotRunning       = errors.New("parent feature is not running")
)
