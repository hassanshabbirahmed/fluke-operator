/*
Copyright 2026.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	flukev1alpha1 "github.com/hassanshabbirahmed/fluke-operator/api/v1alpha1"
)

var _ = Describe("Fluke Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		fluke := &flukev1alpha1.Fluke{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Fluke")
			err := k8sClient.Get(ctx, typeNamespacedName, fluke)
			if err != nil && errors.IsNotFound(err) {
				resource := &flukev1alpha1.Fluke{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					// Image is required (MinLength=1) - the CRD's validation
					// rejects the zero-value Fluke{} the scaffold started with.
					Spec: flukev1alpha1.FlukeSpec{
						Replicas:         1,
						FlukesPerReplica: 1,
						Image:            "busybox",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &flukev1alpha1.Fluke{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Fluke")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FlukeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
