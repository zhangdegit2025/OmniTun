package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	omnitunv1 "github.com/omnitun/omnitun/proto/omnitun/v1"
	apperrors "github.com/omnitun/omnitun/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	omnitunv1.UnimplementedAuthServiceServer
	repo     Repository
	jwtMgr   *JWTManager
	oauthMgr *OAuthManager
	logger   *slog.Logger
}

func NewService(repo Repository, jwtMgr *JWTManager, oauthMgr *OAuthManager) *Service {
	return &Service{
		repo:     repo,
		jwtMgr:   jwtMgr,
		oauthMgr: oauthMgr,
		logger:   slog.Default().With("component", "auth_service"),
	}
}

func (s *Service) Register(ctx context.Context, req *omnitunv1.RegisterRequest) (*omnitunv1.RegisterResponse, error) {
	if err := validateEmail(req.Email); err != nil {
		s.logger.WarnContext(ctx, "register validation failed: invalid email", "email", req.Email)
		return nil, apperrors.BadRequest(err.Error())
	}

	if err := ValidatePassword(req.Password); err != nil {
		s.logger.WarnContext(ctx, "register validation failed: weak password")
		return nil, apperrors.BadRequest(err.Error())
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash password", "error", err)
		return nil, apperrors.Internal("failed to process password")
	}

	orgSlug := emailToSlug(req.Email)
	org := &Organization{
		Name: req.Email,
		Slug: orgSlug,
		Plan: "free",
	}
	if err := s.repo.CreateOrganization(ctx, org); err != nil {
		s.logger.ErrorContext(ctx, "failed to create organization", "error", err)
		return nil, apperrors.Internal("failed to create organization")
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Email
	}

	user := &User{
		OrganizationID: org.ID,
		Email:          req.Email,
		DisplayName:    displayName,
		Role:           "owner",
		AuthProvider:   "email",
	}
	user.PasswordHash = sql.NullString{String: passwordHash, Valid: true}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		s.logger.ErrorContext(ctx, "failed to create user", "error", err)
		return nil, apperrors.Conflict("email already registered")
	}

	s.logger.InfoContext(ctx, "user registered successfully",
		"user_id", user.ID,
		"org_id", org.ID,
		"email", req.Email,
	)

	return &omnitunv1.RegisterResponse{
		UserId:  user.ID,
		Message: "registration successful",
	}, nil
}

func (s *Service) Login(ctx context.Context, req *omnitunv1.LoginRequest) (*omnitunv1.LoginResponse, error) {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.WarnContext(ctx, "login failed: user not found", "email", req.Email)
		return nil, apperrors.Unauthorized("invalid email or password")
	}

	if !user.PasswordHash.Valid {
		s.logger.WarnContext(ctx, "login failed: no password set", "user_id", user.ID)
		return nil, apperrors.Unauthorized("invalid email or password")
	}

	if err := CheckPassword(user.PasswordHash.String, req.Password); err != nil {
		s.logger.WarnContext(ctx, "login failed: invalid password", "user_id", user.ID)
		return nil, apperrors.Unauthorized("invalid email or password")
	}

	if user.MFAEnabled {
		if req.MfaCode == "" {
			s.logger.InfoContext(ctx, "MFA required but not provided", "user_id", user.ID)
			return nil, apperrors.NewAppError("MFA_REQUIRED", "MFA code required", 401)
		}
		if !user.MFASecret.Valid {
			return nil, apperrors.Internal("MFA secret not configured")
		}
		if !ValidateTOTPCode(user.MFASecret.String, req.MfaCode) {
			s.logger.WarnContext(ctx, "login failed: invalid MFA code", "user_id", user.ID)
			return nil, apperrors.Unauthorized("invalid MFA code")
		}
	}

	accessToken, err := s.jwtMgr.IssueAccessToken(user.ID, user.OrganizationID, user.Role)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to issue access token", "error", err, "user_id", user.ID)
		return nil, apperrors.Internal("failed to generate access token")
	}

	refreshToken, err := s.jwtMgr.IssueRefreshToken(user.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to issue refresh token", "error", err, "user_id", user.ID)
		return nil, apperrors.Internal("failed to generate refresh token")
	}

	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
	if err := s.repo.StoreRefreshToken(ctx, user.ID, refreshToken, refreshExpiry); err != nil {
		s.logger.ErrorContext(ctx, "failed to store refresh token", "error", err, "user_id", user.ID)
		return nil, apperrors.Internal("failed to store refresh token")
	}

	if err := s.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		s.logger.WarnContext(ctx, "failed to update last login", "error", err, "user_id", user.ID)
	}

	s.logger.InfoContext(ctx, "user logged in successfully", "user_id", user.ID)

	return &omnitunv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int32(s.jwtMgr.accessExpiry.Seconds()),
		User:         userToProto(user),
	}, nil
}

func (s *Service) RefreshToken(ctx context.Context, req *omnitunv1.RefreshTokenRequest) (*omnitunv1.RefreshTokenResponse, error) {
	tokenHash := HashToken(req.RefreshToken)

	record, err := s.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		s.logger.WarnContext(ctx, "refresh token not found or expired")
		return nil, apperrors.Unauthorized("invalid or expired refresh token")
	}

	user, err := s.repo.GetUserByID(ctx, record.UserID)
	if err != nil {
		s.logger.ErrorContext(ctx, "user not found for refresh token", "user_id", record.UserID)
		return nil, apperrors.Unauthorized("user not found")
	}

	newRefreshToken, err := s.jwtMgr.IssueRefreshToken(user.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to issue new refresh token", "error", err)
		return nil, apperrors.Internal("failed to generate refresh token")
	}

	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
	if err := s.repo.DeleteRefreshToken(ctx, tokenHash); err != nil {
		s.logger.ErrorContext(ctx, "refresh token rotation: failed to revoke old token", "error", err)
		return nil, apperrors.Internal("failed to rotate refresh token")
	}
	if err := s.repo.StoreRefreshToken(ctx, user.ID, newRefreshToken, refreshExpiry); err != nil {
		s.logger.ErrorContext(ctx, "failed to store new refresh token", "error", err)
		return nil, apperrors.Internal("failed to store new refresh token")
	}

	accessToken, err := s.jwtMgr.IssueAccessToken(user.ID, user.OrganizationID, user.Role)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to issue access token", "error", err)
		return nil, apperrors.Internal("failed to generate access token")
	}

	s.logger.InfoContext(ctx, "tokens refreshed successfully", "user_id", user.ID)

	return &omnitunv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int32(s.jwtMgr.accessExpiry.Seconds()),
	}, nil
}

func (s *Service) Logout(ctx context.Context, req *omnitunv1.LogoutRequest) (*omnitunv1.LogoutResponse, error) {
	tokenHash := HashToken(req.RefreshToken)
	if err := s.repo.DeleteRefreshToken(ctx, tokenHash); err != nil {
		s.logger.WarnContext(ctx, "failed to delete refresh token on logout", "error", err)
	}
	return &omnitunv1.LogoutResponse{
		Message: "logged out successfully",
	}, nil
}

func (s *Service) GetMe(ctx context.Context, req *omnitunv1.GetMeRequest) (*omnitunv1.GetMeResponse, error) {
	userID, ok := GetUserID(ctx)
	if !ok {
		return nil, apperrors.Unauthorized("user not authenticated")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "get me: user not found", "user_id", userID)
		return nil, apperrors.NotFound("user not found")
	}

	return &omnitunv1.GetMeResponse{
		User: userToProto(user),
	}, nil
}

func (s *Service) EnrollMFA(ctx context.Context, req *omnitunv1.EnrollMFARequest) (*omnitunv1.EnrollMFAResponse, error) {
	userID, ok := GetUserID(ctx)
	if !ok {
		return nil, apperrors.Unauthorized("user not authenticated")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.NotFound("user not found")
	}

	secret, qrURL, err := GenerateTOTPSecret(user.Email)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate TOTP secret", "error", err)
		return nil, apperrors.Internal("failed to generate MFA secret")
	}

	if err := s.repo.UpdateMFA(ctx, userID, false, secret); err != nil {
		s.logger.ErrorContext(ctx, "failed to store MFA secret", "error", err)
		return nil, apperrors.Internal("failed to store MFA secret")
	}

	s.logger.InfoContext(ctx, "MFA enrollment initiated", "user_id", userID)

	return &omnitunv1.EnrollMFAResponse{
		Secret:    secret,
		QrCodeUrl: qrURL,
	}, nil
}

func (s *Service) VerifyMFA(ctx context.Context, req *omnitunv1.VerifyMFARequest) (*omnitunv1.VerifyMFAResponse, error) {
	userID, ok := GetUserID(ctx)
	if !ok {
		return nil, apperrors.Unauthorized("user not authenticated")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.NotFound("user not found")
	}

	if !user.MFASecret.Valid {
		s.logger.WarnContext(ctx, "verify MFA: no MFA secret found", "user_id", userID)
		return &omnitunv1.VerifyMFAResponse{Success: false}, nil
	}

	if !ValidateTOTPCode(user.MFASecret.String, req.Code) {
		s.logger.WarnContext(ctx, "verify MFA: invalid code", "user_id", userID)
		return &omnitunv1.VerifyMFAResponse{Success: false}, apperrors.BadRequest("invalid MFA code")
	}

	if err := s.repo.UpdateMFA(ctx, userID, true, user.MFASecret.String); err != nil {
		s.logger.ErrorContext(ctx, "verify MFA: failed to enable MFA", "error", err)
		return nil, apperrors.Internal("failed to enable MFA")
	}

	s.logger.InfoContext(ctx, "MFA verified and enabled", "user_id", userID)

	return &omnitunv1.VerifyMFAResponse{Success: true}, nil
}

func (s *Service) DisableMFA(ctx context.Context) error {
	userID, ok := GetUserID(ctx)
	if !ok {
		return apperrors.Unauthorized("user not authenticated")
	}

	if err := s.repo.UpdateMFA(ctx, userID, false, ""); err != nil {
		s.logger.ErrorContext(ctx, "failed to disable MFA", "error", err)
		return apperrors.Internal("failed to disable MFA")
	}

	s.logger.InfoContext(ctx, "MFA disabled", "user_id", userID)
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, req *omnitunv1.RequestPasswordResetRequest) (*omnitunv1.RequestPasswordResetResponse, error) {
	s.logger.InfoContext(ctx, "password reset requested", "email", req.Email)

	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.WarnContext(ctx, "password reset: user not found", "email", req.Email)
		return &omnitunv1.RequestPasswordResetResponse{
			Message: "if the email exists, a password reset link has been sent",
		}, nil
	}

	resetToken, err := generateResetToken()
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to generate reset token", "error", err)
		return nil, apperrors.Internal("failed to generate reset token")
	}

	tokenHash := HashToken(resetToken)
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := s.repo.StorePasswordResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		s.logger.ErrorContext(ctx, "failed to store reset token", "error", err)
		return nil, apperrors.Internal("failed to store reset token")
	}

	s.logger.InfoContext(ctx, "password reset token generated (email would be sent in production)",
		"user_id", user.ID,
		"reset_token", resetToken,
		"expires_at", expiresAt,
	)

	return &omnitunv1.RequestPasswordResetResponse{
		Message: "if the email exists, a password reset link has been sent",
	}, nil
}

func (s *Service) ResetPassword(ctx context.Context, req *omnitunv1.ResetPasswordRequest) (*omnitunv1.ResetPasswordResponse, error) {
	tokenHash := HashToken(req.Token)

	record, err := s.repo.GetPasswordResetToken(ctx, tokenHash)
	if err != nil {
		s.logger.WarnContext(ctx, "reset password: invalid or expired token")
		return nil, apperrors.BadRequest("invalid or expired reset token")
	}

	if err := ValidatePassword(req.NewPassword); err != nil {
		s.logger.WarnContext(ctx, "reset password: weak password")
		return nil, apperrors.BadRequest(err.Error())
	}

	passwordHash, err := HashPassword(req.NewPassword)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to hash new password", "error", err)
		return nil, apperrors.Internal("failed to process password")
	}

	if err := s.repo.UpdateUserPassword(ctx, record.UserID, passwordHash); err != nil {
		s.logger.ErrorContext(ctx, "failed to update password", "error", err)
		return nil, apperrors.Internal("failed to update password")
	}

	if err := s.repo.ConsumePasswordResetToken(ctx, tokenHash); err != nil {
		s.logger.WarnContext(ctx, "failed to consume reset token", "error", err)
	}

	s.logger.InfoContext(ctx, "password reset successfully", "user_id", record.UserID)

	return &omnitunv1.ResetPasswordResponse{
		Message: "password reset successfully",
	}, nil
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func emailToSlug(email string) string {
	slug := ""
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			slug += string(r)
		} else if r == '@' || r == '.' || r == '_' {
			slug += "-"
		}
	}
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}

func generateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func userToProto(user *User) *omnitunv1.User {
	return &omnitunv1.User{
		Id:             user.ID,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		OrganizationId: user.OrganizationID,
		Role:           user.Role,
		MfaEnabled:     user.MFAEnabled,
		CreatedAt:      timestamppb.New(user.CreatedAt),
	}
}
