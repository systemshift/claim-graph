package claim

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ReputationStore tracks witness reputation over time
type ReputationStore struct {
	mu      sync.RWMutex
	records map[string]*ReputationRecord
}

// ReputationRecord tracks a single witness's reputation
type ReputationRecord struct {
	// WitnessID is the witness identifier
	WitnessID string

	// TotalClaims is the number of claims this witness has attested to
	TotalClaims int64

	// AgreedClaims is claims where this witness agreed with consensus
	AgreedClaims int64

	// DisputedClaims is claims where this witness was disputed
	DisputedClaims int64

	// Domains tracks reputation per domain
	Domains map[string]*DomainReputation

	// FirstSeen is when this witness was first observed
	FirstSeen time.Time

	// LastSeen is when this witness was last observed
	LastSeen time.Time
}

// DomainReputation tracks reputation in a specific domain
type DomainReputation struct {
	Domain         string
	TotalClaims    int64
	AgreedClaims   int64
	DisputedClaims int64
}

// NewReputationStore creates a new reputation store
func NewReputationStore() *ReputationStore {
	return &ReputationStore{
		records: make(map[string]*ReputationRecord),
	}
}

// GetRecord returns the reputation record for a witness
func (rs *ReputationStore) GetRecord(witnessID string) (*ReputationRecord, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	record, exists := rs.records[witnessID]
	if !exists {
		return nil, false
	}

	// Return a copy
	copy := *record
	copy.Domains = make(map[string]*DomainReputation)
	for k, v := range record.Domains {
		domainCopy := *v
		copy.Domains[k] = &domainCopy
	}
	return &copy, true
}

// RecordAttestation records that a witness attested to a claim
func (rs *ReputationStore) RecordAttestation(witnessID string, domain string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	record, exists := rs.records[witnessID]
	if !exists {
		record = &ReputationRecord{
			WitnessID: witnessID,
			Domains:   make(map[string]*DomainReputation),
			FirstSeen: time.Now().UTC(),
		}
		rs.records[witnessID] = record
	}

	record.TotalClaims++
	record.LastSeen = time.Now().UTC()

	if domain != "" {
		domainRep, exists := record.Domains[domain]
		if !exists {
			domainRep = &DomainReputation{Domain: domain}
			record.Domains[domain] = domainRep
		}
		domainRep.TotalClaims++
	}
}

// RecordAgreement records that a witness agreed with consensus
func (rs *ReputationStore) RecordAgreement(witnessID string, domain string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	record, exists := rs.records[witnessID]
	if !exists {
		return
	}

	record.AgreedClaims++

	if domain != "" {
		if domainRep, exists := record.Domains[domain]; exists {
			domainRep.AgreedClaims++
		}
	}
}

// RecordDispute records that a witness was disputed
func (rs *ReputationStore) RecordDispute(witnessID string, domain string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	record, exists := rs.records[witnessID]
	if !exists {
		return
	}

	record.DisputedClaims++

	if domain != "" {
		if domainRep, exists := record.Domains[domain]; exists {
			domainRep.DisputedClaims++
		}
	}
}

// Score computes the reputation score for a witness
// Returns a value between 0 and 1
func (rr *ReputationRecord) Score() float64 {
	if rr.TotalClaims == 0 {
		return 0.5 // Neutral for new witnesses
	}

	// Base accuracy: agreed / total
	accuracy := float64(rr.AgreedClaims) / float64(rr.TotalClaims)

	// Penalty for disputes (heavier weight)
	disputeRatio := float64(rr.DisputedClaims) / float64(rr.TotalClaims)
	penalty := disputeRatio * 0.5

	// Longevity bonus (witnesses active longer get slight boost)
	age := time.Since(rr.FirstSeen)
	longevityBonus := math.Min(age.Hours()/(24*365), 0.1) // Max 10% bonus after 1 year

	// Volume confidence (more claims = more confident in score)
	volumeWeight := math.Min(float64(rr.TotalClaims)/100, 1.0)

	// Combine factors
	rawScore := accuracy - penalty + longevityBonus

	// Blend with neutral based on volume
	score := rawScore*volumeWeight + 0.5*(1-volumeWeight)

	// Clamp to [0, 1]
	return math.Max(0, math.Min(1, score))
}

// DomainScore computes the reputation score for a specific domain
func (rr *ReputationRecord) DomainScore(domain string) float64 {
	domainRep, exists := rr.Domains[domain]
	if !exists || domainRep.TotalClaims == 0 {
		return rr.Score() // Fall back to global score
	}

	accuracy := float64(domainRep.AgreedClaims) / float64(domainRep.TotalClaims)
	disputeRatio := float64(domainRep.DisputedClaims) / float64(domainRep.TotalClaims)
	penalty := disputeRatio * 0.5

	volumeWeight := math.Min(float64(domainRep.TotalClaims)/50, 1.0)

	rawScore := accuracy - penalty
	score := rawScore*volumeWeight + rr.Score()*(1-volumeWeight)

	return math.Max(0, math.Min(1, score))
}

// ClaimConfidence computes the confidence score for a claim based on its attestations
func ClaimConfidence(claim *Claim, store *ReputationStore) float64 {
	if len(claim.Witnesses) == 0 {
		return 0
	}

	var totalWeight float64
	var weightedSum float64

	for _, att := range claim.Witnesses {
		record, exists := store.GetRecord(att.WitnessID)
		var score float64
		if exists {
			score = record.DomainScore(claim.Statement.Domain)
		} else {
			score = 0.5 // Neutral for unknown witnesses
		}

		// Weight by reputation score (higher rep = more weight)
		weight := 0.5 + score*0.5 // Range [0.5, 1.0]
		weightedSum += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0
	}

	// Also factor in number of witnesses (diversity)
	witnessBonus := math.Min(float64(len(claim.Witnesses))/5, 0.2) // Max 20% bonus for 5+ witnesses

	confidence := (weightedSum / totalWeight) + witnessBonus
	return math.Max(0, math.Min(1, confidence))
}

// ExportRecord exports a reputation record for portability
type ExportedReputation struct {
	WitnessID      string                       `json:"witness_id"`
	TotalClaims    int64                        `json:"total_claims"`
	AgreedClaims   int64                        `json:"agreed_claims"`
	DisputedClaims int64                        `json:"disputed_claims"`
	Score          float64                      `json:"score"`
	Domains        map[string]ExportedDomain    `json:"domains,omitempty"`
	FirstSeen      time.Time                    `json:"first_seen"`
	LastSeen       time.Time                    `json:"last_seen"`
}

type ExportedDomain struct {
	TotalClaims    int64   `json:"total_claims"`
	AgreedClaims   int64   `json:"agreed_claims"`
	DisputedClaims int64   `json:"disputed_claims"`
	Score          float64 `json:"score"`
}

// Export exports a reputation record for external use
func (rr *ReputationRecord) Export() ExportedReputation {
	export := ExportedReputation{
		WitnessID:      rr.WitnessID,
		TotalClaims:    rr.TotalClaims,
		AgreedClaims:   rr.AgreedClaims,
		DisputedClaims: rr.DisputedClaims,
		Score:          rr.Score(),
		Domains:        make(map[string]ExportedDomain),
		FirstSeen:      rr.FirstSeen,
		LastSeen:       rr.LastSeen,
	}

	for domain, rep := range rr.Domains {
		export.Domains[domain] = ExportedDomain{
			TotalClaims:    rep.TotalClaims,
			AgreedClaims:   rep.AgreedClaims,
			DisputedClaims: rep.DisputedClaims,
			Score:          rr.DomainScore(domain),
		}
	}

	return export
}

// String returns a human-readable summary
func (rr *ReputationRecord) String() string {
	return fmt.Sprintf("Witness %s: score=%.2f claims=%d agreed=%d disputed=%d",
		rr.WitnessID[:16]+"...",
		rr.Score(),
		rr.TotalClaims,
		rr.AgreedClaims,
		rr.DisputedClaims,
	)
}
