# Claim-Graph: Portable Trust for Decentralized Systems

[![Go Reference](https://pkg.go.dev/badge/github.com/systemshift/claim-graph.svg)](https://pkg.go.dev/github.com/systemshift/claim-graph)
[![Go Version](https://img.shields.io/github/go-mod/go-version/systemshift/claim-graph)](https://go.dev/doc/devel/release)
[![License](https://img.shields.io/github/license/systemshift/claim-graph)](LICENSE)

## Overview

Claim-Graph is a chain-neutral system for creating, attesting, and verifying claims about the world. It enables portable trust—reputation that can move between different blockchain ecosystems, prediction markets, and decentralized applications.

Built on top of IPFS and [DAG-Time](https://github.com/systemshift/dag-time), Claim-Graph provides:

- **Content-addressed claims** with cryptographic integrity
- **Witness attestations** using ed25519 signatures
- **Reputation scoring** that tracks witness behavior over time
- **Time anchoring** via DAG-Time's drand integration

## Installation

### As a Library

```bash
go get github.com/systemshift/claim-graph
```

```go
import (
    "github.com/systemshift/claim-graph/claim"
    "github.com/systemshift/claim-graph/store"
)
```

### As a CLI Tool

```bash
go install github.com/systemshift/claim-graph/cmd/claimctl@latest
```

## Quick Start

### 1. Create a Witness Identity

```bash
claimctl identity create
# Created witness identity:
#   ID: 2feac6a20abe481de325d933fbfc4f5d875c2d54290f0f45a875d7f6b6111aea
#   Saved to: ~/.claimctl/identity.json
```

### 2. Create a Claim

```bash
claimctl claim create \
  --subject "https://example.com/sports/match-123" \
  --predicate "result" \
  --object "Manchester United won 2-1" \
  --domain "sports"
# Claim CID: bafkreifzr6prp4qymiioooqzjrpsrvpjdiff2ibqpr6o7czc5daipitnde
```

### 3. Attest to a Claim

```bash
claimctl witness attest bafkreifzr6prp4qymiioooqzjrpsrvpjdiff2ibqpr6o7czc5daipitnde
# Attestation added to claim
```

### 4. Verify a Claim

```bash
claimctl claim verify bafkreifzr6prp4qymiioooqzjrpsrvpjdiff2ibqpr6o7czc5daipitnde
# CID verification: OK
# Attestations: 2 valid
```

## Library Usage

```go
package main

import (
    "fmt"
    "github.com/systemshift/claim-graph/claim"
)

func main() {
    // Create a witness
    witness, _ := claim.GenerateWitness()
    fmt.Printf("Witness ID: %s\n", witness.ID)

    // Create a claim
    statement := claim.Statement{
        Subject:   "https://example.com/page",
        Predicate: "contains",
        Object:    "Hello World",
        Domain:    "web",
    }
    c, _ := claim.NewClaim(statement, []string{"evidence-cid"}, "dag-time-event")
    fmt.Printf("Claim CID: %s\n", c.ID)

    // Attest to the claim
    attestation, _ := witness.Attest(c)
    c.AddAttestation(attestation)

    // Verify
    err := c.VerifyAllAttestations()
    fmt.Printf("Valid: %v\n", err == nil)
}
```

## Core Concepts

### Claims

A claim is a statement about the world with supporting evidence:

```go
type Claim struct {
    ID        string        // Content-addressed identifier (CID)
    Statement Statement     // What is being claimed
    Evidence  []string      // CIDs of supporting data
    TimeEvent string        // DAG-Time event for temporal ordering
    Witnesses []Attestation // Signatures from witnesses
}

type Statement struct {
    Subject   string // What the claim is about
    Predicate string // The relationship or property
    Object    string // The value or target
    Domain    string // Category (e.g., "sports", "web", "finance")
}
```

### Witnesses

Witnesses are entities that attest to claims using ed25519 signatures:

```go
// Generate a new witness
witness, _ := claim.GenerateWitness()

// Attest to a claim
attestation, _ := witness.Attest(claim)

// Verify an attestation
err := claim.VerifyAttestation(claim, attestation)
```

### Reputation

Reputation is computed from witness behavior over time:

```go
store := claim.NewReputationStore()

// Record attestations and outcomes
store.RecordAttestation(witnessID, "sports")
store.RecordAgreement(witnessID, "sports")  // Witness was correct

// Get reputation
record, _ := store.GetRecord(witnessID)
score := record.Score()           // Overall score (0-1)
domainScore := record.DomainScore("sports") // Domain-specific score

// Compute claim confidence based on witness reputations
confidence := claim.ClaimConfidence(c, store)
```

### Storage

Claims can be stored on IPFS:

```go
s, _ := store.NewIPFSStore(store.IPFSConfig{
    APIURL: "http://localhost:5001",
})

// Store a claim
cid, _ := s.Put(ctx, claim)

// Retrieve a claim
claim, _ := s.Get(ctx, cid)

// List claims by filter
cids, _ := s.List(ctx, &store.Filter{
    Domain:    "sports",
    WitnessID: "abc123...",
})
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Consumers                                                  │
│  - Prediction markets (add staking/slashing)                │
│  - Blockchain oracles (consume verified claims)             │
│  - Applications (use reputation for trust decisions)        │
└─────────────────────────────────────────────────────────────┘
                          ↑ reads
┌─────────────────────────────────────────────────────────────┐
│  Claim-Graph                                                │
│  - Claims with evidence + witness signatures                │
│  - Reputation computed from history                         │
│  - No tokens, no staking (chain-neutral)                    │
└─────────────────────────────────────────────────────────────┘
                          ↑ uses
┌─────────────────────────────────────────────────────────────┐
│  DAG-Time                                                   │
│  - "No later than" timestamps via drand                     │
│  - Verifiable temporal ordering                             │
└─────────────────────────────────────────────────────────────┘
                          ↑ runs on
┌─────────────────────────────────────────────────────────────┐
│  IPFS                                                       │
│  - Content-addressed storage                                │
│  - P2P distribution                                         │
└─────────────────────────────────────────────────────────────┘
```

## The Problem This Solves

### Portable Trust

When you join a new blockchain or decentralized system, you start with zero reputation. But your behavior on other systems is publicly visible—just not verifiable across boundaries.

Claim-Graph creates a neutral layer where:
- **Evidence is external** (IPFS CIDs) - anyone can verify content
- **Time is external** (drand via DAG-Time) - anyone can verify ordering
- **Reputation is computed** from verifiable claim history
- **Any system can read** the claim graph to bootstrap trust

### Chain-Neutral Oracles

Traditional oracles are closed loops—reputation exists only within one ecosystem. Claim-Graph enables:

- Witnesses build reputation across multiple systems
- New systems can import existing trust
- No tokens or staking required (consumers add their own incentives)

## CLI Reference

```
claimctl - Claim Graph CLI

Commands:
  identity create     Create new witness keypair
  identity show       Show current witness ID

  claim create        Create a new claim
  claim get <cid>     Get a claim by CID
  claim verify <cid>  Verify a claim's integrity and attestations

  witness attest <cid>      Attest to a claim
  witness reputation <id>   Check witness reputation

Options:
  --ipfs    IPFS API URL (default: http://localhost:5001)
```

## Requirements

- Go 1.22+
- IPFS node (optional, for storage)

## Related Projects

- [DAG-Time](https://github.com/systemshift/dag-time) - Temporal ordering with drand anchoring
- [drand](https://drand.love) - Distributed randomness beacon

## License

BSD 3-Clause License. See [LICENSE](LICENSE) for details.
