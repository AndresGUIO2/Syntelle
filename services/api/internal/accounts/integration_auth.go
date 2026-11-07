package accounts

import (
	"context"
	"strings"
)

func (s *Service) resolveWorkspaceKey(ctx context.Context, authorization string) (Principal, error) {
	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	parts := strings.Split(strings.TrimPrefix(token, "syntelle_wk_"), ".")
	if len(parts) != 2 || len(parts[0]) != 43 || len(parts[1]) != 43 {
		return Principal{}, ErrUnauthorized
	}
	var expected, wid, iid string
	err := s.Pool.QueryRow(ctx, `SELECT secret_hash,workspace_id,integration_id FROM syntelle_accounts.workspace_keys WHERE id=$1 AND revoked_at IS NULL AND scope='streams' AND integration_id IS NOT NULL`, parts[0]).Scan(&expected, &wid, &iid)
	if err != nil || !same(expected, digest(token)) {
		return Principal{}, ErrUnauthorized
	}
	i, err := s.published(ctx, "id", iid)
	if err != nil || i.WorkspaceID != wid {
		return Principal{}, ErrUnauthorized
	}
	return Principal{ID: "workspace_key:" + parts[0], Kind: "workspace_key", KeyID: parts[0], WorkspaceID: wid, IntegrationID: iid, PublicationID: i.PublicationID, IntegrationRevision: i.PublishedRevision}, nil
}
func (s *Service) validateIntegrationPrincipal(ctx context.Context, p Principal) error {
	if p.AccountID != "" || p.WorkspaceID == "" || p.IntegrationID == "" {
		return ErrUnauthorized
	}
	if p.Kind == "workspace_key" {
		if p.KeyID == "" || p.ID != "workspace_key:"+p.KeyID {
			return ErrUnauthorized
		}
		var valid bool
		err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_accounts.workspace_keys WHERE id=$1 AND workspace_id=$2 AND integration_id=$3 AND revoked_at IS NULL AND scope='streams')`, p.KeyID, p.WorkspaceID, p.IntegrationID).Scan(&valid)
		if err != nil || !valid {
			return ErrUnauthorized
		}
	} else if p.Kind != "widget" || p.VisitorID == "" || p.ID != "widget:"+p.VisitorID {
		return ErrUnauthorized
	}
	i, err := s.published(ctx, "id", p.IntegrationID)
	if err != nil || i.WorkspaceID != p.WorkspaceID || i.PublishedRevision != p.IntegrationRevision || i.PublicationID != p.PublicationID {
		return ErrUnauthorized
	}
	return nil
}
