package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"llsoai-websocket/server/internal/config"
	"llsoai-websocket/server/internal/store"

	"github.com/golang-jwt/jwt/v5"
)

// --- Helper functions ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// Claims JWT claims
type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Authenticator JWT 认证器
type Authenticator struct {
	jwtSecret  []byte
	expiration time.Duration
	userStore  *store.UserStore
	apiKeys    map[string]uint64 // key hash -> userID
}

// NewAuthenticator 创建认证器
func NewAuthenticator(cfg config.AuthConfig, userStore *store.UserStore) *Authenticator {
	apiKeys := make(map[string]uint64)
	for _, ak := range cfg.APIKeys {
		h := store.HashToken(ak.Key)
		apiKeys[h] = ak.UserID
	}
	return &Authenticator{
		jwtSecret:  []byte(cfg.JWT.Secret),
		expiration: cfg.JWT.Expiration,
		userStore:  userStore,
		apiKeys:    apiKeys,
	}
}

// GenerateToken 生成 JWT token
func (a *Authenticator) GenerateToken(ctx context.Context, userID uint64, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

// ValidateToken 验证 JWT token，返回 userID
func (a *Authenticator) ValidateToken(tokenStr string) (uint64, string, error) {
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return 0, "", err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, "", errors.New("invalid token")
	}
	return claims.UserID, claims.Username, nil
}

// UserIDFromRequest 从请求中提取用户 ID
// 优先级：1) JWT Bearer token  2) X-API-Key header
func (a *Authenticator) UserID(r *http.Request) (uint64, error) {
	// 优先尝试 JWT Bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		userID, _, err := a.ValidateToken(authHeader)
		if err == nil {
			return userID, nil
		}
	}

	// 回退到 API Key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		return 0, errors.New("missing auth token")
	}
	h := store.HashToken(apiKey)
	userID, ok := a.apiKeys[h]
	if !ok {
		return 0, errors.New("invalid api key")
	}
	return userID, nil
}

// Login 登录处理
func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	user, err := a.userStore.FindByUsername(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	// 从 users 表查询带 password 的记录（FindByUsername 没有返回 password）
	// 我们需要用另一种方式：查 password
	row := a.userStore.DB().QueryRowContext(r.Context(),
		`SELECT password FROM users WHERE username = ? LIMIT 1`, req.Username)
	var hashedPassword string
	if err := row.Scan(&hashedPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !store.CheckPassword(hashedPassword, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := a.GenerateToken(r.Context(), user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Register 注册处理
func (a *Authenticator) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	user, err := a.userStore.CreateUser(r.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	token, err := a.GenerateToken(r.Context(), user.ID, user.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Me 获取当前用户
func (a *Authenticator) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := a.UserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := a.userStore.FindByID(r.Context(), userID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// WebSocketToken 获取当前用户的 WebSocket 令牌（明文），不存在则自动生成
func (a *Authenticator) WebSocketToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, err := a.UserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	token, err := a.userStore.GetOrCreateWebSocketToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get websocket token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
	})
}

// RotateWebSocketToken 重新生成 WebSocket 令牌
func (a *Authenticator) RotateWebSocketToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID, err := a.UserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	token, err := a.userStore.RotateWebSocketToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rotate websocket token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
	})
}
