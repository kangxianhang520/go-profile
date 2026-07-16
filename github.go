package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GitHubUser 对应 GET https://api.github.com/user 返回的字段（只取需要的）
type GitHubUser struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Bio         string `json:"bio"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	HTMLURL     string `json:"html_url"`
}

// FetchGitHubUser 用个人 token 调 GitHub API 获取当前账户信息
func FetchGitHubUser(token string) (*GitHubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api returned %s: %s", resp.Status, body)
	}

	var u GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}
	return &u, nil
}

// GenerateIntro 用 GitHub 用户名等信息生成一段个人介绍
func GenerateIntro(u *GitHubUser) string {
	name := u.Name
	if name == "" {
		name = u.Login
	}
	intro := fmt.Sprintf("大家好，我是 %s（GitHub：@%s）。", name, u.Login)
	if u.Location != "" {
		intro += fmt.Sprintf("我来自 %s。", u.Location)
	}
	if u.Company != "" {
		intro += fmt.Sprintf("目前在 %s。", u.Company)
	}
	if u.Bio != "" {
		intro += fmt.Sprintf("我的签名是：「%s」。", u.Bio)
	}
	intro += fmt.Sprintf("我在 GitHub 上有 %d 个公开仓库、%d 位关注者，欢迎访问 %s 与我交流！",
		u.PublicRepos, u.Followers, u.HTMLURL)
	return intro
}
