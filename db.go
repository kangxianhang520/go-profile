package main

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Profile 是存进数据库、也返回给前端的数据结构
type Profile struct {
	Login     string    `json:"login"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	Bio       string    `json:"bio"`
	Intro     string    `json:"intro"`
	UpdatedAt time.Time `json:"updated_at"`
}

const schema = `
CREATE TABLE IF NOT EXISTS profiles (
    login       TEXT PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT NOT NULL DEFAULT '',
    bio         TEXT NOT NULL DEFAULT '',
    intro       TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// OpenDB 连接 PostgreSQL 并确保建表
func OpenDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}

// SyncProfile 拉取 GitHub 信息 → 生成介绍 → 写入数据库（存在则更新）
func SyncProfile(db *sql.DB, token string) (*Profile, error) {
	u, err := FetchGitHubUser(token)
	if err != nil {
		return nil, err
	}
	p := &Profile{
		Login:     u.Login,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
		Bio:       u.Bio,
		Intro:     GenerateIntro(u),
		UpdatedAt: time.Now(),
	}
	_, err = db.Exec(`
        INSERT INTO profiles (login, name, avatar_url, bio, intro, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (login) DO UPDATE SET
            name = EXCLUDED.name,
            avatar_url = EXCLUDED.avatar_url,
            bio = EXCLUDED.bio,
            intro = EXCLUDED.intro,
            updated_at = EXCLUDED.updated_at`,
		p.Login, p.Name, p.AvatarURL, p.Bio, p.Intro, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// LatestProfile 读取最近更新的一条介绍
func LatestProfile(db *sql.DB) (*Profile, error) {
	var p Profile
	err := db.QueryRow(`
        SELECT login, name, avatar_url, bio, intro, updated_at
        FROM profiles ORDER BY updated_at DESC LIMIT 1`).
		Scan(&p.Login, &p.Name, &p.AvatarURL, &p.Bio, &p.Intro, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
