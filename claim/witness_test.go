package claim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateWitness(t *testing.T) {
	w, err := GenerateWitness()
	require.NoError(t, err)

	assert.NotEmpty(t, w.ID)
	assert.NotNil(t, w.PublicKey)
	assert.NotNil(t, w.PrivateKey)
	assert.Len(t, w.PublicKey, 32)
	assert.Len(t, w.PrivateKey, 64)
}

func TestWitnessFromID(t *testing.T) {
	w, err := GenerateWitness()
	require.NoError(t, err)

	w2, err := WitnessFromID(w.ID)
	require.NoError(t, err)

	assert.Equal(t, w.ID, w2.ID)
	assert.Equal(t, w.PublicKey, w2.PublicKey)
	assert.Nil(t, w2.PrivateKey) // Should not have private key
}

func TestAttest(t *testing.T) {
	w, err := GenerateWitness()
	require.NoError(t, err)

	c, err := NewClaim(Statement{Subject: "test"}, nil, "")
	require.NoError(t, err)

	att, err := w.Attest(c)
	require.NoError(t, err)

	assert.Equal(t, w.ID, att.WitnessID)
	assert.NotEmpty(t, att.Signature)
	assert.False(t, att.Timestamp.IsZero())
}

func TestVerifyAttestation(t *testing.T) {
	w, err := GenerateWitness()
	require.NoError(t, err)

	c, err := NewClaim(Statement{Subject: "test"}, nil, "")
	require.NoError(t, err)

	att, err := w.Attest(c)
	require.NoError(t, err)

	t.Run("valid attestation passes", func(t *testing.T) {
		err := VerifyAttestation(c, att)
		assert.NoError(t, err)
	})

	t.Run("tampered signature fails", func(t *testing.T) {
		att.Signature[0] ^= 0xFF
		err := VerifyAttestation(c, att)
		assert.Error(t, err)
	})
}

func TestAddAttestation(t *testing.T) {
	w, err := GenerateWitness()
	require.NoError(t, err)

	c, err := NewClaim(Statement{Subject: "test"}, nil, "")
	require.NoError(t, err)

	att, err := w.Attest(c)
	require.NoError(t, err)

	err = c.AddAttestation(att)
	require.NoError(t, err)

	assert.Len(t, c.Witnesses, 1)

	t.Run("duplicate attestation rejected", func(t *testing.T) {
		att2, _ := w.Attest(c)
		err := c.AddAttestation(att2)
		assert.Error(t, err)
	})
}

func TestVerifyAllAttestations(t *testing.T) {
	w1, _ := GenerateWitness()
	w2, _ := GenerateWitness()

	c, _ := NewClaim(Statement{Subject: "test"}, nil, "")

	att1, _ := w1.Attest(c)
	att2, _ := w2.Attest(c)

	_ = c.AddAttestation(att1)
	_ = c.AddAttestation(att2)

	err := c.VerifyAllAttestations()
	assert.NoError(t, err)
}
