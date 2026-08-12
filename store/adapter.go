package store

// GraphNode represents a node suitable for GUI tree rendering.
type GraphNode struct {
	ID        int64
	Label     string
	BirthDate string
	DeathDate string
}

// GraphEdge represents a directed relationship between nodes.
type GraphEdge struct {
	From int64
	To   int64
	Type string // e.g. "parent", "spouse"
}

// Adapter converts store data into a graph model useful for visualization.
type Adapter interface {
	Nodes() ([]GraphNode, error)
	Edges() ([]GraphEdge, error)
}

// StoreAdapter implements Adapter backed by the Store.
type StoreAdapter struct {
	S *Store
}

func (a *StoreAdapter) Nodes() ([]GraphNode, error) {
	people, err := a.S.GetPeople()
	if err != nil {
		return nil, err
	}
	out := make([]GraphNode, 0, len(people))
	for _, p := range people {
		label := p.GivenName + " " + p.Surname
		out = append(out, GraphNode{ID: p.ID, Label: label, BirthDate: p.BirthDate, DeathDate: p.DeathDate})
	}
	return out, nil
}

func (a *StoreAdapter) Edges() ([]GraphEdge, error) {
	rels, err := a.S.GetRelationships()
	if err != nil {
		return nil, err
	}
	out := make([]GraphEdge, 0, len(rels))
	for _, r := range rels {
		out = append(out, GraphEdge{From: r.SubjectID, To: r.ObjectID, Type: r.Type})
	}
	return out, nil
}
