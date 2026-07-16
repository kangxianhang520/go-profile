# go-profile — GitHub 个人介绍生成器（作业第 1 题）

Go 服务：用个人 GitHub Token 获取账户信息 → 生成个人介绍 → 存入 PostgreSQL → 前端页面展示。

## 运行前提

- Go 1.24+（已装在 `~/sdk/go`，PATH 已配置在 `~/.zshrc`）
- PostgreSQL（用的是 `~/pgsql` 里那套免安装版，用户 `postgres`，密码 `password`，库 `profile`）

启动数据库（如果没在跑）：

```bash
~/pgsql/pgsql/bin/pg_ctl -D ~/pgsql/data -l ~/pgsql/log.txt start
```

## 运行

1. 去 GitHub → Settings → Developer settings → Personal access tokens 生成一个 token（勾选 `read:user` 即可）。
2. 启动服务：

```bash
cd ~/go-profile
export GITHUB_TOKEN=ghp_你的token
go run .
```

3. 浏览器打开 http://localhost:8080 即可看到个人介绍卡片。

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `GITHUB_TOKEN` | （无） | GitHub 个人访问令牌，必填 |
| `DATABASE_URL` | `postgres://postgres:password@localhost:5432/profile` | 数据库连接串 |
| `PORT` | `8080` | 监听端口 |

## 接口

- `GET /api/profile` — 返回数据库里保存的个人介绍（JSON）
- `POST /api/sync` — 重新调 GitHub API 并更新数据库
- `GET /` — 前端页面

## 代码结构

- `main.go` — HTTP 服务与路由
- `github.go` — 调 GitHub API + 生成介绍文案
- `db.go` — PostgreSQL 连接、建表、增查
- `static/index.html` — 前端页面
- `Dockerfile` — 容器镜像（第 2 题推 ECR 用）

## 打镜像（第 2 题用）

```bash
docker build -t go-profile .
docker run -p 8080:8080 -e GITHUB_TOKEN=xxx -e DATABASE_URL=postgres://... go-profile
```
test
