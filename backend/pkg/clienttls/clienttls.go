package clienttls

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	mgmtv1alpha1 "github.com/nucleuscloud/neosync/backend/gen/go/protos/mgmt/v1alpha1"
	"golang.org/x/sync/errgroup"
)

type ClientTlsFileConfig struct {
	RootCert *string

	ClientCert *string
	ClientKey  *string
}

type ClientTlsFileHandler = func(config *mgmtv1alpha1.ClientTlsConfig) (*ClientTlsFileConfig, error)

func UpsertCLientTlsFiles(config *mgmtv1alpha1.ClientTlsConfig) (*ClientTlsFileConfig, error) {
	if config == nil {
		return nil, errors.New("config was nil")
	}

	errgrp := errgroup.Group{}

	filenames := GetClientTlsFileNames(config)

	errgrp.Go(func() error {
		if filenames.RootCert == nil {
			return nil
		}
		_, err := os.Stat(*filenames.RootCert)
		if err != nil && !os.IsNotExist(err) {
			return err
		} else if err != nil && os.IsNotExist(err) {
			if err := os.WriteFile(*filenames.RootCert, []byte(config.GetRootCert()), 0600); err != nil {
				return err
			}
		}
		return nil
	})
	errgrp.Go(func() error {
		if filenames.ClientCert != nil && filenames.ClientKey != nil {
			_, err := os.Stat(*filenames.ClientKey)
			if err != nil && !os.IsNotExist(err) {
				return err
			} else if err != nil && os.IsNotExist(err) {
				if err := os.WriteFile(*filenames.ClientKey, []byte(config.GetClientKey()), 0600); err != nil {
					return err
				}
			}
		}
		return nil
	})
	errgrp.Go(func() error {
		if filenames.ClientCert != nil && filenames.ClientKey != nil {
			_, err := os.Stat(*filenames.ClientCert)
			if err != nil && !os.IsNotExist(err) {
				return err
			} else if err != nil && os.IsNotExist(err) {
				if err := os.WriteFile(*filenames.ClientCert, []byte(config.GetClientCert()), 0600); err != nil {
					return err
				}
			}
		}
		return nil
	})

	err := errgrp.Wait()
	if err != nil {
		return nil, err
	}

	return &filenames, nil
}

func GetClientTlsFileNames(config *mgmtv1alpha1.ClientTlsConfig) ClientTlsFileConfig {
	if config == nil {
		return ClientTlsFileConfig{}
	}

	basedir := os.TempDir()

	output := ClientTlsFileConfig{}
	if config.GetRootCert() != "" {
		content := hashContent(config.GetRootCert())
		fullpath := filepath.Join(basedir, content)
		output.RootCert = &fullpath
	}
	if config.GetClientCert() != "" && config.GetClientKey() != "" {
		certContent := hashContent(config.GetClientCert())
		certpath := filepath.Join(basedir, certContent)
		keyContent := hashContent(config.GetClientKey())
		keypath := filepath.Join(basedir, keyContent)
		output.ClientCert = &certpath
		output.ClientKey = &keypath
	}
	return output
}

func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
