package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/smtp"
	"os"
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
	ErrSwitchSelf         = errors.New("tidak dapat beralih ke akun sendiri")
	ErrInvalidEmail       = errors.New("Email tidak valid.")
	ErrOtpCooldown        = errors.New("Terlalu sering meminta kode. Coba lagi dalam 60 detik.")
	ErrOtpWrong           = errors.New("Kode OTP salah.")
	ErrOtpExpired         = errors.New("Kode OTP sudah kedaluwarsa. Kirim ulang.")
	ErrOtpMaxAttempts     = errors.New("Terlalu banyak percobaan. Kirim ulang kode OTP.")
	ErrEmailNotVerified   = errors.New("Email belum diverifikasi. Silakan verifikasi kode OTP terlebih dahulu.")
)

var TestOnOTPSent func(email, code string)

type AuthService struct {
	users      *repo.UserRepo
	cashiers   *repo.CashierRepo
	refresh    *repo.RefreshRepo
	otps       *repo.OtpRepo
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(users *repo.UserRepo, cashiers *repo.CashierRepo, refresh *repo.RefreshRepo, otps *repo.OtpRepo, jwtSecret string, accessTTL time.Duration, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		users:      users,
		cashiers:   cashiers,
		refresh:    refresh,
		otps:       otps,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// ── register ─────────────────────────────────────────────────────────

func (s *AuthService) SendOTP(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !isEmail(email) {
		return ErrInvalidEmail
	}

	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, repo.ErrNotFound) {
		return err
	}

	existing, err := s.otps.GetOTP(ctx, email)
	if err == nil && existing != nil {
		if time.Since(existing.LastSentAt) < 60*time.Second {
			return ErrOtpCooldown
		}
	}

	code, err := generate6DigitOTP()
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	if err := s.otps.UpsertOTP(ctx, email, string(hash), expiresAt); err != nil {
		return err
	}

	if TestOnOTPSent != nil {
		TestOnOTPSent(email, code)
	}

	if err := sendOTPEmail(email, code); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	o, err := s.otps.GetOTP(ctx, email)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrOtpWrong
		}
		return err
	}

	if o.Attempts >= 3 {
		return ErrOtpMaxAttempts
	}

	if time.Now().After(o.ExpiresAt) {
		return ErrOtpExpired
	}

	if bcrypt.CompareHashAndPassword([]byte(o.CodeHash), []byte(code)) != nil {
		attempts, _ := s.otps.IncrementAttempts(ctx, email)
		if attempts >= 3 {
			return ErrOtpMaxAttempts
		}
		return ErrOtpWrong
	}

	if err := s.otps.MarkVerified(ctx, email); err != nil {
		return err
	}

	return nil
}

func generate6DigitOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	val := n.Int64() + 100000
	return fmt.Sprintf("%d", val), nil
}

func sendOTPEmail(toEmail, otpCode string) error {
	host := getEnv("SMTP_HOST", "smtp.gmail.com")
	port := getEnv("SMTP_PORT", "587")
	user := getEnv("SMTP_EMAIL", getEnv("SMTP_USER", ""))
	pass := getEnv("SMTP_PASSWORD", getEnv("SMTP_PASS", ""))
	from := getEnv("SMTP_FROM", user)

	subject := "Kode Verifikasi OTP OpenPOS"
	body := fmt.Sprintf("Halo,\n\nKode verifikasi OTP OpenPOS Anda adalah: %s\n\nKode ini berlaku selama 10 menit. Jangan berikan kode ini kepada siapa pun.\n\nSalam,\nTim OpenPOS", otpCode)

	msg := "From: " + from + "\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body

	if user == "" || pass == "" {
		log.Printf("[DEV/MOCK EMAIL] To: %s | OTP Code: %s", toEmail, otpCode)
		return nil
	}

	auth := smtp.PlainAuth("", user, pass, host)
	addr := host + ":" + port
	err := smtp.SendMail(addr, auth, from, []string{toEmail}, []byte(msg))
	if err != nil {
		log.Printf("[SMTP WARNING] Failed to send email to %s: %v. Falling back to console log. OTP was: %s", toEmail, err, otpCode)
		log.Printf("[DEV/MOCK EMAIL FALLBACK] To: %s | OTP Code: %s", toEmail, otpCode)
		return nil
	}
	return nil
}

func (s *AuthService) Register(ctx context.Context, name, email, password, storeName string) (*model.User, *model.TokenPair, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	storeName = strings.TrimSpace(storeName)

	if name == "" {
		return nil, nil, fmt.Errorf("nama wajib diisi")
	}
	if !isEmail(email) {
		return nil, nil, ErrInvalidEmail
	}
	if len(password) < 8 {
		return nil, nil, fmt.Errorf("kata sandi minimal 8 karakter")
	}
	if storeName == "" {
		return nil, nil, fmt.Errorf("nama toko wajib diisi")
	}

	verified, err := s.otps.IsEmailVerified(ctx, email)
	if err != nil || !verified {
		return nil, nil, ErrEmailNotVerified
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

	pair, err := s.issueTokens(ctx, user.ID, nil)
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

	pair, err := s.issueTokens(ctx, user.ID, nil)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// ── me / refresh / logout ────────────────────────────────────────────

func (s *AuthService) Me(ctx context.Context, claims *Claims) (*model.PublicUser, error) {
	if claims.ActingAsCashierID != nil {
		c, err := s.cashiers.GetByID(ctx, *claims.ActingAsCashierID)
		if err != nil {
			return nil, ErrTokenInvalid
		}
		if !c.Active {
			return nil, ErrAccountInactive
		}
		owner, err := s.users.GetByID(ctx, claims.UserID)
		storeName := ""
		if err == nil {
			storeName = owner.StoreName
		}
		pub := c.Public(storeName)
		return &pub, nil
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if !user.Active {
		return nil, ErrAccountInactive
	}
	pub := user.Public()
	return &pub, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*model.User, *model.TokenPair, error) {
	hash := hashToken(refreshToken)
	rt, err := s.refresh.GetActiveByHash(ctx, hash)
	if err != nil || rt.Revoked || time.Now().After(rt.ExpiresAt) {
		return nil, nil, ErrTokenInvalid
	}

	user, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil || !user.Active {
		return nil, nil, ErrTokenInvalid
	}

	if err := s.refresh.Revoke(ctx, hash); err != nil {
		return nil, nil, err
	}
	pair, err := s.issueTokens(ctx, user.ID, nil)
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

// ── switch account ──────────────────────────────────────────────────

// Switch memungkinkan beralih sesi ke Admin atau Kasir lain dalam toko yang sama.
func (s *AuthService) Switch(ctx context.Context, claims *Claims, targetID, passcode string) (*model.PublicUser, *model.TokenPair, error) {
	currentActiveID := claims.UserID
	if claims.ActingAsCashierID != nil {
		currentActiveID = *claims.ActingAsCashierID
	}
	if currentActiveID == targetID {
		return nil, nil, ErrSwitchSelf
	}

	owner, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil || !owner.Active {
		return nil, nil, ErrTokenInvalid
	}

	// Cek apakah target adalah owner (admin) sendiri
	if targetID == owner.ID {
		if owner.PasscodeHash != nil && *owner.PasscodeHash != "" {
			if passcode == "" {
				return nil, nil, ErrPasscodeRequired
			}
			if bcrypt.CompareHashAndPassword([]byte(*owner.PasscodeHash), []byte(passcode)) != nil {
				return nil, nil, ErrPasscodeWrong
			}
		}
		pair, err := s.issueTokens(ctx, owner.ID, nil)
		if err != nil {
			return nil, nil, err
		}
		pub := owner.Public()
		return &pub, pair, nil
	}

	// Cek apakah target adalah cashier
	cashier, err := s.cashiers.GetByID(ctx, targetID)
	if err != nil {
		return nil, nil, repo.ErrNotFound
	}
	if cashier.StoreID != owner.StoreID {
		return nil, nil, repo.ErrNotFound
	}
	if !cashier.Active {
		return nil, nil, ErrAccountInactive
	}

	if cashier.PasscodeHash != nil && *cashier.PasscodeHash != "" {
		if passcode == "" {
			return nil, nil, ErrPasscodeRequired
		}
		if bcrypt.CompareHashAndPassword([]byte(*cashier.PasscodeHash), []byte(passcode)) != nil {
			return nil, nil, ErrPasscodeWrong
		}
	}

	pair, err := s.issueTokens(ctx, owner.ID, &cashier.ID)
	if err != nil {
		return nil, nil, err
	}
	pub := cashier.Public(owner.StoreName)
	return &pub, pair, nil
}

// ── internal ─────────────────────────────────────────────────────────

type Claims struct {
	UserID            string
	StoreID           string
	Name              string
	Email             string
	ActingAsCashierID *string
	Role              model.Role
}

func (c *Claims) ActiveRole() model.Role {
	if c.ActingAsCashierID != nil {
		return model.RoleCashier
	}
	return model.RoleAdmin
}

func (s *AuthService) issueTokens(ctx context.Context, ownerID string, actingAsCashierID *string) (*model.TokenPair, error) {
	owner, err := s.users.GetByID(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	mc := jwt.MapClaims{
		"sub":   owner.ID,
		"sid":   owner.StoreID,
		"name":  owner.Name,
		"email": owner.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(s.accessTTL).Unix(),
	}
	if actingAsCashierID != nil {
		mc["acting_as"] = *actingAsCashierID
	}

	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, mc).SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	refresh := hex.EncodeToString(raw)

	if err := s.refresh.Create(ctx, owner.ID, hashToken(refresh), now.Add(s.refreshTTL)); err != nil {
		return nil, err
	}

	return &model.TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

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
	name, _ := mc["name"].(string)
	email, _ := mc["email"].(string)
	actingAs, _ := mc["acting_as"].(string)
	if sub == "" {
		return nil, ErrTokenInvalid
	}

	c := &Claims{
		UserID:  sub,
		StoreID: sid,
		Name:    name,
		Email:   email,
		Role:    model.RoleAdmin,
	}
	if actingAs != "" {
		c.ActingAsCashierID = &actingAs
		c.Role = model.RoleCashier
	}
	return c, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func isEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1 && strings.Contains(s[at:], ".") && !strings.ContainsAny(s, " \t")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
