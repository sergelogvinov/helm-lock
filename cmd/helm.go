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
	"os"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

// getAllFlags extracts all flags from os.Args except for --lock-timeout
func getAllFlags() []string {
	flags := []string{}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--lock-timeout") {
			continue
		}

		if strings.HasPrefix(arg, "-") {
			if strings.Contains(arg, "=") {
				flags = append(flags, arg)
			} else {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					flags = append(flags, arg, args[i+1])
					i++
				} else {
					flags = append(flags, arg)
				}
			}
		}
	}

	return flags
}

// getReleaseStatus returns the current release status
func getReleaseStatus(actionConfig *action.Configuration, releaseName string) (release.Status, error) {
	getAction := action.NewGet(actionConfig)

	rel, err := getAction.Run(releaseName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return release.StatusUnknown, nil
		}

		return release.StatusUnknown, err
	}

	return rel.Info.Status, nil
}

// performRollback performs a Helm rollback operation using Helm client
func performRollback(actionConfig *action.Configuration, releaseName string) error {
	rollbackAction := action.NewRollback(actionConfig)
	rollbackAction.Version = 0 // 0 means rollback to previous version
	rollbackAction.Wait = true
	rollbackAction.Timeout = 300 * time.Second

	if err := rollbackAction.Run(releaseName); err != nil {
		return err
	}

	return nil
}
