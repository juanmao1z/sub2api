package zammad

import (
	"context"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewClient,
	wire.Bind(new(TicketProvider), new(*Client)),
	wire.Bind(new(IdentityProvider), new(*Client)),
)

// TicketProvider is the boundary used by the future ticket service adapter.
// Keeping it here prevents handlers from depending on Zammad HTTP details.
type TicketProvider interface {
	Configured() bool
	Health(context.Context) error
	CreateTicket(context.Context, CreateTicketRequest) (*Ticket, error)
	GetTicket(context.Context, int64) (*Ticket, error)
	AddArticle(context.Context, int64, Article) (*Article, error)
	ListArticles(context.Context, int64) ([]Article, error)
	UpdateTicket(context.Context, int64, UpdateTicketRequest) (*Ticket, error)
	UpdateTicketStatus(context.Context, int64, TicketStatus) (*Ticket, error)
}

// IdentityProvider contains the user operations needed to resolve the
// Sub2API-to-Zammad email mapping before a ticket is created.
type IdentityProvider interface {
	SearchUsersByEmail(context.Context, string) ([]User, error)
	GetUser(context.Context, int64) (*User, error)
	CreateUser(context.Context, CreateUserRequest) (*User, error)
	UpdateUserEmail(context.Context, int64, string) (*User, error)
}
