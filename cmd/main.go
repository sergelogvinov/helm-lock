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
	"os"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/cli"
)

const globalUsage = `This plugin manages Helm release locks using Kubernetes leader election.
It checks if a Helm release is in a deployed state, and if not, verifies
if there's an active lock preventing deployment. If the lock has timed out,
it performs a rollback operation. After it runs the specified Helm command.`

// Run the main command for the helm-lock CLI application.
func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := &lockOptions{
		timeout:      defaultLockTimeout,
		helmSettings: cli.New(),
		helmFlags:    getAllFlags(),
	}

	cmd := &cobra.Command{
		Use:   "lock [HELM_COMMAND] [ARGS...] [flags]",
		Short: "Execute Helm commands with distributed locking",
		Long:  globalUsage,
		Example: strings.Join([]string{
			"  helm lock secrets upgrade my-release ./my-chart",
			"  helm lock upgrade my-release ./my-chart --lock-timeout 5m",
		}, "\n"),
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.helmCommand = args[0]
			opts.helmArgs = args[1:]

			// debug run
			if opts.helmCommand == "lock" {
				opts.helmCommand = args[1]
				opts.helmArgs = args[2:]
			}

			if n := len(opts.helmArgs); n > 0 {
				if n > 2 {
					n--
				}

				opts.releaseName = opts.helmArgs[n-1]
			}

			return runLockCommand(cmd.Context(), opts)
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.SetHelpCommand(&cobra.Command{}) // Disable the help command

	f := cmd.Flags()
	f.DurationVar(&opts.timeout, "lock-timeout", defaultLockTimeout, "Lock timeout duration")

	opts.helmSettings.AddFlags(f)

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		errorString := err.Error()
		if strings.Contains(errorString, "arg(s)") || strings.Contains(errorString, "required") {
			fmt.Fprintf(os.Stderr, "Error: %s\n\n", errorString)
			fmt.Fprintln(os.Stderr, cmd.UsageString())
		}
	}

	return err
}
