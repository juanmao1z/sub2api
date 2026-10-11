package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportTicketSystemMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("243_support_ticket_system.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS support_tickets",
		"CREATE TABLE IF NOT EXISTS support_ticket_messages",
		"CREATE TABLE IF NOT EXISTS support_ticket_reads",
		"CREATE TABLE IF NOT EXISTS support_ticket_tags",
		"CREATE TABLE IF NOT EXISTS support_ticket_tag_links",
		"client_id VARCHAR(100) NULL",
		"support_ticket_messages_client_idx",
		"first_response_due_at TIMESTAMPTZ",
		"resolution_due_at TIMESTAMPTZ",
		"ON CONFLICT (slug) DO NOTHING",
	} {
		require.Contains(t, sql, fragment)
	}
	require.Contains(t, sql, "CHECK (status IN ('open', 'in_progress', 'waiting_user', 'resolved', 'closed'))")
}
