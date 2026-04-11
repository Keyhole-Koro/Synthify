package postgres

import "github.com/synthify/backend/internal/domain"

func (s *Store) ListWorkspaces() []*domain.Workspace {
	rows, err := s.db.Query(`
		SELECT workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at
		FROM workspaces
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var workspaces []*domain.Workspace
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err == nil {
			workspaces = append(workspaces, workspace)
		}
	}
	return workspaces
}

func (s *Store) GetWorkspace(id string) (*domain.Workspace, []*domain.WorkspaceMember, bool) {
	row := s.db.QueryRow(`
		SELECT workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at
		FROM workspaces
		WHERE workspace_id = $1
	`, id)
	workspace, err := scanWorkspace(row)
	if err != nil {
		return nil, nil, false
	}

	rows, err := s.db.Query(`
		SELECT user_id, email, role, is_dev, invited_at, invited_by
		FROM workspace_members
		WHERE workspace_id = $1
		ORDER BY invited_at ASC
	`, id)
	if err != nil {
		return nil, nil, false
	}
	defer rows.Close()

	var members []*domain.WorkspaceMember
	for rows.Next() {
		member, err := scanWorkspaceMember(rows)
		if err == nil {
			members = append(members, member)
		}
	}
	return workspace, members, true
}

func (s *Store) CreateWorkspace(name string) *domain.Workspace {
	workspace := &domain.Workspace{
		WorkspaceID:       newID("ws"),
		Name:              name,
		OwnerID:           "user_demo",
		Plan:              "free",
		StorageUsedBytes:  0,
		StorageQuotaBytes: 1 << 30,
		MaxFileSizeBytes:  50 << 20,
		MaxUploadsPerDay:  10,
		CreatedAt:         now(),
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO workspaces (workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, workspace.WorkspaceID, workspace.Name, workspace.OwnerID, workspace.Plan, workspace.StorageUsedBytes, workspace.StorageQuotaBytes, workspace.MaxFileSizeBytes, workspace.MaxUploadsPerDay, workspace.CreatedAt); err != nil {
		return nil
	}
	if _, err := tx.Exec(`
		INSERT INTO workspace_members (workspace_id, user_id, email, role, is_dev, invited_at, invited_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, workspace.WorkspaceID, "user_demo", "demo@synthify.dev", "owner", true, workspace.CreatedAt, "user_demo"); err != nil {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return nil
	}
	return workspace
}

func (s *Store) InviteMember(wsID, email, role string, isDev bool) (*domain.WorkspaceMember, bool) {
	member := &domain.WorkspaceMember{
		UserID:    newID("user"),
		Email:     email,
		Role:      domain.WorkspaceRole(role),
		IsDev:     isDev,
		InvitedAt: now(),
		InvitedBy: "user_demo",
	}
	_, err := s.db.Exec(`
		INSERT INTO workspace_members (workspace_id, user_id, email, role, is_dev, invited_at, invited_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, wsID, member.UserID, member.Email, member.Role, member.IsDev, member.InvitedAt, member.InvitedBy)
	return member, err == nil
}

func (s *Store) UpdateMemberRole(wsID, userID, role string, isDev bool) (*domain.WorkspaceMember, bool) {
	res, err := s.db.Exec(`
		UPDATE workspace_members
		SET role = $3, is_dev = $4
		WHERE workspace_id = $1 AND user_id = $2
	`, wsID, userID, role, isDev)
	if err != nil {
		return nil, false
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, false
	}
	row := s.db.QueryRow(`
		SELECT user_id, email, role, is_dev, invited_at, invited_by
		FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2
	`, wsID, userID)
	member, err := scanWorkspaceMember(row)
	return member, err == nil
}

func (s *Store) RemoveMember(wsID, userID string) bool {
	res, err := s.db.Exec(`DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, wsID, userID)
	if err != nil {
		return false
	}
	affected, _ := res.RowsAffected()
	return affected > 0
}

func (s *Store) TransferOwnership(wsID, newOwnerUserID string) (*domain.Workspace, []*domain.WorkspaceMember, bool) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, false
	}
	defer tx.Rollback()

	var oldOwner string
	if err := tx.QueryRow(`SELECT owner_id FROM workspaces WHERE workspace_id = $1`, wsID).Scan(&oldOwner); err != nil {
		return nil, nil, false
	}
	res, err := tx.Exec(`UPDATE workspace_members SET role = 'owner' WHERE workspace_id = $1 AND user_id = $2`, wsID, newOwnerUserID)
	if err != nil {
		return nil, nil, false
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, nil, false
	}
	if _, err := tx.Exec(`UPDATE workspace_members SET role = 'editor' WHERE workspace_id = $1 AND user_id = $2`, wsID, oldOwner); err != nil {
		return nil, nil, false
	}
	if _, err := tx.Exec(`UPDATE workspaces SET owner_id = $2 WHERE workspace_id = $1`, wsID, newOwnerUserID); err != nil {
		return nil, nil, false
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, false
	}
	return s.GetWorkspace(wsID)
}
