/*
Copyright 2023.

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
	logr "log"

	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	a "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	devpodv1alpha1 "github.com/clvx/devpod-operator/api/v1alpha1"
)

// DevPodReconciler reconciles a DevPod object
type DevPodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=devpod.bitclvx.com,namespace=devpod,resources=devpods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=devpod.bitclvx.com,namespace=devpod,resources=devpods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=devpod.bitclvx.com,namespace=devpod,resources=devpods/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,namespace=devpod,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,namespace=devpod,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,namespace=devpod,resources=virtualservices;gateways,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DevPod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *DevPodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	//log.Log.GetSink("Processing DevPodReconciler.")
	devPod := &devpodv1alpha1.DevPod{}
	err := r.Client.Get(ctx, req.NamespacedName, devPod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			//log.Log.Println("DevPod resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		//log.Log.Println(err, "Failed to get DevPod")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	deployment := &a.Deployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: devPod.Name, Namespace: devPod.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		dep := r.deployDevPodDeployment(devPod)
		logr.Println("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Client.Create(ctx, dep)
		if err != nil {
			logr.Println(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		logr.Println("Deployment Successful", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		//return ctrl.Result{Requeue: true}, nil //Do not return yet
	} else if err != nil {
		logr.Println(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	//Check service
	service := &corev1.Service{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: devPod.Name, Namespace: devPod.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		svc := r.deployDevPodService(devPod)
		logr.Println("Creating a new Deployment", "Deployment.Namespace", devPod.Namespace, "Deployment.Name", devPod.Name)
		err = r.Client.Create(ctx, svc)
		if err != nil {
			logr.Println(err, "Failed to create new Service", "Service.Namespace", devPod.Namespace, "Service.Name", devPod.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		logr.Println("Service Successful", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		//return ctrl.Result{Requeue: true}, nil //Do not return yet
	} else if err != nil {
		logr.Println(err, "ERROR: Failed to get Service")
		return ctrl.Result{}, err
	}

	// Check virtualservice
	virtualservice := &istio.VirtualService{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: devPod.Name, Namespace: devPod.Namespace}, virtualservice)
	if err != nil && errors.IsNotFound(err) {
		vs := r.deployDevPodVirtualService(devPod)
		logr.Println("Creating a new VirtualServer", "VirtualServer.Namespace", vs.Namespace, "VirtualServer.Name", vs.Name)
		//logr.Println("Creating a new VirtualServer", "VirtualServer.Namespace", "devpod", "VirtualServer.Name", "test-virtualservice")
		err = r.Client.Create(ctx, vs)
		if err != nil {
			logr.Println(err, "Failed to create new VirtualService", "VirtualService.Namespace", vs.Namespace, "VirtualService.Name", vs.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		logr.Println("VirtualService Successful", "VirtualService.Namespace", vs.Namespace, "VirtualService.Name", vs.Name)
		//return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logr.Println(err, "Failed to get virtualService: test-virtualservice")
		return ctrl.Result{}, err
	}

	//Check gateway
	gateway := &istio.Gateway{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: devPod.Name, Namespace: devPod.Namespace}, gateway)
	if err != nil && errors.IsNotFound(err) {
		gw := r.deployDevPodGateway(devPod)
		logr.Println("Creating a new Gateway", "Gateway.Namespace", gw.Namespace, "Gateway.Name", gw.Name)
		err = r.Client.Create(ctx, gw)
		if err != nil {
			logr.Println(err, "Failed to create new Gateway", "Gateway.Namespace", gw.Namespace, "Gateway.Name", gw.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		logr.Println("Gateway Successful", "Gateway.Namespace", gw.Namespace, "Gateway.Name", gw.Name)
		return ctrl.Result{Requeue: true}, nil //Do not return yet
	} else if err != nil {
		logr.Println(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (c *DevPodReconciler) deployDevPodGateway(dp *devpodv1alpha1.DevPod) *istio.Gateway {
	name := fmt.Sprintf("devpod-%s", dp.Name)
	port := uint32(dp.Spec.Port)
	selector := map[string]string{"istio": "ingressgateway"}
	domain := dp.Spec.ExternalDomain
	protocol := "HTTP"
	gateway := &istio.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dp.Namespace,
		},
		Spec: networkingv1beta1.Gateway{
			Servers: []*networkingv1beta1.Server{
				{
					Port: &networkingv1beta1.Port{
						Number:   port,
						Protocol: protocol,
					},
					Hosts: []string{
						fmt.Sprintf("%s.%s.svc.cluster.local", name, dp.Namespace),
						fmt.Sprintf("%s.%s", name, domain),
					},
				},
			},
			Selector: selector,
		},
	}
	ctrl.SetControllerReference(dp, gateway, c.Scheme)
	return gateway
}

func (c *DevPodReconciler) deployDevPodVirtualService(dp *devpodv1alpha1.DevPod) *istio.VirtualService {
	name := fmt.Sprintf("devpod-%s", dp.Name)
	//labels := dp.Labels
	port := uint32(dp.Spec.Port)
	domain := dp.Spec.ExternalDomain
	gateway := "devpod"
	prefix := "/"
	virtualservice := &istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dp.Namespace,
		},
		Spec: networkingv1beta1.VirtualService{
			Hosts: []string{
				fmt.Sprintf("%s.%s.svc.cluster.local", name, dp.Namespace),
				fmt.Sprintf("%s.%s", name, domain),
			},
			Gateways: []string{
				gateway,
			},
			Http: []*networkingv1beta1.HTTPRoute{
				{
					Match: []*networkingv1beta1.HTTPMatchRequest{
						{
							Uri: &networkingv1beta1.StringMatch{
								MatchType: &networkingv1beta1.StringMatch_Prefix{Prefix: prefix},
							},
						},
					},
					Route: []*networkingv1beta1.HTTPRouteDestination{
						{
							Destination: &networkingv1beta1.Destination{
								Host: fmt.Sprintf("%s.%s.svc.cluster.local", name, dp.Namespace),
								Port: &networkingv1beta1.PortSelector{Number: port},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(dp, virtualservice, c.Scheme)
	return virtualservice
}

func (c *DevPodReconciler) deployDevPodDeployment(dp *devpodv1alpha1.DevPod) *a.Deployment {
	name := fmt.Sprintf("devpod-%s", dp.Name)
	replicas := dp.Spec.Replicas
	image := dp.Spec.Image
	port := dp.Spec.Port
	labels := dp.Labels
	deployment := &a.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dp.Namespace,
		},
		Spec: a.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: image,
							Name:  name,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: port,
								},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(dp, deployment, c.Scheme)
	return deployment
}

func (c *DevPodReconciler) deployDevPodService(dp *devpodv1alpha1.DevPod) *corev1.Service {
	name := fmt.Sprintf("devpod-%s", dp.Name)
	port := dp.Spec.Port
	labels := dp.Labels
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dp.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					TargetPort: intstr.IntOrString{IntVal: port},
				},
			},
		},
	}
	ctrl.SetControllerReference(dp, service, c.Scheme)
	return service
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devpodv1alpha1.DevPod{}).
		Complete(r)
}
