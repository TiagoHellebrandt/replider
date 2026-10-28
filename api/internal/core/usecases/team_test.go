package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ═══════════════════════════ ListTeamMembersUseCase ═══════════════════════════

func TestListTeamMembers_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	role := "admin"
	users := []*domain.User{
		{ID: "u1", AccountID: "acc-1", Name: "Alice", Email: "alice@test.com", Role: &role, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "u2", AccountID: "acc-1", Name: "Bob", Email: "bob@test.com", Role: nil, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	userRepo.On("ListByAccountID", ctx, "acc-1").Return(users, nil)

	uc := NewListTeamMembersUseCase(userRepo)
	members, err := uc.Execute(ctx, "acc-1")

	assert.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Equal(t, "Alice", members[0].Name)
	assert.Equal(t, "admin", members[0].Role)
	assert.Equal(t, "Bob", members[1].Name)
	assert.Equal(t, "attendant", members[1].Role) // nil role defaults to attendant
	userRepo.AssertExpectations(t)
}

func TestListTeamMembers_EmptyList(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{}, nil)

	uc := NewListTeamMembersUseCase(userRepo)
	members, err := uc.Execute(ctx, "acc-1")

	assert.NoError(t, err)
	assert.Len(t, members, 0)
	userRepo.AssertExpectations(t)
}

func TestListTeamMembers_RepoError(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	userRepo.On("ListByAccountID", ctx, "acc-1").Return(([]*domain.User)(nil), errors.New("db error"))

	uc := NewListTeamMembersUseCase(userRepo)
	members, err := uc.Execute(ctx, "acc-1")

	assert.Error(t, err)
	assert.Nil(t, members)
	userRepo.AssertExpectations(t)
}

// ═══════════════════════════ InviteTeamMemberUseCase ═══════════════════════════

func TestInviteTeamMember_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	userRepo.On("GetByEmail", ctx, "new@test.com").Return(nil, nil)
	userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Email == "new@test.com" &&
			u.Name == "New User" &&
			u.AccountID == "acc-1" &&
			*u.Role == "attendant" &&
			u.IsEmailVerified == true &&
			u.PasswordHash != nil &&
			len(u.ID) == 26
	})).Return(nil)
	userRepo.On("SetResetToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	userRepo.On("MarkInviteSent", ctx, mock.AnythingOfType("string")).Return(nil)
	emailSvc.On("SendTeamInviteEmail", "new@test.com", "Admin User", mock.AnythingOfType("string"), "https://app.replider.com", mock.AnythingOfType("string")).Return(nil)

	uc := NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin User",
		Name:        "New User",
		Email:       "new@test.com",
		Role:        "attendant",
	})

	assert.NoError(t, err)
	assert.NotNil(t, member)
	assert.Equal(t, "New User", member.Name)
	assert.Equal(t, "new@test.com", member.Email)
	assert.Equal(t, "attendant", member.Role)
	assert.True(t, member.IsActive)
	userRepo.AssertExpectations(t)
	emailSvc.AssertExpectations(t)
}

func TestInviteTeamMember_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	role := "admin"
	userRepo.On("GetByEmail", ctx, "existing@test.com").Return(&domain.User{
		ID:    "u1",
		Email: "existing@test.com",
		Role:  &role,
	}, nil)

	uc := NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin",
		Name:        "Dup User",
		Email:       "existing@test.com",
		Role:        "attendant",
	})

	assert.Nil(t, member)
	assert.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
	userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestInviteTeamMember_InvalidRole(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	uc := NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin",
		Name:        "User",
		Email:       "user@test.com",
		Role:        "superadmin",
	})

	assert.Nil(t, member)
	assert.ErrorIs(t, err, domain.ErrInvalidRole)
}

func TestInviteTeamMember_RepoError(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	userRepo.On("GetByEmail", ctx, "new@test.com").Return(nil, nil)
	userRepo.On("Create", ctx, mock.Anything).Return(errors.New("db error"))

	uc := NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin",
		Name:        "User",
		Email:       "new@test.com",
		Role:        "attendant",
	})

	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Equal(t, "db error", err.Error())
	userRepo.AssertExpectations(t)
}

func TestInviteTeamMember_EmailServiceError_StillSucceeds(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	emailSvc := new(MockEmailService)

	userRepo.On("GetByEmail", ctx, "new@test.com").Return(nil, nil)
	userRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("SetResetToken", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	userRepo.On("MarkInviteSent", ctx, mock.AnythingOfType("string")).Return(nil)
	emailSvc.On("SendTeamInviteEmail", "new@test.com", "Admin", mock.AnythingOfType("string"), "https://app.replider.com", mock.AnythingOfType("string")).Return(errors.New("smtp error"))

	uc := NewInviteTeamMemberUseCase(userRepo, nil, nil, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin",
		Name:        "New User",
		Email:       "new@test.com",
		Role:        "admin",
	})

	// Email failure is non-fatal; user is still created
	assert.NoError(t, err)
	assert.NotNil(t, member)
	assert.Equal(t, "New User", member.Name)
	userRepo.AssertExpectations(t)
}

// ═══════════════════════════ UpdateMemberRoleUseCase ═══════════════════════════

func TestUpdateMemberRole_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	role := "attendant"
	userRepo.On("GetByID", ctx, "u2").Return(&domain.User{
		ID:        "u2",
		AccountID: "acc-1",
		Role:      &role,
	}, nil)
	userRepo.On("UpdateRole", ctx, "u2", "admin").Return(nil)

	uc := NewUpdateMemberRoleUseCase(userRepo)
	err := uc.Execute(ctx, UpdateMemberRoleDTO{
		AccountID: "acc-1",
		UserID:    "u2",
		Role:      "admin",
	})

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUpdateMemberRole_MemberNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	userRepo.On("GetByID", ctx, "u-nonexistent").Return(nil, nil)

	uc := NewUpdateMemberRoleUseCase(userRepo)
	err := uc.Execute(ctx, UpdateMemberRoleDTO{
		AccountID: "acc-1",
		UserID:    "u-nonexistent",
		Role:      "admin",
	})

	assert.ErrorIs(t, err, domain.ErrMemberNotFound)
	userRepo.AssertExpectations(t)
}

func TestUpdateMemberRole_InvalidRole(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	uc := NewUpdateMemberRoleUseCase(userRepo)
	err := uc.Execute(ctx, UpdateMemberRoleDTO{
		AccountID: "acc-1",
		UserID:    "u2",
		Role:      "superadmin",
	})

	assert.ErrorIs(t, err, domain.ErrInvalidRole)
	userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestUpdateMemberRole_WrongAccount(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	role := "attendant"
	userRepo.On("GetByID", ctx, "u2").Return(&domain.User{
		ID:        "u2",
		AccountID: "acc-other",
		Role:      &role,
	}, nil)

	uc := NewUpdateMemberRoleUseCase(userRepo)
	err := uc.Execute(ctx, UpdateMemberRoleDTO{
		AccountID: "acc-1",
		UserID:    "u2",
		Role:      "admin",
	})

	assert.ErrorIs(t, err, domain.ErrMemberNotFound)
	userRepo.AssertNotCalled(t, "UpdateRole", mock.Anything, mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

// ═══════════════════════════ RemoveMemberUseCase ═══════════════════════════

func TestRemoveMember_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	userRepo.On("GetByID", ctx, "u2").Return(&domain.User{
		ID:        "u2",
		AccountID: "acc-1",
	}, nil)
	userRepo.On("Deactivate", ctx, "u2").Return(nil)

	uc := NewRemoveMemberUseCase(userRepo)
	err := uc.Execute(ctx, RemoveMemberDTO{
		AccountID:    "acc-1",
		UserID:       "u2",
		RequestingID: "u1",
	})

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestRemoveMember_CannotRemoveLastAdmin(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	adminRole := "admin"

	// Target is an admin
	userRepo.On("GetByID", ctx, "u2").Return(&domain.User{
		ID: "u2", AccountID: "acc-1", Role: &adminRole,
	}, nil)
	// GetAdminByAccountID returns the SAME user → it's the only admin
	userRepo.On("GetAdminByAccountID", ctx, "acc-1").Return(&domain.User{
		ID: "u2", AccountID: "acc-1", Role: &adminRole,
	}, nil)

	uc := NewRemoveMemberUseCase(userRepo)
	err := uc.Execute(ctx, RemoveMemberDTO{
		AccountID:    "acc-1",
		UserID:       "u2",
		RequestingID: "u1",
	})

	assert.ErrorIs(t, err, domain.ErrCannotRemoveLastAdmin)
	userRepo.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestRemoveMember_CanRemoveAdminWhenAnotherExists(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	adminRole := "admin"

	// Target is an admin
	userRepo.On("GetByID", ctx, "u2").Return(&domain.User{
		ID: "u2", AccountID: "acc-1", Role: &adminRole,
	}, nil)
	// GetAdminByAccountID returns a DIFFERENT user → there's another admin
	userRepo.On("GetAdminByAccountID", ctx, "acc-1").Return(&domain.User{
		ID: "u1", AccountID: "acc-1", Role: &adminRole,
	}, nil)
	userRepo.On("Deactivate", ctx, "u2").Return(nil)

	uc := NewRemoveMemberUseCase(userRepo)
	err := uc.Execute(ctx, RemoveMemberDTO{
		AccountID:    "acc-1",
		UserID:       "u2",
		RequestingID: "u1",
	})

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestRemoveMember_CannotRemoveSelf(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	uc := NewRemoveMemberUseCase(userRepo)
	err := uc.Execute(ctx, RemoveMemberDTO{
		AccountID:    "acc-1",
		UserID:       "u1",
		RequestingID: "u1",
	})

	assert.ErrorIs(t, err, domain.ErrCannotRemoveSelf)
	userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestRemoveMember_MemberNotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	userRepo.On("GetByID", ctx, "u-nonexistent").Return(nil, nil)

	uc := NewRemoveMemberUseCase(userRepo)
	err := uc.Execute(ctx, RemoveMemberDTO{
		AccountID:    "acc-1",
		UserID:       "u-nonexistent",
		RequestingID: "u1",
	})

	assert.ErrorIs(t, err, domain.ErrMemberNotFound)
	userRepo.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestRemoveMember_WrongAccount(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)

	userRepo.On("GetByID", ctx, "u2").Return(&domain.User{
		ID:        "u2",
		AccountID: "acc-other",
	}, nil)

	uc := NewRemoveMemberUseCase(userRepo)
	err := uc.Execute(ctx, RemoveMemberDTO{
		AccountID:    "acc-1",
		UserID:       "u2",
		RequestingID: "u1",
	})

	assert.ErrorIs(t, err, domain.ErrMemberNotFound)
	userRepo.AssertNotCalled(t, "Deactivate", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestInviteTeamMember_SeatsLimitEnforced(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	emailSvc := new(MockEmailService)

	planID := "plan_starter"
	accountRepo.On("GetByID", ctx, "acc-1").Return(&domain.Account{
		ID: "acc-1", PlanID: &planID,
	}, nil)
	planRepo.On("GetByID", ctx, planID).Return(&domain.Plan{
		ID: planID, MaxSeats: 1,
	}, nil)
	// Account already has 1 active user = at cap.
	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{
		{ID: "u1", IsActive: true},
	}, nil)

	uc := NewInviteTeamMemberUseCase(userRepo, accountRepo, planRepo, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin",
		Name:        "Second Seat",
		Email:       "new@test.com",
		Role:        "attendant",
	})

	assert.Nil(t, member)
	appErr, ok := err.(*domain.AppError)
	assert.True(t, ok, "expected *domain.AppError")
	assert.Equal(t, "PLAN_LIMIT_EXCEEDED", appErr.Code)
	assert.Equal(t, "seats", appErr.Metadata["resource"])
	assert.Equal(t, 1, appErr.Metadata["current"])
	userRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestInviteTeamMember_DeactivatedUsersDoNotCount(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepo)
	accountRepo := new(MockAccountRepo)
	planRepo := new(MockPlanRepo)
	emailSvc := new(MockEmailService)

	planID := "plan_starter"
	accountRepo.On("GetByID", ctx, "acc-1").Return(&domain.Account{
		ID: "acc-1", PlanID: &planID,
	}, nil)
	planRepo.On("GetByID", ctx, planID).Return(&domain.Plan{
		ID: planID, MaxSeats: 2,
	}, nil)
	// 1 active + 1 deactivated = 1 seat used, so invite should pass.
	userRepo.On("ListByAccountID", ctx, "acc-1").Return([]*domain.User{
		{ID: "u1", IsActive: true},
		{ID: "u2", IsActive: false},
	}, nil)
	userRepo.On("GetByEmail", ctx, "new@test.com").Return(nil, nil)
	userRepo.On("Create", ctx, mock.Anything).Return(nil)
	userRepo.On("SetResetToken", ctx, mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
	userRepo.On("MarkInviteSent", ctx, mock.AnythingOfType("string")).Return(nil)
	emailSvc.On("SendTeamInviteEmail", "new@test.com", "Admin",
		mock.AnythingOfType("string"), "https://app.replider.com",
		mock.AnythingOfType("string")).Return(nil)

	uc := NewInviteTeamMemberUseCase(userRepo, accountRepo, planRepo, emailSvc, "https://app.replider.com")
	member, err := uc.Execute(ctx, InviteTeamMemberDTO{
		AccountID:   "acc-1",
		InviterName: "Admin",
		Name:        "New Seat",
		Email:       "new@test.com",
		Role:        "attendant",
	})

	assert.NoError(t, err)
	assert.NotNil(t, member)
}
