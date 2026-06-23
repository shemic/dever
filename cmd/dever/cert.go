package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

const (
	defaultCertServer    = "letsencrypt"
	defaultCertMode      = "nginx"
	defaultCertReloadCmd = "systemctl reload nginx"
)

type certOptions struct {
	host      string
	domains   []string
	email     string
	mode      string
	webroot   string
	server    string
	certDir   string
	reloadCmd string
	force     bool
	debug     bool
}

type stringListFlag []string

func (flagValue *stringListFlag) String() string {
	return strings.Join(*flagValue, ",")
}

func (flagValue *stringListFlag) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			*flagValue = append(*flagValue, trimmed)
		}
	}
	return nil
}

func runCert(args []string) {
	if len(args) == 0 {
		printCertUsage()
		os.Exit(1)
	}

	var err error
	switch args[0] {
	case "issue":
		err = runCertIssue(args[1:])
	case "info":
		err = runCertInfo(args[1:])
	case "renew":
		err = runCertRenew(args[1:])
	default:
		printCertUsage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "cert 执行失败: %v\n", err)
		os.Exit(1)
	}
}

func printCertUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), `dever cert - 远端 HTTPS 证书管理

Usage:
    dever cert issue user@host --domain=example.com --email=admin@example.com [--mode=nginx|webroot|standalone]
    dever cert info user@host --domain=example.com
    dever cert renew user@host --domain=example.com [--force]
`)
}

func runCertIssue(args []string) error {
	options, err := parseCertIssueOptions(args)
	if err != nil {
		return err
	}
	script, err := buildCertIssueScript(options)
	if err != nil {
		return err
	}
	return runCertRemoteScript(options.host, script)
}

func runCertInfo(args []string) error {
	options, err := parseCertInfoOptions(args)
	if err != nil {
		return err
	}
	return runCertRemoteScript(options.host, buildCertInfoScript(options))
}

func runCertRenew(args []string) error {
	options, err := parseCertRenewOptions(args)
	if err != nil {
		return err
	}
	return runCertRemoteScript(options.host, buildCertRenewScript(options))
}

func parseCertIssueOptions(args []string) (certOptions, error) {
	fs := flag.NewFlagSet("cert issue", flag.ExitOnError)
	domains := stringListFlag{}
	fs.Var(&domains, "domain", "证书域名，可重复或逗号分隔")
	fs.Var(&domains, "d", "证书域名，可重复或逗号分隔")
	email := fs.String("email", "", "acme.sh 注册邮箱")
	mode := fs.String("mode", defaultCertMode, "验证方式：nginx、webroot、standalone")
	webroot := fs.String("webroot", "", "webroot 验证目录，仅 --mode=webroot 时需要")
	server := fs.String("server", defaultCertServer, "ACME CA，例如 letsencrypt")
	certDir := fs.String("cert-dir", "", "证书安装目录，默认 /etc/dever/certs/<主域名>")
	reloadCmd := fs.String("reload", defaultCertReloadCmd, "证书安装和续签后的 reload 命令；传 --reload= 可关闭")
	force := fs.Bool("force", false, "强制重新签发")
	debug := fs.Bool("debug", false, "输出 acme.sh debug 日志")
	if err := fs.Parse(normalizeInterspersedFlagArgs(args, fs)); err != nil {
		return certOptions{}, err
	}
	options := certOptions{
		domains:   domains,
		email:     strings.TrimSpace(*email),
		mode:      strings.TrimSpace(*mode),
		webroot:   strings.TrimSpace(*webroot),
		server:    strings.TrimSpace(*server),
		certDir:   strings.TrimSpace(*certDir),
		reloadCmd: strings.TrimSpace(*reloadCmd),
		force:     *force,
		debug:     *debug,
	}
	if err := fillCertHost(fs, &options); err != nil {
		return certOptions{}, err
	}
	return normalizeCertOptions(options, true)
}

func parseCertInfoOptions(args []string) (certOptions, error) {
	fs := flag.NewFlagSet("cert info", flag.ExitOnError)
	domains := stringListFlag{}
	fs.Var(&domains, "domain", "证书域名")
	fs.Var(&domains, "d", "证书域名")
	if err := fs.Parse(normalizeInterspersedFlagArgs(args, fs)); err != nil {
		return certOptions{}, err
	}
	options := certOptions{domains: domains}
	if err := fillCertHost(fs, &options); err != nil {
		return certOptions{}, err
	}
	return normalizeCertOptions(options, false)
}

func parseCertRenewOptions(args []string) (certOptions, error) {
	fs := flag.NewFlagSet("cert renew", flag.ExitOnError)
	domains := stringListFlag{}
	fs.Var(&domains, "domain", "证书域名")
	fs.Var(&domains, "d", "证书域名")
	force := fs.Bool("force", false, "强制续签")
	debug := fs.Bool("debug", false, "输出 acme.sh debug 日志")
	if err := fs.Parse(normalizeInterspersedFlagArgs(args, fs)); err != nil {
		return certOptions{}, err
	}
	options := certOptions{
		domains: domains,
		force:   *force,
		debug:   *debug,
	}
	if err := fillCertHost(fs, &options); err != nil {
		return certOptions{}, err
	}
	return normalizeCertOptions(options, false)
}

func fillCertHost(fs *flag.FlagSet, options *certOptions) error {
	if fs.NArg() != 1 {
		return fmt.Errorf("需要一个远端主机，例如 root@1.2.3.4")
	}
	host := strings.TrimSpace(fs.Arg(0))
	if host == "" {
		return fmt.Errorf("远端主机不能为空")
	}
	if strings.ContainsAny(host, " \t\r\n") {
		return fmt.Errorf("远端主机暂不支持空白字符: %s", host)
	}
	options.host = host
	return nil
}

func normalizeCertOptions(options certOptions, requireIssueFields bool) (certOptions, error) {
	options.domains = normalizeCertDomains(options.domains)
	if len(options.domains) == 0 {
		return certOptions{}, fmt.Errorf("必须指定 --domain")
	}
	if requireIssueFields && options.email == "" {
		return certOptions{}, fmt.Errorf("签发证书必须指定 --email")
	}
	if options.server == "" {
		options.server = defaultCertServer
	}
	if options.mode == "" {
		options.mode = defaultCertMode
	}
	if options.certDir == "" {
		options.certDir = path.Join("/etc/dever/certs", options.domains[0])
	}
	switch options.mode {
	case "nginx", "standalone":
	case "webroot":
		if options.webroot == "" {
			return certOptions{}, fmt.Errorf("--mode=webroot 必须指定 --webroot")
		}
	default:
		return certOptions{}, fmt.Errorf("未知证书验证方式: %s", options.mode)
	}
	return options, nil
}

func normalizeCertDomains(domains []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		trimmed := strings.ToLower(strings.TrimSpace(domain))
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func buildCertIssueScript(options certOptions) (string, error) {
	var builder strings.Builder
	appendCertPrelude(&builder)
	fmt.Fprintf(&builder, "ensure_acme %s\n", shellQuote(options.email))
	fmt.Fprintf(&builder, "$ACME --set-default-ca --server %s\n", shellQuote(options.server))
	fmt.Fprintf(&builder, "mkdir -p %s\n", shellQuote(options.certDir))
	fmt.Fprint(&builder, "$ACME --issue")
	fmt.Fprintf(&builder, " --server %s", shellQuote(options.server))
	appendCertIssueMode(&builder, options)
	appendCertDomains(&builder, options.domains)
	if options.force {
		fmt.Fprint(&builder, " --force")
	}
	if options.debug {
		fmt.Fprint(&builder, " --debug")
	}
	fmt.Fprint(&builder, "\n")
	appendCertInstallCommand(&builder, options)
	fmt.Fprintf(&builder, "echo %s\n", shellQuote("证书已安装到 "+options.certDir))
	return builder.String(), nil
}

func buildCertInfoScript(options certOptions) string {
	var builder strings.Builder
	appendCertPrelude(&builder)
	fmt.Fprint(&builder, "require_acme\n")
	fmt.Fprintf(&builder, "$ACME --info -d %s\n", shellQuote(options.domains[0]))
	return builder.String()
}

func buildCertRenewScript(options certOptions) string {
	var builder strings.Builder
	appendCertPrelude(&builder)
	fmt.Fprint(&builder, "require_acme\n")
	fmt.Fprintf(&builder, "$ACME --renew -d %s", shellQuote(options.domains[0]))
	if options.force {
		fmt.Fprint(&builder, " --force")
	}
	if options.debug {
		fmt.Fprint(&builder, " --debug")
	}
	fmt.Fprint(&builder, "\n")
	return builder.String()
}

func appendCertPrelude(builder *strings.Builder) {
	fmt.Fprint(builder, `set -e
ACME="$HOME/.acme.sh/acme.sh"

require_acme() {
  if [ ! -x "$ACME" ]; then
    echo "未找到 acme.sh，请先执行 cert issue 安装" >&2
    exit 1
  fi
}

ensure_acme() {
  email="$1"
  if [ -x "$ACME" ]; then
    return 0
  fi
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://get.acme.sh | sh -s email="$email"
  elif command -v wget >/dev/null 2>&1; then
    wget -O - https://get.acme.sh | sh -s email="$email"
  else
    echo "远端缺少 curl 或 wget，无法安装 acme.sh" >&2
    exit 1
  fi
  if [ ! -x "$ACME" ]; then
    echo "acme.sh 安装失败或路径不可用: $ACME" >&2
    exit 1
  fi
}

`)
}

func appendCertIssueMode(builder *strings.Builder, options certOptions) {
	switch options.mode {
	case "nginx":
		fmt.Fprint(builder, " --nginx")
	case "webroot":
		fmt.Fprintf(builder, " --webroot %s", shellQuote(options.webroot))
	case "standalone":
		fmt.Fprint(builder, " --standalone")
	}
}

func appendCertDomains(builder *strings.Builder, domains []string) {
	for _, domain := range domains {
		fmt.Fprintf(builder, " -d %s", shellQuote(domain))
	}
}

func appendCertInstallCommand(builder *strings.Builder, options certOptions) {
	fmt.Fprintf(builder, "$ACME --install-cert -d %s", shellQuote(options.domains[0]))
	fmt.Fprintf(builder, " --key-file %s", shellQuote(path.Join(options.certDir, "privkey.pem")))
	fmt.Fprintf(builder, " --fullchain-file %s", shellQuote(path.Join(options.certDir, "fullchain.pem")))
	if options.reloadCmd != "" {
		fmt.Fprintf(builder, " --reloadcmd %s", shellQuote(options.reloadCmd))
	}
	fmt.Fprint(builder, "\n")
}

func runCertRemoteScript(host string, script string) error {
	session, err := newPublishSSHSession(host)
	if err != nil {
		return err
	}
	defer session.cleanup()

	args := append(session.sshOptions(), session.host, "sh", "-s")
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("远端证书命令执行失败: %w", err)
	}
	return nil
}
