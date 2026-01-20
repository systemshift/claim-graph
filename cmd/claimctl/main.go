// claimctl is the command-line interface for the claim-graph system.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/systemshift/claim-graph/claim"
	"github.com/systemshift/claim-graph/store"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "identity":
		handleIdentity(args)
	case "claim":
		handleClaim(args)
	case "witness":
		handleWitness(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`claimctl - Claim Graph CLI

Usage:
  claimctl <command> [arguments]

Commands:
  identity    Manage witness identity
  claim       Create and manage claims
  witness     Attest to claims
  help        Show this help

Identity Commands:
  claimctl identity create              Create new witness keypair
  claimctl identity show                Show current witness ID

Claim Commands:
  claimctl claim create                 Create a new claim
  claimctl claim get <cid>              Get a claim by CID
  claimctl claim verify <cid>           Verify a claim

Witness Commands:
  claimctl witness attest <cid>         Attest to a claim
  claimctl witness reputation <id>      Check witness reputation

Examples:
  claimctl identity create
  claimctl claim create --subject "https://example.com" --predicate "contains" --object "text" --domain "web"
  claimctl witness attest bafyrei...`)
}

func handleIdentity(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: claimctl identity <create|show>")
		return
	}

	switch args[0] {
	case "create":
		witness, err := claim.GenerateWitness()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Save to file
		data := map[string]string{
			"id":          witness.ID,
			"private_key": fmt.Sprintf("%x", witness.PrivateKey),
		}

		file, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		path := os.ExpandEnv("$HOME/.claimctl/identity.json")
		if err := os.MkdirAll(os.ExpandEnv("$HOME/.claimctl"), 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config dir: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(path, file, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving identity: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created witness identity:\n")
		fmt.Printf("  ID: %s\n", witness.ID)
		fmt.Printf("  Saved to: %s\n", path)

	case "show":
		path := os.ExpandEnv("$HOME/.claimctl/identity.json")
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "No identity found. Run 'claimctl identity create' first.\n")
			os.Exit(1)
		}

		var identity map[string]string
		if err := json.Unmarshal(data, &identity); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading identity: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Witness ID: %s\n", identity["id"])

	default:
		fmt.Println("Usage: claimctl identity <create|show>")
	}
}

func handleClaim(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: claimctl claim <create|get|verify>")
		return
	}

	switch args[0] {
	case "create":
		createCmd := flag.NewFlagSet("create", flag.ExitOnError)
		subject := createCmd.String("subject", "", "Subject of the claim")
		predicate := createCmd.String("predicate", "", "Predicate (relationship)")
		object := createCmd.String("object", "", "Object (value)")
		domain := createCmd.String("domain", "", "Domain category")
		evidence := createCmd.String("evidence", "", "Evidence CIDs (comma-separated)")
		timeEvent := createCmd.String("time-event", "", "dag-time event ID")
		ipfsURL := createCmd.String("ipfs", "http://localhost:5001", "IPFS API URL")

		if err := createCmd.Parse(args[1:]); err != nil {
			os.Exit(1)
		}

		if *subject == "" || *predicate == "" || *object == "" {
			fmt.Println("Error: subject, predicate, and object are required")
			createCmd.Usage()
			os.Exit(1)
		}

		statement := claim.Statement{
			Subject:   *subject,
			Predicate: *predicate,
			Object:    *object,
			Domain:    *domain,
		}

		var evidenceList []string
		if *evidence != "" {
			// Simple split - in production would handle properly
			evidenceList = []string{*evidence}
		}

		c, err := claim.NewClaim(statement, evidenceList, *timeEvent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating claim: %v\n", err)
			os.Exit(1)
		}

		// Store in IPFS if available
		s, err := store.NewIPFSStore(store.IPFSConfig{APIURL: *ipfsURL})
		if err != nil {
			fmt.Printf("Warning: IPFS not available, claim not stored\n")
			fmt.Printf("Claim CID: %s\n", c.ID)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cid, err := s.Put(ctx, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error storing claim: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Claim created and stored:\n")
		fmt.Printf("  CID: %s\n", cid)

	case "get":
		if len(args) < 2 {
			fmt.Println("Usage: claimctl claim get <cid>")
			os.Exit(1)
		}

		cid := args[1]
		getCmd := flag.NewFlagSet("get", flag.ExitOnError)
		ipfsURL := getCmd.String("ipfs", "http://localhost:5001", "IPFS API URL")
		_ = getCmd.Parse(args[2:])

		s, err := store.NewIPFSStore(store.IPFSConfig{APIURL: *ipfsURL})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to IPFS: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		c, err := s.Get(ctx, cid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting claim: %v\n", err)
			os.Exit(1)
		}

		output, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(output))

	case "verify":
		if len(args) < 2 {
			fmt.Println("Usage: claimctl claim verify <cid>")
			os.Exit(1)
		}

		cid := args[1]
		verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
		ipfsURL := verifyCmd.String("ipfs", "http://localhost:5001", "IPFS API URL")
		_ = verifyCmd.Parse(args[2:])

		s, err := store.NewIPFSStore(store.IPFSConfig{APIURL: *ipfsURL})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to IPFS: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		c, err := s.Get(ctx, cid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting claim: %v\n", err)
			os.Exit(1)
		}

		// Verify CID
		if err := claim.VerifyCID(c); err != nil {
			fmt.Printf("CID verification: FAILED (%v)\n", err)
		} else {
			fmt.Printf("CID verification: OK\n")
		}

		// Verify attestations
		if len(c.Witnesses) == 0 {
			fmt.Printf("Attestations: none\n")
		} else {
			if err := c.VerifyAllAttestations(); err != nil {
				fmt.Printf("Attestations: INVALID (%v)\n", err)
			} else {
				fmt.Printf("Attestations: %d valid\n", len(c.Witnesses))
			}
		}

	default:
		fmt.Println("Usage: claimctl claim <create|get|verify>")
	}
}

func handleWitness(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: claimctl witness <attest|reputation>")
		return
	}

	switch args[0] {
	case "attest":
		if len(args) < 2 {
			fmt.Println("Usage: claimctl witness attest <cid>")
			os.Exit(1)
		}

		cid := args[1]
		attestCmd := flag.NewFlagSet("attest", flag.ExitOnError)
		ipfsURL := attestCmd.String("ipfs", "http://localhost:5001", "IPFS API URL")
		_ = attestCmd.Parse(args[2:])

		// Load identity
		path := os.ExpandEnv("$HOME/.claimctl/identity.json")
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "No identity found. Run 'claimctl identity create' first.\n")
			os.Exit(1)
		}

		var identity map[string]string
		if err := json.Unmarshal(data, &identity); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading identity: %v\n", err)
			os.Exit(1)
		}

		// Reconstruct witness from stored key
		witness, err := witnessFromStoredKey(identity["private_key"])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading witness: %v\n", err)
			os.Exit(1)
		}

		// Get claim
		s, err := store.NewIPFSStore(store.IPFSConfig{APIURL: *ipfsURL})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to IPFS: %v\n", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		c, err := s.Get(ctx, cid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting claim: %v\n", err)
			os.Exit(1)
		}

		// Create attestation
		attestation, err := witness.Attest(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating attestation: %v\n", err)
			os.Exit(1)
		}

		// Add to claim
		if err := c.AddAttestation(attestation); err != nil {
			fmt.Fprintf(os.Stderr, "Error adding attestation: %v\n", err)
			os.Exit(1)
		}

		// Store updated claim
		_, err = s.Put(ctx, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error storing attested claim: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Attestation added to claim %s\n", cid)
		fmt.Printf("  Witness: %s\n", witness.ID[:32]+"...")

	case "reputation":
		if len(args) < 2 {
			fmt.Println("Usage: claimctl witness reputation <witness-id>")
			os.Exit(1)
		}

		witnessID := args[1]
		fmt.Printf("Reputation lookup for witness %s\n", witnessID[:32]+"...")
		fmt.Println("(Reputation tracking not yet implemented)")

	default:
		fmt.Println("Usage: claimctl witness <attest|reputation>")
	}
}

func witnessFromStoredKey(hexKey string) (*claim.Witness, error) {
	// Decode hex private key
	var keyBytes []byte
	_, err := fmt.Sscanf(hexKey, "%x", &keyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// ed25519 private key is 64 bytes
	if len(keyBytes) != 64 {
		return nil, fmt.Errorf("invalid private key length: %d", len(keyBytes))
	}

	return &claim.Witness{
		ID:         fmt.Sprintf("%x", keyBytes[32:]), // Public key is last 32 bytes
		PublicKey:  keyBytes[32:],
		PrivateKey: keyBytes,
		Metadata:   make(map[string]string),
	}, nil
}
