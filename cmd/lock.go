/*
Copyright 2026 Serge Logvinov.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

const (
	defaultLockTimeout = 10 * time.Minute
	lockPrefix         = "helm-lock-"
)

// lockOptions holds the configuration for the lock command
type lockOptions struct {
	releaseName string
	timeout     time.Duration

	helmSettings *cli.EnvSettings
	helmCommand  string
	helmFlags    []string
	helmArgs     []string
}

func runLockCommand(ctx context.Context, opts *lockOptions) error {
	log.SetFlags(0)

	if opts.releaseName == "" {
		return fmt.Errorf("release name is required")
	}

	config, err := opts.helmSettings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(opts.helmSettings.RESTClientGetter(), opts.helmSettings.Namespace(), os.Getenv("HELM_DRIVER"), func(_ string, _ ...any) {}); err != nil {
		return fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	log.Printf("Checking release '%s' in namespace '%s'", opts.releaseName, opts.helmSettings.Namespace())

	releaseStatus, err := getReleaseStatus(actionConfig, opts.releaseName)
	if err != nil {
		return fmt.Errorf("failed to check release status: %w", err)
	}

	lockName := lockPrefix + opts.releaseName
	if err := acquireLockAndExecute(ctx, clientset, actionConfig, opts, lockName, opts.helmSettings.Namespace(), releaseStatus); err != nil {
		return err
	}

	return nil
}

// acquireLockAndExecute acquires a lock, performs rollback if needed, executes helm command, then releases lock
func acquireLockAndExecute(ctx context.Context, client kubernetes.Interface, actionConfig *action.Configuration, opts *lockOptions, lockName, namespace string, releaseStatus release.Status) error {
	lockCtx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	if !opts.helmSettings.Debug {
		lockCtx = klog.NewContext(lockCtx, klog.TODO().V(1))
	}

	identity := fmt.Sprintf("helm-lock-%s-%d", opts.helmCommand, time.Now().Unix())

	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		namespace,
		lockName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: identity,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create resource lock: %w", err)
	}

	operationCompleted := make(chan error, 1)

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Printf("Acquired lock '%s' for %s operation", lockName, opts.helmCommand)

				if releaseStatus != release.StatusDeployed && releaseStatus != release.StatusUnknown {
					log.Printf("Release status is '%s', performing rollback first", releaseStatus)

					if err := performRollback(actionConfig, opts.releaseName); err != nil {
						operationCompleted <- fmt.Errorf("rollback failed: %w", err)

						return
					}
				}

				if err := executeHelmCommand(ctx, opts); err != nil {
					operationCompleted <- err

					return
				}

				operationCompleted <- nil
			},
			OnStoppedLeading: func() {},
		},
	}

	go func() {
		leaderelection.RunOrDie(lockCtx, leaderElectionConfig)
	}()

	select {
	case err := <-operationCompleted:
		cancel()

		if err != nil {
			return err
		}

		return nil
	case <-lockCtx.Done():
		return fmt.Errorf("failed to acquire lock or operation timed out: %w", lockCtx.Err())
	}
}

// executeHelmCommand executes the original helm command
func executeHelmCommand(ctx context.Context, opts *lockOptions) error {
	args := append([]string{opts.helmCommand}, opts.helmArgs...)
	args = append(args, opts.helmFlags...)

	log.Printf("Executing: helm %s\n\n", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
