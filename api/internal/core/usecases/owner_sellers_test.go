package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const testPlatformID = "01PLATFORMACCOUNT000000000"

// mockOwnerSellerRepo implements domain.OwnerSellerRepository in
// memory. InTx runs fn on a deep copy and commits it back only on
// success, mirroring the transactional contract the use case relies on.
type mockOwnerSellerRepo struct {
	users      []domain.OwnerSellerRow // identity only: UserID/Name/Email/Role/IsActive/CarteiraCount
	settings   map[string]*domain.SellerSettings
	refCodes   map[string]string
	autoAssign *domain.AutoAssignSettings

	rotationUpserts int
	refCodeErr      error
	// ledger is a sentinel — the use case must NEVER touch it.
	ledger []string
}

func newMockOwnerSellerRepo() *mockOwnerSellerRepo {
	return &mockOwnerSellerRepo{
		settings: map[string]*domain.SellerSettings{},
		refCodes: map[string]string{},
		ledger:   []string{"paid-entry-1"},
	}
}

func (m *mockOwnerSellerRepo) clone() *mockOwnerSellerRepo {
	cp := &mockOwnerSellerRepo{
		users:           append([]domain.OwnerSellerRow(nil), m.users...),
		settings:        map[string]*domain.SellerSettings{},
		refCodes:        map[string]string{},
		rotationUpserts: m.rotationUpserts,
		refCodeErr:      m.refCodeErr,
		ledger:          append([]string(nil), m.ledger...),
	}
	for k, v := range m.settings {
		s := *v
		cp.settings[k] = &s
	}
	for k, v := range m.refCodes {
		cp.refCodes[k] = v
	}
	if m.autoAssign != nil {
		aa := *m.autoAssign
		aa.ParticipantUserIDs = append([]string(nil), m.autoAssign.ParticipantUserIDs...)
		cp.autoAssign = &aa
	}
	return cp
}

func (m *mockOwnerSellerRepo) InTx(ctx context.Context, fn func(domain.OwnerSellerRepository) error) error {
	tx := m.clone()
	if err := fn(tx); err != nil {
		return err // rollback: m untouched
	}
	*m = *tx
	return nil
}

func (m *mockOwnerSellerRepo) ListPlatformSellerRows(_ context.Context, _ string) ([]domain.OwnerSellerRow, error) {
	out := make([]domain.OwnerSellerRow, 0, len(m.users))
	for _, u := range m.users {
		row := u
		row.IsActiveAM = true
		if s, ok := m.settings[u.UserID]; ok {
			row.IsSeller = s.IsSeller
			row.IsActiveAM = s.IsActiveAM
			row.PixKey = s.PixKey
		}
		if code, ok := m.refCodes[u.UserID]; ok {
			row.SellerRefCode = &code
		}
		out = append(out, row)
	}
	return out, nil
}

func (m *mockOwnerSellerRepo) SetSellerFlag(_ context.Context, userID string, isSeller bool) error {
	s, ok := m.settings[userID]
	if !ok {
		s = &domain.SellerSettings{UserID: userID, IsActiveAM: true}
		m.settings[userID] = s
	}
	s.IsSeller = isSeller
	return nil
}

func (m *mockOwnerSellerRepo) UpsertSellerActivity(_ context.Context, userID string, isActive bool, deactivatedAt, residualUntil *time.Time) error {
	s, ok := m.settings[userID]
	if !ok {
		s = &domain.SellerSettings{UserID: userID}
		m.settings[userID] = s
	}
	s.IsActiveAM = isActive
	s.DeactivatedAt = deactivatedAt
	s.ResidualUntil = residualUntil
	return nil
}

func (m *mockOwnerSellerRepo) GetOrCreateSellerRefCode(_ context.Context, userID string) (string, error) {
	if m.refCodeErr != nil {
		return "", m.refCodeErr
	}
	if code, ok := m.refCodes[userID]; ok {
		return code, nil
	}
	code := "REF" + userID[len(userID)-4:]
	m.refCodes[userID] = code
	return code, nil
}

func (m *mockOwnerSellerRepo) GetAutoAssignSettings(_ context.Context, _ string) (*domain.AutoAssignSettings, error) {
	return m.autoAssign, nil
}

func (m *mockOwnerSellerRepo) UpsertAutoAssignSettings(_ context.Context, settings *domain.AutoAssignSettings) error {
	m.rotationUpserts++
	m.autoAssign = settings
	return nil
}

// mockSellerUserGetter satisfies commissionUserGetter.
type mockSellerUserGetter struct {
	users map[string]*domain.User
}

func (m *mockSellerUserGetter) GetByID(_ context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}

func attendantRow(id, name string) domain.OwnerSellerRow {
	return domain.OwnerSellerRow{UserID: id, Name: name, Email: name + "@replider.com", Role: strPtr("attendant"), IsActive: true}
}

func newOwnerSellersFixture() (*OwnerSellersUseCase, *mockOwnerSellerRepo, *mockSellerUserGetter) {
	repo := newMockOwnerSellerRepo()
	repo.users = []domain.OwnerSellerRow{
		attendantRow("01USERAAAAAAAAAAAAAAAAAAA1", "ana"),
		attendantRow("01USERBBBBBBBBBBBBBBBBBBB2", "bia"),
	}
	users := &mockSellerUserGetter{users: map[string]*domain.User{
		"01USERAAAAAAAAAAAAAAAAAAA1": {ID: "01USERAAAAAAAAAAAAAAAAAAA1", AccountID: testPlatformID, Role: strPtr("attendant"), IsActive: true},
		"01USERBBBBBBBBBBBBBBBBBBB2": {ID: "01USERBBBBBBBBBBBBBBBBBBB2", AccountID: testPlatformID, Role: strPtr("attendant"), IsActive: true},
		"01USEROTHERACCOUNT0000000X": {ID: "01USEROTHERACCOUNT0000000X", AccountID: "01OTHERACCOUNT00000000000X", Role: strPtr("admin"), IsActive: true},
	}}
	uc := NewOwnerSellersUseCase(repo, users, nil, testPlatformID, "https://replider.com")
	return uc, repo, users
}

// Onboarding must set the flag, mint the ref code and land the user in
// the rotation in ONE call — the fragmentation this section removes.
func TestOwnerSellersOnboardAtomic(t *testing.T) {
	uc, repo, _ := newOwnerSellersFixture()
	// Explicit participant list that does NOT include ana yet.
	repo.autoAssign = &domain.AutoAssignSettings{
		AccountID: testPlatformID, Enabled: true,
		ParticipantUserIDs: []string{"01USERBBBBBBBBBBBBBBBBBBB2"},
	}

	row, err := uc.SetSeller(context.Background(), "owner@replider.com", "01USERAAAAAAAAAAAAAAAAAAA1", true)
	if err != nil {
		t.Fatalf("SetSeller: %v", err)
	}
	if !row.IsSeller {
		t.Error("expected is_seller true")
	}
	if row.SellerRefCode == nil || *row.SellerRefCode == "" {
		t.Error("expected a minted seller_ref_code")
	}
	if row.ShareURL == nil || *row.ShareURL != "https://replider.com/register?ref="+*row.SellerRefCode {
		t.Errorf("expected full share_url for the ref code, got %v", row.ShareURL)
	}
	if !row.InRotation {
		t.Error("expected in_rotation true after onboard")
	}
	if !row.IsActiveAM {
		t.Error("expected is_active_am true after onboard")
	}
	got := repo.autoAssign.ParticipantUserIDs
	if len(got) != 2 {
		t.Fatalf("expected 2 participants, got %v", got)
	}
}

// With the implicit everyone-participates default (no row / empty
// list), onboarding must NOT write a participant list — [user] alone
// would restrict the whole rotation to him.
func TestOwnerSellersOnboardKeepsImplicitPool(t *testing.T) {
	uc, repo, _ := newOwnerSellersFixture()
	repo.autoAssign = nil // platform default: enabled, everyone

	row, err := uc.SetSeller(context.Background(), "owner@replider.com", "01USERAAAAAAAAAAAAAAAAAAA1", true)
	if err != nil {
		t.Fatalf("SetSeller: %v", err)
	}
	if repo.rotationUpserts != 0 {
		t.Errorf("expected no rotation write on implicit pool, got %d upserts", repo.rotationUpserts)
	}
	if !row.InRotation {
		t.Error("expected in_rotation true via implicit pool")
	}
}

// Flagging a platform ADMIN is allowed (warn-only in the UI, US-003)
// — the backend onboards him like any attendant, it just logs it.
func TestOwnerSellersOnboardAllowsPlatformAdmin(t *testing.T) {
	uc, repo, users := newOwnerSellersFixture()
	adminID := "01USERADMINAAAAAAAAAAAAAA3"
	adminRow := attendantRow(adminID, "dono")
	adminRow.Role = strPtr("admin")
	repo.users = append(repo.users, adminRow)
	users.users[adminID] = &domain.User{ID: adminID, AccountID: testPlatformID, Role: strPtr("admin"), IsActive: true}

	row, err := uc.SetSeller(context.Background(), "owner@replider.com", adminID, true)
	if err != nil {
		t.Fatalf("SetSeller on platform admin: %v", err)
	}
	if !row.IsSeller || row.SellerRefCode == nil || !row.InRotation {
		t.Errorf("expected admin fully onboarded (seller+code+rotation), got %+v", row)
	}
}

// A user of another account must be rejected with a clear domain error
// (mapped to 404), never a 500.
func TestOwnerSellersOnboardRejectsForeignUser(t *testing.T) {
	uc, _, _ := newOwnerSellersFixture()

	_, err := uc.SetSeller(context.Background(), "owner@replider.com", "01USEROTHERACCOUNT0000000X", true)
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "COMMISSION_SELLER_NOT_FOUND" {
		t.Fatalf("expected COMMISSION_SELLER_NOT_FOUND, got %v", err)
	}
}

// Offboarding unflags, deactivates (stamping the residual window),
// removes from the rotation — and never touches the ledger.
func TestOwnerSellersOffboard(t *testing.T) {
	uc, repo, _ := newOwnerSellersFixture()
	fixed := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return fixed }
	repo.settings["01USERAAAAAAAAAAAAAAAAAAA1"] = &domain.SellerSettings{
		UserID: "01USERAAAAAAAAAAAAAAAAAAA1", IsSeller: true, IsActiveAM: true,
	}
	repo.refCodes["01USERAAAAAAAAAAAAAAAAAAA1"] = "REFANA1"
	repo.autoAssign = &domain.AutoAssignSettings{
		AccountID: testPlatformID, Enabled: true,
		ParticipantUserIDs: []string{"01USERAAAAAAAAAAAAAAAAAAA1", "01USERBBBBBBBBBBBBBBBBBBB2"},
	}

	row, err := uc.SetSeller(context.Background(), "owner@replider.com", "01USERAAAAAAAAAAAAAAAAAAA1", false)
	if err != nil {
		t.Fatalf("SetSeller: %v", err)
	}
	if row.IsSeller {
		t.Error("expected is_seller false")
	}
	if row.InRotation {
		t.Error("expected in_rotation false after offboard")
	}
	s := repo.settings["01USERAAAAAAAAAAAAAAAAAAA1"]
	if s.IsActiveAM {
		t.Error("expected is_active_am false after offboard")
	}
	if s.DeactivatedAt == nil || !s.DeactivatedAt.Equal(fixed) {
		t.Errorf("expected deactivated_at %v, got %v", fixed, s.DeactivatedAt)
	}
	if s.ResidualUntil == nil || !s.ResidualUntil.Equal(fixed.AddDate(0, residualMonths, 0)) {
		t.Errorf("expected residual_until %v, got %v", fixed.AddDate(0, residualMonths, 0), s.ResidualUntil)
	}
	got := repo.autoAssign.ParticipantUserIDs
	if len(got) != 1 || got[0] != "01USERBBBBBBBBBBBBBBBBBBB2" {
		t.Errorf("expected only bia in rotation, got %v", got)
	}
	// History survives: ref code kept, ledger sentinel untouched.
	if repo.refCodes["01USERAAAAAAAAAAAAAAAAAAA1"] != "REFANA1" {
		t.Error("offboard must not erase the ref code")
	}
	if len(repo.ledger) != 1 {
		t.Error("offboard must never touch the ledger")
	}
}

// Offboarding out of the implicit everyone-pool must materialize the
// remaining members explicitly, or the ex-seller keeps receiving leads.
func TestOwnerSellersOffboardMaterializesImplicitPool(t *testing.T) {
	uc, repo, _ := newOwnerSellersFixture()
	repo.autoAssign = nil
	repo.settings["01USERAAAAAAAAAAAAAAAAAAA1"] = &domain.SellerSettings{
		UserID: "01USERAAAAAAAAAAAAAAAAAAA1", IsSeller: true, IsActiveAM: true,
	}

	row, err := uc.SetSeller(context.Background(), "owner@replider.com", "01USERAAAAAAAAAAAAAAAAAAA1", false)
	if err != nil {
		t.Fatalf("SetSeller: %v", err)
	}
	if row.InRotation {
		t.Error("expected in_rotation false after offboard")
	}
	if repo.autoAssign == nil {
		t.Fatal("expected the implicit pool to be materialized")
	}
	got := repo.autoAssign.ParticipantUserIDs
	if len(got) != 1 || got[0] != "01USERBBBBBBBBBBBBBBBBBBB2" {
		t.Errorf("expected materialized pool [bia], got %v", got)
	}
	if !repo.autoAssign.Enabled {
		t.Error("materializing must keep the platform default enabled=true")
	}
}

// A failure mid-onboard rolls the whole thing back — no half-flagged
// seller without ref code or rotation.
func TestOwnerSellersOnboardRollsBackOnError(t *testing.T) {
	uc, repo, _ := newOwnerSellersFixture()
	repo.refCodeErr = errors.New("mint failed")

	_, err := uc.SetSeller(context.Background(), "owner@replider.com", "01USERAAAAAAAAAAAAAAAAAAA1", true)
	if err == nil {
		t.Fatal("expected error")
	}
	if s := repo.settings["01USERAAAAAAAAAAAAAAAAAAA1"]; s != nil && s.IsSeller {
		t.Error("flag must roll back when the ref-code mint fails")
	}
}

// List resolves rotation membership: disabled settings put nobody in
// rotation, an explicit list gates by membership.
func TestOwnerSellersListRotation(t *testing.T) {
	uc, repo, _ := newOwnerSellersFixture()
	repo.autoAssign = &domain.AutoAssignSettings{AccountID: testPlatformID, Enabled: false}

	rows, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, r := range rows {
		if r.InRotation {
			t.Errorf("user %s in rotation despite disabled settings", r.UserID)
		}
	}

	repo.autoAssign = &domain.AutoAssignSettings{
		AccountID: testPlatformID, Enabled: true,
		ParticipantUserIDs: []string{"01USERBBBBBBBBBBBBBBBBBBB2"},
	}
	rows, err = uc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byID := map[string]domain.OwnerSellerRow{}
	for _, r := range rows {
		byID[r.UserID] = r
	}
	if byID["01USERAAAAAAAAAAAAAAAAAAA1"].InRotation {
		t.Error("ana must not rotate — not in the explicit list")
	}
	if !byID["01USERBBBBBBBBBBBBBBBBBBB2"].InRotation {
		t.Error("bia must rotate — in the explicit list")
	}
}

// Unset PLATFORM_ACCOUNT_ID fails loudly (503 code), never an empty list.
func TestOwnerSellersRequiresPlatformAccount(t *testing.T) {
	repo := newMockOwnerSellerRepo()
	uc := NewOwnerSellersUseCase(repo, &mockSellerUserGetter{users: map[string]*domain.User{}}, nil, "", "https://replider.com")

	_, err := uc.List(context.Background())
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "OWNER_PLATFORM_NOT_CONFIGURED" {
		t.Fatalf("expected OWNER_PLATFORM_NOT_CONFIGURED, got %v", err)
	}
}
