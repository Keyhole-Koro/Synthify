package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
)

func (s *Store) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	row, err := s.q().GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomainUser(row), nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := s.q().GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomainUser(row), nil
}

func (s *Store) UpsertUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	createdAt, _ := time.Parse(time.RFC3339, user.CreatedAt)
	if createdAt.IsZero() {
		createdAt = nowTime()
	}
	lastLoginAt, _ := time.Parse(time.RFC3339, user.LastLoginAt)
	if lastLoginAt.IsZero() {
		lastLoginAt = nowTime()
	}

	row, err := s.q().UpsertUser(ctx, sqlcgen.UpsertUserParams{
		UserID:      user.UserID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   createdAt,
		LastLoginAt: lastLoginAt,
		UpdatedAt:   nowTime(),
	})
	if err != nil {
		return nil, err
	}
	return toDomainUser(row), nil
}

func toDomainUser(row sqlcgen.User) *domain.User {
	return &domain.User{
		UserID:      row.UserID,
		Email:       row.Email,
		DisplayName: row.DisplayName,
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		LastLoginAt: row.LastLoginAt.Format(time.RFC3339),
		UpdatedAt:   row.UpdatedAt.Format(time.RFC3339),
	}
}
