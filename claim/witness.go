package claim

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Witness represents an entity that can attest to claims
type Witness struct {
	// ID is the hex-encoded public key
	ID string

	// PublicKey is the ed25519 public key
	PublicKey ed25519.PublicKey

	// PrivateKey is the ed25519 private key (only set for local witness)
	PrivateKey ed25519.PrivateKey

	// Metadata contains optional witness information
	Metadata map[string]string
}

// GenerateWitness creates a new witness with a fresh keypair
func GenerateWitness() (*Witness, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	return &Witness{
		ID:         hex.EncodeToString(pub),
		PublicKey:  pub,
		PrivateKey: priv,
		Metadata:   make(map[string]string),
	}, nil
}

// WitnessFromPublicKey creates a witness from an existing public key
func WitnessFromPublicKey(pubKey ed25519.PublicKey) *Witness {
	return &Witness{
		ID:        hex.EncodeToString(pubKey),
		PublicKey: pubKey,
		Metadata:  make(map[string]string),
	}
}

// WitnessFromID creates a witness from a hex-encoded public key ID
func WitnessFromID(id string) (*Witness, error) {
	pubBytes, err := hex.DecodeString(id)
	if err != nil {
		return nil, fmt.Errorf("invalid witness ID: %w", err)
	}

	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: got %d, want %d", len(pubBytes), ed25519.PublicKeySize)
	}

	return &Witness{
		ID:        id,
		PublicKey: ed25519.PublicKey(pubBytes),
		Metadata:  make(map[string]string),
	}, nil
}

// Attest creates an attestation for a claim
func (w *Witness) Attest(claim *Claim) (*Attestation, error) {
	if w.PrivateKey == nil {
		return nil, fmt.Errorf("witness has no private key")
	}

	if claim == nil {
		return nil, fmt.Errorf("claim cannot be nil")
	}

	// Sign the claim ID (which is its content hash)
	signature := ed25519.Sign(w.PrivateKey, []byte(claim.ID))

	return &Attestation{
		WitnessID: w.ID,
		Signature: signature,
		Timestamp: time.Now().UTC(),
	}, nil
}

// VerifyAttestation verifies that an attestation is valid for a claim
func VerifyAttestation(claim *Claim, attestation *Attestation) error {
	if claim == nil {
		return fmt.Errorf("claim cannot be nil")
	}
	if attestation == nil {
		return fmt.Errorf("attestation cannot be nil")
	}

	// Decode witness public key from ID
	pubBytes, err := hex.DecodeString(attestation.WitnessID)
	if err != nil {
		return fmt.Errorf("invalid witness ID: %w", err)
	}

	if len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length")
	}

	pubKey := ed25519.PublicKey(pubBytes)

	// Verify signature over claim ID
	if !ed25519.Verify(pubKey, []byte(claim.ID), attestation.Signature) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// AddAttestation adds a verified attestation to a claim
func (c *Claim) AddAttestation(attestation *Attestation) error {
	if err := VerifyAttestation(c, attestation); err != nil {
		return err
	}

	// Check for duplicate
	for _, existing := range c.Witnesses {
		if existing.WitnessID == attestation.WitnessID {
			return fmt.Errorf("witness %s already attested", attestation.WitnessID)
		}
	}

	c.Witnesses = append(c.Witnesses, *attestation)
	return nil
}

// VerifyAllAttestations verifies all attestations on a claim
func (c *Claim) VerifyAllAttestations() error {
	for i, att := range c.Witnesses {
		if err := VerifyAttestation(c, &att); err != nil {
			return fmt.Errorf("attestation %d invalid: %w", i, err)
		}
	}
	return nil
}
