package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPublishDataMode    = "0755"
	defaultPublishBinaryName  = "server"
	defaultPublishReleaseRoot = "tmp/dever/release"
)

type publishOptions struct {
	projectRoot   string
	remote        publishRemote
	version       string
	skipBuild     bool
	skipFront     bool
	binaryPath    string
	goos          string
	goarch        string
	cgoEnabled    bool
	dataMode      os.FileMode
	serviceName   string
	installSystem bool
	restartSystem bool
	serviceUser   string
}

type publishRemote struct {
	host string
	root string
}

type publishArchive struct {
	path       string
	fileName   string
	version    string
	binaryPath string
}

type publishSSHSession struct {
	host        string
	controlDir  string
	controlPath string
}

func runPublish(args []string) {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	projectRoot := fs.String("project-root", ".", "项目根目录（默认当前目录）")
	skipBuild := fs.Bool("skip-build", false, "跳过本地构建，直接打包 --binary 指定的二进制")
	skipFront := fs.Bool("skip-front", false, "构建时跳过 module/package 前端插件")
	binaryPath := fs.String("binary", defaultPublishBinaryName, "跳过构建时使用的二进制路径")
	goos := fs.String("os", defaultBuildOS, "目标操作系统")
	goarch := fs.String("arch", defaultBuildArch, "目标架构")
	cgoEnabled := fs.Bool("cgo", defaultBuildCGO, "是否启用 CGO")
	dataMode := fs.String("data-mode", defaultPublishDataMode, "远端 shared/data 权限，例如 0755、0775、0777")
	serviceName := fs.String("service", "", "systemd 服务名，例如 shemic-admin")
	installSystem := fs.Bool("install-service", false, "创建或覆盖 systemd unit，需要同时指定 --service")
	restartSystem := fs.Bool("restart", false, "发布后重启 systemd 服务，需要同时指定 --service")
	serviceUser := fs.String("user", "", "systemd 服务运行用户；留空则不写 User")
	if err := fs.Parse(normalizePublishArgs(args, fs)); err != nil {
		fmt.Fprintf(os.Stderr, "publish 参数解析失败: %v\n", err)
		os.Exit(1)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "publish 需要一个远端目标，例如：dever publish root@1.2.3.4:/opt/shemic")
		os.Exit(1)
	}

	remote, err := parsePublishRemote(fs.Arg(0))
	exitOnPublishError(err)
	mode, err := parsePublishDataMode(*dataMode)
	exitOnPublishError(err)
	options := publishOptions{
		projectRoot:   resolveProjectRoot(*projectRoot),
		remote:        remote,
		version:       time.Now().Format("20060102150405"),
		skipBuild:     *skipBuild,
		skipFront:     *skipFront,
		binaryPath:    strings.TrimSpace(*binaryPath),
		goos:          normalizeBuildValue(*goos, defaultBuildOS),
		goarch:        normalizeBuildValue(*goarch, defaultBuildArch),
		cgoEnabled:    *cgoEnabled,
		dataMode:      mode,
		serviceName:   strings.TrimSpace(*serviceName),
		installSystem: *installSystem,
		restartSystem: *restartSystem,
		serviceUser:   strings.TrimSpace(*serviceUser),
	}
	exitOnPublishError(runPublishRelease(options))
}

func normalizePublishArgs(args []string, fs *flag.FlagSet) []string {
	if len(args) == 0 {
		return args
	}
	flags := make([]string, 0, len(args))
	targets := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			targets = append(targets, args[index+1:]...)
			break
		}
		if !isFlagArg(arg) {
			targets = append(targets, arg)
			continue
		}

		flags = append(flags, arg)
		name, inlineValue := publishFlagName(arg)
		if inlineValue {
			continue
		}
		currentFlag := fs.Lookup(name)
		if currentFlag != nil && publishFlagNeedsValue(currentFlag) && index+1 < len(args) {
			index++
			flags = append(flags, args[index])
		}
	}
	return append(flags, targets...)
}

func isFlagArg(arg string) bool {
	return strings.HasPrefix(arg, "-") && arg != "-"
}

func publishFlagName(arg string) (string, bool) {
	name := strings.TrimLeft(arg, "-")
	if equalIndex := strings.Index(name, "="); equalIndex >= 0 {
		return name[:equalIndex], true
	}
	return name, false
}

func publishFlagNeedsValue(flagInfo *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	if value, ok := flagInfo.Value.(boolFlag); ok && value.IsBoolFlag() {
		return false
	}
	return true
}

func exitOnPublishError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "publish 执行失败: %v\n", err)
	os.Exit(1)
}

func runPublishRelease(options publishOptions) error {
	if err := validatePublishSystemOptions(options); err != nil {
		return err
	}

	archive, err := preparePublishArchive(options)
	if err != nil {
		return err
	}
	if err := deployPublishArchive(options, archive); err != nil {
		return err
	}

	fmt.Printf("dever publish: 发布完成 %s:%s -> %s\n", options.remote.host, options.remote.root, archive.version)
	if options.serviceName == "" {
		fmt.Printf("dever publish: 未配置 systemd，远端可手动运行：cd %s/current && ./server\n", options.remote.root)
	}
	return nil
}

func validatePublishSystemOptions(options publishOptions) error {
	if (options.installSystem || options.restartSystem) && options.serviceName == "" {
		return fmt.Errorf("--install-service 或 --restart 需要同时指定 --service")
	}
	if options.serviceName != "" && !validSystemdServiceName(options.serviceName) {
		return fmt.Errorf("systemd 服务名只能包含字母、数字、点、下划线、中划线和 @: %s", options.serviceName)
	}
	if options.serviceUser != "" && !validSystemdUser(options.serviceUser) {
		return fmt.Errorf("systemd 运行用户只能包含字母、数字、点、下划线和中划线: %s", options.serviceUser)
	}
	return nil
}

func parsePublishRemote(raw string) (publishRemote, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return publishRemote{}, fmt.Errorf("远端目标不能为空")
	}
	index := strings.LastIndex(target, ":")
	if index <= 0 || index == len(target)-1 {
		return publishRemote{}, fmt.Errorf("远端目标格式应为 user@host:/absolute/path")
	}
	host := strings.TrimSpace(target[:index])
	root := strings.TrimSpace(target[index+1:])
	if host == "" || root == "" {
		return publishRemote{}, fmt.Errorf("远端目标格式应为 user@host:/absolute/path")
	}
	if containsWhitespace(host) || containsWhitespace(root) {
		return publishRemote{}, fmt.Errorf("远端目标暂不支持空格: %s", raw)
	}
	if !strings.HasPrefix(root, "/") {
		return publishRemote{}, fmt.Errorf("远端目录必须是绝对路径: %s", root)
	}
	return publishRemote{
		host: host,
		root: pathpkg.Clean(root),
	}, nil
}

func containsWhitespace(value string) bool {
	return strings.ContainsAny(value, " \t\r\n")
}

func parsePublishDataMode(raw string) (os.FileMode, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = defaultPublishDataMode
	}
	for _, char := range value {
		if char < '0' || char > '7' {
			return 0, fmt.Errorf("data-mode 必须是八进制权限: %s", raw)
		}
	}
	parsed, err := strconv.ParseUint(value, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("解析 data-mode 失败: %w", err)
	}
	mode := os.FileMode(parsed)
	if mode == 0 || mode > 0o777 {
		return 0, fmt.Errorf("data-mode 超出允许范围: %s", raw)
	}
	return mode, nil
}

func preparePublishArchive(options publishOptions) (publishArchive, error) {
	binaryPath, err := resolvePublishBinary(options)
	if err != nil {
		return publishArchive{}, err
	}

	archiveDir := filepath.Join(options.projectRoot, defaultPublishReleaseRoot)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return publishArchive{}, fmt.Errorf("创建发布目录失败: %w", err)
	}
	fileName := fmt.Sprintf("%s-%s.tar.gz", filepath.Base(options.projectRoot), options.version)
	archivePath := filepath.Join(archiveDir, fileName)
	if err := createPublishArchive(options.projectRoot, binaryPath, archivePath); err != nil {
		return publishArchive{}, err
	}

	fmt.Printf("dever publish: 发布包 %s\n", archivePath)
	return publishArchive{
		path:       archivePath,
		fileName:   fileName,
		version:    options.version,
		binaryPath: binaryPath,
	}, nil
}

func resolvePublishBinary(options publishOptions) (string, error) {
	if options.skipBuild {
		binaryPath := strings.TrimSpace(options.binaryPath)
		if binaryPath == "" {
			binaryPath = defaultPublishBinaryName
		}
		if !filepath.IsAbs(binaryPath) {
			binaryPath = filepath.Join(options.projectRoot, binaryPath)
		}
		if err := ensurePublishBinary(binaryPath); err != nil {
			return "", err
		}
		return binaryPath, nil
	}

	binaryPath := filepath.Join(options.projectRoot, "tmp", "dever", "publish", defaultPublishBinaryName)
	if err := runReleaseBuild(releaseBuildOptions{
		projectRoot: options.projectRoot,
		output:      binaryPath,
		goos:        options.goos,
		goarch:      options.goarch,
		cgoEnabled:  options.cgoEnabled,
		skipFront:   options.skipFront,
	}); err != nil {
		return "", err
	}
	if err := ensurePublishBinary(binaryPath); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func ensurePublishBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("读取二进制失败: %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("二进制路径是目录: %s", path)
	}
	return nil
}

func createPublishArchive(projectRoot, binaryPath, archivePath string) error {
	configDir := filepath.Join(projectRoot, "config")
	if info, err := os.Stat(configDir); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("不是目录")
		}
		return fmt.Errorf("读取 config 目录失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("创建发布包目录失败: %w", err)
	}
	output, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("创建发布包失败: %w", err)
	}
	defer output.Close()

	gzipWriter := gzip.NewWriter(output)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	if err := addFileToArchive(tarWriter, binaryPath, defaultPublishBinaryName, 0o755); err != nil {
		return err
	}
	return addDirectoryToArchive(tarWriter, configDir, "config")
}

func addDirectoryToArchive(writer *tar.Writer, sourceDir, archiveRoot string) error {
	return filepath.WalkDir(sourceDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return err
		}
		archiveName := archiveRoot
		if relative != "." {
			archiveName = filepath.ToSlash(filepath.Join(archiveRoot, relative))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return addArchiveEntry(writer, currentPath, archiveName, info, 0)
	})
}

func addFileToArchive(writer *tar.Writer, sourcePath, archiveName string, mode os.FileMode) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("读取待打包文件失败: %s: %w", sourcePath, err)
	}
	return addArchiveEntry(writer, sourcePath, archiveName, info, mode)
}

func addArchiveEntry(writer *tar.Writer, sourcePath, archiveName string, info os.FileInfo, forcedMode os.FileMode) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("创建 tar 头失败: %s: %w", sourcePath, err)
	}
	header.Name = archiveName
	if forcedMode != 0 {
		header.Mode = int64(forcedMode)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(sourcePath)
		if err != nil {
			return fmt.Errorf("读取软链失败: %s: %w", sourcePath, err)
		}
		header.Linkname = target
	}
	if err := writer.WriteHeader(header); err != nil {
		return fmt.Errorf("写入 tar 头失败: %s: %w", sourcePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开待打包文件失败: %s: %w", sourcePath, err)
	}
	defer input.Close()
	if _, err := io.Copy(writer, input); err != nil {
		return fmt.Errorf("写入发布包失败: %s: %w", sourcePath, err)
	}
	return nil
}

func deployPublishArchive(options publishOptions, archive publishArchive) error {
	session, err := newPublishSSHSession(options.remote.host)
	if err != nil {
		return err
	}
	defer session.cleanup()

	remoteArchive := pathpkg.Join(options.remote.root, "releases", archive.fileName)
	if err := runRemotePublishCommand(session, buildRemotePrepareScript(options, remoteArchive)); err != nil {
		return err
	}
	if err := uploadPublishArchive(session, archive.path, remoteArchive); err != nil {
		return err
	}
	return runRemotePublishCommand(session, buildRemoteActivateScript(options, archive, remoteArchive))
}

func newPublishSSHSession(host string) (publishSSHSession, error) {
	controlDir, err := os.MkdirTemp("", "dever-publish-ssh-*")
	if err != nil {
		return publishSSHSession{}, fmt.Errorf("创建 SSH 复用目录失败: %w", err)
	}
	return publishSSHSession{
		host:        host,
		controlDir:  controlDir,
		controlPath: filepath.Join(controlDir, "control-%C"),
	}, nil
}

func (session publishSSHSession) sshOptions() []string {
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=60s",
		"-o", "ControlPath=" + session.controlPath,
	}
}

func (session publishSSHSession) cleanup() {
	_ = exec.Command("ssh", append(session.sshOptions(), "-O", "exit", session.host)...).Run()
	_ = os.RemoveAll(session.controlDir)
}

func uploadPublishArchive(session publishSSHSession, localPath, remotePath string) error {
	fmt.Printf("dever publish: 上传 %s -> %s:%s\n", localPath, session.host, remotePath)
	args := append(session.sshOptions(), localPath, session.host+":"+remotePath)
	cmd := exec.Command("scp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("上传发布包失败: %w", err)
	}
	return nil
}

func runRemotePublishCommand(session publishSSHSession, script string) error {
	args := append(session.sshOptions(), session.host, "sh", "-s")
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("远端命令执行失败: %w", err)
	}
	return nil
}

func buildRemotePrepareScript(options publishOptions, remoteArchive string) string {
	mode := fmt.Sprintf("%04o", options.dataMode.Perm())
	var builder strings.Builder
	fmt.Fprintf(&builder, `set -e
base=%s
mkdir -p "$base/releases" "$base/shared/data"
chmod %s "$base/shared/data"
rm -f %s
`, shellQuote(options.remote.root), mode, shellQuote(remoteArchive))
	appendRemoteDataOwner(&builder, options)
	return builder.String()
}

func buildRemoteActivateScript(options publishOptions, archive publishArchive, remoteArchive string) string {
	releaseDir := pathpkg.Join(options.remote.root, "releases", archive.version)
	var builder strings.Builder
	fmt.Fprintf(&builder, `set -e
base=%s
release=%s
archive=%s
mkdir -p "$release" "$base/shared/data"
tar -xzf "$archive" -C "$release"
chmod +x "$release/server"
ln -sfn "$base/shared/data" "$release/data"
ln -sfn "$release" "$base/current.next"
mv -Tf "$base/current.next" "$base/current"
rm -f "$archive"
`, shellQuote(options.remote.root), shellQuote(releaseDir), shellQuote(remoteArchive))
	appendRemoteDataOwner(&builder, options)

	if options.installSystem {
		unitName := systemdUnitName(options.serviceName)
		unitPath := "/etc/systemd/system/" + unitName
		fmt.Fprintf(&builder, "cat > %s <<'DEVER_SYSTEMD_UNIT'\n%s\nDEVER_SYSTEMD_UNIT\n", shellQuote(unitPath), buildSystemdUnit(options))
		fmt.Fprintf(&builder, "systemctl daemon-reload\n")
		fmt.Fprintf(&builder, "systemctl enable %s\n", shellQuote(unitName))
	}
	if options.restartSystem {
		fmt.Fprintf(&builder, "systemctl restart %s\n", shellQuote(systemdUnitName(options.serviceName)))
	}
	return builder.String()
}

func appendRemoteDataOwner(builder *strings.Builder, options publishOptions) {
	if options.serviceUser == "" {
		return
	}
	fmt.Fprintf(builder, "chown -R %s \"$base/shared/data\"\n", shellQuote(options.serviceUser))
}

func buildSystemdUnit(options publishOptions) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "[Unit]\n")
	fmt.Fprintf(&builder, "Description=Dever App %s\n", options.serviceName)
	fmt.Fprintf(&builder, "After=network.target\n\n")
	fmt.Fprintf(&builder, "[Service]\n")
	fmt.Fprintf(&builder, "Type=simple\n")
	if options.serviceUser != "" {
		fmt.Fprintf(&builder, "User=%s\n", options.serviceUser)
	}
	fmt.Fprintf(&builder, "WorkingDirectory=%s/current\n", options.remote.root)
	fmt.Fprintf(&builder, "ExecStart=%s/current/server\n", options.remote.root)
	fmt.Fprintf(&builder, "Restart=always\n")
	fmt.Fprintf(&builder, "RestartSec=3\n\n")
	fmt.Fprintf(&builder, "[Install]\n")
	fmt.Fprintf(&builder, "WantedBy=multi-user.target\n")
	return builder.String()
}

func systemdUnitName(name string) string {
	if strings.HasSuffix(name, ".service") {
		return name
	}
	return name + ".service"
}

func validSystemdServiceName(name string) bool {
	if name == "" || strings.Contains(name, "/") {
		return false
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '.', char == '_', char == '-', char == '@':
		default:
			return false
		}
	}
	return true
}

func validSystemdUser(name string) bool {
	if name == "" || strings.Contains(name, "/") {
		return false
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '.', char == '_', char == '-':
		default:
			return false
		}
	}
	return true
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
