// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package component

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/gardener/gardener/pkg/controllerutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	StaticPodsManifestsPath  = "/etc/kubernetes/manifests"
	VolumeRootDirPlaceholder = "@"
)

type VolumeData struct {
	data map[string][]byte
	err  error
}

func NewVolumeData(
	data map[string][]byte,
) *VolumeData {
	if data == nil {
		data = map[string][]byte{}
	}
	return &VolumeData{
		data: data,
	}
}

func (v *VolumeData) WriteStaticPodScript(ctx context.Context, client client.Client, namespace, podName string, podSpec *corev1.PodSpec) error {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "staticpod-" + podName, Namespace: namespace}}

	if v.err != nil {
		return fmt.Errorf("collecting volume data failed: %w", v.err)
	}

	volumesPath := path.Join(StaticPodsManifestsPath, podName+"-volumes")

	// fix volume paths
	for i := range podSpec.Volumes {
		hostPath := podSpec.Volumes[i].HostPath
		if hostPath != nil && strings.HasPrefix(hostPath.Path, VolumeRootDirPlaceholder) {
			hostPath.Path = path.Join(volumesPath, hostPath.Path[len(VolumeRootDirPlaceholder)+1:])
		}
	}

	buf := &bytes.Buffer{}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, client, cm, func() error {
		pod := &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "kube-system"},
			Spec:       *podSpec,
		}
		podYaml, err := yaml.Marshal(pod)
		if err != nil {
			return fmt.Errorf("marshalling pod failed: %w", err)
		}
		filename := path.Join(StaticPodsManifestsPath, podName)
		if err := appendFile(buf, filename, []byte(podYaml)); err != nil {
			return err
		}
		for name, data := range v.data {
			if err := appendFile(buf, path.Join(volumesPath, name), data); err != nil {
				return err
			}
		}

		cm.Data = map[string]string{"script": buf.String()}
		return nil
	})
	return err
}

func (v *VolumeData) AddVolume(deployment *appsv1.Deployment, volume corev1.Volume, data any) {

	if v == nil {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volume)
		return
	}
	switch typedData := data.(type) {
	case map[string]string:
		for key, value := range typedData {
			v.data[volume.Name+"/"+key] = []byte(value)
		}
	case map[string][]byte:
		for key, value := range typedData {
			v.data[volume.Name+"/"+key] = value
		}
	default:
		v.err = errors.Join(v.err, fmt.Errorf("unexpected data for volume %s", volume.Name))
	}
	typ := corev1.HostPathDirectoryOrCreate
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volume.Name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path.Join(VolumeRootDirPlaceholder, volume.Name),
				Type: &typ,
			},
		},
	})
}

func appendFile(buf *bytes.Buffer, filename string, data []byte) error {
	if _, err := buf.WriteString("mkdir -p " + path.Dir(filename) + "\n"); err != nil {
		return err
	}

	if _, err := buf.WriteString("cat << EOF | base64 -d > '" + filename + "'\n"); err != nil {
		return err
	}

	str := base64.StdEncoding.EncodeToString(data)
	if _, err := buf.WriteString(str + "\n"); err != nil {
		return err
	}

	if _, err := buf.WriteString("EOF\n"); err != nil {
		return err
	}
	return nil
}
