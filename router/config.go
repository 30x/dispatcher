package router

import (
	"fmt"
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
)

/*
Config is the structure containing the router configuration
*/
type Config struct {
	// The secret name used to store the API Key for the namespace
	APIKeySecret string
	// The secret data field name to store the API Key for the namespace
	APIKeySecretDataField string
	// The label selector used to identify routable namespaces
	NamespaceRoutableLabelSelector string
	// The name of the annotation used to find hosts to route on the namespace
	NamespaceHostsAnnotation string
	// The name of the annotation used to find org name of namespace
	NamespaceOrgAnnotation string
	// The name of the annotation used to find env name of namespace
	NamespaceEnvAnnotation string
	// Nginx Specific configurations
	Nginx NginxConfig
	// The name of the annotation used to find routing information
	PodsPathsAnnotation string
	// The label selector used to identify routable objects
	PodsRoutableLabelSelector string
}

/*
NginxConfig is the structure containing the nginx specific configuration
*/
type NginxConfig struct {
	// The header name used to identify the API Key
	APIKeyHeader string
	// Enable or disable nginx health checks for each pod
	EnableHealthChecks bool
	// Max client request body size. nginx config: client_max_body_size. eg 10m
	MaxClientBodySize string
	// The port that nginx will listen on
	Port int
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
	// The label selector used to identify routable namespaces
	addConfig("NamespaceRoutableLabelSelector", "NAMESPACE_LABEL_SELECTOR", "github.com/30x.dispatcher.ns=true")
	// The name of the annotation used to find hosts to route on the namespace
	addConfig("NamespaceHostsAnnotation", "HOSTS_ANNOTATION", "github.com/30x.dispatcher.hosts")
	// The name of the annotation used to find org name of namespace
	addConfig("NamespaceOrgAnnotation", "ORG_ANNOTATION", "github.com/30x.dispatcher.org")
	// The name of the annotation used to find env name of namespace
	addConfig("NamespaceEnvAnnotation", "ENV_ANNOTATION", "github.com/30x.dispatcher.env")
	// The label selector used to identify routable objects
	addConfig("PodsRoutableLabelSelector", "ROUTABLE_LABEL_SELECTOR", "github.com/30x.dispatcher.routable=true")
	// The name of the annotation used to find routing information
	addConfig("PodsPathsAnnotation", "PATHS_ANNOTATION", "github.com/30x.dispatcher.paths")

	// Nginx Configuration
	//
	// The header name used to identify the API Key
	addConfig("Nginx.APIKeyHeader", "API_KEY_HEADER", "X-ROUTING-API-KEY")
	// Enable or disable nginx health checks using custom upstream check module. Default: disabled
	addConfig("Nginx.EnableHealthChecks", "NGINX_ENABLE_HEALTH_CHECKS", false)
	// Nginx max client request size. Default 0, unlimited
	addConfig("Nginx.MaxClientBodySize", "NGINX_MAX_CLIENT_BODY_SIZE", "0")
	// The port that nginx will listen on
	addConfig("Nginx.Port", "PORT", "80")

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Validate annotations
	for _, annotation := range []string{
		config.NamespaceHostsAnnotation,
		config.NamespaceOrgAnnotation,
		config.NamespaceEnvAnnotation,
		config.PodsPathsAnnotation,
	} {
		err = validateAnnotation(annotation)
		if err != nil {
			return nil, err
		}
	}

	// Validate Nginx port
	if err != nil || !utils.IsValidPort(config.Nginx.Port) {
		return nil, fmt.Errorf(ErrMsgTmplInvalidPort, config.Nginx.Port)
	}

	// Validate label selectors
	for _, selector := range []string{
		config.NamespaceRoutableLabelSelector,
		config.PodsRoutableLabelSelector,
	} {
		err = validateLabelSelector(selector)
		if err != nil {
			return nil, err
		}
	}

	return &config, nil
}
