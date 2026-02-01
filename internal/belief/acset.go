package belief

import (
	"database/sql"
	"fmt"
)

// BeliefSet represents a set of sentences (beliefs)
type BeliefSet struct {
	ID           int
	Name         string
	Description  string
	IsConsistent bool
	GayColor     string
	Trit         int // GF(3): -1, 0, +1
}

// EpistemicEntrenchment represents a preorder on sentences
type EpistemicEntrenchment struct {
	ID          int
	Name        string
	IsConnected bool // false => indeterministic
	GayColor    string
}

// Fallback represents a Grove-style sphere
type Fallback struct {
	ID           int
	BeliefSetID  int
	GayColor     string
	Trit         int
	Level        int // distance from innermost
	Incomparable int // count of incomparable siblings
}

// RevisionOp represents a deterministic belief revision operator
type RevisionOp struct {
	ID               int
	Name             string
	SatisfiesSuccess bool
	SatisfiesAGM     bool
	GayColor         string
	Trit             int
}

// IndetRevisionOp represents an indeterministic revision (set of operators)
type IndetRevisionOp struct {
	ID          int
	Name        string
	Description string
	Ops         []RevisionOp // contained deterministic operators
	GayColor    string
	Trit        int
}

// ACSet wraps the DuckDB connection for belief revision queries
type ACSet struct {
	db *sql.DB
}

// NewACSet connects to the belief revision database
func NewACSet(dbPath string) (*ACSet, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open belief revision ACSet: %w", err)
	}
	return &ACSet{db: db}, nil
}

// GetIncomparablePairs returns fallback pairs that create indeterminism
func (a *ACSet) GetIncomparablePairs() ([][2]Fallback, error) {
	query := `
		SELECT f1.fallback_id, f1.gay_color, f1.trit,
		       f2.fallback_id, f2.gay_color, f2.trit
		FROM Fallback f1, Fallback f2
		WHERE f1.fallback_id < f2.fallback_id
		  AND NOT EXISTS (SELECT 1 FROM FallbackOrder WHERE f1_id = f1.fallback_id AND f2_id = f2.fallback_id)
		  AND NOT EXISTS (SELECT 1 FROM FallbackOrder WHERE f1_id = f2.fallback_id AND f2_id = f1.fallback_id)
	`
	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs [][2]Fallback
	for rows.Next() {
		var f1, f2 Fallback
		if err := rows.Scan(&f1.ID, &f1.GayColor, &f1.Trit, &f2.ID, &f2.GayColor, &f2.Trit); err != nil {
			return nil, err
		}
		pairs = append(pairs, [2]Fallback{f1, f2})
	}
	return pairs, nil
}

// IsIndeterministic checks if the entrenchment yields multiple admissible revisions
func (e *EpistemicEntrenchment) IsIndeterministic() bool {
	return !e.IsConnected
}

// GF3Sum calculates the trit sum for conservation checking
func GF3Sum(trits []int) int {
	sum := 0
	for _, t := range trits {
		sum += t
	}
	return sum
}

// IsBalanced checks if trits sum to 0 mod 3
func IsBalanced(trits []int) bool {
	return GF3Sum(trits)%3 == 0
}
