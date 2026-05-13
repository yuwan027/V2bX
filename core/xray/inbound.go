package xray

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"encoding/json"

	"github.com/InazumaV/V2bX/api/panel"
	"github.com/InazumaV/V2bX/conf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	coreConf "github.com/xtls/xray-core/infra/conf"
)

// BuildInbound build Inbound config for different protocol
func buildInbound(option *conf.Options, nodeInfo *panel.NodeInfo, tag string) (*core.InboundHandlerConfig, error) {
	in := &coreConf.InboundDetourConfig{}
	var err error
	var network string
	switch nodeInfo.Type {
	case "vmess", "vless":
		err = buildV2ray(option, nodeInfo, in)
		network = nodeInfo.VAllss.Network
	case "trojan":
		err = buildTrojan(option, nodeInfo, in)
		if nodeInfo.Trojan.Network != "" {
			network = nodeInfo.Trojan.Network
		} else {
			network = "tcp"
		}
	case "shadowsocks":
		err = buildShadowsocks(option, nodeInfo, in)
		network = "tcp"
	default:
		return nil, fmt.Errorf("unsupported node type: %s, Only support: V2ray, Trojan, Shadowsocks", nodeInfo.Type)
	}
	if err != nil {
		return nil, err
	}
	// Set network protocol
	// Set server port
	in.PortList = &coreConf.PortList{
		Range: []coreConf.PortRange{
			{
				From: uint32(nodeInfo.Common.ServerPort),
				To:   uint32(nodeInfo.Common.ServerPort),
			}},
	}
	// Set Listen IP address
	ipAddress := net.ParseAddress(option.ListenIP)
	in.ListenOn = &coreConf.Address{Address: ipAddress}
	// Set SniffingConfig
	sniffingConfig := &coreConf.SniffingConfig{
		Enabled:      true,
		DestOverride: &coreConf.StringList{"http", "tls"},
	}
	if option.XrayOptions.DisableSniffing {
		sniffingConfig.Enabled = false
	}
	in.SniffingConfig = sniffingConfig
	switch network {
	case "tcp":
		if in.StreamSetting.TCPSettings != nil {
			in.StreamSetting.TCPSettings.AcceptProxyProtocol = option.XrayOptions.EnableProxyProtocol
		} else {
			tcpSetting := &coreConf.TCPConfig{
				AcceptProxyProtocol: option.XrayOptions.EnableProxyProtocol,
			} //Enable proxy protocol
			in.StreamSetting.TCPSettings = tcpSetting
		}
	case "ws":
		if in.StreamSetting.WSSettings != nil {
			in.StreamSetting.WSSettings.AcceptProxyProtocol = option.XrayOptions.EnableProxyProtocol
		} else {
			in.StreamSetting.WSSettings = &coreConf.WebSocketConfig{
				AcceptProxyProtocol: option.XrayOptions.EnableProxyProtocol,
			} //Enable proxy protocol
		}
	default:
		socketConfig := &coreConf.SocketConfig{
			AcceptProxyProtocol: option.XrayOptions.EnableProxyProtocol,
			TFO:                 option.XrayOptions.EnableTFO,
		} //Enable proxy protocol
		in.StreamSetting.SocketSettings = socketConfig
	}
	// Set TLS or Reality settings
	switch nodeInfo.Security {
	case panel.Tls:
		// Normal tls
		if option.CertConfig == nil {
			return nil, errors.New("the CertConfig is not vail")
		}
		switch option.CertConfig.CertMode {
		case "none", "":
			// CertMode none: skip server TLS entirely. Panel-delivered certs
			// are ignored too — operators wanting TLS must set a CertMode.
			break
		default:
			certs, err := collectTLSCerts(option.CertConfig, nodeInfo)
			if err != nil {
				return nil, err
			}
			in.StreamSetting.Security = "tls"
			in.StreamSetting.TLSSettings = &coreConf.TLSConfig{
				Certs:            certs,
				RejectUnknownSNI: option.CertConfig.RejectUnknownSni,
				ALPN:             resolveALPN(option, nodeInfo),
			}
		}
	case panel.Reality:
		// Reality
		in.StreamSetting.Security = "reality"
		v := nodeInfo.VAllss
		dest := v.TlsSettings.Dest
		if dest == "" {
			dest = v.TlsSettings.ServerName
		}
		xver := v.TlsSettings.Xver
		if xver == 0 {
			xver = v.RealityConfig.Xver
		}
		d, err := json.Marshal(fmt.Sprintf(
			"%s:%s",
			dest,
			v.TlsSettings.ServerPort))
		if err != nil {
			return nil, fmt.Errorf("marshal reality dest error: %s", err)
		}
		mtd, _ := time.ParseDuration(v.RealityConfig.MaxTimeDiff)
		in.StreamSetting.REALITYSettings = &coreConf.REALITYConfig{
			Dest:         d,
			Xver:         xver,
			Show:         false,
			ServerNames:  []string{v.TlsSettings.ServerName},
			PrivateKey:   v.TlsSettings.PrivateKey,
			MinClientVer: v.RealityConfig.MinClientVer,
			MaxClientVer: v.RealityConfig.MaxClientVer,
			MaxTimeDiff:  uint64(mtd.Microseconds()),
			ShortIds:     []string{v.TlsSettings.ShortId},
			Mldsa65Seed:  v.TlsSettings.Mldsa65Seed,
		}
	default:
		break
	}
	in.Tag = tag
	return in.Build()
}

func buildV2ray(config *conf.Options, nodeInfo *panel.NodeInfo, inbound *coreConf.InboundDetourConfig) error {
	v := nodeInfo.VAllss
	if nodeInfo.Type == "vless" {
		//Set vless
		inbound.Protocol = "vless"
		decryption, err := vlessDecryption(nodeInfo)
		if err != nil {
			return err
		}
		vcfg := &coreConf.VLessInboundConfig{Decryption: decryption}
		if config.XrayOptions.EnableFallback {
			if len(config.XrayOptions.FallBackConfigs) > 0 {
				fallbackConfigs, err := buildVlessFallbacks(config.XrayOptions.FallBackConfigs)
				if err != nil {
					return err
				}
				vcfg.Fallbacks = fallbackConfigs
			}
			// Append builtin fallback as last-resort default ("" name/alpn/path).
			if sock := config.BuiltinFallbackSocket; sock != "" {
				destRaw, _ := json.Marshal("unix:" + sock)
				vcfg.Fallbacks = append(vcfg.Fallbacks, &coreConf.VLessInboundFallback{
					Dest: destRaw,
				})
			}
			if len(vcfg.Fallbacks) == 0 {
				return fmt.Errorf("EnableFallback is true but neither FallBackConfigs nor BuiltinFallback is configured")
			}
		}
		s, err := json.Marshal(vcfg)
		if err != nil {
			return fmt.Errorf("marshal vless config error: %s", err)
		}
		inbound.Settings = (*json.RawMessage)(&s)
	} else {
		// Set vmess
		inbound.Protocol = "vmess"
		var err error
		s, err := json.Marshal(&coreConf.VMessInboundConfig{})
		if err != nil {
			return fmt.Errorf("marshal vmess settings error: %s", err)
		}
		inbound.Settings = (*json.RawMessage)(&s)
	}
	if len(v.NetworkSettings) == 0 {
		return nil
	}

	t := coreConf.TransportProtocol(v.Network)
	inbound.StreamSetting = &coreConf.StreamConfig{Network: &t}
	switch v.Network {
	case "tcp":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.TCPSettings)
		if err != nil {
			return fmt.Errorf("unmarshal tcp settings error: %s", err)
		}
	case "ws":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.WSSettings)
		if err != nil {
			return fmt.Errorf("unmarshal ws settings error: %s", err)
		}
	case "grpc":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.GRPCSettings)
		if err != nil {
			return fmt.Errorf("unmarshal grpc settings error: %s", err)
		}
	case "httpupgrade":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.HTTPUPGRADESettings)
		if err != nil {
			return fmt.Errorf("unmarshal httpupgrade settings error: %s", err)
		}
	case "splithttp", "xhttp":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.SplitHTTPSettings)
		if err != nil {
			return fmt.Errorf("unmarshal xhttp settings error: %s", err)
		}
		// 合并本地 XHTTP 覆盖（来自 config.json 的 XHTTPSettings 字段）
		if len(config.XrayOptions.XHTTPSettings) > 0 {
			if err := json.Unmarshal(config.XrayOptions.XHTTPSettings, inbound.StreamSetting.SplitHTTPSettings); err != nil {
				return fmt.Errorf("unmarshal xhttp override error: %s", err)
			}
		}
	default:
		return errors.New("the network type is not vail")
	}
	return nil
}

func buildTrojan(config *conf.Options, nodeInfo *panel.NodeInfo, inbound *coreConf.InboundDetourConfig) error {
	inbound.Protocol = "trojan"
	v := nodeInfo.Trojan
	if config.XrayOptions.EnableFallback {
		// Set fallback
		fallbackConfigs, err := buildTrojanFallbacks(config.XrayOptions.FallBackConfigs)
		if err != nil {
			return err
		}
		s, err := json.Marshal(&coreConf.TrojanServerConfig{
			Fallbacks: fallbackConfigs,
		})
		inbound.Settings = (*json.RawMessage)(&s)
		if err != nil {
			return fmt.Errorf("marshal trojan fallback config error: %s", err)
		}
	} else {
		s := []byte("{}")
		inbound.Settings = (*json.RawMessage)(&s)
	}
	network := v.Network
	if network == "" {
		network = "tcp"
	}
	t := coreConf.TransportProtocol(network)
	inbound.StreamSetting = &coreConf.StreamConfig{Network: &t}
	switch network {
	case "tcp":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.TCPSettings)
		if err != nil {
			return fmt.Errorf("unmarshal tcp settings error: %s", err)
		}
	case "ws":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.WSSettings)
		if err != nil {
			return fmt.Errorf("unmarshal ws settings error: %s", err)
		}
	case "grpc":
		err := json.Unmarshal(v.NetworkSettings, &inbound.StreamSetting.GRPCSettings)
		if err != nil {
			return fmt.Errorf("unmarshal grpc settings error: %s", err)
		}
	default:
		return errors.New("the network type is not vail")
	}
	return nil
}

func buildShadowsocks(config *conf.Options, nodeInfo *panel.NodeInfo, inbound *coreConf.InboundDetourConfig) error {
	inbound.Protocol = "shadowsocks"
	s := nodeInfo.Shadowsocks
	settings := &coreConf.ShadowsocksServerConfig{
		Cipher: s.Cipher,
	}
	p := make([]byte, 32)
	_, err := rand.Read(p)
	if err != nil {
		return fmt.Errorf("generate random password error: %s", err)
	}
	randomPasswd := hex.EncodeToString(p)
	cipher := s.Cipher
	if s.ServerKey != "" {
		settings.Password = s.ServerKey
		randomPasswd = base64.StdEncoding.EncodeToString([]byte(randomPasswd))
		cipher = ""
	}
	defaultSSuser := &coreConf.ShadowsocksUserConfig{
		Cipher:   cipher,
		Password: randomPasswd,
	}
	settings.Users = append(settings.Users, defaultSSuser)
	settings.NetworkList = &coreConf.NetworkList{"tcp", "udp"}
	settings.IVCheck = true
	if config.XrayOptions.DisableIVCheck {
		settings.IVCheck = false
	}
	t := coreConf.TransportProtocol("tcp")
	inbound.StreamSetting = &coreConf.StreamConfig{Network: &t}
	sets, err := json.Marshal(settings)
	inbound.Settings = (*json.RawMessage)(&sets)
	if err != nil {
		return fmt.Errorf("marshal shadowsocks settings error: %s", err)
	}
	return nil
}

func buildVlessFallbacks(fallbackConfigs []conf.FallBackConfigForXray) ([]*coreConf.VLessInboundFallback, error) {
	if fallbackConfigs == nil {
		return nil, fmt.Errorf("you must provide FallBackConfigs")
	}
	vlessFallBacks := make([]*coreConf.VLessInboundFallback, len(fallbackConfigs))
	for i, c := range fallbackConfigs {
		if c.Dest == "" {
			return nil, fmt.Errorf("dest is required for fallback fialed")
		}
		var dest json.RawMessage
		dest, err := json.Marshal(c.Dest)
		if err != nil {
			return nil, fmt.Errorf("marshal dest %s config fialed: %s", dest, err)
		}
		vlessFallBacks[i] = &coreConf.VLessInboundFallback{
			Name: c.SNI,
			Alpn: c.Alpn,
			Path: c.Path,
			Dest: dest,
			Xver: c.ProxyProtocolVer,
		}
	}
	return vlessFallBacks, nil
}

// collectTLSCerts merges certificate sources for the TLS inbound: local
// primary, local ExtraCerts (file mode), and panel-delivered ExtraCerts
// (in-memory PEM). Order matters — xray-core falls back to Certs[0] when no
// SNI matches, so the primary must always be first.
func collectTLSCerts(cert *conf.CertConfig, nodeInfo *panel.NodeInfo) ([]*coreConf.TLSCertConfig, error) {
	var certs []*coreConf.TLSCertConfig
	primaryFromLocal := cert.CertFile != "" && cert.KeyFile != ""
	if primaryFromLocal {
		certs = append(certs, &coreConf.TLSCertConfig{
			CertFile:     cert.CertFile,
			KeyFile:      cert.KeyFile,
			OcspStapling: 3600,
		})
	}
	for _, e := range cert.ExtraCerts {
		if e.CertFile == "" || e.KeyFile == "" {
			continue
		}
		certs = append(certs, &coreConf.TLSCertConfig{
			CertFile:     e.CertFile,
			KeyFile:      e.KeyFile,
			OcspStapling: 3600,
		})
	}
	if nodeInfo != nil && nodeInfo.VAllss != nil {
		for i, ec := range nodeInfo.VAllss.ExtraCerts {
			if ec.Cert == "" || ec.Key == "" {
				continue
			}
			cfg := &coreConf.TLSCertConfig{
				CertStr:      []string{ec.Cert},
				KeyStr:       []string{ec.Key},
				OcspStapling: 3600,
			}
			if !primaryFromLocal && i == 0 {
				certs = append([]*coreConf.TLSCertConfig{cfg}, certs...)
			} else {
				certs = append(certs, cfg)
			}
		}
	}
	if len(certs) == 0 {
		return nil, errors.New("no TLS certificate available: neither local CertFile nor panel-delivered ExtraCerts provided")
	}
	return certs, nil
}

// resolveALPN picks the panel-delivered ALPN list when present, then local
// XrayOptions.ALPN, then defaults to h2 + http/1.1. Returns nil to leave the
// list unset on xray-core's TLSConfig (panel may force this by sending []).
func resolveALPN(option *conf.Options, nodeInfo *panel.NodeInfo) *coreConf.StringList {
	var alpn []string
	switch {
	case nodeInfo != nil && nodeInfo.VAllss != nil && nodeInfo.VAllss.ALPN != nil:
		alpn = nodeInfo.VAllss.ALPN
	case option.XrayOptions != nil && option.XrayOptions.ALPN != nil:
		alpn = option.XrayOptions.ALPN
	default:
		alpn = []string{"h2", "http/1.1"}
	}
	if len(alpn) == 0 {
		return nil
	}
	l := coreConf.StringList(alpn)
	return &l
}

// vlessDecryption derives the xray "decryption" string from panel-delivered
// encryption settings. Shared by fallback / non-fallback branches so the
// mlkem768x25519plus configuration isn't lost when EnableFallback is on.
func vlessDecryption(nodeInfo *panel.NodeInfo) (string, error) {
	if nodeInfo.VAllss == nil || nodeInfo.VAllss.Encryption == "" {
		return "none", nil
	}
	switch nodeInfo.VAllss.Encryption {
	case "mlkem768x25519plus":
		es := nodeInfo.VAllss.EncryptionSettings
		parts := []string{"mlkem768x25519plus", es.Mode, es.Ticket}
		if es.ServerPadding != "" {
			parts = append(parts, es.ServerPadding)
		}
		parts = append(parts, es.PrivateKey)
		return strings.Join(parts, "."), nil
	default:
		return "", fmt.Errorf("vless decryption method %s is not support", nodeInfo.VAllss.Encryption)
	}
}

func buildTrojanFallbacks(fallbackConfigs []conf.FallBackConfigForXray) ([]*coreConf.TrojanInboundFallback, error) {
	if fallbackConfigs == nil {
		return nil, fmt.Errorf("you must provide FallBackConfigs")
	}

	trojanFallBacks := make([]*coreConf.TrojanInboundFallback, len(fallbackConfigs))
	for i, c := range fallbackConfigs {

		if c.Dest == "" {
			return nil, fmt.Errorf("dest is required for fallback fialed")
		}

		var dest json.RawMessage
		dest, err := json.Marshal(c.Dest)
		if err != nil {
			return nil, fmt.Errorf("marshal dest %s config fialed: %s", dest, err)
		}
		trojanFallBacks[i] = &coreConf.TrojanInboundFallback{
			Name: c.SNI,
			Alpn: c.Alpn,
			Path: c.Path,
			Dest: dest,
			Xver: c.ProxyProtocolVer,
		}
	}
	return trojanFallBacks, nil
}
