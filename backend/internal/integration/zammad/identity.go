package zammad

import (
	"context"
	"errors"
	"strings"
)

type User struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Active   bool   `json:"active"`
	Username string `json:"login"`
}

type CreateUserRequest struct {
	Email     string `json:"email"`
	Login     string `json:"login"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Active    bool   `json:"active"`
}

type IdentityMapping struct {
	Sub2APIUserID int64
	ZammadUserID  int64
	Email         string
}

type IdentityDecision string

const (
	IdentityCreate              IdentityDecision = "CREATE"
	IdentityLinkExisting        IdentityDecision = "LINK_EXISTING"
	IdentityReuse               IdentityDecision = "REUSE"
	IdentityUpdateEmail         IdentityDecision = "UPDATE_EMAIL"
	IdentityBlockConflict       IdentityDecision = "BLOCK_EMAIL_CONFLICT"
	IdentityBlockDisabled       IdentityDecision = "BLOCK_DISABLED_USER"
	IdentityBlockMissingMapping IdentityDecision = "BLOCK_MISSING_MAPPING"
)

type IdentityResult struct {
	Decision IdentityDecision
	User     *User
	Reason   string
}

// ResolveIdentity performs the read-only part of identity resolution. Writes
// such as creating a user or changing an email remain explicit operations in
// the ticket service so a caller can persist the mapping in the same workflow.
func ResolveIdentity(ctx context.Context, provider IdentityProvider, mapping *IdentityMapping, currentEmail string) (IdentityResult, error) {
	if provider == nil {
		return IdentityResult{}, errors.New("nil zammad identity provider")
	}
	emailMatches, err := provider.SearchUsersByEmail(ctx, currentEmail)
	if err != nil {
		return IdentityResult{}, err
	}
	if mapping == nil {
		return EvaluateIdentity(nil, currentEmail, emailMatches, nil), nil
	}
	mappedUser, err := provider.GetUser(ctx, mapping.ZammadUserID)
	if err != nil {
		return IdentityResult{}, err
	}
	return EvaluateIdentity(mapping, currentEmail, emailMatches, mappedUser), nil
}

// EvaluateIdentity defines the email edge policy before any Zammad write occurs.
// A mapping is anchored to the Sub2API user ID; email matches never replace it.
func EvaluateIdentity(mapping *IdentityMapping, currentEmail string, emailMatches []User, mappedUser *User) IdentityResult {
	currentEmail = normalizeEmail(currentEmail)
	if currentEmail == "" {
		return IdentityResult{Decision: IdentityBlockConflict, Reason: "sub2api user has no usable email"}
	}
	if mapping == nil {
		switch len(emailMatches) {
		case 0:
			return IdentityResult{Decision: IdentityCreate}
		case 1:
			if !emailMatches[0].Active {
				return IdentityResult{Decision: IdentityBlockDisabled, User: &emailMatches[0], Reason: "matching Zammad user is disabled"}
			}
			return IdentityResult{Decision: IdentityLinkExisting, User: &emailMatches[0]}
		default:
			return IdentityResult{Decision: IdentityBlockConflict, Reason: "multiple Zammad users match the email"}
		}
	}
	if mappedUser == nil || mappedUser.ID != mapping.ZammadUserID {
		return IdentityResult{Decision: IdentityBlockMissingMapping, Reason: "mapped Zammad user could not be verified"}
	}
	if !mappedUser.Active {
		return IdentityResult{Decision: IdentityBlockDisabled, User: mappedUser, Reason: "mapped Zammad user is disabled"}
	}
	if normalizeEmail(mapping.Email) == currentEmail || normalizeEmail(mappedUser.Email) == currentEmail {
		return IdentityResult{Decision: IdentityReuse, User: mappedUser}
	}
	for i := range emailMatches {
		if emailMatches[i].ID != mapping.ZammadUserID {
			return IdentityResult{Decision: IdentityBlockConflict, Reason: "new email belongs to another Zammad user"}
		}
	}
	return IdentityResult{Decision: IdentityUpdateEmail, User: mappedUser, Reason: "Sub2API email changed"}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
