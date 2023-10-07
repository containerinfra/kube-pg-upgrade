package kubevolumes

import (
	v1 "k8s.io/api/core/v1"
)

func NewVolumeMount(name, path string, readOnly bool) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      name,
		MountPath: path,
		ReadOnly:  readOnly,
	}
}

func NewVolumeSubPathMount(name, path string, subPath string, readOnly bool) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      name,
		MountPath: path,
		SubPath:   subPath,
		ReadOnly:  readOnly,
	}
}

func NewVolumeFromSecret(name, secretName string) v1.Volume {
	return NewVolumeFromSecretOptional(name, secretName, false)
}

func NewVolumeFromSecretOptional(name, secretName string, optional bool) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: secretName,
				Optional:   &optional,
			},
		},
	}
}

func NewProjectedVolumeFromSecrets(name string, secretNames ...string) v1.Volume {
	sources := make([]v1.VolumeProjection, 0, len(secretNames))
	for _, secretName := range secretNames {
		sources = append(sources, v1.VolumeProjection{
			Secret: &v1.SecretProjection{
				LocalObjectReference: v1.LocalObjectReference{
					Name: secretName,
				},
			},
		})
	}

	var defaultMode int32 = 0600
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			Projected: &v1.ProjectedVolumeSource{
				Sources:     sources,
				DefaultMode: &defaultMode,
			},
		},
	}
}

func NewVolumeFromConfigMap(name, configName string) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configName,
				},
			},
		},
	}
}

func NewHostPathVolume(name, hostPath string, mountType v1.HostPathType) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: hostPath,
				Type: &mountType,
			},
		},
	}
}

func NewEmptyDirVolume(name string) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
}
func NewPersistentVolumeClaimVolume(name, claimName string, readOnly bool) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
				ReadOnly:  readOnly,
			},
		},
	}
}
