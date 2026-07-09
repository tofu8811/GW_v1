# HTTP and HTTPS setup

The gateway keeps Fiber on internal HTTP port `8080`. Caddy is the only public entrypoint and forwards both HTTP and HTTPS traffic to `gateway:8080` inside the Docker network.

## Local

From the project root:

```powershell
docker compose --env-file .\.env -f .\database\docker-compose.yml up -d --build
```

Test both protocols:

```powershell
curl http://localhost/health
curl -k https://localhost/health
```

The gateway container does not publish port `8080` to the host. This should fail from the host when the gateway is only running through Docker Compose:

```powershell
curl http://localhost:8080/health
```

## Production domain

1. Copy `.env.example` to `.env`.
2. Set `CADDY_CONFIG_FILE=../Caddyfile.prod`.
3. Replace `api.yourdomain.com` in `Caddyfile.prod` with the real domain.
4. Point the domain DNS A record to the server.
5. Open public ports `80` and `443`.
6. Keep the `gateway_caddy_data` volume so Caddy can persist certificates.

## Trusted proxies

Set `TRUSTED_PROXIES` only to the internal Docker network CIDR or the Caddy container IP when runtime logic needs the real client IP from `X-Forwarded-For`.

Do not use `0.0.0.0/0`, because external clients could spoof `X-Forwarded-For`.