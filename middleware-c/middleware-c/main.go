package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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
	// UploadEndpoint 已弃用：改为全部使用 base64:// 内联，不再上传到外部服务
	UploadEndpoint        string `json:"upload_endpoint"`
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

// cqMediaKinds are CQ types we rewrite for cross-machine sending
var cqMediaKinds = map[string]bool{"image": true, "record": true}

// rewriteCQMediaInText scans CQ codes in text and rewrites media file/path/base64 to remote URL
func rewriteCQMediaInText(s string, cfg *Config) string {
	re := regexp.MustCompile(`\[CQ:(image|record)([^\]]*)]`)
	return re.ReplaceAllStringFunc(s, func(seg string) string {
		// parse key=value pairs
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
		// 如果有 url 且是 http(s) 开头，直接返回
		if u, ok := args["url"]; ok {
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				return seg
			}
		}
		file := args["file"]
		if file == "" {
			return seg
		}
		// 如果 file 是 http(s) 开头，直接返回
		if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
			return seg
		}
		// 统一改为 base64:// 内联：从本地/路径读取文件并编码
		b64, name := localFileToBase64URI(file, args["name"])
		if b64 == "" {
			return seg
		}
		args["file"] = escapeCommaMaybe(b64)
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

// rewritePictureTagInText converts custom "[图:<path>]" to CQ:image with remote URL
func rewritePictureTagInText(s string, cfg *Config) string {
	re := regexp.MustCompile(`\[图:([^\]]+)]`)
	return re.ReplaceAllStringFunc(s, func(seg string) string {
		m := re.FindStringSubmatch(seg)
		if len(m) < 2 {
			return seg
		}
		src := strings.TrimSpace(m[1])
		// 将 [图:路径] 直接转成 base64://
		b64, _ := localFileToBase64URI(src, "")
		if b64 == "" {
			return seg
		}
		return "[CQ:image,file=" + escapeCommaMaybe(b64) + "]"
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
	return &cfg, nil
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.json", "config path")
	flag.Parse()

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	http.HandleFunc(cfg.ListenWSPath, func(w http.ResponseWriter, r *http.Request) {
		// 鉴权对接协议端的 access_token
		if cfg.ServerAccessToken != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + cfg.ServerAccessToken
			if auth != expected {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
				return
			}
		}
		// 对接到海豹的 Onebot v11 正向 WS 连接
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
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
			log.Printf("upstream dial error: %v", err)
			clientConn.Close()
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for {
				mt, msg, err := clientConn.ReadMessage()
				if err != nil {
					_ = upstreamConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), timeNowPlus())
					return
				}
				if mt == websocket.TextMessage {
					rewritten := rewriteIfUpload(cmdBytes(msg), cfg)
					msg = rewritten
				}
				if err := upstreamConn.WriteMessage(mt, msg); err != nil {
					return
				}
			}
		}()

		go func() {
			defer wg.Done()
			for {
				mt, msg, err := upstreamConn.ReadMessage()
				if err != nil {
					_ = clientConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), timeNowPlus())
					return
				}
				if err := clientConn.WriteMessage(mt, msg); err != nil {
					return
				}
			}
		}()

		wg.Wait()
		clientConn.Close()
		upstreamConn.Close()
	})

	log.Printf("middleware-a listening on %s%s, proxying to %s", cfg.ListenHTTP, cfg.ListenWSPath, cfg.UpstreamWSURL)
	if err := http.ListenAndServe(cfg.ListenHTTP, nil); err != nil {
		log.Fatal(err)
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
					if t == "image" || t == "record" {
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
						// 转为 base64:// 内联
						b64, _ := localFileToBase64URI(src, "")
						if b64 != "" {
							data["url"] = b64
							data["file"] = b64
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
					if t == "image" || t == "record" {
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
						// 转为 base64:// 内联
						b64, _ := localFileToBase64URI(src, "")
						if b64 != "" {
							data["url"] = b64
							data["file"] = b64
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
					if t == "image" || t == "record" {
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
						// 转为 base64:// 内联
						b64, _ := localFileToBase64URI(src, "")
						if b64 != "" {
							data["url"] = b64
							data["file"] = b64
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
		// upload_private_file 统一改为用 base64:// 文件发送
		b64, name := localFileToBase64URI(p.File, p.Name)
		if b64 == "" {
			return msg
		}
		cq := fmt.Sprintf("[CQ:file,file=%s,name=%s]", escapeCommaMaybe(b64), name)
		newCmd := oneBotCommand{
			Action: "send_private_msg",
			Params: sendPrivateMsgParams{UserID: p.UserID, Message: cq},
			Echo:   cmd.Echo,
		}
		b, _ := json.Marshal(newCmd)
		return b
	case "upload_group_file":
		raw, _ := json.Marshal(cmd.Params)
		var p uploadGroupFileParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return msg
		}
		// upload_group_file 统一改为用 base64:// 文件发送
		b64, name := localFileToBase64URI(p.File, p.Name)
		if b64 == "" {
			return msg
		}
		cq := fmt.Sprintf("[CQ:file,file=%s,name=%s]", escapeCommaMaybe(b64), name)
		newCmd := oneBotCommand{
			Action: "send_group_msg",
			Params: sendGroupMsgParams{GroupID: p.GroupID, Message: cq},
			Echo:   cmd.Echo,
		}
		b, _ := json.Marshal(newCmd)
		return b
	default:
		return msg
	}
}

func escapeCommaMaybe(text string) string { return strings.ReplaceAll(text, ",", "%2C") }

// localFileToBase64URI 将本地路径 / file:// / 相对路径文件读取为 base64://data
// 对已是 http(s)、base64:// 的输入保持原样。
func localFileToBase64URI(path string, name string) (string, string) {
	// 已是 http(s) 的：不动，直接返回原始路径（避免破坏已有可访问 URL）
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if name == "" {
			if u, err := url.Parse(path); err == nil {
				base := filepath.Base(u.Path)
				if base != "" && base != "/" {
					name = base
				}
			}
		}
		return path, name
	}
	// 已经是 base64:// 则直接返回
	if strings.HasPrefix(path, "base64://") {
		if name == "" {
			name = "file.bin"
		}
		return path, name
	}
	// file:// 处理
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
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	f, err := os.Open(path)
	if err != nil {
		log.Printf("open file for base64 encode failed: %v", err)
		return "", ""
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("read file for base64 encode failed: %v", err)
		return "", ""
	}
	if name == "" {
		name = filepath.Base(path)
		if name == "" {
			name = "file.bin"
		}
	}
	enc := base64.StdEncoding.EncodeToString(data)
	// OneBot 常见写法是 base64:// 后直接内容
	return "base64://" + enc, name
}
