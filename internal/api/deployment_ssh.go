package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"golang.org/x/crypto/ssh"
)

type realDeploymentSSHTester struct{}
type deploymentDialContext func(ctx context.Context, network, address string) (net.Conn, error)

func (realDeploymentSSHTester) Test(ctx context.Context, req deploymentConnectionTestRequest) (result deploymentConnectionTestResult, err error) {
	started := time.Now()
	result = deploymentConnectionTestResult{
		Host:       req.SSH.Host,
		Port:       req.SSH.Port,
		User:       req.SSH.User,
		AuthMethod: req.SSH.AuthMethod,
		Remote:     map[string]string{},
	}
	defer func() {
		result.DurationMS = time.Since(started).Milliseconds()
	}()

	client, err := newDeploymentSSHClient(ctx, req)
	if err != nil {
		var validationErr deploymentValidationError
		if errors.As(err, &validationErr) {
			return result, validationErr.Unwrap()
		}
		result.Message = sanitizeDeploymentSSHError(err)
		return result, nil
	}
	defer client.Close()

	result.OK = true
	result.Message = "SSH 连接成功"
	output, err := runDeploymentSSHProbe(ctx, client)
	if err != nil {
		result.Remote["probe_error"] = sanitizeDeploymentSSHError(err)
		return result, nil
	}
	for key, value := range parseDeploymentSSHProbe(output) {
		result.Remote[key] = value
	}
	return result, nil
}

func newDeploymentSSHClient(ctx context.Context, req deploymentConnectionTestRequest) (*ssh.Client, error) {
	auth, err := deploymentSSHAuthMethod(req.SSH)
	if err != nil {
		return nil, deploymentValidationError{err: err}
	}
	config := &ssh.ClientConfig{
		User:            req.SSH.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	address := net.JoinHostPort(req.SSH.Host, strconv.Itoa(req.SSH.Port))
	dialContext, err := deploymentDialerForConnection(req.Connection)
	if err != nil {
		return nil, deploymentValidationError{err: err}
	}
	conn, err := dialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(clientConn, chans, reqs), nil
}

type deploymentValidationError struct {
	err error
}

func (e deploymentValidationError) Error() string {
	return e.err.Error()
}

func (e deploymentValidationError) Unwrap() error {
	return e.err
}

func deploymentDialerForConnection(req deploymentConnectionReq) (deploymentDialContext, error) {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" || mode == models.DeploymentConnectionDirect {
		return (&net.Dialer{}).DialContext, nil
	}
	if mode != models.DeploymentConnectionCustomProxy && mode != models.DeploymentConnectionManagedProxy {
		return nil, fmt.Errorf("不支持的连接模式")
	}
	proxyAddress := net.JoinHostPort(strings.TrimSpace(req.ProxyHost), strconv.Itoa(req.ProxyPort))
	switch strings.ToLower(strings.TrimSpace(req.ProxyType)) {
	case "socks5":
		return func(ctx context.Context, network, address string) (net.Conn, error) {
			return socks5Connect(ctx, network, proxyAddress, address, req.ProxyUsername, req.ProxyPassword)
		}, nil
	case "http_connect":
		return func(ctx context.Context, network, address string) (net.Conn, error) {
			return httpConnect(ctx, network, proxyAddress, address, req.ProxyUsername, req.ProxyPassword)
		}, nil
	default:
		return nil, fmt.Errorf("proxy_type 仅支持 socks5 或 http_connect")
	}
}

func socks5Connect(ctx context.Context, network, proxyAddress, targetAddress, username, password string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("SOCKS5 仅支持 tcp")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddress)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := socks5Handshake(conn, targetAddress, username, password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func socks5Handshake(conn net.Conn, targetAddress, username, password string) error {
	methods := []byte{0x00}
	if username != "" || password != "" {
		methods = append(methods, 0x02)
	}
	if _, err := conn.Write(append([]byte{0x05, byte(len(methods))}, methods...)); err != nil {
		return err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}
	if reply[0] != 0x05 {
		return fmt.Errorf("SOCKS5 代理响应无效")
	}
	switch reply[1] {
	case 0x00:
	case 0x02:
		if err := socks5UsernamePasswordAuth(conn, username, password); err != nil {
			return err
		}
	default:
		return fmt.Errorf("SOCKS5 代理不接受当前认证方式")
	}
	host, portText, err := net.SplitHostPort(targetAddress)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("SOCKS5 目标端口无效")
	}
	request, err := socks5ConnectRequest(host, port)
	if err != nil {
		return err
	}
	if _, err := conn.Write(request); err != nil {
		return err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != 0x05 {
		return fmt.Errorf("SOCKS5 代理响应无效")
	}
	if header[1] != 0x00 {
		return fmt.Errorf("SOCKS5 连接失败: %s", socks5ReplyMessage(header[1]))
	}
	var discard int
	switch header[3] {
	case 0x01:
		discard = net.IPv4len + 2
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(conn, length); err != nil {
			return err
		}
		discard = int(length[0]) + 2
	case 0x04:
		discard = net.IPv6len + 2
	default:
		return fmt.Errorf("SOCKS5 代理响应地址类型无效")
	}
	if discard > 0 {
		if _, err := io.CopyN(io.Discard, conn, int64(discard)); err != nil {
			return err
		}
	}
	return nil
}

func socks5UsernamePasswordAuth(conn net.Conn, username, password string) error {
	if len(username) > 255 || len(password) > 255 {
		return fmt.Errorf("SOCKS5 用户名或密码过长")
	}
	request := []byte{0x01, byte(len(username))}
	request = append(request, []byte(username)...)
	request = append(request, byte(len(password)))
	request = append(request, []byte(password)...)
	if _, err := conn.Write(request); err != nil {
		return err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}
	if reply[0] != 0x01 || reply[1] != 0x00 {
		return fmt.Errorf("SOCKS5 认证失败")
	}
	return nil
}

func socks5ConnectRequest(host string, port int) ([]byte, error) {
	request := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			request = append(request, 0x01)
			request = append(request, ipv4...)
		} else {
			request = append(request, 0x04)
			request = append(request, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, fmt.Errorf("SOCKS5 目标主机名过长")
		}
		request = append(request, 0x03, byte(len(host)))
		request = append(request, []byte(host)...)
	}
	request = append(request, byte(port>>8), byte(port))
	return request, nil
}

func socks5ReplyMessage(code byte) string {
	switch code {
	case 0x01:
		return "一般性失败"
	case 0x02:
		return "规则不允许连接"
	case 0x03:
		return "网络不可达"
	case 0x04:
		return "主机不可达"
	case 0x05:
		return "连接被拒绝"
	case 0x06:
		return "TTL 过期"
	case 0x07:
		return "命令不支持"
	case 0x08:
		return "地址类型不支持"
	default:
		return fmt.Sprintf("错误码 %d", code)
	}
}

func httpConnect(ctx context.Context, network, proxyAddress, targetAddress, username, password string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("HTTP CONNECT 仅支持 tcp")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddress)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	var request bytes.Buffer
	fmt.Fprintf(&request, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddress, targetAddress)
	if username != "" || password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		fmt.Fprintf(&request, "Proxy-Authorization: Basic %s\r\n", token)
	}
	request.WriteString("\r\n")
	if _, err := conn.Write(request.Bytes()); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_ = conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT 失败: %s", response.Status)
	}
	_ = conn.SetDeadline(time.Time{})
	return &bufferedDeploymentConn{Conn: conn, reader: reader}, nil
}

type bufferedDeploymentConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedDeploymentConn) Read(p []byte) (int, error) {
	if c.reader != nil && c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

func deploymentSSHAuthMethod(req deploymentSSHRequest) (ssh.AuthMethod, error) {
	switch req.AuthMethod {
	case "password":
		return ssh.Password(req.Password), nil
	case "private_key":
		var (
			signer ssh.Signer
			err    error
		)
		if req.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(req.PrivateKey), []byte(req.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(req.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("SSH 私钥无法解析")
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("不支持的 SSH 认证方式")
	}
}

func runDeploymentSSHProbe(ctx context.Context, client *ssh.Client) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	command := strings.Join([]string{
		"printf 'uname_s='; uname -s 2>/dev/null || true",
		"printf 'uname_m='; uname -m 2>/dev/null || true",
		"printf 'id_u='; id -u 2>/dev/null || true",
		"if command -v systemctl >/dev/null 2>&1; then echo systemd=present; else echo systemd=missing; fi",
	}, "; ")

	done := make(chan struct {
		output []byte
		err    error
	}, 1)
	go func() {
		output, err := session.CombinedOutput(command)
		done <- struct {
			output []byte
			err    error
		}{output: output, err: err}
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	case got := <-done:
		return string(got.output), got.err
	}
}

func parseDeploymentSSHProbe(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			values[key] = value
		}
	}
	return values
}

func sanitizeDeploymentSSHError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "SSH 连接超时"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "SSH 连接超时"
	}
	text := err.Error()
	replacements := []string{
		"ssh: handshake failed: ",
		"ssh: unable to authenticate, attempted methods [none password], no supported methods remain",
		"ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain",
	}
	for _, prefix := range replacements {
		text = strings.ReplaceAll(text, prefix, "")
	}
	if strings.Contains(text, "unable to authenticate") {
		return "SSH 认证失败"
	}
	if strings.Contains(text, "connection refused") {
		return "SSH 端口拒绝连接"
	}
	if strings.Contains(text, "no route to host") {
		return "无法到达 SSH 主机"
	}
	if strings.Contains(text, "i/o timeout") {
		return "SSH 连接超时"
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "SSH 连接失败"
	}
	return stripSensitiveErrorText(text)
}

func stripSensitiveErrorText(text string) string {
	var cleaned bytes.Buffer
	fields := strings.Fields(text)
	for i, field := range fields {
		if i > 0 {
			cleaned.WriteByte(' ')
		}
		if len(field) > 96 {
			cleaned.WriteString(field[:96])
			cleaned.WriteString("...")
			continue
		}
		cleaned.WriteString(field)
	}
	return cleaned.String()
}
