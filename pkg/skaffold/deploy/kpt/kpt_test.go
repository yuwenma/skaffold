/*
Copyright 2020 The Skaffold Authors

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

package kpt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/deploy/kubectl"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/kubernetes/manifest"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/runner/runcontext"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
	"github.com/GoogleContainerTools/skaffold/testutil"
)

const (
	testPod = `apiVersion: v1
kind: Pod
metadata:
   namespace: default
spec:
   containers:
   - image: gcr.io/project/image1
   name: image1
`
)

// Test that kpt deployer manipulate manifests in the given order and no intermediate data is
// stored after each step:
//	Step 1. `kp fn source` (read in the manifest as stdin),
//  Step 2. `kustomize build` (hydrate the manifest),
//  Step 3. `kpt fn run` (validate, transform or generate the manifests via kpt functions),
//  Step 4. `kpt fn sink` (store the stdout in a given dir).
func TestKpt_Deploy(t *testing.T) {
	tests := []struct {
		description    string
		builds         []build.Artifact
		kpt            latest.KptDeploy
		kustomizations map[string]string
		commands       util.Command
		expected       []string
		shouldErr      bool
	}{
		{
			description: "no manifest",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", ``),
		},
		{
			description: "invalid manifest",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", `foo`).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			shouldErr: true,
		},
		{
			description: "invalid user specified applyDir",
			kpt: latest.KptDeploy{
				Dir: ".",
				Live: latest.KptLive{
					Apply: latest.KptApplyInventory{
						Dir: "invalid_path",
					},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			shouldErr: true,
		},

		{
			description: "kustomization and specified kpt fn",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn:  latest.KptFn{FnPath: "kpt-func.yaml"},
				Live: latest.KptLive{
					Apply: latest.KptApplyInventory{
						Dir: "valid_path",
					},
				},
			},
			kustomizations: map[string]string{"Kustomization": `resources:
				- foo.yaml`},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn source kpt-func.yaml", ``).
				AndRunOut(fmt.Sprintf("kpt fn sink %v", tmpKustomizeDir), ``).
				AndRunOut(fmt.Sprintf("kustomize build %v", tmpKustomizeDir), ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRun("kpt live apply valid_path"),
			expected: []string{"default"},
		},
		{
			description: "kpt live apply fails",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRunOut("kpt live init .kpt-hydrated", ``).
				AndRunErr("kpt live apply .kpt-hydrated", errors.New("BUG")),
			shouldErr: true,
		},
		{
			description: "user specifies reconcile timeout and poll period",
			kpt: latest.KptDeploy{
				Dir: ".",
				Live: latest.KptLive{
					Apply: latest.KptApplyInventory{
						Dir: "valid_path",
					},
					Options: latest.KptApplyOptions{
						PollPeriod:       "5s",
						ReconcileTimeout: "2m",
					},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRun("kpt live apply valid_path --poll-period 5s --reconcile-timeout 2m"),
		},
		{
			description: "user specifies invalid reconcile timeout and poll period",
			kpt: latest.KptDeploy{
				Dir: ".",
				Live: latest.KptLive{
					Apply: latest.KptApplyInventory{
						Dir: "valid_path",
					},
					Options: latest.KptApplyOptions{
						PollPeriod:       "foo",
						ReconcileTimeout: "bar",
					},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRun("kpt live apply valid_path --poll-period foo --reconcile-timeout bar"),
		},
		{
			description: "user specifies prune propagation policy and prune timeout",
			kpt: latest.KptDeploy{
				Dir: ".",
				Live: latest.KptLive{
					Apply: latest.KptApplyInventory{
						Dir: "valid_path",
					},
					Options: latest.KptApplyOptions{
						PrunePropagationPolicy: "Orphan",
						PruneTimeout:           "2m",
					},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRun("kpt live apply valid_path --prune-propagation-policy Orphan --prune-timeout 2m"),
		},
		{
			description: "user specifies invalid prune propagation policy and prune timeout",
			kpt: latest.KptDeploy{
				Dir: ".",
				Live: latest.KptLive{
					Apply: latest.KptApplyInventory{
						Dir: "valid_path",
					},
					Options: latest.KptApplyOptions{
						PrunePropagationPolicy: "foo",
						PruneTimeout:           "bar",
					},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", testPod).
				AndRun("kpt live apply valid_path --prune-propagation-policy foo --prune-timeout bar"),
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			t.Override(&util.DefaultExecCommand, test.commands)
			tmpDir := t.NewTempDir().Chdir()

			tmpDir.WriteFiles(test.kustomizations)

			k := NewDeployer(&kptConfig{
				kpt: test.kpt,
			}, nil)

			if k.Live.Apply.Dir == "valid_path" {
				// 0755 is a permission setting where the owner can read, write, and execute.
				// Others can read and execute but not modify the directory.
				os.Mkdir(k.Live.Apply.Dir, 0755)
			}

			_, err := k.Deploy(context.Background(), ioutil.Discard, test.builds)

			t.CheckError(test.shouldErr, err)
		})
	}
}

func TestKpt_Dependencies(t *testing.T) {
	tests := []struct {
		description    string
		kpt            latest.KptDeploy
		createFiles    map[string]string
		kustomizations map[string]string
		expected       []string
		shouldErr      bool
	}{
		{
			description: "bad dir",
			kpt: latest.KptDeploy{
				Dir: "invalid_path",
			},
			shouldErr: true,
		},
		{
			description: "empty dir and unspecified fnPath",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
		},
		{
			description: "dir",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			createFiles: map[string]string{
				"foo.yaml":  "",
				"README.md": "",
			},
			expected: []string{"foo.yaml"},
		},
		{
			description: "dir with subdirs and file path variants",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			createFiles: map[string]string{
				"food.yml":           "",
				"foo/bar.yaml":       "",
				"foo/bat//bad.yml":   "",
				"foo/bat\\README.md": "",
			},
			expected: []string{"foo/bar.yaml", "foo/bat/bad.yml", "food.yml"},
		},
		{
			description: "fnpath inside directory",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn:  latest.KptFn{FnPath: "."},
			},
			createFiles: map[string]string{
				"./kpt-func.yaml": "",
			},
			expected: []string{"kpt-func.yaml"},
		},
		{
			description: "fnpath outside directory",
			kpt: latest.KptDeploy{
				Dir: "./config",
				Fn:  latest.KptFn{FnPath: "./kpt-fn"},
			},
			createFiles: map[string]string{
				"./config/deployment.yaml": "",
				"./kpt-fn/kpt-func.yaml":   "",
			},
			expected: []string{"config/deployment.yaml", "kpt-fn/kpt-func.yaml"},
		},
		{
			description: "bad fnpath outside of config is watched",
			kpt: latest.KptDeploy{
				Dir: "./config",
				Fn:  latest.KptFn{FnPath: "./kpt-fn"},
			},
			createFiles: map[string]string{
				"./config/deployment.yaml": "",
				"./kpt-fn/kpt-func.yaml":   "bad json",
			},
			expected: []string{"config/deployment.yaml", "kpt-fn/kpt-func.yaml"},
		},
		{
			description: "bad fnpath inside of config is watched",
			kpt: latest.KptDeploy{
				Dir: "./config",
				Fn:  latest.KptFn{FnPath: "./config/kpt-fn"},
			},
			createFiles: map[string]string{
				"./config/deployment.yaml":      "",
				"./config/kpt-fn/kpt-func.yaml": "bad json",
			},
			expected: []string{"config/deployment.yaml", "config/kpt-fn/kpt-func.yaml"},
		},
		{
			description: "fnpath and dir and kustomization",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn:  latest.KptFn{FnPath: "./kpt-fn"},
			},
			createFiles: map[string]string{
				"./kpt-fn/func.yaml": "",
			},
			kustomizations: map[string]string{"kustomization.yaml": `configMapGenerator:
   - files: [app1.properties]`},
			expected: []string{"app1.properties", "kpt-fn/func.yaml", "kustomization.yaml"},
		},
		{
			description: "dependencies that can only be detected as a kustomization",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			kustomizations: map[string]string{"kustomization.yaml": `configMapGenerator:
   - files: [app1.properties]`},
			expected: []string{"app1.properties", "kustomization.yaml"},
		},
		{
			description: "kustomization.yml variant",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			kustomizations: map[string]string{"kustomization.yml": `configMapGenerator:
   - files: [app1.properties]`},
			expected: []string{"app1.properties", "kustomization.yml"},
		},
		{
			description: "Kustomization variant",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			kustomizations: map[string]string{"Kustomization": `configMapGenerator:
   - files: [app1.properties]`},
			expected: []string{"Kustomization", "app1.properties"},
		},
		{
			description: "incorrectly named kustomization",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			kustomizations: map[string]string{"customization": `configMapGenerator:
   - files: [app1.properties]`},
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			tmpDir := t.NewTempDir().Chdir()

			tmpDir.WriteFiles(test.createFiles)
			tmpDir.WriteFiles(test.kustomizations)

			k := NewDeployer(&kptConfig{
				kpt: test.kpt,
			}, nil)

			res, err := k.Dependencies()

			t.CheckErrorAndDeepEqual(test.shouldErr, err, tmpDir.Paths(test.expected...), tmpDir.Paths(res...))
		})
	}
}

func TestKpt_Cleanup(t *testing.T) {
	tests := []struct {
		description string
		applyDir    string
		globalFlags []string
		commands    util.Command
		shouldErr   bool
	}{
		{
			description: "invalid user specified applyDir",
			applyDir:    "invalid_path",
			shouldErr:   true,
		},
		{
			description: "valid user specified applyDir w/o template resource",
			applyDir:    "valid_path",
			commands:    testutil.CmdRunErr("kpt live destroy valid_path", errors.New("BUG")),
			shouldErr:   true,
		},
		{
			description: "valid user specified applyDir w/ template resource (emulated)",
			applyDir:    "valid_path",
			commands:    testutil.CmdRun("kpt live destroy valid_path"),
		},
		{
			description: "unspecified applyDir",
			commands: testutil.
				CmdRunOut("kpt live init .kpt-hydrated", "").
				AndRun("kpt live destroy .kpt-hydrated"),
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			t.Override(&util.DefaultExecCommand, test.commands)
			t.NewTempDir().Chdir()

			if test.applyDir == "valid_path" {
				// 0755 is a permission setting where the owner can read, write, and execute.
				// Others can read and execute but not modify the directory.
				os.Mkdir(test.applyDir, 0755)
			}

			k := NewDeployer(&kptConfig{
				workingDir: ".",
				kpt: latest.KptDeploy{
					Live: latest.KptLive{
						Apply: latest.KptApplyInventory{
							Dir: test.applyDir,
						},
					},
				},
			}, nil)

			err := k.Cleanup(context.Background(), ioutil.Discard)

			t.CheckError(test.shouldErr, err)
		})
	}
}

func TestKpt_Render(t *testing.T) {
	// The follow are outputs to `kpt fn run` commands.
	output1 := `apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1
    name: image1
`

	output2 := `apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1
    name: image1
  - image: gcr.io/project/image2
    name: image2
`

	output3 := `apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1
    name: image1
---
apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image2
    name: image2
`

	tests := []struct {
		description    string
		builds         []build.Artifact
		labels         map[string]string
		kpt            latest.KptDeploy
		commands       util.Command
		kustomizations map[string]string
		expected       string
		shouldErr      bool
	}{
		{
			description: "no fnPath or image specified",
			builds: []build.Artifact{
				{
					ImageName: "gcr.io/project/image1",
					Tag:       "gcr.io/project/image1:tag1",
				},
			},
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", output1).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: `apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1:tag1
    name: image1
`,
		},
		{
			description: "fnPath specified, multiple resources, and labels",
			builds: []build.Artifact{
				{
					ImageName: "gcr.io/project/image1",
					Tag:       "gcr.io/project/image1:tag1",
				},
				{
					ImageName: "gcr.io/project/image2",
					Tag:       "gcr.io/project/image2:tag2",
				},
			},
			labels: map[string]string{"user/label": "test"},
			kpt: latest.KptDeploy{
				Dir: "test",
				Fn:  latest.KptFn{FnPath: "kpt-func.yaml"},
			},
			commands: testutil.
				CmdRunOut("kpt fn source test", ``).
				AndRunOut("kpt fn source kpt-func.yaml", ``).
				AndRunOut("kpt fn run --dry-run", output3).
				AndRunOut("kpt fn sink .tmp-sink-dir/test", ``),
			expected: `apiVersion: v1
kind: Pod
metadata:
  labels:
    user/label: test
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1:tag1
    name: image1
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    user/label: test
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image2:tag2
    name: image2
`,
		},
		{
			description: "fn image specified, multiple images in resource",
			builds: []build.Artifact{
				{
					ImageName: "gcr.io/project/image1",
					Tag:       "gcr.io/project/image1:tag1",
				},
				{
					ImageName: "gcr.io/project/image2",
					Tag:       "gcr.io/project/image2:tag2",
				},
			},
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn:  latest.KptFn{Image: "gcr.io/example.com/my-fn:v1.0.0 -- foo=bar"},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run --image gcr.io/example.com/my-fn:v1.0.0 -- foo=bar", output2).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: `apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1:tag1
    name: image1
  - image: gcr.io/project/image2:tag2
    name: image2
`,
		},
		{
			description: "empty output from pipeline",
			builds: []build.Artifact{
				{
					ImageName: "gcr.io/project/image1",
					Tag:       "gcr.io/project/image1:tag1",
				},
			},
			labels: map[string]string{"user/label": "test"},
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", ``).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: "\n",
		},
		{
			description: "both fnPath and image specified",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn: latest.KptFn{
					FnPath: "kpt-func.yaml",
					Image:  "gcr.io/example.com/my-fn:v1.0.0 -- foo=bar"},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn source kpt-func.yaml", ``).
				AndRunOut("kpt fn run --dry-run --image gcr.io/example.com/my-fn:v1.0.0 -- foo=bar", ``).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			shouldErr: true,
		},
		{
			description: "kustomization render",
			builds: []build.Artifact{
				{
					ImageName: "gcr.io/project/image1",
					Tag:       "gcr.io/project/image1:tag1",
				},
			},
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut(fmt.Sprintf("kpt fn sink %v", tmpKustomizeDir), ``).
				AndRunOut(fmt.Sprintf("kustomize build %v", tmpKustomizeDir), ``).
				AndRunOut("kpt fn run --dry-run", output1),
			kustomizations: map[string]string{"kustomization.yaml": `resources:
- foo.yaml`},
			expected: `apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1:tag1
    name: image1
`,
		},
		{
			description: "reading configs from sourceDir fails",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOutErr("kpt fn source .", ``, errors.New("BUG")).
				AndRunOut("kpt fn run --dry-run", "invalid pipeline").
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			shouldErr: true,
		},
		{
			description: "outputting configs to sinkDir fails",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run", "invalid pipeline").
				AndRunOutErr("kpt fn sink .tmp-sink-dir", ``, errors.New("BUG")),
			shouldErr: true,
		},
		{
			description: "kustomize build fails (invalid kustomization config)",
			builds: []build.Artifact{
				{
					ImageName: "gcr.io/project/image1",
					Tag:       "gcr.io/project/image1:tag1",
				},
			},
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut(fmt.Sprintf("kpt fn sink %v", tmpKustomizeDir), ``).
				AndRunOutErr(fmt.Sprintf("kustomize build %v", tmpKustomizeDir), ``, errors.New("BUG")).
				AndRunOut("kpt fn run --dry-run", output1).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			kustomizations: map[string]string{"kustomization.yaml": `resources:
- foo.yaml`},
			shouldErr: true,
		},
		{
			description: "kpt fn run fails",
			kpt: latest.KptDeploy{
				Dir: ".",
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOutErr("kpt fn run --dry-run", "invalid pipeline", errors.New("BUG")).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			shouldErr: true,
		},
		{
			description: "kpt fn run with --global-scope",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn: latest.KptFn{
					Image:       "gcr.io/example.com/my-fn:v1.0.0 -- foo=bar",
					GlobalScope: true,
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run --global-scope --image gcr.io/example.com/my-fn:v1.0.0 -- foo=bar", ``).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: "\n",
		},
		{
			description: "kpt fn run with --mount arguments",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn: latest.KptFn{
					Image: "gcr.io/example.com/my-fn:v1.0.0 -- foo=bar",
					Mount: []string{"type=bind", "src=$(pwd)", "dst=/source"},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run --mount type=bind,src=$(pwd),dst=/source --image gcr.io/example.com/my-fn:v1.0.0 -- foo=bar", ``).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: "\n",
		},
		{
			description: "kpt fn run with invalid --mount arguments",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn: latest.KptFn{
					Image: "gcr.io/example.com/my-fn:v1.0.0 -- foo=bar",
					Mount: []string{"foo", "", "bar"},
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run --mount foo,,bar --image gcr.io/example.com/my-fn:v1.0.0 -- foo=bar", ``).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: "\n",
		},
		{
			description: "kpt fn run flag with --network and --network-name arguments",
			kpt: latest.KptDeploy{
				Dir: ".",
				Fn: latest.KptFn{
					Image:       "gcr.io/example.com/my-fn:v1.0.0 -- foo=bar",
					Network:     true,
					NetworkName: "foo",
				},
			},
			commands: testutil.
				CmdRunOut("kpt fn source .", ``).
				AndRunOut("kpt fn run --dry-run --network --network-name foo --image gcr.io/example.com/my-fn:v1.0.0 -- foo=bar", ``).
				AndRunOut("kpt fn sink .tmp-sink-dir", ``),
			expected: "\n",
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			t.Override(&util.DefaultExecCommand, test.commands)
			tmpDir := t.NewTempDir().Chdir()

			tmpDir.WriteFiles(test.kustomizations)

			k := NewDeployer(&kptConfig{
				workingDir: ".",
				kpt:        test.kpt,
			}, test.labels)

			var b bytes.Buffer
			err := k.Render(context.Background(), &b, test.builds, true, "")

			t.CheckErrorAndDeepEqual(test.shouldErr, err, test.expected, b.String())
		})
	}
}

func TestKpt_GetApplyDir(t *testing.T) {
	tests := []struct {
		description string
		live        latest.KptLive
		expected    string
		commands    util.Command
		shouldErr   bool
	}{
		{
			description: "specified an invalid applyDir",
			live: latest.KptLive{
				Apply: latest.KptApplyInventory{
					Dir: "invalid_path",
				},
			},
			shouldErr: true,
		},
		{
			description: "specified a valid applyDir",
			live: latest.KptLive{
				Apply: latest.KptApplyInventory{
					Dir: "valid_path",
				},
			},
			expected: "valid_path",
		},
		{
			description: "unspecified applyDir",
			expected:    ".kpt-hydrated",
			commands:    testutil.CmdRunOut("kpt live init .kpt-hydrated", ""),
		},
		{
			description: "unspecified applyDir with specified inventory-id and namespace",
			live: latest.KptLive{
				Apply: latest.KptApplyInventory{
					InventoryID:        "1a23bcde-4f56-7891-a2bc-de34fabcde5f6",
					InventoryNamespace: "foo",
				},
			},
			expected: ".kpt-hydrated",
			commands: testutil.CmdRunOut("kpt live init .kpt-hydrated --inventory-id 1a23bcde-4f56-7891-a2bc-de34fabcde5f6 --namespace foo", ""),
		},
		{
			description: "existing template resource in .kpt-hydrated",
			expected:    ".kpt-hydrated",
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			t.Override(&util.DefaultExecCommand, test.commands)
			tmpDir := t.NewTempDir().Chdir()

			if test.live.Apply.Dir == test.expected {
				// 0755 is a permission setting where the owner can read, write, and execute.
				// Others can read and execute but not modify the directory.
				os.Mkdir(test.live.Apply.Dir, 0755)
			}

			if test.description == "existing template resource in .kpt-hydrated" {
				tmpDir.Touch(".kpt-hydrated/inventory-template.yaml")
			}

			k := NewDeployer(&kptConfig{
				workingDir: ".",
				kpt: latest.KptDeploy{
					Live: test.live,
				},
			}, nil)

			applyDir, err := k.getApplyDir(context.Background())

			t.CheckErrorAndDeepEqual(test.shouldErr, err, test.expected, applyDir)
		})
	}
}

func TestKpt_KptCommandArgs(t *testing.T) {
	tests := []struct {
		description string
		dir         string
		commands    []string
		flags       []string
		globalFlags []string
		expected    []string
	}{
		{
			description: "empty",
		},
		{
			description: "all inputs have len >0",
			dir:         "test",
			commands:    []string{"live", "apply"},
			flags:       []string{"--fn-path", "kpt-func.yaml"},
			globalFlags: []string{"-h"},
			expected:    strings.Split("live apply test --fn-path kpt-func.yaml -h", " "),
		},
		{
			description: "empty dir",
			commands:    []string{"live", "apply"},
			flags:       []string{"--fn-path", "kpt-func.yaml"},
			globalFlags: []string{"-v", "3"},
			expected:    strings.Split("live apply --fn-path kpt-func.yaml -v 3", " "),
		},
		{
			description: "empty commands",
			dir:         "test",
			flags:       []string{"--fn-path", "kpt-func.yaml"},
			globalFlags: []string{"-h"},
			expected:    strings.Split("test --fn-path kpt-func.yaml -h", " "),
		},
		{
			description: "empty flags",
			dir:         "test",
			commands:    []string{"live", "apply"},
			globalFlags: []string{"-h"},
			expected:    strings.Split("live apply test -h", " "),
		},
		{
			description: "empty globalFlags",
			dir:         "test",
			commands:    []string{"live", "apply"},
			flags:       []string{"--fn-path", "kpt-func.yaml"},
			expected:    strings.Split("live apply test --fn-path kpt-func.yaml", " "),
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			res := kptCommandArgs(test.dir, test.commands, test.flags, test.globalFlags)
			t.CheckDeepEqual(test.expected, res)
		})
	}
}

// TestKpt_ExcludeKptFn checks the declarative kpt fn has expected annotations added.
func TestKpt_ExcludeKptFn(t *testing.T) {
	// A declarative fn.
	testFn1 := []byte(`apiVersion: v1
data:
  annotation_name: k1
  annotation_value: v1
kind: ConfigMap
metadata:
  annotations:
    config.kubernetes.io/function: fake`)
	// A declarative fn which has `local-config` annotation specified.
	testFn2 := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    config.kubernetes.io/function: fake
    config.kubernetes.io/local-config: "false"
data:
  annotation_name: k2
  annotation_value: v2`)
	testPod := []byte(`apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1
    name: image1`)
	tests := []struct {
		description string
		manifests   manifest.ManifestList
		expected    manifest.ManifestList
	}{
		{
			description: "Add `local-config` annotation to kpt fn",
			manifests:   manifest.ManifestList{testFn1},
			expected: manifest.ManifestList{[]byte(`apiVersion: v1
data:
  annotation_name: k1
  annotation_value: v1
kind: ConfigMap
metadata:
  annotations:
    config.kubernetes.io/function: fake
    config.kubernetes.io/local-config: "true"`)},
		},
		{
			description: "Skip preset `local-config` annotation",
			manifests:   manifest.ManifestList{testFn2},
			expected: manifest.ManifestList{[]byte(`apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    config.kubernetes.io/function: fake
    config.kubernetes.io/local-config: "false"
data:
  annotation_name: k2
  annotation_value: v2`)},
		},
		{
			description: "Valid in kpt fn pipeline.",
			manifests:   manifest.ManifestList{testFn1, testFn2, testPod},
			expected: manifest.ManifestList{[]byte(`apiVersion: v1
data:
  annotation_name: k1
  annotation_value: v1
kind: ConfigMap
metadata:
  annotations:
    config.kubernetes.io/function: fake
    config.kubernetes.io/local-config: "true"`), []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    config.kubernetes.io/function: fake
    config.kubernetes.io/local-config: "false"
data:
  annotation_name: k2
  annotation_value: v2`), []byte(`apiVersion: v1
kind: Pod
metadata:
  namespace: default
spec:
  containers:
  - image: gcr.io/project/image1
    name: image1`)},
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			k := NewDeployer(&kptConfig{}, nil)
			actualManifest, err := k.excludeKptFn(test.manifests)
			t.CheckErrorAndDeepEqual(false, err, test.expected.String(), actualManifest.String())
		})
	}
}

type kptConfig struct {
	runcontext.RunContext // Embedded to provide the default values.
	workingDir            string
	kpt                   latest.KptDeploy
}

func (c *kptConfig) WorkingDir() string       { return c.workingDir }
func (c *kptConfig) GetKubeContext() string   { return kubectl.TestKubeContext }
func (c *kptConfig) GetKubeNamespace() string { return kubectl.TestNamespace }
func (c *kptConfig) Pipeline() latest.Pipeline {
	var pipeline latest.Pipeline
	pipeline.Deploy.DeployType.KptDeploy = &c.kpt
	return pipeline
}
