package storage

import (
	"github.com/argus-labs/cardinal/component"
	"github.com/argus-labs/cardinal/internal/entity"
)

type ArchetypeIndex int

var _ ArchetypeAccessor = &archetypeStorageImpl{}

func NewArchetypeAccessor() ArchetypeAccessor {
	return &archetypeStorageImpl{archs: make([]*Archetype, 0)}
}

type archetypeStorageImpl struct {
	archs []*Archetype
}

func (a *archetypeStorageImpl) Archetypes() []*Archetype {
	return a.archs
}

func (a *archetypeStorageImpl) PushArchetype(index ArchetypeIndex, layout *Layout) {
	a.archs = append(a.archs, &Archetype{
		index:    index,
		entities: make([]entity.Entity, 0, 256),
		layout:   layout,
	})
}

func (a archetypeStorageImpl) Count() int {
	return len(a.archs)
}

func (a archetypeStorageImpl) Archetype(index ArchetypeIndex) ArchetypeStorage {
	return a.archs[index]
}

// Archetype is a collection of entities for a specific layout of components.
// This structure allows to quickly find entities based on their components.
type Archetype struct {
	index    ArchetypeIndex
	entities []entity.Entity
	layout   *Layout
}

// NewArchetype creates a new archetype.
func NewArchetype(index ArchetypeIndex, layout *Layout) *Archetype {
	return &Archetype{
		index:    index,
		entities: make([]entity.Entity, 0, 256),
		layout:   layout,
	}
}

// Layout is a collection of archetypes for a specific layout of components.
func (archetype *Archetype) Layout() *Layout {
	return archetype.layout
}

// Entities returns all entities in this archetype.
func (archetype *Archetype) Entities() []entity.Entity {
	return archetype.entities
}

// SwapRemove removes an entity from the archetype and returns it.
func (archetype *Archetype) SwapRemove(entityIndex int) entity.Entity {
	removed := archetype.entities[entityIndex]
	archetype.entities[entityIndex] = archetype.entities[len(archetype.entities)-1]
	archetype.entities = archetype.entities[:len(archetype.entities)-1]
	return removed
}

// LayoutMatches returns true if the given layout matches this archetype.
func (archetype *Archetype) LayoutMatches(components []component.IComponentType) bool {
	if len(archetype.layout.Components()) != len(components) {
		return false
	}
	for _, componentType := range components {
		if !archetype.layout.HasComponent(componentType) {
			return false
		}
	}
	return true
}

// PushEntity adds an entity to the archetype.
func (archetype *Archetype) PushEntity(entity entity.Entity) {
	archetype.entities = append(archetype.entities, entity)
}

// Count returns the number of entities in the archetype.
func (archetype *Archetype) Count() int {
	return len(archetype.entities)
}
