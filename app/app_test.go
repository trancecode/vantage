package app

import (
	"testing"
	"time"
)

func TestNewAppHasManager(t *testing.T) {
	a := New(Config{WindowTitle: "test"})
	if a.Manager() == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestRequestExitMakesUpdateReturnErrExit(t *testing.T) {
	a := New(Config{})
	a.RequestExit()
	err := a.Update()
	if err != ErrExit {
		t.Fatalf("expected ErrExit, got %v", err)
	}
}

func TestOnUpdateErrorStopsLoop(t *testing.T) {
	a := New(Config{})
	sentinel := time.Duration(0)
	wantErr := errTest
	a.OnUpdate = func(d time.Duration) error {
		sentinel = d
		return wantErr
	}
	if err := a.Update(); err != wantErr {
		t.Fatalf("expected propagated OnUpdate error, got %v", err)
	}
	_ = sentinel
}

var errTest = &appTestError{}

type appTestError struct{}

func (*appTestError) Error() string { return "test error" }
