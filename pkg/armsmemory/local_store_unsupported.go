//go:build mipsle || netbsd || (freebsd && arm)

package armsmemory

import (
	"context"
	"fmt"
)

type LocalStore struct{}

func NewLocalStore(Config) (*LocalStore, error) {
	return nil, fmt.Errorf("arms local driver is unavailable on this platform")
}

func (s *LocalStore) UpsertMethodologies(context.Context, []Methodology) error {
	return fmt.Errorf("arms local driver is unavailable on this platform")
}

func (s *LocalStore) SearchMethodologies(context.Context, string, int, float64) ([]SearchHit, error) {
	return nil, fmt.Errorf("arms local driver is unavailable on this platform")
}

func (s *LocalStore) Status(context.Context) (Status, error) {
	return Status{Enabled: true, Ready: false, Driver: "local", Error: "arms local driver is unavailable on this platform"}, nil
}
