// Package claim provides types and utilities for claims in the claim graph.
package claim

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

// Claim represents a statement with evidence and attestations.
// Claims are content-addressed and can reference dag-time events for ordering.
type Claim struct {
	// ID is the content-addressed identifier (CID) of this claim
	ID string

	// Statement is the claim being made
	Statement Statement

	// Evidence is a list of CIDs pointing to supporting data
	Evidence []string

	// TimeEvent is the dag-time event ID that anchors this claim in time
	TimeEvent string

	// Witnesses contains attestations from witnesses
	Witnesses []Attestation

	// Created is when the claim was first created
	Created time.Time

	// Metadata contains optional additional data
	Metadata map[string]string
}

// Statement represents what is being claimed
type Statement struct {
	// Subject is what the claim is about (e.g., URL, entity ID, event ID)
	Subject string

	// Predicate is the relationship or property (e.g., "contains", "occurred", "equals")
	Predicate string

	// Object is the value or target of the claim
	Object string

	// Domain categorizes the claim (e.g., "web", "sports", "finance")
	Domain string
}

// Attestation represents a witness signature on a claim
type Attestation struct {
	// WitnessID is the public key or DID of the witness
	WitnessID string

	// Signature is the witness's signature over the claim
	Signature []byte

	// Timestamp is when the attestation was made
	Timestamp time.Time
}

// ComputeCID computes the content-addressed identifier for a claim.
// The CID is computed from immutable content only:
// - Statement
// - Evidence (sorted)
// - TimeEvent
// - Created timestamp
//
// Witnesses/attestations are NOT included as they are added after creation.
func ComputeCID(claim *Claim) (string, error) {
	if claim == nil {
		return "", fmt.Errorf("claim cannot be nil")
	}

	data, err := serializeClaimContent(claim)
	if err != nil {
		return "", fmt.Errorf("failed to serialize claim: %w", err)
	}

	mh, err := multihash.Sum(data, multihash.SHA2_256, -1)
	if err != nil {
		return "", fmt.Errorf("failed to create multihash: %w", err)
	}

	c := cid.NewCidV1(cid.Raw, mh)
	return c.String(), nil
}

// serializeClaimContent creates a deterministic byte representation
func serializeClaimContent(claim *Claim) ([]byte, error) {
	var buf bytes.Buffer

	// Write statement
	if err := writeString(&buf, claim.Statement.Subject); err != nil {
		return nil, err
	}
	if err := writeString(&buf, claim.Statement.Predicate); err != nil {
		return nil, err
	}
	if err := writeString(&buf, claim.Statement.Object); err != nil {
		return nil, err
	}
	if err := writeString(&buf, claim.Statement.Domain); err != nil {
		return nil, err
	}

	// Write evidence (sorted for determinism)
	sortedEvidence := make([]string, len(claim.Evidence))
	copy(sortedEvidence, claim.Evidence)
	sort.Strings(sortedEvidence)

	if err := binary.Write(&buf, binary.BigEndian, uint32(len(sortedEvidence))); err != nil {
		return nil, err
	}
	for _, e := range sortedEvidence {
		if err := writeString(&buf, e); err != nil {
			return nil, err
		}
	}

	// Write time event reference
	if err := writeString(&buf, claim.TimeEvent); err != nil {
		return nil, err
	}

	// Write created timestamp (Unix nano for precision)
	if err := binary.Write(&buf, binary.BigEndian, claim.Created.UnixNano()); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func writeString(buf *bytes.Buffer, s string) error {
	if err := binary.Write(buf, binary.BigEndian, uint32(len(s))); err != nil {
		return err
	}
	if _, err := buf.WriteString(s); err != nil {
		return err
	}
	return nil
}

// VerifyCID checks if a claim's ID matches its computed CID
func VerifyCID(claim *Claim) error {
	if claim == nil {
		return fmt.Errorf("claim cannot be nil")
	}

	computed, err := ComputeCID(claim)
	if err != nil {
		return fmt.Errorf("failed to compute CID: %w", err)
	}

	if claim.ID != computed {
		return fmt.Errorf("CID mismatch: expected %s, got %s", computed, claim.ID)
	}

	return nil
}

// NewClaim creates a new claim with computed CID
func NewClaim(statement Statement, evidence []string, timeEvent string) (*Claim, error) {
	claim := &Claim{
		Statement: statement,
		Evidence:  evidence,
		TimeEvent: timeEvent,
		Created:   time.Now().UTC(),
		Witnesses: []Attestation{},
		Metadata:  make(map[string]string),
	}

	id, err := ComputeCID(claim)
	if err != nil {
		return nil, err
	}
	claim.ID = id

	return claim, nil
}
