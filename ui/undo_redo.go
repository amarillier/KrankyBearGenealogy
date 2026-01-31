package ui

import (
	"fmt"
	"sync"
	"time"

	"genealogy/store"
)

// Operation types
const (
	OpEditPerson = "EditPerson"
	OpDeletePerson = "DeletePerson"
	OpAddRelationship = "AddRelationship"
	OpDeleteRelationship = "DeleteRelationship"
	OpEditMarriage = "EditMarriage"
)

// UndoOperation represents a single undoable operation
type UndoOperation struct {
	Type        string
	Description string
	Timestamp   time.Time
	
	// Data for undo
	PersonBefore    *store.Person      // For EditPerson
	PersonAfter     *store.Person      // For EditPerson
	DeletedPerson   *store.Person      // For DeletePerson
	DeletedSpouses  []store.SpouseInfo // For DeletePerson
	DeletedChildren []int64            // For DeletePerson (child IDs)
	DeletedParents  []int64            // For DeletePerson (parent IDs)
	
	// Relationship data
	RelationshipType string // "parent", "spouse", "child"
	PersonID1        int64
	PersonID2        int64
	MarriageBefore   *store.SpouseInfo // For EditMarriage
	MarriageAfter    *store.SpouseInfo // For EditMarriage
}

// UndoRedoManager manages undo/redo history
type UndoRedoManager struct {
	undoStack []*UndoOperation
	redoStack []*UndoOperation
	maxSize   int
	mu        sync.Mutex
	store     *store.Store
	onUpdate  func() // Callback when undo/redo state changes
}

// Global undo/redo manager
var undoRedoManager *UndoRedoManager

// InitUndoRedo initializes the global undo/redo manager
func InitUndoRedo(s *store.Store, onUpdate func()) {
	undoRedoManager = &UndoRedoManager{
		undoStack: make([]*UndoOperation, 0),
		redoStack: make([]*UndoOperation, 0),
		maxSize:   50, // Keep last 50 operations
		store:     s,
		onUpdate:  onUpdate,
	}
}

// RecordEditPerson records a person edit operation
func RecordEditPerson(before, after *store.Person) {
	if undoRedoManager == nil {
		return
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	op := &UndoOperation{
		Type:         OpEditPerson,
		Description:  fmt.Sprintf("Edit %s", formatPersonName(*after)),
		Timestamp:    time.Now(),
		PersonBefore: copyPerson(before),
		PersonAfter:  copyPerson(after),
	}
	
	undoRedoManager.addOperation(op)
}

// RecordDeletePerson records a person deletion operation
func RecordDeletePerson(person *store.Person, spouses []store.SpouseInfo, childIDs, parentIDs []int64) {
	if undoRedoManager == nil {
		return
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	op := &UndoOperation{
		Type:            OpDeletePerson,
		Description:     fmt.Sprintf("Delete %s", formatPersonName(*person)),
		Timestamp:       time.Now(),
		DeletedPerson:   copyPerson(person),
		DeletedSpouses:  spouses,
		DeletedChildren: childIDs,
		DeletedParents:  parentIDs,
	}
	
	undoRedoManager.addOperation(op)
}

// RecordAddRelationship records adding a relationship
func RecordAddRelationship(relType string, personID1, personID2 int64, description string) {
	if undoRedoManager == nil {
		return
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	op := &UndoOperation{
		Type:             OpAddRelationship,
		Description:      description,
		Timestamp:        time.Now(),
		RelationshipType: relType,
		PersonID1:        personID1,
		PersonID2:        personID2,
	}
	
	undoRedoManager.addOperation(op)
}

// RecordDeleteRelationship records deleting a relationship
func RecordDeleteRelationship(relType string, personID1, personID2 int64, description string) {
	if undoRedoManager == nil {
		return
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	op := &UndoOperation{
		Type:             OpDeleteRelationship,
		Description:      description,
		Timestamp:        time.Now(),
		RelationshipType: relType,
		PersonID1:        personID1,
		PersonID2:        personID2,
	}
	
	undoRedoManager.addOperation(op)
}

// addOperation adds an operation to the undo stack
func (m *UndoRedoManager) addOperation(op *UndoOperation) {
	// Clear redo stack when new operation is added
	m.redoStack = make([]*UndoOperation, 0)
	
	// Add to undo stack
	m.undoStack = append(m.undoStack, op)
	
	// Trim if exceeds max size
	if len(m.undoStack) > m.maxSize {
		m.undoStack = m.undoStack[1:]
	}
	
	// Call update callback in goroutine to avoid deadlock
	// (callback may call CanUndo/CanRedo which also need the lock)
	if m.onUpdate != nil {
		go m.onUpdate()
	}
}

// CanUndo returns true if there are operations to undo
func CanUndo() bool {
	if undoRedoManager == nil {
		return false
	}
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	return len(undoRedoManager.undoStack) > 0
}

// CanRedo returns true if there are operations to redo
func CanRedo() bool {
	if undoRedoManager == nil {
		return false
	}
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	return len(undoRedoManager.redoStack) > 0
}

// GetUndoDescription returns description of the next undo operation
func GetUndoDescription() string {
	if undoRedoManager == nil || !CanUndo() {
		return ""
	}
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	return undoRedoManager.undoStack[len(undoRedoManager.undoStack)-1].Description
}

// GetRedoDescription returns description of the next redo operation
func GetRedoDescription() string {
	if undoRedoManager == nil || !CanRedo() {
		return ""
	}
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	return undoRedoManager.redoStack[len(undoRedoManager.redoStack)-1].Description
}

// Undo undoes the last operation
func Undo() error {
	if undoRedoManager == nil {
		return fmt.Errorf("undo/redo not initialized")
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	if len(undoRedoManager.undoStack) == 0 {
		return fmt.Errorf("nothing to undo")
	}
	
	// Pop from undo stack
	op := undoRedoManager.undoStack[len(undoRedoManager.undoStack)-1]
	undoRedoManager.undoStack = undoRedoManager.undoStack[:len(undoRedoManager.undoStack)-1]
	
	// Perform undo
	err := undoRedoManager.performUndo(op)
	if err != nil {
		// Put it back if undo failed
		undoRedoManager.undoStack = append(undoRedoManager.undoStack, op)
		return err
	}
	
	// Add to redo stack
	undoRedoManager.redoStack = append(undoRedoManager.redoStack, op)
	
	// Call update callback in goroutine to avoid deadlock
	if undoRedoManager.onUpdate != nil {
		go undoRedoManager.onUpdate()
	}
	
	return nil
}

// Redo redoes the last undone operation
func Redo() error {
	if undoRedoManager == nil {
		return fmt.Errorf("undo/redo not initialized")
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	if len(undoRedoManager.redoStack) == 0 {
		return fmt.Errorf("nothing to redo")
	}
	
	// Pop from redo stack
	op := undoRedoManager.redoStack[len(undoRedoManager.redoStack)-1]
	undoRedoManager.redoStack = undoRedoManager.redoStack[:len(undoRedoManager.redoStack)-1]
	
	// Perform redo
	err := undoRedoManager.performRedo(op)
	if err != nil {
		// Put it back if redo failed
		undoRedoManager.redoStack = append(undoRedoManager.redoStack, op)
		return err
	}
	
	// Add back to undo stack
	undoRedoManager.undoStack = append(undoRedoManager.undoStack, op)
	
	// Call update callback in goroutine to avoid deadlock
	if undoRedoManager.onUpdate != nil {
		go undoRedoManager.onUpdate()
	}
	
	return nil
}

// performUndo performs the actual undo operation
func (m *UndoRedoManager) performUndo(op *UndoOperation) error {
	switch op.Type {
	case OpEditPerson:
		// Restore person to before state
		return m.store.UpdatePerson(op.PersonBefore)
		
	case OpDeletePerson:
		// Re-create the deleted person
		// First create the person
		err := m.store.CreatePersonWithID(op.DeletedPerson)
		if err != nil {
			return fmt.Errorf("failed to restore person: %v", err)
		}
		
		// Restore relationships
		for _, spouseInfo := range op.DeletedSpouses {
			rel := &store.Relationship{
				SubjectID:      op.DeletedPerson.ID,
				ObjectID:       spouseInfo.Spouse.ID,
				Type:           "spouse",
				MarriageDate:   spouseInfo.MarriageDate,
				MarriagePlace:  spouseInfo.MarriagePlace,
				DivorceDate:    spouseInfo.DivorceDate,
				SeparationDate: spouseInfo.SeparationDate,
				EndReason:      spouseInfo.EndReason,
			}
			err := m.store.CreateRelationship(rel)
			if err != nil {
				// Log but continue
				fmt.Printf("Warning: failed to restore spouse relationship: %v\n", err)
			}
		}
		
		for _, childID := range op.DeletedChildren {
			rel := &store.Relationship{
				SubjectID: op.DeletedPerson.ID,
				ObjectID:  childID,
				Type:      "child",
			}
			err := m.store.CreateRelationship(rel)
			if err != nil {
				fmt.Printf("Warning: failed to restore child relationship: %v\n", err)
			}
		}
		
		for _, parentID := range op.DeletedParents {
			rel := &store.Relationship{
				SubjectID: op.DeletedPerson.ID,
				ObjectID:  parentID,
				Type:      "parent",
			}
			err := m.store.CreateRelationship(rel)
			if err != nil {
				fmt.Printf("Warning: failed to restore parent relationship: %v\n", err)
			}
		}
		
		return nil
		
	case OpAddRelationship:
		// Remove the relationship
		return m.store.RemoveRelationship(op.PersonID1, op.PersonID2, op.RelationshipType)
		
	case OpDeleteRelationship:
		// Re-add the relationship
		rel := &store.Relationship{
			SubjectID: op.PersonID1,
			ObjectID:  op.PersonID2,
			Type:      op.RelationshipType,
		}
		return m.store.CreateRelationship(rel)
		
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// performRedo performs the actual redo operation
func (m *UndoRedoManager) performRedo(op *UndoOperation) error {
	switch op.Type {
	case OpEditPerson:
		// Restore person to after state
		return m.store.UpdatePerson(op.PersonAfter)
		
	case OpDeletePerson:
		// Delete the person again
		return m.store.DeletePerson(op.DeletedPerson.ID)
		
	case OpAddRelationship:
		// Re-add the relationship
		rel := &store.Relationship{
			SubjectID: op.PersonID1,
			ObjectID:  op.PersonID2,
			Type:      op.RelationshipType,
		}
		return m.store.CreateRelationship(rel)
		
	case OpDeleteRelationship:
		// Remove the relationship again
		return m.store.RemoveRelationship(op.PersonID1, op.PersonID2, op.RelationshipType)
		
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// copyPerson creates a deep copy of a person
func copyPerson(p *store.Person) *store.Person {
	if p == nil {
		return nil
	}
	copy := *p
	return &copy
}

// ClearHistory clears all undo/redo history
func ClearHistory() {
	if undoRedoManager == nil {
		return
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	undoRedoManager.undoStack = make([]*UndoOperation, 0)
	undoRedoManager.redoStack = make([]*UndoOperation, 0)
	
	// Call update callback in goroutine to avoid deadlock
	if undoRedoManager.onUpdate != nil {
		go undoRedoManager.onUpdate()
	}
}

// GetHistorySize returns the current size of undo and redo stacks
func GetHistorySize() (undoCount, redoCount int) {
	if undoRedoManager == nil {
		return 0, 0
	}
	
	undoRedoManager.mu.Lock()
	defer undoRedoManager.mu.Unlock()
	
	return len(undoRedoManager.undoStack), len(undoRedoManager.redoStack)
}
