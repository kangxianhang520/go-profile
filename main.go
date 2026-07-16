package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	dsn := getenv("DATABASE_URL", "postgres://postgres:password@localhost:5432/profile")
	port := getenv("PORT", "8080")

	db, err := OpenDB(dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()
	log.Println("database connected")

	// 启动时如果有 token，自动同步一次 GitHub 信息
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		if p, err := SyncProfile(db, token); err != nil {
			log.Printf("initial sync failed: %v", err)
		} else {
			log.Printf("synced GitHub profile for %s", p.Login)
		}
	} else {
		log.Println("GITHUB_TOKEN not set, skip initial sync")
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// 读取数据库里保存的个人介绍
	mux.HandleFunc("GET /api/profile", func(w http.ResponseWriter, r *http.Request) {
		p, err := LatestProfile(db)
		if err != nil {
			httpError(w, http.StatusNotFound, "no profile yet, call POST /api/sync first: "+err.Error())
			return
		}
		writeJSON(w, p)
	})

	// 用 token 拉取 GitHub 信息 → 生成介绍 → 存库
	mux.HandleFunc("POST /api/sync", func(w http.ResponseWriter, r *http.Request) {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			httpError(w, http.StatusBadRequest, "GITHUB_TOKEN env is not set")
			return
		}
		p, err := SyncProfile(db, token)
		if err != nil {
			httpError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, p)
	})

	log.Printf("listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
