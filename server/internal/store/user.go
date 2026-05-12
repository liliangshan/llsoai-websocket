package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        uint64    `json:"userId"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type UserStore struct {
	db *sql.DB
}

func NewDB(dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	}
	if maxLifetime > 0 {
		db.SetConnMaxLifetime(maxLifetime)
	}
	return db, db.Ping()
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword 验证密码
func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// CreateUser 创建新用户（用户名唯一）
// DB 返回底层数据库连接
func (s *UserStore) DB() *sql.DB {
	return s.db
}

func (s *UserStore) CreateUser(ctx context.Context, username, password, email string) (*User, error) {
	if username == "" || password == "" || email == "" {
		return nil, errors.New("username, password and email are required")
	}
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password, email, created_at) VALUES (?, ?, ?, NOW())`,
		username, hashedPassword, email)
	if err != nil {
		if isDuplicateErr(err) {
			return nil, errors.New("username or email already exists")
		}
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &User{ID: uint64(id), Username: username, Email: email}, nil
}

// FindByUsername 根据用户名查找用户
func (s *UserStore) FindByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, created_at FROM users WHERE username = ? LIMIT 1`, username)
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID 根据 ID 查找用户
func (s *UserStore) FindByID(ctx context.Context, id uint64) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, created_at FROM users WHERE id = ? LIMIT 1`, id)
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func isDuplicateErr(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062"))
}

func (s *UserStore) FindByWebSocketToken(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	hash := HashToken(token)
	row := s.db.QueryRowContext(ctx, `
SELECT id, username
FROM users
WHERE websocket_token_hash = ?
  AND websocket_token_disabled_at IS NULL
LIMIT 1`, hash)
	var user User
	if err := row.Scan(&user.ID, &user.Username); err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE users SET websocket_token_last_used_at = NOW() WHERE id = ?`, user.ID)
	return &user, nil
}

// generateRandomToken 生成 32 字节随机 token，带 "wsk_" 前缀
func generateRandomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "wsk_" + hex.EncodeToString(buf), nil
}

// GetOrCreateWebSocketToken 获取用户的 WebSocket 明文令牌
// 如果不存在或已被禁用，则自动生成新的并写入数据库
func (s *UserStore) GetOrCreateWebSocketToken(ctx context.Context, userID uint64) (string, error) {
	if userID == 0 {
		return "", errors.New("invalid user id")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT websocket_token, websocket_token_disabled_at
FROM users
WHERE id = ?
LIMIT 1`, userID)
	var token sql.NullString
	var disabledAt sql.NullTime
	if err := row.Scan(&token, &disabledAt); err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("user not found")
		}
		return "", err
	}
	if token.Valid && token.String != "" && !disabledAt.Valid {
		return token.String, nil
	}
	return s.RotateWebSocketToken(ctx, userID)
}

// RotateWebSocketToken 重新生成 WebSocket 令牌，覆盖旧的
func (s *UserStore) RotateWebSocketToken(ctx context.Context, userID uint64) (string, error) {
	if userID == 0 {
		return "", errors.New("invalid user id")
	}
	plain, err := generateRandomToken()
	if err != nil {
		return "", err
	}
	hash := HashToken(plain)
	prefix := plain
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE users
SET websocket_token = ?,
    websocket_token_hash = ?,
    websocket_token_prefix = ?,
    websocket_token_rotated_at = NOW(),
    websocket_token_disabled_at = NULL,
    websocket_token_last_used_at = NULL
WHERE id = ?`, plain, hash, prefix, userID)
	if err != nil {
		return "", err
	}
	return plain, nil
}
