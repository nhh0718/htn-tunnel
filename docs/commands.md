# htn-tunnel VPS Commands Reference

## Service Management

```bash
# Start / Stop / Restart
systemctl start htn-tunnel
systemctl stop htn-tunnel
systemctl restart htn-tunnel
systemctl status htn-tunnel --no-pager

# Enable on boot
systemctl enable htn-tunnel

# View logs
journalctl -u htn-tunnel -n 50 --no-pager
journalctl -u htn-tunnel -f                    # follow live
journalctl -u htn-tunnel --no-pager | grep -i "error\|registered\|ended"
```

## Config Files

```bash
# Server config
cat /etc/htn-tunnel/server.yaml

# Systemd service
cat /etc/systemd/system/htn-tunnel.service

# Nginx main config (stream block)
cat /etc/nginx/nginx.conf

# Nginx site configs
cat /etc/nginx/conf.d/all_site.conf
ls /etc/nginx/sites-enabled/
```

## Port & Process Check

```bash
# All htn-tunnel ports
ss -tlnp | grep htn

# Check specific port
ss -tlnp | grep :443
ss -tlnp | grep :4443
ss -tlnp | grep :8443
ss -tlnp | grep :1807

# Process info
ps aux | grep htn-server | grep -v grep

# All nginx processes
ps aux | grep nginx | grep -v grep
```

## Nginx

```bash
# Test config
nginx -t

# Reload (no downtime)
nginx -s reload

# Full restart
systemctl restart nginx

# View full config (all includes merged)
nginx -T 2>&1 | head -100

# Check listen ports
nginx -T 2>&1 | grep "listen.*443"

# Error log
tail -20 /var/log/nginx/error.log
```

## TLS Certificates

```bash
# Cert directory
ls /var/lib/htn-tunnel/certs/certificates/acme-v02.api.letsencrypt.org-directory/

# Wildcard cert info
cd /var/lib/htn-tunnel/certs/certificates/acme-v02.api.letsencrypt.org-directory/wildcard_.33.id.vn
openssl x509 -in wildcard_.33.id.vn.crt -noout -subject -dates

# Cert permissions
ls -la /var/lib/htn-tunnel/certs/certificates/acme-v02.api.letsencrypt.org-directory/wildcard_.33.id.vn/
```

## Deploy New Binary

```bash
# From local machine (Windows)
GOOS=linux GOARCH=amd64 go build -o bin/htn-server ./cmd/server
scp bin/htn-server root@202.92.4.122:/usr/local/bin/htn-server

# On VPS after upload
setcap 'cap_net_bind_service=+ep' /usr/local/bin/htn-server
systemctl restart htn-tunnel
```

## Test Tunnel (from VPS)

```bash
# Direct to htn-tunnel HTTPS proxy
curl -vk https://127.0.0.1:8443/ -H "Host: <subdomain>.33.id.vn" 2>&1 | tail -15

# Through nginx stream (port 443)
curl -vk https://127.0.0.1/ -H "Host: <subdomain>.33.id.vn" 2>&1 | tail -15

# Dashboard API
curl -s http://127.0.0.1:1807/_dashboard/api/tunnels
curl -s http://127.0.0.1:1807/_dashboard/api/stats
```

## Client Usage (local machine)

```bash
# Auth (one time)
./bin/htn-tunnel auth <token> --server 33.id.vn:4443

# HTTP tunnel
./bin/htn-tunnel http 3000 --server 33.id.vn:4443 --token Hoang123

# TCP tunnel
./bin/htn-tunnel tcp 5432 --server 33.id.vn:4443 --token Hoang123
```

## Docker (VPS)

```bash
# List containers
docker ps -a --format '{{.Names}} {{.Status}} {{.Ports}}'

# Check container network mode
for id in $(docker ps -q); do docker inspect --format '{{.Name}} Network:{{.HostConfig.NetworkMode}}' $id; done

# Cloud panel agent (disabled)
docker start cloud-panel-agent    # if needed
docker stop cloud-panel-agent
```

## Dashboard (qua domain)

```bash
# User dashboard
https://dashboard.33.id.vn/_dashboard/

# Admin dashboard
https://dashboard.33.id.vn/_admin/

# API test
curl -s https://dashboard.33.id.vn/_dashboard/api/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","subdomain":"test"}'

# Admin: xem keys
curl -s https://dashboard.33.id.vn/_admin/api/keys \
  -H "Authorization: Bearer <admin_token>"
```

## Kiến trúc hiện tại

```
Internet → Port 443 (nginx stream, SNI routing)
├── dashboard.33.id.vn → nginx:4430 → proxy → htn-tunnel:1807
├── *.33.id.vn         → TLS passthrough → htn-tunnel:8443
└── * (default)        → nginx HTTP:4430 (certbot TLS)

htn-tunnel ports:
  :4443  - Control plane (yamux, client connections)
  :8443  - HTTP tunnel proxy (certmagic wildcard TLS)
  :8444  - HTTP redirect
  :1807  - Dashboard (user /_dashboard/ + admin /_admin/)

nginx ports:
  :443   - Stream module (SNI routing, không TLS termination)
  :4430  - HTTPS cho tất cả site khác + dashboard proxy
  :80    - HTTP redirect
```

## Troubleshooting

```bash
# htn-tunnel won't start
journalctl -xeu htn-tunnel -n 20 --no-pager

# nginx won't start (port conflict)
ss -tlnp | grep :443
ps aux | grep nginx | grep -v grep
pkill -9 nginx                     # kill rogue nginx
systemctl start nginx

# Client keeps disconnecting
journalctl -u htn-tunnel -n 30 --no-pager | grep "registered\|ended\|error"

# Cert issues
journalctl -u htn-tunnel --no-pager | grep -i cert

# Permission issues
ls -la /usr/local/bin/htn-server
getcap /usr/local/bin/htn-server   # should show cap_net_bind_service
ls -la /var/lib/htn-tunnel/certs/  # should be owned by nobody:nogroup
```
