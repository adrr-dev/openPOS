package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
)

var (
	ErrInvalidCredentials = errors.New("email atau kata sandi tidak cocok")
	ErrPasscodeRequired   = errors.New("passcode_required")
	ErrPasscodeWrong      = errors.New("passcode salah")
	ErrEmailTaken         = errors.New("email sudah terdaftar")
	ErrAccountInactive    = errors.New("akun dinonaktifkan")
	ErrTokenInvalid       = errors.New("sesi tidak valid, silakan masuk kembali")
)

type AuthService struct {
	users      *repo.UserRepo
	refresh    *repo.RefreshRepo
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(users *repo.UserRepo, refresh *repo.RefreshRepo, jwtSecret string, accessTTL time.Duration, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		users:      users,
		refresh:    refresh,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// ── register ─────────────────────────────────────────────────────────

func (s *AuthService) Register(ctx context.Context, name, email, password, storeName string) (*model.User, *model.TokenPair, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	storeName = strings.TrimSpace(storeName)

	if name == "" {
		return nil, nil, fmt.Errorf("nama wajib diisi")
	}
	if !isEmail(email) {
		return nil, nil, fmt.Errorf("format email tidak valid")
	}
	if len(password) < 8 {
		return nil, nil, fmt.Errorf("kata sandi minimal 8 karakter")
	}
	if storeName == "" {
		return nil, nil, fmt.Errorf("nama toko wajib diisi")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.users.RegisterTx(ctx, storeName, email, name, string(hash))
	if err != nil {
		if errors.Is(err, repo.ErrDuplicate) {
			return nil, nil, ErrEmailTaken
		}
		return nil, nil, err
	}

	pair, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// ── login ────────────────────────────────────────────────────────────

func (s *AuthService) Login(ctx context.Context, email, password, passcode string) (*model.User, *model.TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}
	if !user.Active {
		return nil, nil, ErrAccountInactive
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if user.PasscodeHash != nil && *user.PasscodeHash != "" {
		if passcode == "" {
			return nil, nil, ErrPasscodeRequired
		}
		if bcrypt.CompareHashAndPassword([]byte(*user.PasscodeHash), []byte(passcode)) != nil {
			return nil, nil, ErrPasscodeWrong
		}
	}

	pair, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// ── me / refresh / logout ────────────────────────────────────────────

func (s *AuthService) Me(ctx context.Context, userID string) (*model.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.Active {
		return nil, ErrAccountInactive
	}
	return user, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*model.User, *model.TokenPair, error) {
	hash := hashToken(refreshToken)
	rt, err := s.refresh.GetActiveByHash(ctx, hash)
	if err != nil || rt.Revoked || time.Now().After(rt.ExpiresAt) {
		return nil, nil, ErrTokenInvalid
	}

	user, err := s.Me(ctx, rt.UserID)
	if err != nil {
		return nil, nil, ErrTokenInvalid
	}

	// rotasi: token lama dicabut, pasangan baru diterbitkan
	if err := s.refresh.Revoke(ctx, hash); err != nil {
		return nil, nil, err
	}
	pair, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) {
	if refreshToken == "" {
		return
	}
	_ = s.refresh.Revoke(ctx, hashToken(refreshToken))
}

// ── internal ─────────────────────────────────────────────────────────

// Claims adalah isi payload access token.
type Claims struct {
	UserID  string
	StoreID string
	Role    model.Role
	Name    string
	Email   string
}

func (s *AuthService) issueTokens(ctx context.Context, u *model.User) (*model.TokenPair, error) {
	now := time.Now()
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   u.ID,
		"sid":   u.StoreID,
		"role":  string(u.Role),
		"name":  u.Name,
		"email": u.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(s.accessTTL).Unix(),
	}).SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	refresh := hex.EncodeToString(raw)

	if err := s.refresh.Create(ctx, u.ID, hashToken(refresh), now.Add(s.refreshTTL)); err != nil {
		return nil, err
	}

	return &model.TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// ParseAccess memvalidasi access token dan mengembalikan claims-nya.
func (s *AuthService) ParseAccess(tokenStr string) (*Claims, error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("metode tanda tangan tak terduga: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrTokenInvalid
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenInvalid
	}
	sub, _ := mc["sub"].(string)
	sid, _ := mc["sid"].(string)
	role, _ := mc["role"].(string)
	name, _ := mc["name"].(string)
	email, _ := mc["email"].(string)
	if sub == "" {
		return nil, ErrTokenInvalid
	}
	return &Claims{UserID: sub, StoreID: sid, Role: model.Role(role), Name: name, Email: email}, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func isEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1 && strings.Contains(s[at:], ".") && !strings.ContainsAny(s, " \t")
}
