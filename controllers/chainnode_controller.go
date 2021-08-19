/*
Copyright 2021 buaa-cita.

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
	"fmt"
	citacloudv1 "github.com/buaa-cita/chainnode/api/v1"
	// appv1 "k8s.io/api/apps/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	// corev1 "k8s.io/api/core/v1"
)

// ChainNodeReconciler reconciles a ChainNode object
type ChainNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// 节点级别的配置，每个节点在符合当前链的治理下，选择自己的个性化配置
/*
	pvcName // 此节点配置文件的路径
	name // 节点的名字
	chainconfig // 指定使用的链config
	kms_password  // 节点的kms服务的密码
	network-key // 节点的私钥
	六个微服务的配置：包括：
		is_std_out // 是否将日志输出到标准输出
		log-level // 日志等级
		以及其他可以各个节点不同的配置

	service.port //服务的端口
	service.eipName

	// loadbanlancer，可以交由运维的人员来处理，
		service.endIP //多集群
		service.interface
		service.startIP

*/

//+kubebuilder:rbac:groups=citacloud.buaa.edu.cn,resources=chainnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=citacloud.buaa.edu.cn,resources=chainnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=citacloud.buaa.edu.cn,resources=chainnodes/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=deployments/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ChainNode object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile

func (r *ChainNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("start reconcile")
	var chainNode citacloudv1.ChainNode

	// fetch chainNode
	if err := r.Get(ctx, req.NamespacedName, &chainNode); err != nil {
		logger.Error(err, "get ChainNode failed")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// fetch chainConfig
	var configName string
	if chainNode.Spec.ConfigName == "" {
		// not configName not setted
		configName = "chainconfig-sample"
	} else {
		configName = chainNode.Spec.ConfigName
	}

	configKey := types.NamespacedName{
		Namespace: req.NamespacedName.Namespace,
		Name:      configName}
	var chainConfig citacloudv1.ChainConfig
	if err := r.Get(ctx, configKey, &chainConfig); err != nil {
		if apierror.IsNotFound(err) {
			//dont show too much info if it is a not found error
			logger.Info("can not find chainconfig")
		} else {
			logger.Error(err, "get chainconfig failed")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.reconcileNodeDeployment(ctx, &chainNode, &chainConfig); err != nil {
		logger.Error(err, "reconcile node deployment failed")
		return ctrl.Result{}, nil
	}

	if err := r.reconcileNodeDeployment(ctx, &chainNode, &chainConfig); err != nil {
		logger.Error(err, "reconcile node deployment failed")
		return ctrl.Result{}, nil
	}

	if err := r.reconcileKmsSecret(ctx, &chainConfig); err != nil {
		logger.Error(err, "reconcile kms Secret failed")
		return ctrl.Result{}, nil
	}

	if err := r.reconcileNodeNetworkSecret(ctx, &chainNode, &chainConfig); err != nil {
		logger.Error(err, "reconcile node network secret failed")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChainNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&citacloudv1.ChainNode{}).
		Watches(&source.Kind{Type: &citacloudv1.ChainConfig{}},
			handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []ctrl.Request {
				// fmt.Println("map func called", obj.GetNamespace(), obj.GetName())
				ctx := context.Background()
				var nodeList citacloudv1.ChainNodeList
				reqList := make([]reconcile.Request, 0)
				if err := r.List(ctx, &nodeList); err != nil {
					fmt.Println(err)
					return reqList
				}
				for _, node := range nodeList.Items {

					reqList = append(reqList, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: node.GetNamespace(),
							Name:      node.GetName(),
						}})
					//fmt.Println("req", node.GetName())
				}
				return reqList
			})).
		//Watches(&source.Kind{Type: &citacloudv1.ChainConfig{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
