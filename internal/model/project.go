package model

// RunContext carries runtime-only state about where wgstack is executing.
// This is NEVER persisted to config — it is determined at invocation time
// via the wizard or CLI flags (--local-entry / --local-exit).
type RunContext struct {
	EntryIsLocal bool
	ExitIsLocal  bool
}

type Project struct {
	Project            string        `json:"project"`
	Cloudflare         Cloudflare    `json:"cloudflare"`
	Domains            Domains       `json:"domains"`
	Nodes              Nodes         `json:"nodes"`
	PanelGuide         PanelGuide    `json:"panel_guide"`
	Checks             HealthCheck   `json:"healthcheck"`
	Notifications      Notifications `json:"notifications"`
	ExitDDNS           ExitDDNS      `json:"exit_ddns,omitempty"`
	EntryAutoReconcile AutoReconcile `json:"entry_autoreconcile,omitempty"`
}

// AutoReconcile configures a timer-based watcher deployed ON the entry node.
// It periodically checks the entry node's public IP; on change it updates
// Cloudflare DNS, restarts the exit node's WireGuard, and sends notifications.
type AutoReconcile struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval_seconds,omitempty"`
}

// ExitDDNS configures a lightweight DDNS updater deployed ON the exit node.
// When the exit node has a dynamic public IP, this updater keeps its SSH
// management domain pointing to the current IP, ensuring the management
// machine can always reach the exit node.
type ExitDDNS struct {
	Enabled  bool   `json:"enabled"`
	Domain   string `json:"domain,omitempty"`
	Interval int    `json:"interval_seconds,omitempty"`
}

type Notifications struct {
	Enabled  bool           `json:"enabled"`
	Telegram TelegramConfig `json:"telegram"`
}

type TelegramConfig struct {
	BotToken    string `json:"bot_token,omitempty"`
	BotTokenEnv string `json:"bot_token_env,omitempty"`
	ChatID      string `json:"chat_id,omitempty"`
}

type Cloudflare struct {
	Zone       string `json:"zone"`
	Token      string `json:"token,omitempty"`
	TokenEnv   string `json:"token_env,omitempty"`
	RecordType string `json:"record_type,omitempty"`
	TTL        int    `json:"ttl,omitempty"`
	Proxied    bool   `json:"proxied,omitempty"`
}

type Domains struct {
	Entry     string `json:"entry"`
	Panel     string `json:"panel"`
	WireGuard string `json:"wireguard"`
}

// Unique returns all configured domain names, deduplicated.
// When WireGuard domain reuses the entry domain, duplicates are removed.
func (d Domains) Unique() []string {
	seen := make(map[string]bool, 3)
	var result []string
	for _, name := range []string{d.Entry, d.Panel, d.WireGuard} {
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

// Nodes holds the two deployment targets.
// US/HK are historical JSON key names retained for config compatibility;
// they represent the entry node and exit node respectively.
type Nodes struct {
	US Node `json:"us"` // entry node
	HK Node `json:"hk"` // exit node
}

type Node struct {
	Role            string     `json:"role"`
	Host            string     `json:"host"`
	SSHHost         string     `json:"ssh_host,omitempty"`
	SSH             SSH        `json:"ssh"`
	WGAddress       string     `json:"wg_address"`
	WGPort          int        `json:"wg_port,omitempty"`
	WGPrivateKey    string     `json:"wg_private_key,omitempty"`
	WGPublicKey     string     `json:"wg_public_key,omitempty"`
	SocksListen     string     `json:"socks_listen,omitempty"`
	Proxy           string     `json:"proxy,omitempty"`
	WGConfigPath    string     `json:"wg_config_path,omitempty"`
	WGService       string     `json:"wg_service,omitempty"`
	ProxyConfigPath string     `json:"proxy_config_path,omitempty"`
	ProxyService    string     `json:"proxy_service,omitempty"`
	Deploy          NodeDeploy `json:"deploy,omitempty"`
}

// SSHAddr returns the address to use for SSH connections.
// If SSHHost is set (e.g. a stable domain), it is used; otherwise falls back to Host.
func (n Node) SSHAddr() string {
	if n.SSHHost != "" {
		return n.SSHHost
	}
	return n.Host
}

type SSH struct {
	User                  string `json:"user"`
	Port                  int    `json:"port,omitempty"`
	AuthMethod            string `json:"auth_method,omitempty"`
	Password              string `json:"password,omitempty"`
	PasswordEnv           string `json:"password_env,omitempty"`
	PrivateKeyPath        string `json:"private_key_path,omitempty"`
	PrivateKeyPassphrase  string `json:"private_key_passphrase,omitempty"`
	PassphraseEnv         string `json:"passphrase_env,omitempty"`
	KnownHostsPath        string `json:"known_hosts_path,omitempty"`
	InsecureIgnoreHostKey bool   `json:"insecure_ignore_host_key,omitempty"`
}

type NodeDeploy struct {
	UploadOnly  bool `json:"upload_only,omitempty"`
	AutoInstall bool `json:"auto_install,omitempty"`
}

type PanelGuide struct {
	OutboundTag string `json:"outbound_tag"`
	RouteUser   string `json:"route_user"`
}

type HealthCheck struct {
	TestURL string `json:"test_url"`
	// ExitCheckURL must point to an endpoint that returns a plain-text public IPv4
	// address (e.g. https://api.ipify.org). It is used to detect the exit node's
	// outbound IP via the SOCKS proxy. It is NOT a generic health-check URL.
	ExitCheckURL     string `json:"exit_check_url,omitempty"`
	PublicIPCheckURL string `json:"public_ip_check_url,omitempty"`
	ExitLocation     string `json:"exit_location,omitempty"`
}
