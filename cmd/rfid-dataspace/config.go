package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Dataspace/config"
	"github.com/je4/Library-RFID-Dataspace/pkg/web"
	utilsconfig "github.com/je4/utils/v2/pkg/config"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// ThemeConfig defines explicit light and dark themes to use for auto/light/dark switching.
type ThemeConfig struct {
	Dark  string `toml:"dark"`
	Light string `toml:"light"`
}

type ElasticSearchConfig struct {
	Endpoint []string              `toml:"endpoint"`
	Index    string                `toml:"index"`
	APIKey   utilsconfig.EnvString `toml:"apikey"`
}

// DataspaceAppConfig defines configuration parameters for the rfid-dataspace application.
//
// Fields:
//   - Addr: Local listen address for the HTTP web server (e.g. "localhost:5000").
//   - ExtAddr: External address for links and display (e.g. "http://localhost:5000").
//   - Name: Unique client identifier (e.g. "rfid-dataspace-01") for WSBroker authorization.
//   - WSBroker: Target broker WebSocket endpoint URL (e.g. "websocket-broker.k8s-tls.ub.unibas.ch").
//   - Channels: List of WSBroker channels to subscribe on connect (e.g. ["mediathek01"]).
//   - DefaultISIL: Default ISIL identifier formatted as <Country>-<ISIL> (e.g. "CH-001596-0").
//   - DefaultAFI: Default AFI hex byte (e.g. "07" for secured, "C2" for unsecured).
//   - MaxTags: Maximum number of scanned tags to retain in memory (default: 100).
//   - ReadLimit: Maximum WebSocket message read limit in bytes (optional).
//   - Themes: Explicit light and dark themes (optional).
//   - ElasticSearch: Configuration for Elasticsearch catalog lookups.
//   - SourceRecords: Swisscovery source record mappings.
//   - ClientTLS: mTLS configuration via certloader (MiniVault, File, Env, Dev, or ACME).
//   - WebTLS: TLS configuration for the local web server.
type DataspaceAppConfig struct {
	Addr          string                    `toml:"addr"`
	ExtAddr       string                    `toml:"extaddr"`
	Name          string                    `toml:"name"`
	WSBroker      string                    `toml:"wsbroker"`
	Channels      []string                  `toml:"channels"`
	DefaultISIL   string                    `toml:"default_isil"`
	DefaultAFI    string                    `toml:"default_afi"`
	MaxTags       int                       `toml:"max_tags"`
	ReadLimit     int64                     `toml:"read_limit"`
	Themes        ThemeConfig               `toml:"themes"`
	ElasticSearch ElasticSearchConfig       `toml:"elasticsearch"`
	SourceRecords []web.SourceRecordMapping `toml:"source_records"`
	ClientTLS     loader.Config             `toml:"clienttls"`
	WebTLS        loader.Config             `toml:"webtls"`
}

// Config is an alias for DataspaceAppConfig.
type Config = DataspaceAppConfig

// LoadConfig loads the DataspaceAppConfig from the specified TOML file path, or falls back
// to the embedded default configuration if path is empty.
func LoadConfig(path string) (*DataspaceAppConfig, error) {
	conf := &DataspaceAppConfig{}
	if path == "" {
		if _, err := toml.Decode(config.RFIDDataspaceConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default config")
		}
		return conf, nil
	}
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return nil, errors.Wrapf(err, "cannot decode toml file '%s'", path)
	}
	return conf, nil
}
