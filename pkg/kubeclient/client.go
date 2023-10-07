package kubeclient

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetClientConfig() (clientcmd.ClientConfig, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides), nil
}

// GetClient creates a new k8s client object from the currently configured kubecontext
func GetClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", getKubeConfig())
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfigOrDie(config), nil
}

func GetClientOrDie() *kubernetes.Clientset {
	client, err := GetClient()
	if err != nil {
		panic(err)
	}
	return client
}

func GetKubeConfigOrInCluster() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", getKubeConfig())
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}

func GetKubeConfigFile(filepath string) (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags("", filepath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func GetClientWithConfigFromFile(filepath string) (*kubernetes.Clientset, error) {
	config, err := GetKubeConfigFile(filepath)
	if err != nil {
		return nil, err
	}
	return GetClientWithConfig(config)
}

func GetClientWithConfig(kubeconfigFile *rest.Config) (*kubernetes.Clientset, error) {
	// create the clientset
	clientset, err := kubernetes.NewForConfig(kubeconfigFile)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

// GetKubeConfig returns the currently configured kubeconfig file location
// or error if non has been configured
func GetKubeConfig() string {
	return getKubeConfig()
}

func getKubeConfig() string {
	if kubeConfig := getEnv("KUBECONFIG", ""); kubeConfig != "" {
		return kubeConfig
	}

	if home := homeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	panic("Missing kubeconfig configuration ...")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
