// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caas

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/juju/cmd"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/utils/exec"
)

//go:generate go run github.com/golang/mock/mockgen -package mocks -destination mocks/runner_mock.go github.com/juju/juju/cmd/juju/caas CommandRunner

type CommandRunner interface {
	RunCommands(run exec.RunParams) (*exec.ExecResponse, error)
}

type defaultRunner struct{}

func (*defaultRunner) RunCommands(run exec.RunParams) (*exec.ExecResponse, error) {
	return exec.RunCommands(run)
}

func getEnv(key string) string {
	result := os.Getenv(strings.ToUpper(key))
	if result == "" {
		result = os.Getenv(strings.ToLower(key))
	}
	return result
}

func collapseRunError(result *exec.ExecResponse, err error) error {
	if err != nil {
		return errors.Trace(err)
	}
	if result == nil {
		return nil
	}
	if result.Code != 0 {
		return errors.New(string(result.Stderr))
	}
	return nil
}

func mergeEnv(envs ...[]string) (out []string) {
	m := map[string]string{}
	keys := set.NewStrings()
	for _, env := range envs {
		for _, val := range env {
			kv := strings.SplitN(val, "=", 2)
			k := kv[0]
			m[k] = kv[1]
			keys.Add(k)
		}
	}
	// sort keys for test.
	for _, k := range keys.SortedValues() {
		out = append(out, fmt.Sprintf("%s=%s", k, m[k]))
	}
	return out
}

var runCommand = func(runner CommandRunner, params []string, kubeconfig string) (*exec.ExecResponse, error) {
	cmd := strings.Join(params, " ")

	execParams := exec.RunParams{
		Commands:    cmd,
		Environment: os.Environ(),
	}
	if len(kubeconfig) > 0 {
		execParams.Environment = mergeEnv(execParams.Environment, []string{"KUBECONFIG=" + kubeconfig})
	}
	return runner.RunCommands(execParams)
}

type clusterParams struct {
	name       string
	project    string
	region     string
	zone       string
	credential string
	// used with AKS
	resourceGroup string
}

type cluster struct {
	name   string
	region string
	zone   string
	// for AKS
	resourceGroup string
}

type k8sCluster interface {
	CommandRunner
	getKubeConfig(p *clusterParams) (io.ReadCloser, string, error)
	interactiveParams(ctx *cmd.Context, p *clusterParams) (*clusterParams, error)
	cloud() string
	ensureExecutable() error
}
