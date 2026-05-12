package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"llsoai-websocket/server/internal/config"
	"llsoai-websocket/server/internal/httpapi"
	"llsoai-websocket/server/internal/store"
	"llsoai-websocket/server/internal/wsserver"
)

// webFS 嵌入前端构建产物。`build.sh` 会在编译前把 web/dist 的内容复制到 cmd/server/web/。
//
//go:embed all:web
var webFS embed.FS

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	db, err := store.NewDB(cfg.Database.DSN(), cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime)
	if err != nil {
		log.Fatalf("connect database failed: %v", err)
	}
	defer db.Close()

	hub := wsserver.NewHub()
	users := store.NewUserStore(db)
	wsHandler := wsserver.NewHandler(hub, users, cfg.WebSocket)
	auth := httpapi.NewAuthenticator(cfg.Auth, users)
	api := httpapi.NewHandler(hub, auth, cfg.HTTP)

	mux := http.NewServeMux()
	mux.Handle(cfg.WebSocket.Path, wsHandler)
	api.Register(mux)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 静态前端 (SPA) —— 必须最后注册，作为兜底 "/"，且不能拦截上面已注册的精确前缀。
	spa, err := newSPAHandler(webFS, "web")
	if err != nil {
		log.Fatalf("init spa handler failed: %v", err)
	}
	mux.Handle("/", spa)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			hub.CleanupExpired()
		}
	}()

	server := &http.Server{Addr: cfg.Addr(), Handler: withCORS(mux), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("server listening on %s", cfg.Addr())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

// newSPAHandler 返回一个用于服务 SPA 的 http.Handler：
//   - 真实存在的文件直接 404 兜底前 ServeFile 提供，并对带 hash 的静态资源加上长期缓存
//   - 路径不存在时回退到 index.html（让前端路由生效）
//
// embedRoot 是嵌入文件系统中的目录前缀，例如 "web"。
func newSPAHandler(fsys embed.FS, embedRoot string) (http.Handler, error) {
	sub, err := fs.Sub(fsys, embedRoot)
	if err != nil {
		return nil, err
	}
	httpFS := http.FS(sub)
	fileServer := http.FileServer(httpFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		clean := path.Clean("/" + strings.TrimPrefix(urlPath, "/"))
		// 根路径直接交给 FileServer 处理（会返回 index.html）
		if clean == "/" {
			setStaticCacheHeaders(w, clean)
			fileServer.ServeHTTP(w, r)
			return
		}
		// 检查请求路径在嵌入文件系统中是否存在
		rel := strings.TrimPrefix(clean, "/")
		if f, err := sub.Open(rel); err == nil {
			info, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !info.IsDir() {
				setStaticCacheHeaders(w, clean)
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// 文件不存在 —— SPA fallback：返回 index.html，HTTP 状态仍为 200，让前端路由接管
		indexBytes, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	}), nil
}

// setStaticCacheHeaders 给带内容 hash 的静态资源加长期缓存；其它文件不做强缓存避免发版后用户拿不到新页面。
func setStaticCacheHeaders(w http.ResponseWriter, p string) {
	if strings.HasPrefix(p, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if strings.HasSuffix(p, ".html") || p == "/" {
		w.Header().Set("Cache-Control", "no-cache")
	}
}

// withCORS 给所有 HTTP 路由统一加上跨域响应头：
//   - 回显请求 Origin（任意来源），无 Origin 时回 "*"
//   - 允许常见方法与自定义头（含 Authorization）
//   - 暴露 Content-Type / SSE 常用头
//   - OPTIONS 预检直接 204 返回
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		h := w.Header()
		if origin != "" {
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Credentials", "true")
		} else {
			h.Set("Access-Control-Allow-Origin", "*")
		}
		if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
			h.Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, Accept, Cache-Control, Last-Event-ID")
		}
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		h.Set("Access-Control-Expose-Headers", "Content-Type, Content-Length, X-Request-Id")
		h.Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
