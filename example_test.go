package claimgraph_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/systemshift/claim-graph/claim"
)

func TestEndToEndWithoutIPFS(t *testing.T) {
	// 1. Create two witnesses
	witness1, err := claim.GenerateWitness()
	if err != nil {
		t.Fatalf("Failed to create witness1: %v", err)
	}
	fmt.Printf("Witness 1: %s\n", witness1.ID[:16]+"...")

	witness2, err := claim.GenerateWitness()
	if err != nil {
		t.Fatalf("Failed to create witness2: %v", err)
	}
	fmt.Printf("Witness 2: %s\n", witness2.ID[:16]+"...")

	// 2. Create a claim
	statement := claim.Statement{
		Subject:   "https://example.com/sports/match-123",
		Predicate: "result",
		Object:    "Manchester United won 2-1",
		Domain:    "sports",
	}

	c, err := claim.NewClaim(statement, []string{"evidence-cid-1", "evidence-cid-2"}, "dag-time-event-456")
	if err != nil {
		t.Fatalf("Failed to create claim: %v", err)
	}
	fmt.Printf("Claim CID: %s\n", c.ID)
	fmt.Printf("Statement: %s %s %s\n", c.Statement.Subject, c.Statement.Predicate, c.Statement.Object)

	// 3. Both witnesses attest to the claim
	att1, err := witness1.Attest(c)
	if err != nil {
		t.Fatalf("Witness1 failed to attest: %v", err)
	}
	if err := c.AddAttestation(att1); err != nil {
		t.Fatalf("Failed to add attestation1: %v", err)
	}
	fmt.Printf("Witness 1 attested at %s\n", att1.Timestamp.Format(time.RFC3339))

	att2, err := witness2.Attest(c)
	if err != nil {
		t.Fatalf("Witness2 failed to attest: %v", err)
	}
	if err := c.AddAttestation(att2); err != nil {
		t.Fatalf("Failed to add attestation2: %v", err)
	}
	fmt.Printf("Witness 2 attested at %s\n", att2.Timestamp.Format(time.RFC3339))

	// 4. Verify all attestations
	if err := c.VerifyAllAttestations(); err != nil {
		t.Fatalf("Attestation verification failed: %v", err)
	}
	fmt.Printf("All %d attestations verified\n", len(c.Witnesses))

	// 5. Set up reputation tracking
	repStore := claim.NewReputationStore()

	// Record that both witnesses attested
	repStore.RecordAttestation(witness1.ID, "sports")
	repStore.RecordAttestation(witness2.ID, "sports")

	// Simulate: both agreed with consensus (claim was correct)
	repStore.RecordAgreement(witness1.ID, "sports")
	repStore.RecordAgreement(witness2.ID, "sports")

	// Check reputation
	rep1, _ := repStore.GetRecord(witness1.ID)
	rep2, _ := repStore.GetRecord(witness2.ID)

	fmt.Printf("Witness 1 reputation: %.2f (claims: %d, agreed: %d)\n",
		rep1.Score(), rep1.TotalClaims, rep1.AgreedClaims)
	fmt.Printf("Witness 2 reputation: %.2f (claims: %d, agreed: %d)\n",
		rep2.Score(), rep2.TotalClaims, rep2.AgreedClaims)

	// 6. Compute claim confidence
	confidence := claim.ClaimConfidence(c, repStore)
	fmt.Printf("Claim confidence: %.2f\n", confidence)

	// 7. Verify CID hasn't changed (integrity)
	if err := claim.VerifyCID(c); err != nil {
		t.Fatalf("CID verification failed: %v", err)
	}
	fmt.Println("CID integrity verified")

	// 8. Export reputation for portability
	exported := rep1.Export()
	fmt.Printf("Exported reputation: %+v\n", exported)
}
