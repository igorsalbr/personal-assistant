package vectorstore

import (
	"fmt"

	"personal-assistant/internal/domain"
)

// VectorStoreType represents the type of vector store
type VectorStoreType string

const (
	// PGVector uses PostgreSQL with pgvector extension
	PGVector VectorStoreType = "pgvector"
	// SQLFallback uses PostgreSQL with text search as fallback
	SQLFallback VectorStoreType = "sql_fallback"
)

// Factory creates vector store instances
type Factory struct{}

// NewFactory creates a new vector store factory
func NewFactory() *Factory {
	return &Factory{}
}

// Create creates a new vector store instance based on the type
func (f *Factory) Create(storeType VectorStoreType, config map[string]interface{}) (domain.VectorStore, error) {
	switch storeType {
	case PGVector:
		return NewPGVectorStore(config)
	case SQLFallback:
		return NewSQLFallbackStore(config)
	default:
		return nil, fmt.Errorf("unsupported vector store type: %s", storeType)
	}
}

// GetVectorStoreType parses a string to VectorStoreType
func GetVectorStoreType(s string) VectorStoreType {
	switch s {
	case "pgvector":
		return PGVector
	case "sql_fallback":
		return SQLFallback
	default:
		return PGVector // default to pgvector
	}
}
