package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// PGVectorStore implements VectorStore using PostgreSQL with pgvector
type PGVectorStore struct {
	db     *pgxpool.Pool
	logger *log.Logger
}

// NewPGVectorStore creates a new pgvector-based vector store
func NewPGVectorStore(config map[string]interface{}) (*PGVectorStore, error) {
	dbURL, ok := config["db_url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid db_url in config")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger := log.Init("info")
	if loggerConfig, ok := config["logger"].(*log.Logger); ok {
		logger = loggerConfig
	}

	return &PGVectorStore{
		db:     db,
		logger: logger,
	}, nil
}

// Upsert inserts or updates memory items with embeddings
func (vs *PGVectorStore) Upsert(ctx context.Context, tenantID string, userID uuid.UUID, items []domain.MemoryItem) ([]uuid.UUID, error) {
	if len(items) == 0 {
		return []uuid.UUID{}, nil
	}

	query := `
		INSERT INTO memory_chunks (id, tenant_id, user_id, kind, text, embedding, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			text = EXCLUDED.text,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	var ids []uuid.UUID
	now := time.Now().UTC()

	for _, item := range items {
		id := uuid.New()

		// Convert embedding to pgvector format
		var embedding pgvector.Vector
		if embedData, ok := item.Metadata["embedding"].([]float32); ok {
			embedding = pgvector.NewVector(embedData)
		} else {
			vs.logger.WithContext(ctx).Warn().
				Str("kind", item.Kind).
				Msg("missing embedding in memory item")
			continue
		}

		// Convert metadata to JSONB (remove embedding as it's stored separately)
		metadata := make(map[string]interface{})
		for k, v := range item.Metadata {
			if k != "embedding" {
				metadata[k] = v
			}
		}

		var returnedID uuid.UUID
		err := vs.db.QueryRow(ctx, query,
			id, tenantID, userID, item.Kind, item.Text,
			embedding, metadata, now, now,
		).Scan(&returnedID)

		if err != nil {
			vs.logger.WithContext(ctx).Error().
				Err(err).
				Str("tenant_id", tenantID).
				Str("user_id", userID.String()).
				Str("kind", item.Kind).
				Msg("failed to upsert memory item")
			return nil, fmt.Errorf("failed to upsert memory item: %w", err)
		}

		ids = append(ids, returnedID)
	}

	vs.logger.WithContext(ctx).Debug().
		Str("tenant_id", tenantID).
		Str("user_id", userID.String()).
		Int("count", len(ids)).
		Msg("memory items upserted")

	return ids, nil
}

// Search performs similarity search using cosine similarity
func (vs *PGVectorStore) Search(ctx context.Context, tenantID string, userID uuid.UUID, queryEmbedding []float32, opts *domain.SearchOptions) ([]domain.MemoryHit, error) {
	if opts == nil {
		opts = &domain.SearchOptions{TopK: 5, MinScore: 0.0}
	}

	// Build base query
	query := `
		SELECT id, kind, text, metadata, 1 - (embedding <=> $1) as similarity_score
		FROM memory_chunks
		WHERE tenant_id = $2 AND user_id = $3
	`
	args := []interface{}{
		pgvector.NewVector(queryEmbedding),
		tenantID,
		userID,
	}
	argIndex := 4

	// Add filters if provided
	if opts.Filter != nil {
		if len(opts.Filter.Kinds) > 0 {
			query += fmt.Sprintf(" AND kind = ANY($%d)", argIndex)
			args = append(args, opts.Filter.Kinds)
			argIndex++
		}

		if len(opts.Filter.Tags) > 0 {
			query += fmt.Sprintf(" AND metadata->>'tags' && $%d", argIndex)
			args = append(args, opts.Filter.Tags)
			argIndex++
		}

		if len(opts.Filter.Meta) > 0 {
			for k, v := range opts.Filter.Meta {
				query += fmt.Sprintf(" AND metadata->>$%d = $%d", argIndex, argIndex+1)
				args = append(args, k, v)
				argIndex += 2
			}
		}
	}

	// Add similarity threshold
	if opts.MinScore > 0 {
		query += fmt.Sprintf(" AND (1 - (embedding <=> $1)) >= $%d", argIndex)
		args = append(args, opts.MinScore)
		argIndex++
	}

	// Order by similarity and limit
	query += " ORDER BY similarity_score DESC"
	if opts.TopK > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, opts.TopK)
	}

	vs.logger.WithContext(ctx).Debug().
		Str("query", query).
		Interface("args", args).
		Msg("executing similarity search")

	rows, err := vs.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}
	defer rows.Close()

	var hits []domain.MemoryHit
	for rows.Next() {
		var hit domain.MemoryHit
		var metadataJSON []byte

		err := rows.Scan(
			&hit.ID,
			&hit.Kind,
			&hit.Text,
			&metadataJSON,
			&hit.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			hit.Metadata = make(map[string]interface{})
			if err := json.Unmarshal(metadataJSON, &hit.Metadata); err != nil {
				vs.logger.WithContext(ctx).Warn().
					Err(err).
					Str("id", hit.ID.String()).
					Msg("failed to unmarshal metadata")
			}
		}

		hits = append(hits, hit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	vs.logger.WithContext(ctx).Debug().
		Str("tenant_id", tenantID).
		Str("user_id", userID.String()).
		Int("hits", len(hits)).
		Msg("similarity search completed")

	return hits, nil
}

// GetByID retrieves a memory item by ID
func (vs *PGVectorStore) GetByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) (*domain.MemoryChunk, error) {
	query := `
		SELECT id, tenant_id, user_id, kind, text, embedding, metadata, created_at, updated_at
		FROM memory_chunks
		WHERE tenant_id = $1 AND user_id = $2 AND id = $3
	`

	var chunk domain.MemoryChunk
	var metadataJSON []byte

	err := vs.db.QueryRow(ctx, query, tenantID, userID, id).Scan(
		&chunk.ID,
		&chunk.TenantID,
		&chunk.UserID,
		&chunk.Kind,
		&chunk.Text,
		&chunk.Embedding,
		&metadataJSON,
		&chunk.CreatedAt,
		&chunk.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get memory chunk: %w", err)
	}

	// Parse metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &chunk.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &chunk, nil
}

// UpdateByID updates a memory item by ID
func (vs *PGVectorStore) UpdateByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID, updates map[string]interface{}) error {
	setParts := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIndex := 1

	// Build dynamic UPDATE query based on provided updates
	for field, value := range updates {
		switch field {
		case "text":
			setParts = append(setParts, fmt.Sprintf("text = $%d", argIndex))
			args = append(args, value)
			argIndex++
		case "metadata":
			setParts = append(setParts, fmt.Sprintf("metadata = $%d", argIndex))
			args = append(args, value)
			argIndex++
		case "kind":
			setParts = append(setParts, fmt.Sprintf("kind = $%d", argIndex))
			args = append(args, value)
			argIndex++
		case "embedding":
			if embedData, ok := value.([]float32); ok {
				setParts = append(setParts, fmt.Sprintf("embedding = $%d", argIndex))
				args = append(args, pgvector.NewVector(embedData))
				argIndex++
			}
		}
	}

	if len(setParts) == 1 { // Only updated_at
		return nil // Nothing to update
	}

	query := fmt.Sprintf(`
		UPDATE memory_chunks 
		SET %s
		WHERE tenant_id = $%d AND user_id = $%d AND id = $%d
	`,
		strings.Join(setParts, ", "),
		argIndex, argIndex+1, argIndex+2,
	)

	args = append(args, tenantID, userID, id)

	_, err := vs.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update memory chunk: %w", err)
	}

	vs.logger.WithContext(ctx).Debug().
		Str("id", id.String()).
		Str("tenant_id", tenantID).
		Msg("memory chunk updated")

	return nil
}

// DeleteByID deletes a memory item by ID
func (vs *PGVectorStore) DeleteByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error {
	query := `
		DELETE FROM memory_chunks
		WHERE tenant_id = $1 AND user_id = $2 AND id = $3
	`

	result, err := vs.db.Exec(ctx, query, tenantID, userID, id)
	if err != nil {
		return fmt.Errorf("failed to delete memory chunk: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("memory chunk not found")
	}

	vs.logger.WithContext(ctx).Debug().
		Str("id", id.String()).
		Str("tenant_id", tenantID).
		Msg("memory chunk deleted")

	return nil
}

// Close closes the database connection
func (vs *PGVectorStore) Close() error {
	vs.db.Close()
	return nil
}
