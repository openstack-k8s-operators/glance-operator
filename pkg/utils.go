package glance

func GetLabels(name string) map[string]string {
	return map[string]string{"owner": "glance-operator", "cr": name, "app": AppLabel}
}
