# Artifacts

| Path | Description |
| --- | --- |
| `bin/safe` | Linux amd64 binary produced by `make build` (not committed) |
| `chatadmin/dist/` | Frontend build output from `npm run build` (not committed) |
| `config/app.example.yml` | Sample YAML config |
| `.env.example` | Env placeholders |
| `deploy/systemd/*.service` | systemd units |
| `deploy/nginx-safe.conf.example` | Nginx sample |
| `scripts/install-ubuntu.sh` | Ubuntu install helper |
| `docs/使用教程.md` | Operator tutorial (docs only) |
| `docs/Ubuntu部署.md` | Deploy guide |

Do not ship `node_modules/`, real `.env`, or secret-bearing `config/app.yml`.
