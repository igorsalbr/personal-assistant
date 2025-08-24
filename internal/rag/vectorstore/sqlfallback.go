package vectorstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// SQLFallbackStore implements VectorStore using SQL text search as fallback
type SQLFallbackStore struct {
	db     *pgxpool.Pool
	logger *log.Logger
}

// NewSQLFallbackStore creates a new SQL fallback vector store
func NewSQLFallbackStore(config map[string]interface{}) (*SQLFallbackStore, error) {
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

	return &SQLFallbackStore{
		db:     db,
		logger: logger,
	}, nil
}

// Upsert inserts or updates memory items (ignores embeddings in fallback mode)
func (vs *SQLFallbackStore) Upsert(ctx context.Context, tenantID string, userID uuid.UUID, items []domain.MemoryItem) ([]uuid.UUID, error) {
	if len(items) == 0 {
		return []uuid.UUID{}, nil
	}

	query := `
		INSERT INTO memory_chunks (id, tenant_id, user_id, kind, text, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			text = EXCLUDED.text,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	var ids []uuid.UUID
	now := time.Now().UTC()

	for _, item := range items {
		id := uuid.New()

		// Convert metadata to JSONB (exclude embedding data in fallback mode)
		metadata := make(map[string]interface{})
		for k, v := range item.Metadata {
			if k != "embedding" {
				metadata[k] = v
			}
		}

		var returnedID uuid.UUID
		err := vs.db.QueryRow(ctx, query,
			id, tenantID, userID, item.Kind, item.Text,
			metadata, now, now,
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
		Msg("memory items upserted (SQL fallback)")

	return ids, nil
}

// Search performs text-based search using PostgreSQL full-text search
func (vs *SQLFallbackStore) Search(ctx context.Context, tenantID string, userID uuid.UUID, queryEmbedding []float32, opts *domain.SearchOptions) ([]domain.MemoryHit, error) {
	if opts == nil {
		opts = &domain.SearchOptions{TopK: 5, MinScore: 0.0}
	}

	// Since we don't have query text directly, we'll do a basic text search
	// In a real implementation, you might want to pass the original query text
	query := `
		SELECT id, kind, text, metadata, 
		       ts_rank_cd(text_search, plainto_tsquery('english', $1)) as rank
		FROM memory_chunks
		WHERE tenant_id = $2 AND user_id = $3
		  AND text_search @@ plainto_tsquery('english', $1)
	`

	// For fallback mode, we'll search using a generic query or use ILIKE
	// This is a limitation of the fallback approach
	searchText := "memory" // Default search term
	if opts.Filter != nil && len(opts.Filter.Kinds) > 0 {
		searchText = opts.Filter.Kinds[0] // Use first kind as search term
	}

	args := []interface{}{searchText, tenantID, userID}
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

	// Add rank threshold (equivalent to similarity threshold)
	if opts.MinScore > 0 {
		query += fmt.Sprintf(" AND ts_rank_cd(text_search, plainto_tsquery('english', $1)) >= $%d", argIndex)
		args = append(args, opts.MinScore)
		argIndex++
	}

	// Order by rank and limit
	query += " ORDER BY rank DESC"
	if opts.TopK > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, opts.TopK)
	}

	vs.logger.WithContext(ctx).Debug().
		Str("query", query).
		Interface("args", args).
		Msg("executing text search (SQL fallback)")

	rows, err := vs.db.Query(ctx, query, args...)
	if err != nil {
		// Fallback to simple ILIKE search if full-text search fails
		return vs.searchWithILike(ctx, tenantID, userID, searchText, opts)
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
		Msg("text search completed (SQL fallback)")

	return hits, nil
}

// searchWithILike performs a simple ILIKE-based search as ultimate fallback
func (vs *SQLFallbackStore) searchWithILike(ctx context.Context, tenantID string, userID uuid.UUID, searchText string, opts *domain.SearchOptions) ([]domain.MemoryHit, error) {
	query := `
		SELECT id, kind, text, metadata, 
		       CASE 
		         WHEN text ILIKE $1 THEN 1.0
		         ELSE 0.5
		       END as score
		FROM memory_chunks
		WHERE tenant_id = $2 AND user_id = $3
		  AND (text ILIKE $1 OR kind ILIKE $4)
	`

	searchPattern := "%" + searchText + "%"
	args := []interface{}{searchPattern, tenantID, userID, searchPattern}
	argIndex := 5

	// Add filters if provided
	if opts.Filter != nil {
		if len(opts.Filter.Kinds) > 0 {
			query += fmt.Sprintf(" AND kind = ANY($%d)", argIndex)
			args = append(args, opts.Filter.Kinds)
			argIndex++
		}
	}

	// Order by score and limit
	query += " ORDER BY score DESC, created_at DESC"
	if opts.TopK > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, opts.TopK)
	}

	vs.logger.WithContext(ctx).Debug().
		Str("search_text", searchText).
		Msg("using ILIKE fallback search")

	rows, err := vs.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute ILIKE search: %w", err)
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
			return nil, fmt.Errorf("failed to scan ILIKE search result: %w", err)
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

	return hits, rows.Err()
}

// GetByID retrieves a memory item by ID
func (vs *SQLFallbackStore) GetByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) (*domain.MemoryChunk, error) {
	query := `
		SELECT id, tenant_id, user_id, kind, text, metadata, created_at, updated_at
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
func (vs *SQLFallbackStore) UpdateByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID, updates map[string]interface{}) error {
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
		// Ignore embedding updates in SQL fallback mode
		case "embedding":
			continue
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
		Msg("memory chunk updated (SQL fallback)")

	return nil
}

// DeleteByID deletes a memory item by ID
func (vs *SQLFallbackStore) DeleteByID(ctx context.Context, tenantID string, userID uuid.UUID, id uuid.UUID) error {
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
		Msg("memory chunk deleted (SQL fallback)")

	return nil
}

// Close closes the database connection
func (vs *SQLFallbackStore) Close() error {
	vs.db.Close()
	return nil
}
