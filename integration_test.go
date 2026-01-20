package claimgraph_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systemshift/claim-graph/claim"
	"github.com/systemshift/claim-graph/store"
	"github.com/systemshift/dag-time/dag"
)

// TestFullStackIntegration tests the complete flow:
// 1. Create dag-time events with beacon anchors
// 2. Create claims referencing those events
// 3. Multiple witnesses attest
// 4. Store in IPFS
// 5. Verify the complete chain
func TestFullStackIntegration(t *testing.T) {
	ctx := context.Background()

	// Check if IPFS is available
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post("http://localhost:5001/api/v0/id", "", nil)
	if err != nil {
		t.Skip("IPFS not available, skipping integration test")
	}
	resp.Body.Close()

	t.Run("dag-time event creation", func(t *testing.T) {
		// Create a memory DAG for testing
		d := dag.NewMemoryDAG()

		// Create first event (genesis)
		event1 := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("genesis event"),
			Parents: []string{},
			Beacon: &dag.BeaconAnchor{
				Round:      1000,
				Randomness: []byte("test-randomness-1"),
			},
		}
		event1.ID, err = dag.ComputeCID(event1)
		require.NoError(t, err)

		err = d.AddEvent(ctx, event1)
		require.NoError(t, err)

		// Create second event referencing first
		event2 := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("second event"),
			Parents: []string{event1.ID},
			Beacon: &dag.BeaconAnchor{
				Round:      1001,
				Randomness: []byte("test-randomness-2"),
			},
		}
		event2.ID, err = dag.ComputeCID(event2)
		require.NoError(t, err)

		err = d.AddEvent(ctx, event2)
		require.NoError(t, err)

		// Verify DAG integrity
		err = d.Verify(ctx)
		require.NoError(t, err)

		// Get events back
		retrieved1, err := d.GetEvent(ctx, event1.ID)
		require.NoError(t, err)
		assert.Equal(t, event1.ID, retrieved1.ID)
		assert.Equal(t, uint64(1000), retrieved1.Beacon.Round)

		retrieved2, err := d.GetEvent(ctx, event2.ID)
		require.NoError(t, err)
		assert.Equal(t, event2.ID, retrieved2.ID)
		assert.Contains(t, retrieved2.Parents, event1.ID)
	})

	t.Run("claim creation with time event reference", func(t *testing.T) {
		// Create a dag-time event first
		d := dag.NewMemoryDAG()
		event := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("claim anchor event"),
			Parents: []string{},
			Beacon: &dag.BeaconAnchor{
				Round:      2000,
				Randomness: []byte("beacon-randomness"),
			},
		}
		event.ID, err = dag.ComputeCID(event)
		require.NoError(t, err)
		err = d.AddEvent(ctx, event)
		require.NoError(t, err)

		// Create a claim referencing this event
		c, err := claim.NewClaim(claim.Statement{
			Subject:   "https://example.com/article",
			Predicate: "contains",
			Object:    "factual information",
			Domain:    "news",
		}, []string{"evidence-1", "evidence-2"}, event.ID)
		require.NoError(t, err)

		// Verify claim has the time event reference
		assert.Equal(t, event.ID, c.TimeEvent)
		assert.NotEmpty(t, c.ID)

		// Time event becomes part of claim's identity
		originalCID := c.ID

		// Verify CID changes if we change time event
		c2, err := claim.NewClaim(claim.Statement{
			Subject:   "https://example.com/article",
			Predicate: "contains",
			Object:    "factual information",
			Domain:    "news",
		}, []string{"evidence-1", "evidence-2"}, "different-event-id")
		require.NoError(t, err)

		assert.NotEqual(t, originalCID, c2.ID, "Different time events should produce different CIDs")
	})

	t.Run("multi-witness attestation flow", func(t *testing.T) {
		// Create dag-time event
		d := dag.NewMemoryDAG()
		event := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("multi-witness event"),
			Parents: []string{},
			Beacon: &dag.BeaconAnchor{
				Round:      3000,
				Randomness: []byte("multi-witness-beacon"),
			},
		}
		event.ID, err = dag.ComputeCID(event)
		require.NoError(t, err)
		err = d.AddEvent(ctx, event)
		require.NoError(t, err)

		// Create claim
		c, err := claim.NewClaim(claim.Statement{
			Subject:   "https://trusted-source.com/report",
			Predicate: "verified",
			Object:    "accurate",
			Domain:    "fact-check",
		}, nil, event.ID)
		require.NoError(t, err)

		// Create multiple witnesses
		witness1, err := claim.GenerateWitness()
		require.NoError(t, err)

		witness2, err := claim.GenerateWitness()
		require.NoError(t, err)

		witness3, err := claim.GenerateWitness()
		require.NoError(t, err)

		// Each witness attests to the claim
		att1, err := witness1.Attest(c)
		require.NoError(t, err)
		err = c.AddAttestation(att1)
		require.NoError(t, err)

		att2, err := witness2.Attest(c)
		require.NoError(t, err)
		err = c.AddAttestation(att2)
		require.NoError(t, err)

		att3, err := witness3.Attest(c)
		require.NoError(t, err)
		err = c.AddAttestation(att3)
		require.NoError(t, err)

		// Verify all attestations
		assert.Len(t, c.Witnesses, 3)
		err = c.VerifyAllAttestations()
		require.NoError(t, err)

		// Verify each witness ID is unique
		witnessIDs := make(map[string]bool)
		for _, w := range c.Witnesses {
			assert.False(t, witnessIDs[w.WitnessID], "Duplicate witness ID found")
			witnessIDs[w.WitnessID] = true
		}
	})

	t.Run("IPFS storage integration", func(t *testing.T) {
		// Create IPFS-backed dag
		ipfsDAG, err := dag.NewIPFSDAG(dag.IPFSConfig{})
		require.NoError(t, err)

		// Create event and store in IPFS
		event := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("ipfs-stored event"),
			Parents: []string{},
			Beacon: &dag.BeaconAnchor{
				Round:      4000,
				Randomness: []byte("ipfs-beacon"),
			},
		}
		event.ID, err = dag.ComputeCID(event)
		require.NoError(t, err)

		err = ipfsDAG.AddEvent(ctx, event)
		require.NoError(t, err)

		// Create IPFS store for claims
		claimStore, err := store.NewIPFSStore(store.IPFSConfig{})
		require.NoError(t, err)

		// Create claim referencing IPFS-stored event
		c, err := claim.NewClaim(claim.Statement{
			Subject:   "ipfs://stored-content",
			Predicate: "references",
			Object:    "decentralized data",
			Domain:    "ipfs",
		}, nil, event.ID)
		require.NoError(t, err)

		// Add witness
		witness, err := claim.GenerateWitness()
		require.NoError(t, err)
		att, err := witness.Attest(c)
		require.NoError(t, err)
		err = c.AddAttestation(att)
		require.NoError(t, err)

		// Store claim in IPFS
		cid, err := claimStore.Put(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, cid)

		// Retrieve claim
		retrieved, err := claimStore.Get(ctx, cid)
		require.NoError(t, err)
		assert.Equal(t, c.ID, retrieved.ID)
		assert.Equal(t, event.ID, retrieved.TimeEvent)
		assert.Len(t, retrieved.Witnesses, 1)

		// Verify attestation still valid after retrieval
		err = claim.VerifyAttestation(retrieved, &retrieved.Witnesses[0])
		require.NoError(t, err)
	})

	t.Run("full chain verification", func(t *testing.T) {
		// This test verifies the complete chain:
		// dag-time event -> claim -> witness attestations -> IPFS storage

		// 1. Create dag-time DAG with IPFS
		ipfsDAG, err := dag.NewIPFSDAG(dag.IPFSConfig{})
		require.NoError(t, err)

		// 2. Create sequence of events
		event1 := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("chain-event-1"),
			Parents: []string{},
			Beacon: &dag.BeaconAnchor{
				Round:      5000,
				Randomness: []byte("chain-beacon-1"),
			},
		}
		event1.ID, err = dag.ComputeCID(event1)
		require.NoError(t, err)
		err = ipfsDAG.AddEvent(ctx, event1)
		require.NoError(t, err)

		event2 := &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte("chain-event-2"),
			Parents: []string{event1.ID},
			Beacon: &dag.BeaconAnchor{
				Round:      5001,
				Randomness: []byte("chain-beacon-2"),
			},
		}
		event2.ID, err = dag.ComputeCID(event2)
		require.NoError(t, err)
		err = ipfsDAG.AddEvent(ctx, event2)
		require.NoError(t, err)

		// 3. Create claim store
		claimStore, err := store.NewIPFSStore(store.IPFSConfig{})
		require.NoError(t, err)

		// 4. Create claims for each event
		claim1, err := claim.NewClaim(claim.Statement{
			Subject:   "event-1-subject",
			Predicate: "describes",
			Object:    "first event",
			Domain:    "chain",
		}, nil, event1.ID)
		require.NoError(t, err)

		claim2, err := claim.NewClaim(claim.Statement{
			Subject:   "event-2-subject",
			Predicate: "follows",
			Object:    claim1.ID, // Reference to first claim
			Domain:    "chain",
		}, []string{claim1.ID}, event2.ID)
		require.NoError(t, err)

		// 5. Multiple witnesses attest to both claims
		witnesses := make([]*claim.Witness, 3)
		for i := 0; i < 3; i++ {
			witnesses[i], err = claim.GenerateWitness()
			require.NoError(t, err)
		}

		for _, w := range witnesses {
			att1, err := w.Attest(claim1)
			require.NoError(t, err)
			err = claim1.AddAttestation(att1)
			require.NoError(t, err)

			att2, err := w.Attest(claim2)
			require.NoError(t, err)
			err = claim2.AddAttestation(att2)
			require.NoError(t, err)
		}

		// 6. Store both claims
		cid1, err := claimStore.Put(ctx, claim1)
		require.NoError(t, err)

		cid2, err := claimStore.Put(ctx, claim2)
		require.NoError(t, err)

		// 7. Verify the complete chain
		// Retrieve claim2
		retrievedClaim2, err := claimStore.Get(ctx, cid2)
		require.NoError(t, err)

		// Verify it references claim1
		assert.Contains(t, retrievedClaim2.Evidence, cid1)

		// Retrieve the time event for claim2
		retrievedEvent2, err := ipfsDAG.GetEvent(ctx, retrievedClaim2.TimeEvent)
		require.NoError(t, err)

		// Verify event2 references event1
		assert.Contains(t, retrievedEvent2.Parents, event1.ID)

		// Verify beacon ordering
		retrievedEvent1, err := ipfsDAG.GetEvent(ctx, event1.ID)
		require.NoError(t, err)
		assert.Less(t, retrievedEvent1.Beacon.Round, retrievedEvent2.Beacon.Round)

		// Verify all attestations
		err = retrievedClaim2.VerifyAllAttestations()
		require.NoError(t, err)

		t.Logf("Full chain verified:")
		t.Logf("  Event 1: %s (beacon round %d)", event1.ID[:16]+"...", event1.Beacon.Round)
		t.Logf("  Event 2: %s (beacon round %d)", event2.ID[:16]+"...", event2.Beacon.Round)
		t.Logf("  Claim 1: %s", cid1[:16]+"...")
		t.Logf("  Claim 2: %s (references claim 1)", cid2[:16]+"...")
		t.Logf("  Witnesses: %d attestations on each claim", len(witnesses))
	})
}

// TestTemporalOrdering verifies that claims can be ordered by their dag-time events
func TestTemporalOrdering(t *testing.T) {
	ctx := context.Background()

	// Check IPFS availability
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post("http://localhost:5001/api/v0/id", "", nil)
	if err != nil {
		t.Skip("IPFS not available")
	}
	resp.Body.Close()

	d := dag.NewMemoryDAG()

	// Create events with increasing beacon rounds
	events := make([]*dag.Event, 5)
	for i := 0; i < 5; i++ {
		var parents []string
		if i > 0 {
			parents = []string{events[i-1].ID}
		}
		events[i] = &dag.Event{
			Type:    dag.MainEvent,
			Data:    []byte(fmt.Sprintf("event-%d", i)),
			Parents: parents,
			Beacon: &dag.BeaconAnchor{
				Round:      uint64(1000 + i),
				Randomness: []byte(fmt.Sprintf("randomness-%d", i)),
			},
		}
		events[i].ID, err = dag.ComputeCID(events[i])
		require.NoError(t, err)
		err = d.AddEvent(ctx, events[i])
		require.NoError(t, err)
	}

	// Create claims for each event
	claims := make([]*claim.Claim, 5)
	for i := 0; i < 5; i++ {
		claims[i], err = claim.NewClaim(claim.Statement{
			Subject:   fmt.Sprintf("subject-%d", i),
			Predicate: "at-time",
			Object:    fmt.Sprintf("value-%d", i),
			Domain:    "temporal",
		}, nil, events[i].ID)
		require.NoError(t, err)
	}

	// Verify temporal ordering by comparing beacon rounds
	for i := 0; i < 4; i++ {
		event1, _ := d.GetEvent(ctx, claims[i].TimeEvent)
		event2, _ := d.GetEvent(ctx, claims[i+1].TimeEvent)

		assert.Less(t, event1.Beacon.Round, event2.Beacon.Round,
			"Claims should be temporally ordered by their dag-time events")
	}
}
