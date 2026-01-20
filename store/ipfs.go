package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/systemshift/claim-graph/claim"
)

// IPFSConfig configures the IPFS store
type IPFSConfig struct {
	// APIURL is the IPFS HTTP API URL (default: http://localhost:5001)
	APIURL string
}

// IPFSStore implements Store using IPFS
type IPFSStore struct {
	cfg    IPFSConfig
	client *http.Client

	// Local index for filtering/listing
	mu        sync.RWMutex
	index     map[string]*claim.Claim // CID -> Claim
	byWitness map[string][]string     // WitnessID -> CIDs
	byDomain  map[string][]string     // Domain -> CIDs
	bySubject map[string][]string     // Subject -> CIDs
}

// NewIPFSStore creates a new IPFS-backed store
func NewIPFSStore(cfg IPFSConfig) (*IPFSStore, error) {
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:5001"
	}

	s := &IPFSStore{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second, // Prevent hanging on DHT lookups
		},
		index:     make(map[string]*claim.Claim),
		byWitness: make(map[string][]string),
		byDomain:  make(map[string][]string),
		bySubject: make(map[string][]string),
	}

	// Verify IPFS connection
	if err := s.ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect to IPFS: %w", err)
	}

	return s, nil
}

func (s *IPFSStore) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.APIURL+"/api/v0/id", nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IPFS returned status %d", resp.StatusCode)
	}

	return nil
}

// claimData is the JSON structure stored in IPFS
type claimData struct {
	Statement claim.Statement     `json:"statement"`
	Evidence  []string            `json:"evidence"`
	TimeEvent string              `json:"time_event"`
	Witnesses []claim.Attestation `json:"witnesses"`
	Created   int64               `json:"created"` // Unix nano
	Metadata  map[string]string   `json:"metadata,omitempty"`
}

type ipfsAddResponse struct {
	Hash string `json:"Hash"`
}

func (s *IPFSStore) Put(ctx context.Context, c *claim.Claim) (string, error) {
	if c == nil {
		return "", fmt.Errorf("claim cannot be nil")
	}

	// Compute CID if not set
	if c.ID == "" {
		cid, err := claim.ComputeCID(c)
		if err != nil {
			return "", fmt.Errorf("failed to compute CID: %w", err)
		}
		c.ID = cid
	}

	// Serialize claim
	data := claimData{
		Statement: c.Statement,
		Evidence:  c.Evidence,
		TimeEvent: c.TimeEvent,
		Witnesses: c.Witnesses,
		Created:   c.Created.UnixNano(),
		Metadata:  c.Metadata,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to serialize claim: %w", err)
	}

	// Upload to IPFS
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "claim.json")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(jsonData); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.APIURL+"/api/v0/add", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to add to IPFS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("IPFS add failed: %s", string(body))
	}

	var addResp ipfsAddResponse
	if err := json.NewDecoder(resp.Body).Decode(&addResp); err != nil {
		return "", fmt.Errorf("failed to decode IPFS response: %w", err)
	}

	// Update local index
	s.mu.Lock()
	s.index[c.ID] = c
	s.indexClaim(c)
	s.mu.Unlock()

	return c.ID, nil
}

func (s *IPFSStore) indexClaim(c *claim.Claim) {
	// Index by witness
	for _, w := range c.Witnesses {
		s.byWitness[w.WitnessID] = append(s.byWitness[w.WitnessID], c.ID)
	}

	// Index by domain
	if c.Statement.Domain != "" {
		s.byDomain[c.Statement.Domain] = append(s.byDomain[c.Statement.Domain], c.ID)
	}

	// Index by subject
	if c.Statement.Subject != "" {
		s.bySubject[c.Statement.Subject] = append(s.bySubject[c.Statement.Subject], c.ID)
	}
}

func (s *IPFSStore) Get(ctx context.Context, cid string) (*claim.Claim, error) {
	if cid == "" {
		return nil, fmt.Errorf("CID cannot be empty")
	}

	// Check local index first
	s.mu.RLock()
	if c, exists := s.index[cid]; exists {
		s.mu.RUnlock()
		return c, nil
	}
	s.mu.RUnlock()

	// Fetch from IPFS
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.APIURL+"/api/v0/cat?arg="+cid, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from IPFS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("IPFS cat failed: %s", string(body))
	}

	var data claimData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode claim: %w", err)
	}

	c := &claim.Claim{
		ID:        cid,
		Statement: data.Statement,
		Evidence:  data.Evidence,
		TimeEvent: data.TimeEvent,
		Witnesses: data.Witnesses,
		Created:   time.Unix(0, data.Created).UTC(),
		Metadata:  data.Metadata,
	}

	// Cache in local index
	s.mu.Lock()
	s.index[cid] = c
	s.indexClaim(c)
	s.mu.Unlock()

	return c, nil
}

func (s *IPFSStore) Has(ctx context.Context, cid string) (bool, error) {
	s.mu.RLock()
	_, exists := s.index[cid]
	s.mu.RUnlock()

	// Only check local index - we cannot query IPFS by computed CID
	// since the computed CID differs from the IPFS storage hash.
	// The local index is the source of truth for this store instance.
	return exists, nil
}

func (s *IPFSStore) List(ctx context.Context, filter *Filter) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []string

	// Apply filters to narrow candidates
	if filter != nil && filter.WitnessID != "" {
		candidates = s.byWitness[filter.WitnessID]
	} else if filter != nil && filter.Domain != "" {
		candidates = s.byDomain[filter.Domain]
	} else if filter != nil && filter.Subject != "" {
		candidates = s.bySubject[filter.Subject]
	} else {
		// Return all
		candidates = make([]string, 0, len(s.index))
		for cid := range s.index {
			candidates = append(candidates, cid)
		}
	}

	// Apply additional filters
	var results []string
	for _, cid := range candidates {
		c := s.index[cid]
		if c == nil {
			continue
		}

		// Check all filter criteria
		if filter != nil {
			if filter.Domain != "" && c.Statement.Domain != filter.Domain {
				continue
			}
			if filter.Subject != "" && c.Statement.Subject != filter.Subject {
				continue
			}
			if filter.WitnessID != "" {
				found := false
				for _, w := range c.Witnesses {
					if w.WitnessID == filter.WitnessID {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}

		results = append(results, cid)
	}

	// Apply offset and limit
	if filter != nil && filter.Offset > 0 {
		if filter.Offset >= len(results) {
			return []string{}, nil
		}
		results = results[filter.Offset:]
	}

	if filter != nil && filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

func (s *IPFSStore) Close() error {
	return nil
}
