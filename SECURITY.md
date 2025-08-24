# Security Configuration

## Allowed Contacts

The WhatsApp bot implements sender allowlisting to ensure only authorized contacts can interact with your personal assistant.

### How It Works

1. **Recipient Validation**: Messages must be sent TO your configured WABA (WhatsApp Business API) number
2. **Sender Allowlisting**: Messages must be FROM a phone number in your allowed contacts list
3. **Silent Rejection**: Unauthorized messages are silently ignored and logged

### Database Schema

Allowed contacts are stored in the `allowed_contacts` table with the following fields:

- `tenant_id`: Which tenant (bot instance) owns this contact
- `phone_number`: The WhatsApp number allowed to contact the bot
- `contact_name`: Friendly name for the contact
- `permissions`: Array of permissions (e.g., ["chat", "schedule", "admin"])
- `enabled`: Whether this contact is currently active

### Managing Allowed Contacts

Use the REST API endpoints to manage your allowed contacts:

#### List all contacts for a tenant:
```bash
GET /api/v1/tenants/{tenant_id}/contacts
```

#### Get a specific contact:
```bash
GET /api/v1/tenants/{tenant_id}/contacts/{phone_number}
```

#### Add a new allowed contact:
```bash
POST /api/v1/tenants/{tenant_id}/contacts
Content-Type: application/json

{
  "tenant_id": "your_tenant_id",
  "phone_number": "+1234567890",
  "contact_name": "John Doe",
  "permissions": ["chat", "schedule"],
  "notes": "Primary user"
}
```

#### Update a contact:
```bash
PUT /api/v1/tenants/{tenant_id}/contacts/{phone_number}
Content-Type: application/json

{
  "contact_name": "John Smith",
  "permissions": ["chat", "schedule", "admin"],
  "enabled": true
}
```

#### Delete a contact:
```bash
DELETE /api/v1/tenants/{tenant_id}/contacts/{contact_id}
```

#### Check if a number is allowed:
```bash
GET /api/v1/tenants/{tenant_id}/contacts/check?phone_number={phone_number}
```

### Database Migration

Run the migration to add the allowed_contacts table:

```bash
# Apply migration
migrate -path internal/migrations -database "your_db_url" up

# Or run the specific migration
psql your_db_url < internal/migrations/003_allowed_contacts.up.sql
```

### System Configuration

The following system configuration options control contact allowlisting:

- `require_contact_allowlist`: Whether to enforce allowlisting (default: true)
- `default_contact_permissions`: Default permissions for new contacts
- `unknown_sender_action`: What to do with unknown senders (ignore/notify_admin/auto_add)

### Security Best Practices

1. **Regularly Review**: Periodically review your allowed contacts list
2. **Least Privilege**: Only grant necessary permissions to each contact
3. **Disable Unused**: Disable contacts that are no longer active
4. **Monitor Logs**: Watch for unauthorized contact attempts in the logs
5. **Use Strong Tenant IDs**: Use UUIDs or other strong identifiers for tenant_id

### Emergency Access

If you're locked out, you can manually add a contact via database:

```sql
INSERT INTO allowed_contacts (id, tenant_id, phone_number, contact_name, permissions, enabled) 
VALUES (
    uuid_generate_v4(),
    'your_tenant_id',
    '+1234567890',
    'Emergency Contact',
    ARRAY['chat', 'schedule', 'admin']::TEXT[],
    true
);
```

### Logging

All contact authorization attempts are logged with the following information:
- Tenant ID
- Sender phone number  
- WABA number (recipient)
- Message content (for failed attempts)
- Timestamp

Check logs for unauthorized access attempts:

```bash
grep "unauthorized sender" your_log_file.log
```