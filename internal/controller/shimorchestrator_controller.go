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
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1 "github.com/neonephos-katalis/opg-ewbi-operator/api/operator/v1beta1"
)

const (
	shimManagedByLabel       = "app.kubernetes.io/managed-by"
	shimManagedByValue       = "opg-ewbi-shim-orchestrator"
	shimAppDeploymentLabel   = "opg.ewbi.katalis.com/application-deployment"
	shimDependencyRetryDelay = 2 * time.Second
)

// ShimOrchestratorReconciler is a lab stand-in for the Partner OP edge
// orchestrator. It supports one container component per application.
type ShimOrchestratorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ShimOrchestratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Image{}).
		Named("shim-image").
		Complete(&shimImageReconciler{Client: r.Client}); err != nil {
		return err
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Artefact{}).
		Named("shim-artefact").
		Complete(&shimArtefactReconciler{Client: r.Client}); err != nil {
		return err
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ApplicationOnboarding{}).
		Named("shim-application-onboarding").
		Complete(&shimApplicationOnboardingReconciler{Client: r.Client}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.ApplicationDeployment{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("shim-application-deployment").
		Complete(r)
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=*
// +kubebuilder:rbac:groups="",resources=services,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

type shimImageReconciler struct{ client.Client }

func (r *shimImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var image v1beta1.Image
	if err := r.Get(ctx, req.NamespacedName, &image); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if image.Spec.RelationType != string(v1beta1.FederationRelationHost) || !image.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	if image.Status.State == "" {
		return ctrl.Result{RequeueAfter: shimDependencyRetryDelay}, nil
	}
	if image.Status.State == v1beta1.ImageStateReady {
		return ctrl.Result{}, nil
	}
	original := image.DeepCopy()
	image.Status.State = v1beta1.ImageStateReady
	return ctrl.Result{}, r.Status().Patch(ctx, &image, client.MergeFrom(original))
}

type shimArtefactReconciler struct{ client.Client }

func (r *shimArtefactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var artefact v1beta1.Artefact
	if err := r.Get(ctx, req.NamespacedName, &artefact); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if artefact.Spec.RelationType != string(v1beta1.FederationRelationHost) || !artefact.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	if artefact.Status.State == "" {
		return ctrl.Result{RequeueAfter: shimDependencyRetryDelay}, nil
	}
	if artefact.Status.State == v1beta1.ArtefactStateReady {
		return ctrl.Result{}, nil
	}
	original := artefact.DeepCopy()
	artefact.Status.State = v1beta1.ArtefactStateReady
	return ctrl.Result{}, r.Status().Patch(ctx, &artefact, client.MergeFrom(original))
}

type shimApplicationOnboardingReconciler struct{ client.Client }

func (r *shimApplicationOnboardingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var app v1beta1.ApplicationOnboarding
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if app.Spec.RelationType != string(v1beta1.FederationRelationHost) || !app.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	if app.Status.State == "" {
		return ctrl.Result{RequeueAfter: shimDependencyRetryDelay}, nil
	}
	if app.Status.State == v1beta1.ApplicationOnboardingStateOnboarded {
		return ctrl.Result{}, nil
	}
	original := app.DeepCopy()
	app.Status.State = v1beta1.ApplicationOnboardingStateOnboarded
	return ctrl.Result{}, r.Status().Patch(ctx, &app, client.MergeFrom(original))
}

func (r *ShimOrchestratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("name", req.Name, "namespace", req.Namespace)
	var appDeployment v1beta1.ApplicationDeployment
	if err := r.Get(ctx, req.NamespacedName, &appDeployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if appDeployment.Spec.RelationType != string(v1beta1.FederationRelationHost) || !appDeployment.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	if appDeployment.Status.AppInstanceInfo.AppInstanceState == "" {
		return ctrl.Result{RequeueAfter: shimDependencyRetryDelay}, nil
	}

	component, image, err := r.resolveWorkload(ctx, &appDeployment)
	if err != nil {
		logger.Info("shim dependencies not ready", "reason", err.Error())
		return ctrl.Result{RequeueAfter: shimDependencyRetryDelay}, nil
	}

	deployment, err := r.reconcileDeployment(ctx, &appDeployment, component, image)
	if err != nil {
		return ctrl.Result{}, err
	}
	service, err := r.reconcileService(ctx, &appDeployment, component)
	if err != nil {
		return ctrl.Result{}, err
	}
	if deployment.Status.AvailableReplicas < replicasFor(component) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.publishReady(ctx, &appDeployment, component, service)
}

func (r *ShimOrchestratorReconciler) resolveWorkload(
	ctx context.Context,
	appDeployment *v1beta1.ApplicationDeployment,
) (*v1beta1.ComponentSpec, string, error) {
	federation, err := GetFederationByContextId(ctx, r.Client, appDeployment.Spec.FederationContextId, appDeployment.Namespace)
	if err != nil {
		return nil, "", err
	}
	if federation.Spec.FederationData.RelationType != string(v1beta1.FederationRelationHost) {
		return nil, "", fmt.Errorf("federation %q is not host-side", federation.Name)
	}

	app, err := r.findApplication(ctx, appDeployment.Namespace, appDeployment.Spec.AppId)
	if err != nil {
		return nil, "", err
	}
	if len(app.Spec.AppInfo.AppDeploymentZones) > 0 {
		allowed := false
		for _, zone := range app.Spec.AppInfo.AppDeploymentZones {
			if zone.ZoneId == appDeployment.Spec.ZoneId {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, "", fmt.Errorf("zone %q not allowed by application %q", appDeployment.Spec.ZoneId, app.Spec.AppInfo.AppId)
		}
	}
	if len(app.Spec.AppInfo.AppComponentSpecs) == 0 {
		return nil, "", fmt.Errorf("application %q has no components", app.Spec.AppInfo.AppId)
	}

	artefact, err := r.findArtefact(ctx, appDeployment.Namespace, app.Spec.AppInfo.AppComponentSpecs[0].ArtefactId)
	if err != nil {
		return nil, "", err
	}
	if artefact.Spec.ArtefactBody == nil || len(artefact.Spec.ArtefactBody.ComponentSpec) == 0 {
		return nil, "", fmt.Errorf("artefact %q has no component spec", artefact.Spec.ArtefactId)
	}
	if artefact.Spec.ArtefactBody.ArtefactVirtType != "CONTAINER_TYPE" {
		return nil, "", fmt.Errorf("artefact %q is not a container", artefact.Spec.ArtefactId)
	}
	component := &artefact.Spec.ArtefactBody.ComponentSpec[0]
	if len(component.Images) == 0 {
		return nil, "", fmt.Errorf("component %q has no image", component.ComponentName)
	}

	imageResource, err := r.findImage(ctx, appDeployment.Namespace, component.Images[0])
	if err != nil {
		return nil, "", err
	}
	if imageResource.Spec.ImageBody == nil {
		return nil, "", fmt.Errorf("image %q has no image body", imageResource.Spec.ImageId)
	}
	return component, containerImage(imageResource.Spec.ImageBody), nil
}

func (r *ShimOrchestratorReconciler) findApplication(ctx context.Context, namespace, id string) (*v1beta1.ApplicationOnboarding, error) {
	var list v1beta1.ApplicationOnboardingList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.RelationType == string(v1beta1.FederationRelationHost) && item.Spec.AppInfo != nil && item.Spec.AppInfo.AppId == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("application %q not found", id)
}

func (r *ShimOrchestratorReconciler) findArtefact(ctx context.Context, namespace, id string) (*v1beta1.Artefact, error) {
	var list v1beta1.ArtefactList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.RelationType == string(v1beta1.FederationRelationHost) && item.Spec.ArtefactId == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("artefact %q not found", id)
}

func (r *ShimOrchestratorReconciler) findImage(ctx context.Context, namespace, id string) (*v1beta1.Image, error) {
	var list v1beta1.ImageList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.RelationType == string(v1beta1.FederationRelationHost) && item.Spec.ImageId == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("image %q not found", id)
}

func containerImage(body *v1beta1.ImageBody) string {
	name := body.ImageName
	if body.ImageRepoLocation != nil && body.ImageRepoLocation.RepoURL != "" {
		name = strings.TrimSuffix(body.ImageRepoLocation.RepoURL, "/")
	}
	if body.ImageVersionInfo != "" {
		name += ":" + body.ImageVersionInfo
	}
	return name
}

func (r *ShimOrchestratorReconciler) reconcileDeployment(
	ctx context.Context,
	appDeployment *v1beta1.ApplicationDeployment,
	component *v1beta1.ComponentSpec,
	image string,
) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: appDeployment.Name, Namespace: appDeployment.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		selector := map[string]string{shimAppDeploymentLabel: appDeployment.Name}
		deployment.Labels = map[string]string{shimManagedByLabel: shimManagedByValue, shimAppDeploymentLabel: appDeployment.Name}
		deployment.Spec.Replicas = ptrInt32(replicasFor(component))
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
		deployment.Spec.Template.Labels = selector

		container := corev1.Container{Name: dnsName(component.ComponentName), Image: image}
		if component.CommandLineParams != nil {
			container.Command = component.CommandLineParams.Command
			container.Args = component.CommandLineParams.CommandArgs
		}
		for _, iface := range component.ExposedInterfaces {
			container.Ports = append(container.Ports, corev1.ContainerPort{
				Name:          portName(iface.InterfaceId),
				ContainerPort: iface.Port,
				Protocol:      protocolFor(iface.Protocol),
			})
		}
		if resources := resourcesFor(component); len(resources) > 0 {
			container.Resources = corev1.ResourceRequirements{Requests: resources, Limits: resources}
		}
		deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
		return controllerutil.SetControllerReference(appDeployment, deployment, r.Scheme)
	})
	return deployment, err
}

func (r *ShimOrchestratorReconciler) reconcileService(
	ctx context.Context,
	appDeployment *v1beta1.ApplicationDeployment,
	component *v1beta1.ComponentSpec,
) (*corev1.Service, error) {
	external := externalInterfaces(component)
	if len(external) == 0 {
		return nil, nil
	}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: appDeployment.Name, Namespace: appDeployment.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Labels = map[string]string{shimManagedByLabel: shimManagedByValue, shimAppDeploymentLabel: appDeployment.Name}
		service.Spec.Type = corev1.ServiceTypeNodePort
		service.Spec.Selector = map[string]string{shimAppDeploymentLabel: appDeployment.Name}
		ports := make([]corev1.ServicePort, 0, len(external))
		for _, iface := range external {
			port := corev1.ServicePort{
				Name:       portName(iface.InterfaceId),
				Port:       iface.Port,
				TargetPort: intstr.FromInt32(iface.Port),
				Protocol:   protocolFor(iface.Protocol),
			}
			for _, existing := range service.Spec.Ports {
				if existing.Name == port.Name {
					port.NodePort = existing.NodePort
				}
			}
			ports = append(ports, port)
		}
		service.Spec.Ports = ports
		return controllerutil.SetControllerReference(appDeployment, service, r.Scheme)
	})
	return service, err
}

func (r *ShimOrchestratorReconciler) publishReady(
	ctx context.Context,
	appDeployment *v1beta1.ApplicationDeployment,
	component *v1beta1.ComponentSpec,
	service *corev1.Service,
) error {
	original := appDeployment.DeepCopy()
	if appDeployment.Status.AppInstanceInfo.AppInstIdentifier == "" {
		appDeployment.Status.AppInstanceInfo.AppInstIdentifier = appDeployment.Spec.AppInstanceId
	}
	appDeployment.Status.AppInstanceInfo.AppInstanceState = v1beta1.ApplicationDeploymentStateReady
	appDeployment.Status.AccessPointInfo = r.accessPoints(ctx, component, service)
	return r.Status().Patch(ctx, appDeployment, client.MergeFrom(original))
}

func (r *ShimOrchestratorReconciler) accessPoints(ctx context.Context, component *v1beta1.ComponentSpec, service *corev1.Service) []v1beta1.AccessPointInfo {
	if service == nil {
		return nil
	}
	var nodeIPs []string
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err == nil {
		for _, node := range nodes.Items {
			for _, address := range node.Status.Addresses {
				if address.Type == corev1.NodeInternalIP {
					nodeIPs = append(nodeIPs, address.Address)
				}
			}
		}
	}
	external := externalInterfaces(component)
	result := make([]v1beta1.AccessPointInfo, 0, len(service.Spec.Ports))
	for i, servicePort := range service.Spec.Ports {
		interfaceID := servicePort.Name
		if i < len(external) {
			interfaceID = external[i].InterfaceId
		}
		result = append(result, v1beta1.AccessPointInfo{
			InterfaceId: accessPointInterfaceId(interfaceID),
			AccessPoints: &v1beta1.AccessPoints{
				Port:          int(servicePort.NodePort),
				Fqdn:          fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace),
				Ipv4Addresses: nodeIPs,
			},
		})
	}
	return result
}

func externalInterfaces(component *v1beta1.ComponentSpec) []v1beta1.ExposedInterfaceInfo {
	var result []v1beta1.ExposedInterfaceInfo
	for _, iface := range component.ExposedInterfaces {
		if iface.VisibilityType == "VISIBILITY_EXTERNAL" {
			result = append(result, iface)
		}
	}
	return result
}

func replicasFor(component *v1beta1.ComponentSpec) int32 {
	if component.NumOfInstances > 0 {
		return component.NumOfInstances
	}
	return 1
}

func resourcesFor(component *v1beta1.ComponentSpec) corev1.ResourceList {
	result := corev1.ResourceList{}
	if component.ComputeResourceProfile == nil {
		return result
	}
	if component.ComputeResourceProfile.Memory > 0 {
		result[corev1.ResourceMemory] = resource.MustParse(fmt.Sprintf("%dMi", component.ComputeResourceProfile.Memory))
	}
	if cpu, err := resource.ParseQuantity(component.ComputeResourceProfile.NumCPU); err == nil && !cpu.IsZero() {
		result[corev1.ResourceCPU] = cpu
	}
	return result
}

func protocolFor(protocol string) corev1.Protocol {
	if strings.EqualFold(protocol, "UDP") {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}

func dnsName(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "_", "-"))
	value = strings.Trim(value, "-")
	if len(value) > 63 {
		value = value[:63]
	}
	if value == "" {
		return "application"
	}
	return value
}

func portName(value string) string {
	value = dnsName(value)
	if len(value) > 15 {
		value = value[:15]
	}
	return value
}

// accessPointInterfaceId normalizes an interface identifier so it satisfies
// the EWBI accessPointInfo pattern: ^[A-Za-z0-9][A-Za-z0-9_]{6,30}[A-Za-z0-9]$.
func accessPointInterfaceId(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune('_')
		}
	}
	value = strings.Trim(b.String(), "_")
	if value == "" {
		value = "interface"
	}
	if len(value) > 31 {
		value = value[:31]
	}
	for len(value) < 8 {
		value += "0"
	}
	if strings.HasSuffix(value, "_") {
		value = value[:len(value)-1] + "0"
	}
	return value
}

func ptrInt32(value int32) *int32 { return &value }
