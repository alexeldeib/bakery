package main

import (
	"github.com/Azure/go-autorest/autorest/to"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getTweakPod(node string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "prebake-",
			Namespace:    "kube-system",
			Labels: map[string]string{
				"app": "bakery",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "tweaks",
					Image:           "alexeldeib/nsenter:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"/nsenter",
						"-t",
						"1",
						"-m",
						"--",
						"bash",
						"-xc",
						tweakCmd,
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: to.BoolPtr(true),
					},
				},
			},
			HostNetwork:   true,
			HostPID:       true,
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": node,
			},
			Tolerations: []corev1.Toleration{
				{
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpExists,
				},
				{
					Effect:   corev1.TaintEffectNoExecute,
					Operator: corev1.TolerationOpExists,
				},
			},
		},
	}
}

const tweakCmd = `/bin/sleep 5 && /bin/echo "$(/bin/date) VMSS-Prototype Donor: $(/bin/hostname)" >> /var/log/ancestry.log && /bin/rm -rf /etc/hostname /var/lib/cloud/data/* /var/lib/cloud/instance /var/lib/cloud/instances/* /var/lib/waagent/history/* /var/lib/waagent/events/* && /bin/cp /dev/null /etc/machine-id && /bin/systemctl poweroff --force --message=VMSS-Prototype`
