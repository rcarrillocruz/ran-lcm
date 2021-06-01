/*
Copyright 2021.

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ranv1alpha1 "github.com/redhat-ztp/ran-lcm/api/v1alpha1"
)

// GroupReconciler reconciles a Group object
type GroupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=ran.openshift.io,resources=groups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ran.openshift.io,resources=groups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ran.openshift.io,resources=groups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Group object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *GroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("group", req.NamespacedName)

	// your logic here
	group := &ranv1alpha1.Group{}
	err := r.Get(ctx, req.NamespacedName, group)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get Group")
		return ctrl.Result{}, err
	}

	err = r.ensurePlacementRule(ctx, group)

	return ctrl.Result{}, nil
}

func (r *GroupReconciler) ensurePlacementRule(ctx context.Context, group *ranv1alpha1.Group) error {
	for _, v := range group.Spec.Clusters {
		r.Log.Info("Creating PlacementRule object", "cluster", v)
		pr := r.newPlacementRule(ctx, group, v)
		r.Log.Info("Created PlacementRule object", "placementRule", pr)

		if err := controllerutil.SetControllerReference(group, pr, r.Scheme); err != nil {
			return err
		}

		foundPlacementRule := &unstructured.Unstructured{}
		foundPlacementRule.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "apps.open-cluster-management.io",
			Kind:    "PlacementRule",
			Version: "v1",
		})
		err := r.Client.Get(ctx, client.ObjectKey{
			Name:      group.Name + "-" + v + "placement-rule",
			Namespace: group.Namespace,
		}, foundPlacementRule)
		if err != nil && errors.IsNotFound((err)) {
			err = r.Client.Create(ctx, pr)
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (r *GroupReconciler) newPlacementRule(ctx context.Context, group *ranv1alpha1.Group, cluster string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      group.Name + "-" + cluster + "-" + "placement-rule",
			"namespace": group.Namespace,
			"labels": map[string]interface{}{
				"app": "ran-lcm",
			},
		},
		"spec": map[string]interface{}{
			"clusterConditions": []map[string]interface{}{
				{
					"type": "OK",
				},
			},
			"clusters": []map[string]interface{}{
				{
					"name": cluster,
				},
			},
		},
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "PlacementRule",
		Version: "v1",
	})

	return u
}

// SetupWithManager sets up the controller with the Manager.
func (r *GroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ranv1alpha1.Group{}).
		Complete(r)
}
