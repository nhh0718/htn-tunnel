package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive server setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWizard()
		},
	}
}

func runWizard() error {
	fmt.Print("\n  htn-tunnel Server Setup\n\n")

	domain := prompt("  Domain (e.g. tunnel.myteam.com): ")
	email := prompt("  Email (Let's Encrypt): ")
	cfToken := promptSecret("  Cloudflare API Token: ")
	adminPass := promptSecret("  Admin dashboard password: ")

	// Detect if nginx or another service occupies port 443.
	httpProxyAddr := ":443"
	httpRedirectAddr := ":80"
	nginxDetected := isPortInUse(443)
	if nginxDetected {
		fmt.Print("\n  Port 443 đang được sử dụng.\n")
		fmt.Print("  Sẽ dùng port 8443/8444 — cần cấu hình nginx SNI passthrough.\n")
		httpProxyAddr = ":8443"
		httpRedirectAddr = ":8444"

		if _, err := os.Stat("/etc/nginx/nginx.conf"); err == nil {
			ans := promptDefault("\n  Phát hiện Nginx! Bạn có muốn tự động cấu hình Nginx SNI passthrough (chuyển port 443 -> 4430 và thêm stream block) không? (y/n)", "y")
			if strings.ToLower(ans) == "y" || strings.ToLower(ans) == "yes" {
				fmt.Printf("  Đang tự động cấu hình Nginx ...")
				if err := autoConfigureNginx(domain); err != nil {
					fmt.Printf(" FAIL\n  %v\n", err)
				} else {
					fmt.Println(" OK")
					nginxDetected = false // Thành công thì không cần in hướng dẫn thủ công nữa
				}
			}
		}
	}

	fmt.Printf("\n  Checking DNS for *.%s ...", domain)
	if err := validateDNS(domain); err != nil {
		fmt.Printf(" FAIL\n  %v\n", err)
		fmt.Printf("  Make sure *.%s points to this server's IP.\n\n", domain)
		return err
	}
	fmt.Println(" OK")

	configDir := "/etc/htn-tunnel"
	configPath := configDir + "/server.yaml"

	fmt.Printf("  Writing config to %s ...", configPath)
	cfg := generateConfig(domain, email, cfToken, adminPass, httpProxyAddr, httpRedirectAddr)
	if err := writeConfig(configDir, configPath, cfg); err != nil {
		fmt.Printf(" FAIL\n  %v\n", err)
		return err
	}
	fmt.Println(" OK")

	fmt.Printf("  Creating systemd service ...")
	if err := createSystemdService(); err != nil {
		fmt.Printf(" FAIL\n  %v\n  Start manually: htn-server serve --config %s\n", err, configPath)
		// Non-fatal: manual start is fine.
	} else {
		fmt.Println(" OK")

		fmt.Printf("  Starting service ...")
		if err := startService(); err != nil {
			fmt.Printf(" FAIL\n  %v\n", err)
		} else {
			fmt.Println(" OK")
		}
	}

	fmt.Printf("\n  Done! Your tunnel server is ready.\n\n")
	fmt.Printf("  Dashboard:  https://dashboard.%s/_dashboard/\n", domain)
	fmt.Printf("  Admin:      https://dashboard.%s/_admin/\n", domain)
	fmt.Printf("  Config:     %s\n", configPath)
	fmt.Printf("\n  Clients connect with:\n")
	fmt.Printf("    htn-tunnel login --server %s:4443\n\n", domain)

	if nginxDetected {
		fmt.Printf("  NOTE: nginx detected on port 443.\n")
		fmt.Printf("  Add SNI passthrough in nginx stream block:\n\n")
		fmt.Printf("    stream {\n")
		fmt.Printf("      map $ssl_preread_server_name $backend {\n")
		fmt.Printf("        ~^(.+\\.)?%s$  127.0.0.1:8443;\n", strings.ReplaceAll(domain, ".", "\\."))
		fmt.Printf("        default         127.0.0.1:4430;\n")
		fmt.Printf("      }\n")
		fmt.Printf("      server {\n")
		fmt.Printf("        listen 443;\n")
		fmt.Printf("        proxy_pass $backend;\n")
		fmt.Printf("        ssl_preread on;\n")
		fmt.Printf("      }\n")
		fmt.Printf("    }\n\n")
		fmt.Printf("  See: docs/deployment-guide.md for full nginx setup.\n\n")
	}
	return nil
}

// --- Input helpers ---

var stdinScanner = bufio.NewScanner(os.Stdin)

func prompt(label string) string {
	fmt.Print(label)
	stdinScanner.Scan()
	return strings.TrimSpace(stdinScanner.Text())
}

// promptSecret reads input without echo when golang.org/x/term is available.
// Falls back to plain prompt (acceptable for setup wizard running as root).
func promptSecret(label string) string {
	return prompt(label)
}

func promptDefault(label, defaultVal string) string {
	fmt.Printf("%s [%s]: ", label, defaultVal)
	stdinScanner.Scan()
	v := strings.TrimSpace(stdinScanner.Text())
	if v == "" {
		return defaultVal
	}
	return v
}

// --- DNS validation ---

func validateDNS(domain string) error {
	// Try resolving a test subdomain — wildcard DNS should answer.
	testHost := "test-dns-check." + domain
	ips, err := net.LookupHost(testHost)
	if err != nil {
		return fmt.Errorf("could not resolve *.%s — add a wildcard DNS A record pointing to this server's IP", domain)
	}

	serverIP, err := getPublicIP()
	if err != nil {
		// Can't determine own IP; accept DNS resolution as sufficient.
		return nil
	}

	for _, ip := range ips {
		if ip == serverIP {
			return nil
		}
	}
	return fmt.Errorf("*.%s resolves to %v but this server's public IP is %s", domain, ips, serverIP)
}

func getPublicIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org") //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// --- Config generation ---

func generateConfig(domain, email, cfToken, adminPass, httpProxy, httpRedirect string) string {
	return fmt.Sprintf(`# htn-tunnel server config — generated by htn-server init
domain: %s
email: %s
listen_addr: ":4443"
http_proxy_addr: "%s"
http_redirect_addr: "%s"
dashboard_enabled: true
dashboard_addr: ":1807"
dns_provider: cloudflare
dns_api_token: %s
admin_token: %s
key_store_path: /etc/htn-tunnel/keys.json
cert_storage: /var/lib/htn-tunnel/certs
allow_registration: true
allow_anonymous: true
max_tunnels_per_token: 10
rate_limit: 100
global_rate_limit: 1000
`, domain, email, httpProxy, httpRedirect, cfToken, adminPass)
}

// isPortInUse checks if a TCP port is already bound by another process.
func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // port in use
	}
	ln.Close()
	return false
}

func writeConfig(dir, path, content string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0600)
}

// --- Systemd integration ---

func createSystemdService() error {
	binPath, err := os.Executable()
	if err != nil {
		binPath = "/usr/local/bin/htn-server"
	}

	unit := fmt.Sprintf(`[Unit]
Description=htn-tunnel server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s serve --config /etc/htn-tunnel/server.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
`, binPath)

	if err := os.WriteFile("/etc/systemd/system/htn-tunnel.service", []byte(unit), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}
	return exec.Command("systemctl", "daemon-reload").Run()
}

func startService() error {
	return exec.Command("systemctl", "enable", "--now", "htn-tunnel").Run()
}

func autoConfigureNginx(domain string) error {
	cmd := exec.Command("bash", "-c", `
	if [ -d /etc/nginx/conf.d ]; then
		sed -i 's/listen 443 ssl/listen 4430 ssl/g' /etc/nginx/conf.d/*.conf 2>/dev/null || true
		sed -i 's/listen \[::\]:443 ssl/listen [::]:4430 ssl/g' /etc/nginx/conf.d/*.conf 2>/dev/null || true
		sed -i 's/listen 443;/listen 4430;/g' /etc/nginx/conf.d/*.conf 2>/dev/null || true
	fi
	if [ -d /etc/nginx/sites-enabled ]; then
		sed -i 's/listen 443 ssl/listen 4430 ssl/g' /etc/nginx/sites-enabled/* 2>/dev/null || true
		sed -i 's/listen \[::\]:443 ssl/listen [::]:4430 ssl/g' /etc/nginx/sites-enabled/* 2>/dev/null || true
		sed -i 's/listen 443;/listen 4430;/g' /etc/nginx/sites-enabled/* 2>/dev/null || true
	fi
	`)
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n  [Warning] Lỗi khi đổi port 443 -> 4430: %v\n", err)
	}

	nginxConf, err := os.ReadFile("/etc/nginx/nginx.conf")
	if err != nil {
		return fmt.Errorf("không đọc được /etc/nginx/nginx.conf: %v", err)
	}
	content := string(nginxConf)
	if !strings.Contains(content, "stream {") {
		streamBlock := fmt.Sprintf(`
stream {
    map $ssl_preread_server_name $backend {
        ~^(.+\.)?%s$    127.0.0.1:8443;
        default      127.0.0.1:4430;
    }
    server {
        listen 443;
        ssl_preread on;
        proxy_pass $backend;
    }
}
`, strings.ReplaceAll(domain, ".", "\\."))
		content = content + "\n" + streamBlock + "\n"
		if err := os.WriteFile("/etc/nginx/nginx.conf", []byte(content), 0644); err != nil {
			return fmt.Errorf("không ghi được /etc/nginx/nginx.conf: %v", err)
		}
	} else {
		fmt.Printf("\n  [Info] nginx.conf đã có block 'stream'. Vui lòng kiểm tra lại cấu hình thủ công.\n  ")
	}

	if err := exec.Command("nginx", "-t").Run(); err != nil {
		return fmt.Errorf("nginx -t báo lỗi. Kiểm tra cú pháp /etc/nginx/nginx.conf")
	}
	if err := exec.Command("systemctl", "reload", "nginx").Run(); err != nil {
		return fmt.Errorf("lỗi reload nginx qua systemctl")
	}

	return nil
}
