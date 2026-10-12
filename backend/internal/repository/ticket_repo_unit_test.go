package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTicketWhereKeepsEmptyFiltersUnconstrained(t *testing.T) {
	where, args := ticketWhere(42, true, service.TicketListFilter{})
	require.Equal(t, "1 = 1", where)
	require.Empty(t, args)

	where, args = ticketWhere(42, false, service.TicketListFilter{Status: "active", Category: "", Priority: ""})
	require.Equal(t, "1 = 1 AND t.user_id = $1 AND t.status <> 'closed'", where)
	require.Equal(t, []any{int64(42)}, args)
}

func TestTicketWhereBuildsAdminAssignmentAndSearchFilters(t *testing.T) {
	where, args := ticketWhere(7, true, service.TicketListFilter{
		Search: "Alice", Status: "open", Category: "billing", Priority: "urgent", AssignedTo: "unassigned",
	})
	require.Contains(t, where, "LOWER(t.subject) LIKE $1")
	require.Contains(t, where, "t.status = $2")
	require.Contains(t, where, "t.category = $3")
	require.Contains(t, where, "t.priority = $4")
	require.Contains(t, where, "t.assigned_to IS NULL")
	require.Equal(t, []any{"%alice%", "open", "billing", "urgent"}, args)
}

func TestUnreadSQLUsesViewerCursorAndOppositeSender(t *testing.T) {
	adminSQL := unreadSQL(true, "$9")
	require.Contains(t, adminSQL, "ur.user_id = $9")
	require.Contains(t, adminSQL, "um.sender_type = 'user'")
	userSQL := unreadSQL(false, "$2")
	require.Contains(t, userSQL, "ur.user_id = $2")
	require.Contains(t, userSQL, "um.sender_type = 'admin'")
}

func TestSLADueUsesPriorityTiers(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	first, resolution := slaDue(now, service.TicketPriorityUrgent)
	require.Equal(t, 15*time.Minute, first.Sub(now))
	require.Equal(t, 4*time.Hour, resolution.Sub(now))
	first, resolution = slaDue(now, service.TicketPriorityLow)
	require.Equal(t, 8*time.Hour, first.Sub(now))
	require.Equal(t, 72*time.Hour, resolution.Sub(now))
}

type ticketScannerFunc func(...any) error

func (f ticketScannerFunc) Scan(dest ...any) error {
	return f(dest...)
}

func TestTicketStatsQueryQualifiesStatusAfterUsersJoin(t *testing.T) {
	query, args := ticketStatsQuery(42, true)

	require.Contains(t, query, "SELECT t.status")
	require.Contains(t, query, "GROUP BY t.status")
	require.Empty(t, args)
}

func TestTicketUnreadStatsConditionUsesCountAsBoolean(t *testing.T) {
	query := `SELECT COUNT(*) FROM support_tickets t WHERE t.user_id = t.user_id AND (` + unreadSQL(true, "$1") + `) > 0`

	require.Contains(t, query, ") > 0")
}

func TestScanTicketIncludesLastMessageAtColumn(t *testing.T) {
	called := false
	_, err := scanTicket(ticketScannerFunc(func(dest ...any) error {
		called = true
		require.Len(t, dest, 20)
		return nil
	}))

	require.NoError(t, err)
	require.True(t, called)
}
