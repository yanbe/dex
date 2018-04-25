package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"plugin"

	"github.com/coreos/dex/connector"
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

	connector := &pluginConnector{
		logger:         logger,
		configJSON:     conf.PluginConfig,
		loginURL:       loginURL,
		handleCallback: handleCallback,
	}

	if initSym, err := plug.Lookup("Init"); err == nil {
		init, ok := initSym.(func(Plugin) error)
		if !ok {
			return nil, fmt.Errorf("Init does not implement required interface")
		}
		err := init(connector)
		if err != nil {
			return nil, err
		}
	}

	return connector, nil
}
