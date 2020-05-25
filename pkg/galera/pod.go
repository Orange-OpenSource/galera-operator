// Copyright 2020 Orange SA
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package galera

import (
	"fmt"
	apigalera "galera-operator/pkg/apis/apigalera/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewGaleraPod(spec *apigalera.GaleraSpec, name, clusterName, clusterNamespace, role, addresses, bootstrapImage, backupImage, revision, state string,
	init bool, credsMap map[string]string, owner metav1.OwnerReference) *corev1.Pod {
	pod := newGaleraPod(spec, name, clusterName, clusterNamespace, role, addresses, bootstrapImage, backupImage, revision, state, init, credsMap)
	applyPodTemplate(pod, spec, role)

	addOwnerRefToObject(pod.GetObjectMeta(), owner)
	return pod
}

func newGaleraPod(galspec *apigalera.GaleraSpec, name, clusterName, clusterNamespace, role, addresses, bootstrapImage, backupImage, revision, state string,
	init bool, credsMap map[string]string) *corev1.Pod {
	bootstrap := bootstrapContainer(bootstrapImage, credsMap["user"], credsMap["password"], clusterName, name, addresses, state)
	container := galeraContainer(galspec.Pod.Image, credsMap["user"], credsMap["password"], role)

	var cfgMapVolume corev1.Volume

	if role == apigalera.RoleSpecial {
		container.Env = appendGaleraEnv(galspec.Special.GaleraSpecialEnv, addresses, init)

		if galspec.Special.MycnfSpecialConfigMap == nil {
			cfgMapVolume = configMapVolume(galspec.Pod.MycnfConfigMap)
		} else {
			cfgMapVolume = configMapVolume(galspec.Special.MycnfSpecialConfigMap)
		}
	} else {
		container.Env = appendGaleraEnv(galspec.Pod.Env, addresses, init)
		cfgMapVolume = configMapVolume(galspec.Pod.MycnfConfigMap)
	}

	containers := []corev1.Container{container}

	if galspec.Pod.Metric != nil {
		if galspec.Pod.Metric.Image != "" {
			containers = append(containers, metricContainer(galspec.Pod.Metric.Image, galspec.Pod.Metric.Env, *galspec.Pod.Metric.Port))
		}
	}

	if role == apigalera.Backup || role == apigalera.Restore {
		containers = append(containers, backupContainer(backupImage, role))
	}

	volumes := []corev1.Volume{cfgMapVolume, bootstrapVolume(), dataVolume(name)}

//	if role == apigalera.Backup || role == apigalera.Restore {
	if role == apigalera.Backup {
		volumes = append(volumes, backupVolume(name))
	}

	if role == apigalera.Restore {
		volumes = append(volumes, restoreVolume())
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: PodLabelsForGalera(clusterName, clusterNamespace, role, revision, state),
		},
		Spec: corev1.PodSpec{
			InitContainers:               []corev1.Container{bootstrap},
			Containers:                   containers,
			Volumes:                      volumes,
			RestartPolicy: 				  corev1.RestartPolicyNever,
			// DNS A record: `[Hostname].[Subdomain].Namespace.svc`
			Hostname:                     name,
			Subdomain:                    clusterName,
			AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
			PriorityClassName:            galspec.Pod.PriorityClassName,
			SecurityContext:              podSecurityContext(galspec.Pod),
		},
	}
}

func bootstrapContainer(bootstrapImage, user, password, clusterName, name, addresses, state string) corev1.Container {
	cluster := "true"
	if state == apigalera.StateStandalone {
		cluster = "false"
	}

	argsContainer := []string{fmt.Sprintf("-cluster-addresses=%s", addresses)}
	argsContainer = append(argsContainer, fmt.Sprintf("-input-conf=%s/my.cnf", apigalera.ConfigMapVolumeMountPath))
	argsContainer = append(argsContainer, fmt.Sprintf("-output-conf=%s/my.cnf", apigalera.BootstrapVolumeMountPath))
	argsContainer = append(argsContainer, fmt.Sprintf("-cluster-name=%s", clusterName))
	argsContainer = append(argsContainer, fmt.Sprintf("-node-name=%s", name))
	argsContainer = append(argsContainer, fmt.Sprintf("-user=%s", user))
	argsContainer = append(argsContainer, fmt.Sprintf("-password=%s", password))
	argsContainer = append(argsContainer, fmt.Sprintf("-cluster=%s", cluster))

	envVar := []corev1.EnvVar{
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
	}

	return corev1.Container{
		Image:           bootstrapImage,
		Name:            apigalera.BootstrapContainerName,
		Args:            argsContainer,
		ImagePullPolicy: corev1.PullAlways,
		Env:             envVar,
		VolumeMounts:    []corev1.VolumeMount{configMapVolumeMount(), bootstrapVolumeMount()},
	}
}

func galeraContainer(image, user, password , role string) corev1.Container {
	argContainer := []string{fmt.Sprintf("--defaults-file=%s/my.cnf", apigalera.BootstrapVolumeMountPath)}

	mounts := []corev1.VolumeMount{bootstrapVolumeMount(), galeraVolumeMount(role)}

	if role == apigalera.Backup {
		mounts = append(mounts, backupVolumeMount())
	}

	return corev1.Container{
		Image:           image,
		Name:            apigalera.GaleraContainerName,
		Args:            argContainer,
		ImagePullPolicy: corev1.PullAlways,
		Ports:           galeraContainerPorts(),
		LivenessProbe:   galeraProbe(user, password),
		ReadinessProbe:  galeraProbe(user, password),
		VolumeMounts:    mounts,
	}
}

func metricContainer(image string, envVar []corev1.EnvVar, port int32) corev1.Container {
	return corev1.Container{
		Image:           image,
		Name:            apigalera.MetricContainerName,
		ImagePullPolicy: corev1.PullAlways,
		Ports:           metricContainerPorts(port),
		LivenessProbe:   metricProbe(port),
		Env:             envVar,
	}
}

func backupContainer(image, role string) corev1.Container {
	argsContainer := []string{fmt.Sprintf("--port=%d", apigalera.BackupAPIPort), "--host=$(POD_IP)"}

	var mount corev1.VolumeMount

	if role == apigalera.Restore {
		mount = restoreVolumeMount()
	} else {
		mount = backupVolumeMount()
	}

	return corev1.Container{
		Image:           image,
		Name:            apigalera.BackupContainerName,
		Args:            argsContainer,
		Env:			 []corev1.EnvVar{
							{
								Name: 		"POD_IP",
								ValueFrom:	&corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "status.podIP",
									},
								},
							},
						},
		ImagePullPolicy: corev1.PullAlways,
		Ports:           []corev1.ContainerPort{
							{
								Name:          "backupapi",
								ContainerPort: int32(apigalera.BackupAPIPort),
								Protocol:      corev1.ProtocolTCP,
							},
						 },
		LivenessProbe:   backupProbe(),
		VolumeMounts:    []corev1.VolumeMount{mount},
	}
}

func configMapVolume(obj *corev1.LocalObjectReference) corev1.Volume {
	return corev1.Volume{
		Name: apigalera.ConfigMapVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: *obj,
			},
		},
	}
}

func bootstrapVolume() corev1.Volume {
	return corev1.Volume{
		Name: apigalera.BootstrapVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func dataVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: apigalera.DataVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: BuildClaimNameForGalera(name),
			},
		},
	}
}

func backupVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: apigalera.BackupVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: BuildBackupClaimNameForGalera(name),
			},
		},
	}
}

func restoreVolume() corev1.Volume {
	return corev1.Volume{
		Name: apigalera.RestoreVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func appendGaleraEnv(envVarGalera []corev1.EnvVar, addresses string, init bool) []corev1.EnvVar {

	var envVar []corev1.EnvVar

	if addresses == "" && init == true {
		envVar = append(envVar, corev1.EnvVar{Name: "CLUSTER_INIT", Value: "true"})
	}

	return mergeEnvs(envVar, envVarGalera)
}

// mergeEnvs merges e2 into e1. Conflicting EnvVar will be skipped.
func mergeEnvs(e1, e2 []corev1.EnvVar) []corev1.EnvVar {
	for _, e := range e2 {
		if e.Name == "CLUSTER_INIT" {
			continue
		}

		e1 = append(e1, e)
	}

	return e1
}

func galeraContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "mysql",
			ContainerPort: int32(apigalera.MySQLPort),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          "sst",
			ContainerPort: int32(apigalera.StateSnapshotTransfertPort),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          "ist",
			ContainerPort: int32(apigalera.IncrementalStateTransferPort),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          "replication",
			ContainerPort: int32(apigalera.GaleraReplicationPort),
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func metricContainerPorts(port int32) []corev1.ContainerPort {
	if port == 0 {
		port = int32(apigalera.GaleraMetrics)
	}

	return []corev1.ContainerPort{
		{
			Name:          "metrics",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func configMapVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      apigalera.ConfigMapVolumeName,
		MountPath: apigalera.ConfigMapVolumeMountPath,
	}
}

func bootstrapVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name: apigalera.BootstrapVolumeName,
		MountPath: apigalera.BootstrapVolumeMountPath,
	}
}

func galeraVolumeMount(role string) corev1.VolumeMount {
	if role == apigalera.Restore {
		return corev1.VolumeMount{
			Name: apigalera.RestoreVolumeName,
			MountPath: apigalera.DataVolumeMountPath,
		}
	}
	return corev1.VolumeMount{
		Name: apigalera.DataVolumeName,
		MountPath: apigalera.DataVolumeMountPath,
	}
}

func backupVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name: apigalera.BackupVolumeName,
		MountPath: apigalera.BackupVolumeMountPath,
	}
}

func restoreVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name: apigalera.DataVolumeName,
		MountPath: apigalera.BackupVolumeMountPath,
	}
}

/*
func galeraProbe(user, password string) *corev1.Probe {
	cmd := []string{"mysql", "-h", "localhost"}
//	cmd := []string{"mysql", "-h", "127.0.0.1"}
	cmd = append(cmd, fmt.Sprintf("-u%s", user))
	cmd = append(cmd, fmt.Sprintf("-p%s", password))
	cmd = append(cmd, "-e", "SELECT 1")

	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: cmd,
			},
		},
		InitialDelaySeconds: 15,
//		TimeoutSeconds:      1,
//		PeriodSeconds:       10,
//		SuccessThreshold:    1,
//		FailureThreshold:    3,
	}
}
*/

func galeraProbe(user, password string) *corev1.Probe {
	cmd := []string{"mysqladmin"}
	cmd = append(cmd, fmt.Sprintf("-u%s", user))
	cmd = append(cmd, fmt.Sprintf("-p%s", password))
	cmd = append(cmd, "ping")

	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: cmd,
			},
		},
		InitialDelaySeconds: 30,
//		TimeoutSeconds:      1,
//		PeriodSeconds:       10,
//		SuccessThreshold:    1,
//		FailureThreshold:    3,
	}
}

func metricProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.IntOrString{IntVal: port},
			},
		},
		InitialDelaySeconds: 30,
		//		TimeoutSeconds:      1,
		//		PeriodSeconds:       10,
		//		SuccessThreshold:    1,
		//		FailureThreshold:    3,
	}
}

func backupProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/probe",
				Port: intstr.IntOrString{IntVal: apigalera.BackupAPIPort},
			},
		},
		InitialDelaySeconds: 5,
		//		TimeoutSeconds:      1,
		//		PeriodSeconds:       10,
		//		SuccessThreshold:    1,
		//		FailureThreshold:    3,
	}
}

func applyPodTemplate(pod *corev1.Pod, spec *apigalera.GaleraSpec, role string) {
	podTemplate := spec.Pod

	if podTemplate.Affinity != nil {
		pod.Spec.Affinity = podTemplate.Affinity
	}

	if len(podTemplate.NodeSelector) != 0 {
		pod.Spec.NodeSelector = podTemplate.NodeSelector
	}

	if len(podTemplate.Tolerations) != 0 {
		pod.Spec.Tolerations = podTemplate.Tolerations
	}

	if role != apigalera.RoleSpecial {
		pod.Spec.Containers[0].Resources = podTemplate.Resources
	} else {
		if spec.Special.SpecialResources == nil {
			pod.Spec.Containers[0].Resources = podTemplate.Resources
		} else {
			pod.Spec.Containers[0].Resources = *spec.Special.SpecialResources
		}
	}

	if podTemplate.Metric != nil {
		pod.Spec.Containers[1].Resources = podTemplate.Metric.Resources
	}
}

func podSecurityContext(podPolicy *apigalera.PodTemplate) *corev1.PodSecurityContext {
	if podPolicy == nil {
		return nil
	}
	return podPolicy.SecurityContext
}
