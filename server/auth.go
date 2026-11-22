package server

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtKeyFilename = "jwt.key"
	authPassKey    = "auth.password_hash"
	// authVerKey stores the current session epoch. Tokens carry it at issuance;
	// a bump invalidates every previously issued token (password change, manual
	// revoke) without rotating the signing key or touching other data.
	authVerKey = "auth.token_version"
	jwtTTL     = 7 * 24 * time.Hour
	keyChars   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// 下限与 setup 页的前端校验一致——校验只放在前端等于没放，直接打 API 就能
	// 绕过。上限是 bcrypt 的硬限制：超过 72 字节 GenerateFromPassword 会返回
	// ErrPasswordTooLong，提前挡掉好过让用户收到一句含义不明的「密码加密失败」。
	minPasswordRunes = 8
	maxPasswordBytes = 72
)

// errDataSourceUnavailable 是密码相关读操作失败时统一的回复。这些 handler 绝不能
// 把"读不到"当成"没有设置"：authInit 曾因此在数据库报错时放行，让未认证请求覆盖
// 掉已有的管理员密码。
const errDataSourceUnavailable = "数据源暂时不可用，请稍后重试"

// validatePassword 返回空串表示通过，否则返回可直接展示给用户的中文原因。
func validatePassword(pw string) string {
	if utf8.RuneCountInString(pw) < minPasswordRunes {
		return fmt.Sprintf("密码长度至少 %d 位", minPasswordRunes)
	}
	if len(pw) > maxPasswordBytes {
		return fmt.Sprintf("密码长度不能超过 %d 字节", maxPasswordBytes)
	}
	return ""
}

// loadOrCreateJWTKey reads the 32-byte signing key from keyDir/jwt.key. keyDir
// must be a persistent, non-browsable location: the Docker deployment mounts a
// dedicated volume there so container re-creation doesn't log everyone out, and
// it is never inside the workspace root (dataDir) where the file manager could
// list or download it. Legacy builds kept the key at dataDir/jwt.key or next to
// the executable (BaseDir, passed via legacyDirs); a key found there is migrated
// once (key preserved, so sessions stay valid) and the old file removed. On first
// run a random key is generated and persisted.
func loadOrCreateJWTKey(keyDir, dataDir string, legacyDirs ...string) ([]byte, error) {
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return nil, fmt.Errorf("create jwt key dir: %w", err)
	}
	path := filepath.Join(keyDir, jwtKeyFilename)
	for _, dir := range append([]string{dataDir}, legacyDirs...) {
		migrateJWTKey(path, filepath.Join(dir, jwtKeyFilename))
	}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) >= 32 {
		return []byte(strings.TrimSpace(string(data))), nil
	}
	buf := make([]byte, 32)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(keyChars))))
		if err != nil {
			return nil, fmt.Errorf("generate jwt key: %w", err)
		}
		buf[i] = keyChars[n.Int64()]
	}
	if err := os.WriteFile(path, buf, 0600); err != nil {
		return nil, fmt.Errorf("write jwt key: %w", err)
	}
	log.Printf("[auth] 新 JWT key 已写入 %s", path)
	return buf, nil
}

// migrateJWTKey copies a valid legacy key to dst (never overwriting an existing
// key) and removes the old copy so it stops being exposed.
func migrateJWTKey(dst, src string) {
	if dst == src {
		return
	}
	if _, err := os.Stat(dst); err == nil {
		return
	}
	data, err := os.ReadFile(src)
	if err != nil || len(strings.TrimSpace(string(data))) < 32 {
		return
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		log.Printf("[auth] JWT key 迁移失败 %s -> %s: %v", src, dst, err)
		return
	}
	_ = os.Remove(src)
	log.Printf("[auth] JWT key 已从 %s 迁移到 %s", src, dst)
}

// authTokenClaims is the admin session token payload. Version is compared with
// authVerKey on every authenticated request, so bumping the stored epoch revokes
// all tokens issued before it.
type authTokenClaims struct {
	Version int64 `json:"ver"`
	jwt.RegisteredClaims
}

// signJWT issues a 7-day HS256 token tagged with the current session epoch.
func signJWT(key []byte, version int64) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, authTokenClaims{
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "ARTEX",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}).SignedString(key)
}

func parseAuthToken(tokenStr string, key []byte) (authTokenClaims, bool) {
	var claims authTokenClaims
	t, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return key, nil
	})
	if err != nil || !t.Valid {
		return authTokenClaims{}, false
	}
	return claims, true
}

// verifyJWT returns true when tokenStr is a valid, non-expired HS256 token.
// It checks signature/expiry only; session revocation is enforced by
// verifyAdminJWT (which also compares the token's version).
func verifyJWT(tokenStr string, key []byte) bool {
	_, ok := parseAuthToken(tokenStr, key)
	return ok
}

// loadAuthTokenVersion reads the session epoch from the settings table. Absent
// or malformed (admin-editable) → 0, so a corrupted value cannot lock everyone out.
// A Server with no settings store at all (test wiring; production NewManager
// fails hard without PG) has no persisted epoch either, so it also reads as 0 —
// unlike a store that exists but errors, which must surface as 503.
func (s *Server) loadAuthTokenVersion() (int64, error) {
	if s == nil || s.m == nil || s.m.PG() == nil {
		return 0, nil
	}
	raw, ok, err := s.m.PG().GetSetting(authVerKey)
	if err != nil {
		return 0, err
	}
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		log.Printf("[auth] token 版本值无效(%q)，按 0 处理", raw)
		return 0, nil
	}
	return n, nil
}

// authTokenVersion returns the cached session epoch, loading it once from the DB.
func (s *Server) authTokenVersion() (int64, error) {
	if s.authVerLoaded.Load() {
		return s.authVer.Load(), nil
	}
	s.authVerMu.Lock()
	defer s.authVerMu.Unlock()
	if s.authVerLoaded.Load() {
		return s.authVer.Load(), nil
	}
	v, err := s.loadAuthTokenVersion()
	if err != nil {
		return 0, err
	}
	s.authVer.Store(v)
	s.authVerLoaded.Store(true)
	return v, nil
}

// verifyAdminJWT validates signature/expiry and the session epoch. A DB error is
// returned separately so callers can answer 503 instead of a misleading 401.
func (s *Server) verifyAdminJWT(tokenStr string) (bool, error) {
	if tokenStr == "" {
		return false, nil
	}
	claims, ok := parseAuthToken(tokenStr, s.jwtKey)
	if !ok {
		return false, nil
	}
	ver, err := s.authTokenVersion()
	if err != nil {
		return false, err
	}
	return claims.Version == ver, nil
}

// bumpAuthTokenVersion persists epoch+1 and updates the cache, revoking every
// token issued with an earlier epoch.
func (s *Server) bumpAuthTokenVersion() (int64, error) {
	if s == nil || s.m == nil || s.m.PG() == nil {
		return 0, fmt.Errorf("数据库不可用")
	}
	s.authVerMu.Lock()
	defer s.authVerMu.Unlock()
	cur, err := s.loadAuthTokenVersion()
	if err != nil {
		return 0, err
	}
	next := cur + 1
	if err := s.m.PG().SetSetting(authVerKey, strconv.FormatInt(next, 10)); err != nil {
		return 0, err
	}
	s.authVer.Store(next)
	s.authVerLoaded.Store(true)
	return next, nil
}

// extractToken reads the JWT from Authorization: Bearer header,
// artex_token cookie, or ?token= query param (for SSE connections).
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("artex_token"); err == nil && c.Value != "" {
		return c.Value
	}
	return r.URL.Query().Get("token")
}

// requireAuth wraps h with JWT validation.
// /api/auth/*, /api/health and the Basic Auth gate check are exempt.
func (s *Server) requireAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/auth/") || p == "/api/health" || p == "/api/basic-auth/check" {
			h.ServeHTTP(w, r)
			return
		}
		tok := extractToken(r)
		if tok == "" {
			writeProductError(w, 401, "unauthorized", "未授权", nil)
			return
		}
		ok, err := s.verifyAdminJWT(tok)
		if err != nil {
			writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
			return
		}
		if !ok {
			writeProductError(w, 401, "token_expired", "token 无效或已过期", nil)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// GET /api/auth/status — reports whether the admin password has been initialised.
// 读失败必须回 503 而不是 initialized:false：前端在 initialized:false 时会把用户
// 送到 /setup 去设置密码（login/page.tsx），把数据库故障包装成 200 等于把用户往
// 覆盖已有密码的路上推。
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	hash, _, err := pg.GetSetting(authPassKey)
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	writeJSON(w, 200, map[string]any{"initialized": hash != ""})
}

// POST /api/auth/init — sets the password for the first time; rejected if already set.
func (s *Server) authInit(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	existing, _, err := pg.GetSetting(authPassKey)
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	if existing != "" {
		writeProductError(w, 403, "password_already_set", "密码已设置", nil)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil || req.Password == "" {
		writeProductError(w, 400, "password_required", "密码不能为空", nil)
		return
	}
	if msg := validatePassword(req.Password); msg != "" {
		code, params := passwordValidationCode(req.Password)
		writeProductError(w, 400, code, msg, params)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeProductError(w, 500, "password_hash_failed", "密码加密失败", nil)
		return
	}
	// 用 INSERT ... ON CONFLICT DO NOTHING 而不是 upsert：上面那次 GetSetting 只是
	// 快速失败路径，真正"仅首次可设"的保证落在主键约束上。bcrypt 要跑几十毫秒，
	// 这期间别的请求完全可能先把密码设好，而读检查本身也可能因故障而失效。
	inserted, err := pg.InsertSettingIfAbsent(authPassKey, string(hash))
	if err != nil {
		writeProductError(w, 500, "save_failed", "保存失败: "+err.Error(), nil)
		return
	}
	if !inserted {
		writeProductError(w, 403, "password_already_set", "密码已设置", nil)
		return
	}
	ver, err := s.authTokenVersion()
	if err != nil {
		writeProductError(w, 503, "data_unavailable", "密码已设置，但读取会话版本失败，请重新登录", nil)
		return
	}
	tok, err := signJWT(s.jwtKey, ver)
	if err != nil {
		writeProductError(w, 500, "token_failed", "token 生成失败", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"token": tok})
}

// POST /api/auth/change-password — changes the admin password. Requires a valid
// token (this route is under /api/auth/* which requireAuth exempts, so the token
// is validated here) AND the current password.
func (s *Server) authChangePassword(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	ok, err := s.verifyAdminJWT(extractToken(r))
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	if !ok {
		writeProductError(w, 401, "unauthorized", "未授权", nil)
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decode(r, &req); err != nil {
		writeProductError(w, 400, "invalid_request", "请求格式错误", nil)
		return
	}
	if req.NewPassword == "" {
		writeProductError(w, 400, "new_password_required", "新密码不能为空", nil)
		return
	}
	if msg := validatePassword(req.NewPassword); msg != "" {
		code, params := passwordValidationCode(req.NewPassword)
		writeProductError(w, 400, code, msg, params)
		return
	}
	hash, ok, err := pg.GetSetting(authPassKey)
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	if !ok || hash == "" {
		writeProductError(w, 403, "password_not_initialized", "密码未初始化，请先设置密码", nil)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)); err != nil {
		writeProductError(w, 401, "current_password_incorrect", "当前密码错误", nil)
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeProductError(w, 500, "password_hash_failed", "密码加密失败", nil)
		return
	}
	if err := pg.SetSetting(authPassKey, string(newHash)); err != nil {
		writeProductError(w, 500, "save_failed", "保存失败: "+err.Error(), nil)
		return
	}
	// A password change revokes every existing session, then hands the caller a
	// fresh token so this browser stays logged in.
	next, err := s.bumpAuthTokenVersion()
	if err != nil {
		writeProductError(w, 500, "revoke_failed", "密码已修改，但撤销旧会话失败，请重试: "+err.Error(), nil)
		return
	}
	tok, err := signJWT(s.jwtKey, next)
	if err != nil {
		writeProductError(w, 500, "token_failed", "密码已修改，请重新登录", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "token": tok})
}

// POST /api/auth/revoke — invalidates every issued admin token (this request's
// included). Used to kill sessions after a token leak. The caller must log in
// again; password change already performs the same bump.
func (s *Server) authRevokeSessions(w http.ResponseWriter, r *http.Request) {
	if s.pg(w) == nil {
		return
	}
	ok, err := s.verifyAdminJWT(extractToken(r))
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	if !ok {
		writeProductError(w, 401, "unauthorized", "未授权", nil)
		return
	}
	next, err := s.bumpAuthTokenVersion()
	if err != nil {
		writeProductError(w, 500, "revoke_failed", "撤销失败: "+err.Error(), nil)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "token_version": next})
}

// POST /api/auth/login — validates username/password and returns a JWT.
func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	key := loginClientKey(r, pg)
	if wait := s.loginLimiter.lockedFor(key); wait > 0 {
		retryAfter := int((wait + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeProductError(w, http.StatusTooManyRequests, "too_many_attempts", "登录失败次数过多，请稍后再试", map[string]any{"retry_after": retryAfter})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		writeProductError(w, 400, "invalid_request", "请求格式错误", nil)
		return
	}
	if req.Username != "ARTEX" {
		s.loginLimiter.fail(key)
		writeProductError(w, 401, "invalid_credentials", "用户名或密码错误", nil)
		return
	}
	hash, ok, err := pg.GetSetting(authPassKey)
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	if !ok || hash == "" {
		writeProductError(w, 403, "password_not_initialized", "密码未初始化，请先设置密码", nil)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		s.loginLimiter.fail(key)
		writeProductError(w, 401, "invalid_credentials", "用户名或密码错误", nil)
		return
	}
	s.loginLimiter.reset(key)
	ver, err := s.authTokenVersion()
	if err != nil {
		writeProductError(w, 503, "data_unavailable", errDataSourceUnavailable, nil)
		return
	}
	tok, err := signJWT(s.jwtKey, ver)
	if err != nil {
		writeProductError(w, 500, "token_failed", "token 生成失败", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"token": tok})
}

func passwordValidationCode(password string) (string, map[string]any) {
	if utf8.RuneCountInString(password) < minPasswordRunes {
		return "password_too_short", map[string]any{"min": minPasswordRunes}
	}
	return "password_too_long", map[string]any{"max": maxPasswordBytes}
}
