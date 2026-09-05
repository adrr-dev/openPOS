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
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"

	"github.com/0xMinomus/openPOS/backend/model"
	"github.com/0xMinomus/openPOS/backend/repo"
)

var (
	ErrInvalidCredentials  = errors.New("email atau kata sandi tidak cocok")
	ErrPasscodeRequired    = errors.New("passcode_required")
	ErrPasscodeWrong       = errors.New("passcode salah")
	ErrEmailTaken          = errors.New("email sudah terdaftar")
	ErrAccountInactive     = errors.New("akun dinonaktifkan")
	ErrTokenInvalid        = errors.New("sesi tidak valid, silakan masuk kembali")
	ErrSwitchSelf          = errors.New("tidak dapat beralih ke akun sendiri")
	ErrInvalidEmail        = errors.New("Email tidak valid.")
	ErrOtpCooldown         = errors.New("Terlalu sering meminta kode. Coba lagi dalam 60 detik.")
	ErrOtpWrong            = errors.New("Kode OTP salah.")
	ErrOtpExpired          = errors.New("Kode OTP sudah kedaluwarsa. Kirim ulang.")
	ErrOtpMaxAttempts      = errors.New("Terlalu banyak percobaan. Kirim ulang kode OTP.")
	ErrEmailNotVerified    = errors.New("Email belum diverifikasi. Silakan verifikasi kode OTP terlebih dahulu.")
	ErrGoogleInvalid       = errors.New("login Google tidak valid")
	ErrGoogleNotConfigured = errors.New("login Google belum dikonfigurasi di server")
)

var TestOnOTPSent func(email, code string)

// VerifyGoogleToken validates a GIS ID token. Package-level var (same
// pattern as TestOnOTPSent) so tests can stub it — no network needed.
var VerifyGoogleToken = idtoken.Validate

type AuthService struct {
	users          *repo.UserRepo
	cashiers       *repo.CashierRepo
	refresh        *repo.RefreshRepo
	otps           *repo.OtpRepo
	jwtSecret      []byte
	accessTTL      time.Duration
	refreshTTL     time.Duration
	googleClientID string
}

func NewAuthService(users *repo.UserRepo, cashiers *repo.CashierRepo, refresh *repo.RefreshRepo, otps *repo.OtpRepo, jwtSecret string, accessTTL time.Duration, refreshTTL time.Duration, googleClientID ...string) *AuthService {
	svc := &AuthService{
		users:      users,
		cashiers:   cashiers,
		refresh:    refresh,
		otps:       otps,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
	if len(googleClientID) > 0 {
		svc.googleClientID = googleClientID[0]
	}
	return svc
}

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

// GoogleLogin verifies a GIS ID token and returns our own token pair.
// Existing emails auto-link (password keeps working); new emails get a
// fresh store + admin, mirroring Register without the OTP gate or password.
func (s *AuthService) GoogleLogin(ctx context.Context, idToken, storeName string) (*model.User, *model.TokenPair, error) {
	if s.googleClientID == "" {
		return nil, nil, ErrGoogleNotConfigured
	}
	if strings.TrimSpace(idToken) == "" {
		return nil, nil, ErrGoogleInvalid
	}

	payload, err := VerifyGoogleToken(ctx, idToken, s.googleClientID)
	if err != nil {
		return nil, nil, ErrGoogleInvalid
	}

	email, _ := payload.Claims["email"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if !isEmail(email) {
		return nil, nil, ErrGoogleInvalid
	}
	if !googleEmailVerified(payload.Claims["email_verified"]) {
		return nil, nil, ErrEmailNotVerified
	}
	name := strings.TrimSpace(stringClaim(payload.Claims, "name"))
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		if !user.Active {
			return nil, nil, ErrAccountInactive
		}
		pair, err := s.issueTokens(ctx, user.ID, nil)
		if err != nil {
			return nil, nil, err
		}
		return user, pair, nil
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, nil, err
	}

	storeName = strings.TrimSpace(storeName)
	if storeName == "" {
		storeName = name + "'s Store"
	}

	// Empty password hash: column stays NOT NULL, and bcrypt can never
	// match it, so password-login for Google users safely 401s.
	user, err = s.users.RegisterTx(ctx, storeName, email, name, "")
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

func stringClaim(claims map[string]interface{}, key string) string {
	v, _ := claims[key].(string)
	return v
}

// Google sends email_verified as bool; accept "true"/1 defensively.
func googleEmailVerified(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	case float64:
		return t == 1
	default:
		return false
	}
}

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

func (s *AuthService) Switch(ctx context.Context, claims *Claims, targetID uint, passcode string) (*model.PublicUser, *model.TokenPair, error) {
	owner, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil || !owner.Active {
		return nil, nil, ErrTokenInvalid
	}

	// NOTE: users and cashiers live in separate tables with independent
	// numeric sequences, so an owner and a cashier can share the same number
	// (e.g. both id 1). Disambiguate by session state, never by number alone.
	if claims.ActingAsCashierID != nil {
		if targetID == *claims.ActingAsCashierID {
			return nil, nil, ErrSwitchSelf
		}
		if targetID == owner.ID {
			return s.switchToOwner(ctx, owner, passcode)
		}
		return s.switchToCashier(ctx, owner, targetID, passcode)
	}

	// Acting as admin: a cashier wins ties (it is the only reachable
	// interpretation that isn't "self"); self-switch stays 400 like before.
	if c, err := s.cashiers.GetByID(ctx, targetID); err == nil && c.StoreID == owner.StoreID {
		if !c.Active {
			return nil, nil, ErrAccountInactive
		}
		if err := checkPasscode(c.PasscodeHash, passcode); err != nil {
			return nil, nil, err
		}
		pair, err := s.issueTokens(ctx, owner.ID, &c.ID)
		if err != nil {
			return nil, nil, err
		}
		pub := c.Public(owner.StoreName)
		return &pub, pair, nil
	}
	if targetID == owner.ID {
		return nil, nil, ErrSwitchSelf
	}
	return nil, nil, repo.ErrNotFound
}

func (s *AuthService) switchToOwner(ctx context.Context, owner *model.User, passcode string) (*model.PublicUser, *model.TokenPair, error) {
	if err := checkPasscode(owner.PasscodeHash, passcode); err != nil {
		return nil, nil, err
	}
	pair, err := s.issueTokens(ctx, owner.ID, nil)
	if err != nil {
		return nil, nil, err
	}
	pub := owner.Public()
	return &pub, pair, nil
}

func (s *AuthService) switchToCashier(ctx context.Context, owner *model.User, targetID uint, passcode string) (*model.PublicUser, *model.TokenPair, error) {
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
	if err := checkPasscode(cashier.PasscodeHash, passcode); err != nil {
		return nil, nil, err
	}
	pair, err := s.issueTokens(ctx, owner.ID, &cashier.ID)
	if err != nil {
		return nil, nil, err
	}
	pub := cashier.Public(owner.StoreName)
	return &pub, pair, nil
}

func checkPasscode(hash *string, passcode string) error {
	if hash == nil || *hash == "" {
		return nil
	}
	if passcode == "" {
		return ErrPasscodeRequired
	}
	if bcrypt.CompareHashAndPassword([]byte(*hash), []byte(passcode)) != nil {
		return ErrPasscodeWrong
	}
	return nil
}

type Claims struct {
	UserID            uint
	StoreID           uint
	Name              string
	Email             string
	ActingAsCashierID *uint
	Role              model.Role
}

func (c *Claims) ActiveRole() model.Role {
	if c.ActingAsCashierID != nil {
		return model.RoleCashier
	}
	return model.RoleAdmin
}

func (s *AuthService) issueTokens(ctx context.Context, ownerID uint, actingAsCashierID *uint) (*model.TokenPair, error) {
	owner, err := s.users.GetByID(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	mc := jwt.MapClaims{
		"sub":   strconv.FormatUint(uint64(owner.ID), 10),
		"sid":   strconv.FormatUint(uint64(owner.StoreID), 10),
		"name":  owner.Name,
		"email": owner.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(s.accessTTL).Unix(),
	}
	if actingAsCashierID != nil {
		mc["acting_as"] = strconv.FormatUint(uint64(*actingAsCashierID), 10)
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
	uid, err := strconv.ParseUint(sub, 10, 64)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	sidNum, err := strconv.ParseUint(sid, 10, 64)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	c := &Claims{
		UserID:  uint(uid),
		StoreID: uint(sidNum),
		Name:    name,
		Email:   email,
		Role:    model.RoleAdmin,
	}
	if actingAs != "" {
		actingNum, err := strconv.ParseUint(actingAs, 10, 64)
		if err != nil {
			return nil, ErrTokenInvalid
		}
		actingID := uint(actingNum)
		c.ActingAsCashierID = &actingID
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
