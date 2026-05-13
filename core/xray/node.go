package xray

import (
	"context"
	"fmt"

	"github.com/InazumaV/V2bX/api/panel"
	"github.com/InazumaV/V2bX/conf"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/outbound"
)

type DNSConfig struct {
	Servers []interface{} `json:"servers"`
	Tag     string        `json:"tag"`
}

func (c *Xray) AddNode(tag string, info *panel.NodeInfo, config *conf.Options) error {
	c.nodeReportMinTrafficBytes[tag] = config.ReportMinTraffic * 1024
	err := updateDNSConfig(info)
	if err != nil {
		return fmt.Errorf("build dns error: %s", err)
	}

	// Start builtin fallback HTTP server if enabled — panel-delivered spec
	// takes precedence over local XrayOptions.BuiltinFallback. Only meaningful
	// when EnableFallback is also true (otherwise xray won't emit Fallbacks).
	if spec := resolveBuiltinFallback(info, config); spec != nil && spec.Enabled && config.XrayOptions != nil && config.XrayOptions.EnableFallback {
		fb := NewFallbackServer(tag, *spec)
		sock, err := fb.Start()
		if err != nil {
			return fmt.Errorf("start builtin fallback: %s", err)
		}
		c.fallbackServers[tag] = fb
		config.BuiltinFallbackSocket = sock
	}

	inboundConfig, err := buildInbound(config, info, tag)
	if err != nil {
		return fmt.Errorf("build inbound error: %s", err)
	}
	err = c.addInbound(inboundConfig)
	if err != nil {
		return fmt.Errorf("add inbound error: %s", err)
	}
	outBoundConfig, err := buildOutbound(config, tag)
	if err != nil {
		return fmt.Errorf("build outbound error: %s", err)
	}
	err = c.addOutbound(outBoundConfig)
	if err != nil {
		return fmt.Errorf("add outbound error: %s", err)
	}
	return nil
}

// resolveBuiltinFallback picks the panel spec if present, otherwise the local
// one. Returns nil when neither side configured anything.
func resolveBuiltinFallback(info *panel.NodeInfo, config *conf.Options) *conf.BuiltinFallbackSpec {
	if info != nil && info.VAllss != nil && info.VAllss.BuiltinFallback != nil {
		p := info.VAllss.BuiltinFallback
		return &conf.BuiltinFallbackSpec{
			Enabled: p.Enabled,
			Mode:    p.Mode,
			Status:  p.Status,
			Headers: p.Headers,
			Body:    p.Body,
		}
	}
	if config != nil && config.XrayOptions != nil && config.XrayOptions.BuiltinFallback != nil {
		return config.XrayOptions.BuiltinFallback
	}
	return nil
}

func (c *Xray) addInbound(config *core.InboundHandlerConfig) error {
	rawHandler, err := core.CreateObject(c.Server, config)
	if err != nil {
		return err
	}
	handler, ok := rawHandler.(inbound.Handler)
	if !ok {
		return fmt.Errorf("not an InboundHandler: %s", err)
	}
	if err := c.ihm.AddHandler(context.Background(), handler); err != nil {
		return err
	}
	return nil
}

func (c *Xray) addOutbound(config *core.OutboundHandlerConfig) error {
	rawHandler, err := core.CreateObject(c.Server, config)
	if err != nil {
		return err
	}
	handler, ok := rawHandler.(outbound.Handler)
	if !ok {
		return fmt.Errorf("not an InboundHandler: %s", err)
	}
	if err := c.ohm.AddHandler(context.Background(), handler); err != nil {
		return err
	}
	return nil
}

func (c *Xray) DelNode(tag string) error {
	err := c.removeInbound(tag)
	if err != nil {
		return fmt.Errorf("remove in error: %s", err)
	}
	err = c.removeOutbound(tag)
	if err != nil {
		return fmt.Errorf("remove out error: %s", err)
	}
	if fb, ok := c.fallbackServers[tag]; ok {
		_ = fb.Stop()
		delete(c.fallbackServers, tag)
	}
	return nil
}

func (c *Xray) removeInbound(tag string) error {
	return c.ihm.RemoveHandler(context.Background(), tag)
}

func (c *Xray) removeOutbound(tag string) error {
	err := c.ohm.RemoveHandler(context.Background(), tag)
	return err
}
