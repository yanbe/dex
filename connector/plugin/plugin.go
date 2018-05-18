package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"plugin"

	"github.com/dexidp/dex/connector"
	"github.com/sirupsen/logrus"
)

// Config holds configuration parameters for a plugin connector.
type Config struct {
	Path         string          `json:"path"`
	PluginConfig json.RawMessage `json:"pluginConfig"`
}

// Plugin holds ... and passed to the implementation.
type Plugin interface {
	ConfigJSON() json.RawMessage
	Logger() logrus.FieldLogger
}

type pluginConnector struct {
	logger         logrus.FieldLogger
	configJSON     json.RawMessage
	loginURL       func(plugin Plugin, scopes connector.Scopes, callbackURL, state string) (string, error)
	handleCallback func(plugin Plugin, scopes connector.Scopes, r *http.Request) (connector.Identity, error)
	refresh        func(plugin Plugin, ctx context.Context, s connector.Scopes, identity connector.Identity) (connector.Identity, error)
}

func (c *pluginConnector) ConfigJSON() json.RawMessage {
	return c.configJSON
}

func (c *pluginConnector) Logger() logrus.FieldLogger {
	return c.logger
}

func (c *pluginConnector) LoginURL(scopes connector.Scopes, callbackURL, state string) (string, error) {
	return c.loginURL(c, scopes, callbackURL, state)
}

func (c *pluginConnector) HandleCallback(scopes connector.Scopes, r *http.Request) (connector.Identity, error) {
	return c.handleCallback(c, scopes, r)
}

func (c *pluginConnector) Refresh(ctx context.Context, scopes connector.Scopes, identity connector.Identity) (connector.Identity, error) {
	if c.refresh == nil {
		return identity, fmt.Errorf("refresh() is not implemented")
	}

	return c.refresh(c, ctx, scopes, identity)
}

// Open returns a connector whose details are implemented in a plugin located
// at conf.Path.
func (conf Config) Open(id string, logger logrus.FieldLogger) (connector.Connector, error) {
	plug, err := plugin.Open(conf.Path)
	if err != nil {
		return nil, err
	}

	loginURLSym, err := plug.Lookup("LoginURL")
	if err != nil {
		return nil, err
	}
	loginURL, ok := loginURLSym.(func(Plugin, connector.Scopes, string, string) (string, error))
	if !ok {
		return nil, fmt.Errorf("LoginURL does not implement required interface")
	}

	handleCallbackSym, err := plug.Lookup("HandleCallback")
	if err != nil {
		return nil, err
	}
	handleCallback, ok := handleCallbackSym.(func(Plugin, connector.Scopes, *http.Request) (connector.Identity, error))
	if !ok {
		return nil, fmt.Errorf("HandleCallback does not implement required interface")
	}

	conn := &pluginConnector{
		logger:         logger,
		configJSON:     conf.PluginConfig,
		loginURL:       loginURL,
		handleCallback: handleCallback,
	}

	if refreshSym, err := plug.Lookup("Refresh"); err == nil {
		conn.refresh, ok = refreshSym.(func(plugin Plugin, ctx context.Context, s connector.Scopes, identity connector.Identity) (connector.Identity, error))
		if !ok {
			return nil, fmt.Errorf("Refresh does not implement required interface")
		}
	}

	if initSym, err := plug.Lookup("Init"); err == nil {
		init, ok := initSym.(func(Plugin) error)
		if !ok {
			return nil, fmt.Errorf("Init does not implement required interface")
		}
		err := init(conn)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}
