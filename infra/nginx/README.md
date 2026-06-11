# Nginx Configuration

## Purpose

Reverse proxy configurations for production deployments of TPT Online Video.

## Structure

- `tpt.conf` — main site configuration
- `tpt-ssl.conf` — SSL/TLS configuration
- `common/` — shared snippets (security headers, caching, gzip)

## Usage

```bash
# Symlink the config to sites-enabled
sudo ln -s /etc/nginx/sites-available/tpt.conf /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## Notes

- API proxied to `localhost:8080`
- Frontend served as static files from `/var/www/tpt`
- Live HLS served with appropriate caching headers
- WebSocket connections proxied with `proxy_http_version 1.1` and `upgrade` header