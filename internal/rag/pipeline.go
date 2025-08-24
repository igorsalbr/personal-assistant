package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// Pipeline implements the RAGPipeline interface
type Pipeline struct {
	llmProvider domain.LLMProvider
	vectorStore domain.VectorStore
	repository  domain.Repository
	logger      *log.Logger
	config      *PipelineConfig
}

// PipelineConfig holds configuration for the RAG pipeline
type PipelineConfig struct {
	MaxContextTokens   int     // Maximum tokens for context
	DefaultTopK        int     // Default number of results to retrieve
	DefaultMinScore    float64 // Default minimum similarity score
	ChunkSize          int     // Size for text chunking
	ChunkOverlap       int     // Overlap between chunks
	SummarizeThreshold int     // Token threshold for summarization
}

// NewPipeline creates a new RAG pipeline
func NewPipeline(
	llmProvider domain.LLMProvider,
	vectorStore domain.VectorStore,
	repository domain.Repository,
	logger *log.Logger,
	config *PipelineConfig,
) *Pipeline {
	if config == nil {
		config = &PipelineConfig{
			MaxContextTokens:   2000,
			DefaultTopK:        5,
			DefaultMinScore:    0.7,
			ChunkSize:          500,
			ChunkOverlap:       50,
			SummarizeThreshold: 4000,
		}
	}
	
	return &Pipeline{
		llmProvider: llmProvider,
		vectorStore: vectorStore,
		repository:  repository,
		logger:      logger,
		config:      config,
	}
}

// StoreMemory stores a memory item with embedding
func (p *Pipeline) StoreMemory(ctx context.Context, tenantID string, userID uuid.UUID, item *domain.MemoryItem) (*uuid.UUID, error) {
	start := time.Now()
	
	logger := p.logger.WithContext(ctx).WithTenant(tenantID).WithUser(userID.String())
	
	logger.Debug().
		Str("kind", item.Kind).
		Str("text", log.SanitizeText(item.Text)).
		Msg("storing memory item")
	
	// Generate embedding for the text
	embeddings, err := p.llmProvider.Embed(ctx, []string{item.Text})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated")
	}
	
	// Add embedding to metadata
	if item.Metadata == nil {
		item.Metadata = make(map[string]interface{})
	}
	item.Metadata["embedding"] = embeddings[0]
	item.Metadata["stored_at"] = time.Now().UTC().Format(time.RFC3339)
	
	// If text is very long, consider chunking
	items := []*domain.MemoryItem{item}
	if len(item.Text) > p.config.ChunkSize {
		items = p.chunkMemoryItem(item)
		
		// Generate embeddings for all chunks
		texts := make([]string, len(items))
		for i, chunk := range items {
			texts[i] = chunk.Text
		}
		
		chunkEmbeddings, err := p.llmProvider.Embed(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate chunk embeddings: %w", err)
		}
		
		// Assign embeddings to chunks
		for i, chunk := range items {
			if i < len(chunkEmbeddings) {
				chunk.Metadata["embedding"] = chunkEmbeddings[i]
			}
		}
	}
	
	// Store all items
	var ids []uuid.UUID
	for _, memItem := range items {
		storedIDs, err := p.vectorStore.Upsert(ctx, tenantID, userID, []domain.MemoryItem{*memItem})
		if err != nil {
			return nil, fmt.Errorf("failed to store memory item: %w", err)
		}
		ids = append(ids, storedIDs...)
	}
	
	if len(ids) == 0 {
		return nil, fmt.Errorf("no items were stored")
	}
	
	logger.Info().
		Str("id", ids[0].String()).
		Int("chunks", len(items)).
		Dur("duration", time.Since(start)).
		Msg("memory item stored successfully")
	
	return &ids[0], nil
}

// SearchMemory searches for relevant memories
func (p *Pipeline) SearchMemory(ctx context.Context, tenantID string, userID uuid.UUID, query string, opts *domain.SearchOptions) ([]domain.MemoryHit, error) {
	start := time.Now()
	
	logger := p.logger.WithContext(ctx).WithTenant(tenantID).WithUser(userID.String())
	
	if opts == nil {
		opts = &domain.SearchOptions{
			TopK:     p.config.DefaultTopK,
			MinScore: p.config.DefaultMinScore,
		}
	}
	
	logger.Debug().
		Str("query", query).
		Int("top_k", opts.TopK).
		Float64("min_score", opts.MinScore).
		Msg("searching memory")
	
	// Generate embedding for the query
	embeddings, err := p.llmProvider.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings generated for query")
	}
	
	// Perform vector search
	hits, err := p.vectorStore.Search(ctx, tenantID, userID, embeddings[0], opts)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	
	// Post-process results
	processedHits := p.postProcessResults(hits, query)
	
	logger.Info().
		Int("hits", len(processedHits)).
		Dur("duration", time.Since(start)).
		Msg("memory search completed")
	
	return processedHits, nil
}

// GetContext builds context from search results
func (p *Pipeline) GetContext(ctx context.Context, memories []domain.MemoryHit, maxTokens int) string {
	if len(memories) == 0 {
		return ""
	}
	
	logger := p.logger.WithContext(ctx)
	
	if maxTokens <= 0 {
		maxTokens = p.config.MaxContextTokens
	}
	
	// Build context string from memory hits
	var contextParts []string
	currentTokens := 0
	
	// Add a header
	contextParts = append(contextParts, "Relevant context from your memory:")
	currentTokens += p.estimateTokens("Relevant context from your memory:")
	
	for _, memory := range memories {
		// Format each memory item
		itemText := fmt.Sprintf("- %s (%s, score: %.2f): %s",
			memory.Kind,
			p.formatTimestamp(memory.Metadata),
			memory.Score,
			memory.Text,
		)
		
		itemTokens := p.estimateTokens(itemText)
		
		// Check if adding this item would exceed token limit
		if currentTokens+itemTokens > maxTokens {
			// Try to add a truncated version
			if currentTokens+100 <= maxTokens { // Reserve 100 tokens for truncation
				truncated := p.truncateText(itemText, maxTokens-currentTokens-20)
				contextParts = append(contextParts, truncated+"...")
			}
			break
		}
		
		contextParts = append(contextParts, itemText)
		currentTokens += itemTokens
	}
	
	context := strings.Join(contextParts, "\n")
	
	logger.Debug().
		Int("memory_items", len(memories)).
		Int("context_items_used", len(contextParts)-1). // -1 for header
		Int("estimated_tokens", currentTokens).
		Msg("context built from memories")
	
	return context
}

// chunkMemoryItem splits a large memory item into smaller chunks
func (p *Pipeline) chunkMemoryItem(item *domain.MemoryItem) []*domain.MemoryItem {
	text := item.Text
	if len(text) <= p.config.ChunkSize {
		return []*domain.MemoryItem{item}
	}
	
	var chunks []*domain.MemoryItem
	chunkIndex := 0
	
	for i := 0; i < len(text); i += p.config.ChunkSize - p.config.ChunkOverlap {
		end := i + p.config.ChunkSize
		if end > len(text) {
			end = len(text)
		}
		
		chunkText := text[i:end]
		
		// Create chunk metadata
		chunkMetadata := make(map[string]interface{})
		for k, v := range item.Metadata {
			chunkMetadata[k] = v
		}
		chunkMetadata["chunk_index"] = chunkIndex
		chunkMetadata["chunk_count"] = (len(text) + p.config.ChunkSize - 1) / p.config.ChunkSize
		chunkMetadata["parent_text_length"] = len(text)
		
		chunk := &domain.MemoryItem{
			Kind:     item.Kind,
			Text:     chunkText,
			Metadata: chunkMetadata,
		}
		
		chunks = append(chunks, chunk)
		chunkIndex++
		
		// If we've reached the end, break
		if end >= len(text) {
			break
		}
	}
	
	return chunks
}

// postProcessResults performs post-processing on search results
func (p *Pipeline) postProcessResults(hits []domain.MemoryHit, query string) []domain.MemoryHit {
	if len(hits) == 0 {
		return hits
	}
	
	// Remove duplicates based on text similarity
	uniqueHits := p.deduplicateResults(hits)
	
	// Sort by score (should already be sorted, but ensure it)
	// This is already done by the vector store, but we could add custom scoring here
	
	// Apply query-specific boosts
	boostedHits := p.applyQueryBoosts(uniqueHits, query)
	
	return boostedHits
}

// deduplicateResults removes very similar results
func (p *Pipeline) deduplicateResults(hits []domain.MemoryHit) []domain.MemoryHit {
	if len(hits) <= 1 {
		return hits
	}
	
	var unique []domain.MemoryHit
	
	for _, hit := range hits {
		isDuplicate := false
		for _, existing := range unique {
			// Simple text similarity check
			if p.textSimilarity(hit.Text, existing.Text) > 0.9 {
				isDuplicate = true
				break
			}
		}
		
		if !isDuplicate {
			unique = append(unique, hit)
		}
	}
	
	return unique
}

// applyQueryBoosts applies query-specific score boosts
func (p *Pipeline) applyQueryBoosts(hits []domain.MemoryHit, query string) []domain.MemoryHit {
	queryLower := strings.ToLower(query)
	
	for i := range hits {
		// Boost recent items for time-sensitive queries
		if strings.Contains(queryLower, "recent") || strings.Contains(queryLower, "latest") {
			if timestamp, ok := hits[i].Metadata["created_at"].(string); ok {
				if createdAt, err := time.Parse(time.RFC3339, timestamp); err == nil {
					age := time.Since(createdAt)
					if age < 24*time.Hour {
						hits[i].Score *= 1.2
					} else if age < 7*24*time.Hour {
						hits[i].Score *= 1.1
					}
				}
			}
		}
		
		// Boost tasks for task-related queries
		if strings.Contains(queryLower, "task") || strings.Contains(queryLower, "todo") {
			if hits[i].Kind == "task" {
				hits[i].Score *= 1.15
			}
		}
		
		// Boost events for schedule-related queries
		if strings.Contains(queryLower, "schedule") || strings.Contains(queryLower, "appointment") || strings.Contains(queryLower, "meeting") {
			if hits[i].Kind == "event" {
				hits[i].Score *= 1.15
			}
		}
	}
	
	return hits
}

// textSimilarity calculates a simple text similarity score
func (p *Pipeline) textSimilarity(text1, text2 string) float64 {
	// Simple Jaccard similarity on words
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))
	
	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}
	
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}
	
	// Create sets
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)
	
	for _, word := range words1 {
		set1[word] = true
	}
	
	for _, word := range words2 {
		set2[word] = true
	}
	
	// Calculate intersection
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}
	
	// Calculate union
	union := len(set1) + len(set2) - intersection
	
	if union == 0 {
		return 1.0
	}
	
	return float64(intersection) / float64(union)
}

// estimateTokens provides a rough token count estimation
func (p *Pipeline) estimateTokens(text string) int {
	// Rough approximation: 1 token ≈ 4 characters
	return (len(text) + 3) / 4
}

// truncateText truncates text to approximately the specified token count
func (p *Pipeline) truncateText(text string, maxTokens int) string {
	maxChars := maxTokens * 4 // Rough approximation
	if len(text) <= maxChars {
		return text
	}
	
	// Try to truncate at word boundaries
	truncated := text[:maxChars]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxChars/2 { // Only truncate at word boundary if it's not too far back
		truncated = truncated[:lastSpace]
	}
	
	return truncated
}

// formatTimestamp formats timestamp from metadata
func (p *Pipeline) formatTimestamp(metadata map[string]interface{}) string {
	if timestamp, ok := metadata["created_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
			return parsed.Format("Jan 2, 15:04")
		}
		return timestamp
	}
	
	if timestamp, ok := metadata["stored_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
			return parsed.Format("Jan 2, 15:04")
		}
		return timestamp
	}
	
	return "unknown"
}

// DefaultPipelineConfig returns the default pipeline configuration
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		MaxContextTokens:   2000,
		DefaultTopK:        5,
		DefaultMinScore:    0.7,
		ChunkSize:          500,
		ChunkOverlap:       50,
		SummarizeThreshold: 4000,
	}
}