package claim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClaim(t *testing.T) {
	statement := Statement{
		Subject:   "https://example.com",
		Predicate: "contains",
		Object:    "hello world",
		Domain:    "web",
	}

	c, err := NewClaim(statement, []string{"evidence1", "evidence2"}, "time-event-123")
	require.NoError(t, err)

	assert.NotEmpty(t, c.ID)
	assert.Equal(t, statement, c.Statement)
	assert.Equal(t, []string{"evidence1", "evidence2"}, c.Evidence)
	assert.Equal(t, "time-event-123", c.TimeEvent)
	assert.Empty(t, c.Witnesses)
}

func TestComputeCID(t *testing.T) {
	t.Run("same content produces same CID", func(t *testing.T) {
		now := time.Now().UTC()

		c1 := &Claim{
			Statement: Statement{Subject: "test", Predicate: "is", Object: "value"},
			Evidence:  []string{"a", "b"},
			TimeEvent: "event1",
			Created:   now,
		}

		c2 := &Claim{
			Statement: Statement{Subject: "test", Predicate: "is", Object: "value"},
			Evidence:  []string{"a", "b"},
			TimeEvent: "event1",
			Created:   now,
		}

		cid1, err := ComputeCID(c1)
		require.NoError(t, err)

		cid2, err := ComputeCID(c2)
		require.NoError(t, err)

		assert.Equal(t, cid1, cid2)
	})

	t.Run("different content produces different CID", func(t *testing.T) {
		now := time.Now().UTC()

		c1 := &Claim{
			Statement: Statement{Subject: "test1"},
			Created:   now,
		}

		c2 := &Claim{
			Statement: Statement{Subject: "test2"},
			Created:   now,
		}

		cid1, err := ComputeCID(c1)
		require.NoError(t, err)

		cid2, err := ComputeCID(c2)
		require.NoError(t, err)

		assert.NotEqual(t, cid1, cid2)
	})

	t.Run("witnesses do not affect CID", func(t *testing.T) {
		now := time.Now().UTC()

		c1 := &Claim{
			Statement: Statement{Subject: "test"},
			Created:   now,
			Witnesses: []Attestation{},
		}

		c2 := &Claim{
			Statement: Statement{Subject: "test"},
			Created:   now,
			Witnesses: []Attestation{{WitnessID: "witness1"}},
		}

		cid1, err := ComputeCID(c1)
		require.NoError(t, err)

		cid2, err := ComputeCID(c2)
		require.NoError(t, err)

		assert.Equal(t, cid1, cid2)
	})
}

func TestVerifyCID(t *testing.T) {
	c, err := NewClaim(Statement{Subject: "test"}, nil, "")
	require.NoError(t, err)

	t.Run("valid CID passes", func(t *testing.T) {
		err := VerifyCID(c)
		assert.NoError(t, err)
	})

	t.Run("tampered claim fails", func(t *testing.T) {
		c.Statement.Subject = "tampered"
		err := VerifyCID(c)
		assert.Error(t, err)
	})
}
