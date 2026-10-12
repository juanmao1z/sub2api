package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type TicketRepository struct {
	db *sql.DB
}

func NewTicketRepository(db *sql.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

func (r *TicketRepository) CreateTicket(ctx context.Context, userID int64, in service.TicketCreateInput) (service.Ticket, error) {
	now := time.Now().UTC()
	clientID := strings.TrimSpace(in.ClientID)
	var clientValue any
	if clientID != "" {
		clientValue = clientID
	}
	firstResponseDue, resolutionDue := slaDue(now, in.Priority)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return service.Ticket{}, err
	}
	defer tx.Rollback()

	var ticketID int64
	var lastMessageID int64
	insertQuery := `
		INSERT INTO support_tickets
			(user_id, subject, description, contact, category, priority, status, order_id, client_id, first_response_due_at, resolution_due_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,'open',$7,$8,$9,$10,$11,$11)`
	if clientID != "" {
		insertQuery += ` ON CONFLICT (user_id, client_id) WHERE client_id IS NOT NULL DO UPDATE SET updated_at = support_tickets.updated_at`
	}
	insertQuery += ` RETURNING id, COALESCE(last_message_id, 0)`
	if err := tx.QueryRowContext(ctx, insertQuery,
		userID, in.Subject, in.Content, in.Contact, in.Category, in.Priority, in.OrderID, clientValue,
		firstResponseDue, resolutionDue, now,
	).Scan(&ticketID, &lastMessageID); err != nil {
		return service.Ticket{}, err
	}
	if lastMessageID == 0 {
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO support_ticket_messages (ticket_id, sender_id, sender_type, kind, body, created_at)
			VALUES ($1,$2,'user','message',$3,$4) RETURNING id`, ticketID, userID, in.Content, now).Scan(&lastMessageID); err != nil {
			return service.Ticket{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE support_tickets
			SET last_message_id = $2, last_message_preview = LEFT($3, 240), updated_at = $4
			WHERE id = $1`, ticketID, lastMessageID, in.Content, now); err != nil {
			return service.Ticket{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return service.Ticket{}, err
	}
	detail, err := r.GetTicket(ctx, ticketID, userID, false, 0)
	return detail.Ticket, err
}
func (r *TicketRepository) ListTickets(ctx context.Context, viewerID int64, admin bool, filter service.TicketListFilter) (service.TicketPage, error) {
	where, args := ticketWhere(viewerID, admin, filter)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM support_tickets t JOIN users u ON u.id = t.user_id WHERE `+where, args...).Scan(&total); err != nil {
		return service.TicketPage{}, err
	}
	offset := (filter.Page - 1) * filter.PageSize
	queryArgs := append([]any(nil), args...)
	unreadPlaceholder := "$1"
	if admin {
		unreadPlaceholder = "$" + strconv.Itoa(len(queryArgs)+1)
		queryArgs = append(queryArgs, viewerID)
	}
	query := `
		SELECT t.id, t.user_id, t.subject, t.description, t.contact, t.category, t.priority, t.status, t.order_id,
			t.assigned_to, t.first_response_due_at, t.resolution_due_at, COALESCE(t.last_message_id, 0),
			t.last_message_preview, COALESCE((SELECT lm.created_at FROM support_ticket_messages lm WHERE lm.id = t.last_message_id), t.updated_at), ` + unreadSQL(admin, unreadPlaceholder) + `,
			COALESCE(u.username, ''), u.email, t.created_at, t.updated_at
		FROM support_tickets t
		JOIN users u ON u.id = t.user_id
		WHERE ` + where + `
		ORDER BY CASE t.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
			t.updated_at DESC, t.id DESC
		LIMIT $` + strconv.Itoa(len(queryArgs)+1) + ` OFFSET $` + strconv.Itoa(len(queryArgs)+2)
	queryArgs = append(queryArgs, filter.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return service.TicketPage{}, err
	}
	defer rows.Close()
	items := make([]service.Ticket, 0, filter.PageSize)
	for rows.Next() {
		item, err := scanTicket(rows)
		if err != nil {
			return service.TicketPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return service.TicketPage{}, err
	}
	pages := int(math.Ceil(float64(total) / float64(filter.PageSize)))
	if pages < 1 {
		pages = 1
	}
	return service.TicketPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, Pages: pages}, nil
}

func (r *TicketRepository) Stats(ctx context.Context, viewerID int64, admin bool) (service.TicketStats, error) {
	where, args := ticketWhere(viewerID, admin, service.TicketListFilter{})
	query := `SELECT status, COUNT(*) FROM support_tickets t JOIN users u ON u.id = t.user_id WHERE ` + where + ` GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return service.TicketStats{}, err
	}
	defer rows.Close()
	stats := service.TicketStats{CanCreate: true, DailyLimit: 10}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return service.TicketStats{}, err
		}
		stats.Total += count
		switch status {
		case service.TicketStatusOpen:
			stats.Open = count
		case service.TicketStatusInProgress:
			stats.InProgress = count
		case service.TicketStatusWaitingUser:
			stats.WaitingUser = count
		case service.TicketStatusResolved:
			stats.Resolved = count
		case service.TicketStatusClosed:
			stats.Closed = count
		}
	}
	if err := rows.Err(); err != nil {
		return service.TicketStats{}, err
	}
	unreadArgs := []any{viewerID}
	unreadWhere := `t.user_id = t.user_id`
	if !admin {
		unreadWhere = `t.user_id = $1`
	}
	unreadQuery := `SELECT COUNT(*) FROM support_tickets t WHERE ` + unreadWhere + ` AND ` + unreadSQL(admin, "$1")
	if err := r.db.QueryRowContext(ctx, unreadQuery, unreadArgs...).Scan(&stats.Unread); err != nil {
		return service.TicketStats{}, err
	}
	if !admin {
		var used int64
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM support_tickets WHERE user_id = $1 AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'`, viewerID).Scan(&used); err != nil {
			return service.TicketStats{}, err
		}
		stats.DailyUsed = used
		stats.CanCreate = used < stats.DailyLimit
		if !stats.CanCreate {
			if err := r.db.QueryRowContext(ctx, `SELECT MIN(created_at + INTERVAL '24 hours') FROM support_tickets WHERE user_id = $1 AND created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'`, viewerID).Scan(&stats.NextCreateAt); err != nil {
				return service.TicketStats{}, err
			}
		}
	}
	return stats, nil
}

func (r *TicketRepository) GetTicket(ctx context.Context, id, viewerID int64, admin bool, beforeID int64) (service.TicketDetail, error) {
	where := `t.id = $1`
	args := []any{id}
	if !admin {
		where += ` AND t.user_id = $2`
		args = append(args, viewerID)
	} else {
		args = append(args, viewerID)
	}
	query := `
		SELECT t.id, t.user_id, t.subject, t.description, t.contact, t.category, t.priority, t.status, t.order_id,
			t.assigned_to, t.first_response_due_at, t.resolution_due_at, COALESCE(t.last_message_id, 0),
			t.last_message_preview, COALESCE((SELECT lm.created_at FROM support_ticket_messages lm WHERE lm.id = t.last_message_id), t.updated_at), ` + unreadSQL(admin, "$2") + `,
			COALESCE(u.username, ''), u.email, t.created_at, t.updated_at
		FROM support_tickets t JOIN users u ON u.id = t.user_id WHERE ` + where
	var detail service.TicketDetail
	var err error
	var ticket service.Ticket
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&ticket.ID, &ticket.UserID, &ticket.Subject, &ticket.Description, &ticket.Contact, &ticket.Category, &ticket.Priority,
		&ticket.Status, &ticket.OrderID, &ticket.AssignedTo, &ticket.FirstResponseDueAt, &ticket.ResolutionDueAt,
		&ticket.LastMessageID, &ticket.LastMessagePreview, &ticket.UnreadCount, &ticket.UserName, &ticket.UserEmail,
		&ticket.CreatedAt, &ticket.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return service.TicketDetail{}, notFoundTicket()
	}
	if err != nil {
		return service.TicketDetail{}, err
	}
	detail.Ticket = ticket
	if admin {
		detail.Requester, err = r.requester(ctx, ticket.UserID)
		if err != nil {
			return service.TicketDetail{}, err
		}
	}
	tags, err := r.ticketTags(ctx, id)
	if err != nil {
		return service.TicketDetail{}, err
	}
	detail.Tags = tags
	messages, hasMore, err := r.messages(ctx, id, admin, beforeID)
	if err != nil {
		return service.TicketDetail{}, err
	}
	detail.Messages = messages
	detail.HasMore = hasMore
	return detail, nil
}

func (r *TicketRepository) AddMessage(ctx context.Context, id, senderID int64, role, body, clientID string, internal bool) (service.TicketDetail, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return service.TicketDetail{}, err
	}
	defer tx.Rollback()
	var status string
	var userID int64
	if err := tx.QueryRowContext(ctx, `SELECT status, user_id FROM support_tickets WHERE id = $1 FOR UPDATE`, id).Scan(&status, &userID); err != nil {
		if err == sql.ErrNoRows {
			return service.TicketDetail{}, notFoundTicket()
		}
		return service.TicketDetail{}, err
	}
	clientID = strings.TrimSpace(clientID)
	if !internal && clientID != "" {
		var existingMessageID int64
		lookupErr := tx.QueryRowContext(ctx, `SELECT id FROM support_ticket_messages WHERE ticket_id = $1 AND sender_id = $2 AND client_id = $3`, id, senderID, clientID).Scan(&existingMessageID)
		if lookupErr == nil {
			if err := tx.Commit(); err != nil {
				return service.TicketDetail{}, err
			}
			return r.GetTicket(ctx, id, userID, role == "admin", 0)
		}
		if lookupErr != sql.ErrNoRows {
			return service.TicketDetail{}, lookupErr
		}
	}
	if !internal && status == service.TicketStatusClosed {
		return service.TicketDetail{}, ticketError("TICKET_CLOSED", "closed tickets cannot accept replies")
	}
	now := time.Now().UTC()
	if internal {
		_, err = tx.ExecContext(ctx, `INSERT INTO support_ticket_messages (ticket_id, sender_id, sender_type, kind, body, event_type, event_data, created_at) VALUES ($1,$2,'admin','event',$3,'internal_note',$4,$5)`, id, senderID, body, []byte(`{"visibility":"internal"}`), now)
	} else {
		nextStatus := service.TicketStatusOpen
		if role == "admin" {
			nextStatus = service.TicketStatusWaitingUser
		}
		var clientValue any
		if clientID != "" {
			clientValue = clientID
		}
		var messageID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO support_ticket_messages (ticket_id, sender_id, sender_type, kind, body, client_id, created_at) VALUES ($1,$2,$3,'message',$4,$5,$6) RETURNING id`, id, senderID, role, body, clientValue, now).Scan(&messageID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE support_tickets SET status = $2, first_response_at = CASE WHEN $3 = 'admin' AND first_response_at IS NULL THEN $4 ELSE first_response_at END, resolved_at = NULL, closed_at = NULL, last_message_id = $5, last_message_preview = LEFT($6, 240), updated_at = $4 WHERE id = $1`, id, nextStatus, role, now, messageID, body)
		}
	}
	if err != nil {
		return service.TicketDetail{}, err
	}
	if internal {
		_, err = tx.ExecContext(ctx, `UPDATE support_tickets SET updated_at = $2 WHERE id = $1`, id, now)
	}
	if err != nil {
		return service.TicketDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return service.TicketDetail{}, err
	}
	return r.GetTicket(ctx, id, userID, role == "admin", 0)
}

func (r *TicketRepository) UpdateTicket(ctx context.Context, id, actorID int64, admin bool, in service.TicketUpdateInput) (service.TicketDetail, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return service.TicketDetail{}, err
	}
	defer tx.Rollback()
	var userID int64
	var oldStatus, oldPriority, oldCategory string
	var oldAssigned sql.NullInt64
	query := `SELECT user_id, status, priority, category, assigned_to FROM support_tickets WHERE id = $1`
	args := []any{id}
	if !admin {
		query += ` AND user_id = $2`
		args = append(args, actorID)
	}
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&userID, &oldStatus, &oldPriority, &oldCategory, &oldAssigned); err != nil {
		if err == sql.ErrNoRows {
			return service.TicketDetail{}, notFoundTicket()
		}
		return service.TicketDetail{}, err
	}
	if in.Status != nil {
		if err := service.ValidateTicketStatusTransition(oldStatus, *in.Status); err != nil {
			return service.TicketDetail{}, err
		}
	}
	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	values := []any{}
	add := func(expr string, value any) {
		values = append(values, value)
		sets = append(sets, expr+" = $"+strconv.Itoa(len(values)))
	}
	if in.Status != nil {
		add("status", *in.Status)
		if *in.Status == service.TicketStatusResolved {
			sets = append(sets, "resolved_at = CURRENT_TIMESTAMP", "closed_at = NULL")
		} else if *in.Status == service.TicketStatusClosed {
			sets = append(sets, "closed_at = CURRENT_TIMESTAMP")
		} else {
			sets = append(sets, "resolved_at = NULL", "closed_at = NULL")
		}
	}
	if in.Category != nil {
		add("category", *in.Category)
	}
	if in.Priority != nil {
		add("priority", *in.Priority)
	}
	if in.ClearOwner {
		sets = append(sets, "assigned_to = NULL")
	} else if in.AssignedTo != nil {
		add("assigned_to", *in.AssignedTo)
	}
	values = append(values, id)
	updateQuery := `UPDATE support_tickets SET ` + strings.Join(sets, ", ") + ` WHERE id = $` + strconv.Itoa(len(values))
	if _, err := tx.ExecContext(ctx, updateQuery, append(values[:len(values)-1], id)...); err != nil {
		return service.TicketDetail{}, err
	}
	if in.Status != nil && oldStatus != *in.Status {
		if err := insertEvent(ctx, tx, id, actorID, "status_changed", map[string]any{"from": oldStatus, "to": *in.Status}); err != nil {
			return service.TicketDetail{}, err
		}
	}
	if in.Priority != nil && oldPriority != *in.Priority {
		if err := insertEvent(ctx, tx, id, actorID, "priority_changed", map[string]any{"from": oldPriority, "to": *in.Priority}); err != nil {
			return service.TicketDetail{}, err
		}
	}
	if in.Category != nil && oldCategory != *in.Category {
		if err := insertEvent(ctx, tx, id, actorID, "category_changed", map[string]any{"from": oldCategory, "to": *in.Category}); err != nil {
			return service.TicketDetail{}, err
		}
	}
	oldAssignedID := int64(0)
	if oldAssigned.Valid {
		oldAssignedID = oldAssigned.Int64
	}
	newAssignedID := oldAssignedID
	if in.ClearOwner {
		newAssignedID = 0
	} else if in.AssignedTo != nil {
		newAssignedID = *in.AssignedTo
	}
	if newAssignedID != oldAssignedID {
		if err := insertEvent(ctx, tx, id, actorID, "assignment_changed", map[string]any{"from": oldAssignedID, "to": newAssignedID}); err != nil {
			return service.TicketDetail{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return service.TicketDetail{}, err
	}
	return r.GetTicket(ctx, id, actorID, admin, 0)
}

func (r *TicketRepository) MarkRead(ctx context.Context, id, userID, lastMessageID int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO support_ticket_reads (ticket_id, user_id, last_message_id, updated_at)
		VALUES ($1,$2,$3,CURRENT_TIMESTAMP)
		ON CONFLICT (ticket_id, user_id) DO UPDATE SET last_message_id = GREATEST(support_ticket_reads.last_message_id, EXCLUDED.last_message_id), updated_at = CURRENT_TIMESTAMP`, id, userID, lastMessageID)
	return err
}

func (r *TicketRepository) ReplaceTags(ctx context.Context, id int64, tagIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM support_ticket_tag_links WHERE ticket_id = $1`, id); err != nil {
		return err
	}
	unique := make(map[int64]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		if tagID > 0 {
			unique[tagID] = struct{}{}
		}
	}
	inserted := 0
	for tagID := range unique {
		result, err := tx.ExecContext(ctx, `INSERT INTO support_ticket_tag_links (ticket_id, tag_id) SELECT $1, id FROM support_ticket_tags WHERE id = $2 ON CONFLICT DO NOTHING`, id, tagID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		inserted += int(count)
	}
	if inserted != len(unique) {
		return ticketError("INVALID_TAGS", "one or more ticket tags do not exist")
	}
	return tx.Commit()
}

func (r *TicketRepository) ListAssignees(ctx context.Context) ([]service.TicketAssignee, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, COALESCE(username, ''), email FROM users WHERE role = 'admin' AND status = 'active' ORDER BY COALESCE(NULLIF(username, ''), email), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]service.TicketAssignee, 0)
	for rows.Next() {
		var assignee service.TicketAssignee
		if err := rows.Scan(&assignee.ID, &assignee.Username, &assignee.Email); err != nil {
			return nil, err
		}
		result = append(result, assignee)
	}
	return result, rows.Err()
}

func (r *TicketRepository) ListTags(ctx context.Context) ([]service.TicketTag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, slug FROM support_ticket_tags ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]service.TicketTag, 0)
	for rows.Next() {
		var tag service.TicketTag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, err
		}
		result = append(result, tag)
	}
	return result, rows.Err()
}

func (r *TicketRepository) ticketTags(ctx context.Context, ticketID int64) ([]service.TicketTag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id, t.name, t.slug FROM support_ticket_tags t JOIN support_ticket_tag_links l ON l.tag_id = t.id WHERE l.ticket_id = $1 ORDER BY t.name`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]service.TicketTag, 0)
	for rows.Next() {
		var tag service.TicketTag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, err
		}
		result = append(result, tag)
	}
	return result, rows.Err()
}

func (r *TicketRepository) messages(ctx context.Context, ticketID int64, admin bool, beforeID int64) ([]service.TicketMessage, bool, error) {
	visibility := `m.kind = 'message' OR (m.kind = 'event' AND m.event_type <> 'internal_note')`
	if admin {
		visibility = `TRUE`
	}
	args := []any{ticketID}
	condition := `m.ticket_id = $1 AND (` + visibility + `)`
	if beforeID > 0 {
		args = append(args, beforeID)
		condition += ` AND m.id < $2`
	}
	query := `SELECT m.id, m.ticket_id, m.kind, m.sender_type, COALESCE(u.username, ''), m.body, m.event_type, m.event_data, m.created_at FROM support_ticket_messages m LEFT JOIN users u ON u.id = m.sender_id WHERE ` + condition + ` ORDER BY m.id DESC LIMIT 51`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	result := make([]service.TicketMessage, 0, 50)
	for rows.Next() {
		var message service.TicketMessage
		var raw json.RawMessage
		if err := rows.Scan(&message.ID, &message.TicketID, &message.Kind, &message.AuthorRole, &message.AuthorName, &message.Content, &message.EventType, &raw, &message.CreatedAt); err != nil {
			return nil, false, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &message.EventData)
		}
		result = append(result, message)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(result) > 50
	if hasMore {
		result = result[:50]
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, hasMore, nil
}

func (r *TicketRepository) requester(ctx context.Context, userID int64) (*service.TicketRequester, error) {
	var requester service.TicketRequester
	err := r.db.QueryRowContext(ctx, `SELECT id, COALESCE(username,''), email, balance, status, created_at FROM users WHERE id = $1`, userID).Scan(&requester.ID, &requester.Username, &requester.Email, &requester.Balance, &requester.Status, &requester.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &requester, nil
}

func ticketWhere(viewerID int64, admin bool, filter service.TicketListFilter) (string, []any) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if !admin {
		args = append(args, viewerID)
		conditions = append(conditions, `t.user_id = $`+strconv.Itoa(len(args)))
	}
	if value := strings.TrimSpace(filter.Search); value != "" {
		args = append(args, "%"+strings.ToLower(value)+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, `(LOWER(t.subject) LIKE `+placeholder+` OR LOWER(t.description) LIKE `+placeholder+` OR LOWER(COALESCE(u.username,'')) LIKE `+placeholder+` OR LOWER(u.email) LIKE `+placeholder+`)`)
	}
	for _, pair := range [][2]string{{"t.status", filter.Status}, {"t.category", filter.Category}, {"t.priority", filter.Priority}} {
		value := strings.TrimSpace(pair[1])
		if value == "" {
			continue
		}
		if pair[0] == "t.status" && value == "active" {
			conditions = append(conditions, `t.status <> 'closed'`)
			continue
		}
		args = append(args, value)
		conditions = append(conditions, pair[0]+` = $`+strconv.Itoa(len(args)))
	}
	if admin && filter.AssignedTo != "" && filter.AssignedTo != "all" {
		if filter.AssignedTo == "unassigned" {
			conditions = append(conditions, `t.assigned_to IS NULL`)
		} else if id, err := strconv.ParseInt(filter.AssignedTo, 10, 64); err == nil && id > 0 {
			args = append(args, id)
			conditions = append(conditions, `t.assigned_to = $`+strconv.Itoa(len(args)))
		}
	}
	return strings.Join(conditions, " AND "), args
}

func unreadSQL(admin bool, viewerPlaceholder string) string {
	role := "admin"
	if admin {
		role = "user"
	}
	return `(SELECT COUNT(*) FROM support_ticket_messages um LEFT JOIN support_ticket_reads ur ON ur.ticket_id = t.id AND ur.user_id = ` + viewerPlaceholder + ` WHERE um.ticket_id = t.id AND um.id > COALESCE(ur.last_message_id, 0) AND um.kind = 'message' AND um.sender_type = '` + role + `')`
}

func scanTicket(scanner interface{ Scan(...any) error }) (service.Ticket, error) {
	var ticket service.Ticket
	err := scanner.Scan(
		&ticket.ID, &ticket.UserID, &ticket.Subject, &ticket.Description, &ticket.Contact, &ticket.Category, &ticket.Priority,
		&ticket.Status, &ticket.OrderID, &ticket.AssignedTo, &ticket.FirstResponseDueAt, &ticket.ResolutionDueAt,
		&ticket.LastMessageID, &ticket.LastMessagePreview, &ticket.LastMessageAt, &ticket.UnreadCount, &ticket.UserName, &ticket.UserEmail,
		&ticket.CreatedAt, &ticket.UpdatedAt,
	)
	return ticket, err
}

func slaDue(now time.Time, priority string) (time.Time, time.Time) {
	switch priority {
	case service.TicketPriorityUrgent:
		return now.Add(15 * time.Minute), now.Add(4 * time.Hour)
	case service.TicketPriorityHigh:
		return now.Add(time.Hour), now.Add(8 * time.Hour)
	case service.TicketPriorityLow:
		return now.Add(8 * time.Hour), now.Add(72 * time.Hour)
	default:
		return now.Add(4 * time.Hour), now.Add(24 * time.Hour)
	}
}

func insertEvent(ctx context.Context, tx *sql.Tx, ticketID, actorID int64, eventType string, data map[string]any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO support_ticket_messages (ticket_id, sender_id, sender_type, kind, body, event_type, event_data) VALUES ($1,$2,'system','event',$3,$4,$5)`, ticketID, actorID, eventType, eventType, encoded)
	return err
}

func notFoundTicket() error {
	return ticketError("NOT_FOUND", "ticket not found")
}

func ticketError(reason, message string) error {
	return infraError(reason, message)
}

func infraError(reason, message string) error {
	return &ticketApplicationError{reason: reason, message: message}
}

type ticketApplicationError struct {
	reason  string
	message string
}

func (e *ticketApplicationError) Error() string  { return e.message }
func (e *ticketApplicationError) Reason() string { return e.reason }
