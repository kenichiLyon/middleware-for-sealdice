package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
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
    if err := os.MkdirAll(cfg.StorageDir, 0o755); err != nil {
        log.Fatalf("mkdir storage: %v", err)
    }

    http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            w.WriteHeader(http.StatusMethodNotAllowed)
            return
        }
        if err := r.ParseMultipartForm(64 << 20); err != nil {
            http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
            return
        }
        file, hdr, err := r.FormFile("file")
        if err != nil {
            http.Error(w, fmt.Sprintf("form file: %v", err), http.StatusBadRequest)
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
        if err := os.MkdirAll(dir, 0o755); err != nil {
            http.Error(w, fmt.Sprintf("mkdir: %v", err), http.StatusInternalServerError)
            return
        }
        // ensure clean name
        name = filepath.Base(name)
        safeName := strings.ReplaceAll(name, " ", "_")
        outPath := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName))
        out, err := os.Create(outPath)
        if err != nil {
            http.Error(w, fmt.Sprintf("create: %v", err), http.StatusInternalServerError)
            return
        }
        defer out.Close()
        if _, err := io.Copy(out, file); err != nil {
            http.Error(w, fmt.Sprintf("write: %v", err), http.StatusInternalServerError)
            return
        }

        publicURL := fmt.Sprintf("%s/files/%s", strings.TrimRight(cfg.PublicBaseURL, "/"), strings.TrimLeft(strings.ReplaceAll(outPath, cfg.StorageDir+"/", ""), "/"))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "url":  publicURL,
            "name": name,
        })
    })

    // static serving
    fs := http.FileServer(http.Dir(cfg.StorageDir))
    http.Handle("/files/", http.StripPrefix("/files/", fs))

    log.Printf("middleware-b listening on %s, storage=%s", cfg.ListenHTTP, cfg.StorageDir)
    if err := http.ListenAndServe(cfg.ListenHTTP, nil); err != nil {
        log.Fatal(err)
    }
}