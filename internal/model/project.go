package model

type Project struct {
	Project    string      `json:"project"`
	Cloudflare Cloudflare  `json:"cloudflare"`
	Domains    Domains     `json:"domains"`
	Nodes      Nodes       `json:"nodes"`
	PanelGuide PanelGuide  `json:"panel_guide"`
	Checks     HealthCheck `json:"healthcheck"`
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

type Nodes struct {
	US Node `json:"us"`
	HK Node `json:"hk"`
}

type Node struct {
	Role            string     `json:"role"`
	Host            string     `json:"host"`
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
	TestURL          string `json:"test_url"`
	ExitCheckURL     string `json:"exit_check_url,omitempty"`
	PublicIPCheckURL string `json:"public_ip_check_url,omitempty"`
	ExitLocation     string `json:"exit_location"`
}
