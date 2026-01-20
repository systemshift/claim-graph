package store

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systemshift/claim-graph/claim"
)

func ipfsAvailable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post("http://localhost:5001/api/v0/id", "", nil)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func TestIPFSStore(t *testing.T) {
	if !ipfsAvailable() {
		t.Skip("IPFS not available, skipping IPFS tests")
	}

	t.Run("NewIPFSStore", func(t *testing.T) {
		s, err := NewIPFSStore(IPFSConfig{})
		require.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("Put and Get", func(t *testing.T) {
		s, err := NewIPFSStore(IPFSConfig{})
		require.NoError(t, err)

		c, err := claim.NewClaim(claim.Statement{
			Subject:   "https://example.com",
			Predicate: "contains",
			Object:    "test content",
			Domain:    "web",
		}, []string{"evidence1"}, "time-event-1")
		require.NoError(t, err)

		ctx := context.Background()

		// Store
		cid, err := s.Put(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, cid)
		assert.Equal(t, c.ID, cid)

		// Retrieve
		retrieved, err := s.Get(ctx, cid)
		require.NoError(t, err)
		assert.Equal(t, c.ID, retrieved.ID)
		assert.Equal(t, c.Statement.Subject, retrieved.Statement.Subject)
		assert.Equal(t, c.Statement.Predicate, retrieved.Statement.Predicate)
		assert.Equal(t, c.Statement.Object, retrieved.Statement.Object)
		assert.Equal(t, c.Evidence, retrieved.Evidence)
	})

	t.Run("Put with attestations", func(t *testing.T) {
		s, err := NewIPFSStore(IPFSConfig{})
		require.NoError(t, err)

		// Create claim
		c, err := claim.NewClaim(claim.Statement{
			Subject:   "https://example.com/attested",
			Predicate: "result",
			Object:    "value",
			Domain:    "test",
		}, nil, "")
		require.NoError(t, err)

		// Add attestation
		witness, err := claim.GenerateWitness()
		require.NoError(t, err)

		att, err := witness.Attest(c)
		require.NoError(t, err)

		err = c.AddAttestation(att)
		require.NoError(t, err)

		ctx := context.Background()

		// Store
		cid, err := s.Put(ctx, c)
		require.NoError(t, err)

		// Retrieve and verify attestation
		retrieved, err := s.Get(ctx, cid)
		require.NoError(t, err)

		assert.Len(t, retrieved.Witnesses, 1)
		assert.Equal(t, witness.ID, retrieved.Witnesses[0].WitnessID)

		// Verify attestation is still valid
		err = claim.VerifyAttestation(retrieved, &retrieved.Witnesses[0])
		assert.NoError(t, err)
	})

	t.Run("Has", func(t *testing.T) {
		s, err := NewIPFSStore(IPFSConfig{})
		require.NoError(t, err)

		c, _ := claim.NewClaim(claim.Statement{Subject: "test-has"}, nil, "")
		ctx := context.Background()

		// Before storing
		exists, err := s.Has(ctx, c.ID)
		require.NoError(t, err)
		assert.False(t, exists)

		// Store
		_, err = s.Put(ctx, c)
		require.NoError(t, err)

		// After storing
		exists, err = s.Has(ctx, c.ID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("List with filters", func(t *testing.T) {
		s, err := NewIPFSStore(IPFSConfig{})
		require.NoError(t, err)

		ctx := context.Background()

		// Create claims in different domains
		c1, _ := claim.NewClaim(claim.Statement{
			Subject: "sports-1",
			Domain:  "sports",
		}, nil, "")
		c2, _ := claim.NewClaim(claim.Statement{
			Subject: "sports-2",
			Domain:  "sports",
		}, nil, "")
		c3, _ := claim.NewClaim(claim.Statement{
			Subject: "finance-1",
			Domain:  "finance",
		}, nil, "")

		_, _ = s.Put(ctx, c1)
		_, _ = s.Put(ctx, c2)
		_, _ = s.Put(ctx, c3)

		// List all
		all, err := s.List(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, all, 3)

		// Filter by domain
		sports, err := s.List(ctx, &Filter{Domain: "sports"})
		require.NoError(t, err)
		assert.Len(t, sports, 2)

		finance, err := s.List(ctx, &Filter{Domain: "finance"})
		require.NoError(t, err)
		assert.Len(t, finance, 1)
	})
}

func TestIPFSStoreConnectionFailure(t *testing.T) {
	_, err := NewIPFSStore(IPFSConfig{
		APIURL: "http://localhost:59999",
	})
	assert.Error(t, err)
}
