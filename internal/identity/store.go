package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulselog/pulselog/internal/auth"
	"github.com/pulselog/pulselog/internal/models"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidCreds = errors.New("invalid credentials")
	ErrRevoked      = errors.New("revoked")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}

type Org struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type Project struct {
	ID    uuid.UUID `json:"id"`
	OrgID uuid.UUID `json:"org_id"`
	Name  string    `json:"name"`
	Slug  string    `json:"slug"`
}

type Service struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name      string    `json:"name"`
}

type Member struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   auth.Role `json:"role"`
}

type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  uuid.UUID  `json:"project_id"`
	ServiceID  uuid.UUID  `json:"service_id"`
	Service    string     `json:"service"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type IngestKey struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	OrgID     uuid.UUID
	Service   string
}

type Principal struct {
	UserID     uuid.UUID
	Email      string
	JTI        string
	TokenExp   time.Time
	OrgRoles   map[uuid.UUID]auth.Role
	ProjectIDs []uuid.UUID
}

func (p *Principal) RoleForOrg(orgID uuid.UUID) (auth.Role, bool) {
	r, ok := p.OrgRoles[orgID]
	return r, ok
}

func (p *Principal) Can(orgID uuid.UUID, perm auth.Permission) bool {
	role, ok := p.OrgRoles[orgID]
	return ok && auth.HasPermission(role, perm)
}

func (p *Principal) HasProject(id uuid.UUID) bool {
	for _, pID := range p.ProjectIDs {
		if pID == id {
			return true
		}
	}
	return false
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen && b.Len() > 0 {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "org"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Store) Register(ctx context.Context, email, password, orgName string) (User, Org, Project, error) {
	email = NormalizeEmail(email)
	if email == "" || !strings.Contains(email, "@") {
		return User{}, Org{}, Project{}, fmt.Errorf("%w: invalid email", ErrInvalidCreds)
	}
	if err := auth.ValidatePassword(password); err != nil {
		return User{}, Org{}, Project{}, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return User{}, Org{}, Project{}, err
	}
	orgName = strings.TrimSpace(orgName)
	if orgName == "" {
		orgName = "Organization"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, Org{}, Project{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var user User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email, password_hash`,
		email, hash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		if isUnique(err) {
			return User{}, Org{}, Project{}, ErrConflict
		}
		return User{}, Org{}, Project{}, err
	}
	slug := slugify(orgName)
	var org Org
	err = tx.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id, name, slug`,
		orgName, uniqueSlug(slug, user.ID),
	).Scan(&org.ID, &org.Name, &org.Slug)
	if err != nil {
		return User{}, Org{}, Project{}, err
	}
	var project Project
	err = tx.QueryRow(ctx,
		`INSERT INTO projects (org_id, name, slug) VALUES ($1, 'default', 'default') RETURNING id, org_id, name, slug`,
		org.ID,
	).Scan(&project.ID, &project.OrgID, &project.Name, &project.Slug)
	if err != nil {
		return User{}, Org{}, Project{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'owner')`,
		org.ID, user.ID,
	); err != nil {
		return User{}, Org{}, Project{}, err
	}
	if err := auditTx(ctx, tx, org.ID, &user.ID, "member.added", "user", user.ID.String(), map[string]any{"role": "owner"}); err != nil {
		return User{}, Org{}, Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, Org{}, Project{}, err
	}
	return user, org, project, nil
}

func uniqueSlug(base string, id uuid.UUID) string {
	return fmt.Sprintf("%s-%s", base, id.String()[:8])
}

func (s *Store) AuthenticateUser(ctx context.Context, email, password string) (User, error) {
	email = NormalizeEmail(email)
	var user User
	err := s.pool.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidCreds
		}
		return User{}, err
	}
	if !auth.VerifyPassword(password, user.PasswordHash) {
		return User{}, ErrInvalidCreds
	}
	return user, nil
}

func (s *Store) LoadPrincipal(ctx context.Context, userID uuid.UUID) (*Principal, error) {
	var email string
	if err := s.pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p := &Principal{
		UserID:   userID,
		Email:    email,
		OrgRoles: map[uuid.UUID]auth.Role{},
	}
	rows, err := s.pool.Query(ctx, `SELECT org_id, role FROM memberships WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orgs := make([]uuid.UUID, 0)
	for rows.Next() {
		var orgID uuid.UUID
		var role string
		if err := rows.Scan(&orgID, &role); err != nil {
			return nil, err
		}
		p.OrgRoles[orgID] = auth.Role(role)
		orgs = append(orgs, orgID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(orgs) == 0 {
		return p, nil
	}
	prows, err := s.pool.Query(ctx, `SELECT id FROM projects WHERE org_id = ANY($1)`, orgs)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var id uuid.UUID
		if err := prows.Scan(&id); err != nil {
			return nil, err
		}
		p.ProjectIDs = append(p.ProjectIDs, id)
	}
	return p, prows.Err()
}

func (s *Store) CreateProject(ctx context.Context, orgID uuid.UUID, name string) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, fmt.Errorf("name is required")
	}
	slug := slugify(name)
	var p Project
	err := s.pool.QueryRow(ctx,
		`INSERT INTO projects (org_id, name, slug) VALUES ($1, $2, $3) RETURNING id, org_id, name, slug`,
		orgID, name, slug,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug)
	if err != nil {
		if isUnique(err) {
			return Project{}, ErrConflict
		}
		return Project{}, err
	}
	return p, nil
}

func (s *Store) Project(ctx context.Context, id uuid.UUID) (Project, error) {
	var p Project
	err := s.pool.QueryRow(ctx, `SELECT id, org_id, name, slug FROM projects WHERE id = $1`, id).
		Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

func (s *Store) ListProjects(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, org_id, name, slug FROM projects WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateService(ctx context.Context, projectID uuid.UUID, name string) (Service, error) {
	name = strings.TrimSpace(name)
	if !models.IsValidServiceName(name) {
		return Service{}, fmt.Errorf("invalid service name")
	}
	var svc Service
	err := s.pool.QueryRow(ctx,
		`INSERT INTO services (project_id, name) VALUES ($1, $2) RETURNING id, project_id, name`,
		projectID, name,
	).Scan(&svc.ID, &svc.ProjectID, &svc.Name)
	if err != nil {
		if isUnique(err) {
			return Service{}, ErrConflict
		}
		return Service{}, err
	}
	return svc, nil
}

func (s *Store) ListServices(ctx context.Context, projectID uuid.UUID) ([]Service, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, project_id, name FROM services WHERE project_id = $1 ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Service
	for rows.Next() {
		var svc Service
		if err := rows.Scan(&svc.ID, &svc.ProjectID, &svc.Name); err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

func (s *Store) ServiceByName(ctx context.Context, projectID uuid.UUID, name string) (Service, error) {
	var svc Service
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, name FROM services WHERE project_id = $1 AND name = $2`,
		projectID, name,
	).Scan(&svc.ID, &svc.ProjectID, &svc.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Service{}, ErrNotFound
	}
	return svc, err
}

func (s *Store) AddMember(ctx context.Context, orgID, actor, userID uuid.UUID, role auth.Role) error {
	if _, ok := auth.ParseRole(string(role)); !ok {
		return fmt.Errorf("invalid role")
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, $3)`,
		orgID, userID, string(role),
	)
	if err != nil {
		if isUnique(err) {
			return ErrConflict
		}
		return err
	}
	return s.Audit(ctx, orgID, &actor, "member.added", "user", userID.String(), map[string]any{"role": role})
}

func (s *Store) UpdateMemberRole(ctx context.Context, orgID, actor, userID uuid.UUID, role auth.Role) error {
	if _, ok := auth.ParseRole(string(role)); !ok {
		return fmt.Errorf("invalid role")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE memberships SET role = $1 WHERE org_id = $2 AND user_id = $3`,
		string(role), orgID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return s.Audit(ctx, orgID, &actor, "role.changed", "user", userID.String(), map[string]any{"role": role})
}

func (s *Store) RemoveMember(ctx context.Context, orgID, actor, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM memberships WHERE org_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return s.Audit(ctx, orgID, &actor, "member.removed", "user", userID.String(), nil)
}

func (s *Store) ListMembers(ctx context.Context, orgID uuid.UUID) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
SELECT m.user_id, u.email, m.role
FROM memberships m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1 ORDER BY u.email`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		var role string
		if err := rows.Scan(&m.UserID, &m.Email, &role); err != nil {
			return nil, err
		}
		m.Role = auth.Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE email = $1`, NormalizeEmail(email)).
		Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateAPIKey(ctx context.Context, projectID, serviceID, actor uuid.UUID, name string) (APIKey, string, error) {
	raw, prefix, hash, err := auth.GenerateAPIKey()
	if err != nil {
		return APIKey{}, "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	var key APIKey
	err = s.pool.QueryRow(ctx, `
INSERT INTO api_keys (project_id, service_id, name, prefix, key_hash, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, service_id, name, prefix, created_at`,
		projectID, serviceID, name, prefix, hash, actor,
	).Scan(&key.ID, &key.ProjectID, &key.ServiceID, &key.Name, &key.Prefix, &key.CreatedAt)
	if err != nil {
		return APIKey{}, "", err
	}
	var orgID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT org_id FROM projects WHERE id = $1`, projectID).Scan(&orgID); err != nil {
		return APIKey{}, "", err
	}
	if err := s.Audit(ctx, orgID, &actor, "apikey.created", "api_key", key.ID.String(), map[string]any{"prefix": prefix}); err != nil {
		return APIKey{}, "", err
	}
	return key, raw, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, projectID uuid.UUID) ([]APIKey, error) {
	rows, err := s.pool.Query(ctx, `
SELECT k.id, k.project_id, k.service_id, s.name, k.name, k.prefix, k.created_at, k.last_used_at, k.revoked_at
FROM api_keys k JOIN services s ON s.id = k.service_id
WHERE k.project_id = $1 ORDER BY k.created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.ProjectID, &k.ServiceID, &k.Service, &k.Name, &k.Prefix, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) RevokeAPIKey(ctx context.Context, keyID, actor uuid.UUID) (APIKey, error) {
	var k APIKey
	var orgID uuid.UUID
	err := s.pool.QueryRow(ctx, `
UPDATE api_keys k SET revoked_at = now()
FROM projects p
WHERE k.id = $1 AND k.revoked_at IS NULL AND p.id = k.project_id
RETURNING k.id, k.project_id, k.service_id, k.name, k.prefix, k.created_at, k.revoked_at, p.org_id`,
		keyID,
	).Scan(&k.ID, &k.ProjectID, &k.ServiceID, &k.Name, &k.Prefix, &k.CreatedAt, &k.RevokedAt, &orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrNotFound
	}
	if err != nil {
		return APIKey{}, err
	}
	if err := s.Audit(ctx, orgID, &actor, "apikey.revoked", "api_key", k.ID.String(), map[string]any{"prefix": k.Prefix}); err != nil {
		return APIKey{}, err
	}
	return k, nil
}

func (s *Store) APIKeyProject(ctx context.Context, keyID uuid.UUID) (uuid.UUID, error) {
	var projectID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT project_id FROM api_keys WHERE id = $1`, keyID).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return projectID, err
}

func (s *Store) Verify(ctx context.Context, raw string) (IngestKey, error) {
	return s.VerifyAPIKey(ctx, raw)
}

func (s *Store) VerifyAPIKey(ctx context.Context, raw string) (IngestKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return IngestKey{}, ErrNotFound
	}
	hash := auth.HashAPIKey(raw)
	var k IngestKey
	var revoked *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT k.id, k.project_id, p.org_id, s.name, k.revoked_at
FROM api_keys k
JOIN projects p ON p.id = k.project_id
JOIN services s ON s.id = k.service_id
WHERE k.key_hash = $1`, hash).Scan(&k.ID, &k.ProjectID, &k.OrgID, &k.Service, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return IngestKey{}, ErrNotFound
	}
	if err != nil {
		return IngestKey{}, err
	}
	if revoked != nil {
		return IngestKey{}, ErrRevoked
	}
	_, _ = s.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, k.ID)
	return k, nil
}

func (s *Store) RawKeyHashExists(ctx context.Context, raw string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys WHERE key_hash = $1 OR prefix = $2 OR name = $2`,
		raw, raw).Scan(&n)
	return n > 0, err
}

func (s *Store) PasswordLooksHashed(ctx context.Context, userID uuid.UUID) (bool, error) {
	var hash string
	if err := s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		return false, err
	}
	return auth.LooksHashedPassword(hash) && !strings.Contains(hash, " "), nil
}

func (s *Store) Audit(ctx context.Context, orgID uuid.UUID, actor *uuid.UUID, action, targetType, targetID string, meta map[string]any) error {
	return insertAudit(ctx, s.pool.Exec, orgID, actor, action, targetType, targetID, meta)
}

func auditTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, actor *uuid.UUID, action, targetType, targetID string, meta map[string]any) error {
	return insertAudit(ctx, tx.Exec, orgID, actor, action, targetType, targetID, meta)
}

func insertAudit(ctx context.Context, exec func(context.Context, string, ...any) (pgconn.CommandTag, error), orgID uuid.UUID, actor *uuid.UUID, action, targetType, targetID string, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	body, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = exec(ctx, `
INSERT INTO audit_events (org_id, actor_user_id, action, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6)`,
		orgID, actor, action, targetType, targetID, body)
	return err
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
