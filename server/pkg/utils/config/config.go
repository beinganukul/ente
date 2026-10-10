package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ente/stacktrace"
	"github.com/spf13/viper"
)

func ConfigureViper(environment string) error {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ENTE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	viper.SetConfigFile("configurations/" + environment + ".yaml")
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	credentialsFile := viper.GetString("credentials-file")
	if credentialsFile == "" {
		credentialsFile = "credentials.yaml"
	}
	err = mergeConfigFileIfExists(credentialsFile)
	if err != nil {
		return err
	}

	err = mergeConfigFileIfExists("museum.yaml")
	if err != nil {
		return err
	}

	return nil
}

func mergeConfigFileIfExists(configFile string) error {
	configFileExists, err := doesFileExist(configFile)
	if err != nil {
		return err
	}
	if configFileExists {
		viper.SetConfigFile(configFile)
		err = viper.MergeInConfig()
		if err != nil {
			return err
		}
	}

	return nil
}

func doesFileExist(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info == nil {
		return false, nil
	}
	// Return false if the stat entry exists, but is a directory.
	//
	// This allows us to ignore the default museum.yaml directory that gets
	// mounted on a fresh checkout.
	if info.IsDir() {
		return false, nil
	}
	return true, nil
}

func GetPGInfo() string {
	return fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=%s %s",
		quotePGValue(viper.GetString("db.host")),
		viper.GetInt("db.port"),
		quotePGValue(viper.GetString("db.user")),
		quotePGValue(viper.GetString("db.password")),
		quotePGValue(viper.GetString("db.name")),
		quotePGValue(viper.GetString("db.sslmode")),
		viper.GetString("db.extra"))
}

func quotePGValue(value string) string {
	// lib/pq's keyword DSNs require quoted empty values and backslash escaping.
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(value) + "'"
}

func IsLocalEnvironment() bool {
	evn := os.Getenv("ENVIRONMENT")
	return evn == "" || evn == "local"
}

func CredentialFilePath(name string) (string, error) {
	credentialsDir := viper.GetString("credentials-dir")
	if credentialsDir == "" {
		credentialsDir = "credentials"
	}

	path := credentialsDir + "/" + name
	return productionFilePath(path)
}

func BillingConfigFilePath(name string) (string, error) {
	billingConfigDir := viper.GetString("billing-config-dir")
	if billingConfigDir == "" {
		billingConfigDir = "data/billing/"
	}

	path := billingConfigDir + name
	return productionFilePath(path)
}

func productionFilePath(path string) (string, error) {
	pathExists, err := doesFileExist(path)
	if err != nil {
		return "", stacktrace.Propagate(err, "")
	}
	if pathExists {
		return path, nil
	}
	if IsLocalEnvironment() || os.Getenv("ENVIRONMENT") == "workers" {
		return "", nil
	}
	return "", fmt.Errorf("required file not found at %s", path)
}
