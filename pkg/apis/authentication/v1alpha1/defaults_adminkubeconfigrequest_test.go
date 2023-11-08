/*
Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
)

var _ = Describe("AdminKubeconfigRequest defaulting", func() {
	var obj *AdminKubeconfigRequest

	BeforeEach(func() {
		obj = &AdminKubeconfigRequest{}
	})

	Describe("ExpirationSeconds defaulting", func() {
		It("should default expirationSeconds field", func() {
			SetObjectDefaults_AdminKubeconfigRequest(obj)

			Expect(obj.Spec.ExpirationSeconds).To(Equal(pointer.Int64(60 * 60)))
		})

		It("should not default expirationSeconds field if it is already set", func() {
			obj.Spec.ExpirationSeconds = pointer.Int64(10 * 60)

			SetObjectDefaults_AdminKubeconfigRequest(obj)

			Expect(obj.Spec.ExpirationSeconds).To(Equal(pointer.Int64(10 * 60)))
		})
	})
})