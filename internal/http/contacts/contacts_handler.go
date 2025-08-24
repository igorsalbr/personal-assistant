package contacts

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"personal-assistant/internal/domain"
	"personal-assistant/internal/log"
)

// ContactsHandler handles HTTP requests for allowed contacts management
type ContactsHandler struct {
	tenantManager domain.TenantManager
	logger        *log.Logger
}

// NewContactsHandler creates a new contacts handler
func NewContactsHandler(tenantManager domain.TenantManager, logger *log.Logger) *ContactsHandler {
	return &ContactsHandler{
		tenantManager: tenantManager,
		logger:        logger,
	}
}

// CreateContactRequest represents a request to create an allowed contact
type CreateContactRequest struct {
	TenantID    string   `json:"tenant_id" validate:"required"`
	PhoneNumber string   `json:"phone_number" validate:"required"`
	ContactName string   `json:"contact_name"`
	Permissions []string `json:"permissions"`
	Notes       string   `json:"notes"`
}

// UpdateContactRequest represents a request to update an allowed contact
type UpdateContactRequest struct {
	ContactName string   `json:"contact_name"`
	Permissions []string `json:"permissions"`
	Notes       string   `json:"notes"`
	Enabled     *bool    `json:"enabled"`
}

// ListContacts returns all allowed contacts for a tenant
func (h *ContactsHandler) ListContacts(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant_id")
	
	if tenantID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id is required",
		})
	}

	// Get repository for the tenant
	repo, err := h.tenantManager.GetRepository(tenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Msg("failed to get repository")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get tenant repository",
		})
	}

	// Get allowed contacts
	contacts, err := repo.GetAllowedContacts(ctx, tenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Msg("failed to get allowed contacts")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get allowed contacts",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"contacts": contacts,
		"count":    len(contacts),
	})
}

// GetContact returns a specific allowed contact
func (h *ContactsHandler) GetContact(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant_id")
	phoneNumber := c.Param("phone_number")

	if tenantID == "" || phoneNumber == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id and phone_number are required",
		})
	}

	// Get repository for the tenant
	repo, err := h.tenantManager.GetRepository(tenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Msg("failed to get repository")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get tenant repository",
		})
	}

	// Get the contact
	contact, err := repo.GetAllowedContact(ctx, tenantID, phoneNumber)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Str("phone_number", phoneNumber).
			Msg("failed to get allowed contact")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get allowed contact",
		})
	}

	if contact == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "contact not found",
		})
	}

	return c.JSON(http.StatusOK, contact)
}

// CreateContact creates a new allowed contact
func (h *ContactsHandler) CreateContact(c echo.Context) error {
	ctx := c.Request().Context()
	
	var req CreateContactRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if req.TenantID == "" || req.PhoneNumber == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id and phone_number are required",
		})
	}

	// Get repository for the tenant
	repo, err := h.tenantManager.GetRepository(req.TenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", req.TenantID).
			Msg("failed to get repository")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get tenant repository",
		})
	}

	// Check if contact already exists
	existingContact, err := repo.GetAllowedContact(ctx, req.TenantID, req.PhoneNumber)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", req.TenantID).
			Str("phone_number", req.PhoneNumber).
			Msg("failed to check existing contact")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to check existing contact",
		})
	}

	if existingContact != nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "contact already exists",
		})
	}

	// Set default permissions if not provided
	if len(req.Permissions) == 0 {
		req.Permissions = []string{"chat", "schedule"}
	}

	// Create the contact
	contact := &domain.AllowedContact{
		ID:          uuid.New(),
		TenantID:    req.TenantID,
		PhoneNumber: req.PhoneNumber,
		ContactName: req.ContactName,
		Permissions: req.Permissions,
		Notes:       req.Notes,
		Enabled:     true,
	}

	if err := repo.CreateAllowedContact(ctx, contact); err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", req.TenantID).
			Str("phone_number", req.PhoneNumber).
			Msg("failed to create allowed contact")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create allowed contact",
		})
	}

	h.logger.WithContext(ctx).Info().
		Str("tenant_id", contact.TenantID).
		Str("phone_number", contact.PhoneNumber).
		Str("contact_name", contact.ContactName).
		Msg("allowed contact created")

	return c.JSON(http.StatusCreated, contact)
}

// UpdateContact updates an existing allowed contact
func (h *ContactsHandler) UpdateContact(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant_id")
	phoneNumber := c.Param("phone_number")

	if tenantID == "" || phoneNumber == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id and phone_number are required",
		})
	}

	var req UpdateContactRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	// Get repository for the tenant
	repo, err := h.tenantManager.GetRepository(tenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Msg("failed to get repository")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get tenant repository",
		})
	}

	// Get existing contact
	contact, err := repo.GetAllowedContact(ctx, tenantID, phoneNumber)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Str("phone_number", phoneNumber).
			Msg("failed to get allowed contact")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get allowed contact",
		})
	}

	if contact == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "contact not found",
		})
	}

	// Update fields
	if req.ContactName != "" {
		contact.ContactName = req.ContactName
	}
	if len(req.Permissions) > 0 {
		contact.Permissions = req.Permissions
	}
	if req.Notes != "" {
		contact.Notes = req.Notes
	}
	if req.Enabled != nil {
		contact.Enabled = *req.Enabled
	}

	// Update the contact
	if err := repo.UpdateAllowedContact(ctx, contact); err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Str("phone_number", phoneNumber).
			Msg("failed to update allowed contact")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update allowed contact",
		})
	}

	h.logger.WithContext(ctx).Info().
		Str("tenant_id", contact.TenantID).
		Str("phone_number", contact.PhoneNumber).
		Str("contact_name", contact.ContactName).
		Msg("allowed contact updated")

	return c.JSON(http.StatusOK, contact)
}

// DeleteContact deletes an allowed contact
func (h *ContactsHandler) DeleteContact(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant_id")
	contactIDStr := c.Param("contact_id")

	if tenantID == "" || contactIDStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id and contact_id are required",
		})
	}

	contactID, err := uuid.Parse(contactIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid contact_id format",
		})
	}

	// Get repository for the tenant
	repo, err := h.tenantManager.GetRepository(tenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Msg("failed to get repository")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get tenant repository",
		})
	}

	// Delete the contact
	if err := repo.DeleteAllowedContact(ctx, tenantID, contactID); err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Str("contact_id", contactID.String()).
			Msg("failed to delete allowed contact")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete allowed contact",
		})
	}

	h.logger.WithContext(ctx).Info().
		Str("tenant_id", tenantID).
		Str("contact_id", contactID.String()).
		Msg("allowed contact deleted")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "contact deleted successfully",
	})
}

// CheckContact checks if a phone number is allowed for a tenant
func (h *ContactsHandler) CheckContact(c echo.Context) error {
	ctx := c.Request().Context()
	tenantID := c.Param("tenant_id")
	phoneNumber := c.QueryParam("phone_number")

	if tenantID == "" || phoneNumber == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "tenant_id and phone_number are required",
		})
	}

	// Get repository for the tenant
	repo, err := h.tenantManager.GetRepository(tenantID)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Msg("failed to get repository")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get tenant repository",
		})
	}

	// Check if contact is allowed
	isAllowed, err := repo.IsContactAllowed(ctx, tenantID, phoneNumber)
	if err != nil {
		h.logger.WithContext(ctx).Error().
			Err(err).
			Str("tenant_id", tenantID).
			Str("phone_number", phoneNumber).
			Msg("failed to check if contact is allowed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to check contact",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"tenant_id":     tenantID,
		"phone_number":  phoneNumber,
		"is_allowed":    isAllowed,
	})
}