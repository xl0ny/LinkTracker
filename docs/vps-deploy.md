# VPS Docker Deploy

This deployment profile is intended for a small VPS with Docker Compose and no Go toolchain.
It runs the application with one Kafka broker and lower JVM heap settings.

Do not run Docker prune or firewall commands on a shared VPN server unless you have checked what else is running there.

## First Run

Create `.env` from `.env.example` and fill real secret values:

```bash
cp .env.example .env
nano .env
```

Start everything:

```bash
make docker-up-vps
```

Check status and logs:

```bash
make compose-vps-ps
make compose-apps-logs
```

## URLs

- Grafana: `http://<server-ip>:3000` (`admin` / `admin`)
- Prometheus: `http://<server-ip>:9090`
- Kafka UI: `http://<server-ip>:8085`
- Scrapper Swagger: `http://<server-ip>:8080/swagger`
- Bot Swagger: `http://<server-ip>:18082/swagger`

## Restart After Reboot

Infrastructure and app containers use `restart: unless-stopped`.
After a server reboot, Docker should bring them back automatically.

If you pulled new code, rebuild only app images:

```bash
git pull
docker compose -f compose.apps.yaml up -d --build
```

## Stop

```bash
make docker-down-vps
```
