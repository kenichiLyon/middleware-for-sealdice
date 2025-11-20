package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	ListenHTTP            string `json:"listen_http"`
	ListenWSPath          string `json:"listen_ws_path"`
	UpstreamWSURL         string `json:"upstream_ws_url"`
	UpstreamAccessToken   string `json:"upstream_access_token"`
	UpstreamUseQueryToken bool   `json:"upstream_use_query_token"`
	ServerAccessToken     string `json:"server_access_token"`
	UploadEndpoint        string `json:"upload_endpoint"`
	LogLevel              string `json:"log_level"`
	LogFile               string `json:"log_file"`
	LogFormat             string `json:"log_format"`
	LogConsole            bool   `json:"log_console"`
}

var (
	levelVarA = new(slog.LevelVar)
	loggerA   *slog.Logger
)

func initLoggerADefault() {
	levelVarA.Set(slog.LevelInfo)
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: levelVarA})
	loggerA = slog.New(h).With("component", "middleware-a")
	slog.SetDefault(loggerA)
}

func initLoggerAFromConfig(cfg *Config) {
	switch strings.ToLower(strings.TrimSpace(cfg.LogLevel)) {
	case "debug":
		levelVarA.Set(slog.LevelDebug)
	case "warn":
		levelVarA.Set(slog.LevelWarn)
	case "error":
		levelVarA.Set(slog.LevelError)
	default:
		levelVarA.Set(slog.LevelInfo)
	}
	out := []io.Writer{}
	if cfg.LogConsole {
		out = append(out, os.Stdout)
	}
	lf := cfg.LogFile
	if lf == "" {
		_ = os.MkdirAll("logs", 0o755)
		lf = filepath.Join("logs", "middleware-a.log")
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
		h = slog.NewTextHandler(w, &slog.HandlerOptions{Level: levelVarA})
	} else {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: levelVarA})
	}
	loggerA = slog.New(h).With("component", "middleware-a")
	slog.SetDefault(loggerA)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorder) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusRecorder) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return n, err
}

func withHTTPLogging(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := fmt.Sprintf("%x", time.Now().UnixNano())
		rw := &statusRecorder{ResponseWriter: w}
		loggerA.Info("http start", "rid", rid, "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		h(rw, r)
		dur := time.Since(start)
		loggerA.Info("http end", "rid", rid, "status", rw.status, "bytes", rw.bytes, "duration", dur)
	}
}

type oneBotCommand struct {
	Action string      `json:"action"`
	Params interface{} `json:"params"`
	Echo   interface{} `json:"echo"`
}

type uploadPrivateFileParams struct {
	UserID int64  `json:"user_id"`
	File   string `json:"file"`
	Name   string `json:"name"`
}

type uploadGroupFileParams struct {
	GroupID int64  `json:"group_id"`
	File    string `json:"file"`
	Name    string `json:"name"`
}

type sendPrivateMsgParams struct {
	UserID  int64  `json:"user_id"`
	Message string `json:"message"`
}

type sendGroupMsgParams struct {
	GroupID int64  `json:"group_id"`
	Message string `json:"message"`
}

func rewriteCQMediaInText(s string, cfg *Config) string {
	re := regexp.MustCompile(`\[CQ:(image|record|video)([^\]]*)]`)
	return re.ReplaceAllStringFunc(s, func(seg string) string {
		m := re.FindStringSubmatch(seg)
		if len(m) < 3 {
			return seg
		}
		kind := m[1]
		argsStr := m[2]
		args := map[string]string{}
		for _, kv := range strings.Split(strings.TrimLeft(argsStr, ","), ",") {
			if kv == "" {
				continue
			}
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				args[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if u, ok := args["url"]; ok {
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				return seg
			}
		}
		file := args["file"]
		if file == "" {
			return seg
		}
		if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
			return seg
		}
		up, name := uploadViaB(file, args["name"], cfg)
		if up.URL == "" {
			return seg
		}
		args["file"] = escapeCommaMaybe(up.URL)
		if name != "" {
			args["name"] = name
		}
		// 重新构造 cqcode
		var b strings.Builder
		b.WriteString("[CQ:")
		b.WriteString(kind)
		first := true

		if v, ok := args["file"]; ok {
			b.WriteString(",file=")
			b.WriteString(v)
			first = false
		}
		for k, v := range args {
			if k == "file" {
				continue
			}
			if first {
				b.WriteString(",")
				first = false
			} else {
				b.WriteString(",")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v)
		}
		b.WriteString("]")
		return b.String()
	})
}

func rewritePictureTagInText(s string, cfg *Config) string {
	re := regexp.MustCompile(`\[图:([^\]]+)]`)
	return re.ReplaceAllStringFunc(s, func(seg string) string {
		m := re.FindStringSubmatch(seg)
		if len(m) < 2 {
			return seg
		}
		src := strings.TrimSpace(m[1])
		up, _ := uploadViaB(src, "", cfg)
		if up.URL == "" {
			return seg
		}
		return "[CQ:image,file=" + escapeCommaMaybe(up.URL) + "]"
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
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
		cfg.ListenHTTP = ":8081"
	}
	if cfg.ListenWSPath == "" {
		cfg.ListenWSPath = "/ws"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "json"
	}
	if cfg.LogFile == "" {
		cfg.LogFile = filepath.Join("logs", "middleware-a.log")
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
	initLoggerADefault()

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		loggerA.Error("加载配置失败", "err", err)
		os.Exit(1)
	}
	initLoggerAFromConfig(cfg)

	http.HandleFunc(cfg.ListenWSPath, withHTTPLogging(func(w http.ResponseWriter, r *http.Request) {
		// 鉴权对接协议端的 access_token
		if cfg.ServerAccessToken != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + cfg.ServerAccessToken
			if auth != expected {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
				loggerA.Warn("未授权访问", "remote", r.RemoteAddr, "path", r.URL.Path)
				return
			}
		}
		// 对接到海豹的 Onebot v11 正向 WS 连接
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			loggerA.Error("WebSocket 升级失败", "err", err, "remote", r.RemoteAddr)
			return
		}

		// 连接 Onebot V11 协议实现端
		header := http.Header{}
		if cfg.UpstreamAccessToken != "" && !cfg.UpstreamUseQueryToken {
			header.Set("Authorization", "Bearer "+cfg.UpstreamAccessToken)
		}
		upstreamURL := cfg.UpstreamWSURL
		if cfg.UpstreamAccessToken != "" && cfg.UpstreamUseQueryToken {
			if u, e := url.Parse(upstreamURL); e == nil {
				q := u.Query()
				q.Set("access_token", cfg.UpstreamAccessToken)
				u.RawQuery = q.Encode()
				upstreamURL = u.String()
			}
		}
		upstreamConn, _, err := websocket.DefaultDialer.Dial(upstreamURL, header)
		if err != nil {
			loggerA.Error("连接协议端失败", "err", err, "url", upstreamURL)
			_ = clientConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "upstream dial error"), timeNowPlus())
			clientConn.Close()
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					loggerA.Error("发生异常 (客户端到上游)", "err", rec)
				}
			}()
			for {
				mt, msg, err := clientConn.ReadMessage()
				if err != nil {
					loggerA.Error("读取海豹消息失败", "err", err)
					_ = upstreamConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), timeNowPlus())
					return
				}
				if mt == websocket.TextMessage {
					rewritten := rewriteIfUpload(cmdBytes(msg), cfg)
					msg = rewritten
				}
				if err := upstreamConn.WriteMessage(mt, msg); err != nil {
					loggerA.Error("写入协议端消息失败", "err", err)
					return
				}
			}
		}()

		go func() {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					loggerA.Error("发生异常 (上游到客户端)", "err", rec)
				}
			}()
			for {
				mt, msg, err := upstreamConn.ReadMessage()
				if err != nil {
					loggerA.Error("读取协议端消息失败", "err", err)
					_ = clientConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), timeNowPlus())
					return
				}
				if err := clientConn.WriteMessage(mt, msg); err != nil {
					loggerA.Error("写入海豹消息失败", "err", err)
					return
				}
			}
		}()

		wg.Wait()
		loggerA.Info("ws closed", "remote", r.RemoteAddr, "upstream", upstreamURL)
		clientConn.Close()
		upstreamConn.Close()
	}))

	loggerA.Info("服务启动", "http", cfg.ListenHTTP, "ws_path", cfg.ListenWSPath, "upstream", cfg.UpstreamWSURL)
	if err := http.ListenAndServe(cfg.ListenHTTP, nil); err != nil {
		loggerA.Error("HTTP 服务启动失败", "err", err)
		os.Exit(1)
	}
}

func timeNowPlus() (deadline time.Time) { // minimal helper to satisfy control writes
	return time.Now().Add(1 * time.Second)
}

func cmdBytes(b []byte) []byte { return b }

func rewriteIfUpload(msg []byte, cfg *Config) []byte {
	var cmd oneBotCommand
	if err := json.Unmarshal(msg, &cmd); err != nil {
		return msg
	}
	switch cmd.Action {
	case "send_msg":
		raw, _ := json.Marshal(cmd.Params)
		var p map[string]interface{}
		if json.Unmarshal(raw, &p) == nil {
			if v, ok := p["message"].(string); ok {
				nv := rewriteCQMediaInText(v, cfg)
				nv = rewritePictureTagInText(nv, cfg)
				if nv != v {
					p["message"] = nv
					newCmd := oneBotCommand{Action: "send_msg", Params: p, Echo: cmd.Echo}
					b, _ := json.Marshal(newCmd)
					return b
				}
			} else if arr, ok := p["message"].([]interface{}); ok {
				changed := false
				for i := range arr {
					el, ok := arr[i].(map[string]interface{})
					if !ok {
						continue
					}
					t, _ := el["type"].(string)
					data, _ := el["data"].(map[string]interface{})
					if t == "image" || t == "record" || t == "video" {
						if u, _ := data["url"].(string); strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
							continue
						}
						src := ""
						if f, _ := data["file"].(string); f != "" {
							src = f
						}
						if src == "" {
							if pth, _ := data["path"].(string); pth != "" {
								src = pth
							}
						}
						if src == "" {
							continue
						}
						up, _ := uploadViaB(src, "", cfg)
						if up.URL != "" {
							data["url"] = up.URL
							data["file"] = up.URL
							delete(data, "path")
							changed = true
						}
					} else if t == "text" {
						if txt, _ := data["text"].(string); txt != "" {
							nv := rewritePictureTagInText(txt, cfg)
							if nv != txt {
								data["text"] = nv
								changed = true
							}
						}
					}
				}
				if changed {
					p["message"] = arr
					newCmd := oneBotCommand{Action: "send_msg", Params: p, Echo: cmd.Echo}
					b, _ := json.Marshal(newCmd)
					return b
				}
			}
		}
		return msg
	case "send_private_msg":
		raw, _ := json.Marshal(cmd.Params)
		var p map[string]interface{}
		if json.Unmarshal(raw, &p) == nil {
			if v, ok := p["message"].(string); ok {
				nv := rewriteCQMediaInText(v, cfg)
				nv = rewritePictureTagInText(nv, cfg)
				if nv != v {
					p["message"] = nv
					newCmd := oneBotCommand{Action: "send_private_msg", Params: p, Echo: cmd.Echo}
					b, _ := json.Marshal(newCmd)
					return b
				}
			} else if arr, ok := p["message"].([]interface{}); ok {
				changed := false
				for i := range arr {
					el, ok := arr[i].(map[string]interface{})
					if !ok {
						continue
					}
					t, _ := el["type"].(string)
					data, _ := el["data"].(map[string]interface{})
					if t == "image" || t == "record" || t == "video" {
						// prefer existing http(s) url
						if u, _ := data["url"].(string); strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
							continue
						}
						// candidate source
						src := ""
						if f, _ := data["file"].(string); f != "" {
							src = f
						}
						if src == "" {
							if pth, _ := data["path"].(string); pth != "" {
								src = pth
							}
						}
						if src == "" {
							continue
						}
						up, _ := uploadViaB(src, "", cfg)
						if up.URL != "" {
							// set both url and file to remote URL to support impls that prefer 'file'
							data["url"] = up.URL
							data["file"] = up.URL
							// drop local-only path if present
							delete(data, "path")
							changed = true
						}
					} else if t == "text" {
						if txt, _ := data["text"].(string); txt != "" {
							nv := rewritePictureTagInText(txt, cfg)
							if nv != txt {
								data["text"] = nv
								changed = true
							}
						}
					}
				}
				if changed {
					p["message"] = arr
					newCmd := oneBotCommand{Action: "send_private_msg", Params: p, Echo: cmd.Echo}
					b, _ := json.Marshal(newCmd)
					return b
				}
			}
		}
		return msg
	case "send_group_msg":
		raw, _ := json.Marshal(cmd.Params)
		var p map[string]interface{}
		if json.Unmarshal(raw, &p) == nil {
			if v, ok := p["message"].(string); ok {
				nv := rewriteCQMediaInText(v, cfg)
				nv = rewritePictureTagInText(nv, cfg)
				if nv != v {
					p["message"] = nv
					newCmd := oneBotCommand{Action: "send_group_msg", Params: p, Echo: cmd.Echo}
					b, _ := json.Marshal(newCmd)
					return b
				}
			} else if arr, ok := p["message"].([]interface{}); ok {
				changed := false
				for i := range arr {
					el, ok := arr[i].(map[string]interface{})
					if !ok {
						continue
					}
					t, _ := el["type"].(string)
					data, _ := el["data"].(map[string]interface{})
					if t == "image" || t == "record" || t == "video" {
						if u, _ := data["url"].(string); strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
							continue
						}
						src := ""
						if f, _ := data["file"].(string); f != "" {
							src = f
						}
						if src == "" {
							if pth, _ := data["path"].(string); pth != "" {
								src = pth
							}
						}
						if src == "" {
							continue
						}
						up, _ := uploadViaB(src, "", cfg)
						if up.URL != "" {
							data["url"] = up.URL
							data["file"] = up.URL
							delete(data, "path")
							changed = true
						}
					} else if t == "text" {
						if txt, _ := data["text"].(string); txt != "" {
							nv := rewritePictureTagInText(txt, cfg)
							if nv != txt {
								data["text"] = nv
								changed = true
							}
						}
					}
				}
				if changed {
					p["message"] = arr
					newCmd := oneBotCommand{Action: "send_group_msg", Params: p, Echo: cmd.Echo}
					b, _ := json.Marshal(newCmd)
					return b
				}
			}
		}
		return msg
	case "upload_private_file":
		raw, _ := json.Marshal(cmd.Params)
		var p uploadPrivateFileParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return msg
		}
		up, name := uploadViaB(p.File, p.Name, cfg)
		if up.LocalPath != "" {
			newCmd := oneBotCommand{
				Action: "upload_private_file",
				Params: uploadPrivateFileParams{UserID: p.UserID, File: up.LocalPath, Name: name},
				Echo:   cmd.Echo,
			}
			b, _ := json.Marshal(newCmd)
			return b
		}
		if up.URL != "" {
			// 用 cqcode 发送
			cq := fmt.Sprintf("[CQ:file,file=%s,name=%s]", escapeCommaMaybe(up.URL), name)
			newCmd := oneBotCommand{
				Action: "send_private_msg",
				Params: sendPrivateMsgParams{UserID: p.UserID, Message: cq},
				Echo:   cmd.Echo,
			}
			b, _ := json.Marshal(newCmd)
			return b
		}
		return msg
	case "upload_group_file":
		raw, _ := json.Marshal(cmd.Params)
		var p uploadGroupFileParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return msg
		}
		up, name := uploadViaB(p.File, p.Name, cfg)
		if up.LocalPath != "" {
			newCmd := oneBotCommand{
				Action: "upload_group_file",
				Params: uploadGroupFileParams{GroupID: p.GroupID, File: up.LocalPath, Name: name},
				Echo:   cmd.Echo,
			}
			b, _ := json.Marshal(newCmd)
			return b
		}
		if up.URL != "" {
			cq := fmt.Sprintf("[CQ:file,file=%s,name=%s]", escapeCommaMaybe(up.URL), name)
			newCmd := oneBotCommand{
				Action: "send_group_msg",
				Params: sendGroupMsgParams{GroupID: p.GroupID, Message: cq},
				Echo:   cmd.Echo,
			}
			b, _ := json.Marshal(newCmd)
			return b
		}
		return msg
	default:
		return msg
	}
}

func escapeCommaMaybe(text string) string { return strings.ReplaceAll(text, ",", "%2C") }

type uploadResult struct {
	URL       string
	LocalPath string
}

func uploadViaB(fileField string, name string, cfg *Config) (uploadResult, string) {
	path := fileField
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if name == "" {
			if u, err := url.Parse(path); err == nil {
				base := filepath.Base(u.Path)
				if base != "" && base != "/" {
					name = base
				}
			}
		}
		return uploadResult{URL: path}, name
	}
	// Handle base64:// content
	if strings.HasPrefix(path, "base64://") {
		enc := strings.TrimPrefix(path, "base64://")
		// support optional data URI header like data:...;base64,xxxx
		if idx := strings.IndexByte(enc, ','); idx != -1 {
			enc = enc[idx+1:]
		}
		data, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			loggerA.Error("Base64 解码失败", "err", err)
			return uploadResult{}, ""
		}
		if name == "" {
			name = "file.bin"
		}
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			loggerA.Error("创建表单文件失败", "err", err)
			return uploadResult{}, ""
		}
		if _, e := part.Write(data); e != nil {
			loggerA.Error("写入 Base64 数据失败", "err", e)
			return uploadResult{}, ""
		}
		_ = writer.WriteField("name", name)
		writer.Close()

		req, err := http.NewRequest("POST", cfg.UploadEndpoint, &body)
		if err != nil {
			loggerA.Error("创建 HTTP 请求失败", "err", err)
			return uploadResult{}, ""
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			loggerA.Error("上传 HTTP 请求失败", "err", err)
			return uploadResult{}, ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			loggerA.Error("上传失败", "status", resp.StatusCode, "body", string(b))
			return uploadResult{}, ""
		}
		var ret struct {
			URL       string `json:"url"`
			Name      string `json:"name"`
			LocalPath string `json:"local_path"`
		}
		if err := json.Unmarshal(b, &ret); err != nil {
			loggerA.Error("解析上传响应失败", "err", err)
			return uploadResult{}, ""
		}
		if ret.Name != "" {
			name = ret.Name
		}
		return uploadResult{URL: ret.URL, LocalPath: ret.LocalPath}, name
	}
	if strings.HasPrefix(path, "file://") {
		u, err := url.Parse(path)
		if err == nil {
			path = u.Path
			if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
				if len(path) >= 3 && path[2] == ':' {
					path = path[1:]
				} else {
					path = strings.TrimPrefix(path, "/")
				}
			}
		}
	}
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
	}
	f, err := os.Open(path)
	if err != nil {
		loggerA.Error("打开上传文件失败", "err", err)
		return uploadResult{}, ""
	}
	defer f.Close()
	if name == "" {
		name = filepath.Base(path)
	}

	// 多平台构建
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		loggerA.Error("创建表单文件失败", "err", err)
		return uploadResult{}, ""
	}
	if _, err := io.Copy(part, f); err != nil {
		loggerA.Error("拷贝文件失败", "err", err)
		return uploadResult{}, ""
	}
	_ = writer.WriteField("name", name)
	writer.Close()

	req, err := http.NewRequest("POST", cfg.UploadEndpoint, &body)
	if err != nil {
		loggerA.Error("创建 HTTP 请求失败", "err", err)
		return uploadResult{}, ""
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		loggerA.Error("上传 HTTP 请求失败", "err", err)
		return uploadResult{}, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		loggerA.Error("上传失败", "status", resp.StatusCode, "body", string(b))
		return uploadResult{}, ""
	}
	var ret struct {
		URL       string `json:"url"`
		Name      string `json:"name"`
		LocalPath string `json:"local_path"`
	}
	if err := json.Unmarshal(b, &ret); err != nil {
		loggerA.Error("解析上传响应失败", "err", err)
		return uploadResult{}, ""
	}
	if ret.Name != "" {
		name = ret.Name
	}
	return uploadResult{URL: ret.URL, LocalPath: ret.LocalPath}, name
}
