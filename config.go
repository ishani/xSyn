package main

/* xSyn, a compact server implementing the xBrowserSync API;
 *
 * the config stuff is based on TOML; it will load a chosen file that
 * can be overridden by either command-line or envvar, by default 'prod.toml'
 *
 * each entry in the TOML structure hierarchy can optionally define a 'env' tag
 * we reflect across the whole lot, find these and check if the chosen envvars
 * are present, overriding the values if so.
 *
 * this means it's easy to develop locally and also easy to twist settings when
 * deploying a baked docker image by fiddling env vars
 *
 *
 * harry denholm, 2018; ishani.org
 *
 */

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"
)

type tomlConfig struct {
	Server   tomlServer
	Bolt     tomlBolt
	Security tomlSecurity
}
type tomlBolt struct {
	StorageFile string `toml:"file" env:"XS_BOLT_FILE"`
	InitTimeout int32  `toml:"init_timeout"`
}
type tomlServer struct {
	ReleaseMode    bool   `toml:"release_mode" env:"XS_SRV_RELEASE"`
	ServiceMessage string `toml:"service_message" env:"XS_SRV_MESSAGE"`
	MaxSyncSizeKb  int32  `toml:"max_sync_size_kb" env:"XS_SRV_MAXSYNC"`
	Port           int32  `toml:"port" env:"XS_SRV_PORT"`
	StatusRoute    string `toml:"status_route" env:"XS_SRV_STATUS"`
}
type tomlSecurity struct {
	ReqPerSecond     float64 `toml:"max_requests_per_second" env:"XS_SEC_RPS"`
	AcceptNewSyncs   bool    `toml:"accept_new_syncs" env:"XS_SEC_ACCEPT_NEW_SYNC"`
	SyncToggleRoute  string  `toml:"sync_toggle_route" env:"XS_SEC_SYNCTOGGLE"`
	TLSCert          string  `toml:"tls_cert" env:"XS_SEC_TLSCERT"`
	UseLetsEncrypt   string  `toml:"lets_encrypt" env:"XS_SEC_LE"`
	LetsEncryptCache string  `toml:"lets_encrypt_cache" env:"XS_SEC_LE_CACHE"`
}

// AppConfig is the config data parsed from disk
var AppConfig tomlConfig

// LoadConfig checks the command line for any -config= prefix changes, otherwise loads the default prod.toml
func LoadConfig() {

	// check for command-line override, default to 'prod'
	var configFilePrefix string
	flag.StringVar(&configFilePrefix, "config", "prod", "Set config file prefix")
	flag.Parse()

	// optional override from an envvar
	configFromEnv := os.Getenv("XS_CONFIG")
	if configFromEnv != "" {
		configFilePrefix = configFromEnv
	}

	configFilename := fmt.Sprintf("%s.toml", configFilePrefix)

	// create default structure for logging errors from config phase
	cfgLog := zLog.With(
		zap.String("phase", "config"),
		zap.String("configFile", configFilename))
	cfgLog.Info("Loading config...")

	cfgBytes, err := ioutil.ReadFile(configFilename)
	if err != nil {
		cfgLog.Panic("File not found")
	}

	// parse and map the data onto the structs
	if _, err := toml.Decode(string(cfgBytes), &AppConfig); err != nil {
		cfgLog.Panic("Decode failure", zap.Error(err))
	}

	// loop throught the config fields; anything with an 'env' tag allows for override with envvars
	if err = checkOverrides(&AppConfig, cfgLog); err != nil {
		cfgLog.Panic("Override failure", zap.Error(err))
	}
}

func checkOverrides(configData interface{}, cfgLog *zap.Logger) error {

	var err error

	smType := reflect.TypeOf(configData).Elem()
	smValue := reflect.ValueOf(configData).Elem()

	// walk each field, look for 'env' items
	for i := 0; i < smType.NumField(); i++ {

		fieldType := smType.Field(i)
		field := smValue.Field(i)

		envOverride := fieldType.Tag.Get("env")

		if envOverride != "" {

			overrideFromEnv := os.Getenv(envOverride)
			if overrideFromEnv != "" {
				cfgLog.Debug("Overriding config",
					zap.String("key", envOverride),
					zap.String("value", overrideFromEnv),
				)

				switch field.Kind() {
				case reflect.String:
					field.Set(reflect.ValueOf(overrideFromEnv))

				case reflect.Int32:
					ivalue, err := strconv.ParseInt(overrideFromEnv, 0, 32)
					if err != nil {
						return err
					}
					field.Set(reflect.ValueOf(int32(ivalue)))

				case reflect.Float64:
					fvalue, err := strconv.ParseFloat(overrideFromEnv, 64)
					if err != nil {
						return err
					}
					field.Set(reflect.ValueOf(float64(fvalue)))

				case reflect.Bool:
					bvalue, err := strconv.ParseBool(overrideFromEnv)
					if err != nil {
						return err
					}
					field.Set(reflect.ValueOf(bvalue))
				}

			}
		}

		if field.Kind().String() == "struct" {
			vx := smValue.Field(i).Addr()
			err = checkOverrides(vx.Interface(), cfgLog)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
