package identity

import (
	"context"

	"github.com/google/uuid"
	"github.com/pulselog/pulselog/internal/auth"
)

type OrgMembership struct {
	Org  Org       `json:"org"`
	Role auth.Role `json:"role"`
}

func (s *Store) ListOrgs(ctx context.Context, userID uuid.UUID) ([]OrgMembership, error) {
	rows, err := s.pool.Query(ctx, `
SELECT o.id, o.name, o.slug, m.role
FROM memberships m
JOIN organizations o ON o.id = m.org_id
WHERE m.user_id = $1
ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrgMembership
	for rows.Next() {
		var item OrgMembership
		var role string
		if err := rows.Scan(&item.Org.ID, &item.Org.Name, &item.Org.Slug, &role); err != nil {
			return nil, err
		}
		item.Role = auth.Role(role)
		out = append(out, item)
	}
	return out, rows.Err()
}
