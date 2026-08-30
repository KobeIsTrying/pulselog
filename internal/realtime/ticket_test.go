package realtime

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestMemoryTicketRedeemOnce(t *testing.T) {
	store := &MemoryTickets{}
	id, err := store.Issue(context.Background(), Ticket{
		UserID:     uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		ProjectIDs: []uuid.UUID{uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Redeem(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Redeem(context.Background(), id); err == nil {
		t.Fatal("expected one-time ticket")
	}
}
