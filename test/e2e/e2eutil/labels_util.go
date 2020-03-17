package e2eutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// OperatorLabelSelector returns a label selector for Galera Operator Pod
func OperatorLabelSelector() map[string]string {
	return map[string]string{"name": "galera-operator", "stage":"test"}
}

// GaleraLabelSelector returns a label selector for Galera Cluster
func GaleraLabelSelector() map[string]string {
	return map[string]string{"name": "galera-cluster", "stage":"test"}
}

func GaleraListOpt() metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(GaleraLabelSelector()).String(),
	}
}

