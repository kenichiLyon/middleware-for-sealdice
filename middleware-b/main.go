package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ListenHTTP    string `json:"listen_http"`
	StorageDir    string `json:"storage_dir"`
	PublicBaseURL string `json:"public_base_url"`
	LogLevel      string `json:"log_level"`
	LogFile       string `json:"log_file"`
	LogFormat     string `json:"log_format"`
	LogConsole    bool   `json:"log_console"`
}

var (
	levelVarB = new(slog.LevelVar)
	loggerB   *slog.Logger
)

func initLoggerBDefault() {
	levelVarB.Set(slog.LevelInfo)
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: levelVarB})
	loggerB = slog.New(h).With("component", "middleware-b")
	slog.SetDefault(loggerB)
}

func initLoggerBFromConfig(cfg *Config) {
	switch strings.ToLower(strings.TrimSpace(cfg.LogLevel)) {
	case "debug":
		levelVarB.Set(slog.LevelDebug)
	case "warn":
		levelVarB.Set(slog.LevelWarn)
	case "error":
		levelVarB.Set(slog.LevelError)
	default:
		levelVarB.Set(slog.LevelInfo)
	}
	out := []io.Writer{}
	if cfg.LogConsole {
		out = append(out, os.Stdout)
	}
	lf := cfg.LogFile
	if lf == "" {
		_ = os.MkdirAll("logs", 0o755)
		lf = filepath.Join("logs", "middleware-b.log")
	}
	if lf != "" {
		_ = os.MkdirAll(filepath.Dir(lf), 0o755)
		if f, err := os.OpenFile(lf, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			out = append(out, f)
		}
	}
	var w io.Writer = os.Stdout
	if len(out) > 0 {
		w = io.MultiWriter(out...)
	}
	var h slog.Handler
	if strings.ToLower(strings.TrimSpace(cfg.LogFormat)) == "text" {
		h = slog.NewTextHandler(w, &slog.HandlerOptions{Level: levelVarB})
	} else {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: levelVarB})
	}
	loggerB = slog.New(h).With("component", "middleware-b")
	slog.SetDefault(loggerB)
}

type statusRecorderB struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorderB) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusRecorderB) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return n, err
}

func withHTTPLoggingB(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := fmt.Sprintf("%x", time.Now().UnixNano())
		rw := &statusRecorderB{ResponseWriter: w}
		loggerB.Info("http start", "rid", rid, "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		h.ServeHTTP(rw, r)
		dur := time.Since(start)
		loggerB.Info("http end", "rid", rid, "status", rw.status, "bytes", rw.bytes, "duration", dur)
	})
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	dec := json.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	if cfg.ListenHTTP == "" {
		cfg.ListenHTTP = ":8082"
	}
	if cfg.StorageDir == "" {
		cfg.StorageDir = "uploads"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "json"
	}
	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join("logs", "middleware-b.log")
	}
	if !cfg.LogConsole {
		cfg.LogConsole = true
	}
	return &cfg, nil
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.json", "config path")
	flag.Parse()
	initLoggerBDefault()

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		loggerB.Error("加载配置失败", "err", err)
		os.Exit(1)
	}
	initLoggerBFromConfig(cfg)
	if err := os.MkdirAll(cfg.StorageDir, 0o755); err != nil {
		loggerB.Error("创建存储目录失败", "err", err)
		os.Exit(1)
	}

	http.Handle("/upload", withHTTPLoggingB(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
			loggerB.Error("解析表单失败", "err", err)
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("form file: %v", err), http.StatusBadRequest)
			loggerB.Error("获取表单文件失败", "err", err)
			return
		}
		defer file.Close()
		name := hdr.Filename
		if n := r.FormValue("name"); n != "" {
			name = n
		}
		// create dated dir
		sub := time.Now().Format("2006/01/02")
		dir := filepath.Join(cfg.StorageDir, sub)
		if err = os.MkdirAll(dir, 0o755); err != nil {
			http.Error(w, fmt.Sprintf("mkdir: %v", err), http.StatusInternalServerError)
			loggerB.Error("创建目录失败", "err", err)
			return
		}
		// ensure clean name
		name = filepath.Base(name)
		safeName := strings.ReplaceAll(name, " ", "_")
		outPath := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName))
		out, err := os.Create(outPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("create: %v", err), http.StatusInternalServerError)
			loggerB.Error("创建文件失败", "err", err)
			return
		}
		defer out.Close()
		wrote, copyErr := io.Copy(out, file)
		if copyErr != nil {
			http.Error(w, fmt.Sprintf("write: %v", copyErr), http.StatusInternalServerError)
			loggerB.Error("写入文件失败", "err", copyErr)
			return
		}

		absOut := outPath
		if !filepath.IsAbs(absOut) {
			if a, err := filepath.Abs(absOut); err == nil {
				absOut = a
			}
		}
		rel, err := filepath.Rel(cfg.StorageDir, outPath)
		if err != nil {
			rel = strings.TrimPrefix(outPath, cfg.StorageDir)
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimLeft(rel, "/")
		publicURL := fmt.Sprintf("%s/files/%s", strings.TrimRight(cfg.PublicBaseURL, "/"), rel)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"url":        publicURL,
			"name":       name,
			"local_path": absOut,
		}); err != nil {
			loggerB.Error("编码响应失败", "err", err)
		} else {
			loggerB.Info("upload success", "name", name, "bytes", wrote, "local_path", absOut, "url", publicURL)
		}
	})))

	fs := http.FileServer(http.Dir(cfg.StorageDir))
	http.Handle("/files/", withHTTPLoggingB(http.StripPrefix("/files/", fs)))

	loggerB.Info("服务启动", "http", cfg.ListenHTTP, "storage", cfg.StorageDir)
	if err := http.ListenAndServe(cfg.ListenHTTP, nil); err != nil {
		loggerB.Error("HTTP 服务启动失败", "err", err)
		os.Exit(1)
	}
}
