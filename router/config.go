package router

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/30x/dispatcher/utils"
	"github.com/spf13/viper"

	"k8s.io/client-go/pkg/labels"
	"k8s.io/client-go/pkg/util/validation"
)

const (
	// ErrMsgTmplInvalidAnnotationName is the error message template for an invalid annotation name
	ErrMsgTmplInvalidAnnotationName = "invalid annotation name: %s %v"
	// ErrMsgTmplInvalidLabelSelector is the error message template for an invalid label selector
	ErrMsgTmplInvalidLabelSelector = "has an invalid label selector: %s $v\n"
	// ErrMsgTmplInvalidPort is the error message template for an invalid port
	ErrMsgTmplInvalidPort = "%s is an invalid port\n"
	// ErrMsgTmplInvalidServerReturnHTTPStatusCode is the error message template for invalid status code
	ErrMsgTmplInvalidServerReturnHTTPStatusCode = "%d is an invalid status code 100-999 for default server return"
	// ErrMsgTmplInvalidServerReturnURL is the error message for an invalid url used for default server
	ErrMsgTmplInvalidServerReturnURL = "%s is an invalid url for default server return %v"
	//ErrMsgTmplInvalidPath is the error message for an invalid path
	ErrMsgTmplInvalidPath = "%s is an invalid path"
	//ErrMsgTmplMissingSSLKey is the error message for missing ssl key
	ErrMsgTmplMissingSSLKey = "must provide a ssl key when ssl cert is provided"
	// ErrMsgTmplSSLCertMissing error for when ssl cert/key does not exist on disk
	ErrMsgTmplSSLCertMissing = "ssl cert does not exist %s"
)

/*
Config is the structure containing the router configuration
*/
type Config struct {
	// The secret name used to store the API Key for the namespace
	APIKeySecret string
	// The secret data field name to store the API Key for the namespace
	APIKeySecretDataField string
	// The label selector used to identify routable namespaces and pods
	RoutableLabelSelector string
	// The name of the annotation used to find hosts to route on the namespace
	NamespaceHostsAnnotation string
	// The name of the label used to find org name of namespace
	NamespaceOrgLabel string
	// The name of the label used to find env name of namespace
	NamespaceEnvLabel string
	// Nginx Specific configurations
	Nginx NginxConfig
	// The name of the annotation used to find routing information
	PodsPathsAnnotation string
	// The name of the label used for applications name
	PodsAppNameLabel string
	// The name of the label used for applications revision
	PodsAppRevLabel string
}

/*
NginxConfig is the structure containing the nginx specific configuration
*/
type NginxConfig struct {
	// The header name used to identify the API Key
	APIKeyHeader string
	// Enable or disable nginx health checks for each pod
	EnableHealthChecks bool
	// Status path for nginx status endpoint on default server.
	StatusPath string
	// Max client request body size. nginx config: client_max_body_size. eg 10m
	MaxClientBodySize string
	// The port that nginx will listen on
	Port int
	// The port that nginx will listen for ssl connections
	SSLPort int
	// SSLCertificateDir path to store certificates
	SSLCertificateDir string
	// SSLCert the default ssl cert used for all request that don't match a valid ssl host
	SSLCert string
	// SSLKey the default ssl cert key used for all request that don't match a valid ssl host
	SSLKey string
	// SSLEnabled flag to enable/disable ssl support, set by ConfigFromEnv
	SSLEnabled bool
	// Default location return if request does not match any patjs, Defaults: 404
	DefaultLocationReturn string
	// RunInMockMode enables starting/stopping nginx if disabled. In mock mode starting/stopping is ignored.
	RunInMockMode bool
}

// addConfig adds a default and env binding to viper
func addConfig(prop, env string, value interface{}) {
	viper.SetDefault(prop, value)
	viper.BindEnv(prop, env)
}

// validateAnnotation validates k8s annotation
func validateAnnotation(value string) error {
	errs := validation.IsQualifiedName(strings.ToLower(value))
	if len(errs) > 0 {
		return fmt.Errorf(ErrMsgTmplInvalidAnnotationName, value, errs[0])
	}
	return nil
}

// validateLabelSelector validates k8s label selector query
func validateLabelSelector(value string) error {
	_, err := labels.Parse(value)
	if err != nil {
		return fmt.Errorf(ErrMsgTmplInvalidLabelSelector, value, err)
	}
	return nil
}

/*
ConfigFromEnv returns the configuration based on the environment variables and validates the values
*/
func ConfigFromEnv() (*Config, error) {

	// Router Configuration
	//
	// The secret name used to store the API Key for the namespace
	addConfig("APIKeySecret", "API_KEY_SECRET_NAME", "routing")
	// The secret data field name to store the API Key for the namespace
	addConfig("APIKeySecretDataField", "API_KEY_SECRET_FIELD", "api-key")
	// The label selector used to identify routable namespaces and pods
	addConfig("RoutableLabelSelector", "ROUTABLE_LABEL_SELECTOR", "github.com/30x.dispatcher.routable=true")
	// The name of the annotation used to find hosts to route on the namespace
	addConfig("NamespaceHostsAnnotation", "HOSTS_ANNOTATION", "github.com/30x.dispatcher.hosts")
	// The name of the lable used to find org name of namespace
	addConfig("NamespaceOrgLabel", "ORG_LABEL", "github.com/30x.dispatcher.org")
	// The name of the lable used to find env name of namespace
	addConfig("NamespaceEnvLabel", "ENV_LABEL", "github.com/30x.dispatcher.env")
	// The name of the annotation used to find routing information
	addConfig("PodsPathsAnnotation", "PATHS_ANNOTATION", "github.com/30x.dispatcher.paths")
	// The name of the lable used to find app name of the pod
	addConfig("PodsAppNameLabel", "APP_NAME_LABEL", "github.com/30x.dispatcher.app.name")
	// The name of the lable used to find app revision of the pod
	addConfig("PodsAppRevLabel", "APP_REV_LABEL", "github.com/30x.dispatcher.app.rev")

	// Nginx Configuration
	//
	// The header name used to identify the API Key
	addConfig("Nginx.APIKeyHeader", "API_KEY_HEADER", "X-ROUTING-API-KEY")
	// Enable or disable nginx health checks using custom upstream check module. Default: disabled
	addConfig("Nginx.EnableHealthChecks", "NGINX_ENABLE_HEALTH_CHECKS", false)
	// Enable or disable /dispatcher/status endpoint on default nginx server.
	addConfig("Nginx.StatusPath", "NGINX_STATUS_PATH", "/dispatcher/status")
	// Nginx max client request size. Default 0, unlimited
	addConfig("Nginx.MaxClientBodySize", "NGINX_MAX_CLIENT_BODY_SIZE", "0")
	// The port that nginx will listen on
	addConfig("Nginx.Port", "PORT", "80")
	// The port that nginx will listen on for ssl connections
	addConfig("Nginx.SSLPort", "SSL_PORT", "443")
	// SSLKey the default ssl cert used for all request that don't match a valid ssl host
	addConfig("Nginx.SSLCert", "SSL_CERT", "")
	// SSLKey the default ssl cert key used for all request that don't match a valid ssl host
	addConfig("Nginx.SSLKey", "SSL_KEY", "")
	// Dir to write ssl certs to
	addConfig("Nginx.SSLCertificateDir", "SSL_CERT_DIR", "/etc/nginx/ssl")
	// If request does not match any paths nginx will return a status code or uri, defaults to 404
	addConfig("Nginx.DefaultLocationReturn", "DEFAULT_LOCATION_RETURN", "404")

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Validate annotations
	for _, annotation := range []string{
		config.NamespaceHostsAnnotation,
		config.PodsPathsAnnotation,
	} {
		err = validateAnnotation(annotation)
		if err != nil {
			return nil, err
		}
	}

	// Validate Nginx port
	if !utils.IsValidPort(config.Nginx.Port) {
		return nil, fmt.Errorf(ErrMsgTmplInvalidPort, config.Nginx.Port)
	}

	// Validate Nginx port
	if !utils.IsValidPort(config.Nginx.SSLPort) {
		return nil, fmt.Errorf(ErrMsgTmplInvalidPort, config.Nginx.SSLPort)
	}

	// Validate label selectors
	for _, selector := range []string{
		config.RoutableLabelSelector,
		config.NamespaceOrgLabel,
		config.NamespaceEnvLabel,
		config.PodsAppNameLabel,
		config.PodsAppRevLabel,
	} {
		err = validateLabelSelector(selector)
		if err != nil {
			return nil, err
		}
	}

	// Validate default server return can either be a http status code or valid url
	code, err := strconv.Atoi(config.Nginx.DefaultLocationReturn)
	if err != nil {
		// check for valid url
		_, err := url.Parse(config.Nginx.DefaultLocationReturn)
		if err != nil {
			return nil, fmt.Errorf(ErrMsgTmplInvalidServerReturnURL, config.Nginx.DefaultLocationReturn, err)
		}
	} else {
		if code < 100 || code > 999 {
			return nil, fmt.Errorf(ErrMsgTmplInvalidServerReturnHTTPStatusCode, code)
		}
	}

	// Validate nginx status path
	if !validatePath(config.Nginx.StatusPath) {
		return nil, fmt.Errorf(ErrMsgTmplInvalidPath, config.Nginx.StatusPath)
	}

	// Validate SSL Cert
	if config.Nginx.SSLCert != "" {
		// If cert if provided must have key
		if config.Nginx.SSLKey == "" {
			return nil, fmt.Errorf(ErrMsgTmplMissingSSLKey)
		}

		// check if ssl cert exists
		if _, err := os.Stat(config.Nginx.SSLCert); os.IsNotExist(err) {
			return nil, fmt.Errorf(ErrMsgTmplSSLCertMissing, config.Nginx.SSLCert)
		}

		// check if ssl key exists
		if _, err := os.Stat(config.Nginx.SSLCert); os.IsNotExist(err) {
			return nil, fmt.Errorf(ErrMsgTmplSSLCertMissing, config.Nginx.SSLKey)
		}

		config.Nginx.SSLEnabled = true
	}

	return &config, nil
}
