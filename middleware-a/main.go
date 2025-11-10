package main

import (
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "mime/multipart"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type Config struct {
    ListenHTTP         string `json:"listen_http"`
    ListenWSPath       string `json:"listen_ws_path"`
    UpstreamWSURL      string `json:"upstream_ws_url"`
    UpstreamAccessToken string `json:"upstream_access_token"`
    UploadEndpoint     string `json:"upload_endpoint"`
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

var upgrader = websocket.Upgrader{
    ReadBufferSize:  64 * 1024,
    WriteBufferSize: 64 * 1024,
    CheckOrigin: func(r *http.Request) bool { return true },
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
        // Accept WS from sealdice-core adapter
        clientConn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Printf("upgrade error: %v", err)
            return
        }

        // Connect to upstream go-cqhttp
        header := http.Header{}
        if cfg.UpstreamAccessToken != "" {
            header.Set("Authorization", "Bearer "+cfg.UpstreamAccessToken)
        }
        upstreamConn, _, err := websocket.DefaultDialer.Dial(cfg.UpstreamWSURL, header)
        if err != nil {
            log.Printf("upstream dial error: %v", err)
            clientConn.Close()
            return
        }

        var wg sync.WaitGroup
        wg.Add(2)

        // Client -> Upstream
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

        // Upstream -> Client
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

        // Wait for both loops
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
    // Attempt to parse JSON command
    var cmd oneBotCommand
    if err := json.Unmarshal(msg, &cmd); err != nil {
        return msg
    }
    switch cmd.Action {
    case "upload_private_file":
        // params must be uploadPrivateFileParams
        raw, _ := json.Marshal(cmd.Params)
        var p uploadPrivateFileParams
        if err := json.Unmarshal(raw, &p); err != nil {
            return msg
        }
        urlStr, name := uploadViaB(p.File, p.Name, cfg)
        if urlStr == "" {
            return msg
        }
        // Construct send_private_msg with CQ:file
        cq := fmt.Sprintf("[CQ:file,file=%s,name=%s]", escapeCommaMaybe(urlStr), name)
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
        urlStr, name := uploadViaB(p.File, p.Name, cfg)
        if urlStr == "" {
            return msg
        }
        cq := fmt.Sprintf("[CQ:file,file=%s,name=%s]", escapeCommaMaybe(urlStr), name)
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

func uploadViaB(fileField string, name string, cfg *Config) (string, string) {
    path := fileField
    // Handle file:// URI
    if strings.HasPrefix(path, "file://") {
        u, err := url.Parse(path)
        if err == nil {
            path = u.Path
            if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
                // drop leading slash for windows drive path
                if len(path) >= 3 && path[2] == ':' {
                    path = path[1:]
                } else {
                    path = strings.TrimPrefix(path, "/")
                }
            }
        }
    }
    // If still not absolute, try to make absolute
    if !filepath.IsAbs(path) {
        abs, err := filepath.Abs(path)
        if err == nil {
            path = abs
        }
    }
    f, err := os.Open(path)
    if err != nil {
        log.Printf("open file for upload failed: %v", err)
        return "", ""
    }
    defer f.Close()
    if name == "" {
        name = filepath.Base(path)
    }

    // Build multipart form
    var body bytes.Buffer
    writer := multipart.NewWriter(&body)
    part, err := writer.CreateFormFile("file", name)
    if err != nil {
        log.Printf("create form file failed: %v", err)
        return "", ""
    }
    if _, err := io.Copy(part, f); err != nil {
        log.Printf("copy file failed: %v", err)
        return "", ""
    }
    _ = writer.WriteField("name", name)
    writer.Close()

    req, err := http.NewRequest("POST", cfg.UploadEndpoint, &body)
    if err != nil {
        log.Printf("new request failed: %v", err)
        return "", ""
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("upload HTTP error: %v", err)
        return "", ""
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
    if resp.StatusCode/100 != 2 {
        log.Printf("upload failed status=%d body=%s", resp.StatusCode, string(b))
        return "", ""
    }
    var ret struct {
        URL  string `json:"url"`
        Name string `json:"name"`
    }
    if err := json.Unmarshal(b, &ret); err != nil {
        log.Printf("parse upload response failed: %v", err)
        return "", ""
    }
    if ret.Name != "" {
        name = ret.Name
    }
    return ret.URL, name
}