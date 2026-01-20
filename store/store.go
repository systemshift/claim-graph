// Package store provides storage backends for claims
package store

import (
	"context"

	"github.com/systemshift/claim-graph/claim"
)

// Store is the interface for claim storage backends
type Store interface {
	// Put stores a claim and returns its CID
	Put(ctx context.Context, c *claim.Claim) (string, error)

	// Get retrieves a claim by CID
	Get(ctx context.Context, cid string) (*claim.Claim, error)

	// Has checks if a claim exists
	Has(ctx context.Context, cid string) (bool, error)

	// List returns all claim CIDs (optionally filtered)
	List(ctx context.Context, filter *Filter) ([]string, error)

	// Close closes the store
	Close() error
}

// Filter specifies criteria for listing claims
type Filter struct {
	// Domain filters by statement domain
	Domain string

	// WitnessID filters by claims attested by this witness
	WitnessID string

	// Subject filters by statement subject
	Subject string

	// Limit limits the number of results
	Limit int

	// Offset skips the first N results
	Offset int
}
