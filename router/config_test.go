package router

import (
	"os"
	"testing"
)

func resetEnv() {
	// Validate annotations
	for _, name := range []string{
		"NAMESPACE_LABEL_SELECTOR",
		"ORG_ANNOTATION",
		"PORT",
	} {
		os.Unsetenv(name)
	}
}

/*
Test for ConfigFromEnv should throw error on invalid label selector
*/
func TestConfigFromEnvInvailidLabelSelector(t *testing.T) {
	resetEnv()
	os.Setenv("NAMESPACE_LABEL_SELECTOR", "...invalid selector")
	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("Error should not nil")
	}
}

/*
Test for ConfigFromEnv should throw error on invalid annotation
*/
func TestConfigFromEnvInvailidAnnotation(t *testing.T) {
	resetEnv()
	os.Setenv("ORG_ANNOTATION", "...")
	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("Error should not nil")
	}
}

/*
Test for ConfigFromEnv should throw error on invalid Port
*/
func TestConfigFromEnvInvailidPort(t *testing.T) {
	resetEnv()
	os.Setenv("PORT", "-1")
	_, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("Error should not nil")
	}
}

/*
Test for ConfigFromEnv config should contain unmarshed data
*/
func TestConfigFromEnv(t *testing.T) {
	resetEnv()
	os.Setenv("PORT", "12345")
	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("Error should be null. %s", err)
	}

	if config.Nginx.Port != 12345 {
		t.Fatalf("Expected port to be 12345 was %d", config.Nginx.Port)
	}

}
