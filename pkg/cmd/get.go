// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	yaml2 "github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewGetCmd returns a new get command.
func NewGetCmd(targetReader TargetReader, configReader ConfigReader,
	kubeconfigReader KubeconfigReader, kubeconfigWriter KubeconfigWriter, ioStreams IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get [(garden|project|seed|shoot|target) <name>]",
		Short:        "Get single resource instance or target stack, e.g. CRD of a shoot (default: current target)\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) < 1 || len(args) > 2 {
				return errors.New("command must be in the format: get [(garden|project|seed|shoot|target) <name>]")
			}

			switch args[0] {
			case "project":
				if len(args) == 1 {
					err = getProject("", targetReader, ioStreams)
					if err != nil {
						return err
					}
				} else if len(args) == 2 {
					err = getProject(args[1], targetReader, ioStreams)
					if err != nil {
						return err
					}
				}
			case "garden":
				if len(args) == 1 {
					err = getGarden("", configReader, targetReader, kubeconfigReader, ioStreams)
					if err != nil {
						return err
					}
				} else if len(args) == 2 {
					err = getGarden(args[1], configReader, targetReader, kubeconfigReader, ioStreams)
					if err != nil {
						return err
					}
				}
			case "seed":
				if len(args) == 1 {
					err = getSeed("", targetReader, ioStreams)
					if err != nil {
						return err
					}
				} else if len(args) == 2 {
					err = getSeed(args[1], targetReader, ioStreams)
					if err != nil {
						return err
					}
				}
			case "shoot":
				if len(args) == 1 {
					err = getShoot("", targetReader, kubeconfigWriter, ioStreams)
					if err != nil {
						return err
					}
				} else if len(args) == 2 {
					err = getShoot(args[1], targetReader, kubeconfigWriter, ioStreams)
					if err != nil {
						return err
					}
				}
			case "target":
				err = getTarget(targetReader, ioStreams)
				if err != nil {
					return err
				}
			default:
				fmt.Fprint(ioStreams.Out, "command must be in the format: get [project|garden|seed|shoot|target] + <NAME>")
			}

			return nil
		},
		ValidArgs: []string{"project", "garden", "seed", "shoot", "target"},
	}

	return cmd
}

// getProject lists
func getProject(name string, targetReader TargetReader, ioStreams IOStreams) error {
	target := targetReader.ReadTarget(pathTarget)
	if name == "" {
		if len(target.Stack()) < 2 {
			return errors.New("no project targeted")
		} else if len(target.Stack()) > 1 && target.Stack()[1].Kind == "project" {
			name = target.Stack()[1].Name
		} else if len(target.Stack()) > 1 && target.Stack()[1].Kind == "seed" {
			return errors.New("seed targeted, project expected")
		}
	}
	clientset, err := target.GardenerClient()
	if err != nil {
		return err
	}
	project, err := clientset.CoreV1beta1().Projects().Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if outputFormat == "yaml" {
		var output []byte
		if output, err = yaml.Marshal(project); err != nil {
			return err
		}
		fmt.Fprint(ioStreams.Out, string(output))
	} else if outputFormat == "json" {
		var output []byte
		if output, err = json.MarshalIndent(project, "", "  "); err != nil {
			return err
		}
		fmt.Fprint(ioStreams.Out, string(output))
	}

	return nil
}

// getGarden lists kubeconfig of garden cluster
func getGarden(name string, configReader ConfigReader, targetReader TargetReader, kubeconfigReader KubeconfigReader, ioStreams IOStreams) error {
	if name == "" {
		target := targetReader.ReadTarget(pathTarget)
		if len(target.Stack()) > 0 {
			name = target.Stack()[0].Name
		} else {
			return errors.New("no garden targeted")
		}
	}

	config := configReader.ReadConfig(pathGardenConfig)
	match := false
	for index, garden := range config.GardenClusters {
		if garden.Name == name {
			pathToKubeconfig := config.GardenClusters[index].KubeConfig
			if strings.Contains(pathToKubeconfig, "~") {
				pathToKubeconfig = filepath.Clean(filepath.Join(HomeDir(), strings.Replace(pathToKubeconfig, "~", "", 1)))
			}
			kubeconfig, err := kubeconfigReader.ReadKubeconfig(pathToKubeconfig)
			if err != nil {
				return err
			}
			if outputFormat == "yaml" {
				fmt.Fprint(ioStreams.Out, fmt.Sprintf("%s\n", kubeconfig))
			} else if outputFormat == "json" {
				y, err := yaml2.YAMLToJSON(kubeconfig)
				if err != nil {
					return err
				}
				var out bytes.Buffer
				if err = json.Indent(&out, y, "", "  "); err != nil {
					return err
				}
				fmt.Fprint(ioStreams.Out, out.String())
			}
			match = true
		}
	}
	if !match {
		return fmt.Errorf("no garden cluster found for %s", name)
	}

	return nil
}

// getSeed lists kubeconfig of seed cluster
func getSeed(name string, targetReader TargetReader, ioStreams IOStreams) error {
	target := targetReader.ReadTarget(pathTarget)
	if name == "" {
		if len(target.Stack()) > 1 && target.Stack()[1].Kind == "seed" {
			name = target.Stack()[1].Name
		} else if len(target.Stack()) > 1 && target.Stack()[1].Kind == "project" && len(target.Stack()) == 3 {
			name = getSeedForProject(target.Stack()[2].Name)
		} else {
			return errors.New("no seed targeted or shoot targeted")
		}
	}
	client, err := target.K8SClientToKind(TargetKindGarden)
	if err != nil {
		return err
	}
	gardenClientset, err := target.GardenerClient()
	if err != nil {
		return err
	}
	seed, err := gardenClientset.CoreV1beta1().Seeds().Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	kubeSecret, err := client.CoreV1().Secrets(seed.Spec.SecretRef.Namespace).Get(seed.Spec.SecretRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if outputFormat == "yaml" {
		fmt.Fprint(ioStreams.Out, fmt.Sprintf("%s\n", kubeSecret.Data["kubeconfig"]))
	} else if outputFormat == "json" {
		y, err := yaml2.YAMLToJSON(kubeSecret.Data["kubeconfig"])
		if err != nil {
			return err
		}
		var out bytes.Buffer
		if err = json.Indent(&out, y, "", "  "); err != nil {
			return err
		}
		fmt.Fprint(ioStreams.Out, out.String())
	}

	return nil
}

// getShoot lists kubeconfig of shoot
func getShoot(name string, targetReader TargetReader, kubeconfigWriter KubeconfigWriter, ioStreams IOStreams) error {
	target := targetReader.ReadTarget(pathTarget)
	if name == "" {
		if len(target.Stack()) < 3 {
			return errors.New("no shoot targeted")
		}
	} else if name != "" {
		if len(target.Stack()) < 2 {
			return errors.New("no seed or project targeted")
		}
	}
	client, err := target.K8SClientToKind(TargetKindGarden)
	if err != nil {
		return err
	}
	gardenClientset, err := target.GardenerClient()
	if err != nil {
		return err
	}
	var namespace string
	var shoot *gardencorev1beta1.Shoot
	if target.Stack()[1].Kind == "project" {
		project, err := gardenClientset.CoreV1beta1().Projects().Get(target.Stack()[1].Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if name == "" {
			shoot, err = gardenClientset.CoreV1beta1().Shoots(*project.Spec.Namespace).Get(target.Stack()[2].Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		if name != "" {
			shoot, err = gardenClientset.CoreV1beta1().Shoots(*project.Spec.Namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		namespace = shoot.Status.TechnicalID
	}
	if target.Stack()[1].Kind == "seed" {
		shootList, err := gardenClientset.CoreV1beta1().Shoots("").List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for index, s := range shootList.Items {
			if s.Name == target.Stack()[2].Name && *s.Spec.SeedName == target.Stack()[1].Name {
				if (name == "") && (s.Name == target.Stack()[2].Name) {
					shoot = &shootList.Items[index]
					namespace = shootList.Items[index].Status.TechnicalID
					break
				}
				if (name != "") && (s.Name == name) {
					shoot = &shootList.Items[index]
					namespace = shootList.Items[index].Status.TechnicalID
					break
				}
			}
		}
	}
	seed, err := gardenClientset.CoreV1beta1().Seeds().Get(*shoot.Spec.SeedName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	kubeSecret, err := client.CoreV1().Secrets(seed.Spec.SecretRef.Namespace).Get(seed.Spec.SecretRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	gardenName := target.Stack()[0].Name
	kubeconfigPath := filepath.Join(pathGardenHome, "cache", gardenName, "seeds", seed.Spec.SecretRef.Name, "kubeconfig.yaml")
	err = kubeconfigWriter.Write(kubeconfigPath, kubeSecret.Data["kubeconfig"])
	checkError(err)
	KUBECONFIG = kubeconfigPath

	seedClient, err := target.K8SClientToKind(TargetKindSeed)
	if err != nil {
		return err
	}
	kubeSecret, err = seedClient.CoreV1().Secrets(namespace).Get("kubecfg", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if outputFormat == "yaml" {
		fmt.Fprint(ioStreams.Out, fmt.Sprintf("%s\n", kubeSecret.Data["kubeconfig"]))
	} else if outputFormat == "json" {
		y, err := yaml2.YAMLToJSON(kubeSecret.Data["kubeconfig"])
		if err != nil {
			return err
		}
		var out bytes.Buffer
		if err = json.Indent(&out, y, "", "  "); err != nil {
			return err
		}
		fmt.Fprint(ioStreams.Out, out.String())
	}

	return nil
}

// getTarget prints the target stack.
func getTarget(targetReader TargetReader, ioStreams IOStreams) (err error) {
	target := targetReader.ReadTarget(pathTarget)
	if len(target.Stack()) == 0 {
		return errors.New("target stack is empty")
	}

	if outputFormat == "yaml" {
		var output []byte
		if output, err = yaml.Marshal(target); err != nil {
			return err
		}
		fmt.Fprint(ioStreams.Out, string(output))
	} else if outputFormat == "json" {
		var output []byte
		if output, err = json.MarshalIndent(target, "", "  "); err != nil {
			return err
		}
		fmt.Fprint(ioStreams.Out, string(output))
	}

	return nil
}
