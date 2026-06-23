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
	defaultPublishConfigPath  = "config"
	defaultPublishDataPath    = "data"
)

var defaultPublishIncludePaths = []string{defaultPublishBinaryName, defaultPublishConfigPath}

type publishOptions struct {
	projectRoot   string
	remote        publishRemote
	version       string
	skipBuild     bool
	skipFront     bool
	includePaths  []string
	excludePaths  []string
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
	path           string
	fileName       string
	version        string
	binaryPath     string
	preserveConfig bool
	hasData        bool
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
	includeRaw := fs.String("include", "", "发布包白名单，逗号分隔；默认 server,config")
	excludeRaw := fs.String("exclude", "", "从 include 选中的目录中排除路径，逗号分隔")
	binaryPath := fs.String("binary", defaultPublishBinaryName, "跳过构建时使用的二进制路径")
	goos := fs.String("os", defaultBuildOS, "目标操作系统")
	goarch := fs.String("arch", defaultBuildArch, "目标架构")
	cgoEnabled := fs.Bool("cgo", defaultBuildCGO, "是否启用 CGO")
	dataMode := fs.String("data-mode", defaultPublishDataMode, "远端 shared/data 权限，例如 0755、0775、0777")
	serviceName := fs.String("service", "", "systemd 服务名，例如 shemic-admin")
	installSystem := fs.Bool("install-service", false, "创建或覆盖 systemd unit，需要同时指定 --service")
	restartSystem := fs.Bool("restart", false, "发布后重启 systemd 服务，需要同时指定 --service")
	serviceUser := fs.String("user", "", "systemd 服务运行用户；留空则不写 User")
	if err := fs.Parse(normalizeInterspersedFlagArgs(args, fs)); err != nil {
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
	includePaths, err := parsePublishIncludePathList(*includeRaw)
	exitOnPublishError(err)
	excludePaths, err := parsePublishPathList(*excludeRaw, "--exclude")
	exitOnPublishError(err)
	options := publishOptions{
		projectRoot:   resolveProjectRoot(*projectRoot),
		remote:        remote,
		version:       time.Now().Format("20060102150405"),
		skipBuild:     *skipBuild,
		skipFront:     *skipFront,
		includePaths:  includePaths,
		excludePaths:  excludePaths,
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

func normalizeInterspersedFlagArgs(args []string, fs *flag.FlagSet) []string {
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
		name, inlineValue := flagName(arg)
		if inlineValue {
			continue
		}
		currentFlag := fs.Lookup(name)
		if currentFlag != nil && flagNeedsValue(currentFlag) && index+1 < len(args) {
			index++
			flags = append(flags, args[index])
		}
	}
	return append(flags, targets...)
}

func isFlagArg(arg string) bool {
	return strings.HasPrefix(arg, "-") && arg != "-"
}

func flagName(arg string) (string, bool) {
	name := strings.TrimLeft(arg, "-")
	if equalIndex := strings.Index(name, "="); equalIndex >= 0 {
		return name[:equalIndex], true
	}
	return name, false
}

func flagNeedsValue(flagInfo *flag.Flag) bool {
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

func parsePublishPathList(raw string, flagName string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	seen := map[string]struct{}{}
	paths := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		cleaned, err := cleanPublishPath(item, flagName)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		paths = append(paths, cleaned)
	}
	return paths, nil
}

func parsePublishIncludePathList(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return append([]string(nil), defaultPublishIncludePaths...), nil
	}
	return parsePublishPathList(raw, "--include")
}

func cleanPublishPath(raw string, flagName string) (string, error) {
	value := filepath.ToSlash(strings.TrimSpace(raw))
	if value == "" {
		return "", fmt.Errorf("%s 包含空路径", flagName)
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("%s 不支持绝对路径: %s", flagName, raw)
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("%s 包含空路径", flagName)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%s 不支持上级目录: %s", flagName, raw)
	}
	if cleaned == ".git" || strings.HasPrefix(cleaned, ".git/") {
		return "", fmt.Errorf("%s 不支持包含 .git 目录", flagName)
	}
	if cleaned == "tmp" || strings.HasPrefix(cleaned, "tmp/") {
		return "", fmt.Errorf("%s 不支持包含 tmp 目录", flagName)
	}
	return cleaned, nil
}

func preparePublishArchive(options publishOptions) (publishArchive, error) {
	binaryPath, err := resolvePublishBinary(options)
	if err != nil {
		return publishArchive{}, err
	}
	selection, err := resolvePublishArchiveSelection(options)
	if err != nil {
		return publishArchive{}, err
	}

	archiveDir := filepath.Join(options.projectRoot, defaultPublishReleaseRoot)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return publishArchive{}, fmt.Errorf("创建发布目录失败: %w", err)
	}
	fileName := fmt.Sprintf("%s-%s.tar.gz", filepath.Base(options.projectRoot), options.version)
	archivePath := filepath.Join(archiveDir, fileName)
	if err := createPublishArchive(options, selection, binaryPath, archivePath); err != nil {
		return publishArchive{}, err
	}

	fmt.Printf("dever publish: 发布包 %s\n", archivePath)
	return publishArchive{
		path:           archivePath,
		fileName:       fileName,
		version:        options.version,
		binaryPath:     binaryPath,
		preserveConfig: selection.preserveConfig,
		hasData:        selection.hasData,
	}, nil
}

type publishArchiveSelection struct {
	entries        []string
	preserveConfig bool
	hasData        bool
}

func resolvePublishArchiveSelection(options publishOptions) (publishArchiveSelection, error) {
	entries := []string{}
	includeServer := false
	for _, currentPath := range options.includePaths {
		if publishPathExcluded(currentPath, options.excludePaths) {
			continue
		}
		if currentPath == defaultPublishBinaryName {
			includeServer = true
			continue
		}
		entries = append(entries, currentPath)
	}
	if !includeServer {
		return publishArchiveSelection{}, fmt.Errorf("publish 发布包必须包含 server；如只更新二进制请使用 --include=server")
	}
	entries = compactPublishArchiveEntries(entries)

	return publishArchiveSelection{
		entries:        entries,
		preserveConfig: !publishEntriesContainExact(entries, defaultPublishConfigPath),
		hasData:        publishEntriesContainPath(entries, defaultPublishDataPath),
	}, nil
}

func compactPublishArchiveEntries(entries []string) []string {
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if publishEntryCovered(entry, result) {
			continue
		}
		result = removeCoveredPublishEntries(entry, result)
		result = append(result, entry)
	}
	return result
}

func publishEntryCovered(entry string, existingEntries []string) bool {
	for _, existing := range existingEntries {
		if publishPathMatches(entry, existing) {
			return true
		}
	}
	return false
}

func removeCoveredPublishEntries(entry string, existingEntries []string) []string {
	result := existingEntries[:0]
	for _, existing := range existingEntries {
		if publishPathMatches(existing, entry) {
			continue
		}
		result = append(result, existing)
	}
	return result
}

func publishEntriesContainExact(entries []string, target string) bool {
	for _, entry := range entries {
		if entry == target {
			return true
		}
	}
	return false
}

func publishEntriesContainPath(entries []string, target string) bool {
	for _, entry := range entries {
		if publishPathMatches(entry, target) || publishPathMatches(target, entry) {
			return true
		}
	}
	return false
}

func publishPathExcluded(currentPath string, excludes []string) bool {
	for _, excludePath := range excludes {
		if publishPathMatches(currentPath, excludePath) {
			return true
		}
	}
	return false
}

func publishPathMatches(currentPath string, target string) bool {
	return currentPath == target || strings.HasPrefix(currentPath, target+"/")
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

func createPublishArchive(options publishOptions, selection publishArchiveSelection, binaryPath, archivePath string) error {
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

	for _, entry := range selection.entries {
		if err := addPublishPathToArchive(tarWriter, options.projectRoot, entry, options.excludePaths); err != nil {
			return err
		}
	}
	return nil
}

func addPublishPathToArchive(writer *tar.Writer, projectRoot, relativePath string, excludes []string) error {
	if publishPathExcluded(relativePath, excludes) {
		return nil
	}
	sourcePath := filepath.Join(projectRoot, filepath.FromSlash(relativePath))
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("读取待打包路径失败: %s: %w", relativePath, err)
	}
	if info.IsDir() {
		return addDirectoryToArchive(writer, sourcePath, relativePath, excludes)
	}
	return addFileToArchive(writer, sourcePath, relativePath, 0)
}

func addDirectoryToArchive(writer *tar.Writer, sourceDir, archiveRoot string, excludes []string) error {
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
		if publishPathExcluded(archiveName, excludes) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=4",
	}
}

func (session publishSSHSession) cleanup() {
	_ = exec.Command("ssh", append(session.sshOptions(), "-O", "exit", session.host)...).Run()
	_ = os.RemoveAll(session.controlDir)
}

func uploadPublishArchive(session publishSSHSession, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("读取发布包失败: %s: %w", localPath, err)
	}
	if shouldUsePublishRsync(session) {
		if err := uploadPublishArchiveWithRsync(session, localPath, remotePath, info.Size()); err == nil {
			return nil
		} else {
			fmt.Printf("dever publish: rsync 上传失败，回退 SSH 流式上传: %v\n", err)
		}
	} else {
		fmt.Println("dever publish: 未检测到本地或远端 rsync，使用 SSH 流式上传")
	}
	return uploadPublishArchiveWithSSH(session, localPath, remotePath, info.Size())
}

func shouldUsePublishRsync(session publishSSHSession) bool {
	if !localPublishCommandExists("rsync") {
		return false
	}
	return remotePublishCommandExists(session, "rsync")
}

func localPublishCommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func remotePublishCommandExists(session publishSSHSession, name string) bool {
	script := fmt.Sprintf("command -v %s >/dev/null 2>&1", shellQuote(name))
	args := append(session.sshOptions(), session.host, "sh -c "+shellQuote(script))
	cmd := exec.Command("ssh", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func uploadPublishArchiveWithRsync(session publishSSHSession, localPath, remotePath string, size int64) error {
	remoteUploadPath := publishRsyncUploadPath(remotePath)
	fmt.Printf("dever publish: 使用 rsync 上传 %s -> %s:%s (%s)\n", localPath, session.host, remoteUploadPath, formatPublishSize(size))
	args := []string{
		"-a",
		"--partial",
		"--append-verify",
		"--progress",
		"-e", session.rsyncSSHCommand(),
		localPath,
		session.host + ":" + remoteUploadPath,
	}
	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync 执行失败: %w", err)
	}

	script := fmt.Sprintf(`set -e
source=%s
target=%s
tmp="${target}.ready.$$"
cleanup() { rm -f "$tmp"; }
trap cleanup INT TERM HUP EXIT
cp -f "$source" "$tmp"
mv -f "$tmp" "$target"
trap - INT TERM HUP EXIT
`, shellQuote(remoteUploadPath), shellQuote(remotePath))
	return runRemotePublishCommand(session, script)
}

func publishRsyncUploadPath(remotePath string) string {
	return pathpkg.Join(pathpkg.Dir(remotePath), ".dever-upload.tar.gz")
}

func (session publishSSHSession) rsyncSSHCommand() string {
	args := append([]string{"ssh"}, session.sshOptions()...)
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func uploadPublishArchiveWithSSH(session publishSSHSession, localPath, remotePath string, size int64) error {
	input, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开发布包失败: %s: %w", localPath, err)
	}
	defer input.Close()

	fmt.Printf("dever publish: 上传 %s -> %s:%s (%s)\n", localPath, session.host, remotePath, formatPublishSize(size))
	script := fmt.Sprintf(`set -e
target=%s
tmp="${target}.uploading.$$"
cleanup() { rm -f "$tmp"; }
trap cleanup INT TERM HUP EXIT
cat > "$tmp"
mv -f "$tmp" "$target"
trap - INT TERM HUP EXIT
`, shellQuote(remotePath))
	args := append(session.sshOptions(), session.host, "sh -c "+shellQuote(script))
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = newPublishProgressReader(input, size)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println()
		return fmt.Errorf("上传发布包失败: %w", err)
	}
	fmt.Println()
	return nil
}

type publishProgressReader struct {
	reader    io.Reader
	total     int64
	read      int64
	startedAt time.Time
	lastPrint time.Time
}

func newPublishProgressReader(reader io.Reader, total int64) *publishProgressReader {
	now := time.Now()
	return &publishProgressReader{
		reader:    reader,
		total:     total,
		startedAt: now,
		lastPrint: now,
	}
}

func (reader *publishProgressReader) Read(buffer []byte) (int, error) {
	n, err := reader.reader.Read(buffer)
	if n > 0 {
		reader.read += int64(n)
		now := time.Now()
		if now.Sub(reader.lastPrint) >= time.Second || reader.read >= reader.total {
			reader.print(now)
		}
	}
	return n, err
}

func (reader *publishProgressReader) print(now time.Time) {
	elapsed := now.Sub(reader.startedAt).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	speed := int64(float64(reader.read) / elapsed)
	if reader.total > 0 {
		percent := float64(reader.read) * 100 / float64(reader.total)
		fmt.Printf("\rdever publish: 上传进度 %s/%s %.1f%% %s/s", formatPublishSize(reader.read), formatPublishSize(reader.total), percent, formatPublishSize(speed))
	} else {
		fmt.Printf("\rdever publish: 上传进度 %s %s/s", formatPublishSize(reader.read), formatPublishSize(speed))
	}
	reader.lastPrint = now
}

func formatPublishSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, currentUnit := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f%s", value, currentUnit)
		}
	}
	return fmt.Sprintf("%.1fPB", value/unit)
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
`, shellQuote(options.remote.root), shellQuote(releaseDir), shellQuote(remoteArchive))
	if archive.preserveConfig {
		fmt.Fprintf(&builder, `if [ ! -d "$base/current/config" ]; then
  echo "本次发布包未包含完整 config，需要远端已有 current/config，请先执行一次完整 publish" >&2
  exit 1
fi
if [ -e "$release/config" ]; then
  rm -rf "$release/.dever-config-overlay"
  mv "$release/config" "$release/.dever-config-overlay"
fi
cp -a "$base/current/config" "$release/config"
if [ -d "$release/.dever-config-overlay" ]; then
  cp -a "$release/.dever-config-overlay/." "$release/config/"
  rm -rf "$release/.dever-config-overlay"
fi
`)
	}
	if archive.hasData {
		fmt.Fprintf(&builder, `if [ -d "$release/data" ]; then
  cp -a "$release/data/." "$base/shared/data/"
  rm -rf "$release/data"
fi
`)
	}
	fmt.Fprintf(&builder, `ln -sfn "$base/shared/data" "$release/data"
ln -sfn "$release" "$base/current.next"
mv -Tf "$base/current.next" "$base/current"
rm -f "$archive"
`)
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
