package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/zammad"
)

const (
	ZammadIdentityStateActive   = "ACTIVE"
	ZammadIdentityStateConflict = "CONFLICT"
)

type ZammadIdentityMapping struct {
	ID             int64
	Sub2APIUserID  int64
	ZammadUserID   *int64
	Email          string
	SyncState      string
	ConflictReason string
	LastSyncedAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ZammadIdentityMappingRepository interface {
	GetBySub2APIUserID(context.Context, int64) (*ZammadIdentityMapping, error)
	Upsert(context.Context, *ZammadIdentityMapping) (*ZammadIdentityMapping, error)
	MarkConflict(context.Context, int64, string, string) error
}

type ZammadIdentityConflictError struct {
	Decision zammad.IdentityDecision
	Reason   string
}

func (e *ZammadIdentityConflictError) Error() string {
	if e == nil || e.Reason == "" {
		return "zammad identity conflict"
	}
	return fmt.Sprintf("zammad identity conflict (%s): %s", e.Decision, e.Reason)
}

type ZammadIdentityService struct {
	provider zammad.IdentityProvider
	mappings ZammadIdentityMappingRepository
}

func NewZammadIdentityService(provider zammad.IdentityProvider, mappings ZammadIdentityMappingRepository) *ZammadIdentityService {
	return &ZammadIdentityService{provider: provider, mappings: mappings}
}

// EnsureIdentity resolves and persists the Zammad customer used by a ticket.
// A conflict is persisted before it is returned so administrators can inspect
// the reason without retrying an unsafe external write.
func (s *ZammadIdentityService) EnsureIdentity(ctx context.Context, sub2APIUserID int64, email string) (*zammad.User, error) {
	if s == nil || s.provider == nil || s.mappings == nil {
		return nil, errors.New("zammad identity service is not configured")
	}
	if sub2APIUserID <= 0 {
		return nil, errors.New("sub2api user id must be positive")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("sub2api user email is required")
	}
	mapping, err := s.mappings.GetBySub2APIUserID(ctx, sub2APIUserID)
	if err != nil {
		return nil, fmt.Errorf("load zammad identity mapping: %w", err)
	}
	var identityMapping *zammad.IdentityMapping
	if mapping != nil && mapping.ZammadUserID != nil {
		identityMapping = &zammad.IdentityMapping{
			Sub2APIUserID: sub2APIUserID,
			ZammadUserID:  *mapping.ZammadUserID,
			Email:         mapping.Email,
		}
	}
	decision, err := zammad.ResolveIdentity(ctx, s.provider, identityMapping, email)
	if err != nil {
		return nil, fmt.Errorf("resolve zammad identity: %w", err)
	}
	switch decision.Decision {
	case zammad.IdentityCreate:
		user, createErr := s.provider.CreateUser(ctx, zammad.CreateUserRequest{Email: email, Login: email, Active: true})
		if createErr != nil {
			return nil, fmt.Errorf("create zammad user: %w", createErr)
		}
		return user, s.persistActiveMapping(ctx, sub2APIUserID, email, user)
	case zammad.IdentityLinkExisting, zammad.IdentityReuse:
		return decision.User, s.persistActiveMapping(ctx, sub2APIUserID, email, decision.User)
	case zammad.IdentityUpdateEmail:
		user, updateErr := s.provider.UpdateUserEmail(ctx, decision.User.ID, email)
		if updateErr != nil {
			return nil, fmt.Errorf("update zammad user email: %w", updateErr)
		}
		return user, s.persistActiveMapping(ctx, sub2APIUserID, email, user)
	default:
		if conflictErr := s.mappings.MarkConflict(ctx, sub2APIUserID, email, decision.Reason); conflictErr != nil {
			return nil, fmt.Errorf("persist zammad identity conflict: %w", conflictErr)
		}
		return nil, &ZammadIdentityConflictError{Decision: decision.Decision, Reason: decision.Reason}
	}
}

func (s *ZammadIdentityService) persistActiveMapping(ctx context.Context, sub2APIUserID int64, email string, user *zammad.User) error {
	if user == nil || user.ID <= 0 {
		return errors.New("zammad user response is missing id")
	}
	_, err := s.mappings.Upsert(ctx, &ZammadIdentityMapping{
		Sub2APIUserID: sub2APIUserID,
		ZammadUserID:  &user.ID,
		Email:         email,
		SyncState:     ZammadIdentityStateActive,
	})
	if err != nil {
		return fmt.Errorf("persist zammad identity mapping: %w", err)
	}
	return nil
}
