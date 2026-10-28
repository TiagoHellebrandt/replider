package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// --- OAuthClientRepository ---

type OAuthClientRepositoryPG struct {
	queries *Queries
}

func NewOAuthClientRepository(db DBTX) *OAuthClientRepositoryPG {
	return &OAuthClientRepositoryPG{queries: New(db)}
}

func (r *OAuthClientRepositoryPG) Create(ctx context.Context, client *domain.OAuthClient) error {
	q := getQueries(ctx, r.queries)
	return q.CreateOAuthClient(ctx, CreateOAuthClientParams{
		ID:                      client.ID,
		ClientSecretHash:        client.ClientSecretHash,
		TokenEndpointAuthMethod: client.TokenEndpointAuthMethod,
		ClientName:              client.ClientName,
		RedirectUris:            toBytes(client.RedirectURIs),
		GrantTypes:              toBytes(client.GrantTypes),
		Scope:                   client.Scope,
		LogoUri:                 client.LogoURI,
	})
}

func (r *OAuthClientRepositoryPG) GetByID(ctx context.Context, id string) (*domain.OAuthClient, error) {
	q := getQueries(ctx, r.queries)
	row, err := q.GetOAuthClient(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var redirectURIs, grantTypes []string
	_ = json.Unmarshal(row.RedirectUris, &redirectURIs)
	_ = json.Unmarshal(row.GrantTypes, &grantTypes)
	return &domain.OAuthClient{
		ID:                      row.ID,
		ClientSecretHash:        row.ClientSecretHash,
		TokenEndpointAuthMethod: row.TokenEndpointAuthMethod,
		ClientName:              row.ClientName,
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		Scope:                   row.Scope,
		LogoURI:                 row.LogoUri,
		CreatedAt:               row.CreatedAt.Time,
	}, nil
}

func (r *OAuthClientRepositoryPG) TouchLastUsed(ctx context.Context, id string) error {
	q := getQueries(ctx, r.queries)
	return q.TouchOAuthClientLastUsed(ctx, id)
}
