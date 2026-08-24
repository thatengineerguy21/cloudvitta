package rest_test

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// mockQuerier implements store.Querier for REST test suites.
type mockQuerier struct {
	store.Querier
	createUserFunc                        func(ctx context.Context, arg store.CreateUserParams) (store.User, error)
	getUserByEmailFunc                    func(ctx context.Context, email string) (store.User, error)
	getUserByIDFunc                       func(ctx context.Context, id pgtype.UUID) (store.User, error)
	insertRefreshTokenFunc                func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error)
	getRefreshTokenByHashForUpdateFunc    func(ctx context.Context, tokenHash string) (store.RefreshToken, error)
	getRefreshTokenByIDFunc               func(ctx context.Context, id pgtype.UUID) (store.RefreshToken, error)
	revokeRefreshTokenWithReplacementFunc func(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error
	revokeRefreshTokenByHashFunc          func(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error
	revokeRefreshTokenFamilyFunc          func(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error
	listRefreshTokensByFamilyIDFunc       func(ctx context.Context, familyID pgtype.UUID) ([]store.RefreshToken, error)
	getProviderCategoryStatusFunc         func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error)
}

func (m *mockQuerier) CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, arg)
	}
	return store.User{}, errors.New("CreateUser not implemented")
}

func (m *mockQuerier) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	if m.getUserByEmailFunc != nil {
		return m.getUserByEmailFunc(ctx, email)
	}
	return store.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (store.User, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, id)
	}
	return store.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) InsertRefreshToken(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
	if m.insertRefreshTokenFunc != nil {
		return m.insertRefreshTokenFunc(ctx, arg)
	}
	return store.RefreshToken{
		ID:        store.UUIDToPg(store.PgToUUID(arg.UserID)),
		UserID:    arg.UserID,
		FamilyID:  arg.FamilyID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func (m *mockQuerier) GetRefreshTokenByHashForUpdate(ctx context.Context, tokenHash string) (store.RefreshToken, error) {
	if m.getRefreshTokenByHashForUpdateFunc != nil {
		return m.getRefreshTokenByHashForUpdateFunc(ctx, tokenHash)
	}
	return store.RefreshToken{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetRefreshTokenByID(ctx context.Context, id pgtype.UUID) (store.RefreshToken, error) {
	if m.getRefreshTokenByIDFunc != nil {
		return m.getRefreshTokenByIDFunc(ctx, id)
	}
	return store.RefreshToken{}, pgx.ErrNoRows
}

func (m *mockQuerier) RevokeRefreshTokenWithReplacement(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error {
	if m.revokeRefreshTokenWithReplacementFunc != nil {
		return m.revokeRefreshTokenWithReplacementFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) RevokeRefreshTokenByHash(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
	if m.revokeRefreshTokenByHashFunc != nil {
		return m.revokeRefreshTokenByHashFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) RevokeRefreshTokenFamily(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
	if m.revokeRefreshTokenFamilyFunc != nil {
		return m.revokeRefreshTokenFamilyFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) ListRefreshTokensByFamilyID(ctx context.Context, familyID pgtype.UUID) ([]store.RefreshToken, error) {
	if m.listRefreshTokensByFamilyIDFunc != nil {
		return m.listRefreshTokensByFamilyIDFunc(ctx, familyID)
	}
	return nil, nil
}

func (m *mockQuerier) GetPriceObservations(ctx context.Context, arg store.GetPriceObservationsParams) ([]store.PriceObservation, error) {
	return nil, errors.New("GetPriceObservations not implemented")
}

func (m *mockQuerier) GetLatestPriceForSKU(ctx context.Context, arg store.GetLatestPriceForSKUParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKU not implemented")
}

func (m *mockQuerier) GetLatestPriceForSKUAndCategory(ctx context.Context, arg store.GetLatestPriceForSKUAndCategoryParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKUAndCategory not implemented")
}

func (m *mockQuerier) UpdatePriceObservationLastSeenAt(ctx context.Context, arg store.UpdatePriceObservationLastSeenAtParams) error {
	return nil
}

func (m *mockQuerier) InsertPriceObservation(ctx context.Context, arg store.InsertPriceObservationParams) (int64, error) {
	return 0, errors.New("InsertPriceObservation not implemented")
}

func (m *mockQuerier) GetProviderCategoryStatus(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
	if m.getProviderCategoryStatusFunc != nil {
		return m.getProviderCategoryStatusFunc(ctx, provider)
	}
	return nil, nil
}
